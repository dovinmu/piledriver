# Implementation Plan - Use-After-Free Reproduction

## Goal Description
Reproduce the "Use-After-Free" vulnerability in `SessionManager`. The vulnerability is that `SessionManager.Close()` destroys sessions that are currently in use by concurrent clients, because it does not track active usage (reference counting).

## Proposed Changes
### `termite/pkg/termite/lib/hugot`

#### [NEW] [race_test.go](file:///Users/rowan/Dropbox/programmery/piledriver-2/termite/pkg/termite/lib/hugot/race_test.go)
- Implement `MockBackend` struct that satisfies `Backend` interface.
- Implement `TestSessionManager_CloseRace`:
    - Setup `SessionManager` with `MockBackend`.
    - Spawn "User" goroutine: GetSession, then Sleep (simulate inference).
    - Main thread: Sleep briefly (let User start), then `Close()`.
    - Assert: `Close()` returns *before* User finishes.
    - Demonstrates that `Destroy` (called by Close) happened during usage.

## Verification Plan

### Automated Tests
- Run the new test:
  ```bash
  go test -v -race termite/pkg/termite/lib/hugot/race_test.go
  ```
- **Expected Result**: PASS (confirming the race exists/timing overlap happens).
- **Note**: This test "passes" if it successfully reproduces the *ordering* issue. It does not fail the build. It is a proof-of-concept.

### Cleanup
- Delete `termite/pkg/termite/lib/hugot/check_struct_test.go`
