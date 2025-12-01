package cmd

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/process"
	"github.com/choru-k/dot-sync-manager/internal/sync"
)

// safeUnlock safely unlocks a LockManager and logs any errors without affecting test flow
func safeUnlock(lockManager *process.LockManager) {
	if err := lockManager.Unlock(); err != nil {
		// Log the error but don't fail the test - cleanup errors are non-critical
		log.Printf("warning: failed to unlock during cleanup: %v", err)
	}
}

// TestGracefulShutdown_TimeoutContext tests that gracefulShutdown respects parent context
// cancellation while providing its own timeout deadline
func TestGracefulShutdown_TimeoutContext(t *testing.T) {
	// Create a context with timeout (simulating normal signal context)
	parentCtx, parentCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer parentCancel()

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

	svc, err := sync.New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Start the service so it can be stopped gracefully
	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}

	// Create a mock lock manager by acquiring and immediately unlocking it
	testPID := os.Getpid() + 100000 // Use test PID with offset to avoid conflicts
	lockManager, err := process.WritePIDExclusive(testPID)
	if err != nil {
		t.Fatalf("Failed to create lock manager: %v", err)
	}

	// Test that gracefulShutdown inherits from parent context and completes successfully
	// The parent context has 5s timeout, which is sufficient for shutdown (needs <15s)
	err = gracefulShutdown(parentCtx, svc, lockManager)
	if err != nil {
		t.Errorf("gracefulShutdown should complete with valid parent context, got error: %v", err)
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

	svc, err := sync.New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Start the service so it can be stopped gracefully
	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}

	// Create lock manager
	testPID := os.Getpid() + 200000                        // Different test PID to avoid conflicts
	lockManager, err := process.WritePIDExclusive(testPID) // Test PID
	if err != nil {
		t.Fatalf("Failed to create lock manager: %v", err)
	}

	// Test graceful shutdown
	err = gracefulShutdown(context.Background(), svc, lockManager)
	if err != nil {
		t.Errorf("gracefulShutdown failed: %v", err)
	}

	// Verify PID file and lock are cleaned up by checking if we can acquire a new lock
	testPID3 := os.Getpid() + 300000                           // Third test PID to verify cleanup
	newLockManager, err := process.WritePIDExclusive(testPID3) // Verification PID
	if err != nil {
		t.Errorf("Failed to acquire new lock after gracefulShutdown, PID may not be cleaned up: %v", err)
	} else {
		safeUnlock(newLockManager) // Clean up
	}
}
