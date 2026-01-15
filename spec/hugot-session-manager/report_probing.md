# Real-World Probing Report: hugot-session-manager

## Summary
The real-world probing phase has **confirmed** the Use-After-Free race condition identified by the TLA+ model.

## 1. Assumption A3 (Exclusive Ownership) Verification
- **Status**: **VIOLATED**
- **Evidence**: `grep` search found multiple calls to `hugot.NewSession` bypassing the `SessionManager`.
- **Files**:
    - `pkg/termite/lib/reranking/hugot.go`
    - `pkg/termite/lib/rebel/rebel.go`
    - `pkg/termite/lib/seq2seq/hugot.go`
- **Impact**: Architecture does not enforce "Single ONNX Session". Multithreaded usage is unsafe.

## 2. Race Condition Reproduction
- **Status**: **CONFIRMED**
- **Test**: `termite/pkg/termite/lib/hugot/race_test.go`
- **Method**: Spun up a client thread using a session, then called `Close()` immediately from another thread.
- **Result**: `Close()` proceeded to call `Destroy()` (causing a panic in our mock setup because the empty session struct was invalid).
- **Correct Behavior**: `Close()` should have blocked until the client released the session.
- **Observed Behavior**: `Close()` executed immediately, creating a window where the client is holding a destroyed pointer.

## Recommendations
1. **Refactoring**: Introduce `SessionHandle` with reference counting (Add/Release).
    - `GetSession` returns `*SessionHandle`.
    - `SessionHandle` increments refcount.
    - `SessionHandle.Close()` decrements refcount.
    - `SessionManager.Close()` sets `closed` flag and waits for refcount == 0.
2. **Architecture**: Enforce Singleton pattern for ONNX backend within the `hugot` library itself or strictly funnel all access through `SessionManager`. Remove direct `NewSession` calls from other packages.

## Artifacts
- **Test File**: `termite/pkg/termite/lib/hugot/race_test.go`
