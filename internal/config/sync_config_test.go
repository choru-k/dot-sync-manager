package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", config.Version)
	}

	if config.Machine.Name == "" {
		t.Error("Expected machine name to be set")
	}

	if config.Git.RepoPath == "" {
		t.Error("Expected git repo path to be set")
	}

	if !config.Sync.AutoSyncEnabled {
		t.Error("Expected auto-sync to be enabled by default")
	}

	if config.Sync.PullIntervalSeconds != 300 {
		t.Errorf("Expected pull interval 300s, got %d", config.Sync.PullIntervalSeconds)
	}

	if config.Sync.DebounceSeconds != 30 {
		t.Errorf("Expected debounce 30s, got %d", config.Sync.DebounceSeconds)
	}

	if config.Notifications.Enabled != true {
		t.Error("Expected notifications to be enabled by default")
	}

	if config.ConflictResolution.Strategy != "manual" {
		t.Errorf("Expected manual conflict resolution, got %s", config.ConflictResolution.Strategy)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *SyncConfig
		wantErr bool
	}{
		{
			name:   "valid config",
			config: DefaultConfig(),
		},
		{
			name: "empty machine name",
			config: func() *SyncConfig {
				c := DefaultConfig()
				c.Machine.Name = ""
				return c
			}(),
			wantErr: true,
		},
		{
			name: "empty repo path",
			config: func() *SyncConfig {
				c := DefaultConfig()
				c.Git.RepoPath = ""
				return c
			}(),
			wantErr: true,
		},
		{
			name: "relative repo path",
			config: func() *SyncConfig {
				c := DefaultConfig()
				c.Git.RepoPath = "relative/path"
				return c
			}(),
			wantErr: true,
		},
		{
			name: "empty author name",
			config: func() *SyncConfig {
				c := DefaultConfig()
				c.Git.AuthorName = ""
				return c
			}(),
			wantErr: true,
		},
		{
			name: "negative pull interval",
			config: func() *SyncConfig {
				c := DefaultConfig()
				c.Sync.PullIntervalSeconds = -1
				return c
			}(),
			wantErr: true,
		},
		{
			name: "zero debounce",
			config: func() *SyncConfig {
				c := DefaultConfig()
				c.Sync.DebounceSeconds = 0
				return c
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigSaveAndLoad(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")

	// Create a custom config
	originalConfig := &SyncConfig{
		Version: "1.0",
		Machine: MachineConfig{
			Name: "test-machine",
		},
		Git: GitConfig{
			RepoPath:    "/tmp/dotfiles",
			RemoteURL:   "https://github.com/test/test.git",
			RemoteName:  "origin",
			Branch:      "main",
			AuthorName:  "Test User",
			AuthorEmail: "test@example.com",
			AuthType:    gitmanager.AuthStrategySSH,
		},
		Sync: SyncSettings{
			AutoSyncEnabled:     true,
			PullIntervalSeconds: 600,
			DebounceSeconds:     60,
			AutoCommit:          true,
			AutoPush:            true,
			AutoPull:            true,
		},
		Notifications: NotificationConfig{
			Enabled:     true,
			ShowSuccess: true,
			ShowPulls:   false,
		},
		Mappings: map[string]string{
			"bashrc": "~/.bashrc",
			"vimrc":  "~/.vimrc",
		},
	}

	// Save config
	err := originalConfig.SaveToFile(configFile)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load config
	loadedConfig, err := LoadFromFile(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded config matches original
	if loadedConfig.Version != originalConfig.Version {
		t.Errorf("Expected version %s, got %s", originalConfig.Version, loadedConfig.Version)
	}

	if loadedConfig.Machine.Name != originalConfig.Machine.Name {
		t.Errorf("Expected machine name %s, got %s", originalConfig.Machine.Name, loadedConfig.Machine.Name)
	}

	if loadedConfig.Git.RepoPath != originalConfig.Git.RepoPath {
		t.Errorf("Expected repo path %s, got %s", originalConfig.Git.RepoPath, loadedConfig.Git.RepoPath)
	}

	if loadedConfig.Sync.PullIntervalSeconds != originalConfig.Sync.PullIntervalSeconds {
		t.Errorf("Expected pull interval %d, got %d", originalConfig.Sync.PullIntervalSeconds, loadedConfig.Sync.PullIntervalSeconds)
	}

	if len(loadedConfig.Mappings) != len(originalConfig.Mappings) {
		t.Errorf("Expected %d mappings, got %d", len(originalConfig.Mappings), len(loadedConfig.Mappings))
	}
}

func TestConfigLoadNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "nonexistent.json")

	// Loading non-existent file should return default config
	config, err := LoadFromFile(configFile)
	if err != nil {
		t.Errorf("Expected no error for non-existent file, got %v", err)
	}

	if config == nil {
		t.Error("Expected config to be returned")
	}

	// Should be default values
	if config.Version != "1.0" {
		t.Errorf("Expected default version 1.0, got %s", config.Version)
	}
}

func TestConfigLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(configFile, []byte("{ invalid json }"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	// Loading invalid JSON should return error
	_, err = LoadFromFile(configFile)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestConfigToGitManagerConfig(t *testing.T) {
	config := DefaultConfig()
	
	gitConfig := config.ToGitManagerConfig()
	
	if gitConfig.RepoPath != config.Git.RepoPath {
		t.Errorf("Expected repo path %s, got %s", config.Git.RepoPath, gitConfig.RepoPath)
	}
	
	if gitConfig.RemoteURL != config.Git.RemoteURL {
		t.Errorf("Expected remote URL %s, got %s", config.Git.RemoteURL, gitConfig.RemoteURL)
	}
	
	if gitConfig.AuthorName != config.Git.AuthorName {
		t.Errorf("Expected author name %s, got %s", config.Git.AuthorName, gitConfig.AuthorName)
	}
	
	if gitConfig.AuthType != config.Git.AuthType {
		t.Errorf("Expected auth type %s, got %s", config.Git.AuthType, gitConfig.AuthType)
	}
}

func TestConfigToSyncServiceConfig(t *testing.T) {
	config := DefaultConfig()
	
	syncConfig := config.ToSyncServiceConfig()
	
	if syncConfig.RepoPath != config.Git.RepoPath {
		t.Errorf("Expected repo path %s, got %s", config.Git.RepoPath, syncConfig.RepoPath)
	}
	
	expectedDelay := time.Duration(config.Sync.DebounceSeconds) * time.Second
	if syncConfig.DebounceDelay != expectedDelay {
		t.Errorf("Expected debounce delay %v, got %v", expectedDelay, syncConfig.DebounceDelay)
	}
	
	if syncConfig.AutoSyncEnabled != config.Sync.AutoSyncEnabled {
		t.Errorf("Expected auto-sync %v, got %v", config.Sync.AutoSyncEnabled, syncConfig.AutoSyncEnabled)
	}
	
	if syncConfig.IgnoreFile != ".syncignore" {
		t.Errorf("Expected ignore file .syncignore, got %s", syncConfig.IgnoreFile)
	}
}

func TestConfigJSONSerialization(t *testing.T) {
	config := DefaultConfig()
	
	// Marshal to JSON
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	
	// Unmarshal from JSON
	var unmarshaled SyncConfig
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}
	
	// Verify key fields match
	if unmarshaled.Version != config.Version {
		t.Errorf("Expected version %s, got %s", config.Version, unmarshaled.Version)
	}
	
	if unmarshaled.Machine.Name != config.Machine.Name {
		t.Errorf("Expected machine name %s, got %s", config.Machine.Name, unmarshaled.Machine.Name)
	}
	
	if unmarshaled.Sync.PullIntervalSeconds != config.Sync.PullIntervalSeconds {
		t.Errorf("Expected pull interval %d, got %d", config.Sync.PullIntervalSeconds, unmarshaled.Sync.PullIntervalSeconds)
	}
}
