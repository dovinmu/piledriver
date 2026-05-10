package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/dovinmu/piledriver/pilemonkey/differ"
	"github.com/dovinmu/piledriver/pilemonkey/store"
)

// Default ignore patterns
var defaultIgnorePatterns = []string{
	".git",
	"node_modules",
	"__pycache__",
	".pytest_cache",
	"venv",
	".venv",
	"vendor",
	"target",
	"build",
	"dist",
	".idea",
	".vscode",
	"*.pyc",
	"*.pyo",
	"*.class",
	"*.o",
	"*.so",
	"*.dylib",
	"*.exe",
	"*.dll",
	"*.swp",
	"*.swo",
	"*~",
	".DS_Store",
}

// Watcher watches a directory for file changes
type Watcher struct {
	fsWatcher      *fsnotify.Watcher
	store          *store.Store
	rootDir        string
	ignorePatterns []string
	debounceTime   time.Duration

	// Debouncing: one timer per path so concurrent edits to different
	// files don't cancel each other.
	mu     sync.Mutex
	timers map[string]*time.Timer

	// Callback for new changesets
	OnChangeset func(store.Changeset)
}

// New creates a new Watcher
func New(rootDir string, s *store.Store, extraIgnorePatterns []string) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Combine default and extra ignore patterns
	patterns := append([]string{}, defaultIgnorePatterns...)
	patterns = append(patterns, extraIgnorePatterns...)

	w := &Watcher{
		fsWatcher:      fsWatcher,
		store:          s,
		rootDir:        rootDir,
		ignorePatterns: patterns,
		debounceTime:   100 * time.Millisecond,
		timers:         make(map[string]*time.Timer),
	}

	return w, nil
}

// Start begins watching the directory
func (w *Watcher) Start() error {
	// Add root directory and all subdirectories
	err := filepath.Walk(w.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			if w.shouldIgnore(path) {
				return filepath.SkipDir
			}
			return w.fsWatcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Start event loop in goroutine
	go w.eventLoop()

	return nil
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	return w.fsWatcher.Close()
}

func (w *Watcher) eventLoop() {
	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case _, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			// Log errors but continue
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Only care about writes and creates
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) && !event.Has(fsnotify.Remove) {
		return
	}

	path := event.Name

	// Ignore patterns
	if w.shouldIgnore(path) {
		return
	}

	// For directories, add to watcher
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		if event.Has(fsnotify.Create) {
			w.fsWatcher.Add(path)
		}
		return
	}

	// Debounce file changes
	w.debounce(path, event.Has(fsnotify.Remove))
}

func (w *Watcher) debounce(path string, isDelete bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if t, ok := w.timers[path]; ok {
		t.Stop()
	}

	w.timers[path] = time.AfterFunc(w.debounceTime, func() {
		w.mu.Lock()
		delete(w.timers, path)
		w.mu.Unlock()
		w.processChange(path, isDelete)
	})
}

func (w *Watcher) processChange(path string, isDelete bool) {
	cs := store.Changeset{
		FilePath:  path,
		Timestamp: time.Now(),
		IsDeleted: isDelete,
	}

	if isDelete {
		// File was deleted
		cs.OldContent = w.store.GetOldContent(path)
		cs.NewContent = nil
	} else {
		// Read new content
		content, err := os.ReadFile(path)
		if err != nil {
			return // Skip if can't read
		}

		// Check if binary
		if differ.IsBinary(content) {
			cs.IsBinary = true
			cs.NewContent = content
		} else {
			cs.NewContent = content
			cs.OldContent = w.store.GetOldContent(path)

			if cs.OldContent == nil {
				cs.IsNew = true
			}

			// Compute diff
			hunks, err := differ.ComputeDiff(cs.OldContent, cs.NewContent)
			if err == nil {
				cs.Hunks = hunks
			}

			// Skip if no actual changes
			if len(cs.Hunks) == 0 && !cs.IsNew && !cs.IsDeleted {
				return
			}
		}
	}

	// Store changeset
	w.store.Push(cs)

	// Notify callback
	if w.OnChangeset != nil {
		w.OnChangeset(cs)
	}
}

func (w *Watcher) shouldIgnore(path string) bool {
	// Get relative path from root
	relPath, err := filepath.Rel(w.rootDir, path)
	if err != nil {
		relPath = path
	}

	// Check each path component
	parts := strings.Split(relPath, string(filepath.Separator))
	for _, part := range parts {
		for _, pattern := range w.ignorePatterns {
			// Direct match
			if part == pattern {
				return true
			}
			// Glob match
			matched, _ := filepath.Match(pattern, part)
			if matched {
				return true
			}
		}
	}

	return false
}
