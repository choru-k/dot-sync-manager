package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/util"
)

// mustDefaultConfig is a test helper that calls DefaultConfig and fails the test if it errors
func mustDefaultConfig(t *testing.T) *SyncConfig {
	t.Helper()
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}
	return cfg
}

func TestDefaultConfig(t *testing.T) {
	config, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}

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
			config: mustDefaultConfig(t),
		},
		{
			name: "empty machine name",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.Machine.Name = ""
				return c
			}(),
			wantErr: true,
		},
		{
			name: "empty repo path",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.Git.RepoPath = ""
				return c
			}(),
			wantErr: true,
		},
		{
			name: "relative repo path",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.Git.RepoPath = "relative/path"
				return c
			}(),
			wantErr: true,
		},
		{
			name: "empty author name",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.Git.AuthorName = ""
				return c
			}(),
			wantErr: true,
		},
		{
			name: "negative pull interval",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.Sync.PullIntervalSeconds = -1
				return c
			}(),
			wantErr: true,
		},
		{
			name: "zero debounce",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
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
		ConflictResolution: ConflictConfig{
			Strategy:        "manual",
			BackupDir:       "/tmp/backup",
			KeepBackupsDays: 7,
		},
		Mappings: map[string]string{
			"bashrc": "~/.bashrc",
			"vimrc":  "~/.vimrc",
		},
		UI: UIConfig{
			StartAtBoot:    false,
			MinimizeToTray: true,
			Theme:          "auto",
		},
		Advanced: AdvancedConfig{
			DebugLogging: false,
			LogFile:      "/tmp/test.log",
			MaxLogSizeMB: 10,
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
		t.Fatal("Expected config to be returned")
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
	config, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}

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
	config, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}

	syncConfig := config.ToSyncConfig()

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
	config, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}

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

func TestFindConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	// Create dotfiles directory
	dotfilesDir := filepath.Join(tmpDir, "dotfiles")
	if err := os.MkdirAll(dotfilesDir, 0755); err != nil {
		t.Fatalf("Failed to create dotfiles directory: %v", err)
	}

	tests := []struct {
		name               string
		explicitPath       string
		prdConfigExists    bool
		legacyConfigExists bool
		expectedPath       string
		expectedExists     bool
	}{
		{
			name:               "explicit path takes priority",
			explicitPath:       filepath.Join(tmpDir, "custom.json"),
			prdConfigExists:    true,
			legacyConfigExists: true,
			expectedPath:       filepath.Join(tmpDir, "custom.json"),
			expectedExists:     false, // We don't create it in this test
		},
		{
			name:               "PRD location used when exists",
			explicitPath:       "",
			prdConfigExists:    true,
			legacyConfigExists: false,
			expectedPath:       filepath.Join(dotfilesDir, ".sync-config.json"),
			expectedExists:     true,
		},
		{
			name:               "legacy location used when PRD doesn't exist",
			explicitPath:       "",
			prdConfigExists:    false,
			legacyConfigExists: true,
			expectedPath:       filepath.Join(tmpDir, ".dotfile-sync.json"),
			expectedExists:     true,
		},
		{
			name:               "no config files exist",
			explicitPath:       "",
			prdConfigExists:    false,
			legacyConfigExists: false,
			expectedPath:       filepath.Join(dotfilesDir, ".sync-config.json"),
			expectedExists:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing files
			_ = os.Remove(filepath.Join(dotfilesDir, ".sync-config.json"))
			_ = os.Remove(filepath.Join(tmpDir, ".dotfile-sync.json"))
			_ = os.Remove(tt.explicitPath)

			// Create config files as specified by test
			if tt.prdConfigExists {
				if err := os.WriteFile(filepath.Join(dotfilesDir, ".sync-config.json"), []byte("{}"), 0644); err != nil {
					t.Fatalf("Failed to create PRD config file: %v", err)
				}
			}
			if tt.legacyConfigExists {
				if err := os.WriteFile(filepath.Join(tmpDir, ".dotfile-sync.json"), []byte("{}"), 0644); err != nil {
					t.Fatalf("Failed to create legacy config file: %v", err)
				}
			}
			if tt.explicitPath != "" {
				// For explicit path tests, we control whether the file exists based on tt.expectedExists
				if tt.expectedExists {
					if err := os.WriteFile(tt.explicitPath, []byte("{}"), 0644); err != nil {
						t.Fatalf("Failed to create explicit config file: %v", err)
					}
				}
			}

			// Test FindConfigFile
			path, exists, err := FindConfigFile(tt.explicitPath)
			if err != nil {
				t.Fatalf("FindConfigFile() error: %v", err)
			}

			if path != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, path)
			}
			if exists != tt.expectedExists {
				t.Errorf("Expected exists %v, got %v", tt.expectedExists, exists)
			}
		})
	}
}

func TestLoadFromDefaultLocation(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	// Create dotfiles directory
	dotfilesDir := filepath.Join(tmpDir, "dotfiles")
	if err := os.MkdirAll(dotfilesDir, 0755); err != nil {
		t.Fatalf("Failed to create dotfiles directory: %v", err)
	}

	t.Run("no config file exists", func(t *testing.T) {
		// Ensure no config files exist
		_ = os.Remove(filepath.Join(dotfilesDir, ".sync-config.json"))
		_ = os.Remove(filepath.Join(tmpDir, ".dotfile-sync.json"))

		config, err := LoadFromDefaultLocation()
		if err != nil {
			t.Fatalf("LoadFromDefaultLocation() error: %v", err)
		}

		// Should return default config
		if config.Version != "1.0" {
			t.Errorf("Expected default version 1.0, got %s", config.Version)
		}
	})

	t.Run("PRD config file exists", func(t *testing.T) {
		// Create a PRD config file
		configPath := filepath.Join(dotfilesDir, ".sync-config.json")
		testConfig := `{
			"version": "1.0",
			"machine": {"name": "test-machine"},
			"sync": {"auto_sync_enabled": false, "pull_interval_seconds": 600, "debounce_seconds": 60}
		}`
		if err := os.WriteFile(configPath, []byte(testConfig), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		config, err := LoadFromDefaultLocation()
		if err != nil {
			t.Fatalf("LoadFromDefaultLocation() error: %v", err)
		}

		if config.Machine.Name != "test-machine" {
			t.Errorf("Expected machine name 'test-machine', got '%s'", config.Machine.Name)
		}
		if config.Sync.AutoSyncEnabled != false {
			t.Errorf("Expected auto_sync_enabled false, got %v", config.Sync.AutoSyncEnabled)
		}
	})
}

func TestConfigValidationEnhanced(t *testing.T) {
	tests := []struct {
		name    string
		config  *SyncConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			config:  mustDefaultConfig(t),
			wantErr: false,
		},
		{
			name: "empty version",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.Version = ""
				return c
			}(),
			wantErr: true,
			errMsg:  "configuration version is required",
		},
		{
			name: "invalid email format",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.Git.AuthorEmail = "invalid-email"
				return c
			}(),
			wantErr: true,
			errMsg:  "git author email must be a valid email address",
		},
		{
			name: "pull interval too short",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.Sync.PullIntervalSeconds = 30
				return c
			}(),
			wantErr: true,
			errMsg:  "pull interval must be at least 60 seconds",
		},
		{
			name: "debounce exceeds pull interval",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.Sync.PullIntervalSeconds = 300
				c.Sync.DebounceSeconds = 400
				return c
			}(),
			wantErr: true,
			errMsg:  "debounce delay must not exceed pull interval",
		},
		{
			name: "invalid conflict strategy",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.ConflictResolution.Strategy = "invalid"
				return c
			}(),
			wantErr: true,
			errMsg:  "invalid conflict resolution strategy",
		},
		{
			name: "empty conflict strategy",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.ConflictResolution.Strategy = ""
				return c
			}(),
			wantErr: true,
			errMsg:  "conflict resolution strategy is required",
		},
		{
			name: "backup retention too long",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.ConflictResolution.KeepBackupsDays = 500
				return c
			}(),
			wantErr: true,
			errMsg:  "backup retention days must not exceed 365",
		},
		{
			name: "invalid UI theme",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.UI.Theme = "invalid"
				return c
			}(),
			wantErr: true,
			errMsg:  "invalid UI theme",
		},
		{
			name: "empty UI theme",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.UI.Theme = ""
				return c
			}(),
			wantErr: true,
			errMsg:  "UI theme is required",
		},
		{
			name: "log size too large",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.Advanced.MaxLogSizeMB = 2000
				return c
			}(),
			wantErr: true,
			errMsg:  "maximum log size must not exceed 1000 MB",
		},
		{
			name: "mapping target requires absolute path",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.Mappings = map[string]string{
					"bashrc": "relative/path", // Relative path should fail validation
				}
				return c
			}(),
			wantErr: true,
			errMsg:  "mapping target for 'bashrc' must be an absolute path",
		},
		{
			name: "valid mapping target with absolute path",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				// Use absolute paths (as they would be after expandPaths())
				homeDir, err := os.UserHomeDir()
				if err != nil {
					t.Fatalf("could not get user home dir: %v", err)
				}
				c.Mappings = map[string]string{
					"bashrc": filepath.Join(homeDir, ".bashrc"),
					"config": filepath.Join(homeDir, ".config"),
				}
				return c
			}(),
			wantErr: false,
		},
		{
			name: "empty mapping source",
			config: func() *SyncConfig {
				c := mustDefaultConfig(t)
				c.Mappings = map[string]string{
					"": "~/.bashrc",
				}
				return c
			}(),
			wantErr: true,
			errMsg:  "mapping source cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Expected error message to contain '%s', got '%s'", tt.errMsg, err.Error())
			}
		})
	}
}

func TestMachineConfigJSONCompatibility(t *testing.T) {
	tests := []struct {
		name           string
		jsonData       string
		expectName     string
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:        "marshal to string",
			jsonData:    "", // Not used for marshal test
			expectName:  "test-machine",
			expectError: false,
		},
		{
			name:        "PRD string input",
			jsonData:    `"prd-machine"`,
			expectName:  "prd-machine",
			expectError: false,
		},
		{
			name:        "legacy object input",
			jsonData:    `{"name": "legacy-machine"}`,
			expectName:  "legacy-machine",
			expectError: false,
		},
		{
			name:           "invalid format error",
			jsonData:       `123`,
			expectName:     "",
			expectError:    true,
			expectedErrMsg: "machine config must be a string or object with 'name' field",
		},
		{
			name:        "empty object valid legacy case",
			jsonData:    `{}`,
			expectName:  "", // Empty name is acceptable for legacy configs and validated later
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "marshal to string" {
				// Test marshaling to string format
				mc := MachineConfig{Name: tt.expectName}
				data, err := json.Marshal(mc)
				if err != nil {
					t.Fatalf("Failed to marshal MachineConfig: %v", err)
				}

				// Verify it's a string, not an object
				var strVal string
				if err := json.Unmarshal(data, &strVal); err != nil {
					t.Errorf("Expected marshaled data to be a string, got: %s", string(data))
				}

				if strVal != tt.expectName {
					t.Errorf("Expected marshaled name '%s', got '%s'", tt.expectName, strVal)
				}
				return
			}

			// Test unmarshaling from JSON
			var mc MachineConfig
			err := json.Unmarshal([]byte(tt.jsonData), &mc)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.expectedErrMsg != "" && !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.expectedErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if mc.Name != tt.expectName {
				t.Errorf("Expected name '%s', got '%s'", tt.expectName, mc.Name)
			}
		})
	}
}

func TestGitConfigUnmarshalPreservesDefaults(t *testing.T) {
	originalConfig := GitConfig{
		RepoPath:       "/default/repo",
		RemoteURL:      "https://github.com/default/default.git",
		RemoteName:     "origin",
		Branch:         "main",
		AuthorName:     "Default User",
		AuthorEmail:    "default@example.com",
		AuthType:       gitmanager.AuthStrategySSH,
		Username:       "defaultuser",
		Password:       "defaultpass",
		SSHKeyPath:     "/default/key",
		KnownHostsPath: "/default/known_hosts",
	}

	tests := []struct {
		name     string
		jsonData string
		expected GitConfig
	}{
		{
			name:     "empty JSON preserves all defaults",
			jsonData: `{}`,
			expected: originalConfig,
		},
		{
			name:     "partial JSON only overwrites present fields",
			jsonData: `{"repo_path": "/custom/repo"}`,
			expected: func() GitConfig {
				cfg := originalConfig
				cfg.RepoPath = "/custom/repo"
				return cfg
			}(),
		},
		{
			name:     "null JSON preserves all defaults",
			jsonData: `{"repo_path": null, "remote_url": null, "remote": null, "branch": null, "user_name": null, "user_email": null}`,
			expected: originalConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := originalConfig // Start with a fresh copy
			err := json.Unmarshal([]byte(tt.jsonData), &config)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !reflect.DeepEqual(config, tt.expected) {
				t.Errorf("Config mismatch. got=%+v, want=%+v", config, tt.expected)
			}
		})
	}
}

func TestGitConfigUnmarshalFieldPreference(t *testing.T) {
	// Create a GitConfig with initial values
	config := GitConfig{
		RepoPath:    "/initial/repo",
		RemoteURL:   "",
		RemoteName:  "initial-remote",
		Branch:      "initial-branch",
		AuthorName:  "Initial User",
		AuthorEmail: "initial@example.com",
		AuthType:    gitmanager.AuthStrategySSH,
	}

	tests := []struct {
		name            string
		jsonData        string
		expectRemote    string
		expectUserName  string
		expectUserEmail string
	}{
		{
			name: "PRD keys win when both PRD and legacy keys provided",
			jsonData: `{
				"remote": "prd-remote",
				"remote_name": "legacy-remote",
				"user_name": "prd-user",
				"author_name": "legacy-user",
				"user_email": "prd@example.com",
				"author_email": "legacy@example.com"
			}`,
			expectRemote:    "prd-remote",
			expectUserName:  "prd-user",
			expectUserEmail: "prd@example.com",
		},
		{
			name: "legacy keys populate fields when PRD keys absent",
			jsonData: `{
				"remote_name": "legacy-remote",
				"author_name": "legacy-user",
				"author_email": "legacy@example.com"
			}`,
			expectRemote:    "legacy-remote",
			expectUserName:  "legacy-user",
			expectUserEmail: "legacy@example.com",
		},
		{
			name: "PRD keys work independently",
			jsonData: `{
				"remote": "prd-only-remote",
				"user_name": "prd-only-user",
				"user_email": "prd-only@example.com"
			}`,
			expectRemote:    "prd-only-remote",
			expectUserName:  "prd-only-user",
			expectUserEmail: "prd-only@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset config to initial values
			testConfig := config

			err := json.Unmarshal([]byte(tt.jsonData), &testConfig)
			if err != nil {
				t.Fatalf("Unexpected error unmarshaling JSON: %v", err)
			}

			if testConfig.RemoteName != tt.expectRemote {
				t.Errorf("Expected remote_name to be '%s', got '%s'", tt.expectRemote, testConfig.RemoteName)
			}
			if testConfig.AuthorName != tt.expectUserName {
				t.Errorf("Expected author_name to be '%s', got '%s'", tt.expectUserName, testConfig.AuthorName)
			}
			if testConfig.AuthorEmail != tt.expectUserEmail {
				t.Errorf("Expected author_email to be '%s', got '%s'", tt.expectUserEmail, testConfig.AuthorEmail)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	originalHome := os.Getenv("HOME")
	testHome := "/test/home"
	if err := os.Setenv("HOME", testHome); err != nil {
		t.Fatalf("Failed to set HOME: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Setenv("HOME", originalHome); err != nil {
			t.Errorf("Failed to restore HOME: %v", err)
		}
	})

	tests := []struct {
		input          string
		expected       string
		checkPrefix    bool   // For relative paths, just check they became absolute
		expectedPrefix string // What prefix to check for relative paths
	}{
		{"~/file.txt", "/test/home/file.txt", false, ""},
		{"~/dir/subdir/file.txt", "/test/home/dir/subdir/file.txt", false, ""},
		{"/absolute/path", "/absolute/path", false, ""},
		{"relative/path", "", true, "/"}, // Relative paths get converted to absolute
		{"", "", true, "/"},              // Empty paths get converted to current dir (absolute)
		{"~", "/test/home", false, ""},
		{"~/", "/test/home", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := util.ExpandPath(tt.input)
			if err != nil {
				t.Fatalf("util.ExpandPath(%s) returned error: %v", tt.input, err)
			}
			if tt.checkPrefix {
				// For relative paths, just verify they became absolute
				if !filepath.IsAbs(result) {
					t.Errorf("util.ExpandPath(%s) = %s, expected absolute path", tt.input, result)
				}
			} else {
				if result != tt.expected {
					t.Errorf("util.ExpandPath(%s) = %s, expected %s", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestSyncConfigMarshalPRDFormat(t *testing.T) {
	// Create a full config with all fields populated
	config := mustDefaultConfig(t)
	
	// Set additional fields to ensure comprehensive testing
	config.Git.RemoteURL = "https://github.com/example/dotfiles.git"
	config.Git.Username = "testuser"
	config.Mappings = map[string]string{
		".bashrc": "/home/user/.bashrc",
		".vimrc":  "/home/user/.vimrc",
	}
	config.Sync.AutoCommit = false
	config.Notifications.ShowSuccess = true
	config.ConflictResolution.Strategy = "auto_keep_local"
	config.UI.MinimizeToTray = false
	config.Advanced.DebugLogging = true

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Parse the JSON to verify structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal marshaled JSON: %v", err)
	}

	// Verify machine is serialized as a string (PRD format)
	if machine, ok := parsed["machine"].(string); !ok {
		t.Errorf("Expected 'machine' to be a string in JSON, got %T", parsed["machine"])
	} else if machine != config.Machine.Name {
		t.Errorf("Expected machine name %q, got %q", config.Machine.Name, machine)
	}

	// Verify git section uses PRD field names
	if gitSection, ok := parsed["git"].(map[string]interface{}); !ok {
		t.Fatal("Expected 'git' to be an object in JSON")
	} else {
		// Check for PRD fields
		if _, hasRemote := gitSection["remote"]; !hasRemote && config.Git.RemoteName != "" {
			t.Error("Expected 'remote' field in git section for PRD format")
		}
		if _, hasUserName := gitSection["user_name"]; !hasUserName && config.Git.AuthorName != "" {
			t.Error("Expected 'user_name' field in git section for PRD format")
		}
		if _, hasUserEmail := gitSection["user_email"]; !hasUserEmail && config.Git.AuthorEmail != "" {
			t.Error("Expected 'user_email' field in git section for PRD format")
		}

		// Verify legacy fields are NOT present
		if _, hasRemoteName := gitSection["remote_name"]; hasRemoteName {
			t.Error("Legacy field 'remote_name' should not be present in PRD format output")
		}
		if _, hasAuthorName := gitSection["author_name"]; hasAuthorName {
			t.Error("Legacy field 'author_name' should not be present in PRD format output")
		}
		if _, hasAuthorEmail := gitSection["author_email"]; hasAuthorEmail {
			t.Error("Legacy field 'author_email' should not be present in PRD format output")
		}
	}

	// Verify required PRD fields are present
	requiredFields := []string{"version", "machine", "git", "sync", "notifications", "conflict_resolution", "ui", "advanced"}
	for _, field := range requiredFields {
		if _, exists := parsed[field]; !exists {
			t.Errorf("Required field %q missing from marshaled JSON", field)
		}
	}
}

func TestStartAtBootMigration(t *testing.T) {
	// Test 1: New config files (no explicit start_at_boot) should use new default (true)
	t.Run("new_config_uses_new_default", func(t *testing.T) {
		jsonData := `{
			"version": "1.0",
			"machine": "test-machine",
			"git": {
				"repo_path": "/tmp/dotfiles",
				"remote_url": "https://github.com/example/dotfiles.git",
				"remote": "origin",
				"branch": "main",
				"user_name": "Test User",
				"user_email": "test@example.com",
				"auth_type": "ssh"
			},
			"sync": {
				"auto_sync_enabled": true,
				"pull_interval_seconds": 300,
				"debounce_seconds": 30,
				"auto_commit": true,
				"auto_push": true,
				"auto_pull": true
			},
			"notifications": {
				"enabled": true,
				"show_success": false,
				"show_pulls": true,
				"play_sound_on_conflict": false
			},
			"conflict_resolution": {
				"strategy": "manual",
				"backup_dir": "/tmp/dotfiles/.backup",
				"keep_backups_days": 7
			},
			"ui": {
				"theme": "auto"
			},
			"advanced": {
				"debug_logging": false,
				"log_file": "/tmp/.dotfile-sync.log",
				"max_log_size_mb": 10
			}
		}`

		// Create temporary config file
		tmpFile := t.TempDir() + "/config.json"
		if err := os.WriteFile(tmpFile, []byte(jsonData), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		// Load the config
		config, err := LoadFromFile(tmpFile)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Should use new default (true) since no explicit start_at_boot was provided
		if !config.UI.StartAtBoot {
			t.Error("Expected StartAtBoot to be true (new default) for configs without explicit setting")
		}
	})

	// Test 2: Existing config files with explicit start_at_boot=false should preserve false
	t.Run("existing_config_preserves_false", func(t *testing.T) {
		jsonData := `{
			"version": "1.0",
			"machine": "test-machine",
			"git": {
				"repo_path": "/tmp/dotfiles",
				"remote_url": "https://github.com/example/dotfiles.git",
				"remote": "origin",
				"branch": "main",
				"user_name": "Test User",
				"user_email": "test@example.com",
				"auth_type": "ssh"
			},
			"sync": {
				"auto_sync_enabled": true,
				"pull_interval_seconds": 300,
				"debounce_seconds": 30,
				"auto_commit": true,
				"auto_push": true,
				"auto_pull": true
			},
			"notifications": {
				"enabled": true,
				"show_success": false,
				"show_pulls": true,
				"play_sound_on_conflict": false
			},
			"conflict_resolution": {
				"strategy": "manual",
				"backup_dir": "/tmp/dotfiles/.backup",
				"keep_backups_days": 7
			},
			"ui": {
				"start_at_boot": false,
				"theme": "auto"
			},
			"advanced": {
				"debug_logging": false,
				"log_file": "/tmp/.dotfile-sync.log",
				"max_log_size_mb": 10
			}
		}`

		// Create temporary config file
		tmpFile := t.TempDir() + "/config.json"
		if err := os.WriteFile(tmpFile, []byte(jsonData), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		// Load the config
		config, err := LoadFromFile(tmpFile)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Should preserve explicit false value
		if config.UI.StartAtBoot {
			t.Error("Expected StartAtBoot to be preserved as false from existing config")
		}
	})

	// Test 3: Existing config files with explicit start_at_boot=true should preserve true
	t.Run("existing_config_preserves_true", func(t *testing.T) {
		jsonData := `{
			"version": "1.0",
			"machine": "test-machine",
			"git": {
				"repo_path": "/tmp/dotfiles",
				"remote_url": "https://github.com/example/dotfiles.git",
				"remote": "origin",
				"branch": "main",
				"user_name": "Test User",
				"user_email": "test@example.com",
				"auth_type": "ssh"
			},
			"sync": {
				"auto_sync_enabled": true,
				"pull_interval_seconds": 300,
				"debounce_seconds": 30,
				"auto_commit": true,
				"auto_push": true,
				"auto_pull": true
			},
			"notifications": {
				"enabled": true,
				"show_success": false,
				"show_pulls": true,
				"play_sound_on_conflict": false
			},
			"conflict_resolution": {
				"strategy": "manual",
				"backup_dir": "/tmp/dotfiles/.backup",
				"keep_backups_days": 7
			},
			"ui": {
				"start_at_boot": true,
				"theme": "auto"
			},
			"advanced": {
				"debug_logging": false,
				"log_file": "/tmp/.dotfile-sync.log",
				"max_log_size_mb": 10
			}
		}`

		// Create temporary config file
		tmpFile := t.TempDir() + "/config.json"
		if err := os.WriteFile(tmpFile, []byte(jsonData), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		// Load the config
		config, err := LoadFromFile(tmpFile)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Should preserve explicit true value
		if !config.UI.StartAtBoot {
			t.Error("Expected StartAtBoot to be preserved as true from existing config")
		}
	})
}
