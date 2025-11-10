package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/process"
	"github.com/choru-k/dot-sync-manager/internal/sync"
)

// TestGracefulShutdown_TimeoutContext tests that gracefulShutdown uses a fresh timeout context
// and doesn't inherit cancellation from the signal context
func TestGracefulShutdown_TimeoutContext(t *testing.T) {
	// Create a cancelled context (simulating the signal context being cancelled)
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Create a mock sync service and lock manager
	gitConfig := gitmanager.Config{
		RepoPath:    t.TempDir(),
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	gitMgr, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	if err != nil {
		t.Fatalf("Failed to create git manager: %v", err)
	}

	syncConfig := &sync.Config{
		RepoPath:        t.TempDir(),
		DebounceDelay:   50 * time.Millisecond,
		AutoSyncEnabled: false, // Disable auto-sync for test
		IgnoreFile:      ".syncignore",
	}

	syncSvc, err := sync.New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Create a mock lock manager by acquiring and immediately unlocking it
	lockManager, err := process.WritePIDExclusive(999999) // Fake PID
	if err != nil {
		t.Fatalf("Failed to create lock manager: %v", err)
	}
	defer lockManager.Unlock() // Clean up

	// Test that gracefulShutdown works even with cancelled signal context
	// This verifies Bug Fix 1: Line 223 uses context.Background() instead of signalCtx
	err = gracefulShutdown(cancelledCtx, syncSvc, lockManager)
	if err != nil {
		t.Errorf("gracefulShutdown should work with cancelled signal context, got error: %v", err)
	}
}

// TestGracefulShutdown_PIDLockRelease tests that PID lock is released exactly once
func TestGracefulShutdown_PIDLockRelease(t *testing.T) {
	// Create mock sync service
	gitConfig := gitmanager.Config{
		RepoPath:    t.TempDir(),
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	gitMgr, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	if err != nil {
		t.Fatalf("Failed to create git manager: %v", err)
	}

	syncConfig := &sync.Config{
		RepoPath:        t.TempDir(),
		DebounceDelay:   50 * time.Millisecond,
		AutoSyncEnabled: false,
		IgnoreFile:      ".syncignore",
	}

	syncSvc, err := sync.New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Create lock manager
	lockManager, err := process.WritePIDExclusive(999998) // Fake PID
	if err != nil {
		t.Fatalf("Failed to create lock manager: %v", err)
	}

	// Test graceful shutdown
	err = gracefulShutdown(context.Background(), syncSvc, lockManager)
	if err != nil {
		t.Errorf("gracefulShutdown failed: %v", err)
	}

	// Verify PID file and lock are cleaned up by checking if we can acquire a new lock
	newLockManager, err := process.WritePIDExclusive(999997) // Different fake PID
	if err != nil {
		t.Errorf("Failed to acquire new lock after gracefulShutdown, PID may not be cleaned up: %v", err)
	} else {
		newLockManager.Unlock() // Clean up
	}
}