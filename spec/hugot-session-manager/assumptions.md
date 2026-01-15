# Boundary Assumptions

## A1: Backend Creation Stability
- **Interface**: `Backend.CreateSession()`
- **Assumption**: Returns either a valid `*hugot.Session` or a non-nil `error`. It never panics and never returns `nil, nil`.
- **Source**: Go conventions, inspection of `backend.go`.
- **Risk**: LOW
- **Verification idea**: Mock random failures and successes in a TLA+ model.

## A2: Session Destruction
- **Interface**: `Session.Destroy()`
- **Assumption**: It cleanly releases resources. Calling it multiple times on the same session object (if that were possible) or on a closed session might be unsafe, but `SessionManager` prevents this via its mutex and map.
- **Risk**: MEDIUM (Is `Destroy` itself idempotent? The manager assumes it owns the lifecycle).

## A3: Exclusive Ownership
- **Interface**: `hugot` package usage
- **Assumption**: No other component in the system creates ONNX sessions directly using `hugot.NewSession` or via the `Backend` directly, bypassing the `SessionManager`.
- **Source**: `SessionManager` docs ("IMPORTANT: ONNX Runtime allows only ONE active session at a time").
- **Risk**: HIGH. If another part of the app creates a session, `SessionManager`'s accounting is wrong.

## A4: Device Availability
- **Interface**: Hardware (GPU/TPU)
- **Assumption**: Devices requested (CUDA, TPU) are available if `Available()` returned true. We don't model hardware disappearing mid-operation.
- **Risk**: LOW

## A5: Backend Registry Stability
- **Interface**: `GetBackend()`
- **Assumption**: The backend registry is immutable after `init()` time. We assume new backends don't appear/disappear at runtime.
- **Source**: `backend.go` uses `sync.RWMutex` which implies it *could* change, but in practice it's likely static.
- **Risk**: LOW

## Critical Assumptions (HIGH risk)
- **A3 (Exclusive Ownership)**: The correctness of the "one active session" invariant relies entirely on the assumption that `SessionManager` is the *sole* gatekeeper.
