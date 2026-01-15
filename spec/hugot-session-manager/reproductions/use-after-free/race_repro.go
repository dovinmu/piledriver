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
		handle, err := sm.GetSession("mock")
		if err != nil {
			fmt.Printf("GetSession failed: %v\n", err)
			return
		}

		// Signal we have the pointer
		close(clientGotSession)

		fmt.Println("Client: Got session, simulating work...")
		time.Sleep(200 * time.Millisecond)

		// Access the session to show we still hold it
		_ = handle.Session
		
		fmt.Println("Client: Work done, releasing handle...")
		handle.Close()
		
		close(clientDone)
	}()

	// 3. Trigger Race
	<-clientGotSession // Wait for client to acquire session
	fmt.Println("Admin: Closing SessionManager...")

	// Invoke Close immediately.
	// In a correct system (RefCounting), Close() should block until clientDone.
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered from expected mock session panic: %v\n", r)
			}
		}()
		err := sm.Close()
		if err != nil {
			fmt.Printf("Close completed with error: %v\n", err)
		}
	}()
	
	closeData := time.Now()
	fmt.Println("Admin: Close returned (or recovered).")

	// 4. Verification
	select {
	case <-clientDone:
		fmt.Println("SUCCESS: Client finished before Close returned.")
	default:
		fmt.Println("FAILURE: Close returned before client finished!")
	}
	
	_ = closeData
}
