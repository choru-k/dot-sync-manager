package sync

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/debouncer"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
)

func TestSyncService_ManualSyncWithAutoSyncDisabled(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create git manager config
	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	// Create git manager
	gitManager, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	if err != nil {
		t.Fatalf("Failed to create git manager: %v", err)
	}

	// Create sync service config with AutoSyncEnabled = false
	backoffConfig := &debouncer.AdvancedDebouncerConfig{
		BaseDelay:          50 * time.Millisecond,
		MaxDelay:           5 * time.Minute,
		BackoffEnabled:     true,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     10,
		ChurnWindow:        1 * time.Minute,
		DecayResetDuration: 5 * time.Minute,
	}

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   100 * time.Millisecond,
		AutoSyncEnabled: false, // Disabled!
		IgnoreFile:      ".syncignore",
		Backoff:         backoffConfig,
	}

	// Create sync service
	syncService, err := New(gitManager, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Start the service
	err = syncService.Start()
	if err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}
	defer syncService.Stop()

	// Test manual sync - should work even though AutoSyncEnabled is false
	err = syncService.ManualSync()
	if err != nil {
		t.Errorf("Manual sync should work even when AutoSyncEnabled is false, got error: %v", err)
	}
}

func TestSyncService_ManualSyncAfterStop(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create git manager config
	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	// Create git manager
	gitManager, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	if err != nil {
		t.Fatalf("Failed to create git manager: %v", err)
	}

	// Create sync service config
	backoffConfig := &debouncer.AdvancedDebouncerConfig{
		BaseDelay:          50 * time.Millisecond,
		MaxDelay:           5 * time.Minute,
		BackoffEnabled:     true,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     10,
		ChurnWindow:        1 * time.Minute,
		DecayResetDuration: 5 * time.Minute,
	}

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   100 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
		Backoff:         backoffConfig,
	}

	// Create sync service
	syncService, err := New(gitManager, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Start the service
	err = syncService.Start()
	if err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}

	// Stop the service
	syncService.Stop()

	// Test manual sync after stop - should return error, not panic
	err = syncService.ManualSync()
	if err == nil {
		t.Error("Expected error when calling ManualSync after Stop")
	}

	if err != nil && !strings.Contains(err.Error(), "sync service is stopped") {
		t.Errorf("Expected error containing 'sync service is stopped', got: %v", err)
	}
}

func TestSyncService_ManualSyncAfterStop_BasicDebouncer(t *testing.T) {
	// Test with basic debouncer (no backoff config)
	tmpDir := t.TempDir()

	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	gitManager, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	if err != nil {
		t.Fatalf("Failed to create git manager: %v", err)
	}

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   100 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
		Backoff:         nil, // Use basic debouncer
	}

	syncService, err := New(gitManager, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	err = syncService.Start()
	if err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}

	syncService.Stop()

	// Should also return error with basic debouncer
	err = syncService.ManualSync()
	if err == nil {
		t.Error("Expected error when calling ManualSync after Stop with basic debouncer")
	}

	if err != nil && !strings.Contains(err.Error(), "sync service is stopped") {
		t.Errorf("Expected error containing 'sync service is stopped', got: %v", err)
	}
}
