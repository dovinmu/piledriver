# Bug 1: Close() During Embed() Race

## Summary
Calling `Close()` while `Embed()` is running destroys the ONNX session mid-inference, causing a segmentation fault.

## Verbose Description
`PooledHugotEmbedder.Close()` has no synchronization with `Embed()`. When `Close()` is called:
1. It immediately calls `session.Destroy()`
2. This frees the ONNX Runtime session memory
3. Any in-flight `Embed()` calls are still holding references to pipelines that use this session
4. When ONNX Runtime tries to use the freed session → SIGSEGV

The crash occurs in the CGO layer:
```
SIGSEGV: segmentation violation
signal arrived during cgo execution
github.com/yalue/onnxruntime_go._Cfunc_RunOrtSessionWithOptions(...)
```

## Root Cause
```go
// hugot.go:377-385
func (p *PooledHugotEmbedder) Close() error {
    if p.session != nil && !p.sessionShared {
        return p.session.Destroy()  // ← Immediate, no waiting!
    }
    return nil
}
```

No `closed` flag, no WaitGroup, no mutex - just immediate destruction.

## Files
- `bug1_test.go` - Reproducer (crashes without fix)
- `bug1_fixed_test.go` - Same test with fix applied (passes)

## Run
```bash
# From termite directory with ONNX env vars set:
go test -v -race -tags="onnx,ORT" -run TestBug1 ./spec/hugot-usage/reproducers/bug1_close_during_embed/
```
