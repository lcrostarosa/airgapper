// Package filelock provides file-based locking for concurrent access protection
package filelock

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// FileLock represents a file-based lock
type FileLock struct {
	path string
	file *os.File
}

// New creates a new file lock for the given path
// The lock file will be created at path + ".lock"
func New(path string) *FileLock {
	return &FileLock{
		path: path + ".lock",
	}
}

// NewForDir creates a new file lock for operations on a directory
// The lock file will be created at dir/.lock
func NewForDir(dir string) *FileLock {
	return &FileLock{
		path: filepath.Join(dir, ".lock"),
	}
}

// Lock acquires an exclusive lock on the file
// This is a blocking operation - it will wait until the lock is acquired
func (fl *FileLock) Lock() error {
	// Ensure parent directory exists
	dir := filepath.Dir(fl.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Open or create the lock file
	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}
	fl.file = f

	// Acquire exclusive lock (blocking)
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		fl.file = nil
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	return nil
}

// TryLock attempts to acquire an exclusive lock without blocking
// Returns true if the lock was acquired, false otherwise
func (fl *FileLock) TryLock() (bool, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(fl.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false, fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Open or create the lock file
	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return false, fmt.Errorf("failed to open lock file: %w", err)
	}
	fl.file = f

	// Try to acquire exclusive lock (non-blocking)
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		fl.file = nil
		if err == syscall.EWOULDBLOCK {
			return false, nil
		}
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	return true, nil
}

// LockWithTimeout attempts to acquire a lock with a timeout
func (fl *FileLock) LockWithTimeout(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	retryInterval := 10 * time.Millisecond

	for {
		acquired, err := fl.TryLock()
		if err != nil {
			return err
		}
		if acquired {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for lock on %s", fl.path)
		}

		time.Sleep(retryInterval)
		// Exponential backoff up to 100ms
		if retryInterval < 100*time.Millisecond {
			retryInterval *= 2
		}
	}
}

// Unlock releases the lock
func (fl *FileLock) Unlock() error {
	if fl.file == nil {
		return nil
	}

	err := syscall.Flock(int(fl.file.Fd()), syscall.LOCK_UN)
	closeErr := fl.file.Close()
	fl.file = nil

	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close lock file: %w", closeErr)
	}

	return nil
}

// WithLock executes a function while holding a lock
func (fl *FileLock) WithLock(fn func() error) error {
	if err := fl.Lock(); err != nil {
		return err
	}
	defer fl.Unlock()
	return fn()
}

// WithLockTimeout executes a function while holding a lock, with a timeout
func (fl *FileLock) WithLockTimeout(timeout time.Duration, fn func() error) error {
	if err := fl.LockWithTimeout(timeout); err != nil {
		return err
	}
	defer fl.Unlock()
	return fn()
}
