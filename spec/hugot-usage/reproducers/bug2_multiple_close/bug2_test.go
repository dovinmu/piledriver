//go:build onnx && ORT

// Bug 2 Reproducer: Multiple/Concurrent Close() → Panic
//
// Run from termite directory:
//   export ONNXRUNTIME_ROOT=$PWD/onnxruntime
//   export DYLD_LIBRARY_PATH=$ONNXRUNTIME_ROOT/darwin-arm64/lib:$DYLD_LIBRARY_PATH
//   go test -v -tags="onnx,ORT" -run TestBug2_MultipleClose ./spec/hugot-usage/reproducers/bug2_multiple_close/

package bug2

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/antflydb/antfly-go/libaf/ai"
	"github.com/antflydb/termite/pkg/termite/lib/embeddings"
	"go.uber.org/zap"
)

func findModel(t *testing.T) string {
	home := os.Getenv("HOME")
	paths := []string{
		filepath.Join(home, ".termite/models/embedders/BAAI/bge-small-en-v1.5"),
		filepath.Join(home, ".termite/models/embedders/bge-small-en-v1.5"),
	}
	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(p, "model.onnx")); err == nil {
			return p
		}
	}
	t.Skip("Model not found")
	return ""
}

// TestBug2_MultipleClose demonstrates the "panic during panic" crash.
// EXPECTED: This test CRASHES with fatal error (unrecoverable).
func TestBug2_MultipleClose(t *testing.T) {
	modelPath := findModel(t)
	logger := zap.NewNop()

	// Create embedder that OWNS its session
	embedder, err := embeddings.NewPooledHugotEmbedder(modelPath, "model.onnx", 2, logger)
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	// Do one successful embed first
	ctx := context.Background()
	contents := [][]ai.ContentPart{
		{ai.TextContent{Text: "test"}},
	}
	_, err = embedder.Embed(ctx, contents)
	if err != nil {
		t.Fatalf("Initial embed failed: %v", err)
	}

	// Call Close() from 5 goroutines concurrently - THIS CAUSES PANIC
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			t.Logf("Goroutine %d calling Close()...", idx)
			embedder.Close()
			t.Logf("Goroutine %d Close() returned", idx)
		}(i)
	}

	wg.Wait()

	// If we get here, something is wrong - should have crashed
	t.Log("WARNING: Did not crash - timing may have been lucky")
}
