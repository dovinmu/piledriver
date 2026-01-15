# Implementation Plan - Fix Use-After-Free & Exclusive Ownership

## Goal Description
Fix the confirmed Use-After-Free race condition in `SessionManager` by implementing Reference Counting. Also enforce the singleton constraint for ONNX sessions by routing all session creation through `SessionManager`.

## User Review Required
> [!WARNING]
> **API Breaking Change**: `SessionManager.GetSession` will now return `*SessionHandle` instead of `*hugot.Session`. Callers must call `handle.Close()` (or `defer handle.Close()`) to release the reference. This requires updating all call sites.

## Proposed Changes

### Logic: Reference Counting
- Introduce `SessionHandle` struct wrapping `*hugot.Session` and a `release` function.
- `SessionManager` tracks active usage count (implied by number of open handles).
    - Actually, `SessionManager.Close()` needs to wait.
    - We can use a `sync.WaitGroup` per session? OR just a global WaitGroup if we close everything at once?
    - Problem: `SessionManager` manages multiple sessions.
    - Solution: Wrap each `hugot.Session` in a `managedSession` struct that contains the `WaitGroup` or refcount.
- `SessionManager.Close()`:
    1. Lock manager.
    2. Mark as closed (prevent new handles).
    3. Iterate all `managedSession`s.
    4. For each, wait for its refcount/WG to hit zero.
    5. Destroy the underlying session.

### Logic: Singleton Enforcement
- Refactor `reranking/hugot.go`, `rebel/rebel.go`, `seq2seq/hugot.go` to accept a `*hugot.SessionManager` or valid `SessionHandle` instead of creating their own sessions.
- This might be complex if dependency injection isn't set up.
- **Pragmatic Fix**: For now, update them to use `SessionManager` singleton or passed instance if available.

### Files

#### [MODIFY] [session_manager.go](file:///Users/rowan/Dropbox/programmery/piledriver-2/termite/pkg/termite/lib/hugot/session_manager.go)
- Add `managedSession` struct:
  ```go
  type managedSession struct {
      session *hugot.Session
      wg      sync.WaitGroup
      mu      sync.Mutex // protects access if needed, though SessionManager handles map lock
  }
  ```
- Add `SessionHandle` struct:
  ```go
  type SessionHandle struct {
      session *hugot.Session
      release func()
      closed  bool
      mu      sync.Mutex
  }
  ```
- Update `GetSession` to return `*SessionHandle`.

#### [MODIFY] [session.go](file:///Users/rowan/Dropbox/programmery/piledriver-2/termite/pkg/termite/lib/hugot/session.go)
- Add helpers if needed.

#### [MODIFY] consumer files (grep results)
- Update to use `SessionHandle` and `defer handle.Close()`.

## Verification Plan

### Automated Tests
- Update `race_repro.go` to use the new API.
- Run `race_repro.go`. **Expected**: It should NOT panic. It should print "Admin: Close completed..." AFTER the client finishes (or block until timeout if we set it up that way).
- Run `e2e/verify.sh`. **Expected**: PASS.
