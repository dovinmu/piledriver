# Bug Reproducers for hugot-usage Hunt

Two critical bugs were found in `PooledHugotEmbedder` and confirmed with real-world testing.

## Test Matrix

| Test Suite | Pre-Fix | Post-Fix |
|------------|---------|----------|
| Happy Path (7 tests) | PASS | PASS |
| Bug 1: Close during Embed | **SIGSEGV crash** | PASS |
| Bug 2: Multiple Close | **Panic** | PASS |

## Quick Summary

| Bug | Trigger | Result | Severity |
|-----|---------|--------|----------|
| Bug 1 | `Close()` during `Embed()` | SIGSEGV crash | CRITICAL |
| Bug 2 | Multiple `Close()` calls | Panic during panic | HIGH |

## Setup (Required for all ONNX tests)

From the `termite` directory:

```bash
cd termite

# Download ONNX Runtime if not already done
make e2e-deps

# Set environment variables (macOS)
export ONNXRUNTIME_ROOT=$PWD/onnxruntime
export LIBRARY_PATH=$ONNXRUNTIME_ROOT/darwin-arm64/lib:$LIBRARY_PATH
export DYLD_LIBRARY_PATH=$ONNXRUNTIME_ROOT/darwin-arm64/lib:$DYLD_LIBRARY_PATH

# Set environment variables (Linux - use instead of above)
# export ONNXRUNTIME_ROOT=$PWD/onnxruntime
# export LIBRARY_PATH=$ONNXRUNTIME_ROOT/linux-amd64/lib:$LIBRARY_PATH
# export LD_LIBRARY_PATH=$ONNXRUNTIME_ROOT/linux-amd64/lib:$LD_LIBRARY_PATH
```

## Pattern Demos (No Dependencies)

These demonstrate the bug patterns without requiring ONNX Runtime:

```bash
cd spec/hugot-usage/reproducers

# Bug 1: Close() during active operation
cd bug1_close_during_embed && go run pattern_demo.go

# Bug 2: Multiple Close() calls
cd bug2_multiple_close && go run pattern_demo.go
```

## Bug Reproducers (Requires ONNX)

These tests are in `pkg/termite/lib/embeddings/close_race_test.go`.

**Pre-fix**: These tests will CRASH (SIGSEGV or panic).
**Post-fix**: These tests will PASS.

From the `termite` directory (after setup above):

```bash
# Bug 1 - Close() during Embed()
# Pre-fix: SIGSEGV crash
# Post-fix: PASS
go test -v -tags="onnx,ORT" -run TestCloseWhileEmbedding ./pkg/termite/lib/embeddings/

# Bug 2 - Multiple Close() calls
# Pre-fix: panic
# Post-fix: PASS
go test -v -tags="onnx,ORT" -run TestMultipleCloseIsSafe ./pkg/termite/lib/embeddings/

# Run both bug tests together
go test -v -tags="onnx,ORT" -run "TestCloseWhileEmbedding|TestMultipleCloseIsSafe" ./pkg/termite/lib/embeddings/
```

## Happy Path Tests (Requires ONNX)

These tests verify normal usage works. They should PASS before and after the fix.

### E2E Bash Script (Full Stack Test)

Builds the binary, starts the server, tests HTTP endpoints:

```bash
cd termite
../spec/hugot-usage/reproducers/e2e_happy_path.sh
```

Tests:
1. Single text embedding
2. Multiple text embeddings (batch of 3)
3. Concurrent requests (10 parallel)
4. Large batch (20 texts)
5. Graceful shutdown

### Unit Tests

From the `termite` directory (after setup above):

```bash
# Run all 7 happy path tests
go test -v -tags="onnx,ORT" -run TestHappyPath ./pkg/termite/lib/embeddings/
```

## The Fixes

Both bugs are fixed by adding synchronization to `PooledHugotEmbedder`:

```go
type PooledHugotEmbedder struct {
    // ... existing fields ...

    // NEW: Synchronization
    closed    atomic.Bool    // Prevents new Embed() after Close()
    wg        sync.WaitGroup // Waits for in-flight Embed() calls
    closeOnce sync.Once      // Ensures Close() runs exactly once
    closeErr  error          // Stores error from close
}

func (p *PooledHugotEmbedder) Embed(ctx context.Context, contents [][]ai.ContentPart) ([][]float32, error) {
    // Check closed before starting
    if p.closed.Load() {
        return nil, errors.New("embedder is closed")
    }

    // Track in-flight operation
    p.wg.Add(1)
    defer p.wg.Done()

    // Double-check after registration
    if p.closed.Load() {
        return nil, errors.New("embedder is closed")
    }

    // ... rest unchanged ...
}

func (p *PooledHugotEmbedder) Close() error {
    p.closeOnce.Do(func() {
        // Set closed flag (prevents new Embed calls)
        p.closed.Store(true)

        // Wait for in-flight operations
        p.wg.Wait()

        // Now safe to destroy
        if p.session != nil && !p.sessionShared {
            p.logger.Info("Destroying Hugot session")
            p.closeErr = p.session.Destroy()
        }
    })
    return p.closeErr
}
```

## Comparing Before/After the Fix

The fix is on branch `fix/pooled-hugot-embedder-close-race` in the termite repo.

```bash
cd termite

# Setup (one time)
make e2e-deps
export ONNXRUNTIME_ROOT=$PWD/onnxruntime
export LIBRARY_PATH=$ONNXRUNTIME_ROOT/darwin-arm64/lib:$LIBRARY_PATH
export DYLD_LIBRARY_PATH=$ONNXRUNTIME_ROOT/darwin-arm64/lib:$DYLD_LIBRARY_PATH

# === TEST PRE-FIX (expect crashes) ===
git checkout 709c7f9

# Bug 1 - expect SIGSEGV
go test -v -tags="onnx,ORT" -run TestCloseWhileEmbedding ./pkg/termite/lib/embeddings/

# Bug 2 - expect panic
go test -v -tags="onnx,ORT" -run TestMultipleCloseIsSafe ./pkg/termite/lib/embeddings/

# === TEST POST-FIX (expect passes) ===
git checkout fix/pooled-hugot-embedder-close-race

# Bug 1 - expect PASS
go test -v -tags="onnx,ORT" -run TestCloseWhileEmbedding ./pkg/termite/lib/embeddings/

# Bug 2 - expect PASS
go test -v -tags="onnx,ORT" -run TestMultipleCloseIsSafe ./pkg/termite/lib/embeddings/

# === HAPPY PATH (should pass on both) ===
go test -v -tags="onnx,ORT" -run TestHappyPath ./pkg/termite/lib/embeddings/
```

## Git References

| Repo | Branch | Description |
|------|--------|-------------|
| termite | `709c7f9` | Pre-fix (bugs present) |
| termite | `fix/pooled-hugot-embedder-close-race` | Post-fix (bugs fixed) |
| piledriver | `fix/pooled-hugot-embedder-close-race` | Reproducers and E2E tests |

## File Structure

```
reproducers/
├── README.md                          # This file
├── e2e_happy_path.sh                  # Full stack E2E test (build, run, HTTP)
├── bug1_close_during_embed/
│   ├── README.md                      # Bug description
│   ├── pattern_demo.go                # Standalone demo (no ONNX)
│   ├── bug1_test.go                   # Real reproducer (crashes)
│   └── fix.go                         # Proposed fix
└── bug2_multiple_close/
    ├── README.md                      # Bug description
    ├── pattern_demo.go                # Standalone demo (no ONNX)
    ├── bug2_test.go                   # Real reproducer (crashes)
    └── fix.go                         # Proposed fix
```
