package main

import (
	"fmt"
	"time"

	termite_hugot "github.com/antflydb/termite/pkg/termite/lib/hugot"
	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/options"
)

// MockBackend implements termite_hugot.Backend for testing
type MockBackend struct {
	createDelay time.Duration
}

func (m *MockBackend) Type() termite_hugot.BackendType {
	return "mock"
}

func (m *MockBackend) Name() string {
	return "Mock Backend"
}

func (m *MockBackend) Available() bool {
	return true
}

func (m *MockBackend) Priority() int {
	return 0
}

func (m *MockBackend) CreateSession(opts ...options.WithOption) (*hugot.Session, error) {
	// Return a bare session struct.
	return &hugot.Session{}, nil
}

func main() {
	fmt.Println("Starting Use-After-Free Reproduction...")

	// 1. Register Mock Backend
	mock := &MockBackend{}
	termite_hugot.RegisterBackend(mock)

	// Ensure we use the mock
	originalPriority := termite_hugot.GetPriority()
	defer termite_hugot.SetPriority(originalPriority)
	termite_hugot.SetPriority([]termite_hugot.BackendType{"mock"})

	sm := termite_hugot.NewSessionManager()

	// Control channels
	clientGotSession := make(chan struct{})
	clientDone := make(chan struct{})

	// 2. Spawn Client Routine
	go func() {
		session, err := sm.GetSession("mock")
		if err != nil {
			fmt.Printf("GetSession failed: %v\n", err)
			return
		}

		// Signal we have the pointer
		close(clientGotSession)

		fmt.Println("Client: Got session, simulating work...")
		time.Sleep(200 * time.Millisecond)

		// Access the session to show we still hold it
		_ = session
		fmt.Println("Client: Finished work (if you see this, race might have been missed or handled)")
		close(clientDone)
	}()

	// 3. Trigger Race
	<-clientGotSession // Wait for client to acquire session
	fmt.Println("Admin: Closing SessionManager...")

	// Invoke Close immediately.
	// In a correct system (RefCounting), Close() should block until clientDone.
	// In the buggy system, Close() will proceed immediately and call Destroy().
	err := sm.Close()
	if err != nil {
		// We expect a panic in Destroy() usually, but if it returns error, print it
		fmt.Printf("Close completed with error: %v\n", err)
	} else {
		fmt.Println("Admin: Close completed successfully (BUG: Should have blocked or panicked)")
	}

	// 4. Verification
	select {
	case <-clientDone:
		fmt.Println("Test finished.")
	case <-time.After(1 * time.Second):
		fmt.Println("Timeout waiting for client.")
	}
}
