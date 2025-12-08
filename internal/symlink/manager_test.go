package symlink_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/symlink"
)

const (
	// testFilePerms allows owner write, all read (standard test files)
	testFilePerms os.FileMode = 0644 // -rw-r--r--
)

func TestManager_CreateLink_SourceNotExists(t *testing.T) {
	// Setup temp directories
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create manager with config
	cfg := &config.SyncConfig{}
	cfg.Git.RepoPath = repoDir
	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Try to create link with non-existent source
	target := filepath.Join(targetDir, ".bashrc")
	err = mgr.CreateLink(".nonexistent", target)

	// Expect error
	if err == nil {
		t.Fatal("Expected error for non-existent source, got nil")
	}
	if !strings.Contains(err.Error(), "SYMLINK_SOURCE_NOT_FOUND") {
		t.Errorf("Expected error to contain [SYMLINK_SOURCE_NOT_FOUND], got: %v", err)
	}
}

func TestManager_CreateLink_TargetParentNotExists(t *testing.T) {
	repoDir := t.TempDir()

	// Create source file in repo
	sourceFile := filepath.Join(repoDir, ".bashrc")
	if err := os.WriteFile(sourceFile, []byte("# bash config"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	cfg := &config.SyncConfig{}
	cfg.Git.RepoPath = repoDir
	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Target with non-existent parent directory
	target := "/nonexistent/directory/.bashrc"
	err = mgr.CreateLink(".bashrc", target)

	if err == nil {
		t.Fatal("Expected error for non-existent target parent")
	}
	if !strings.Contains(err.Error(), "SYMLINK_TARGET_PARENT_NOT_FOUND") {
		t.Errorf("Expected error to contain [SYMLINK_TARGET_PARENT_NOT_FOUND], got: %v", err)
	}
}

func TestManager_CreateLink_TargetExists(t *testing.T) {
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create source file in repo
	sourceFile := filepath.Join(repoDir, ".bashrc")
	if err := os.WriteFile(sourceFile, []byte("# bash config"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Create existing file at target
	target := filepath.Join(targetDir, ".bashrc")
	if err := os.WriteFile(target, []byte("# existing"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	cfg := &config.SyncConfig{}
	cfg.Git.RepoPath = repoDir
	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	err = mgr.CreateLink(".bashrc", target)

	if err == nil {
		t.Fatal("Expected error for existing target file")
	}
	if !strings.Contains(err.Error(), "SYMLINK_TARGET_EXISTS") {
		t.Errorf("Expected error to contain [SYMLINK_TARGET_EXISTS], got: %v", err)
	}
}

func TestManager_CreateLink_Success(t *testing.T) {
	// Setup temp directories
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create source file in repo
	sourceFile := filepath.Join(repoDir, ".bashrc")
	if err := os.WriteFile(sourceFile, []byte("# bash config"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Create manager with config
	cfg := &config.SyncConfig{}
	cfg.Git.RepoPath = repoDir
	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create symlink
	target := filepath.Join(targetDir, ".bashrc")
	err = mgr.CreateLink(".bashrc", target)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// Verify symlink exists and points to correct location
	linkDest, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if linkDest != sourceFile {
		t.Errorf("Symlink points to %s, expected %s", linkDest, sourceFile)
	}
}

func TestManager_RemoveLink(t *testing.T) {
	// Setup common test resources
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create source file for symlink test
	sourceFile := filepath.Join(repoDir, ".bashrc")
	if err := os.WriteFile(sourceFile, []byte("# bash"), testFilePerms); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Create regular file for non-symlink test
	regularFile := filepath.Join(targetDir, ".vimrc")
	if err := os.WriteFile(regularFile, []byte("# regular file"), testFilePerms); err != nil {
		t.Fatalf("Failed to write regular file: %v", err)
	}

	// Create symlink for success test
	symlinkTarget := filepath.Join(targetDir, ".bashrc")
	if err := os.Symlink(sourceFile, symlinkTarget); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	testCases := []struct {
		name        string
		target      string
		expectErr   bool
		errContains string
		postCheck   func(t *testing.T)
	}{
		{
			name:        "TargetNotFound",
			target:      "/nonexistent/path/.zshrc",
			expectErr:   true,
			errContains: "SYMLINK_TARGET_NOT_FOUND",
		},
		{
			name:        "NotASymlink",
			target:      regularFile,
			expectErr:   true,
			errContains: "SYMLINK_NOT_A_SYMLINK",
			postCheck: func(t *testing.T) {
				// Verify regular file was not removed
				if _, err := os.Stat(regularFile); os.IsNotExist(err) {
					t.Error("Regular file should not have been removed")
				}
			},
		},
		{
			name:      "Success",
			target:    symlinkTarget,
			expectErr: false,
			postCheck: func(t *testing.T) {
				// Verify symlink was removed
				if _, err := os.Lstat(symlinkTarget); !os.IsNotExist(err) {
					t.Error("Symlink should have been removed")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use DefaultConfig as per Rule 18
			cfg, err := config.DefaultConfig()
			if err != nil {
				t.Fatalf("DefaultConfig() failed: %v", err)
			}
			cfg.Git.RepoPath = repoDir

			mgr, err := symlink.NewManager(cfg)
			if err != nil {
				t.Fatalf("NewManager failed: %v", err)
			}

			err = mgr.RemoveLink(tc.target)

			if tc.expectErr {
				if err == nil {
					t.Fatal("Expected an error, but got nil")
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("Expected error to contain '%s', but got: %v", tc.errContains, err)
				}
			} else if err != nil {
				t.Fatalf("Expected no error, but got: %v", err)
			}

			if tc.postCheck != nil {
				tc.postCheck(t)
			}
		})
	}
}

func TestManager_VerifyMappings_AllValid(t *testing.T) {
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create source files in repo
	bashrc := filepath.Join(repoDir, ".bashrc")
	vimrc := filepath.Join(repoDir, ".vimrc")
	if err := os.WriteFile(bashrc, []byte("# bash"), testFilePerms); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vimrc, []byte("# vim"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Create symlinks at targets
	bashrcTarget := filepath.Join(targetDir, ".bashrc")
	vimrcTarget := filepath.Join(targetDir, ".vimrc")
	if err := os.Symlink(bashrc, bashrcTarget); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(vimrc, vimrcTarget); err != nil {
		t.Fatal(err)
	}

	// Create config with mappings
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.Mappings = map[string]string{
		".bashrc": bashrcTarget,
		".vimrc":  vimrcTarget,
	}

	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	results := mgr.VerifyMappings()

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Status != symlink.StateValid {
			t.Errorf("Expected valid status for %s, got %s: %s", r.RepoPath, r.Status, r.Error)
		}
	}
}

func TestManager_VerifyMappings_BrokenLink(t *testing.T) {
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create source file in repo
	bashrc := filepath.Join(repoDir, ".bashrc")
	if err := os.WriteFile(bashrc, []byte("# bash"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Create symlink at target
	bashrcTarget := filepath.Join(targetDir, ".bashrc")
	if err := os.Symlink(bashrc, bashrcTarget); err != nil {
		t.Fatal(err)
	}

	// Now delete source file to make broken link
	if err := os.Remove(bashrc); err != nil {
		t.Fatal(err)
	}

	// Create config with mapping
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.Mappings = map[string]string{
		".bashrc": bashrcTarget,
	}

	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	results := mgr.VerifyMappings()

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Status != symlink.StateBroken {
		t.Errorf("Expected broken status, got %s", results[0].Status)
	}
}

func TestManager_VerifyMappings_MissingSymlink(t *testing.T) {
	repoDir := t.TempDir()

	// Create config with mapping, but no symlink exists
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.Mappings = map[string]string{
		".bashrc": "/nonexistent/.bashrc",
	}

	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	results := mgr.VerifyMappings()

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Status != symlink.StateMissing {
		t.Errorf("Expected missing status, got %s", results[0].Status)
	}
}

func TestManager_VerifyMappings_NotSymlink(t *testing.T) {
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create regular file at target (not a symlink)
	target := filepath.Join(targetDir, ".bashrc")
	if err := os.WriteFile(target, []byte("# regular file"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Create config with mapping
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.Mappings = map[string]string{
		".bashrc": target,
	}

	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	results := mgr.VerifyMappings()

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Status != symlink.StateNotSymlink {
		t.Errorf("Expected not_symlink status, got %s", results[0].Status)
	}
}

func TestManager_VerifyMappings_RelativeSymlink(t *testing.T) {
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create source file in repo
	bashrc := filepath.Join(repoDir, ".bashrc")
	if err := os.WriteFile(bashrc, []byte("# bash"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Create RELATIVE symlink (not absolute)
	bashrcTarget := filepath.Join(targetDir, ".bashrc")
	relPath, err := filepath.Rel(targetDir, bashrc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(relPath, bashrcTarget); err != nil {
		t.Fatal(err)
	}

	// Create config with mapping
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.Mappings = map[string]string{
		".bashrc": bashrcTarget,
	}

	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	results := mgr.VerifyMappings()

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Status != symlink.StateValid {
		t.Errorf("Expected valid status for relative symlink, got %s: %s",
			results[0].Status, results[0].Error)
	}
}
