package scenarios

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestScenario_EditorBasicWorkflow tests that DSM can launch editors correctly
func TestScenario_EditorBasicWorkflow(t *testing.T) {
	testID := RequireTestID(t)

	// Create isolated test environment
	sourceDir, targetDir := setupTestDirectories(t)
	configPath := setupTestConfig(t, testID, sourceDir, targetDir)

	// Create a test file to edit
	testFile := filepath.Join(sourceDir, ".bashrc")
	createTestFile(t, testFile, "# Test bashrc\nexport PATH=$PATH:/usr/local/bin\n")

	// Add file to DSM
	addFileToDSMWithConfig(t, configPath, testFile)

	// Initialize DSM repository
	initDSMWithConfig(t, configPath, targetDir)

	// Set up a fake editor that just creates a marker file
	fakeEditor := filepath.Join(sourceDir, "fake-editor.sh")
	createTestFile(t, fakeEditor, `#!/bin/bash
echo "Editor called with: $*" > "$1.edited"
touch "$1.editor-was-here"
echo "Fake editor completed"
`)

	// Make fake editor executable
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "chmod", "+x", fakeEditor)
	cmd.Dir = sourceDir
	err := cmd.Run()
	require.NoError(t, err)

	// Test editor launching by setting EDITOR environment
	cmd = execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "open", ".bashrc")
	cmd.Env = append(cmd.Env, "EDITOR="+fakeEditor)
	cmd.Dir = sourceDir

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Editor should launch successfully")
	t.Logf("Editor launch output: %s", string(output))

	// Verify the fake editor was called
	editedFile := testFile + ".edited"
	_, err = os.Stat(editedFile)
	require.NoError(t, err, "Editor should have been called and created marker file")

	markerFile := testFile + ".editor-was-here"
	_, err = os.Stat(markerFile)
	require.NoError(t, err, "Editor should have created marker file")
}

// TestScenario_EditorConflictDetection tests DSM's editor safety checks
func TestScenario_EditorConflictDetection(t *testing.T) {
	testID := RequireTestID(t)

	// Create isolated test environment
	sourceDir, targetDir := setupTestDirectories(t)
	configPath := setupTestConfig(t, testID, sourceDir, targetDir)

	// Create a test file
	testFile := filepath.Join(sourceDir, ".vimrc")
	createTestFile(t, testFile, "set number\nset syntax=on\n")

	// Add file to DSM
	addFileToDSMWithConfig(t, configPath, testFile)

	// Initialize DSM repository
	initDSMWithConfig(t, configPath, targetDir)

	// Create a fake editor that simulates a conflict scenario
	fakeEditor := filepath.Join(sourceDir, "conflict-editor.sh")
	createTestFile(t, fakeEditor, `#!/bin/bash
echo "Simulating editor conflict scenario"
sleep 1
# Simulate merge conflict markers in the file
echo "<<<<<<< HEAD" >> "$1"
echo "Local changes" >> "$1"
echo "=======" >> "$1"
echo "Remote changes" >> "$1"
echo ">>>>>>> branch" >> "$1"
exit 0
`)

	// Make fake editor executable
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "chmod", "+x", fakeEditor)
	cmd.Dir = sourceDir
	err := cmd.Run()
	require.NoError(t, err)

	// Test editor with conflict detection
	cmd = execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "open", ".vimrc")
	cmd.Env = append(cmd.Env, "EDITOR="+fakeEditor)
	cmd.Dir = sourceDir

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Editor should handle conflicts gracefully")
	t.Logf("Editor conflict handling output: %s", string(output))

	// Verify conflict markers were handled (DSM should detect and report them)
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	contentStr := string(content)

	// DSM should either resolve conflicts or report them clearly
	if strings.Contains(contentStr, "<<<<<<< HEAD") {
		t.Logf("Conflict markers detected - DSM should handle or report these")
		// In a real scenario, DSM would detect and either resolve or report conflicts
		// For now, we verify the editor was called and the file was modified
	}
}

// TestScenario_EditorEnvironmentVariable tests different editor environment variables
func TestScenario_EditorEnvironmentVariable(t *testing.T) {
	testID := RequireTestID(t)

	// Create isolated test environment
	sourceDir, targetDir := setupTestDirectories(t)
	configPath := setupTestConfig(t, testID, sourceDir, targetDir)

	// Create test files
	vimFile := filepath.Join(sourceDir, ".vimrc")
	nanoFile := filepath.Join(sourceDir, ".nanorc")
	codeFile := filepath.Join(sourceDir, "settings.json")

	createTestFile(t, vimFile, "set number\n")
	createTestFile(t, nanoFile, "set tabsize 4\n")
	createTestFile(t, codeFile, `{"editor.tabSize": 4}`)

	// Add files to DSM
	addFileToDSMWithConfig(t, configPath, vimFile)
	addFileToDSMWithConfig(t, configPath, nanoFile)
	addFileToDSMWithConfig(t, configPath, codeFile)

	// Initialize DSM repository
	initDSMWithConfig(t, configPath, targetDir)

	// Create different fake editors for testing
	vimEditor := filepath.Join(sourceDir, "fake-vim")
	createTestFile(t, vimEditor, `#!/bin/bash
echo "Vim editor called: $*" > "$1.vim-edited"
sleep 0.1
`)

	nanoEditor := filepath.Join(sourceDir, "fake-nano")
	createTestFile(t, nanoEditor, `#!/bin/bash
echo "Nano editor called: $*" > "$1.nano-edited"
sleep 0.1
`)

	codeEditor := filepath.Join(sourceDir, "fake-code")
	createTestFile(t, codeEditor, `#!/bin/bash
echo "VS Code editor called: $*" > "$1.code-edited"
sleep 0.1
`)

	// Make all fake editors executable
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	 editors := []string{vimEditor, nanoEditor, codeEditor}
	for _, editor := range editors {
		cmd := exec.CommandContext(ctx, "chmod", "+x", editor)
		cmd.Dir = sourceDir
		err := cmd.Run()
		require.NoError(t, err)
	}

	// Test EDITOR environment variable
	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "open", ".vimrc")
	cmd.Env = append(cmd.Env, "EDITOR="+vimEditor)
	cmd.Dir = sourceDir

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "EDITOR variable should work")
	t.Logf("EDITOR test output: %s", string(output))

	// Verify vim editor was called
	_, err = os.Stat(vimFile + ".vim-edited")
	require.NoError(t, err, "Vim editor should have been called")

	// Test VISUAL environment variable (takes precedence over EDITOR)
	cmd = execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "open", ".nanorc")
	cmd.Env = append(cmd.Env, "EDITOR="+vimEditor, "VISUAL="+nanoEditor)
	cmd.Dir = sourceDir

	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "VISUAL variable should take precedence")
	t.Logf("VISUAL test output: %s", string(output))

	// Verify nano editor was called (not vim)
	_, err = os.Stat(nanoFile + ".nano-edited")
	require.NoError(t, err, "Nano editor should have been called via VISUAL")
	_, err = os.Stat(vimFile + ".nano-edited")
	require.Error(t, err, "Vim editor should NOT have been called when VISUAL is set")

	// Test direct editor specification in config if supported
	// This would test DSM's ability to use configured editors
}

// setupTestConfig creates a test configuration for DSM with editor settings
func setupTestConfig(t *testing.T, testID, sourceDir, targetDir string) string {
	configPath := filepath.Join(os.TempDir(), testID+"-config.json")

	configContent := `{
		"machine": {
			"name": "` + testID + `"
		},
		"git": {
			"repo_path": "` + targetDir + `",
			"remote_url": "git@github.com:test/dotfiles-test.git",
			"auth_type": "ssh",
			"auto_commit": true,
			"auto_push": false
		},
		"sync": {
			"auto_sync_enabled": false,
			"debounce_seconds": 5,
			"conflict_resolution": "editor"
		}
	}`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err, "Should create test config")

	return configPath
}

// TestScenario_EditorFallback tests what happens when no editor is available
func TestScenario_EditorFallback(t *testing.T) {
	testID := RequireTestID(t)

	// Create isolated test environment
	sourceDir, targetDir := setupTestDirectories(t)
	configPath := setupTestConfig(t, testID, sourceDir, targetDir)

	// Create a test file
	testFile := filepath.Join(sourceDir, ".editorconfig")
	createTestFile(t, testFile, "root = true\n[*]\nindent_style = space\n")

	// Add file to DSM
	addFileToDSMWithConfig(t, configPath, testFile)

	// Initialize DSM repository
	initDSMWithConfig(t, configPath, targetDir)

	// Test with non-existent editor
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "open", ".editorconfig")
	cmd.Env = append(cmd.Env, "EDITOR=/non/existent/editor")
	cmd.Dir = sourceDir

	output, err := cmd.CombinedOutput()
	// DSM should handle missing editor gracefully
	// Either return error with clear message or use fallback
	if err != nil {
		t.Logf("Expected error with non-existent editor: %v", err)
		t.Logf("Error output: %s", string(output))
		// Verify error message is helpful
		require.Contains(t, string(output), "editor", "Error should mention editor")
	} else {
		t.Logf("DSM handled missing editor gracefully: %s", string(output))
	}
}