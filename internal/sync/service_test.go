package sync

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
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
	ignoreFile := filepath.Join(tmpDir, ".syncignore")
	err := os.WriteFile(ignoreFile, []byte(ignoreContent), 0644)
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
		IgnoreFile:      ".syncignore",
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
		DebounceDelay:   100 * time.Millisecond,
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
	defer func() {
		if err := service.Stop(); err != nil {
			t.Fatalf("Failed to stop service: %v", err)
		}
	}()

	// Give the watcher a moment to start
	time.Sleep(50 * time.Millisecond)

	// Create a new directory
	newDir := filepath.Join(tmpDir, "new_directory")
	if err := os.Mkdir(newDir, 0755); err != nil {
		t.Fatalf("Failed to create new directory: %v", err)
	}

	// Wait for the watcher to pick up the change
	time.Sleep(200 * time.Millisecond)

	// Create a file in the new directory to verify it's being watched
	testFile := filepath.Join(newDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for debounce
	time.Sleep(200 * time.Millisecond)

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
		DebounceDelay:   100 * time.Millisecond,
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
			service.Stop()
		}()
	}

	wg.Wait()

	// Wait for cleanup to complete
	time.Sleep(50 * time.Millisecond)

	// Additional verification: multiple calls should be safe
	// Call Stop a few more times sequentially
	for i := 0; i < 3; i++ {
		service.Stop() // Should not panic
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
	{
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
			service.Stop() // Should not panic
		}
	}

	// Test multiple sequential Stop() calls on running service
	{
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
			service.Stop() // Should not panic, cleanup should happen only once
			time.Sleep(10 * time.Millisecond) // Small delay between calls
		}

		// Wait for cleanup to complete
		time.Sleep(50 * time.Millisecond)

		// Verify service is stopped
		if service.IsRunning() {
			t.Error("Service should be stopped")
		}
	}

	t.Log("Multiple Stop calls test completed successfully")
}

func TestSyncService_StopIdempotency(t *testing.T) {
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
	service.Stop()

	if service.IsRunning() {
		t.Error("Service should be stopped after first Stop() call")
	}

	// Get initial stats
	initialStats := service.GetStats()

	// Multiple subsequent stops should not change the state
	for i := 0; i < 10; i++ {
		service.Stop()
	}

	// Verify stats are unchanged
	finalStats := service.GetStats()

	if initialStats["running"] != finalStats["running"] {
		t.Error("Running state should not change after multiple Stop() calls")
	}

	t.Log("Stop idempotency test completed successfully")
}
