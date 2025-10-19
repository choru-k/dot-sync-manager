package process

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
)

func TestProcessExistsEdgeCases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific process detection test")
	}

	// Test signal 0 check for non-existent PID
	nonExistentPID := findNonExistentPID()
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

func findNonExistentPID() int {
	// Start from a high PID and work down to find one that doesn't exist
	for pid := 99999; pid > 50000; pid-- {
		if err := syscall.Kill(pid, 0); err != nil {
			// Error means process doesn't exist
			return pid
		}
	}
	// Fallback (unlikely to reach here)
	return 99999
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
	nonExistentPID := findNonExistentPID()
	err = syscall.Kill(nonExistentPID, 0)
	if err == nil {
		t.Fatalf("syscall.Kill(nonExistentPID, 0) should have failed")
	}
}

const testPID = 12345 // Arbitrary non-existent PID for testing file operations

func TestPIDFileManagement(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	originalPID := testPID
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
		expectedPerm := os.FileMode(pidFilePerms)
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

func TestConcurrentPIDFileOperations(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	const numGoroutines = 10
	const numOperations = 50

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines*numOperations)

	// Test concurrent WritePID operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				pid := goroutineID*1000 + j + 1
				if err := WritePID(pid); err != nil {
					errChan <- err
					return
				}
			}
		}(i)
	}

	// Test concurrent RemovePID operations
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				if err := RemovePID(); err != nil && !os.IsNotExist(err) {
					errChan <- err
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		t.Errorf("concurrent operation failed: %v", err)
	}

	// Final cleanup
	if err := RemovePID(); err != nil && !os.IsNotExist(err) {
		t.Errorf("final cleanup failed: %v", err)
	}
}

func TestWritePIDRejectsNonPositive(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	if err := WritePID(0); err == nil {
		t.Fatal("expected error when writing PID 0, got nil")
	}
	if err := WritePID(-42); err == nil {
		t.Fatal("expected error when writing negative PID, got nil")
	}
}

func TestReadPIDRejectsNonPositive(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	path, err := pidFilePath()
	if err != nil {
		t.Fatalf("failed to determine pid file path: %v", err)
	}

	if err := os.WriteFile(path, []byte("0\n"), 0o600); err != nil {
		t.Fatalf("failed to write pid file: %v", err)
	}

	if _, err := readPID(); err == nil {
		t.Fatal("expected error when reading non-positive PID, got nil")
	}
}
