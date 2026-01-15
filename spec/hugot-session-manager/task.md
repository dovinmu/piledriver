# Task: Implement and Verify Fix for Hugot Session Manager

## Phase 1: Implementation
- [x] Define `SessionHandle` and RefCounting logic in `session_manager.go` <!-- id: 0 -->
- [x] Update `SessionManager.GetSession` to return `SessionHandle` (Breaking Change) <!-- id: 1 -->
- [x] Refactor call sites in `termite/pkg/termite/lib` to use `SessionHandle`
	- [x] `reranking/hugot.go`
	- [x] `rebel/rebel.go`
	- [x] `seq2seq/hugot.go`
	- [x] `generation/hugot.go`
	- [x] `chunking/hugot.go`
	- [x] `classification/hugot.go`
	- [x] `embeddings/hugot.go`
	- [x] `ner/hugot.go`
	- [x] `gliner/gliner.go`
	- [x] `embeddings/clip_hugot.go` <!-- id: 2 -->
- [ ] Refactor direct `hugot.NewSession` usage (A3 violation) in `reranking`, `rebel`, etc. <!-- id: 3 -->

## Phase 2: Verification
- [ ] Update `race_repro.go` to match new API <!-- id: 4 -->
- [ ] Run `use-after-free/run.sh` (Expect: PASS, no panic) <!-- id: 5 -->
- [ ] Verify fix with `reproductions/e2e/verify.sh`
	- [x] Compilation succeeds
	- [ ] End-to-end test passes (Expect: PASS) <!-- id: 6 -->

## Phase 3: Completion
- [ ] Update walkthrough/report with fix details <!-- id: 7 -->
