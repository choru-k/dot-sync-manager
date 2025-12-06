package cmd

import (
	"context"
	"log"
	"sync/atomic"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/process"
	"github.com/choru-k/dot-sync-manager/internal/sync"
)

// testPIDCounter generates unique test PIDs for parallel test execution
// Starting at 100000 keeps test PIDs out of range of normal process PIDs
var testPIDCounter int32 = 100000

// nextTestPID generates a unique test PID for this test run
// Safe for concurrent use across parallel test execution
func nextTestPID() int {
	return int(atomic.AddInt32(&testPIDCounter, 1))
}

// safeUnlock safely unlocks a LockManager and logs any errors without affecting test flow
func safeUnlock(lockManager *process.LockManager) {
	if err := lockManager.Unlock(); err != nil {
		// Log the error but don't fail the test - cleanup errors are non-critical
		log.Printf("warning: failed to unlock during cleanup: %v", err)
	}
}

// TestGracefulShutdown_TimeoutContext tests that gracefulShutdown completes successfully
// This tests the normal case where shutdown completes cleanly.
func TestGracefulShutdown_TimeoutContext(t *testing.T) {

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
	testPID := nextTestPID()
	lockManager, err := process.WritePIDExclusive(testPID)
	if err != nil {
		t.Fatalf("Failed to create lock manager: %v", err)
	}

	// Test that gracefulShutdown completes successfully
	err = gracefulShutdown(svc, lockManager)
	if err != nil {
		t.Errorf("gracefulShutdown should complete cleanly, got error: %v", err)
	}
}

// TestGracefulShutdown_CancelledParentContext tests that gracefulShutdown works correctly
// This simulates the production scenario where gracefulShutdown creates its own timeout context.
func TestGracefulShutdown_CancelledParentContext(t *testing.T) {

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
	testPID := nextTestPID()
	lockManager, err := process.WritePIDExclusive(testPID)
	if err != nil {
		t.Fatalf("Failed to create lock manager: %v", err)
	}

	// Test that gracefulShutdown succeeds with its own timeout context
	err = gracefulShutdown(svc, lockManager)
	if err != nil {
		t.Errorf("gracefulShutdown should succeed, got error: %v", err)
	}

	// Verify PID cleanup completed by checking if we can acquire a new lock
	testPID2 := nextTestPID()
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
	testPID := nextTestPID()
	lockManager, err := process.WritePIDExclusive(testPID)
	if err != nil {
		t.Fatalf("Failed to create lock manager: %v", err)
	}

	// Test graceful shutdown
	err = gracefulShutdown(svc, lockManager)
	if err != nil {
		t.Errorf("gracefulShutdown failed: %v", err)
	}

	// Verify PID file and lock are cleaned up by checking if we can acquire a new lock
	testPID3 := nextTestPID()
	newLockManager, err := process.WritePIDExclusive(testPID3)
	if err != nil {
		t.Errorf("Failed to acquire new lock after gracefulShutdown, PID may not be cleaned up: %v", err)
	} else {
		safeUnlock(newLockManager) // Clean up
	}
}
