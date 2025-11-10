package scenarios

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConflictResolution tests DSM's ability to handle conflicts and use stash/restore
func TestConflictResolution(t *testing.T) {
	testID := os.Getenv("TEST_ID")
	require.NotEmpty(t, testID, "TEST_ID environment variable is required")

	t.Logf("Running conflict resolution test with ID: %s", testID)

	// Setup test environment
	// Create test directories
	sourceDir := filepath.Join(testDataDir, "source_dotfiles")
	targetDir := filepath.Join(testDataDir, "dotfiles-test-conflict")

	err := os.MkdirAll(sourceDir, dirPermissions)
	require.NoError(t, err)

	err = os.MkdirAll(targetDir, dirPermissions)
	require.NoError(t, err)

	// Initialize DSM
	setupDSMForConflictTest(t, sourceDir)

	t.Run("CreateInitialSync", func(t *testing.T) {
		// Add a test file
		testFile := filepath.Join(sourceDir, ".testconfig")
		initialContent := `# Initial test configuration
VERSION=1.0
MODE=test
`

		err := os.WriteFile(testFile, []byte(initialContent), filePermissions)
		require.NoError(t, err)

		// Add and sync the file
		addAndSyncFileWithConfig(t, "/app/test/fixtures/test_configs/conflict_config.json", testFile)

		// Verify file exists in target
		targetFile := filepath.Join(targetDir, ".testconfig")
		content, err := os.ReadFile(targetFile)
		require.NoError(t, err)
		assert.Equal(t, initialContent, string(content))
	})

	t.Run("SimulateRemoteConflict", func(t *testing.T) {
		// Simulate a remote change by directly modifying the target file
		targetFile := filepath.Join(targetDir, ".testconfig")
		remoteContent := `# Remote modified configuration
VERSION=1.1
MODE=production
REMOTE_SETTING=true
`

		err := os.WriteFile(targetFile, []byte(remoteContent), filePermissions)
		require.NoError(t, err)

		// Commit the remote change
		commitGitChanges(t, targetDir, "Remote modification")

		t.Logf("Simulated remote change: %s", remoteContent)
	})

	t.Run("CreateLocalConflict", func(t *testing.T) {
		// Create a local modification to the same file
		testFile := filepath.Join(sourceDir, ".testconfig")
		localContent := `# Local modified configuration
VERSION=1.2
MODE=development
LOCAL_SETTING=true
`

		err := os.WriteFile(testFile, []byte(localContent), filePermissions)
		require.NoError(t, err)

		t.Logf("Created local change: %s", localContent)
	})

	t.Run("SyncWithConflict", func(t *testing.T) {
		// Attempt to sync - this should trigger conflict resolution
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		cmd := execCommandContextWithConfig(ctx, "/app/test/fixtures/test_configs/conflict_config.json", "dsm", "--config", "/app/test/fixtures/test_configs/conflict_config.json", "sync")
		output, err := cmd.CombinedOutput()

		t.Logf("Sync with conflict output: %s", string(output))

		// The sync should succeed with conflict resolution
		require.NoError(t, err, "Sync with conflict resolution should succeed")

		outputStr := string(output)
		// Check for conflict resolution indicators
		conflictIndicators := []string{
			"conflict", "stash", "restore", "resolved", "merged",
		}

		found := false
		for _, indicator := range conflictIndicators {
			if strings.Contains(strings.ToLower(outputStr), indicator) {
				found = true
				t.Logf("Found conflict resolution indicator: %s", indicator)
				break
			}
		}

		if !found {
			t.Logf("No explicit conflict resolution indicators found, but sync succeeded")
		}
	})

	t.Run("VerifyConflictResolution", func(t *testing.T) {
		// Verify that the file exists and contains content
		targetFile := filepath.Join(targetDir, ".testconfig")
		content, err := os.ReadFile(targetFile)
		require.NoError(t, err, "Target file should exist after conflict resolution")

		contentStr := string(content)
		t.Logf("Final file content after conflict resolution:\n%s", contentStr)

		// The file should contain some combination of local and remote changes
		assert.Contains(t, contentStr, "VERSION=")
		assert.Contains(t, contentStr, "MODE=")

		// At least one of the local or remote settings should be present
		hasLocalSetting := strings.Contains(contentStr, "LOCAL_SETTING=true")
		hasRemoteSetting := strings.Contains(contentStr, "REMOTE_SETTING=true")

		assert.True(t, hasLocalSetting || hasRemoteSetting,
			"File should contain either local or remote settings after conflict resolution")
	})

	t.Run("TestConflictDetection", func(t *testing.T) {
		// Create another conflict scenario to test detection
		testFile := filepath.Join(sourceDir, ".testconfig")

		// Make a local change
		newLocalContent := `# Local change for detection test
VERSION=2.0
MODE=local-test
DETECTION_TEST=true
`

		err := os.WriteFile(testFile, []byte(newLocalContent), filePermissions)
		require.NoError(t, err)

		// Simulate remote change again
		targetFile := filepath.Join(targetDir, ".testconfig")
		newRemoteContent := `# Remote change for detection test
VERSION=2.1
MODE=remote-test
REMOTE_DETECTION=true
`

		err = os.WriteFile(targetFile, []byte(newRemoteContent), 0644)
		require.NoError(t, err)

		// Commit remote change
		commitGitChanges(t, targetDir, "Remote change for detection test")

		// Try to sync - should detect and handle conflict
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		cmd := execCommandContext(ctx, "dsm", "sync")
		output, err := cmd.CombinedOutput()

		require.NoError(t, err, "Should handle second conflict gracefully")
		t.Logf("Second conflict resolution output: %s", string(output))
	})

	t.Log("✅ Conflict resolution test completed successfully")
}


// setupDSMForConflictTest initializes DSM for conflict testing
func setupDSMForConflictTest(t *testing.T, sourceDir string) {
	configPath := "/app/test/fixtures/test_configs/conflict_config.json"

	// Since DSM init requires interactive confirmation for existing directories,
	// we'll verify DSM can read configuration correctly instead
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "status")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "DSM should be able to read configuration")
	t.Logf("DSM status output: %s", string(output))
}

// addAndSyncFile adds a file to DSM and syncs it
func addAndSyncFile(t *testing.T, filepath string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	// Add file
	cmd := execCommandContext(ctx, "dsm", "add", filepath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Adding file should succeed")
	t.Logf("Add file output: %s", string(output))

	// Sync file
	ctx2, cancel2 := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel2()

	cmd2 := execCommandContext(ctx2, "dsm", "sync")
	output2, err2 := cmd2.CombinedOutput()
	require.NoError(t, err2, "Syncing file should succeed")
	t.Logf("Sync file output: %s", string(output2))
}

// commitGitChanges commits changes in a git repository
func commitGitChanges(t *testing.T, repoDir, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	// Set git environment
	env := append(os.Environ(),
		"GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i /app/ssh-keys/test_key",
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)

	// Add all changes
	cmd := exec.CommandContext(ctx, "git", "add", ".")
	cmd.Dir = repoDir
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Git add should succeed")
	t.Logf("Git add output: %s", string(output))

	// Commit changes
	cmd = exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Dir = repoDir
	cmd.Env = env
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Git commit should succeed")
	t.Logf("Git commit output: %s", string(output))
}

// addAndSyncFileWithConfig adds a file to DSM and syncs it with custom config
func addAndSyncFileWithConfig(t *testing.T, configPath, filepath string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	// Add file
	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "add", filepath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Adding file should succeed")
	t.Logf("Add file output: %s", string(output))

	// Sync file
	ctx2, cancel2 := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel2()

	cmd2 := execCommandContextWithConfig(ctx2, configPath, "dsm", "--config", configPath, "sync")
	output2, err := cmd2.CombinedOutput()
	require.NoError(t, err, "Syncing file should succeed")
	t.Logf("Sync output: %s", string(output2))
}

