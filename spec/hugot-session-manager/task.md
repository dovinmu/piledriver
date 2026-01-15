# Task: Real-World Probing of Hugot Session Manager

## Phase 1: Preparation
- [x] Check for existing direct session creation (Assumption A3 violation) <!-- id: 0 -->
- [x] Verify if `hugot.Session` can be instantiated for mocking <!-- id: 1 -->
- [x] Create `MockBackend` for testing <!-- id: 2 -->

## Phase 2: Reproduction
- [x] Implement `TestSessionManager_CloseRace` <!-- id: 3 -->
- [x] Run reproduction test and confirm failure <!-- id: 4 -->

## Phase 3: Reporting & Recommendations
- [x] Document findings in `investigation.md` (or update report) <!-- id: 5 -->
- [ ] Propose fix (RefCounting) <!-- id: 6 -->

## Phase 4: Reproduction Artifacts
- [x] Create `reproductions/use-after-free/race_repro.go` <!-- id: 7 -->
- [x] Create `reproductions/use-after-free/run.sh` <!-- id: 8 -->
- [x] Create `reproductions/e2e/verify.sh` <!-- id: 9 -->
- [ ] Create `reproductions/README.md` <!-- id: 10 -->
