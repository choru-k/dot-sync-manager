package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	err := os.MkdirAll(path, testDirPerms)
	require.NoError(t, err)

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = path
	require.NoError(t, cmd.Run(), "git init failed")

	// Configure git user
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = path
	require.NoError(t, cmd.Run(), "git config user.name failed")

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = path
	require.NoError(t, cmd.Run(), "git config user.email failed")

	// Create initial file and commit
	initialFile := filepath.Join(path, ".bashrc")
	err = os.WriteFile(initialFile, []byte("initial content"), testFilePerms)
	require.NoError(t, err)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = path
	require.NoError(t, cmd.Run(), "git add failed")

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = path
	require.NoError(t, cmd.Run(), "git commit failed")
}

// setupTestConfig creates and saves a test configuration with the given repository path and optional remote URL
func setupTestConfig(t *testing.T, repoPath, remoteURL string) string {
	t.Helper()

	configPath := filepath.Join(repoPath, ".sync-config.json")
	testConfig, err := config.DefaultConfig()
	require.NoError(t, err)

	// Override with test values
	testConfig.Git.RepoPath = repoPath
	testConfig.Git.AuthorName = "Test User"
	testConfig.Git.AuthorEmail = "test@example.com"
	testConfig.Git.AuthType = "none" // No auth needed for local test repo
	testConfig.Git.RemoteURL = remoteURL
	testConfig.ConfigPath = configPath

	require.NoError(t, testConfig.SaveToFile(configPath))

	// Set config file for this test
	oldConfigFile := getConfigFile()
	setConfigFile(configPath)
	t.Cleanup(func() {
		setConfigFile(oldConfigFile)
	})

	return configPath
}

func TestSyncCmd_DryRunShowsFileOperationTypes(t *testing.T) {
	// Setup: Create temp directory with git repo
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	// Initialize git repo and create test config
	setupTestRepo(t, repoPath)
	setupTestConfig(t, repoPath, "")

	// Create files in different states
	addedFile := filepath.Join(repoPath, "new-file.txt")
	modifiedFile := filepath.Join(repoPath, ".bashrc")

	// Add new file (untracked)
	err := os.WriteFile(addedFile, []byte("new content"), testFilePerms)
	require.NoError(t, err)

	// Modify existing file
	err = os.WriteFile(modifiedFile, []byte("modified content"), testFilePerms)
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
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)
	setupTestConfig(t, repoPath, "")

	// Modify a file to have changes to commit
	testFile := filepath.Join(repoPath, ".bashrc")
	err := os.WriteFile(testFile, []byte("changed"), testFilePerms)
	require.NoError(t, err)

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
	setupTestConfig(t, repoPath, "")

	// Create multiple files of different types
	addedFile1 := filepath.Join(repoPath, "new1.txt")
	addedFile2 := filepath.Join(repoPath, "new2.txt")
	modifiedFile := filepath.Join(repoPath, ".bashrc")

	require.NoError(t, os.WriteFile(addedFile1, []byte("content"), testFilePerms))
	require.NoError(t, os.WriteFile(addedFile2, []byte("content"), testFilePerms))
	require.NoError(t, os.WriteFile(modifiedFile, []byte("modified"), testFilePerms))

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
	err := runSync(cmd, []string{})

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
	setupTestConfig(t, repoPath, "")

	// Create a test file and record its modification time
	testFile := filepath.Join(repoPath, ".bashrc")
	err := os.WriteFile(testFile, []byte("modified content"), testFilePerms)
	require.NoError(t, err)

	beforeStat, err := os.Stat(testFile)
	require.NoError(t, err)

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
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	require.NoError(t, err)
	os.Stdout = devNull
	t.Cleanup(func() {
		if err := devNull.Close(); err != nil {
			t.Logf("Warning: failed to close devNull: %v", err)
		}
		os.Stdout = oldStdout
	})

	// Execute sync
	cobraCmd := &cobra.Command{}
	err = runSync(cobraCmd, []string{})

	// Assertions: No filesystem changes
	assert.NoError(t, err)

	// Verify file modification time unchanged
	afterStat, err := os.Stat(testFile)
	require.NoError(t, err)
	assert.Equal(t, beforeStat.ModTime(), afterStat.ModTime(), "File should not be modified during dry-run")

	// Verify commit count unchanged
	cmd = exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repoPath
	output, err = cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, initialCommitCount, string(output), "No new commits should be created in dry-run")

	// Verify staging area unchanged
	cmd = exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = repoPath
	err = cmd.Run()
	assert.NoError(t, err, "Staging area should remain clean")
}

func TestSyncCmd_DryRunHandlesCleanRepository(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)
	setupTestConfig(t, repoPath, "")

	// Commit the config file to ensure clean repository
	gitCmd := exec.Command("git", "add", ".sync-config.json")
	gitCmd.Dir = repoPath
	require.NoError(t, gitCmd.Run())

	gitCmd = exec.Command("git", "commit", "-m", "Add config")
	gitCmd.Dir = repoPath
	require.NoError(t, gitCmd.Run())

	// Don't create any changes - repository is clean

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

	// Execute sync
	cmd := &cobra.Command{}
	err := runSync(cmd, []string{})

	_ = w.Close()
	output := <-outputChan

	// Assertions: Should handle clean repo gracefully
	assert.NoError(t, err, "Should not return error for clean repo")
	assert.Contains(t, output, "No changes to sync", "Should show no changes message")
}

func TestSyncCmd_DryRunShowsRemotePushDetails(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)
	setupTestConfig(t, repoPath, "https://github.com/test/repo.git")

	// Create a modified file
	testFile := filepath.Join(repoPath, ".bashrc")
	err := os.WriteFile(testFile, []byte("modified"), testFilePerms)
	require.NoError(t, err)

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

	// Assertions: Should show push details
	assert.NoError(t, err)
	assert.Contains(t, output, "Would push to remote repository", "Should show push message")
	assert.Contains(t, output, "https://github.com/test/repo.git", "Should show remote URL")
}

func TestSyncCmd_DryRunHandlesNoRemote(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)
	setupTestConfig(t, repoPath, "") // No remote configured

	// Create a modified file
	testFile := filepath.Join(repoPath, ".bashrc")
	err := os.WriteFile(testFile, []byte("modified"), testFilePerms)
	require.NoError(t, err)

	// Setup dry-run mode
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

	// Execute sync
	cmd := &cobra.Command{}
	err = runSync(cmd, []string{})

	_ = w.Close()
	output := <-outputChan

	// Assertions: Should indicate no remote
	assert.NoError(t, err)
	assert.NotContains(t, output, "Would push", "Should not show push message when no remote")
	assert.Contains(t, output, "Would modify", "Should still show file operations")
}

func TestSyncCmd_DryRunCategorizesUntrackedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)
	setupTestConfig(t, repoPath, "")

	// Create an untracked file (not in git yet)
	untrackedFile := filepath.Join(repoPath, "new-untracked.txt")
	err := os.WriteFile(untrackedFile, []byte("untracked content"), testFilePerms)
	require.NoError(t, err)

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

	// Assertions: Untracked files should be in "added" category
	assert.NoError(t, err)
	assert.Contains(t, output, "Would add:", "Should have added section")
	assert.Contains(t, output, "new-untracked.txt", "Should list untracked file")

	// Verify NOT in modified section (this was the bug from Gemini review)
	addedSection := output[strings.Index(output, "Would add:"):]
	if modIndex := strings.Index(output, "Would modify:"); modIndex != -1 {
		modifiedSection := output[modIndex:]
		if commitIndex := strings.Index(modifiedSection, "Would create commit:"); commitIndex != -1 {
			modifiedSection = modifiedSection[:commitIndex]
		}
		assert.NotContains(t, modifiedSection, "new-untracked.txt",
			"Untracked file should NOT be in modified section")
	}
	assert.Contains(t, addedSection, "new-untracked.txt", "Untracked file should be in added section")
}

func TestSyncCmd_DryRunRespectsGitignore(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)

	// Create .gitignore file
	gitignorePath := filepath.Join(repoPath, ".gitignore")
	gitignoreContent := "*.log\ntemp/\n"
	err := os.WriteFile(gitignorePath, []byte(gitignoreContent), testFilePerms)
	require.NoError(t, err)

	// Commit .gitignore
	gitCmd := exec.Command("git", "add", ".gitignore")
	gitCmd.Dir = repoPath
	require.NoError(t, gitCmd.Run())

	gitCmd = exec.Command("git", "commit", "-m", "Add gitignore")
	gitCmd.Dir = repoPath
	require.NoError(t, gitCmd.Run())

	setupTestConfig(t, repoPath, "")

	// Create ignored file (should be filtered out)
	ignoredFile := filepath.Join(repoPath, "debug.log")
	err = os.WriteFile(ignoredFile, []byte("log content"), testFilePerms)
	require.NoError(t, err)

	// Create non-ignored file (should appear in output)
	normalFile := filepath.Join(repoPath, "data.txt")
	err = os.WriteFile(normalFile, []byte("normal content"), testFilePerms)
	require.NoError(t, err)

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

	// Assertions: .gitignore patterns should be respected
	assert.NoError(t, err)
	assert.NotContains(t, output, "debug.log", "Ignored file should not appear in dry-run output")
	assert.Contains(t, output, "data.txt", "Non-ignored file should appear in dry-run output")
}

func TestSyncCmd_DryRunShowsDeletedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)

	// Create and commit a file that will be deleted
	deletedFile := filepath.Join(repoPath, "to-delete.txt")
	err := os.WriteFile(deletedFile, []byte("will be deleted"), testFilePerms)
	require.NoError(t, err)

	cmd := exec.Command("git", "add", "to-delete.txt")
	cmd.Dir = repoPath
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "commit", "-m", "Add file to delete")
	cmd.Dir = repoPath
	err = cmd.Run()
	require.NoError(t, err)

	// Delete the file
	err = os.Remove(deletedFile)
	require.NoError(t, err)

	setupTestConfig(t, repoPath, "")

	// Enable dry-run mode
	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() { globalDryRun = oldDryRun })

	// Capture stdout
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
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
	cobraCmd := &cobra.Command{}
	err = runSync(cobraCmd, []string{})

	_ = w.Close()
	output := <-outputChan

	// Assertions: Deleted files should be shown
	assert.NoError(t, err)
	assert.Contains(t, output, "Would delete:", "Should show deletion section")
	assert.Contains(t, output, "to-delete.txt", "Deleted file should appear in output")
	assert.Contains(t, output, "1 deleted", "Summary should show 1 deleted file")

	// Verify file is still deleted (dry-run shouldn't restore it)
	_, err = os.Stat(deletedFile)
	assert.True(t, os.IsNotExist(err), "Deleted file should remain deleted after dry-run")
}

// TestSyncCmd_DryRunEdgeCase_StagedThenDeleted tests the AD (Added-Deleted) edge case
// A file is staged for addition, then deleted from the worktree.
// Expected: git add . would remove it from the index, resulting in no change (skip it)
func TestSyncCmd_DryRunEdgeCase_StagedThenDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)
	setupTestConfig(t, repoPath, "")

	// Create a new file and stage it
	newFile := filepath.Join(repoPath, "staged-then-deleted.txt")
	err := os.WriteFile(newFile, []byte("content"), testFilePerms)
	require.NoError(t, err)

	cmd := exec.Command("git", "add", "staged-then-deleted.txt")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run(), "git add should succeed")

	// Delete the file from worktree (now it's AD: staged for addition, deleted from worktree)
	err = os.Remove(newFile)
	require.NoError(t, err)

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
	cobraCmd := &cobra.Command{}
	err = runSync(cobraCmd, []string{})

	_ = w.Close()
	output := <-outputChan

	// Assertions: File should NOT appear in any category
	// git add . would remove it from index, resulting in no change
	assert.NoError(t, err)
	assert.NotContains(t, output, "staged-then-deleted.txt", "AD file should not appear in dry-run (git add . would remove from index)")
}

// TestSyncCmd_DryRunEdgeCase_DeletedThenRecreated tests the DU (Deleted-Untracked) edge case
// A file is deleted from the index (git rm), then recreated as untracked.
// Expected: git add . would stage it as a modification, not addition
func TestSyncCmd_DryRunEdgeCase_DeletedThenRecreated(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)

	// Create and commit a file
	testFile := filepath.Join(repoPath, "deleted-then-recreated.txt")
	err := os.WriteFile(testFile, []byte("original"), testFilePerms)
	require.NoError(t, err)

	cmd := exec.Command("git", "add", "deleted-then-recreated.txt")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Add file to be deleted")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	// Delete from index (git rm --cached to keep worktree file)
	cmd = exec.Command("git", "rm", "--cached", "deleted-then-recreated.txt")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run(), "git rm --cached should succeed")

	// Now the file is DU: Deleted from index, Untracked in worktree
	setupTestConfig(t, repoPath, "")

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
	cobraCmd := &cobra.Command{}
	err = runSync(cobraCmd, []string{})

	_ = w.Close()
	output := <-outputChan

	// Assertions: File should appear in "modified" section, not "added"
	// git add . would stage it as a modification (the file existed before)
	assert.NoError(t, err)
	assert.Contains(t, output, "Would modify:", "DU file should be in modified section")
	assert.Contains(t, output, "deleted-then-recreated.txt", "DU file should appear in output")

	// Verify it's NOT in the added section
	if strings.Contains(output, "Would add:") {
		addedSection := output[strings.Index(output, "Would add:"):]
		if modIndex := strings.Index(addedSection, "Would modify:"); modIndex != -1 {
			addedSection = addedSection[:modIndex]
		}
		assert.NotContains(t, addedSection, "deleted-then-recreated.txt",
			"DU file should NOT be in added section (git add . treats it as modification)")
	}
}

// TestSyncCmd_DryRunEdgeCase_StagedThenModified tests the AM (Added-Modified) edge case
// A file is staged for addition, then modified again in the worktree.
// Expected: git add . updates staging with latest content, file is still categorized as "added"
func TestSyncCmd_DryRunEdgeCase_StagedThenModified(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoPath)
	setupTestConfig(t, repoPath, "")

	// Create a new file, stage it, then modify it
	testFile := filepath.Join(repoPath, "staged-modified.txt")
	err := os.WriteFile(testFile, []byte("version 1"), testFilePerms)
	require.NoError(t, err)

	cmd := exec.Command("git", "add", "staged-modified.txt")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run(), "git add should succeed")

	// Modify the file again (now it's AM: Added in staging, Modified in worktree)
	err = os.WriteFile(testFile, []byte("version 2"), testFilePerms)
	require.NoError(t, err)

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
	cobraCmd := &cobra.Command{}
	err = runSync(cobraCmd, []string{})

	_ = w.Close()
	output := <-outputChan

	// Assertions: File should appear in "added" section
	// git add . will update staging with latest content, but file is still new (added)
	assert.NoError(t, err)
	assert.Contains(t, output, "Would add:", "AM file should be in added section")
	assert.Contains(t, output, "staged-modified.txt", "AM file should appear in output")

	// Verify it's NOT in modified or deleted sections
	assert.NotContains(t, output, "Would modify:", "AM file should not be in modified section")
	if strings.Contains(output, "Would delete:") {
		deletedSection := output[strings.Index(output, "Would delete:"):]
		assert.NotContains(t, deletedSection, "staged-modified.txt",
			"AM file should NOT be in deleted section")
	}
}
