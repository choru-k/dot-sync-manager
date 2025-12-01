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

// TestGracefulShutdown_TimeoutContext tests that gracefulShutdown completes successfully
// with a valid parent context that has sufficient timeout for shutdown operations.
// This tests the normal case where the parent context is not cancelled or expired.
func TestGracefulShutdown_TimeoutContext(t *testing.T) {
	// Create a context with timeout (simulating normal signal context with deadline)
	// This tests that shutdown completes within the timeout when parent is still valid
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

	// Test that gracefulShutdown completes successfully with valid parent context
	err = gracefulShutdown(parentCtx, svc, lockManager)
	if err != nil {
		t.Errorf("gracefulShutdown should complete with valid parent context, got error: %v", err)
	}
}

// TestGracefulShutdown_CancelledParentContext tests that gracefulShutdown works correctly
// when called with an already-cancelled parent context (the real production scenario).
// In production, gracefulShutdown is called AFTER <-signalCtx.Done(), so the parent is cancelled.
func TestGracefulShutdown_CancelledParentContext(t *testing.T) {
	// Create a context and cancel it immediately (simulating signal already received)
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before calling gracefulShutdown

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

	// Create a mock lock manager
	testPID := os.Getpid() + 200000 // Different test PID to avoid conflicts
	lockManager, err := process.WritePIDExclusive(testPID)
	if err != nil {
		t.Fatalf("Failed to create lock manager: %v", err)
	}

	// Test that gracefulShutdown succeeds even with cancelled parent
	// It should create its own fresh timeout context (context.Background() with 15s deadline)
	err = gracefulShutdown(cancelledCtx, svc, lockManager)
	if err != nil {
		t.Errorf("gracefulShutdown should succeed with cancelled parent, got error: %v", err)
	}

	// Verify PID cleanup completed by checking if we can acquire a new lock
	testPID2 := os.Getpid() + 300000 // Third test PID to verify cleanup
	newLockManager, err := process.WritePIDExclusive(testPID2)
	if err != nil {
		t.Errorf("Failed to acquire new lock after gracefulShutdown, PID may not be cleaned up: %v", err)
	} else {
		_ = newLockManager.Unlock() // Clean up
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
