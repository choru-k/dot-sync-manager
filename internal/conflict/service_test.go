package conflict

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

// Test file permission constants per Rule 18.
const (
	testDirPerms  = 0755
	testFilePerms = 0644
)

// createTestConflict creates a gitmanager-style conflict directory structure.
func createTestConflict(t *testing.T, repoDir, file string, withBase bool) string {
	t.Helper()

	// Use a fixed timestamp for consistent testing
	timestamp := time.Now().Format("20060102T150405Z0700")
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts", timestamp)
	if err := os.MkdirAll(conflictDir, testDirPerms); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	// Create conflict files with suffix naming
	if err := os.WriteFile(filepath.Join(conflictDir, file+".local"), []byte("local content"), testFilePerms); err != nil {
		t.Fatalf("Failed to write local file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, file+".remote"), []byte("remote content"), testFilePerms); err != nil {
		t.Fatalf("Failed to write remote file: %v", err)
	}
	if withBase {
		if err := os.WriteFile(filepath.Join(conflictDir, file+".base"), []byte("base content"), testFilePerms); err != nil {
			t.Fatalf("Failed to write base file: %v", err)
		}
	}

	return conflictDir
}

func TestService_GetConflicts_Empty(t *testing.T) {
	repoDir := t.TempDir()

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	conflicts, err := svc.GetConflicts()

	if err != nil {
		t.Fatalf("GetConflicts failed: %v", err)
	}

	if len(conflicts) != 0 {
		t.Errorf("Expected 0 conflicts, got %d", len(conflicts))
	}
}

func TestService_GetConflicts_WithConflicts(t *testing.T) {
	repoDir := t.TempDir()
	createTestConflict(t, repoDir, ".bashrc", true)

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	conflicts, err := svc.GetConflicts()

	if err != nil {
		t.Fatalf("GetConflicts failed: %v", err)
	}

	if len(conflicts) != 1 {
		t.Fatalf("Expected 1 conflict, got %d", len(conflicts))
	}

	if conflicts[0].File != ".bashrc" {
		t.Errorf("Expected .bashrc, got %s", conflicts[0].File)
	}

	if !conflicts[0].HasBase {
		t.Error("Expected HasBase to be true")
	}
}

func TestService_GetConflicts_MultipleFiles(t *testing.T) {
	repoDir := t.TempDir()

	// Create multiple conflicts in the same timestamp directory
	timestamp := time.Now().Format("20060102T150405Z0700")
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts", timestamp)
	if err := os.MkdirAll(conflictDir, testDirPerms); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	// Create .bashrc conflict
	if err := os.WriteFile(filepath.Join(conflictDir, ".bashrc.local"), []byte("local"), testFilePerms); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, ".bashrc.remote"), []byte("remote"), testFilePerms); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Create .vimrc conflict
	if err := os.WriteFile(filepath.Join(conflictDir, ".vimrc.local"), []byte("local"), testFilePerms); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, ".vimrc.remote"), []byte("remote"), testFilePerms); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	conflicts, err := svc.GetConflicts()

	if err != nil {
		t.Fatalf("GetConflicts failed: %v", err)
	}

	if len(conflicts) != 2 {
		t.Fatalf("Expected 2 conflicts, got %d", len(conflicts))
	}

	// Conflicts should be sorted alphabetically
	if conflicts[0].File != ".bashrc" {
		t.Errorf("Expected first conflict to be .bashrc, got %s", conflicts[0].File)
	}
	if conflicts[1].File != ".vimrc" {
		t.Errorf("Expected second conflict to be .vimrc, got %s", conflicts[1].File)
	}
}

func TestService_HasConflicts(t *testing.T) {
	repoDir := t.TempDir()

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir
	svc := NewService(nil, cfg)

	// Initially no conflicts
	if svc.HasConflicts() {
		t.Error("Should have no conflicts initially")
	}

	// Create conflict
	createTestConflict(t, repoDir, ".bashrc", false)

	// Now should have conflicts
	if !svc.HasConflicts() {
		t.Error("Should have conflicts after creating conflict files")
	}
}

func TestService_GetConflictDir(t *testing.T) {
	repoDir := t.TempDir()

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	expectedDir := filepath.Join(repoDir, ".dsm", "conflicts")

	if svc.GetConflictDir() != expectedDir {
		t.Errorf("Expected conflict dir %s, got %s", expectedDir, svc.GetConflictDir())
	}
}

func TestService_GetConflictDetails(t *testing.T) {
	repoDir := t.TempDir()
	createTestConflict(t, repoDir, ".bashrc", true)

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	details, err := svc.GetConflictDetails(".bashrc")

	if err != nil {
		t.Fatalf("GetConflictDetails failed: %v", err)
	}

	if string(details.LocalContent) != "local content" {
		t.Errorf("Expected 'local content', got %s", details.LocalContent)
	}

	if string(details.RemoteContent) != "remote content" {
		t.Errorf("Expected 'remote content', got %s", details.RemoteContent)
	}

	if string(details.BaseContent) != "base content" {
		t.Errorf("Expected 'base content', got %s", details.BaseContent)
	}
}

func TestService_GetConflictDetails_NoBase(t *testing.T) {
	repoDir := t.TempDir()
	createTestConflict(t, repoDir, ".bashrc", false)

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	details, err := svc.GetConflictDetails(".bashrc")

	if err != nil {
		t.Fatalf("GetConflictDetails failed: %v", err)
	}

	if string(details.LocalContent) != "local content" {
		t.Errorf("Expected 'local content', got %s", details.LocalContent)
	}

	if string(details.RemoteContent) != "remote content" {
		t.Errorf("Expected 'remote content', got %s", details.RemoteContent)
	}

	if len(details.BaseContent) != 0 {
		t.Errorf("Expected empty base content, got %s", details.BaseContent)
	}
}

func TestService_GetConflictDetails_NotFound(t *testing.T) {
	repoDir := t.TempDir()

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	_, err = svc.GetConflictDetails(".nonexistent")

	if err == nil {
		t.Error("Expected error for non-existent conflict")
	}
}

func TestService_GetConflicts_LatestTimestamp(t *testing.T) {
	repoDir := t.TempDir()

	// Create old conflict
	oldTimestamp := time.Now().Add(-1 * time.Hour).Format("20060102T150405Z0700")
	oldDir := filepath.Join(repoDir, ".dsm", "conflicts", oldTimestamp)
	if err := os.MkdirAll(oldDir, testDirPerms); err != nil {
		t.Fatalf("Failed to create old conflict directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, ".bashrc.local"), []byte("old local"), testFilePerms); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, ".bashrc.remote"), []byte("old remote"), testFilePerms); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Create new conflict
	newTimestamp := time.Now().Format("20060102T150405Z0700")
	newDir := filepath.Join(repoDir, ".dsm", "conflicts", newTimestamp)
	if err := os.MkdirAll(newDir, testDirPerms); err != nil {
		t.Fatalf("Failed to create new conflict directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(newDir, ".bashrc.local"), []byte("new local"), testFilePerms); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(newDir, ".bashrc.remote"), []byte("new remote"), testFilePerms); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)

	// GetConflicts should return the latest timestamp's info
	conflicts, err := svc.GetConflicts()
	if err != nil {
		t.Fatalf("GetConflicts failed: %v", err)
	}

	if len(conflicts) != 1 {
		t.Fatalf("Expected 1 conflict, got %d", len(conflicts))
	}

	// GetConflictDetails should read from the latest timestamp
	details, err := svc.GetConflictDetails(".bashrc")
	if err != nil {
		t.Fatalf("GetConflictDetails failed: %v", err)
	}

	if string(details.LocalContent) != "new local" {
		t.Errorf("Expected 'new local', got %s", details.LocalContent)
	}
}
