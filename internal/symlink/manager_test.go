package symlink_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/symlink"
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
	if err := os.WriteFile(sourceFile, []byte("# bash config"), 0644); err != nil {
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
	if err := os.WriteFile(sourceFile, []byte("# bash config"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create existing file at target
	target := filepath.Join(targetDir, ".bashrc")
	if err := os.WriteFile(target, []byte("# existing"), 0644); err != nil {
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
	if err := os.WriteFile(sourceFile, []byte("# bash config"), 0644); err != nil {
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

func TestManager_RemoveLink_TargetNotFound(t *testing.T) {
	cfg := &config.SyncConfig{}
	cfg.Git.RepoPath = t.TempDir() // Need valid path even though not used
	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Try to remove non-existent symlink
	err = mgr.RemoveLink("/nonexistent/path/.bashrc")

	if err == nil {
		t.Fatal("Expected error for non-existent target, got nil")
	}
	if !strings.Contains(err.Error(), "SYMLINK_TARGET_NOT_FOUND") {
		t.Errorf("Expected error to contain [SYMLINK_TARGET_NOT_FOUND], got: %v", err)
	}
}

func TestManager_RemoveLink_NotSymlink(t *testing.T) {
	targetDir := t.TempDir()

	// Create regular file (not symlink)
	target := filepath.Join(targetDir, ".bashrc")
	if err := os.WriteFile(target, []byte("# regular file"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.SyncConfig{}
	cfg.Git.RepoPath = t.TempDir() // Need valid path even though not used
	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Try to remove regular file as symlink
	err = mgr.RemoveLink(target)

	if err == nil {
		t.Fatal("Expected error for non-symlink target")
	}
	if !strings.Contains(err.Error(), "SYMLINK_NOT_A_SYMLINK") {
		t.Errorf("Expected error to contain [SYMLINK_NOT_A_SYMLINK], got: %v", err)
	}

	// Verify file still exists (safety check - should not remove non-symlinks)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		t.Error("Regular file should not be removed")
	}
}
