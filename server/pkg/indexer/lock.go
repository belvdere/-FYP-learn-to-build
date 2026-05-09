package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const lockFileName = ".fyp/index.lock"

// IndexLock provides file-based locking for indexing operations.
type IndexLock struct {
	lockPath string
	lockFile *os.File
}

// AcquireLock attempts to acquire an exclusive lock for indexing.
// Returns an error if a lock is already held by another process.
func AcquireLock(workspaceRoot string) (*IndexLock, error) {
	lockPath := filepath.Join(workspaceRoot, lockFileName)

	// Ensure .fyp directory exists
	fypDir := filepath.Dir(lockPath)
	if err := os.MkdirAll(fypDir, 0755); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}

	// Try to open/create lock file
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	err = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		lockFile.Close()

		// Check if lock file has stale info
		if existingPID, err := readLockPID(lockPath); err == nil {
			if !isProcessRunning(existingPID) {
				// Stale lock - remove and retry
				os.Remove(lockPath)
				return AcquireLock(workspaceRoot)
			}
			return nil, fmt.Errorf("indexing already in progress (PID: %d)\nPlease wait for the current indexing operation to complete", existingPID)
		}

		return nil, fmt.Errorf("indexing already in progress\nPlease wait for the current indexing operation to complete")
	}

	// Write current process PID to lock file
	pid := os.Getpid()
	lockFile.Truncate(0)
	lockFile.Seek(0, 0)
	fmt.Fprintf(lockFile, "%d\n%s\n", pid, time.Now().Format(time.RFC3339))
	lockFile.Sync()

	return &IndexLock{
		lockPath: lockPath,
		lockFile: lockFile,
	}, nil
}

// Release releases the lock and removes the lock file.
func (l *IndexLock) Release() error {
	if l.lockFile == nil {
		return nil
	}

	// Release flock
	syscall.Flock(int(l.lockFile.Fd()), syscall.LOCK_UN)

	// Close file
	l.lockFile.Close()

	// Remove lock file
	os.Remove(l.lockPath)

	l.lockFile = nil
	return nil
}

// readLockPID reads the PID from an existing lock file.
func readLockPID(lockPath string) (int, error) {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return 0, err
	}

	var pid int
	_, err = fmt.Sscanf(string(data), "%d", &pid)
	if err != nil {
		return 0, err
	}

	return pid, nil
}

// isProcessRunning checks if a process with the given PID is running.
func isProcessRunning(pid int) bool {
	// Send signal 0 to check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, need to send signal to check
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
