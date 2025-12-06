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
