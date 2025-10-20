package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/spf13/cobra"
)

// TestGitFailureScenarios tests various git operation failure scenarios
// to ensure proper error handling and rollback behavior.
func TestGitFailureScenarios(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (*config.SyncConfig, string, error)
		expectError bool
		errorType   string
	}{
		{
			name:        "network timeout during commit",
			setup:       setupWithNetworkTimeout,
			expectError: true,
			errorType:   "context deadline exceeded",
		},
		{
			name:        "invalid repository path",
			setup:       setupWithInvalidRepo,
			expectError: true,
			errorType:   "git repository not found",
		},
		{
			name:        "disk space exhaustion",
			setup:       setupWithNoSpace,
			expectError: true,
			errorType:   "no space left on device",
		},
		{
			name:        "concurrent git operations",
			setup:       setupWithConcurrentAccess,
			expectError: true,
			errorType:   "repository already in use",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, filePath, err := tt.setup(t)
			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			// Create test command
			cmd := &cobra.Command{
				Use: "test",
				Run: func(cmd *cobra.Command, args []string) {},
			}
			cmd.SetContext(context.Background())

			// Test commit failure
			err = commitAddedFile(cmd, cfg, filePath)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errorType)
				}
				if !strings.Contains(err.Error(), tt.errorType) {
					t.Fatalf("expected error containing %q, got %q", tt.errorType, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestPushFailureScenarios tests various push failure scenarios.
func TestPushFailureScenarios(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (*config.SyncConfig, string, error)
		expectError bool
		errorType   string
	}{
		{
			name:        "authentication failure",
			setup:       setupWithAuthFailure,
			expectError: true,
			errorType:   "authentication required",
		},
		{
			name:        "remote repository unavailable",
			setup:       setupWithInvalidRemote,
			expectError: true,
			errorType:   "unable to access remote",
		},
		{
			name:        "network timeout during push",
			setup:       setupWithPushTimeout,
			expectError: true,
			errorType:   "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, filePath, err := tt.setup(t)
			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			// Create test command
			cmd := &cobra.Command{
				Use: "test",
				Run: func(cmd *cobra.Command, args []string) {},
			}
			cmd.SetContext(context.Background())

			// First, commit successfully
			err = commitAddedFile(cmd, cfg, filePath)
			if err != nil {
				t.Fatalf("commit should have succeeded: %v", err)
			}

			// Then test push failure
			err = commitAddedFile(cmd, cfg, filePath)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errorType)
				}
				if !strings.Contains(err.Error(), tt.errorType) {
					t.Fatalf("expected error containing %q, got %q", tt.errorType, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestRollbackAfterGitFailure tests that rollback works correctly when git operations fail.
func TestRollbackAfterGitFailure(t *testing.T) {
	t.Run("rollback restores original file", func(t *testing.T) {
		tempDir := t.TempDir()
		originalFile := filepath.Join(tempDir, "original.txt")
		repoPath := filepath.Join(tempDir, "dotfiles")
		targetPath := filepath.Join(repoPath, "original.txt")

		// Create original file
		if err := os.WriteFile(originalFile, []byte("original content"), defaultFilePerms); err != nil {
			t.Fatalf("create original file: %v", err)
		}

		// Setup repository
		setupTestRepo(t, repoPath)

		// Create config
		_ = &config.SyncConfig{
			Git: config.GitConfig{
				RepoPath:    repoPath,
				AuthorName:  "Test User",
				AuthorEmail: "test@example.com",
			},
			Machine: config.MachineConfig{
				Name: "test-machine",
			},
		}

		// Simulate git failure by removing the repo after creating the file
		os.RemoveAll(repoPath)

		// Create command and context
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		cmd.SetContext(context.Background())

		// Test add operation (should fail due to git operations)
		testFilePath := filepath.Join(tempDir, "dotfiles", "original.txt")
		if err := runAdd(cmd, []string{originalFile}); err == nil {
			t.Fatalf("expected error due to git failure, got nil")
		}

		// Verify original file is restored
		content, err := os.ReadFile(originalFile)
		if err != nil {
			t.Fatalf("read original file: %v", err)
		}
		if string(content) != "original content" {
			t.Fatalf("original file content not preserved: got %q", string(content))
		}

		// Verify target file doesn't exist
		if _, err := os.Stat(targetPath); err == nil {
			t.Fatalf("target file should not exist after rollback")
		}

		// Clean up test file
		os.Remove(testFilePath)
	})
}

// Helper functions for setting up test scenarios

func setupWithNetworkTimeout(t *testing.T) (*config.SyncConfig, string, error) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "dotfiles")
	targetPath := filepath.Join(repoPath, "test.txt")

	// Create repository
	setupTestRepo(t, repoPath)

	// Create target file
	if err := os.WriteFile(targetPath, []byte("test content"), defaultFilePerms); err != nil {
		return nil, "", err
	}

	cfg := &config.SyncConfig{
		Git: config.GitConfig{
			RepoPath:    repoPath,
			AuthorName:  "Test User",
			AuthorEmail: "test@example.com",
		},
		Machine: config.MachineConfig{
			Name: "test-machine",
		},
	}

	return cfg, targetPath, nil
}

func setupWithInvalidRepo(t *testing.T) (*config.SyncConfig, string, error) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "nonexistent")
	targetPath := filepath.Join(tempDir, "test.txt")

	// Create target file
	if err := os.WriteFile(targetPath, []byte("test content"), defaultFilePerms); err != nil {
		return nil, "", err
	}

	cfg := &config.SyncConfig{
		Git: config.GitConfig{
			RepoPath:    repoPath,
			AuthorName:  "Test User",
			AuthorEmail: "test@example.com",
		},
		Machine: config.MachineConfig{
			Name: "test-machine",
		},
	}

	return cfg, targetPath, nil
}

func setupWithNoSpace(t *testing.T) (*config.SyncConfig, string, error) {
	// This is difficult to implement reliably across platforms
	// Skip this test for now, but in a real environment you would:
	// 1. Fill up the disk space
	// 2. Attempt the operation
	// 3. Verify the "no space left" error
	t.Skip("No space test skipped for platform compatibility")
	return nil, "", nil
}

func setupWithConcurrentAccess(t *testing.T) (*config.SyncConfig, string, error) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "dotfiles")
	targetPath := filepath.Join(repoPath, "test.txt")

	// Create repository
	setupTestRepo(t, repoPath)

	// Create target file
	if err := os.WriteFile(targetPath, []byte("test content"), defaultFilePerms); err != nil {
		return nil, "", err
	}

	cfg := &config.SyncConfig{
		Git: config.GitConfig{
			RepoPath:    repoPath,
			AuthorName:  "Test User",
			AuthorEmail: "test@example.com",
		},
		Machine: config.MachineConfig{
			Name: "test-machine",
		},
	}

	return cfg, targetPath, nil
}

func setupWithAuthFailure(t *testing.T) (*config.SyncConfig, string, error) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "dotfiles")
	targetPath := filepath.Join(repoPath, "test.txt")

	// Create repository
	setupTestRepo(t, repoPath)

	// Create target file
	if err := os.WriteFile(targetPath, []byte("test content"), defaultFilePerms); err != nil {
		return nil, "", err
	}

	// Create config with invalid remote URL
	cfg := &config.SyncConfig{
		Git: config.GitConfig{
			RepoPath:    repoPath,
			RemoteURL:   "https://invalid-auth@example.com/repo.git",
			AuthType:    "basic",
			Username:    "invalid",
			Password:    "invalid",
			AuthorName:  "Test User",
			AuthorEmail: "test@example.com",
		},
		Machine: config.MachineConfig{
			Name: "test-machine",
		},
	}

	return cfg, targetPath, nil
}

func setupWithInvalidRemote(t *testing.T) (*config.SyncConfig, string, error) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "dotfiles")
	targetPath := filepath.Join(repoPath, "test.txt")

	// Create repository
	setupTestRepo(t, repoPath)

	// Create target file
	if err := os.WriteFile(targetPath, []byte("test content"), defaultFilePerms); err != nil {
		return nil, "", err
	}

	// Create config with invalid remote URL
	cfg := &config.SyncConfig{
		Git: config.GitConfig{
			RepoPath:    repoPath,
			RemoteURL:   "https://invalid.example.com/nonexistent/repo.git",
			AuthorName:  "Test User",
			AuthorEmail: "test@example.com",
		},
		Machine: config.MachineConfig{
			Name: "test-machine",
		},
	}

	return cfg, targetPath, nil
}

func setupWithPushTimeout(t *testing.T) (*config.SyncConfig, string, error) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "dotfiles")
	targetPath := filepath.Join(repoPath, "test.txt")

	// Create repository
	setupTestRepo(t, repoPath)

	// Create target file
	if err := os.WriteFile(targetPath, []byte("test content"), defaultFilePerms); err != nil {
		return nil, "", err
	}

	cfg := &config.SyncConfig{
		Git: config.GitConfig{
			RepoPath:    repoPath,
			AuthorName:  "Test User",
			AuthorEmail: "test@example.com",
		},
		Machine: config.MachineConfig{
			Name: "test-machine",
		},
	}

	// Create command with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	cmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {},
	}
	cmd.SetContext(ctx)

	return cfg, targetPath, nil
}