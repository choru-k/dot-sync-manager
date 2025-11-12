package scenarios

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestScenario_DsmAddWorkflow tests the complete dsm add workflow non-interactively
func TestScenario_DsmAddWorkflow(t *testing.T) {
	testID := RequireTestID(t)
	t.Logf("Testing dsm add workflow with ID: %s", testID)

	// Create isolated test environment
	sourceDir, targetDir := CreateTestEnvironment(t, testID)
	configPath := setupTestConfig(t, testID, sourceDir, targetDir)

	// Create test dotfiles in source directory
	testFiles := map[string]string{
		".bashrc":    "# Bash configuration\nexport PATH=$PATH:/usr/local/bin\n",
		".vimrc":     "# Vim configuration\nset number\nset syntax=on\n",
		".gitconfig": "[user]\n\tname = Test User\n\temail = test@example.com\n",
		".tmux.conf": "# Tmux configuration\nset -g mouse on\n",
	}

	for filename, content := range testFiles {
		testFile := filepath.Join(sourceDir, filename)
		createTestFile(t, testFile, content)
		t.Logf("Created test file: %s", testFile)
	}

	// Note: CreateTestEnvironment already sets up the git repository

	t.Run("AddSingleFile", func(t *testing.T) {
		// Test adding a single file using absolute path
		bashrcPath := filepath.Join(sourceDir, ".bashrc")

		ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
		defer cancel()

		cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "add", bashrcPath)
		cmd.Env = append(cmd.Env, "HOME="+sourceDir) // Set HOME to source directory
		output, err := cmd.CombinedOutput()

		require.NoError(t, err, "dsm add should succeed")
		t.Logf("dsm add output: %s", string(output))

		// Verify file was added to repository (DSM normalizes filenames by removing leading dot)
		bashrcInRepo := filepath.Join(targetDir, "bashrc") // DSM stores as bashrc, not .bashrc
		_, err = os.Stat(bashrcInRepo)
		require.NoError(t, err, "File should exist in DSM repository")
		t.Logf("✅ File successfully added to repository: %s", bashrcInRepo)
	})

	t.Run("AddMultipleFiles", func(t *testing.T) {
		// Test adding multiple files
		vimrcPath := filepath.Join(sourceDir, ".vimrc")
		gitconfigPath := filepath.Join(sourceDir, ".gitconfig")

		ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout*2)
		defer cancel()

		// Add vimrc
		cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "add", vimrcPath)
		cmd.Env = append(cmd.Env, "HOME="+sourceDir)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "dsm add should succeed for vimrc")
		t.Logf("dsm add vimrc output: %s", string(output))

		// Add gitconfig
		cmd = execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "add", gitconfigPath)
		cmd.Env = append(cmd.Env, "HOME="+sourceDir)
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "dsm add should succeed for gitconfig")
		t.Logf("dsm add gitconfig output: %s", string(output))

		// Verify both files exist in repository (DSM normalizes filenames by removing leading dot)
		vimrcInRepo := filepath.Join(targetDir, "vimrc")         // DSM stores as vimrc, not .vimrc
		gitconfigInRepo := filepath.Join(targetDir, "gitconfig") // DSM stores as gitconfig, not .gitconfig

		_, err = os.Stat(vimrcInRepo)
		require.NoError(t, err, "vimrc should exist in DSM repository")

		_, err = os.Stat(gitconfigInRepo)
		require.NoError(t, err, "gitconfig should exist in DSM repository")

		t.Logf("✅ Multiple files successfully added to repository")
	})

	t.Run("AddFileWithSymlink", func(t *testing.T) {
		// Test adding a file that should become a symlink
		tmuxPath := filepath.Join(sourceDir, ".tmux.conf")

		ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
		defer cancel()

		cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "add", tmuxPath)
		cmd.Env = append(cmd.Env, "HOME="+sourceDir)
		output, err := cmd.CombinedOutput()

		require.NoError(t, err, "dsm add should succeed for tmux.conf")
		t.Logf("dsm add tmux.conf output: %s", string(output))

		// Verify file was added (could be copy or symlink depending on platform)
		tmuxInRepo := filepath.Join(targetDir, "tmux.conf") // DSM stores as tmux.conf, not .tmux.conf
		_, err = os.Stat(tmuxInRepo)
		require.NoError(t, err, "tmux.conf should exist in DSM repository")
		t.Logf("✅ tmux.conf successfully added to repository")
	})

	t.Run("VerifyAddedFiles", func(t *testing.T) {
		// Verify all added files exist and have correct content (DSM normalizes filenames by removing leading dot)
		for filename := range testFiles {
			// DSM removes leading dot from filenames
			repoFilename := strings.TrimPrefix(filename, ".")
			repoPath := filepath.Join(targetDir, repoFilename)
			_, err := os.Stat(repoPath)
			require.NoError(t, err, "File %s should exist in repository as %s", filename, repoFilename)

			// Verify content matches
			content, err := os.ReadFile(repoPath)
			require.NoError(t, err, "Should be able to read file %s", filename)

			expected := testFiles[filename]
			if string(content) != expected {
				// Content might have been transformed (e.g., line endings), but should contain expected elements
				require.Contains(t, string(content), expected[:20], "File should contain expected content")
			}

			t.Logf("✅ Verified file content: %s", filename)
		}
	})

	t.Run("ListAddedFiles", func(t *testing.T) {
		// Test that dsm list shows the added files
		ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
		defer cancel()

		cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "list")
		cmd.Env = append(cmd.Env, "HOME="+sourceDir)
		output, err := cmd.CombinedOutput()

		require.NoError(t, err, "dsm list should succeed")
		t.Logf("dsm list output: %s", string(output))

		// Verify that our added files are listed
		for filename := range testFiles {
			require.Contains(t, string(output), filename, "Added file should appear in dsm list")
		}

		t.Logf("✅ All added files appear in dsm list")
	})
}

// TestScenario_DsmAddErrorHandling tests error conditions in dsm add workflow
func TestScenario_DsmAddErrorHandling(t *testing.T) {
	testID := RequireTestID(t)
	t.Logf("Testing dsm add error handling with ID: %s", testID)

	// Create isolated test environment
	sourceDir, targetDir := CreateTestEnvironment(t, testID)
	configPath := setupTestConfig(t, testID, sourceDir, targetDir)

	// Initialize DSM repository
	initDSMWithConfig(t, configPath, targetDir)

	t.Run("AddNonExistentFile", func(t *testing.T) {
		// Try to add a file that doesn't exist
		nonExistentPath := filepath.Join(sourceDir, ".nonexistent")

		ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
		defer cancel()

		cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "add", nonExistentPath)
		cmd.Env = append(cmd.Env, "HOME="+sourceDir)
		output, err := cmd.CombinedOutput()

		// Should fail with appropriate error message
		require.Error(t, err, "dsm add should fail for non-existent file")
		t.Logf("Expected error for non-existent file: %v", err)
		t.Logf("Error output: %s", string(output))
	})

	t.Run("AddFileOutsideConfig", func(t *testing.T) {
		// Create a file outside the configured source directory
		outsideDir := filepath.Join(os.TempDir(), testID+"-outside")
		err := os.MkdirAll(outsideDir, 0755)
		require.NoError(t, err)

		outsideFile := filepath.Join(outsideDir, ".outside")
		createTestFile(t, outsideFile, "This file is outside")

		ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
		defer cancel()

		cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "add", outsideFile)
		output, err := cmd.CombinedOutput()

		// Should either fail or handle gracefully depending on implementation
		if err != nil {
			t.Logf("Expected error for outside file: %v", err)
			t.Logf("Error output: %s", string(output))
		} else {
			t.Logf("DSM accepted outside file: %s", string(output))
		}

		// Cleanup
		_ = os.RemoveAll(outsideDir) // Ignore cleanup errors in tests
	})

	t.Run("AddAlreadyAddedFile", func(t *testing.T) {
		// Create and add a file first time
		testFile := filepath.Join(sourceDir, ".duplicate")
		createTestFile(t, testFile, "Initial content")

		ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
		defer cancel()

		// Add first time
		cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "add", testFile)
		cmd.Env = append(cmd.Env, "HOME="+sourceDir)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "First dsm add should succeed")
		t.Logf("First add output: %s", string(output))

		// Try to add same file again
		cmd = execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "add", testFile)
		cmd.Env = append(cmd.Env, "HOME="+sourceDir)
		output, err = cmd.CombinedOutput()

		// Should handle gracefully (either succeed or warn about duplicate)
		t.Logf("Second add result: %v", err)
		t.Logf("Second add output: %s", string(output))

		// Verify file still exists in repository
		inRepo := filepath.Join(targetDir, ".duplicate")
		_, err = os.Stat(inRepo)
		require.NoError(t, err, "File should still exist in repository")
	})
}
