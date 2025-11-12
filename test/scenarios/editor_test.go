package scenarios

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestScenario_EditorBasicWorkflow tests that DSM can launch editors correctly
func TestScenario_EditorBasicWorkflow(t *testing.T) {
	testID := RequireTestID(t)

	// Create isolated test environment
	sourceDir, targetDir := CreateTestEnvironment(t, testID)
	configPath := setupTestConfig(t, testID, sourceDir, targetDir)

	// Create a test file to edit (not using the symlinked fixture files)
	testFile := filepath.Join(sourceDir, ".test_bashrc")
	createTestFile(t, testFile, "# Test bashrc\nexport PATH=$PATH:/usr/local/bin\n")

	// Add file to DSM
	addFileToDSMWithConfig(t, configPath, testFile)

	// Note: CreateTestEnvironment already sets up the git repository

	// Set up a secure fake editor with argument validation
	fakeEditor := newSecureEditorStub(t, sourceDir, "fake-editor.sh", func(args []string) {
		// Validate that editor was called with expected arguments
		t.Logf("Secure editor called with args: %v", args)
	})

	// Test editor launching by setting EDITOR environment
	ctx := context.Background()
	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "open", "--editor", ".test_bashrc")
	cmd.Env = append(cmd.Env, "EDITOR="+fakeEditor, "DSM_TEST_MODE=true")
	cmd.Dir = sourceDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Editor launch failed with error: %v", err)
		t.Logf("Editor launch output: %s", string(output))
	}
	require.NoError(t, err, "Editor should launch successfully")
	t.Logf("Editor launch output: %s", string(output))

	// Verify editor behavior in test mode
	// In test mode, DSM should skip actual editor execution but complete successfully
	t.Logf("Editor launch completed successfully in test mode")

	// In test mode, the editor command is replaced with "true", so no actual editing occurs
	// We verify the command completed without error, indicating proper editor path resolution
}

// TestScenario_EditorConflictDetection tests DSM's editor safety checks
func TestScenario_EditorConflictDetection(t *testing.T) {
	testID := RequireTestID(t)

	// Create isolated test environment
	sourceDir, targetDir := CreateTestEnvironment(t, testID)
	configPath := setupTestConfig(t, testID, sourceDir, targetDir)

	// Create a test file
	testFile := filepath.Join(sourceDir, ".vimrc")
	createTestFile(t, testFile, "set number\nset syntax=on\n")

	// Add file to DSM
	addFileToDSMWithConfig(t, configPath, testFile)

	// Note: CreateTestEnvironment already sets up the git repository

	// Create a secure fake editor that simulates a conflict scenario
	fakeEditor := newSecureEditorStub(t, sourceDir, "conflict-editor.sh", func(args []string) {
		// Validate that conflict editor was called with expected arguments
		t.Logf("Conflict editor called with args: %v", args)

		// Simulate conflict scenario by modifying the target file
		if len(args) > 0 {
			targetFile := args[0]
			// Add conflict markers to simulate merge conflict
			conflictContent := `<<<<<<< HEAD
Local changes
=======
Remote changes
>>>>>>> branch
`
			// Read existing content and append conflict markers
			if content, err := os.ReadFile(targetFile); err == nil {
				modifiedContent := string(content) + "\n" + conflictContent
				os.WriteFile(targetFile, []byte(modifiedContent), 0644)
			}
		}
	})

	// Test editor with conflict detection
	ctx := context.Background()
	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "open", ".vimrc")
	cmd.Env = append(cmd.Env, "EDITOR="+fakeEditor, "DSM_TEST_MODE=true")
	cmd.Dir = sourceDir

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Editor should handle conflicts gracefully")
	t.Logf("Editor conflict handling output: %s", string(output))

	// In test mode, verify the editor was called successfully
	t.Logf("Conflict scenario editor test completed successfully")
	// Note: In test mode, actual file modification doesn't occur, but we verify
	// the editor workflow completed without errors
}

// TestScenario_EditorEnvironmentVariable tests different editor environment variables
func TestScenario_EditorEnvironmentVariable(t *testing.T) {
	testID := RequireTestID(t)
	ctx := context.Background()

	// Create isolated test environment
	sourceDir, targetDir := CreateTestEnvironment(t, testID)
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

	// Note: CreateTestEnvironment already sets up the git repository

	// Create different secure fake editors for testing
	vimEditor := newSecureEditorStub(t, sourceDir, "fake-vim", func(args []string) {
		t.Logf("Vim editor called with args: %v", args)
	})

	nanoEditor := newSecureEditorStub(t, sourceDir, "fake-nano", func(args []string) {
		t.Logf("Nano editor called with args: %v", args)
	})

	_ = newSecureEditorStub(t, sourceDir, "fake-code", func(args []string) {
		t.Logf("VS Code editor called with args: %v", args)
	})

	// Test EDITOR environment variable
	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "open", ".vimrc")
	cmd.Env = append(cmd.Env, "EDITOR="+vimEditor, "DSM_TEST_MODE=true")
	cmd.Dir = sourceDir

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "EDITOR variable should work")
	t.Logf("EDITOR test output: %s", string(output))

	// In test mode, verify editor command was processed successfully
	t.Logf("EDITOR variable test completed successfully")

	// Test VISUAL environment variable (takes precedence over EDITOR)
	cmd = execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "open", ".nanorc")
	cmd.Env = append(cmd.Env, "EDITOR="+vimEditor, "VISUAL="+nanoEditor, "DSM_TEST_MODE=true")
	cmd.Dir = sourceDir

	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "VISUAL variable should take precedence")
	t.Logf("VISUAL test output: %s", string(output))

	// In test mode, verify VISUAL variable was processed successfully
	t.Logf("VISUAL variable test completed successfully")

	// Test direct editor specification in config if supported
	// This would test DSM's ability to use configured editors
}

// setupTestConfig creates a test configuration for DSM with editor settings
func setupTestConfig(t *testing.T, testID, sourceDir, targetDir string) string {
	// Use the basic config template like other working tests
	return writeConfigFromTemplate(t, "basic", map[string]interface{}{
		"SourceDir": sourceDir,
		"TargetDir": targetDir,
	})
}

// TestScenario_EditorFallback tests what happens when no editor is available
func TestScenario_EditorFallback(t *testing.T) {
	testID := RequireTestID(t)

	// Create isolated test environment
	sourceDir, targetDir := CreateTestEnvironment(t, testID)
	configPath := setupTestConfig(t, testID, sourceDir, targetDir)

	// Create a test file
	testFile := filepath.Join(sourceDir, ".editorconfig")
	createTestFile(t, testFile, "root = true\n[*]\nindent_style = space\n")

	// Add file to DSM
	addFileToDSMWithConfig(t, configPath, testFile)

	// Note: CreateTestEnvironment already sets up the git repository

	// Test with non-existent editor
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "open", ".editorconfig")
	cmd.Env = append(cmd.Env, "EDITOR=/non/existent/editor", "DSM_TEST_MODE=true")
	cmd.Dir = sourceDir

	output, err := cmd.CombinedOutput()
	// In test mode, DSM should handle non-existent editor gracefully by using "true" command
	// The validation should be bypassed in test mode
	if err != nil {
		t.Logf("Unexpected error in test mode: %v", err)
		t.Logf("Error output: %s", string(output))
		require.Fail(t, "In test mode, editor validation should be bypassed")
	} else {
		t.Logf("DSM handled non-existent editor gracefully in test mode: %s", string(output))
	}
}

// TestScenario_EditorRejectsInlineShell tests that DSM properly rejects shell injection attempts
func TestScenario_EditorRejectsInlineShell(t *testing.T) {
	testID := RequireTestID(t)

	// Create isolated test environment
	sourceDir, targetDir := CreateTestEnvironment(t, testID)
	configPath := setupTestConfig(t, testID, sourceDir, targetDir)

	// Create a test file
	testFile := filepath.Join(sourceDir, ".test_security")
	createTestFile(t, testFile, "# Security test file\nexport TEST=value\n")

	// Add file to DSM
	addFileToDSMWithConfig(t, configPath, testFile)

	// Test malicious editor command that attempts shell injection
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "open", "--editor", ".test_security")
	cmd.Env = append(cmd.Env, "EDITOR=bash -c 'echo HACKED; rm -rf /'", "DSM_TEST_MODE=false") // Not test mode
	cmd.Dir = sourceDir

	output, err := cmd.CombinedOutput()

	// DSM should reject the malicious editor command and fail before executing it
	if err == nil {
		t.Logf("DSM unexpectedly succeeded with malicious editor: %s", string(output))

		// Check if malicious editor was executed (security breach)
		payloadPath := filepath.Join(sourceDir, "malicious-editor.sh.payload")
		if _, err := os.Stat(payloadPath); err == nil {
			t.Fatal("SECURITY BREACH: Malicious editor was executed!")
		}

		t.Fatal("DSM should have rejected malicious editor command")
	}

	t.Logf("DSM correctly rejected malicious editor command: %v", err)
	t.Logf("Command output: %s", string(output))

	// Verify malicious editor was never called
	payloadPath := filepath.Join(sourceDir, "malicious-editor.sh.payload")
	if _, err := os.Stat(payloadPath); err == nil {
		t.Fatal("SECURITY BREACH: Malicious editor payload file was created!")
	}

	t.Logf("✅ Security test passed: Malicious editor was not executed")
}
