// +build ignore

// Bug 2: Multiple/Concurrent Close() Calls - Pattern Demonstration
//
// This file demonstrates the synchronization bug pattern WITHOUT requiring
// ONNX Runtime. Run with: go run pattern_demo.go
//
// The bug: Close() can be called multiple times, causing double-free.

package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// BUGGY VERSION - Matches current PooledHugotEmbedder behavior
// ============================================================================

type BuggyEmbedder struct {
	resource    *sharedResource
	destroyCount atomic.Int32
}

type sharedResource struct {
	data      []byte
	destroyed bool
}

func NewBuggyEmbedder() *BuggyEmbedder {
	return &BuggyEmbedder{
		resource: &sharedResource{data: make([]byte, 1024)},
	}
}

func (e *BuggyEmbedder) Close() error {
	// BUG: No protection against multiple calls!
	count := e.destroyCount.Add(1)
	fmt.Printf("  [BUGGY] Close() called (attempt #%d)\n", count)

	// Simulate expensive destruction that could race
	time.Sleep(10 * time.Millisecond)

	if e.resource.destroyed {
		// In real code: SIGSEGV or "panic during panic"
		return fmt.Errorf("DOUBLE FREE: resource already destroyed!")
	}

	e.resource.destroyed = true
	e.resource.data = nil
	fmt.Printf("  [BUGGY] Destroy completed (attempt #%d)\n", count)
	return nil
}

// ============================================================================
// FIXED VERSION - Uses sync.Once
// ============================================================================

type FixedEmbedder struct {
	resource  *sharedResource
	closeOnce sync.Once
	closeErr  error
}

func NewFixedEmbedder() *FixedEmbedder {
	return &FixedEmbedder{
		resource: &sharedResource{data: make([]byte, 1024)},
	}
}

func (e *FixedEmbedder) Close() error {
	// FIX: sync.Once ensures destroy only happens once
	e.closeOnce.Do(func() {
		fmt.Println("  [FIXED] Close() executing (only once)")
		time.Sleep(10 * time.Millisecond)
		e.resource.destroyed = true
		e.resource.data = nil
		fmt.Println("  [FIXED] Destroy completed")
	})
	return e.closeErr
}

// ============================================================================
// DEMONSTRATION
// ============================================================================

func main() {
	fmt.Println("=== Bug 2: Multiple/Concurrent Close() Calls ===")
	fmt.Println()

	// --- Buggy version ---
	fmt.Println("--- BUGGY VERSION (5 concurrent Close() calls) ---")
	buggy := NewBuggyEmbedder()

	var wg sync.WaitGroup
	errors := make([]error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errors[idx] = buggy.Close()
		}(i)
	}

	wg.Wait()

	fmt.Println("Results:")
	for i, err := range errors {
		if err != nil {
			fmt.Printf("  Goroutine %d: ERROR - %v\n", i, err)
		} else {
			fmt.Printf("  Goroutine %d: success (or lucky timing)\n", i)
		}
	}
	fmt.Printf("Total Close() calls: %d (should be 1)\n", buggy.destroyCount.Load())

	fmt.Println()

	// --- Fixed version ---
	fmt.Println("--- FIXED VERSION (5 concurrent Close() calls) ---")
	fixed := NewFixedEmbedder()

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			fixed.Close()
			fmt.Printf("  Goroutine %d: Close() returned\n", idx)
		}(i)
	}

	wg.Wait()
	fmt.Println("All goroutines completed safely - destroy ran exactly once")

	fmt.Println()
	fmt.Println("=== Pattern demonstration complete ===")
}
