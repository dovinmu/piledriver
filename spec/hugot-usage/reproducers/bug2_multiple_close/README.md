# Bug 2: Multiple/Concurrent Close() Calls

## Summary
Calling `Close()` multiple times or concurrently causes a fatal panic in the Hugot/ONNX session destruction.

## Verbose Description
`session.Destroy()` in the Hugot library is not idempotent or thread-safe. When multiple goroutines call `Close()` concurrently:

1. Multiple goroutines enter `session.Destroy()` simultaneously
2. They race on releasing ONNX Runtime resources
3. The CGO layer panics with "semasleep on Darwin signal stack"
4. This triggers "panic during panic" - an unrecoverable state

The crash:
```
fatal error: semasleep on Darwin signal stack
panic during panic

github.com/knights-analytics/hugot.(*Session).Destroy(...)
```

## Root Cause
```go
// hugot.go:377-385
func (p *PooledHugotEmbedder) Close() error {
    if p.session != nil && !p.sessionShared {
        return p.session.Destroy()  // ← Not protected by sync.Once!
    }
    return nil
}
```

No `sync.Once` or mutex to ensure `Destroy()` is only called once.

## Files
- `pattern_demo.go` - Standalone pattern demonstration (no ONNX required)
- `bug2_test.go` - Reproducer with real ONNX (crashes)
- `fix.go` - Proposed fix

## Run
```bash
# Pattern demo (no dependencies):
go run pattern_demo.go

# Real reproducer (from termite directory with ONNX env vars):
go test -v -tags="onnx,ORT" -run TestBug2 ./spec/hugot-usage/reproducers/bug2_multiple_close/
```
