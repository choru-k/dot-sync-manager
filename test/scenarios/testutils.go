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

// dsmBinaryPath - absolute path to DSM binary in container (from Dockerfile.test:47)
const dsmBinaryPath = "/usr/local/bin/dsm"

// execCommandContext creates an exec.Cmd context for DSM commands
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
		"GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i /app/ssh-keys/test_key",
	)

	return cmd
}

// execCommandContextWithConfig creates an exec.Cmd context for DSM commands with custom config path
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
		"GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i /app/ssh-keys/test_key",
	}

	if configPath != "" {
		envVars = append(envVars, fmt.Sprintf("DSM_CONFIG_PATH=%s", configPath))
	}

	cmd.Env = append(os.Environ(), envVars...)
	return cmd
}

// addFileToDSM adds a file to DSM
func addFileToDSM(t *testing.T, filepath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := execCommandContext(ctx, "dsm", "add", filepath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Adding file should succeed")
	t.Logf("Added file %s: %s", filepath, string(output))
}

// addFileToDSMWithConfig adds a file to DSM with custom config
func addFileToDSMWithConfig(t *testing.T, configPath, filepath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "add", filepath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Adding file should succeed")
	t.Logf("Added file %s: %s", filepath, string(output))
}

// syncChanges triggers a sync operation
func syncChanges(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := execCommandContext(ctx, "dsm", "sync")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Sync should succeed")
	t.Logf("Sync output: %s", string(output))
}

// syncChangesWithConfig triggers a sync operation with custom config
func syncChangesWithConfig(t *testing.T, configPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "sync")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Sync should succeed")
	t.Logf("Sync output: %s", string(output))
}

// initDSMWithConfig initializes DSM with custom config
func initDSMWithConfig(t *testing.T, configPath, repoPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "init", "--repo-path", repoPath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "DSM init should succeed")
	t.Logf("DSM init output: %s", string(output))
}

// createTestFile creates a test file with specified content
func createTestFile(t *testing.T, filePath, content string) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file %s: %v", filePath, err)
	}
}

// createTestFileWithTemplate creates a test file using a template format
func createTestFileWithTemplate(t *testing.T, filePath, template string, args ...interface{}) {
	content := fmt.Sprintf(template, args...)
	createTestFile(t, filePath, content)
}