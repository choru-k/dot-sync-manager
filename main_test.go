package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

func TestExpandPath(t *testing.T) {
	// Test home directory expansion
	path := expandPath("~/test.txt")
	if path == "~/test.txt" {
		t.Error("Expected ~ to be expanded")
	}

	homeDir, _ := os.UserHomeDir()
	expected := filepath.Join(homeDir, "test.txt")
	if path != expected {
		t.Errorf("Expected %s, got %s", expected, path)
	}

	// Test regular path (no expansion)
	regularPath := expandPath("/tmp/test.txt")
	if regularPath != "/tmp/test.txt" {
		t.Errorf("Expected /tmp/test.txt, got %s", regularPath)
	}
}

func TestNewApplication(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create a test configuration
	testConfig := &config.SyncConfig{
		Version: "1.0",
		Machine: config.MachineConfig{
			Name: "test-machine",
		},
		Git: config.GitConfig{
			RepoPath:    tmpDir,
			RemoteURL:   "https://github.com/test/test.git",
			RemoteName:  "origin",
			Branch:      "main",
			AuthorName:  "Test User",
			AuthorEmail: "test@example.com",
			AuthType:    "none",
		},
		Sync: config.SyncSettings{
			AutoSyncEnabled:     false, // Disable for testing
			PullIntervalSeconds: 300,
			DebounceSeconds:     30,
		},
	}

	// Create application
	app, err := NewApplication(testConfig)
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}

	// Test basic properties
	if app.config.Machine.Name != "test-machine" {
		t.Errorf("Expected machine name 'test-machine', got %s", app.config.Machine.Name)
	}

	if app.config.Git.RepoPath != tmpDir {
		t.Errorf("Expected repo path %s, got %s", tmpDir, app.config.Git.RepoPath)
	}

	// Test GetStatus
	status := app.GetStatus()
	if status["machine"] != "test-machine" {
		t.Errorf("Expected status machine 'test-machine', got %v", status["machine"])
	}

	if status["running"] != false {
		t.Errorf("Expected status running=false, got %v", status["running"])
	}

	// Test start and stop
	if err := app.Start(); err != nil {
		t.Errorf("Failed to start application: %v", err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	if !app.syncService.IsRunning() {
		t.Error("Expected sync service to be running")
	}

	if err := app.Stop(); err != nil {
		t.Errorf("Failed to stop application: %v", err)
	}

	if app.syncService.IsRunning() {
		t.Error("Expected sync service to be stopped")
	}
}

func TestApplicationWithInvalidConfig(t *testing.T) {
	// Test with invalid configuration (missing repo path)
	invalidConfig := &config.SyncConfig{
		Version: "1.0",
		Machine: config.MachineConfig{
			Name: "test-machine",
		},
		Git: config.GitConfig{
			RepoPath: "", // Invalid
		},
		Sync: config.SyncSettings{
			PullIntervalSeconds: 300,
			DebounceSeconds:     30,
		},
	}

	// Should fail to create application
	_, err := NewApplication(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

func TestConfigFileLoading(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.json")
	repoPath := filepath.Join(tmpDir, "dotfiles")

	testConfig := &config.SyncConfig{
		Version: "1.0",
		Machine: config.MachineConfig{
			Name: "file-test-machine",
		},
		Git: config.GitConfig{
			RepoPath:    repoPath,
			RemoteURL:   "https://github.com/test/test.git",
			RemoteName:  "origin",
			Branch:      "main",
			AuthorName:  "Test User",
			AuthorEmail: "test@example.com",
			AuthType:    "none",
		},
		Sync: config.SyncSettings{
			AutoSyncEnabled:     false,
			PullIntervalSeconds: 300,
			DebounceSeconds:     30,
		},
	}

	// Save config to file
	err := testConfig.SaveToFile(configFile)
	if err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}

	// Load config from file
	loadedConfig, err := config.LoadFromFile(configFile)
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	// Verify loaded config
	if loadedConfig.Machine.Name != "file-test-machine" {
		t.Errorf("Expected machine name 'file-test-machine', got %s", loadedConfig.Machine.Name)
	}

	// Create repo directory so it doesn't try to clone
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}

	// Create application with loaded config
	app, err := NewApplication(loadedConfig)
	if err != nil {
		t.Fatalf("Failed to create application from loaded config: %v", err)
	}

	// Clean up
	app.Stop()
}
