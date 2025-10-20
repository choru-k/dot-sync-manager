package sync

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/debouncer"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
)

// TestSyncService_GoroutineLeakDetection tests that Stop() properly cleans up all goroutines
func TestSyncService_GoroutineLeakDetection(t *testing.T) {
	tmpDir := t.TempDir()

	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
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

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   50 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Get initial goroutine count
	initialGoroutines := runtime.NumGoroutine()

	// Start the service (this should spawn the eventLoop goroutine)
	err = service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait a moment for the eventLoop to start
	time.Sleep(10 * time.Millisecond)

	// Verify goroutines have increased
	startedGoroutines := runtime.NumGoroutine()
	if startedGoroutines <= initialGoroutines {
		t.Errorf("Expected goroutine count to increase after Start(), got %d (was %d)",
			startedGoroutines, initialGoroutines)
	}

	// Stop the service
	err = service.Stop()
	if err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	// Wait a moment for goroutines to clean up
	time.Sleep(50 * time.Millisecond)

	// Check that goroutines have been cleaned up
	finalGoroutines := runtime.NumGoroutine()
	goroutineDiff := finalGoroutines - initialGoroutines

	// Allow for some tolerance in goroutine counting (GC, testing framework, etc.)
	// But there should be no significant increase
	if goroutineDiff > 2 {
		t.Errorf("Potential goroutine leak detected. Initial: %d, After Start: %d, After Stop: %d (diff: %d)",
			initialGoroutines, startedGoroutines, finalGoroutines, goroutineDiff)
	}
}

// TestSyncService_MultipleStartStopGoroutineLeaks tests for leaks across multiple cycles
func TestSyncService_MultipleStartStopGoroutineLeaks(t *testing.T) {
	tmpDir := t.TempDir()

	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
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

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   10 * time.Millisecond, // Short delay for faster testing
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Get initial goroutine count
	initialGoroutines := runtime.NumGoroutine()

	// Perform multiple start/stop cycles
	numCycles := 5
	for i := 0; i < numCycles; i++ {
		// Start
		err := service.Start()
		if err != nil {
			t.Fatalf("Cycle %d: Failed to start service: %v", i, err)
		}

		// Let it run briefly
		time.Sleep(5 * time.Millisecond)

		// Stop
		err = service.Stop()
		if err != nil {
			t.Fatalf("Cycle %d: Failed to stop service: %v", i, err)
		}

		// Brief pause between cycles
		time.Sleep(5 * time.Millisecond)
	}

	// Wait for all goroutines to clean up
	time.Sleep(100 * time.Millisecond)

	// Check for goroutine leaks
	finalGoroutines := runtime.NumGoroutine()
	goroutineDiff := finalGoroutines - initialGoroutines

	if goroutineDiff > 3 {
		t.Errorf("Potential goroutine leak after %d cycles. Initial: %d, Final: %d (diff: %d)",
			numCycles, initialGoroutines, finalGoroutines, goroutineDiff)
	}
}

// TestSyncService_ConcurrentStartStopGoroutineLeaks tests for leaks under concurrent usage
func TestSyncService_ConcurrentStartStopGoroutineLeaks(t *testing.T) {
	tmpDir := t.TempDir()

	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
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

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   10 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Get initial goroutine count
	initialGoroutines := runtime.NumGoroutine()

	// Perform concurrent start/stop operations
	done := make(chan bool, 1)

	go func() {
		// Start the service
		err := service.Start()
		if err != nil {
			t.Errorf("Failed to start service: %v", err)
			return
		}

		// Let it run briefly
		time.Sleep(20 * time.Millisecond)

		// Stop the service
		err = service.Stop()
		if err != nil {
			t.Errorf("Failed to stop service: %v", err)
			return
		}

		done <- true
	}()

	// Wait for the goroutine to complete
	select {
	case <-done:
		// Operation completed
	case <-time.After(1 * time.Second):
		t.Fatal("Concurrent start/stop operation timed out")
	}

	// Wait for cleanup
	time.Sleep(50 * time.Millisecond)

	// Check for goroutine leaks
	finalGoroutines := runtime.NumGoroutine()
	goroutineDiff := finalGoroutines - initialGoroutines

	if goroutineDiff > 2 {
		t.Errorf("Potential goroutine leak after concurrent operations. Initial: %d, Final: %d (diff: %d)",
			initialGoroutines, finalGoroutines, goroutineDiff)
	}
}

// TestSyncService_AdvancedDebouncerGoroutineLeaks tests goroutine cleanup when using AdvancedDebouncer
func TestSyncService_AdvancedDebouncerGoroutineLeaks(t *testing.T) {
	tmpDir := t.TempDir()

	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
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

	// Create config with AdvancedDebouncer
	backoffConfig := &debouncer.AdvancedDebouncerConfig{
		BaseDelay:          10 * time.Millisecond,
		MaxDelay:           100 * time.Millisecond,
		BackoffEnabled:     true,
		ChurnThreshold:     5,
		ChurnWindow:        50 * time.Millisecond,
		DecayResetDuration: 200 * time.Millisecond,
		ManualSyncTimeout:  50 * time.Millisecond,
	}

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   10 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
		Backoff:         backoffConfig,
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Get initial goroutine count
	initialGoroutines := runtime.NumGoroutine()

	// Start the service
	err = service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// AdvancedDebouncer spawns additional goroutines for manual sync queue processing
	// Let it run briefly
	time.Sleep(20 * time.Millisecond)

	// Stop the service
	err = service.Stop()
	if err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	// Check for goroutine leaks
	finalGoroutines := runtime.NumGoroutine()
	goroutineDiff := finalGoroutines - initialGoroutines

	if goroutineDiff > 3 {
		t.Errorf("Potential goroutine leak with AdvancedDebouncer. Initial: %d, Final: %d (diff: %d)",
			initialGoroutines, finalGoroutines, goroutineDiff)
	}
}