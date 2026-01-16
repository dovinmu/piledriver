package store

import (
	"sync"
	"time"
)

// LineType represents the type of a diff line
type LineType int

const (
	LineContext LineType = iota
	LineAdded
	LineRemoved
)

// DiffLine represents a single line in a diff
type DiffLine struct {
	Type    LineType
	Content string
	OldLine int // line number in old file (0 if added)
	NewLine int // line number in new file (0 if removed)
}

// DiffHunk represents a contiguous block of changes
type DiffHunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

// Changeset represents a single file change event with its diff
type Changeset struct {
	FilePath   string
	Timestamp  time.Time
	OldContent []byte // nil for new files
	NewContent []byte
	Hunks      []DiffHunk
	IsBinary   bool
	IsDeleted  bool
	IsNew      bool
}

// Store is a thread-safe circular buffer of changesets
type Store struct {
	mu         sync.RWMutex
	changesets []Changeset
	capacity   int
	size       int
	head       int // index of newest item

	// Track last known content per file for diffing
	fileContents map[string][]byte
}

// New creates a new Store with the given capacity
func New(capacity int) *Store {
	if capacity < 1 {
		capacity = 100
	}
	return &Store{
		changesets:   make([]Changeset, capacity),
		capacity:     capacity,
		size:         0,
		head:         -1,
		fileContents: make(map[string][]byte),
	}
}

// Push adds a new changeset to the store
func (s *Store) Push(cs Changeset) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.head = (s.head + 1) % s.capacity
	s.changesets[s.head] = cs

	if s.size < s.capacity {
		s.size++
	}

	// Update file contents cache
	if cs.IsDeleted {
		delete(s.fileContents, cs.FilePath)
	} else {
		s.fileContents[cs.FilePath] = cs.NewContent
	}
}

// Size returns the number of changesets in the store
func (s *Store) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.size
}

// Capacity returns the maximum capacity of the store
func (s *Store) Capacity() int {
	return s.capacity
}

// Get returns the changeset at the given index (0 = oldest, size-1 = newest)
func (s *Store) Get(index int) (Changeset, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if index < 0 || index >= s.size {
		return Changeset{}, false
	}

	// Calculate actual index in circular buffer
	// oldest is at (head - size + 1 + capacity) % capacity
	oldest := (s.head - s.size + 1 + s.capacity) % s.capacity
	actualIndex := (oldest + index) % s.capacity

	return s.changesets[actualIndex], true
}

// GetNewest returns the most recent changeset
func (s *Store) GetNewest() (Changeset, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.size == 0 {
		return Changeset{}, false
	}
	return s.changesets[s.head], true
}

// GetOldContent returns the last known content for a file (for diffing)
func (s *Store) GetOldContent(filePath string) []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fileContents[filePath]
}
