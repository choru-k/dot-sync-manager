package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

// ErrCommandLockHeld is returned when a command-level lock is already held.
var ErrCommandLockHeld = errors.New("process: command lock already held")

// CommandLock serializes high-level CLI commands (like daemon start) to prevent
// concurrent executions that would otherwise race on shared resources.
type CommandLock struct {
	name     string
	lockPath string
	lock     *flock.Flock
}

// AcquireCommandLock obtains an advisory filesystem lock for the given command
// name. The lock is stored alongside other DSM runtime files in the user's
// home directory. Callers must Release the returned lock to allow subsequent
// command executions.
func AcquireCommandLock(name string) (*CommandLock, error) {
	normalized := normalizeProcessName(name)
	if normalized == "" {
		return nil, fmt.Errorf("process: command lock: invalid command name %q", name)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("process: command lock: resolve home: %w", err)
	}

	lockPath := filepath.Join(home, fmt.Sprintf(".dotfile-sync-manager.%s.cmdlock", normalized))
	fileLock := flock.New(lockPath)

	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()

	locked, err := fileLock.TryLockContext(ctx, lockRetryInterval)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, ErrCommandLockHeld
		}
		return nil, fmt.Errorf("process: command lock: acquire %q: %w", normalized, err)
	}
	if !locked {
		return nil, ErrCommandLockHeld
	}

	return &CommandLock{
		name:     normalized,
		lockPath: lockPath,
		lock:     fileLock,
	}, nil
}

// Release frees the underlying filesystem lock and removes the lock file. It is
// safe to call multiple times.
func (cl *CommandLock) Release() error {
	if cl == nil || cl.lock == nil {
		return nil
	}

	var err error
	if unlockErr := cl.lock.Unlock(); unlockErr != nil {
		err = fmt.Errorf("process: command lock: release %q: %w", cl.name, unlockErr)
	}

	if removeErr := os.Remove(cl.lockPath); removeErr != nil && !os.IsNotExist(removeErr) {
		if err != nil {
			return fmt.Errorf("%v; remove lock file: %w", err, removeErr)
		}
		return fmt.Errorf("process: command lock: cleanup %q: %w", cl.name, removeErr)
	}

	return err
}
