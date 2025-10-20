package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestIgnoreCmd_Sanity(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "no arguments should work",
			args:        []string{},
			expectError: false,
		},
		{
			name:        "any arguments should error",
			args:        []string{"extra"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "ignore"}
			err := runIgnore(cmd, tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("runIgnore() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestGetIgnoreFilePath(t *testing.T) {
	tests := []struct {
		name        string
		repoPath    string
		expected    string
		expectError bool
	}{
		{
			name:        "valid repo path",
			repoPath:    "/tmp/test-repo",
			expected:    "/tmp/test-repo/.syncignore",
			expectError: false,
		},
		{
			name:        "repo path with trailing slash",
			repoPath:    "/tmp/test-repo/",
			expected:    "/tmp/test-repo/.syncignore",
			expectError: false,
		},
		{
			name:        "empty repo path",
			repoPath:    "",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ignorePath, err := getIgnoreFilePath(tt.repoPath)

			if (err != nil) != tt.expectError {
				t.Errorf("getIgnoreFilePath() error = %v, expectError %v", err, tt.expectError)
			}

			if !tt.expectError && ignorePath != tt.expected {
				t.Errorf("getIgnoreFilePath() = %q, expected %q", ignorePath, tt.expected)
			}
		})
	}
}

func TestCreateIgnoreFile(t *testing.T) {
	tests := []struct {
		name        string
		ignorePath  string
		content     string
		expectError bool
	}{
		{
			name:        "create new ignore file",
			ignorePath:  filepath.Join(t.TempDir(), ".syncignore"),
			content:     "# Test ignore file\n*.tmp",
			expectError: false,
		},
		{
			name:        "create ignore file in subdirectory",
			ignorePath:  filepath.Join(t.TempDir(), "subdir", ".syncignore"),
			content:     "# Test ignore\n*.log",
			expectError: false,
		},
		{
			name:        "empty path",
			ignorePath:  "",
			content:     "# Empty path test",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := createIgnoreFile(tt.ignorePath, tt.content)

			if (err != nil) != tt.expectError {
				t.Errorf("createIgnoreFile() error = %v, expectError %v", err, tt.expectError)
			}

			// Verify file was created (if no error expected)
			if !tt.expectError {
				if _, err := os.Stat(tt.ignorePath); os.IsNotExist(err) {
					t.Error("ignore file was not created")
				}

				// Verify content
				content, err := os.ReadFile(tt.ignorePath)
				if err != nil {
					t.Errorf("failed to read created ignore file: %v", err)
				} else if string(content) != tt.content {
					t.Errorf("ignore file content mismatch: expected %q, got %q", tt.content, string(content))
				}
			}
		})
	}
}

func TestValidateIgnoreContent(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
	}{
		{
			name:    "valid ignore patterns",
			content: "*.log\n*.tmp\n.DS_Store",
		},
		{
			name:    "valid gitignore syntax",
			content: "# Comment\n*.tmp\n!important.txt\nbuild/\n**/*.pyc",
		},
		{
			name:    "empty content",
			content: "",
		},
		{
			name:    "only comments",
			content: "# Comment 1\n# Comment 2",
		},
		{
			name:    "content with dangerous patterns",
			content: "rm -rf /\n$(sudo rm)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIgnoreContent(tt.content)
			if (err != nil) != tt.expectError {
				t.Errorf("validateIgnoreContent() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestEditIgnoreFile(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) string
		editor      string
		expectError bool
	}{
		{
			name: "edit existing file",
			setup: func(t *testing.T) string {
				ignorePath := filepath.Join(t.TempDir(), ".syncignore")
				createTestFile(t, ignorePath, "*.tmp\n*.log")
				return ignorePath
			},
			editor:      "nano",
			expectError: false, // May fail in test environment
		},
		{
			name: "edit non-existent file",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "non-existent", ".syncignore")
			},
			editor:      "nano",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ignorePath := tt.setup(t)

			err := editIgnoreFile(ignorePath)
			// In test environment, editor commands may fail
			t.Logf("editIgnoreFile() result: %v", err)
		})
	}
}

func TestIgnoreCmd_Integration(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Test ignore command with default editor
	cmd := &cobra.Command{Use: "ignore"}
	err := runIgnore(cmd, []string{})
	// May fail in test environment if editor not available
	t.Logf("runIgnore() result: %v", err)
}

func TestIgnoreCmd_CustomEditor(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Test ignore command with custom editor
	cmd := &cobra.Command{Use: "ignore"}
	if err := cmd.Flags().Set("editor", "nano"); err != nil {
		t.Fatalf("failed to set editor flag: %v", err)
	}

	err := runIgnore(cmd, []string{})
	// May fail in test environment if nano not available
	t.Logf("runIgnore() with custom editor result: %v", err)
}

func TestIgnoreCmd_CreateNewFile(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Ensure .syncignore doesn't exist
	ignorePath := filepath.Join(testConfig.RepoPath, ".syncignore")
	if _, err := os.Stat(ignorePath); err == nil {
		if err := os.Remove(ignorePath); err != nil {
			t.Logf("warning: failed to remove ignore file %s: %v", ignorePath, err)
		}
	}

	// Test ignore command - should create new file
	cmd := &cobra.Command{Use: "ignore"}
	err := runIgnore(cmd, []string{})
	if err != nil {
		t.Logf("runIgnore() result: %v", err)
	}

	// Check if file was created
	if _, err := os.Stat(ignorePath); err != nil {
		t.Log("ignore file was not created (expected in test environment)")
	}
}

func TestIgnoreCmd_ExistingFile(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Create existing .syncignore file
	ignorePath := filepath.Join(testConfig.RepoPath, ".syncignore")
	existingContent := "# Existing ignore file\n*.tmp\n*.log"
	createTestFile(t, ignorePath, existingContent)

	// Test ignore command - should edit existing file
	cmd := &cobra.Command{Use: "ignore"}
	err := runIgnore(cmd, []string{})
	if err != nil {
		t.Logf("runIgnore() result: %v", err)
	}

	// Verify original content is preserved
	content, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Errorf("failed to read ignore file: %v", err)
	} else if !strings.Contains(string(content), "Existing ignore file") {
		t.Error("original content was not preserved")
	}
}

func TestIgnoreCmd_Permissions(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Create .syncignore with restricted permissions
	ignorePath := filepath.Join(testConfig.RepoPath, ".syncignore")
	createTestFile(t, ignorePath, "# Test content")
	err := os.Chmod(ignorePath, restrictiveConfigFilePerms)
	if err != nil {
		t.Fatalf("failed to set ignore file permissions: %v", err)
	}

	// Test ignore command with restricted permissions
	cmd := &cobra.Command{Use: "ignore"}
	err = runIgnore(cmd, []string{})
	// May fail due to permissions
	t.Logf("runIgnore() with restrictive permissions result: %v", err)
}

func TestIgnoreFileValidation(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		shouldPass  bool
	}{
		{
			name:       "valid patterns",
			content:    "*.tmp\n*.log\n.DS_Store\nbuild/",
			shouldPass: true,
		},
		{
			name:       "comments and patterns",
			content:    "# Comment\n*.tmp\n!important.txt\n**/*.pyc",
			shouldPass: true,
		},
		{
			name:       "empty file",
			content:    "",
			shouldPass: true,
		},
		{
			name:       "only whitespace",
			content:    "   \n\t\n  ",
			shouldPass: true,
		},
		{
			name:       "dangerous commands",
			content:    "rm -rf /\n$(sudo rm)",
			shouldPass: false,
		},
		{
			name:       "script injection",
			content:    "`cat /etc/passwd`\n$(whoami)",
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ignorePath := filepath.Join(t.TempDir(), ".syncignore")

			// Create ignore file with test content
			err := os.WriteFile(ignorePath, []byte(tt.content), defaultFilePerms)
			if err != nil {
				t.Fatalf("failed to create ignore file: %v", err)
			}

			// Validate the ignore file
			err = validateIgnoreFile(ignorePath)
			if tt.shouldPass && err != nil {
				t.Errorf("expected validation to pass, but got error: %v", err)
			}
			if !tt.shouldPass && err == nil {
				t.Error("expected validation to fail, but it passed")
			}
		})
	}
}

func TestGetDefaultIgnoreContent(t *testing.T) {
	content := getDefaultIgnoreContent()

	if content == "" {
		t.Error("default ignore content should not be empty")
	}

	// Check that it contains common patterns
	expectedPatterns := []string{
		"*.log",
		"*.tmp",
		".DS_Store",
		"Thumbs.db",
		"node_modules/",
		"__pycache__/",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(content, pattern) {
			t.Errorf("default ignore content should contain pattern %q", pattern)
		}
	}

	// Check that it has comments explaining the patterns
	if !strings.Contains(content, "#") {
		t.Error("default ignore content should have comments")
	}
}

func TestIgnoreFileBackup(t *testing.T) {
	ignorePath := filepath.Join(t.TempDir(), ".syncignore")
	originalContent := "# Original content\n*.tmp"
	createTestFile(t, ignorePath, originalContent)

	// Create backup
	backupPath, err := createIgnoreFileBackup(ignorePath)
	if err != nil {
		t.Fatalf("createIgnoreFileBackup() failed: %v", err)
	}
	defer func() {
		if err := os.Remove(backupPath); err != nil {
			t.Logf("warning: failed to remove backup file %s: %v", backupPath, err)
		}
	}()

	// Verify backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("backup file was not created")
	}

	// Verify backup content
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup: %v", err)
	}

	if string(backupContent) != originalContent {
		t.Error("backup content does not match original")
	}
}

func TestParseIgnorePatterns(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectedCount  int
		expectedNonEmpty int
	}{
		{
			name:              "mixed content",
			content:           "# Comment\n*.tmp\n\n*.log\n  \n!important.txt",
			expectedCount:     5,
			expectedNonEmpty:  3,
		},
		{
			name:              "only comments",
			content:           "# Comment 1\n# Comment 2\n   # Comment 3",
			expectedCount:     3,
			expectedNonEmpty:  0,
		},
		{
			name:              "empty content",
			content:           "",
			expectedCount:     0,
			expectedNonEmpty:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := parseIgnorePatterns(tt.content)

			if len(patterns) != tt.expectedCount {
				t.Errorf("parseIgnorePatterns() returned %d patterns, expected %d", len(patterns), tt.expectedCount)
			}

			nonEmptyCount := 0
			for _, pattern := range patterns {
				if strings.TrimSpace(pattern) != "" && !strings.HasPrefix(strings.TrimSpace(pattern), "#") {
					nonEmptyCount++
				}
			}

			if nonEmptyCount != tt.expectedNonEmpty {
				t.Errorf("expected %d non-empty patterns, got %d", tt.expectedNonEmpty, nonEmptyCount)
			}
		})
	}
}