package state

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// LockTimeout is the default timeout for acquiring a lock
const LockTimeout = 5 * time.Second

// FileLock represents a file-based lock
type FileLock struct {
	path string
	file *os.File
}

// NewFileLock creates a new file lock
func NewFileLock(path string) *FileLock {
	return &FileLock{
		path: path + ".lock",
	}
}

// Lock acquires an exclusive lock, blocking until available or timeout
func (l *FileLock) Lock() error {
	return l.LockWithTimeout(LockTimeout)
}

// LockWithTimeout acquires a lock with a specific timeout
func (l *FileLock) LockWithTimeout(timeout time.Duration) error {
	var err error
	l.file, err = os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire lock with timeout
	done := make(chan error, 1)
	go func() {
		done <- syscall.Flock(int(l.file.Fd()), syscall.LOCK_EX)
	}()

	select {
	case err := <-done:
		if err != nil {
			l.file.Close()
			return fmt.Errorf("failed to acquire lock: %w", err)
		}
		return nil
	case <-time.After(timeout):
		l.file.Close()
		return fmt.Errorf("timeout waiting for lock on %s", l.path)
	}
}

// TryLock attempts to acquire the lock without blocking
// Returns true if lock was acquired, false otherwise
func (l *FileLock) TryLock() (bool, error) {
	var err error
	l.file, err = os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return false, fmt.Errorf("failed to open lock file: %w", err)
	}

	err = syscall.Flock(int(l.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		l.file.Close()
		if err == syscall.EWOULDBLOCK {
			return false, nil
		}
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	return true, nil
}

// Unlock releases the lock
func (l *FileLock) Unlock() error {
	if l.file == nil {
		return nil
	}

	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil

	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close lock file: %w", closeErr)
	}

	return nil
}

// WithLock executes a function while holding an exclusive lock
func WithLock(path string, fn func() error) error {
	lock := NewFileLock(path)
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()
	return fn()
}

// WithLockTimeout executes a function while holding an exclusive lock with timeout
func WithLockTimeout(path string, timeout time.Duration, fn func() error) error {
	lock := NewFileLock(path)
	if err := lock.LockWithTimeout(timeout); err != nil {
		return err
	}
	defer lock.Unlock()
	return fn()
}

// LoadStateWithLock reads state while holding the lock
func LoadStateWithLock(sessionDir string) (*SessionState, error) {
	var state *SessionState
	err := WithLock(StateFilePath(sessionDir), func() error {
		var loadErr error
		state, loadErr = LoadState(sessionDir)
		return loadErr
	})
	return state, err
}

// SaveStateWithLock writes state while holding the lock
func SaveStateWithLock(sessionDir string, state *SessionState) error {
	return WithLock(StateFilePath(sessionDir), func() error {
		return SaveState(sessionDir, state)
	})
}

// UpdateStateWithLock reads, modifies, and writes state atomically
func UpdateStateWithLock(sessionDir string, updateFn func(*SessionState) error) error {
	return WithLock(StateFilePath(sessionDir), func() error {
		state, err := LoadState(sessionDir)
		if err != nil {
			return err
		}
		if state == nil {
			return fmt.Errorf("state file not found")
		}
		if err := updateFn(state); err != nil {
			return err
		}
		return SaveState(sessionDir, state)
	})
}
