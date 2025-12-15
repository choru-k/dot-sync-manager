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
	eventTestDirPerms  = 0755
	eventTestFilePerms = 0644
)

// mockNotifier implements ConflictNotifier for testing
type mockNotifier struct {
	detectedCalls    [][]ConflictInfo
	resolvedCalls    []string
	allResolvedCalls int
}

func (m *mockNotifier) OnConflictDetected(conflicts []ConflictInfo) {
	m.detectedCalls = append(m.detectedCalls, conflicts)
}

func (m *mockNotifier) OnConflictResolved(file string) {
	m.resolvedCalls = append(m.resolvedCalls, file)
}

func (m *mockNotifier) OnAllConflictsResolved() {
	m.allResolvedCalls++
}

func TestService_SetNotifier(t *testing.T) {
	repoDir := t.TempDir()
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	notifier := &mockNotifier{}

	svc.SetNotifier(notifier)

	// Verify notifier is set by calling notify method
	svc.notifyAllConflictsResolved()

	if notifier.allResolvedCalls != 1 {
		t.Errorf("Expected 1 allResolvedCall, got %d", notifier.allResolvedCalls)
	}
}

func TestService_SetNotifier_Nil(t *testing.T) {
	repoDir := t.TempDir()
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	svc.SetNotifier(nil)

	// Should not panic when notifier is nil
	svc.notifyAllConflictsResolved()
	svc.notifyConflictResolved(".bashrc")
	svc.notifyConflictDetected([]ConflictInfo{})
}

func TestService_NotifyConflictDetected(t *testing.T) {
	repoDir := t.TempDir()
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	notifier := &mockNotifier{}
	svc.SetNotifier(notifier)

	conflicts := []ConflictInfo{
		{File: ".bashrc"},
		{File: ".vimrc"},
	}
	svc.notifyConflictDetected(conflicts)

	if len(notifier.detectedCalls) != 1 {
		t.Fatalf("Expected 1 detected call, got %d", len(notifier.detectedCalls))
	}

	if len(notifier.detectedCalls[0]) != 2 {
		t.Errorf("Expected 2 conflicts, got %d", len(notifier.detectedCalls[0]))
	}
}

func TestService_NotifyConflictResolved(t *testing.T) {
	repoDir := t.TempDir()
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	notifier := &mockNotifier{}
	svc.SetNotifier(notifier)

	svc.notifyConflictResolved(".bashrc")

	if len(notifier.resolvedCalls) != 1 {
		t.Fatalf("Expected 1 resolved call, got %d", len(notifier.resolvedCalls))
	}

	if notifier.resolvedCalls[0] != ".bashrc" {
		t.Errorf("Expected .bashrc, got %s", notifier.resolvedCalls[0])
	}
}

func TestService_CheckForConflicts_FiresEvent(t *testing.T) {
	repoDir := t.TempDir()

	// Create gitmanager-style conflict directory with timestamp
	timestamp := time.Now().Format("20060102T150405Z0700")
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts", timestamp)
	if err := os.MkdirAll(conflictDir, eventTestDirPerms); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	// Create conflict files with suffix naming
	if err := os.WriteFile(filepath.Join(conflictDir, ".bashrc.local"), []byte("local content"), eventTestFilePerms); err != nil {
		t.Fatalf("Failed to write local file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, ".bashrc.remote"), []byte("remote content"), eventTestFilePerms); err != nil {
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

	conflicts, err := svc.CheckForConflicts()
	if err != nil {
		t.Fatalf("CheckForConflicts failed: %v", err)
	}

	if len(conflicts) != 1 {
		t.Fatalf("Expected 1 conflict, got %d", len(conflicts))
	}

	if len(notifier.detectedCalls) != 1 {
		t.Errorf("Expected 1 detected event, got %d", len(notifier.detectedCalls))
	}
}

func TestService_CheckForConflicts_NoEventWhenEmpty(t *testing.T) {
	repoDir := t.TempDir()

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	notifier := &mockNotifier{}
	svc.SetNotifier(notifier)

	conflicts, err := svc.CheckForConflicts()
	if err != nil {
		t.Fatalf("CheckForConflicts failed: %v", err)
	}

	if len(conflicts) != 0 {
		t.Errorf("Expected 0 conflicts, got %d", len(conflicts))
	}

	if len(notifier.detectedCalls) != 0 {
		t.Errorf("Expected 0 detected events, got %d", len(notifier.detectedCalls))
	}
}
