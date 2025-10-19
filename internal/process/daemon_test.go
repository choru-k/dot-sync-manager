package process

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

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

	if err := os.WriteFile(path, []byte("0\n"), pidFilePerms); err != nil {
		t.Fatalf("failed to write pid file: %v", err)
	}

	if _, err := readPID(); err == nil {
		t.Fatal("expected error when reading non-positive PID, got nil")
	}
}


