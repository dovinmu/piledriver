package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dovinmu/piledriver/pilemonkey/store"
)

// TestRapidMultiFileEdits exposes the global-debounce bug: when many
// distinct files change within the debounce window, the single shared
// timer keeps getting reset and only the last file (if any) is ever
// reported. With a per-file debounce, every file should be reported.
func TestRapidMultiFileEdits(t *testing.T) {
	tmp := t.TempDir()

	s := store.New(100)
	w, err := New(tmp, s, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	received := make(chan string, 64)
	w.OnChangeset = func(cs store.Changeset) {
		received <- cs.FilePath
	}

	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	// Give fsnotify a moment to register the watch.
	time.Sleep(20 * time.Millisecond)

	const n = 20
	want := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		p := filepath.Join(tmp, fmt.Sprintf("f%02d.txt", i))
		want[p] = true
		if err := os.WriteFile(p, []byte(fmt.Sprintf("content %d\n", i)), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		// Tight loop: writes are well within the 100ms debounce window.
	}

	// Wait well past the debounce window for everything to flush.
	deadline := time.After(2 * time.Second)
	got := make(map[string]bool, n)
	for len(got) < n {
		select {
		case p := <-received:
			got[p] = true
		case <-deadline:
			missing := make([]string, 0, n-len(got))
			for p := range want {
				if !got[p] {
					missing = append(missing, p)
				}
			}
			t.Fatalf("only %d/%d files reported; missing %d (e.g. %v)",
				len(got), n, len(missing), missing[:min(3, len(missing))])
		}
	}

	for p := range want {
		if !got[p] {
			t.Errorf("missing changeset for %s", p)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
