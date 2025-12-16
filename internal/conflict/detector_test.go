package conflict

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

// TestDetector_StartStop tests basic detector lifecycle.
// TDD: RED - This test calls methods that don't exist yet.
func TestDetector_StartStop(t *testing.T) {
	repoDir := t.TempDir()
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts")
	if err := os.MkdirAll(conflictDir, testDirPerms); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	detector := NewDetector(svc)

	// Start
	err = detector.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !detector.IsRunning() {
		t.Error("Detector should be running after Start")
	}

	// Stop
	detector.Stop()

	if detector.IsRunning() {
		t.Error("Detector should not be running after Stop")
	}
}

// TestDetector_DoubleStart tests that Start() is idempotent.
func TestDetector_DoubleStart(t *testing.T) {
	repoDir := t.TempDir()
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts")
	if err := os.MkdirAll(conflictDir, testDirPerms); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	svc := NewService(nil, cfg)
	detector := NewDetector(svc)

	// First start
	err = detector.Start()
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	if !detector.IsRunning() {
		t.Error("Detector should be running after first Start")
	}

	// Second start should be idempotent (no error, still running)
	err = detector.Start()
	if err != nil {
		t.Errorf("Second Start should not return error, got: %v", err)
	}

	if !detector.IsRunning() {
		t.Error("Detector should still be running after second Start")
	}

	// Cleanup
	detector.Stop()
}

// TestDetector_DetectsNewConflict tests that detector triggers on new conflict files.
// TDD: RED - This test requires fsnotify watcher implementation.
func TestDetector_DetectsNewConflict(t *testing.T) {
	repoDir := t.TempDir()
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts")
	if err := os.MkdirAll(conflictDir, testDirPerms); err != nil {
		t.Fatalf("Failed to create conflict directory: %v", err)
	}

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir

	// Track whether CheckForConflicts was called
	checkCalled := make(chan struct{}, 1)
	svc := NewService(nil, cfg)

	detector := NewDetector(svc)
	detector.SetOnCheck(func() {
		select {
		case checkCalled <- struct{}{}:
		default:
		}
	})

	err = detector.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer detector.Stop()

	// Create a new conflict file (timestamp directory with .remote file)
	timestampDir := filepath.Join(conflictDir, "20240101T120000Z+0000")
	if err := os.MkdirAll(timestampDir, testDirPerms); err != nil {
		t.Fatalf("Failed to create timestamp directory: %v", err)
	}
	conflictFile := filepath.Join(timestampDir, "testfile.remote")
	if err := os.WriteFile(conflictFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create conflict file: %v", err)
	}

	// Wait for detection (with timeout)
	// Use 5 seconds to avoid flakiness in CI environments
	select {
	case <-checkCalled:
		// Success - conflict was detected
	case <-time.After(5 * time.Second):
		t.Error("Detector did not trigger CheckForConflicts within timeout")
	}
}
