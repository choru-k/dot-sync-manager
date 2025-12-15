package conflict

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

func TestActions_UseLocal(t *testing.T) {
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create conflict in timestamp-based format
	timestamp := time.Now().Format("20060102T150405Z0700")
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts", timestamp)
	if err := os.MkdirAll(conflictDir, testDirPerms); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	localContent := []byte("local version content")
	if err := os.WriteFile(filepath.Join(conflictDir, ".bashrc.local"), localContent, testFilePerms); err != nil {
		t.Fatalf("Failed to write local file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, ".bashrc.remote"), []byte("remote content"), testFilePerms); err != nil {
		t.Fatalf("Failed to write remote file: %v", err)
	}

	// Setup config with mapping
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir
	targetPath := filepath.Join(targetDir, ".bashrc")
	cfg.Mappings = map[string]string{".bashrc": targetPath}

	svc := NewService(nil, cfg)
	notifier := &mockNotifier{}
	svc.SetNotifier(notifier)

	// Use local version
	err = svc.UseLocal(".bashrc")
	if err != nil {
		t.Fatalf("UseLocal failed: %v", err)
	}

	// Verify target file has local content
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read target: %v", err)
	}
	if string(content) != "local version content" {
		t.Errorf("Expected local content, got %s", content)
	}

	// Verify conflict directory removed
	if _, err := os.Stat(conflictDir); !os.IsNotExist(err) {
		t.Error("Conflict directory should be removed")
	}

	// Verify events fired
	if len(notifier.resolvedCalls) != 1 {
		t.Errorf("Expected 1 resolved event, got %d", len(notifier.resolvedCalls))
	}
	if notifier.resolvedCalls[0] != ".bashrc" {
		t.Errorf("Expected resolved event for .bashrc, got %s", notifier.resolvedCalls[0])
	}
	if notifier.allResolvedCalls != 1 {
		t.Errorf("Expected 1 allResolved event, got %d", notifier.allResolvedCalls)
	}
}

func TestActions_UseLocal_NotFound(t *testing.T) {
	repoDir := t.TempDir()
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)

	err = svc.UseLocal(".nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent conflict")
	}
}

func TestActions_UseLocal_FallbackToHomeDir(t *testing.T) {
	repoDir := t.TempDir()

	// Create conflict in timestamp-based format
	timestamp := time.Now().Format("20060102T150405Z0700")
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts", timestamp)
	if err := os.MkdirAll(conflictDir, testDirPerms); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	// Use a temp file name that won't conflict with real home files
	testFile := ".dsm-test-actions-file"
	localContent := []byte("local fallback content")
	if err := os.WriteFile(filepath.Join(conflictDir, testFile+".local"), localContent, testFilePerms); err != nil {
		t.Fatalf("Failed to write local file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, testFile+".remote"), []byte("remote content"), testFilePerms); err != nil {
		t.Fatalf("Failed to write remote file: %v", err)
	}

	// Setup config WITHOUT mapping - should fall back to home dir
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir
	// No mappings set

	svc := NewService(nil, cfg)

	// Use local version
	err = svc.UseLocal(testFile)
	if err != nil {
		t.Fatalf("UseLocal failed: %v", err)
	}

	// Verify file was created in home directory
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}
	targetPath := filepath.Join(home, testFile)
	t.Cleanup(func() { _ = os.Remove(targetPath) })

	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read target: %v", err)
	}
	if string(content) != "local fallback content" {
		t.Errorf("Expected local content, got %s", content)
	}
}

func TestActions_UseRemote(t *testing.T) {
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create conflict in timestamp-based format
	timestamp := time.Now().Format("20060102T150405Z0700")
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts", timestamp)
	if err := os.MkdirAll(conflictDir, testDirPerms); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	remoteContent := []byte("remote version content")
	if err := os.WriteFile(filepath.Join(conflictDir, ".bashrc.local"), []byte("local content"), testFilePerms); err != nil {
		t.Fatalf("Failed to write local file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, ".bashrc.remote"), remoteContent, testFilePerms); err != nil {
		t.Fatalf("Failed to write remote file: %v", err)
	}

	// Setup config with mapping
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir
	targetPath := filepath.Join(targetDir, ".bashrc")
	cfg.Mappings = map[string]string{".bashrc": targetPath}

	svc := NewService(nil, cfg)
	notifier := &mockNotifier{}
	svc.SetNotifier(notifier)

	// Use remote version
	err = svc.UseRemote(".bashrc")
	if err != nil {
		t.Fatalf("UseRemote failed: %v", err)
	}

	// Verify target file has remote content
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read target: %v", err)
	}
	if string(content) != "remote version content" {
		t.Errorf("Expected remote content, got %s", content)
	}

	// Verify conflict directory removed
	if _, err := os.Stat(conflictDir); !os.IsNotExist(err) {
		t.Error("Conflict directory should be removed")
	}

	// Verify events fired
	if len(notifier.resolvedCalls) != 1 {
		t.Errorf("Expected 1 resolved event, got %d", len(notifier.resolvedCalls))
	}
	if notifier.resolvedCalls[0] != ".bashrc" {
		t.Errorf("Expected resolved event for .bashrc, got %s", notifier.resolvedCalls[0])
	}
	if notifier.allResolvedCalls != 1 {
		t.Errorf("Expected 1 allResolved event, got %d", notifier.allResolvedCalls)
	}
}

func TestActions_UseRemote_NotFound(t *testing.T) {
	repoDir := t.TempDir()
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)

	err = svc.UseRemote(".nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent conflict")
	}
}

func TestActions_MarkResolved_Single(t *testing.T) {
	repoDir := t.TempDir()

	// Create conflict in timestamp-based format
	timestamp := time.Now().Format("20060102T150405Z0700")
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts", timestamp)
	if err := os.MkdirAll(conflictDir, testDirPerms); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(conflictDir, ".bashrc.local"), []byte("local"), testFilePerms); err != nil {
		t.Fatalf("Failed to write local file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, ".bashrc.remote"), []byte("remote"), testFilePerms); err != nil {
		t.Fatalf("Failed to write remote file: %v", err)
	}

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	notifier := &mockNotifier{}
	svc.SetNotifier(notifier)

	err = svc.MarkResolved([]string{".bashrc"})
	if err != nil {
		t.Fatalf("MarkResolved failed: %v", err)
	}

	// Verify conflict directory removed (was the only conflict)
	if _, err := os.Stat(conflictDir); !os.IsNotExist(err) {
		t.Error("Conflict directory should be removed")
	}

	// Verify events
	if len(notifier.resolvedCalls) != 1 {
		t.Errorf("Expected 1 resolved event, got %d", len(notifier.resolvedCalls))
	}
	if notifier.allResolvedCalls != 1 {
		t.Errorf("Expected 1 allResolved event, got %d", notifier.allResolvedCalls)
	}
}

func TestActions_MarkResolved_NonExistent(t *testing.T) {
	repoDir := t.TempDir()
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)

	// Should not error when marking non-existent file (already resolved)
	err = svc.MarkResolved([]string{".nonexistent"})
	if err != nil {
		t.Errorf("MarkResolved should not error for non-existent: %v", err)
	}
}

func TestActions_MarkAllResolved(t *testing.T) {
	repoDir := t.TempDir()

	// Create two conflicts in same timestamp directory
	timestamp := time.Now().Format("20060102T150405Z0700")
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts", timestamp)
	if err := os.MkdirAll(conflictDir, testDirPerms); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	for _, file := range []string{".bashrc", ".vimrc"} {
		if err := os.WriteFile(filepath.Join(conflictDir, file+".local"), []byte("local"), testFilePerms); err != nil {
			t.Fatalf("Failed to write local file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(conflictDir, file+".remote"), []byte("remote"), testFilePerms); err != nil {
			t.Fatalf("Failed to write remote file: %v", err)
		}
	}

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	notifier := &mockNotifier{}
	svc.SetNotifier(notifier)

	err = svc.MarkAllResolved()
	if err != nil {
		t.Fatalf("MarkAllResolved failed: %v", err)
	}

	// Verify no conflicts remain
	if svc.HasConflicts() {
		t.Error("Should have no conflicts after MarkAllResolved")
	}

	// Verify events - 2 resolved events and 1 allResolved
	if len(notifier.resolvedCalls) != 2 {
		t.Errorf("Expected 2 resolved events, got %d", len(notifier.resolvedCalls))
	}
	if notifier.allResolvedCalls != 1 {
		t.Errorf("Expected 1 allResolved event, got %d", notifier.allResolvedCalls)
	}
}

func TestActions_MarkAllResolved_Empty(t *testing.T) {
	repoDir := t.TempDir()
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	notifier := &mockNotifier{}
	svc.SetNotifier(notifier)

	err = svc.MarkAllResolved()
	if err != nil {
		t.Errorf("MarkAllResolved should succeed with no conflicts: %v", err)
	}

	// No events should fire when nothing to resolve
	if notifier.allResolvedCalls != 0 {
		t.Errorf("Expected 0 allResolved events, got %d", notifier.allResolvedCalls)
	}
}
