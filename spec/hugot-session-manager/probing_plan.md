# Real-World Probing Plan: hugot-session-manager

## 1. Verify Assumption A3 (Exclusive Ownership) - **CONFIRMED**
**Goal**: Confirm that components other than `SessionManager` are creating `hugot.Session` objects directly, violating the "Single ONNX Session" constraint.

**Method**: 
- Search for `hugot.NewSession` and `hugot.NewSessionOrUseExisting` in the codebase.
- **Command**: `grep -r "NewSession" termite/pkg/termite/lib` (already ran, confirmed violations in `reranking`, `rebel`, `seq2seq`).

## 2. Reproduction of Use-After-Free (The Race)
**Goal**: Demonstrate that `SessionManager.Close()` can destroy a session while a client is using it.

**Method**:
Create a Go test `TestSessionManager_CloseRace` in `pkg/termite/lib/hugot/session_manager_test.go`.

### Test Setup
1. **Mock Backend**: Implement a `MockBackend` that satisfies the `Backend` interface.
   - `CreateSession`: Returns a dummy `*hugot.Session` (if possible to construct safely) or a minimal real session if we have a test model.
   - *Constraint*: `hugot.Session` is a struct. If we cannot create a safe dummy, we might need to rely on the fact that `Close` calls `Destroy` and just prove the *ordering* of events via logging/sleeping in the mock, even if we assume `Destroy` interacts with CGO.

2. **The Race**:
   - **Goroutine 1 (User)**:
     - Call `sm.GetSession(MockBackend)`.
     - "Use" the session (simulate work).
   - **Goroutine 2 (Admin)**:
     - Sleep briefly.
     - Call `sm.Close()`.

3. **Detection**:
   - Since we cannot easily intercept `session.Destroy()` (method on struct), we verify the *implication*:
   - If `SessionManager` returns the session pointer, and then `Close` is called, `Close` *does* call `Destroy`.
   - We will inspect `session_manager.go` logic to confirm `Close` iterates and calls `Destroy`.
   - The test will simply show that `Close` returns *while* the user is holding the pointer.

## 3. Verify Session.Destroy Behavior
**Goal**: Confirm `Destroy` is destructive.
- If we can look at `hugot` source (external), we would see if it frees C memory.
- Without source, we assume ONNX Runtime `release` behavior (standard).

## Execution Plan
1. [x] **A3 Check**: Run grep. (Done, violations found).
2. [ ] **Test Implementation**: Create `session_manager_race_test.go`.
   - Define `MockBackend`.
   - Register it with `RegisterBackend`.
   - Run the race.
3. [ ] **Report**: Summarize that the race is architecturally guaranteed because `SessionManager` returns the raw pointer and doesn't track usage.

## Recommendations (Preview)
- **RefCount**: `SessionManager` must return a **Handle**, not the raw session. The Handle increments a refcount on creation and decrements on Close. `SessionManager.Close` waits for refcounts to drain.
- **Singleton**: Enforce singleton usage for ONNX backend.
