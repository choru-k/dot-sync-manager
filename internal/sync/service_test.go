package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/debouncer"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
)

// fileModeUserReadWrite defines standard file permissions for user read/write access
const fileModeUserReadWrite = 0644

// testDebounceDelay defines the debounce delay used in tests for faster execution
const testDebounceDelay = 100 * time.Millisecond

// Test timing constants for consistent test behavior
const (
	testTickInterval    = 10 * time.Millisecond  // Small delay for event processing
	testProcessingDelay = 50 * time.Millisecond  // Delay for file system events
	testShutdownDelay   = 200 * time.Millisecond // Delay for shutdown verification
	testCleanupDelay    = 100 * time.Millisecond // Delay for final cleanup
)

func TestSyncService_New(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create git manager config
	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	// Create git manager
	gitMgr, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	if err != nil {
		t.Fatalf("Failed to create git manager: %v", err)
	}

	// Create sync config
	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   1 * time.Second,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
	}

	// Create sync service
	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Test basic properties
	if service.GetConfig().RepoPath != tmpDir {
		t.Errorf("Expected repo path %s, got %s", tmpDir, service.GetConfig().RepoPath)
	}

	if service.GetConfig().DebounceDelay != 1*time.Second {
		t.Errorf("Expected debounce delay 1s, got %v", service.GetConfig().DebounceDelay)
	}

	if !service.GetConfig().AutoSyncEnabled {
		t.Error("Expected auto-sync to be enabled")
	}

	// Test default values
	defaultConfig := &Config{
		RepoPath: tmpDir,
	}

	service2, err := New(gitMgr, defaultConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service with defaults: %v", err)
	}

	if service2.GetConfig().DebounceDelay != 30*time.Second {
		t.Errorf("Expected default debounce delay 30s, got %v", service2.GetConfig().DebounceDelay)
	}

	if service2.GetConfig().IgnoreFile != ".syncignore" {
		t.Errorf("Expected default ignore file .syncignore, got %s", service2.GetConfig().IgnoreFile)
	}
}

func TestSyncService_ConfigValidation(t *testing.T) {
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

	// Test nil config
	_, err = New(gitMgr, nil)
	if err == nil {
		t.Error("Expected error for nil config")
	}

	// Test empty repo path
	config := &Config{
		RepoPath: "",
	}
	_, err = New(gitMgr, config)
	if err == nil {
		t.Error("Expected error for empty repo path")
	}

	// Test valid config
	config.RepoPath = tmpDir
	_, err = New(gitMgr, config)
	if err != nil {
		t.Errorf("Expected no error for valid config, got %v", err)
	}
}

func TestSyncService_EventCallbacks(t *testing.T) {
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
		AutoSyncEnabled: false, // Disable to prevent actual sync during test
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Test setting callbacks - just verify they don't panic
	service.SetEventCallbacks(
		func() { /* sync start callback */ },
		func(files []string, err error) { /* sync complete callback */ },
		func(err error) { /* error callback */ },
	)

	// Test manual sync (should trigger callbacks)
	err = service.ManualSync()
	if err != nil {
		// Expected to fail since we don't have a real git repo
		t.Logf("Expected sync failure: %v", err)
	}

	// Note: In a real test with a proper git repo, callbacks would be triggered
	// For now, we just test that the callbacks can be set without panicking
}

func TestSyncService_IgnoreFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a .syncignore file
	ignoreContent := `*.log
*.tmp
!important.log
node_modules/
`
	// ignoreFilePath defines the path to the ignore file relative to repo root
	const ignoreFilePath = ".syncignore"
	ignoreFile := filepath.Join(tmpDir, ignoreFilePath)
	err := os.WriteFile(ignoreFile, []byte(ignoreContent), fileModeUserReadWrite)
	if err != nil {
		t.Fatalf("Failed to write ignore file: %v", err)
	}

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
		AutoSyncEnabled: false,
		IgnoreFile:      ignoreFilePath,
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Test that ignore patterns were loaded
	stats := service.GetStats()
	if ignorePatterns, ok := stats["ignore_patterns"].(int); ok {
		if ignorePatterns == 0 {
			t.Error("Expected ignore patterns to be loaded")
		}
	}

	// Test reload ignore patterns
	err = service.ReloadIgnorePatterns()
	if err != nil {
		t.Errorf("Failed to reload ignore patterns: %v", err)
	}
}

func TestSyncService_GetStats(t *testing.T) {
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
		DebounceDelay:   5 * time.Second,
		AutoSyncEnabled: true,
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	stats := service.GetStats()

	// Test stats values
	if stats["running"] != false {
		t.Errorf("Expected running=false, got %v", stats["running"])
	}

	if stats["repo_path"] != tmpDir {
		t.Errorf("Expected repo_path=%s, got %v", tmpDir, stats["repo_path"])
	}

	if stats["debounce_delay"] != "5s" {
		t.Errorf("Expected debounce_delay=5s, got %v", stats["debounce_delay"])
	}

	if stats["auto_sync"] != true {
		t.Errorf("Expected auto_sync=true, got %v", stats["auto_sync"])
	}
}

func TestSyncService_DynamicDirectoryWatching(t *testing.T) {
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
		DebounceDelay:   testDebounceDelay,
		AutoSyncEnabled: false, // Disable to prevent actual sync
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Start the service
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Stop(); err != nil {
			t.Errorf("Failed to stop service: %v", err)
		}
	})

	// Give the watcher a moment to start
	time.Sleep(testProcessingDelay)

	// Create a new directory
	newDir := filepath.Join(tmpDir, "new_directory")
	if err := os.Mkdir(newDir, 0755); err != nil {
		t.Fatalf("Failed to create new directory: %v", err)
	}

	// Wait for the watcher to pick up the change
	time.Sleep(testShutdownDelay)

	// Create a file in the new directory to verify it's being watched
	testFile := filepath.Join(newDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), fileModeUserReadWrite); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for debounce
	time.Sleep(testShutdownDelay)

	// The test passes if no panics occurred and the watcher handled the events
	// In a real scenario, we'd verify the file event was detected
	t.Log("Dynamic directory watching test completed successfully")
}

func TestSyncService_ConcurrentStop(t *testing.T) {
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
		DebounceDelay:   testDebounceDelay,
		AutoSyncEnabled: false, // Disable to prevent actual sync
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Start the service to have something to stop
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Test concurrent Stop calls
	numGoroutines := 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := service.Stop(); err != nil {
				// Log errors but don't fail test for concurrent shutdowns
				t.Logf("Concurrent Stop() error: %v", err)
			}
		}()
	}

	wg.Wait()

	// Stop() now blocks until cleanup is complete via sync.Once - no sleep needed

	// Additional verification: multiple calls should be safe
	// Call Stop a few more times sequentially
	for i := 0; i < 3; i++ {
		if err := service.Stop(); err != nil {
			// Log errors but don't fail test for multiple shutdowns
			t.Logf("Multiple Stop() error: %v", err)
		}
	}

	// Service should still be stopped
	if service.IsRunning() {
		t.Error("Service should still be stopped after multiple Stop() calls")
	}

	t.Log("Concurrent Stop test completed successfully")
}

func TestSyncService_MultipleStopCalls(t *testing.T) {
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

	// Test multiple sequential Stop() calls on non-started service
	t.Run("on non-started service", func(t *testing.T) {
		syncConfig := &Config{
			RepoPath:        tmpDir,
			AutoSyncEnabled: false,
		}

		service, err := New(gitMgr, syncConfig)
		if err != nil {
			t.Fatalf("Failed to create sync service: %v", err)
		}

		// Verify service is not running initially
		if service.IsRunning() {
			t.Error("Service should not be running without Start()")
		}

		// Test multiple sequential Stop() calls on non-started service
		for i := 0; i < 5; i++ {
			if err := service.Stop(); err != nil {
				// Log errors but don't fail test for stopping non-started service
				t.Logf("Non-started service Stop() error: %v", err)
			}
		}
	})

	// Test multiple sequential Stop() calls on running service
	t.Run("on running service", func(t *testing.T) {
		syncConfig := &Config{
			RepoPath:        tmpDir,
			AutoSyncEnabled: false,
		}

		service, err := New(gitMgr, syncConfig)
		if err != nil {
			t.Fatalf("Failed to create sync service: %v", err)
		}

		// Start the service
		if err := service.Start(); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}

		// Verify service is running
		if !service.IsRunning() {
			t.Error("Service should be running after Start()")
		}

		// Test multiple sequential Stop() calls on running service
		for i := 0; i < 5; i++ {
			if err := service.Stop(); err != nil {
				// Log errors but don't fail test for multiple shutdowns
				t.Logf("Running service Stop() error %d: %v", i, err)
			}
			time.Sleep(testTickInterval) // Small delay between calls
		}

		// Wait for cleanup to complete
		time.Sleep(testProcessingDelay)

		// Verify service is stopped
		if service.IsRunning() {
			t.Error("Service should be stopped")
		}
	})

	t.Log("Multiple Stop calls test completed successfully")
}

func TestSyncService_StopIdempotencyInServiceFile(t *testing.T) {
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
		AutoSyncEnabled: false,
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Start the service
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// First stop should stop the service
	if err := service.Stop(); err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	if service.IsRunning() {
		t.Error("Service should be stopped after first Stop() call")
	}

	// Get initial stats
	initialStats := service.GetStats()

	// Multiple subsequent stops should not change the state
	for i := 0; i < 10; i++ {
		if err := service.Stop(); err != nil {
			// Log errors but don't fail test for multiple shutdowns
			t.Logf("Multiple subsequent Stop() error %d: %v", i, err)
		}
	}

	// Verify stats are unchanged
	finalStats := service.GetStats()

	if initialStats["running"] != finalStats["running"] {
		t.Error("Running state should not change after multiple Stop() calls")
	}

	t.Log("Stop idempotency test completed successfully")
}

func TestSyncService_PrematureStopDoesNotDisableCleanup(t *testing.T) {
	// Test for P1 issue: Stop() called before Start() should not prevent cleanup
	// when the service is actually started and stopped later
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
		AutoSyncEnabled: false,
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Verify service is not running initially
	if service.IsRunning() {
		t.Error("Service should not be running without Start()")
	}

	// Step 1: Call Stop() before Start() - this should not burn the sync.Once
	if err := service.Stop(); err != nil {
		// Log errors but don't fail test for premature stop
		t.Logf("Premature Stop() error: %v", err)
	}

	// Step 2: Start the service normally
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Verify service is running
	if !service.IsRunning() {
		t.Error("Service should be running after Start()")
	}

	// Step 3: Call Stop() after the service is actually running
	// This should perform proper cleanup and not be blocked by the premature Stop() call
	if err := service.Stop(); err != nil {
		t.Fatalf("Failed to stop running service: %v", err)
	}

	// Stop() now blocks until cleanup is complete via sync.Once - no sleep needed

	// Verify service is stopped
	if service.IsRunning() {
		t.Error("Service should be stopped after Stop()")
	}

	// Additional verification: Multiple Stop() calls after actual start/stop should still work
	for i := 0; i < 3; i++ {
		if err := service.Stop(); err != nil {
			// Log errors but don't fail test for final verification calls
			t.Logf("Final verification Stop() error %d: %v", i, err)
		}
	}

	t.Log("Premature Stop() test completed successfully - cleanup was not disabled")
}

func TestSyncService_GracefulShutdownCompletion(t *testing.T) {
	// Test that graceful shutdown properly waits for the event loop to complete
	// and that all resources are cleaned up correctly
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
		DebounceDelay:   testDebounceDelay,
		AutoSyncEnabled: false, // Disable to prevent actual sync during test
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Start the service
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Verify service is running
	if !service.IsRunning() {
		t.Error("Service should be running after Start()")
	}

	// Create a file to trigger file system events
	testFile := filepath.Join(tmpDir, "test_graceful_shutdown.txt")
	if err := os.WriteFile(testFile, []byte("test content"), fileModeUserReadWrite); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Give a moment for the event loop to process the file event
	time.Sleep(testProcessingDelay)

	// Record the time before starting shutdown
	shutdownStart := time.Now()

	// Stop the service - this should wait for graceful shutdown
	if err := service.Stop(); err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	shutdownDuration := time.Since(shutdownStart)

	// Verify service is stopped
	if service.IsRunning() {
		t.Error("Service should be stopped after Stop()")
	}

	// Shutdown should complete reasonably quickly (not hang indefinitely)
	// Allow some buffer for cleanup operations
	if shutdownDuration > 5*time.Second {
		t.Errorf("Graceful shutdown took too long: %v", shutdownDuration)
	}

	// Verify that the service cannot be started again after graceful shutdown
	if err := service.Start(); err == nil {
		t.Fatal("Expected service to fail on restart, but it succeeded")
	}

	t.Logf("Graceful shutdown completed in: %v", shutdownDuration)
}

func TestSyncService_EventLoopRaceConditions(t *testing.T) {
	// Test race conditions in the event loop with concurrent file operations
	// and service lifecycle operations
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
		DebounceDelay:   testDebounceDelay,
		AutoSyncEnabled: false, // Disable to prevent actual sync during test
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Start the service
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Verify service is running
	if !service.IsRunning() {
		t.Error("Service should be running after Start()")
	}

	// Use a WaitGroup to coordinate concurrent operations
	var operationsWG sync.WaitGroup
	numFileOperations := 20
	numConcurrentStarts := 5
	numConcurrentStops := 3

	// Concurrent file operations to stress the event loop
	operationsWG.Add(numFileOperations)
	for i := 0; i < numFileOperations; i++ {
		go func(id int) {
			defer operationsWG.Done()

			// Create files with some delay to stress the event loop
			for j := 0; j < 5; j++ {
				testFile := filepath.Join(tmpDir, fmt.Sprintf("race_test_%d_%d.txt", id, j))
				content := fmt.Sprintf("content from goroutine %d, iteration %d", id, j)

				if err := os.WriteFile(testFile, []byte(content), fileModeUserReadWrite); err != nil {
					t.Errorf("Failed to create test file %s: %v", testFile, err)
					return
				}

				// Small delay to create race conditions
				time.Sleep(time.Duration(1+j) * 10 * time.Millisecond)
			}
		}(i)
	}

	// Concurrent Start() operations (should fail gracefully after first one)
	operationsWG.Add(numConcurrentStarts)
	for i := 0; i < numConcurrentStarts; i++ {
		go func(id int) {
			defer operationsWG.Done()

			if err := service.Start(); err != nil {
				// Expected to fail after first successful start
				t.Logf("Concurrent start %d failed as expected: %v", id, err)
			} else {
				t.Logf("Concurrent start %d succeeded", id)
			}
		}(i)
	}

	// Concurrent Stop() operations
	operationsWG.Add(numConcurrentStops)
	for i := 0; i < numConcurrentStops; i++ {
		go func(id int) {
			defer operationsWG.Done()

			// Add some delay before stopping to allow file operations
			time.Sleep(testProcessingDelay)

			if err := service.Stop(); err != nil {
				t.Logf("Concurrent stop %d failed: %v", id, err)
			} else {
				t.Logf("Concurrent stop %d succeeded", id)
			}
		}(i)
	}

	// Wait for all operations to complete
	operationsWG.Wait()

	// Give a moment for any remaining events to be processed
	time.Sleep(testCleanupDelay)

	// Verify final state - service should be stopped
	if service.IsRunning() {
		t.Error("Service should be stopped after concurrent operations")
	}

	// Verify that service cannot be restarted cleanly
	if err := service.Start(); err == nil {
		t.Fatal("Expected service to fail on restart, but it succeeded")
	}

	t.Log("Event loop race conditions test completed successfully")
}

func TestSyncService_ErrorHandlingDuringGracefulShutdown(t *testing.T) {
	// Test error handling during graceful shutdown, including errors from
	// git manager, watcher, and debouncer cleanup operations
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

	// Create a sync service with advanced debouncer for testing advanced cleanup
	advancedConfig := debouncer.AdvancedDebouncerConfig{
		BaseDelay:          testDebounceDelay,
		MaxDelay:           1 * time.Minute,
		BackoffEnabled:     false,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     5,
		ChurnWindow:        200 * time.Millisecond,
		DecayResetDuration: 1 * time.Second,
		ManualSyncTimeout:  1 * time.Second,
	}

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   testDebounceDelay,
		AutoSyncEnabled: false, // Disable to prevent actual sync during test
		Backoff:         &advancedConfig,
	}

	// Test 1: Normal graceful shutdown should not return errors
	t.Run("normal graceful shutdown", func(t *testing.T) {
		service, err := New(gitMgr, syncConfig)
		if err != nil {
			t.Fatalf("Failed to create sync service: %v", err)
		}

		if err := service.Start(); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}

		// Create some activity
		testFile := filepath.Join(tmpDir, "error_test.txt")
		if err := os.WriteFile(testFile, []byte("test content"), fileModeUserReadWrite); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Give a moment for processing
		time.Sleep(testProcessingDelay)

		// Stop should complete without errors
		if err := service.Stop(); err != nil {
			t.Errorf("Normal graceful shutdown should not return errors, got: %v", err)
		}

		if service.IsRunning() {
			t.Error("Service should be stopped after normal shutdown")
		}
	})

	// Test 2: Multiple concurrent Stop() calls should handle errors gracefully
	t.Run("concurrent shutdown handling", func(t *testing.T) {
		service, err := New(gitMgr, syncConfig)
		if err != nil {
			t.Fatalf("Failed to create sync service: %v", err)
		}

		if err := service.Start(); err != nil {
			t.Fatalf("Failed to restart service: %v", err)
		}

		var shutdownWG sync.WaitGroup
		numConcurrentStops := 5
		var errorCount int32
		var mu sync.Mutex

		// Concurrent Stop() calls
		shutdownWG.Add(numConcurrentStops)
		for i := 0; i < numConcurrentStops; i++ {
			go func(id int) {
				defer shutdownWG.Done()

				if err := service.Stop(); err != nil {
					mu.Lock()
					errorCount++
					mu.Unlock()
					t.Logf("Concurrent stop %d returned error: %v", id, err)
				} else {
					t.Logf("Concurrent stop %d succeeded", id)
				}
			}(i)
		}

		shutdownWG.Wait()

		// At least one Stop() should succeed without errors
		mu.Lock()
		if int(errorCount) == numConcurrentStops {
			t.Errorf("All %d concurrent Stop() calls failed - expected at least one to succeed", numConcurrentStops)
		} else {
			t.Logf("Concurrent shutdown test: %d/%d calls failed, %d succeeded",
				errorCount, numConcurrentStops, numConcurrentStops-int(errorCount))
		}
		mu.Unlock()

		if service.IsRunning() {
			t.Error("Service should be stopped after concurrent shutdown test")
		}
	})

	// Test 3: Error callback handling during shutdown
	t.Run("error callback handling", func(t *testing.T) {
		service, err := New(gitMgr, syncConfig)
		if err != nil {
			t.Fatalf("Failed to create sync service: %v", err)
		}

		if err := service.Start(); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}

		var errorCallbackCalled bool
		var syncCompleteCalled bool
		var syncStartCalled bool

		// Set error callbacks
		service.SetEventCallbacks(
			func() { syncStartCalled = true },
			func(files []string, err error) {
				syncCompleteCalled = true
				if err != nil {
					errorCallbackCalled = true
				}
			},
			func(err error) { errorCallbackCalled = true },
		)

		// Trigger a manual sync to test callback handling
		if err := service.ManualSync(); err != nil {
			t.Logf("Manual sync returned error (expected for test): %v", err)
		}

		// Stop the service
		if err := service.Stop(); err != nil {
			t.Logf("Service stop returned error: %v", err)
		}

		// Verify callbacks were set up properly (they may or may not have been called depending on implementation)
		t.Logf("Error handling test - callbacks called: start=%v, complete=%v, error=%v",
			syncStartCalled, syncCompleteCalled, errorCallbackCalled)

		if service.IsRunning() {
			t.Error("Service should be stopped after error callback test")
		}
	})

	// Test 4: Service restart after error scenarios
	t.Run("restart after error scenarios", func(t *testing.T) {
		// Test that a new service can be started successfully after a previous one was stopped
		for i := 0; i < 3; i++ {
			service, err := New(gitMgr, syncConfig)
			if err != nil {
				t.Fatalf("Failed to create service in restart test iteration %d: %v", i, err)
			}

			if err := service.Start(); err != nil {
				t.Fatalf("Failed to start service in restart test iteration %d: %v", i, err)
			}

			// Simulate some activity
			testFile := filepath.Join(tmpDir, fmt.Sprintf("restart_test_%d.txt", i))
			if err := os.WriteFile(testFile, []byte(fmt.Sprintf("iteration %d", i)), fileModeUserReadWrite); err != nil {
				t.Logf("Warning: failed to create test file in iteration %d: %v", i, err)
			}

			// Small delay
			time.Sleep(testTickInterval)

			// Stop the service
			if err := service.Stop(); err != nil {
				t.Logf("Stop in restart test iteration %d returned error: %v", i, err)
			}

			if service.IsRunning() {
				t.Errorf("Service should be stopped after restart test iteration %d", i)
				break
			}
		}

		t.Log("Restart after error scenarios test completed")
	})

	t.Log("Error handling during graceful shutdown test completed successfully")
}
