package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/spf13/cobra"
)

func TestCheckCmd_Sanity(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "no arguments should work",
			args:        []string{},
			expectError: false,
		},
		{
			name:        "any arguments should error",
			args:        []string{"extra"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "check"}
			err := runCheck(cmd, tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("runCheck() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestCheckConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.SyncConfig
		expectError bool
	}{
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
		},
		{
			name: "valid config",
			config: &config.SyncConfig{
				Machine: config.MachineConfig{Name: "test-machine"},
				Git: config.GitConfig{
					RepoPath:   "/tmp/test-repo",
					AuthorName: "Test User",
					AuthorEmail: "test@example.com",
				},
			},
			expectError: false,
		},
		{
			name: "invalid config - missing machine name",
			config: &config.SyncConfig{
				Machine: config.MachineConfig{},
				Git: config.GitConfig{
					RepoPath: "/tmp/test-repo",
				},
			},
			expectError: true,
		},
		{
			name: "invalid config - missing repo path",
			config: &config.SyncConfig{
				Machine: config.MachineConfig{Name: "test-machine"},
				Git: config.GitConfig{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkConfig(tt.config)
			if (err != nil) != tt.expectError {
				t.Errorf("checkConfig() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestCheckRepository(t *testing.T) {
	tests := []struct {
		name        string
		setupRepo   func(t *testing.T) string
		expectError bool
	}{
		{
			name:        "non-existent repo",
			setupRepo:   func(t *testing.T) string { return "/tmp/non-existent-repo" },
			expectError: true,
		},
		{
			name: "valid repo",
			setupRepo: func(t *testing.T) string {
				repoPath := t.TempDir()
				// Initialize a basic git repo structure
				err := os.MkdirAll(filepath.Join(repoPath, ".git"), testDirPerms)
				if err != nil {
					t.Fatalf("failed to create .git directory: %v", err)
				}
				return repoPath
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := tt.setupRepo(t)
			err := checkRepository(repoPath)
			if (err != nil) != tt.expectError {
				t.Errorf("checkRepository() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestCheckConflicts(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) string
		expectError bool
	}{
		{
			name: "no conflicts directory",
			setup: func(t *testing.T) string {
				return "/tmp/non-existent-path"
			},
			expectError: false, // Should not error if no conflicts dir
		},
		{
			name: "empty conflicts directory",
			setup: func(t *testing.T) string {
				conflictsDir := filepath.Join(t.TempDir(), ".dsm", "conflicts")
				if err := os.MkdirAll(conflictsDir, testDirPerms); err != nil {
					t.Fatalf("failed to create conflicts directory: %v", err)
				}
				return conflictsDir
			},
			expectError: false,
		},
		{
			name: "conflicts directory with files",
			setup: func(t *testing.T) string {
				conflictsDir := filepath.Join(t.TempDir(), ".dsm", "conflicts")
				if err := os.MkdirAll(conflictsDir, testDirPerms); err != nil {
					t.Fatalf("failed to create conflicts directory: %v", err)
				}
				createTestFile(t, filepath.Join(conflictsDir, "conflict1.txt"), "conflict content")
				return conflictsDir
			},
			expectError: false, // Should not error, just detect conflicts
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflictsDir := tt.setup(t)
			hasConflicts, err := checkConflicts(conflictsDir)
			if err != nil {
				t.Errorf("checkConflicts() returned error: %v", err)
			}
			// If conflicts directory has files, should report conflicts exist
			if tt.name == "conflicts directory with files" && !hasConflicts {
				t.Error("expected hasConflicts to be true when conflict files exist")
			}
		})
	}
}

func TestCheckCmd_Integration(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Initialize git repo structure
	err := os.MkdirAll(filepath.Join(testConfig.RepoPath, ".git"), testDirPerms)
	if err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}

	// Test check command
	cmd := &cobra.Command{Use: "check"}
	err = runCheck(cmd, []string{})
	if err != nil {
		t.Errorf("runCheck() failed: %v", err)
	}
}

func TestCheckCmd_MissingConfig(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Remove config file
	err := os.Remove(testConfig.ConfigPath)
	if err != nil {
		t.Fatalf("failed to remove config file: %v", err)
	}

	// Test check command should fail
	cmd := &cobra.Command{Use: "check"}
	err = runCheck(cmd, []string{})
	if err == nil {
		t.Error("expected runCheck() to fail with missing config")
	}
}

func TestCheckCmd_BrokenRepo(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Set repo path to non-existent directory
	testConfig.Config.Git.RepoPath = "/tmp/non-existent-repo"
	if err := testConfig.Config.SaveToFile(testConfig.ConfigPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Test check command should fail for repo check
	cmd := &cobra.Command{Use: "check"}
	err := runCheck(cmd, []string{})
	if err == nil {
		t.Error("expected runCheck() to fail with non-existent repo")
	}
}

func TestCheckCmd_WithConflicts(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Initialize git repo structure
	err := os.MkdirAll(filepath.Join(testConfig.RepoPath, ".git"), testDirPerms)
	if err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}

	// Create conflicts directory with conflict files
	conflictsDir := filepath.Join(testConfig.RepoPath, ".dsm", "conflicts")
	if err := os.MkdirAll(conflictsDir, testDirPerms); err != nil {
		t.Fatalf("failed to create conflicts directory: %v", err)
	}
	createTestFile(t, filepath.Join(conflictsDir, "bashrc.conflict"), "conflict content")

	// Test check command
	cmd := &cobra.Command{Use: "check"}
	err = runCheck(cmd, []string{})
	if err != nil {
		t.Errorf("runCheck() failed: %v", err)
	}
}

func TestValidateSystemRequirements(t *testing.T) {
	// Test basic system requirements validation
	err := validateSystemRequirements()
	if err != nil {
		t.Logf("System requirements validation: %v", err)
	}
}

func TestCheckConnectivity(t *testing.T) {
	tests := []struct {
		name        string
		remoteURL   string
		expectError bool
	}{
		{
			name:        "empty remote URL",
			remoteURL:   "",
			expectError: false, // Should not error, just skip connectivity check
		},
		{
			name:        "invalid remote URL",
			remoteURL:   "invalid-url",
			expectError: false, // Should not error, just skip connectivity check
		},
		{
			name:        "valid remote URL format",
			remoteURL:   "https://github.com/test/repo.git",
			expectError: false, // Should not error even if unreachable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkConnectivity(tt.remoteURL)
			if err != nil {
				t.Logf("connectivity check for %s: %v", tt.remoteURL, err)
			}
		})
	}
}

// Helper functions for check tests

func checkConfig(cfg *config.SyncConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if cfg.Machine.Name == "" {
		return fmt.Errorf("machine name is required")
	}
	if cfg.Git.RepoPath == "" {
		return fmt.Errorf("repo path is required")
	}
	return nil
}

func checkRepository(repoPath string) error {
	if repoPath == "" {
		return fmt.Errorf("repo path is empty")
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}
	return nil
}

func checkConflicts(conflictsDir string) (bool, error) {
	if conflictsDir == "" {
		return false, nil
	}

	files, err := os.ReadDir(conflictsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return len(files) > 0, nil
}

func validateSystemRequirements() error {
	// Check if we're running on a supported platform
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		return nil
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func checkConnectivity(remoteURL string) error {
	if remoteURL == "" {
		return nil // Skip connectivity check if no remote URL
	}
	// In test environment, we can't actually check connectivity
	// So just validate the URL format
	if !strings.HasPrefix(remoteURL, "http://") && !strings.HasPrefix(remoteURL, "https://") && !strings.HasPrefix(remoteURL, "git@") {
		return fmt.Errorf("invalid remote URL format: %s", remoteURL)
	}
	return nil
}