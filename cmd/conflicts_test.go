package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestConflictsCmd_Sanity(t *testing.T) {
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
			cmd := &cobra.Command{Use: "conflicts"}
			err := runConflicts(cmd, tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("runConflicts() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestDetectConflicts(t *testing.T) {
	tests := []struct {
		name          string
		setupRepo     func(t *testing.T) string
		expectError   bool
		expectConflicts bool
	}{
		{
			name: "non-existent repo",
			setupRepo: func(t *testing.T) string {
				return "/tmp/non-existent-repo"
			},
			expectError:    true,
			expectConflicts: false,
		},
		{
			name: "clean repo",
			setupRepo: func(t *testing.T) string {
				repoPath := t.TempDir()
				// Create git repo structure
				if err := os.MkdirAll(filepath.Join(repoPath, ".git"), testDirPerms); err != nil {
					t.Fatalf("failed to create git directory: %v", err)
				}
				return repoPath
			},
			expectError:    false,
			expectConflicts: false,
		},
		{
			name: "repo with conflicts directory",
			setupRepo: func(t *testing.T) string {
				repoPath := t.TempDir()
				if err := os.MkdirAll(filepath.Join(repoPath, ".git"), testDirPerms); err != nil {
					t.Fatalf("failed to create git directory: %v", err)
				}

				// Create conflicts directory with files
				conflictsDir := filepath.Join(repoPath, ".dsm", "conflicts")
				if err := os.MkdirAll(conflictsDir, testDirPerms); err != nil {
					t.Fatalf("failed to create conflicts directory: %v", err)
				}
				createTestFile(t, filepath.Join(conflictsDir, "bashrc.conflict"), "conflict content")
				return repoPath
			},
			expectError:    false,
			expectConflicts: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := tt.setupRepo(t)
			hasConflicts, conflicts, err := detectConflicts(repoPath)

			if (err != nil) != tt.expectError {
				t.Errorf("detectConflicts() error = %v, expectError %v", err, tt.expectError)
			}

			if !tt.expectError && hasConflicts != tt.expectConflicts {
				t.Errorf("detectConflicts() hasConflicts = %v, expectConflicts %v", hasConflicts, tt.expectConflicts)
			}

			if tt.expectConflicts && len(conflicts) == 0 {
				t.Error("expected conflicts list to be non-empty")
			}
		})
	}
}

func TestListConflictFiles(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) string
		expectCount int
		expectError bool
	}{
		{
			name: "non-existent conflicts directory",
			setup: func(t *testing.T) string {
				return "/tmp/non-existent-conflicts"
			},
			expectCount: 0,
			expectError: false,
		},
		{
			name: "empty conflicts directory",
			setup: func(t *testing.T) string {
				conflictsDir := filepath.Join(t.TempDir(), "conflicts")
				if err := os.MkdirAll(conflictsDir, testDirPerms); err != nil {
					t.Fatalf("failed to create conflicts directory: %v", err)
				}
				return conflictsDir
			},
			expectCount: 0,
			expectError: false,
		},
		{
			name: "conflicts directory with files",
			setup: func(t *testing.T) string {
				conflictsDir := filepath.Join(t.TempDir(), "conflicts")
				if err := os.MkdirAll(conflictsDir, testDirPerms); err != nil {
					t.Fatalf("failed to create conflicts directory: %v", err)
				}
				createTestFile(t, filepath.Join(conflictsDir, "file1.conflict"), "content1")
				createTestFile(t, filepath.Join(conflictsDir, "file2.conflict"), "content2")
				return conflictsDir
			},
			expectCount: 2,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflictsDir := tt.setup(t)
			conflicts, err := listConflictFiles(conflictsDir)

			if (err != nil) != tt.expectError {
				t.Errorf("listConflictFiles() error = %v, expectError %v", err, tt.expectError)
			}

			if len(conflicts) != tt.expectCount {
				t.Errorf("listConflictFiles() returned %d files, expected %d", len(conflicts), tt.expectCount)
			}
		})
	}
}

func TestShowConflictDetails(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) string
		expectError bool
	}{
		{
			name: "non-existent conflict file",
			setup: func(t *testing.T) string {
				return "/tmp/non-existent.conflict"
			},
			expectError: true,
		},
		{
			name: "valid conflict file",
			setup: func(t *testing.T) string {
				conflictFile := filepath.Join(t.TempDir(), "test.conflict")
				content := `<<<<<<< HEAD
Local content
=======
Remote content
>>>>>>> branch-name`
				createTestFile(t, conflictFile, content)
				return conflictFile
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflictFile := tt.setup(t)
			err := showConflictDetails(conflictFile)

			if (err != nil) != tt.expectError {
				t.Errorf("showConflictDetails() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestConflictsCmd_Integration(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Initialize git repo structure
	err := os.MkdirAll(filepath.Join(testConfig.RepoPath, ".git"), testDirPerms)
	if err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}

	// Create conflicts directory with conflict files
	conflictsDir := filepath.Join(testConfig.RepoPath, ".dsm", "conflicts")
	if err := os.MkdirAll(conflictsDir, testDirPerms); err != nil {
					t.Fatalf("failed to create conflicts directory: %v", err)
				}

	conflictContent := `<<<<<<< HEAD
export PATH="$PATH:/usr/local/bin"
=======
export PATH="$PATH:/opt/local/bin"
>>>>>>> main`
	createTestFile(t, filepath.Join(conflictsDir, "bashrc.conflict"), conflictContent)

	// Test conflicts command
	cmd := &cobra.Command{Use: "conflicts"}
	err = runConflicts(cmd, []string{})
	if err != nil {
		t.Errorf("runConflicts() failed: %v", err)
	}
}

func TestConflictsCmd_NoConflicts(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Initialize git repo structure but no conflicts
	err := os.MkdirAll(filepath.Join(testConfig.RepoPath, ".git"), testDirPerms)
	if err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}

	// Test conflicts command with no conflicts
	cmd := &cobra.Command{Use: "conflicts"}
	err = runConflicts(cmd, []string{})
	if err != nil {
		t.Errorf("runConflicts() failed: %v", err)
	}
}

func TestConflictsCmd_MissingRepo(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Don't create repo structure - test should handle missing repo gracefully
	cmd := &cobra.Command{Use: "conflicts"}
	err := runConflicts(cmd, []string{})
	if err == nil {
		t.Error("expected runConflicts() to fail with missing repo")
	}
}

func TestParseConflictFile(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectError    bool
		expectSections int
	}{
		{
			name:           "empty file",
			content:        "",
			expectError:    false,
			expectSections: 0,
		},
		{
			name: "valid conflict format",
			content: `<<<<<<< HEAD
Local content
=======
Remote content
>>>>>>> branch-name`,
			expectError:    false,
			expectSections: 2,
		},
		{
			name: "file without conflict markers",
			content: "Just regular content\nwith no conflicts",
			expectError:    false,
			expectSections: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflictFile := filepath.Join(t.TempDir(), "test.conflict")
			createTestFile(t, conflictFile, tt.content)

			sections, err := parseConflictFile(conflictFile)

			if (err != nil) != tt.expectError {
				t.Errorf("parseConflictFile() error = %v, expectError %v", err, tt.expectError)
			}

			if len(sections) != tt.expectSections {
				t.Errorf("parseConflictFile() returned %d sections, expected %d", len(sections), tt.expectSections)
			}
		})
	}
}

func TestGetConflictStatus(t *testing.T) {
	tests := []struct {
		name           string
		conflictsDir   string
		expectedStatus string
	}{
		{
			name:           "no conflicts directory",
			conflictsDir:   "/tmp/non-existent",
			expectedStatus: "No conflicts detected",
		},
		{
			name:         "empty conflicts directory",
			conflictsDir: func() string {
				dir := filepath.Join(t.TempDir(), "conflicts")
				if err := os.MkdirAll(dir, testDirPerms); err != nil {
					t.Fatalf("failed to create directory: %v", err)
				}
				return dir
			}(),
			expectedStatus: "No conflicts detected",
		},
		{
			name:         "conflicts exist",
			conflictsDir: func() string {
				dir := filepath.Join(t.TempDir(), "conflicts")
				if err := os.MkdirAll(dir, testDirPerms); err != nil {
					t.Fatalf("failed to create directory: %v", err)
				}
				createTestFile(t, filepath.Join(dir, "test.conflict"), "content")
				return dir
			}(),
			expectedStatus: "Conflicts detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := getConflictStatus(tt.conflictsDir)
			if status != tt.expectedStatus {
				t.Errorf("getConflictStatus() = %q, expected %q", status, tt.expectedStatus)
			}
		})
	}
}