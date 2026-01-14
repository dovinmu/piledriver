// +build ignore

// Bug 1: Close() During Active Operation - Pattern Demonstration
//
// This file demonstrates the synchronization bug pattern WITHOUT requiring
// ONNX Runtime. Run with: go run pattern_demo.go
//
// The bug: Close() destroys shared resources while operations are in-flight.

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
	resource *sharedResource
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

func (e *BuggyEmbedder) Embed() error {
	// Simulate work using the shared resource
	for i := 0; i < 100; i++ {
		if e.resource.destroyed {
			return fmt.Errorf("resource destroyed mid-operation!")
		}
		// Simulate ONNX inference touching the resource
		_ = e.resource.data[i%len(e.resource.data)]
		time.Sleep(time.Millisecond)
	}
	return nil
}

func (e *BuggyEmbedder) Close() error {
	// BUG: No synchronization! Just destroy immediately.
	e.resource.destroyed = true
	e.resource.data = nil // Simulate session.Destroy() freeing memory
	return nil
}

// ============================================================================
// FIXED VERSION - Proper synchronization
// ============================================================================

type FixedEmbedder struct {
	resource *sharedResource
	closed   atomic.Bool
	wg       sync.WaitGroup
}

func NewFixedEmbedder() *FixedEmbedder {
	return &FixedEmbedder{
		resource: &sharedResource{data: make([]byte, 1024)},
	}
}

func (e *FixedEmbedder) Embed() error {
	// FIX: Check closed flag before starting
	if e.closed.Load() {
		return fmt.Errorf("embedder is closed")
	}

	// FIX: Track in-flight operation
	e.wg.Add(1)
	defer e.wg.Done()

	// Double-check after registering (handles race with Close)
	if e.closed.Load() {
		return fmt.Errorf("embedder is closed")
	}

	// Simulate work
	for i := 0; i < 100; i++ {
		_ = e.resource.data[i%len(e.resource.data)]
		time.Sleep(time.Millisecond)
	}
	return nil
}

func (e *FixedEmbedder) Close() error {
	// FIX: Set closed flag first (prevents new operations)
	if !e.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	// FIX: Wait for all in-flight operations to complete
	e.wg.Wait()

	// Now safe to destroy
	e.resource.destroyed = true
	e.resource.data = nil
	return nil
}

// ============================================================================
// DEMONSTRATION
// ============================================================================

func main() {
	fmt.Println("=== Bug 1: Close() During Active Operation ===")
	fmt.Println()

	// --- Buggy version ---
	fmt.Println("--- BUGGY VERSION ---")
	buggy := NewBuggyEmbedder()

	var buggyErr error
	go func() {
		buggyErr = buggy.Embed()
	}()

	time.Sleep(10 * time.Millisecond) // Let Embed() start
	buggy.Close()
	time.Sleep(150 * time.Millisecond) // Let Embed() finish/crash

	if buggyErr != nil {
		fmt.Printf("BUGGY: Embed() failed: %v\n", buggyErr)
		fmt.Println("(In real code, this would be SIGSEGV)")
	} else {
		fmt.Println("BUGGY: Got lucky with timing - try again")
	}

	fmt.Println()

	// --- Fixed version ---
	fmt.Println("--- FIXED VERSION ---")
	fixed := NewFixedEmbedder()

	var fixedErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		fixedErr = fixed.Embed()
	}()

	time.Sleep(10 * time.Millisecond) // Let Embed() start
	fmt.Println("Calling Close() while Embed() is running...")
	closeStart := time.Now()
	fixed.Close()
	fmt.Printf("Close() returned after %v (waited for Embed to finish)\n", time.Since(closeStart))

	wg.Wait()

	if fixedErr != nil {
		fmt.Printf("FIXED: Embed() returned error: %v\n", fixedErr)
	} else {
		fmt.Println("FIXED: Embed() completed successfully before Close() destroyed resources")
	}

	fmt.Println()
	fmt.Println("=== Pattern demonstration complete ===")
}
