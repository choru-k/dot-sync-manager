package conflict

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

func TestService_GetConflicts_Empty(t *testing.T) {
	repoDir := t.TempDir()

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	conflicts := svc.GetConflicts()

	if len(conflicts) != 0 {
		t.Errorf("Expected 0 conflicts, got %d", len(conflicts))
	}
}

func TestService_GetConflicts_WithConflicts(t *testing.T) {
	repoDir := t.TempDir()
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts", ".bashrc")
	if err := os.MkdirAll(conflictDir, 0755); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	// Create metadata
	info := ConflictInfo{
		File:       ".bashrc",
		DetectedAt: time.Now(),
		LocalMod:   time.Now().Add(-1 * time.Hour),
		RemoteMod:  time.Now().Add(-30 * time.Minute),
		HasBase:    true,
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal conflict info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, "metadata.json"), data, 0644); err != nil {
		t.Fatalf("Failed to write metadata: %v", err)
	}

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	conflicts := svc.GetConflicts()

	if len(conflicts) != 1 {
		t.Fatalf("Expected 1 conflict, got %d", len(conflicts))
	}

	if conflicts[0].File != ".bashrc" {
		t.Errorf("Expected .bashrc, got %s", conflicts[0].File)
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

	// Create conflict directory
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts", ".bashrc")
	if err := os.MkdirAll(conflictDir, 0755); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	// Now should have conflicts
	if !svc.HasConflicts() {
		t.Error("Should have conflicts after creating directory")
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
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts", ".bashrc")
	if err := os.MkdirAll(conflictDir, 0755); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	// Create conflict files
	if err := os.WriteFile(filepath.Join(conflictDir, "local"), []byte("local content"), 0644); err != nil {
		t.Fatalf("Failed to write local file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, "remote"), []byte("remote content"), 0644); err != nil {
		t.Fatalf("Failed to write remote file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, "base"), []byte("base content"), 0644); err != nil {
		t.Fatalf("Failed to write base file: %v", err)
	}

	// Create metadata
	info := ConflictInfo{
		File:       ".bashrc",
		DetectedAt: time.Now(),
		HasBase:    true,
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal conflict info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, "metadata.json"), data, 0644); err != nil {
		t.Fatalf("Failed to write metadata: %v", err)
	}

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
		t.Errorf("Expected local content, got %s", details.LocalContent)
	}

	if string(details.RemoteContent) != "remote content" {
		t.Errorf("Expected remote content, got %s", details.RemoteContent)
	}

	if string(details.BaseContent) != "base content" {
		t.Errorf("Expected base content, got %s", details.BaseContent)
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
