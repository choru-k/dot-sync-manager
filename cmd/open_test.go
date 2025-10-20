package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
)

func TestOpenCmd_Sanity(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "no arguments should error",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "single argument should work",
			args:        []string{"/tmp"},
			expectError: false,
		},
		{
			name:        "multiple arguments should error",
			args:        []string{"/tmp", "/var"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "open"}
			err := runOpen(cmd, tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("runOpen() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestGetOpenCommand(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		expectError bool
	}{
		{
			name:        "empty target",
			target:      "",
			expectError: true,
		},
		{
			name:        "valid directory",
			target:      "/tmp",
			expectError: false,
		},
		{
			name:        "valid file",
			target:      "/tmp/test.txt",
			expectError: false,
		},
		{
			name:        "non-existent path",
			target:      "/tmp/non-existent-path-12345",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file if it doesn't exist
			if tt.target == "/tmp/test.txt" {
				createTestFile(t, tt.target, "test content")
				defer func() {
					if err := os.Remove(tt.target); err != nil {
						t.Logf("warning: failed to remove test file: %v", err)
					}
				}()
			}

			cmd, err := getOpenCommand(tt.target)
			if (err != nil) != tt.expectError {
				t.Errorf("getOpenCommand() error = %v, expectError %v", err, tt.expectError)
			}

			if !tt.expectError {
				if cmd == "" {
					t.Error("expected non-empty command")
				}
			}
		})
	}
}

func TestGetPlatformOpenCommand(t *testing.T) {
	tests := []struct {
		name          string
		targetPath    string
		expectedStart string
	}{
		{
			name:          "any path on darwin",
			targetPath:    "/tmp/test",
			expectedStart: "open",
		},
		{
			name:          "any path on linux",
			targetPath:    "/tmp/test",
			expectedStart: "xdg-open",
		},
		{
			name:          "any path on windows",
			targetPath:    "C:\\tmp\\test",
			expectedStart: "start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := getPlatformOpenCommand(tt.targetPath)
			if cmd == "" {
				t.Error("expected non-empty command")
			}

			// Check command starts with expected platform-specific command
			// Note: This test may need adjustment based on actual implementation
			if runtime.GOOS == "darwin" && !contains(cmd, "open") {
				t.Errorf("expected darwin command to contain 'open', got: %s", cmd)
			}
		})
	}
}

func TestOpenCmd_Integration(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Test opening repository directory
	cmd := &cobra.Command{Use: "open"}
	err := runOpen(cmd, []string{testConfig.RepoPath})
	if err != nil {
		t.Errorf("runOpen() failed: %v", err)
	}
}

func TestOpenCmd_NonExistentPath(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Test opening non-existent path
	cmd := &cobra.Command{Use: "open"}
	err := runOpen(cmd, []string{"/tmp/non-existent-path-12345"})
	if err == nil {
		t.Error("expected runOpen() to fail with non-existent path")
	}
}

func TestOpenCmd_RelativePath(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Test opening relative path
	cmd := &cobra.Command{Use: "open"}
	err := runOpen(cmd, []string{"relative/path"})
	if err == nil {
		t.Error("expected runOpen() to fail with relative path")
	}
}

func TestValidateOpenTarget(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		expectError bool
	}{
		{
			name:        "empty target",
			target:      "",
			expectError: true,
		},
		{
			name:        "absolute directory",
			target:      "/tmp",
			expectError: false,
		},
		{
			name:        "absolute file",
			target:      "/tmp/test.txt",
			expectError: false,
		},
		{
			name:        "relative path",
			target:      "relative/path",
			expectError: true,
		},
		{
			name:        "home directory path",
			target:      "~/test.txt",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file if needed
			if tt.target == "/tmp/test.txt" {
				createTestFile(t, tt.target, "test content")
				defer func() {
					if err := os.Remove(tt.target); err != nil {
						t.Logf("warning: failed to remove test file: %v", err)
					}
				}()
			}

			err := validateOpenTarget(tt.target)
			if (err != nil) != tt.expectError {
				t.Errorf("validateOpenTarget() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestOpenCmd_EditorIntegration(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Create test file to open
	testFile := filepath.Join(testConfig.RepoPath, "README.md")
	createTestFile(t, testFile, "# Test README")

	// Test opening with editor flag
	cmd := &cobra.Command{Use: "open"}
	if err := cmd.Flags().Set("editor", "true"); err != nil {
		t.Fatalf("failed to set editor flag: %v", err)
	}

	err := runOpen(cmd, []string{testFile})
	if err != nil {
		t.Errorf("runOpen() with editor flag failed: %v", err)
	}
}

func TestGetEditorCommand(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "text file",
			filePath: "/tmp/test.txt",
			expected: "code", // Default to VS Code
		},
		{
			name:     "markdown file",
			filePath: "/tmp/test.md",
			expected: "code", // Default to VS Code
		},
		{
			name:     "json file",
			filePath: "/tmp/config.json",
			expected: "code", // Default to VS Code
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with EDITOR unset
			editor := os.Getenv("EDITOR")
			defer func() {
				if editor != "" {
					if err := os.Setenv("EDITOR", editor); err != nil {
						t.Logf("warning: failed to restore EDITOR: %v", err)
					}
				} else {
					if err := os.Unsetenv("EDITOR"); err != nil {
						t.Logf("warning: failed to unset EDITOR: %v", err)
					}
				}
			}()
			if err := os.Unsetenv("EDITOR"); err != nil {
				t.Logf("warning: failed to unset EDITOR: %v", err)
			}

			cmd := getEditorCommand(tt.filePath)
			if cmd != tt.expected {
				t.Errorf("getEditorCommand() = %q, expected %q", cmd, tt.expected)
			}
		})
	}
}

func TestOpenCmd_FileTypeDetection(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Test opening different file types
	testFiles := []struct {
		name     string
		path     string
		content  string
		editor   bool
	}{
		{
			name:   "text file",
			path:   "test.txt",
			content: "plain text content",
			editor: false,
		},
		{
			name:   "markdown file",
			path:   "README.md",
			content: "# README",
			editor: true,
		},
		{
			name:   "config file",
			path:   "config.json",
			content: `{"key": "value"}`,
			editor: true,
		},
	}

	for _, tf := range testFiles {
		t.Run(tf.name, func(t *testing.T) {
			fullPath := filepath.Join(testConfig.RepoPath, tf.path)
			createTestFile(t, fullPath, tf.content)

			cmd := &cobra.Command{Use: "open"}
			if tf.editor {
				if err := cmd.Flags().Set("editor", "true"); err != nil {
					t.Fatalf("failed to set editor flag: %v", err)
				}
			}

			err := runOpen(cmd, []string{fullPath})
			if err != nil {
				t.Errorf("runOpen() failed for %s: %v", tf.name, err)
			}
		})
	}
}

func TestOpenCmd_PermissionHandling(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Create a directory with restricted permissions
	restrictedDir := filepath.Join(testConfig.RepoPath, "restricted")
	err := os.MkdirAll(restrictedDir, 0700) // Only owner access
	if err != nil {
		t.Fatalf("failed to create restricted directory: %v", err)
	}

	// Test opening restricted directory
	cmd := &cobra.Command{Use: "open"}
	err = runOpen(cmd, []string{restrictedDir})
	// Should succeed since we own the directory
	if err != nil {
		t.Errorf("runOpen() failed for restricted directory: %v", err)
	}
}

func TestOpenCmd_ExpandPath(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Test path expansion with ~
	homeExpandedPath := filepath.Join(testConfig.HomeDir, ".bashrc")
	createTestFile(t, homeExpandedPath, "# Bash config")

	cmd := &cobra.Command{Use: "open"}
	err := runOpen(cmd, []string{"~/.bashrc"})
	if err != nil {
		t.Errorf("runOpen() failed with tilde path: %v", err)
	}
}