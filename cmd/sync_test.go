package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRepo creates a git repository with initial commit for testing
func setupTestRepo(t *testing.T, path string) {
	t.Helper()

	// Create directory
	err := os.MkdirAll(path, 0755)
	require.NoError(t, err)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = path
	err = cmd.Run()
	require.NoError(t, err)

	// Create initial file
	initialFile := filepath.Join(path, ".bashrc")
	err = os.WriteFile(initialFile, []byte("initial content"), 0644)
	require.NoError(t, err)

	// Configure git user
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = path
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = path
	_ = cmd.Run()

	// Add and commit
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = path
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = path
	_ = cmd.Run()
}

func TestSyncCmd_DryRunShowsFileOperationTypes(t *testing.T) {
	// Setup: Create temp directory with git repo
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	// Initialize git repo and create test config
	setupTestRepo(t, repoPath)

	// Create config file pointing to test repo
	configPath := filepath.Join(repoPath, ".sync-config.json")
	testConfig, err := config.DefaultConfig()
	require.NoError(t, err)

	// Override with test values
	testConfig.Git.RepoPath = repoPath
	testConfig.Git.AuthorName = "Test User"
	testConfig.Git.AuthorEmail = "test@example.com"
	testConfig.Git.AuthType = "none" // No auth needed for local test repo
	testConfig.ConfigPath = configPath

	err = testConfig.SaveToFile(configPath)
	require.NoError(t, err)

	// Set config file for this test
	oldConfigFile := getConfigFile()
	setConfigFile(configPath)
	t.Cleanup(func() {
		setConfigFile(oldConfigFile)
	})

	// Create files in different states
	addedFile := filepath.Join(repoPath, "new-file.txt")
	modifiedFile := filepath.Join(repoPath, ".bashrc")

	// Add new file (untracked)
	err = os.WriteFile(addedFile, []byte("new content"), 0644)
	require.NoError(t, err)

	// Modify existing file
	err = os.WriteFile(modifiedFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Setup dry-run mode with cleanup
	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() {
		globalDryRun = oldDryRun
	})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	// Read output in goroutine
	outputChan := make(chan string, 1)
	go func() {
		defer close(outputChan)
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outputChan <- buf.String()
	}()

	// Execute sync command
	cmd := &cobra.Command{}
	err = runSync(cmd, []string{})

	// Close writer and get output
	_ = w.Close()
	output := <-outputChan

	// Assertions: Should show operation types
	assert.NoError(t, err)
	assert.Contains(t, output, "Would add:", "Should show added files")
	assert.Contains(t, output, "new-file.txt", "Should list added file")
	assert.Contains(t, output, "Would modify:", "Should show modified files")
	assert.Contains(t, output, ".bashrc", "Should list modified file")
}
