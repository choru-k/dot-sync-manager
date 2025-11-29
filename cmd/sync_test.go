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

func TestSyncCmd_DryRunShowsCommitMessage(t *testing.T) {
	// Setup test repo
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)

	// Create config file
	configPath := filepath.Join(repoPath, ".sync-config.json")
	testConfig, err := config.DefaultConfig()
	require.NoError(t, err)

	testConfig.Git.RepoPath = repoPath
	testConfig.Git.AuthorName = "Test User"
	testConfig.Git.AuthorEmail = "test@example.com"
	testConfig.Git.AuthType = "none"
	testConfig.ConfigPath = configPath

	err = testConfig.SaveToFile(configPath)
	require.NoError(t, err)

	// Set config file for this test
	oldConfigFile := getConfigFile()
	setConfigFile(configPath)
	t.Cleanup(func() {
		setConfigFile(oldConfigFile)
	})

	// Modify a file
	testFile := filepath.Join(repoPath, ".bashrc")
	err = os.WriteFile(testFile, []byte("modified"), 0644)
	require.NoError(t, err)

	// Setup dry-run mode
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

	outputChan := make(chan string, 1)
	go func() {
		defer close(outputChan)
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outputChan <- buf.String()
	}()

	// Execute sync
	cmd := &cobra.Command{}
	err = runSync(cmd, []string{})

	_ = w.Close()
	output := <-outputChan

	// Assertions
	assert.NoError(t, err)
	assert.Contains(t, output, "Would create commit:", "Should preview commit")
	assert.Contains(t, output, "Auto-sync:", "Should show commit message format")
	assert.Contains(t, output, ".bashrc", "Should list changed file in message")
}

func TestSyncCmd_DryRunShowsStatistics(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)

	// Create config file
	configPath := filepath.Join(repoPath, ".sync-config.json")
	testConfig, err := config.DefaultConfig()
	require.NoError(t, err)

	testConfig.Git.RepoPath = repoPath
	testConfig.Git.AuthorName = "Test User"
	testConfig.Git.AuthorEmail = "test@example.com"
	testConfig.Git.AuthType = "none"
	testConfig.ConfigPath = configPath

	err = testConfig.SaveToFile(configPath)
	require.NoError(t, err)

	// Set config file for this test
	oldConfigFile := getConfigFile()
	setConfigFile(configPath)
	t.Cleanup(func() {
		setConfigFile(oldConfigFile)
	})

	// Create multiple files of different types
	addedFile1 := filepath.Join(repoPath, "new1.txt")
	addedFile2 := filepath.Join(repoPath, "new2.txt")
	modifiedFile := filepath.Join(repoPath, ".bashrc")

	require.NoError(t, os.WriteFile(addedFile1, []byte("content"), 0644))
	require.NoError(t, os.WriteFile(addedFile2, []byte("content"), 0644))
	require.NoError(t, os.WriteFile(modifiedFile, []byte("modified"), 0644))

	// Setup dry-run
	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() { globalDryRun = oldDryRun })

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	outputChan := make(chan string, 1)
	go func() {
		defer close(outputChan)
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outputChan <- buf.String()
	}()

	cmd := &cobra.Command{}
	err = runSync(cmd, []string{})

	_ = w.Close()
	output := <-outputChan

	// Assertions: Should show summary statistics
	assert.NoError(t, err)
	assert.Contains(t, output, "Summary:", "Should show summary section")
	assert.Contains(t, output, "4 files", "Should show total file count")
	assert.Contains(t, output, "3 added", "Should show added count")
	assert.Contains(t, output, "1 modified", "Should show modified count")
}

func TestSyncCmd_DryRunNoFilesystemChanges(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)

	// Create config file
	configPath := filepath.Join(repoPath, ".sync-config.json")
	testConfig, err := config.DefaultConfig()
	require.NoError(t, err)

	testConfig.Git.RepoPath = repoPath
	testConfig.Git.AuthorName = "Test User"
	testConfig.Git.AuthorEmail = "test@example.com"
	testConfig.Git.AuthType = "none"
	testConfig.ConfigPath = configPath

	err = testConfig.SaveToFile(configPath)
	require.NoError(t, err)

	// Set config file for this test
	oldConfigFile := getConfigFile()
	setConfigFile(configPath)
	t.Cleanup(func() {
		setConfigFile(oldConfigFile)
	})

	// Create a modified file
	testFile := filepath.Join(repoPath, ".bashrc")
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Record file modification times before dry-run
	beforeStat, err := os.Stat(testFile)
	require.NoError(t, err)
	beforeModTime := beforeStat.ModTime()

	// Get initial commit count
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	require.NoError(t, err)
	initialCommitCount := string(output)

	// Setup dry-run mode
	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() { globalDryRun = oldDryRun })

	// Suppress output for this test
	oldStdout := os.Stdout
	os.Stdout = nil
	t.Cleanup(func() { os.Stdout = oldStdout })

	// Execute sync
	cobraCmd := &cobra.Command{}
	err = runSync(cobraCmd, []string{})

	// Restore stdout
	os.Stdout = oldStdout

	// Assertions: No filesystem changes
	assert.NoError(t, err)

	// Verify file modification time unchanged
	afterStat, err := os.Stat(testFile)
	require.NoError(t, err)
	afterModTime := afterStat.ModTime()
	assert.Equal(t, beforeModTime, afterModTime, "File modification time should not change")

	// Verify no new commits created
	cmd = exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repoPath
	output, err = cmd.Output()
	require.NoError(t, err)
	finalCommitCount := string(output)
	assert.Equal(t, initialCommitCount, finalCommitCount, "No new commits should be created")
}

func TestSyncCmd_DryRunHandlesCleanRepository(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)

	// Create config file
	configPath := filepath.Join(repoPath, ".sync-config.json")
	testConfig, err := config.DefaultConfig()
	require.NoError(t, err)

	testConfig.Git.RepoPath = repoPath
	testConfig.Git.AuthorName = "Test User"
	testConfig.Git.AuthorEmail = "test@example.com"
	testConfig.Git.AuthType = "none"
	testConfig.ConfigPath = configPath

	err = testConfig.SaveToFile(configPath)
	require.NoError(t, err)

	// Commit the config file to ensure clean repository
	gitCmd := exec.Command("git", "add", ".sync-config.json")
	gitCmd.Dir = repoPath
	require.NoError(t, gitCmd.Run())

	gitCmd = exec.Command("git", "commit", "-m", "Add config")
	gitCmd.Dir = repoPath
	require.NoError(t, gitCmd.Run())

	// Set config file for this test
	oldConfigFile := getConfigFile()
	setConfigFile(configPath)
	t.Cleanup(func() {
		setConfigFile(oldConfigFile)
	})

	// Don't create any changes - repository is clean

	// Setup dry-run mode
	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() { globalDryRun = oldDryRun })

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	outputChan := make(chan string, 1)
	go func() {
		defer close(outputChan)
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outputChan <- buf.String()
	}()

	// Execute sync
	cmd := &cobra.Command{}
	err = runSync(cmd, []string{})

	_ = w.Close()
	output := <-outputChan

	// Assertions: Should handle clean repo gracefully
	assert.NoError(t, err, "Should not return error for clean repo")
	assert.Contains(t, output, "No changes to sync", "Should show no changes message")
}
