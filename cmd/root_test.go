package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

func TestGetConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test config file
	configPath := filepath.Join(tmpDir, ".sync-config.json")
	testConfig := `{
		"version": "1.0",
		"machine": {"name": "test-machine"},
		"git": {
			"repo_path": "/tmp/test-repo",
			"remote_url": "",
			"remote_name": "origin",
			"branch": "main",
			"author_name": "Test User",
			"author_email": "test@example.com",
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
			"backup_dir": "/tmp/backup",
			"keep_backups_days": 7
		},
		"mappings": {},
		"ui": {
			"start_at_boot": false,
			"minimize_to_tray": true,
			"theme": "auto"
		},
		"advanced": {
			"debug_logging": false,
			"log_file": "/tmp/test.log",
			"max_log_size_mb": 10
		}
	}`
	if err := os.WriteFile(configPath, []byte(testConfig), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	t.Run("loads config from explicit file", func(t *testing.T) {
		// Set the global configFile variable
		oldConfigFile := configFile
		configFile = configPath
		t.Cleanup(func() {
			configFile = oldConfigFile
		})

		cfg, err := getConfig()
		if err != nil {
			t.Fatalf("getConfig() returned error: %v", err)
		}

		if cfg.Machine.Name != "test-machine" {
			t.Errorf("Expected machine name 'test-machine', got '%s'", cfg.Machine.Name)
		}
		if cfg.Git.RepoPath != "/tmp/test-repo" {
			t.Errorf("Expected repo path '/tmp/test-repo', got '%s'", cfg.Git.RepoPath)
		}
	})

	t.Run("returns default config for nonexistent file", func(t *testing.T) {
		oldConfigFile := configFile
		configFile = filepath.Join(tmpDir, "nonexistent.json")
		t.Cleanup(func() {
			configFile = oldConfigFile
		})

		cfg, err := getConfig()
		// LoadFromFile returns default config if file doesn't exist (not an error)
		if err != nil {
			t.Errorf("Unexpected error for nonexistent config file: %v", err)
		}
		if cfg == nil {
			t.Error("Expected default config, got nil")
		}
		// Verify it's the default config
		if cfg.Version != config.CurrentVersion {
			t.Errorf("Expected default version, got %s", cfg.Version)
		}
	})

	t.Run("loads from default location", func(t *testing.T) {
		// Set up a mock home directory
		oldHome := os.Getenv("HOME")
		mockHome := tmpDir
		if err := os.Setenv("HOME", mockHome); err != nil {
			t.Fatalf("Failed to set HOME: %v", err)
		}
		t.Cleanup(func() {
			if err := os.Setenv("HOME", oldHome); err != nil {
				t.Errorf("Failed to restore HOME: %v", err)
			}
		})

		// Create dotfiles directory with config
		dotfilesDir := filepath.Join(mockHome, "dotfiles")
		if err := os.MkdirAll(dotfilesDir, 0755); err != nil {
			t.Fatalf("Failed to create dotfiles dir: %v", err)
		}
		defaultConfigPath := filepath.Join(dotfilesDir, ".sync-config.json")
		if err := os.WriteFile(defaultConfigPath, []byte(testConfig), 0644); err != nil {
			t.Fatalf("Failed to create default config: %v", err)
		}

		oldConfigFile := configFile
		configFile = ""
		t.Cleanup(func() {
			configFile = oldConfigFile
		})

		cfg, err := getConfig()
		if err != nil {
			t.Fatalf("getConfig() returned error: %v", err)
		}

		if cfg == nil {
			t.Fatal("Expected non-nil config")
		}
	})
}

func TestGetConfigWithInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	invalidConfigPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidConfigPath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("Failed to create invalid config: %v", err)
	}

	oldConfigFile := configFile
	configFile = invalidConfigPath
	t.Cleanup(func() {
		configFile = oldConfigFile
	})

	_, err := getConfig()
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestIsDaemonRunning(t *testing.T) {
	// Test stub function
	running := isDaemonRunning()
	if running {
		t.Error("Expected isDaemonRunning() stub to return false")
	}
}

func TestGetDaemonPID(t *testing.T) {
	// Test stub function
	pid, err := getDaemonPID()
	if err == nil {
		t.Error("Expected getDaemonPID() stub to return error")
	}
	if pid != 0 {
		t.Errorf("Expected PID 0, got %d", pid)
	}
}

func TestDefaultConstants(t *testing.T) {
	// Verify exported constants are available and have correct values
	if config.CurrentVersion != "1.0" {
		t.Errorf("Expected CurrentVersion '1.0', got '%s'", config.CurrentVersion)
	}
	if config.DefaultPullIntervalSeconds != 300 {
		t.Errorf("Expected DefaultPullIntervalSeconds 300, got %d", config.DefaultPullIntervalSeconds)
	}
	if config.DefaultDebounceSeconds != 30 {
		t.Errorf("Expected DefaultDebounceSeconds 30, got %d", config.DefaultDebounceSeconds)
	}
	if config.DefaultMaxLogSizeMB != 10 {
		t.Errorf("Expected DefaultMaxLogSizeMB 10, got %d", config.DefaultMaxLogSizeMB)
	}
	if config.DefaultKeepBackupsDays != 7 {
		t.Errorf("Expected DefaultKeepBackupsDays 7, got %d", config.DefaultKeepBackupsDays)
	}
}
