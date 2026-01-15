//go:build onnx && ORT

// Bug 1 Reproducer: Close() During Embed() → SIGSEGV
//
// Run from termite directory:
//   export ONNXRUNTIME_ROOT=$PWD/onnxruntime
//   export DYLD_LIBRARY_PATH=$ONNXRUNTIME_ROOT/darwin-arm64/lib:$DYLD_LIBRARY_PATH
//   go test -v -tags="onnx,ORT" -run TestBug1_CloseWhileEmbedding ./spec/hugot-usage/reproducers/bug1_close_during_embed/

package bug1

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

// TestBug1_CloseWhileEmbedding demonstrates the SIGSEGV crash.
// EXPECTED: This test CRASHES with SIGSEGV (unrecoverable).
func TestBug1_CloseWhileEmbedding(t *testing.T) {
	modelPath := findModel(t)
	logger := zap.NewNop()

	// Create embedder that OWNS its session (sessionShared=false)
	embedder, err := embeddings.NewPooledHugotEmbedder(modelPath, "model.onnx", 2, logger)
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// Start slow embed operation
	go func() {
		defer wg.Done()

		// Many texts = slow operation
		contents := make([][]ai.ContentPart, 50)
		for i := range contents {
			contents[i] = []ai.ContentPart{
				ai.TextContent{Text: fmt.Sprintf("Test sentence %d for bug reproduction", i)},
			}
		}

		ctx := context.Background()
		_, err := embedder.Embed(ctx, contents)
		if err != nil {
			t.Logf("Embed returned error: %v", err)
		}
	}()

	// Let inference start
	time.Sleep(10 * time.Millisecond)

	// Close while Embed is running - THIS CAUSES SIGSEGV
	t.Log("Calling Close() while Embed() is running...")
	embedder.Close()

	wg.Wait()

	// If we get here, something is wrong - should have crashed
	t.Log("WARNING: Did not crash - timing may have been lucky")
}
