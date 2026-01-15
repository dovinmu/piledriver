# Hunt Report: hugot-session-manager

## Summary
The suspect was the `SessionManager` in `termite/pkg/termite/lib/hugot/session_manager.go`. We suspected concurrency issues with session management. Model checking confirmed a **Use-After-Free** vulnerability where `Close()` destroys sessions that are actively being used by clients for inference. Real-world probing revealed a crucial violation of system assumptions: **Assumption A3 (Exclusive Ownership) is false**, as multiple components create sessions directly, bypassing the manager completely.

## Boundary
We modeled:
- `SessionManager` state (sessions map, mutex, closed flag)
- `GetSession` logic (creation, retrieval, locking)
- `Close` logic
- Concurrent Clients performing proper API usage (GetSession -> Inference -> Loop)

## Findings

### From TLA+ Model Checking
- **Invariant Violation**: `UseAfterFree`
- **Trace**:
    1. Client 1 gets a session (ONNX).
    2. Client 1 starts inference (using the session pointer).
    3. Client 2 calls `Close()` on the manager.
    4. `Close()` acquires the lock and calls `Destroy()` on the session.
    5. Client 1 is now holding a destroyed session handle.
- This confirms that `SessionManager` does not track active usage of sessions, only ownership.

### Assumption Analysis
- **A1, A4, A5**: Held.
- **A3 (Exclusive Ownership) FAILED**:
    - The `SessionManager` documentation states: "IMPORTANT: ONNX Runtime allows only ONE active session at a time."
    - **Violation 1**: `termite/pkg/termite/lib/reranking/hugot.go` calls `hugot.NewSession()` directly in `NewPooledHugotReranker`, creating a standalone session.
    - **Violation 2**: `termite/pkg/termite/lib/rebel/rebel.go` calls `hugot.NewSessionOrUseExisting()` which can create a new session if the shared one is nil.
    - **Impact**: If `SessionManager` has an active session (e.g., for embeddings) and `Reranker` creates another one, two ONNX sessions exist simultaneously, causing undefined behavior or crashes.

## Phase 5: Real-World Probing (Completed)

### 1. Assumption Verification
We statically analyzed the codebase to verify A3.
- **Confirmed Violation**: Found multiple independent calls to `hugot.NewSession` bypassing `SessionManager`.
- **Files**:
    - `pkg/termite/lib/reranking/hugot.go` lines 154-158.
    - `pkg/termite/lib/rebel/rebel.go` lines 119-123.
    - `pkg/termite/lib/chunking/hugot.go` (and others).

### 2. Use-After-Free Verification
We inspected `SessionManager.Close` and backend implementations (`backend_onnx.go`, `backend_go.go`).
- **Confirmed**: `SessionManager.Close()` iterates and calls `session.Destroy()` immediately.
- **Confirmed**: Backends do not wrap the session in any reference-counting mechanism.
- **Result**: The race condition identified by TLA+ exists in the Go code. Any concurrent call to `Close()` during inference is unsafe.
- **Reproduction**: We implemented `TestSessionManager_CloseRace` which successfully triggered a panic by closing the session while it was in use, confirming the lack of synchronization.

*Note: The reproduction test confirmed the sequence of events (Close calling Destroy during usage).*

## Recommendations

### 1. Enforce Global Singleton
Since ONNX Runtime implementation forbids multiple sessions, `termite` must enforce a global singleton for the ONNX backend.
- **Action**: Modify `hugot` package to maintain a global `sync.Mutex` or reference-counted singleton for ONNX sessions, preventing `NewSession` from creating a second one if one exists.

### 2. Implement Reference Counting in SessionManager
To fix the Use-After-Free:
- **Action**: `SessionManager` should return a wrapper `SessionHandle` that increments a usage counter.
- **Action**: `Close()` must wait for the usage counter to drop to zero before calling `Destroy()`.

### 3. Refactor Direct Usage
- **Action**: Update `reranking`, `rebel`, and other packages to *require* a `SessionManager` or a shared session, removing the fallback to `hugot.NewSession()`.

## Confidence
**High**. The model checking found the UAF race, and the probing proved that the "Single Session" constraint is actively violated by the codebase structure.
