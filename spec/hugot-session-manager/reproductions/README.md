# Reproductions

This directory contains scripts and code to reproduce the bugs found in the `hugot-session-manager` hunt.

## 1. Use-After-Free Race Condition
**Directory**: `use-after-free/`

Demonstrates that `SessionManager.Close()` destroys sessions that are currently in use by concurrent clients.

### Running the Reproduction
```bash
cd use-after-free
./run.sh
```

**Expected Result**: The program should crash (panic) or exit with a non-zero code, indicating that `Destroy()` was called while the client was simulated to be using the session. A "successful" reproduction is a crash.

## 2. End-to-End Verification
**Directory**: `e2e/`

Verifies the full Termite embedding workflow. This script can be used to ensure the system is functional and to verify the fix later.

### Running the Verification
```bash
cd e2e
./verify.sh
```

**Prerequisites**:
- Go installed
- Termite dependencies (run `make download-omni-deps` in termite root if needed)
- Model `BAAI/bge-small-en-v1.5` present in `~/.termite/models/embedders/`

**Expected Result**: "Pass" for single and multiple batch requests.
