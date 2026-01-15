# Piledriver Boundary Definition

## Hunt
hugot-session-manager

## Suspect
The `SessionManager` in `termite/pkg/termite/lib/hugot/session_manager.go` is responsible for managing ML backend sessions. It must enforce constraints like "only one ONNX session at a time" while handling concurrent requests for various models and backends (ONNX, XLA, Go). We suspect race conditions or logic errors in how sessions are reused, created, or destroyed under concurrent load.

## Inside (formally modeled)
- `SessionManager` state (map of active sessions, closed flag, priority list).
- Logic for `GetSession`, `GetSessionWithFallback`, and `GetSessionForModel`.
- Concurrency: Multiple clients calling these methods simultaneously.
- The constraint logic: re-using existing sessions vs creating new ones.
- Error handling paths (backend unavailable, session creation failure).

## Outside (assumptions)
- **Implementations of `Backend` (ONNX, XLA, Go)**: We assume `CreateSession` either succeeds or returns a predictable error. We do not model the internal state of the actual ML libraries.
- **Physical Devices**: We assume device availability is static during the test (or changes predictably if we model ephemeral failures).
- **`hugot` Library Internals**: The `hugot.Session` object itself is treated as a black box handle; we only care about its existence and lifecycle state (created/destroyed).

## Interface Points

| Boundary Crossing | Direction | Assumption |
|-------------------|-----------|------------|
| `Backend.CreateSession()` | IN→OUT | Returns a valid Session object or error. Does not block indefinitely. |
| `Session.Destroy()` | IN→OUT | Cleanly releases resources. Idempotent or safe to call once. |
| `GetBackend()` / registry | IN→OUT | Global registry is stable or thread-safe (it uses a mutex). |

## What This Scoping EXCLUDES
- The actual inference performance or correctness of the ML models.
- File system I/O for loading models (`CreateSession` arguments are abstract).
- Memory management details (GC, etc.), other than "Session Created" / "Session Destroyed".
