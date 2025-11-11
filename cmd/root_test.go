package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

// createTestConfig creates a test configuration based on DefaultConfig with test-specific overrides.
// This is more maintainable than hardcoded JSON as it adapts to config schema changes.
func createTestConfig(t *testing.T) *config.SyncConfig {
	t.Helper()
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create default config: %v", err)
	}

	// Override with test-specific values
	cfg.Machine.Name = "test-machine"
	cfg.Git.RepoPath = "/tmp/test-repo"
	cfg.Git.AuthorName = "Test User"
	cfg.Git.AuthorEmail = "test@example.com"
	cfg.ConflictResolution.BackupDir = "/tmp/backup"
	cfg.Advanced.LogFile = "/tmp/test.log"

	return cfg
}

func TestGetConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test config file using helper
	testCfg := createTestConfig(t)
	configPath := filepath.Join(tmpDir, ".sync-config.json")
	if err := testCfg.SaveToFile(configPath); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	t.Run("loads config from explicit file", func(t *testing.T) {
		// Set the global configFile variable
		oldConfigFile := getConfigFile()
		setConfigFile(configPath)
		t.Cleanup(func() {
			setConfigFile(oldConfigFile)
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

	t.Run("returns error for nonexistent explicit config file", func(t *testing.T) {
		oldConfigFile := getConfigFile()
		setConfigFile(filepath.Join(tmpDir, "nonexistent.json"))
		t.Cleanup(func() {
			setConfigFile(oldConfigFile)
		})

		cfg, err := getConfig()
		// With explicit config file, should fail fast if file doesn't exist
		if err == nil {
			t.Error("Expected error for nonexistent explicit config file, got nil")
		}
		if cfg != nil {
			t.Error("Expected nil config on error, got non-nil")
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
		if err := os.MkdirAll(dotfilesDir, dirPerms); err != nil {
			t.Fatalf("Failed to create dotfiles dir: %v", err)
		}
		defaultConfigPath := filepath.Join(dotfilesDir, ".sync-config.json")
		testCfg := createTestConfig(t)
		if err := testCfg.SaveToFile(defaultConfigPath); err != nil {
			t.Fatalf("Failed to create default config: %v", err)
		}

		oldConfigFile := getConfigFile()
		setConfigFile("")
		t.Cleanup(func() {
			setConfigFile(oldConfigFile)
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

	oldConfigFile := getConfigFile()
	setConfigFile(invalidConfigPath)
	t.Cleanup(func() {
		setConfigFile(oldConfigFile)
	})

	_, err := getConfig()
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
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
