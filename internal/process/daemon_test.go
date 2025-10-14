package process

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

func TestProcessDetectionRaceCondition(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific process detection test")
	}

	// Test signal 0 check for non-existent PID
	nonExistentPID := 99999
	if processExists(nonExistentPID) {
		t.Fatalf("processExists returned true for non-existent PID %d", nonExistentPID)
	}

	// Test signal 0 check for current PID
	currentPID := os.Getpid()
	if !processExists(currentPID) {
		t.Fatalf("processExists returned false for current PID %d", currentPID)
	}

	// Test invalid PID handling
	if processExists(0) {
		t.Fatalf("processExists returned true for PID 0")
	}

	if processExists(-1) {
		t.Fatalf("processExists returned true for negative PID")
	}
}

func TestProcessExistsImplementation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Test the actual syscall.Kill(pid, 0) implementation
	currentPID := os.Getpid()

	// This should succeed (process exists)
	err := syscall.Kill(currentPID, 0)
	if err != nil {
		t.Fatalf("syscall.Kill(currentPID, 0) failed: %v", err)
	}

	// This should fail (process doesn't exist)
	nonExistentPID := 99999
	err = syscall.Kill(nonExistentPID, 0)
	if err == nil {
		t.Fatalf("syscall.Kill(nonExistentPID, 0) should have failed")
	}
}

func TestPIDFileManagement(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	originalPID := 12345
	if err := WritePID(originalPID); err != nil {
		t.Fatalf("failed to write PID: %v", err)
	}

	pidPath := filepath.Join(homeDir, ".dotfile-sync-manager.pid")

	// Verify PID file was created
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("PID file was not created: %v", err)
	}

	// Verify PID file permissions are secure (0600)
	info, err := os.Stat(pidPath)
	if err != nil {
		t.Fatalf("failed to stat PID file: %v", err)
	}
	if runtime.GOOS != "windows" {
		expectedPerm := os.FileMode(0o600)
		if info.Mode().Perm() != expectedPerm {
			t.Fatalf("expected PID file permissions %v, got %v", expectedPerm, info.Mode().Perm())
		}
	}

	// Test PID removal
	if err := RemovePID(); err != nil {
		t.Fatalf("failed to remove PID file: %v", err)
	}

	// Verify PID file was removed
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("PID file was not removed: %v", err)
	}
}
