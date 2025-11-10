package scenarios

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// File permission constants for test operations
const (
	// dirPermissions represents standard directory permissions (rwxr-xr-x)
	// Directories need execute bits to allow traversal and listing
	dirPermissions = 0755

	// filePermissions represents standard file permissions (rw-r--r--)
	// Owner can read/write, group and others can read only
	filePermissions = 0644

	// sshKeyPermissions represents restrictive SSH key permissions (rw-------)
	// SSH keys must only be accessible by owner for security
	sshKeyPermissions = 0600
)

// Time constants for test operations
const (
	// defaultCommandTimeout is the standard timeout for DSM commands in tests
	// 30 seconds provides adequate time for most git operations in containers
	defaultCommandTimeout = 30 * time.Second

	// extendedCommandTimeout is for operations that may take longer
	// Used for complex operations like repository initialization
	extendedCommandTimeout = 60 * time.Second
)

// Path constants for test environment
const (
	// dsmBinaryPath is the absolute path to DSM binary in container (from Dockerfile.test:47)
	// Using absolute path avoids PATH dependency issues in containerized environments
	dsmBinaryPath = "/usr/local/bin/dsm"

	// testDataDir is the base directory for all test data
	testDataDir = "/app/test-data"

	// sshKeyPath is the path to the SSH key used for git operations
	sshKeyPath = "/app/ssh-keys/test_key"
)

// execCommandContext creates an exec.Cmd with proper environment setup for DSM commands.
// Uses absolute binary path to avoid PATH dependency issues in containerized test environments.
// Configures SSH environment for git operations in test containers.
func execCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	var cmd *exec.Cmd

	// Use absolute path for DSM binary to avoid PATH dependency in containers
	if name == "dsm" {
		cmd = exec.CommandContext(ctx, dsmBinaryPath, args...)
	} else {
		cmd = exec.CommandContext(ctx, name, args...)
	}

	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i %s", sshKeyPath),
	)

	return cmd
}

// execCommandContextWithConfig creates an exec.Cmd with custom configuration path support.
// Extends execCommandContext by adding DSM_CONFIG_PATH environment variable for test isolation.
// Ensures each test scenario can use its own configuration file without conflicts.
func execCommandContextWithConfig(ctx context.Context, configPath string, name string, args ...string) *exec.Cmd {
	var cmd *exec.Cmd

	// Use absolute path for DSM binary to avoid PATH dependency in containers
	if name == "dsm" {
		cmd = exec.CommandContext(ctx, dsmBinaryPath, args...)
	} else {
		cmd = exec.CommandContext(ctx, name, args...)
	}

	// Set environment variables
	envVars := []string{
		fmt.Sprintf("GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i %s", sshKeyPath),
	}

	if configPath != "" {
		envVars = append(envVars, fmt.Sprintf("DSM_CONFIG_PATH=%s", configPath))
	}

	cmd.Env = append(os.Environ(), envVars...)
	return cmd
}

// addFileToDSM adds a file to DSM using the default configuration.
// Creates a DSM command with standard timeout and executes it to add the specified file.
// Logs the command output for debugging and test verification purposes.
func addFileToDSM(t *testing.T, filepath string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	cmd := execCommandContext(ctx, "dsm", "add", filepath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Adding file should succeed")
	t.Logf("Added file %s: %s", filepath, string(output))
}

// addFileToDSMWithConfig adds a file to DSM using a custom configuration file.
// Ensures test isolation by using configuration-specific command execution.
// Useful for testing different DSM configurations without interference.
func addFileToDSMWithConfig(t *testing.T, configPath, filepath string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "add", filepath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Adding file should succeed")
	t.Logf("Added file %s: %s", filepath, string(output))
}

// syncChanges triggers a sync operation using the default DSM configuration.
// Executes DSM sync command with standard timeout to commit and push changes.
// Essential for testing the core sync functionality in isolation.
func syncChanges(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	cmd := execCommandContext(ctx, "dsm", "sync")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Sync should succeed")
	t.Logf("Sync output: %s", string(output))
}

// syncChangesWithConfig triggers a sync operation using a custom configuration file.
// Enables testing of sync behavior under different configuration scenarios.
// Maintains test isolation by using configuration-specific command execution.
func syncChangesWithConfig(t *testing.T, configPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "sync")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Sync should succeed")
	t.Logf("Sync output: %s", string(output))
}

// initDSMWithConfig initializes DSM with a custom configuration file and repository path.
// Sets up DSM for testing with isolated configuration to avoid conflicts with other tests.
// Uses extended timeout to accommodate repository initialization operations.
func initDSMWithConfig(t *testing.T, configPath, repoPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), extendedCommandTimeout)
	defer cancel()

	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "init", "--repo-path", repoPath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "DSM init should succeed")
	t.Logf("DSM init output: %s", string(output))
}

// createTestFile creates a test file with the specified content, creating parent directories as needed.
// Uses standard file and directory permissions for test environments.
// Essential utility for setting up test scenarios with required file structures.
func createTestFile(t *testing.T, filePath, content string) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(filePath, []byte(content), filePermissions); err != nil {
		t.Fatalf("Failed to write test file %s: %v", filePath, err)
	}
}

// createTestFileWithTemplate creates a test file using a format template with arguments.
// Convenience function for generating test files with dynamic content.
// Delegates to createTestFile for consistent file creation behavior.
func createTestFileWithTemplate(t *testing.T, filePath, template string, args ...interface{}) {
	content := fmt.Sprintf(template, args...)
	createTestFile(t, filePath, content)
}

// setupGitRepository initializes a git repository with standard test configuration.
// Creates the repository at the specified path and configures git user settings.
// Essential utility for DSM tests that require a git repository backend.
func setupGitRepository(t *testing.T, repoPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	// Initialize git repository
	cmd := execCommandContext(ctx, "git", "init", repoPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Git init output: %s", string(output))
		t.Logf("Git init error: %v", err)
	}
	require.NoError(t, err, "Git repository initialization should succeed")

	// Configure git user
	cmd = execCommandContext(ctx, "git", "-C", repoPath, "config", "user.name", "Test User")
	err = cmd.Run()
	require.NoError(t, err, "Git user config should succeed")

	cmd = execCommandContext(ctx, "git", "-C", repoPath, "config", "user.email", "test@example.com")
	err = cmd.Run()
	require.NoError(t, err, "Git email config should succeed")

	t.Logf("✅ Git repository initialized at: %s", repoPath)
}

// setupTestDirectories creates standard test directories for DSM tests.
// Creates source and target directories under the test data directory.
// Returns paths for both directories to use in test scenarios.
func setupTestDirectories(t *testing.T) (sourceDir, targetDir string) {
	sourceDir = filepath.Join(testDataDir, "source_dotfiles")
	targetDir = filepath.Join(testDataDir, "dotfiles-test")

	err := os.MkdirAll(sourceDir, dirPermissions)
	require.NoError(t, err, "Failed to create source directory")

	err = os.MkdirAll(targetDir, dirPermissions)
	require.NoError(t, err, "Failed to create target directory")

	return sourceDir, targetDir
}