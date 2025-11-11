package util

import (
	"os"
	"syscall"
)

// FLockType represents the type of file lock to apply
type FLockType int

const (
	// LockShared is a shared lock (read lock)
	LockShared FLockType = syscall.LOCK_SH
	// LockExclusive is an exclusive lock (write lock)
	LockExclusive FLockType = syscall.LOCK_EX
	// LockUnlock unlocks a file
	LockUnlock FLockType = syscall.LOCK_UN
)

// FileLock represents a file lock
type FileLock struct {
	file   *os.File
	path   string
	locked bool
}

// NewFileLock creates a new FileLock instance
func NewFileLock(path string) *FileLock {
	return &FileLock{
		path: path,
	}
}

// Lock attempts to acquire a file lock with the specified type and blocking behavior
func (fl *FileLock) Lock(lockType FLockType, blocking bool) error {
	if fl.locked {
		return nil // Already locked
	}

	flags := os.O_RDWR
	if lockType == LockShared {
		flags |= os.O_CREATE
	} else {
		// For exclusive locks, we want to ensure the file exists
		flags |= os.O_CREATE
	}

	file, err := os.OpenFile(fl.path, flags, 0644)
	if err != nil {
		return err
	}

	fl.file = file

	// Apply the lock
	if blocking {
		err = syscall.Flock(int(file.Fd()), int(lockType))
	} else {
		err = syscall.Flock(int(file.Fd()), int(lockType)|syscall.LOCK_NB)
	}

	if err != nil {
		_ = file.Close()
		fl.file = nil
		return err
	}

	fl.locked = true
	return nil
}

// TryLock attempts to acquire a lock without blocking
func (fl *FileLock) TryLock(lockType FLockType) error {
	return fl.Lock(lockType, false)
}

// Unlock releases the file lock
func (fl *FileLock) Unlock() error {
	if !fl.locked || fl.file == nil {
		return nil
	}

	err := syscall.Flock(int(fl.file.Fd()), int(LockUnlock))
	if err != nil {
		return err
	}

	// Close the file
	if closeErr := fl.file.Close(); closeErr != nil {
		return closeErr
	}

	fl.file = nil
	fl.locked = false
	return nil
}

// IsLocked returns true if the lock is currently held
func (fl *FileLock) IsLocked() bool {
	return fl.locked
}

// Close releases the lock and closes the file
func (fl *FileLock) Close() error {
	if fl.locked {
		if err := fl.Unlock(); err != nil {
			return err
		}
	}
	return nil
}

// WithLock executes a function while holding an exclusive lock on the specified file
func WithLock(path string, fn func() error) error {
	lock := NewFileLock(path)
	defer func() { _ = lock.Close() }()

	if err := lock.Lock(LockExclusive, true); err != nil {
		return err
	}

	return fn()
}

// WithSharedLock executes a function while holding a shared lock on the specified file
func WithSharedLock(path string, fn func() error) error {
	lock := NewFileLock(path)
	defer func() { _ = lock.Close() }()

	if err := lock.Lock(LockShared, true); err != nil {
		return err
	}

	return fn()
}

// TryWithLock attempts to execute a function with an exclusive lock, without blocking
// Returns an error if the lock cannot be acquired immediately
func TryWithLock(path string, fn func() error) error {
	lock := NewFileLock(path)
	defer func() { _ = lock.Close() }()

	if err := lock.TryLock(LockExclusive); err != nil {
		return err
	}

	return fn()
}

// TryWithSharedLock attempts to execute a function with a shared lock, without blocking
// Returns an error if the lock cannot be acquired immediately
func TryWithSharedLock(path string, fn func() error) error {
	lock := NewFileLock(path)
	defer func() { _ = lock.Close() }()

	if err := lock.TryLock(LockShared); err != nil {
		return err
	}

	return fn()
}

// IsFileLocked checks if a file is currently locked
func IsFileLocked(path string) (bool, error) {
	lock := NewFileLock(path)
	defer func() { _ = lock.Close() }()

	err := lock.TryLock(LockExclusive)
	if err != nil {
		// If we can't get an exclusive lock, the file might be locked
		// Try shared lock to see if we can get any access
		if err2 := lock.TryLock(LockShared); err2 != nil {
			// Can't get any lock, assume it's locked
			return true, nil
		}
		// Got shared lock, so not exclusively locked
		_ = lock.Unlock()
		return false, nil
	}

	// Got exclusive lock, so it wasn't locked before
	_ = lock.Unlock()
	return false, nil
}
