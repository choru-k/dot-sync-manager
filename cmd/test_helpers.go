package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

// Test constants for consistent testing across all CLI command tests
const (
	// testFilePerms allows owner read/write access with group/others read
	testFilePerms = 0644 // -rw-r--r--

	// testDirPerms grants owner full access with group/others read/execute
	testDirPerms = 0755 // -rwxr-xr-x

	// testRepoName is used for test repositories
	testRepoName = "test-dotfiles"

	// testMachineName is used for test configurations
	testMachineName = "test-machine"

	// testAuthorName and testAuthorEmail are used for git configurations
	testAuthorName  = "Test User"
	testAuthorEmail = "test@example.com"
)

// TestConfig represents a test configuration setup
type TestConfig struct {
	HomeDir    string
	RepoPath   string
	ConfigPath  string
	Config     *config.SyncConfig
}

// setupTestEnvironment creates a temporary test environment with proper configuration
func setupTestEnvironment(t *testing.T) *TestConfig {
	t.Helper()

	// Create temporary home directory
	homeDir := t.TempDir()
	if err := os.MkdirAll(homeDir, testDirPerms); err != nil {
		t.Fatalf("failed to create test home directory: %v", err)
	}

	// Set HOME environment variable for the test
	t.Setenv("HOME", homeDir)

	// Create repository directory
	repoPath := filepath.Join(homeDir, testRepoName)
	if err := os.MkdirAll(repoPath, testDirPerms); err != nil {
		t.Fatalf("failed to create test repo directory: %v", err)
	}

	// Create default configuration
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}

	// Configure test-specific values
	cfg.Machine.Name = testMachineName
	cfg.Git.RepoPath = repoPath
	cfg.Git.AuthorName = testAuthorName
	cfg.Git.AuthorEmail = testAuthorEmail
	cfg.ConflictResolution.BackupDir = filepath.Join(repoPath, ".backup")
	cfg.Mappings = make(map[string]string)
	cfg.ConfigPath = filepath.Join(repoPath, ".sync-config.json")

	// Save configuration to file
	if err := cfg.SaveToFile(cfg.ConfigPath); err != nil {
		t.Fatalf("failed to save test config: %v", err)
	}

	return &TestConfig{
		HomeDir:    homeDir,
		RepoPath:   repoPath,
		ConfigPath:  cfg.ConfigPath,
		Config:     cfg,
	}
}

// createTestFile creates a test file with specified content
func createTestFile(t *testing.T, path string, content string) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, testDirPerms); err != nil {
		t.Fatalf("failed to create directory for test file: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), testFilePerms); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
}

// createTestSymlink creates a test symlink
func createTestSymlink(t *testing.T, target, link string) {
	t.Helper()

	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation not supported on Windows: %v", err)
		}
		t.Fatalf("failed to create test symlink: %v", err)
	}
}

// requireSymlinkSupport skips tests on platforms that don't support symlinks
func requireSymlinkSupport(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src")
	dst := filepath.Join(tempDir, "dst")

	// Create source file
	if err := os.WriteFile(src, []byte("test"), testFilePerms); err != nil {
		t.Fatalf("failed to create test source file: %v", err)
	}

	// Test symlink creation
	if err := os.Symlink(src, dst); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation not supported on Windows: %v", err)
		}
		t.Fatalf("symlink support required for this test: %v", err)
	}

	// Cleanup
	if err := os.Remove(dst); err != nil {
		t.Fatalf("failed to cleanup test symlink: %v", err)
	}
}


// assertFileExists checks if a file exists and fails the test if it doesn't
func assertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file to exist: %s", path)
	}
}

// assertFileNotExists checks if a file doesn't exist and fails the test if it does
func assertFileNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected file to not exist: %s", path)
	}
}


// cleanupTestEnvironment cleans up a test environment
func cleanupTestEnvironment(config *TestConfig) {
	// No explicit cleanup needed when using t.TempDir()
	// This function exists for compatibility with existing test code
}