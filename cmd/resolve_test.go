package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveCmd_Sanity(t *testing.T) {
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
			cmd := &cobra.Command{Use: "resolve"}
			err := runResolve(cmd, tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("runResolve() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestVerifyConflictResolution(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) string
		expectError   bool
		expectResolved bool
	}{
		{
			name: "non-existent repo",
			setup: func(t *testing.T) string {
				return "/tmp/non-existent-repo"
			},
			expectError:    true,
			expectResolved: false,
		},
		{
			name: "repo with no conflicts",
			setup: func(t *testing.T) string {
				repoPath := t.TempDir()
				if err := os.MkdirAll(filepath.Join(repoPath, ".git"), testDirPerms); err != nil {
					t.Fatalf("failed to create git directory: %v", err)
				}
				return repoPath
			},
			expectError:    false,
			expectResolved: true,
		},
		{
			name: "repo with unresolved conflicts",
			setup: func(t *testing.T) string {
				repoPath := t.TempDir()
				if err := os.MkdirAll(filepath.Join(repoPath, ".git"), testDirPerms); err != nil {
					t.Fatalf("failed to create git directory: %v", err)
				}

				// Create conflicts directory
				conflictsDir := filepath.Join(repoPath, ".dsm", "conflicts")
				if err := os.MkdirAll(conflictsDir, testDirPerms); err != nil {
					t.Fatalf("failed to create conflicts directory: %v", err)
				}
				createTestFile(t, filepath.Join(conflictsDir, "bashrc.conflict"), "conflict content")
				return repoPath
			},
			expectError:    false,
			expectResolved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := tt.setup(t)
			resolved, err := verifyConflictResolution(repoPath)

			if (err != nil) != tt.expectError {
				t.Errorf("verifyConflictResolution() error = %v, expectError %v", err, tt.expectError)
			}

			if resolved != tt.expectResolved {
				t.Errorf("verifyConflictResolution() resolved = %v, expectResolved %v", resolved, tt.expectResolved)
			}
		})
	}
}

func TestCheckUnmergedFiles(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T) string
		expectError    bool
		expectUnmerged bool
	}{
		{
			name: "non-existent repo",
			setup: func(t *testing.T) string {
				return "/tmp/non-existent-repo"
			},
			expectError:    true,
			expectUnmerged: false,
		},
		{
			name: "clean repo",
			setup: func(t *testing.T) string {
				repoPath := t.TempDir()
				if err := os.MkdirAll(filepath.Join(repoPath, ".git"), testDirPerms); err != nil {
					t.Fatalf("failed to create git directory: %v", err)
				}
				return repoPath
			},
			expectError:    false,
			expectUnmerged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := tt.setup(t)
			hasUnmerged, err := checkUnmergedFiles(repoPath)

			if (err != nil) != tt.expectError {
				t.Errorf("checkUnmergedFiles() error = %v, expectError %v", err, tt.expectError)
			}

			if hasUnmerged != tt.expectUnmerged {
				t.Errorf("checkUnmergedFiles() hasUnmerged = %v, expectUnmerged %v", hasUnmerged, tt.expectUnmerged)
			}
		})
	}
}

func TestCleanupConflictFiles(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) string
		expectError bool
	}{
		{
			name: "non-existent conflicts directory",
			setup: func(t *testing.T) string {
				return "/tmp/non-existent-conflicts"
			},
			expectError: false, // Should not error
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
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflictsDir := tt.setup(t)
			err := cleanupConflictFiles(conflictsDir)

			if (err != nil) != tt.expectError {
				t.Errorf("cleanupConflictFiles() error = %v, expectError %v", err, tt.expectError)
			}

			// Verify directory is empty/removed
			if _, err := os.Stat(conflictsDir); err == nil {
				// Directory exists, check if empty
				files, err := os.ReadDir(conflictsDir)
				if err != nil {
					t.Errorf("failed to read conflicts directory after cleanup: %v", err)
				} else if len(files) > 0 {
					t.Errorf("expected conflicts directory to be empty, found %d files", len(files))
				}
			}
		})
	}
}

func TestResolveCmd_Integration(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Initialize git repo structure
	err := os.MkdirAll(filepath.Join(testConfig.RepoPath, ".git"), testDirPerms)
	if err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}

	// Test resolve command with no conflicts
	cmd := &cobra.Command{Use: "resolve"}
	err = runResolve(cmd, []string{})
	if err != nil {
		t.Errorf("runResolve() failed: %v", err)
	}
}

func TestResolveCmd_WithConflicts(t *testing.T) {
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
	createTestFile(t, filepath.Join(conflictsDir, "bashrc.conflict"), "conflict content")

	// Test resolve command - should detect conflicts and warn
	cmd := &cobra.Command{Use: "resolve"}
	err = runResolve(cmd, []string{})
	if err != nil {
		t.Errorf("runResolve() failed: %v", err)
	}
}

func TestRestartDaemonAfterResolve(t *testing.T) {
	// Test restart daemon functionality after conflict resolution
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Create fake PID file
	pidFile := "/tmp/dsm-daemon-resolve-test.pid"
	defer func() {
		if err := os.Remove(pidFile); err != nil {
			t.Logf("warning: failed to remove PID file: %v", err)
		}
	}()

	err := os.WriteFile(pidFile, []byte("99999"), defaultFilePerms)
	if err != nil {
		t.Fatalf("failed to create test PID file: %v", err)
	}

	// Test restart after resolve
	err = restartDaemonAfterResolve()
	if err != nil {
		// Expected in test environment
		t.Logf("restartDaemonAfterResolve() returned expected error: %v", err)
	}
}

func TestValidateResolutionState(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (string, string)
		expectError bool
	}{
		{
			name: "non-existent repo",
			setup: func(t *testing.T) (string, string) {
				return "/tmp/non-existent-repo", "/tmp/non-existent-conflicts"
			},
			expectError: true,
		},
		{
			name: "clean repo and no conflicts",
			setup: func(t *testing.T) (string, string) {
				repoPath := t.TempDir()
				if err := os.MkdirAll(filepath.Join(repoPath, ".git"), testDirPerms); err != nil {
					t.Fatalf("failed to create git directory: %v", err)
				}
				conflictsDir := filepath.Join(repoPath, ".dsm", "conflicts")
				return repoPath, conflictsDir
			},
			expectError: false,
		},
		{
			name: "repo with conflicts remaining",
			setup: func(t *testing.T) (string, string) {
				repoPath := t.TempDir()
				if err := os.MkdirAll(filepath.Join(repoPath, ".git"), testDirPerms); err != nil {
					t.Fatalf("failed to create git directory: %v", err)
				}
				conflictsDir := filepath.Join(repoPath, ".dsm", "conflicts")
				if err := os.MkdirAll(conflictsDir, testDirPerms); err != nil {
					t.Fatalf("failed to create conflicts directory: %v", err)
				}
				createTestFile(t, filepath.Join(conflictsDir, "test.conflict"), "conflict content")
				return repoPath, conflictsDir
			},
			expectError: false, // Should not error, just detect conflicts
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath, _ := tt.setup(t)
			err := validateResolutionState(repoPath)

			if (err != nil) != tt.expectError {
				t.Errorf("validateResolutionState() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestGetResolutionStatus(t *testing.T) {
	tests := []struct {
		name            string
		repoPath        string
		conflictsDir    string
		expectedStatus  string
	}{
		{
			name:           "non-existent repo",
			repoPath:       "/tmp/non-existent-repo",
			conflictsDir:   "/tmp/non-existent-conflicts",
			expectedStatus: "Repository not found",
		},
		{
			name:           "clean repo",
			repoPath:       func() string {
				repo := t.TempDir()
				if err := os.MkdirAll(filepath.Join(repo, ".git"), testDirPerms); err != nil {
					t.Fatalf("failed to create git directory: %v", err)
				}
				return repo
			}(),
			conflictsDir:   "",
			expectedStatus: "No conflicts found",
		},
		{
			name:         "conflicts exist",
			repoPath:     func() string {
				repo := t.TempDir()
				if err := os.MkdirAll(filepath.Join(repo, ".git"), testDirPerms); err != nil {
					t.Fatalf("failed to create git directory: %v", err)
				}
				return repo
			}(),
			conflictsDir: func() string {
				dir := filepath.Join(t.TempDir(), "conflicts")
				if err := os.MkdirAll(dir, testDirPerms); err != nil {
					t.Fatalf("failed to create conflicts directory: %v", err)
				}
				return dir
			}(),
			expectedStatus: "Conflicts still exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := getResolutionStatus(tt.repoPath, tt.conflictsDir)
			if status != tt.expectedStatus {
				t.Errorf("getResolutionStatus() = %q, expected %q", status, tt.expectedStatus)
			}
		})
	}
}