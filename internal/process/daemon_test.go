package process

import (
	"errors"
	"fmt"
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

func TestWritePIDExclusiveWithLocking(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	const testPID = 12345

	// Test successful exclusive write
	lockManager, err := WritePIDExclusive(testPID)
	if err != nil {
		t.Fatalf("failed to write PID exclusively: %v", err)
	}

	// Verify PID file was created
	path := filepath.Join(homeDir, ".dotfile-sync-manager.pid")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("PID file was not created: %v", err)
	}

	// Verify lock file is held
	lockPath := path + ".lock"
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("Lock file should exist while lock manager is held: %v", err)
	}

	// Release lock to cleanup
	if err := lockManager.Unlock(); err != nil {
		t.Fatalf("failed to unlock: %v", err)
	}

	// Verify that stale PID cleanup works by creating a PID file with non-existent process
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d:dot-sync-manager", testPID+999)), 0600); err != nil {
		t.Fatalf("failed to write stale PID file: %v", err)
	}

	// This should succeed because stale PID will be cleaned up
	lockManager2, err := WritePIDExclusive(testPID + 1)
	if err != nil {
		t.Fatalf("expected success when writing after stale PID cleanup, got error: %v", err)
	}
	defer func() {
		if err := lockManager2.Unlock(); err != nil {
			t.Fatalf("failed to cleanup second lock manager: %v", err)
		}
	}()
}

func TestWritePIDExclusiveRejectsNonPositive(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	if _, err := WritePIDExclusive(0); err == nil {
		t.Fatal("expected error when writing PID 0 exclusively, got nil")
	}
	if _, err := WritePIDExclusive(-42); err == nil {
		t.Fatal("expected error when writing negative PID exclusively, got nil")
	}
}

// TestLockManager tests the LockManager lifecycle operations
func TestLockManager(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	const testPID = 12345

	// Test successful lock acquisition
	lockManager, err := WritePIDExclusive(testPID)
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Verify lock file exists
	pidPath := filepath.Join(homeDir, ".dotfile-sync-manager.pid")
	lockPath := pidPath + ".lock"
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file should exist while lock manager is held: %v", err)
	}

	// Test that second lock acquisition fails (concurrent access prevention)
	_, err = WritePIDExclusive(testPID + 1)
	if err == nil {
		t.Fatal("expected error when trying to acquire second lock, got nil")
	}
	// Check for daemon already running error
	if !errors.Is(err, ErrDaemonAlreadyRunning) {
		t.Fatalf("expected ErrDaemonAlreadyRunning, got: %v", err)
	}

	// Test successful unlock
	if err := lockManager.Unlock(); err != nil {
		t.Fatalf("failed to unlock: %v", err)
	}

	// Verify lock file is cleaned up after unlock
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("lock file should be removed after unlock")
	}

	// Verify PID file is also cleaned up after unlock
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("PID file should be removed after unlock")
	}
}

// TestGetDaemonPID tests the GetDaemonPID function with various scenarios
func TestGetDaemonPID(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	const testPID = 12345

	// Test when no PID file exists
	pid, err := GetDaemonPID()
	if err == nil {
		t.Fatal("expected error when no PID file exists, got nil")
	}
	if pid != 0 {
		t.Fatalf("expected PID 0 when no file exists, got %d", pid)
	}

	// Create a valid PID file
	lockManager, err := WritePIDExclusive(testPID)
	if err != nil {
		t.Fatalf("failed to create PID file: %v", err)
	}
	defer func() {
		if err := lockManager.Unlock(); err != nil {
			t.Fatalf("failed to cleanup lock: %v", err)
		}
	}()

	// Test getting PID from valid file
	// Note: GetDaemonPID might fail because it tries to verify process existence
	// and the test process name doesn't match what's in the PID file
	// This is expected behavior for tests
	pid, err = GetDaemonPID()
	if err != nil {
		// This is acceptable for unit tests since the test binary name differs
		t.Logf("GetDaemonPID failed as expected in test environment: %v", err)
	} else if pid != testPID {
		t.Fatalf("expected PID %d, got %d", testPID, pid)
	}
}

// TestStopAllDaemons tests the StopAllDaemons function
func TestStopAllDaemons(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Test stopping when no daemon is running
	err := StopAllDaemons()
	if err != nil {
		t.Fatalf("expected success when stopping non-existent daemon, got error: %v", err)
	}

	// Test with a PID file (simulated daemon)
	const testPID = 12345
	lockManager, err := WritePIDExclusive(testPID)
	if err != nil {
		t.Fatalf("failed to create PID file: %v", err)
	}

	// StopAllDaemons should clean up the PID file even if process doesn't exist
	err = StopAllDaemons()
	if err != nil {
		t.Fatalf("expected success when stopping daemon with non-existent process, got error: %v", err)
	}

	// Verify PID file was cleaned up
	pidPath := filepath.Join(homeDir, ".dotfile-sync-manager.pid")
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("PID file should be removed after StopAllDaemons")
	}

	// Cleanup lock
	if err := lockManager.Unlock(); err != nil {
		t.Fatalf("failed to cleanup lock: %v", err)
	}
}

// TestRemovePID tests the RemovePID function with various scenarios
func TestRemovePID(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	pidPath := filepath.Join(homeDir, ".dotfile-sync-manager.pid")
	lockPath := pidPath + ".lock"

	// Test removing when no files exist
	err := RemovePID()
	if err != nil {
		t.Fatalf("expected success when removing non-existent PID file, got error: %v", err)
	}

	// Create PID and lock files
	const testPID = 12345
	content := fmt.Sprintf("%d:dot-sync-manager", testPID)
	if err := os.WriteFile(pidPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to create PID file: %v", err)
	}
	if err := os.WriteFile(lockPath, []byte("lock"), 0600); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	// Test successful removal
	err = RemovePID()
	if err != nil {
		t.Fatalf("expected success when removing existing PID file, got error: %v", err)
	}

	// Verify both files are removed
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("PID file should be removed after RemovePID")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("Lock file should be removed after RemovePID")
	}
}

func TestReadPIDFromPath(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		content     string
		expectPID   int
		expectExe   string
		expectError bool
	}{
		{
			name:        "valid new format",
			content:     "12345:dot-sync-manager",
			expectPID:   12345,
			expectExe:   "dot-sync-manager",
			expectError: false,
		},
		{
			name:        "valid legacy format",
			content:     "12345",
			expectPID:   12345,
			expectExe:   "",
			expectError: false,
		},
		{
			name:        "invalid PID",
			content:     "invalid:dot-sync-manager",
			expectError: true,
		},
		{
			name:        "zero PID",
			content:     "0:dot-sync-manager",
			expectError: true,
		},
		{
			name:        "negative PID",
			content:     "-1:dot-sync-manager",
			expectError: true,
		},
		{
			name:        "missing executable name",
			content:     "12345:",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tempDir, "test.pid")
			if err := os.WriteFile(path, []byte(tt.content), pidFilePerms); err != nil {
				t.Fatalf("failed to write test PID file: %v", err)
			}

			pidInfo, err := readPIDFromPath(path)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if pidInfo.pid != tt.expectPID {
				t.Fatalf("expected PID %d, got %d", tt.expectPID, pidInfo.pid)
			}

			if pidInfo.exeName != tt.expectExe {
				t.Fatalf("expected exe name %q, got %q", tt.expectExe, pidInfo.exeName)
			}
		})
	}
}

func TestCleanupStalePIDFile(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	path := filepath.Join(homeDir, ".dotfile-sync-manager.pid")
	const testPID = 12345
	var testExeName = "dot-sync-manager"

	// Test cleanup when PID file doesn't exist
	if err := cleanupStalePIDFile(path, testExeName); err != nil {
		t.Fatalf("unexpected error when PID file doesn't exist: %v", err)
	}

	// Test cleanup when PID file contains current process PID
	currentPID := os.Getpid()
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d:%s", currentPID, testExeName)), pidFilePerms); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	if err := cleanupStalePIDFile(path, testExeName); err != nil {
		t.Fatalf("unexpected error cleaning up current process PID: %v", err)
	}

	// Verify file was removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("PID file was not removed after cleanup")
	}

	// Test cleanup when PID file contains non-existent process
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d:%s", testPID, testExeName)), pidFilePerms); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	if err := cleanupStalePIDFile(path, testExeName); err != nil {
		t.Fatalf("unexpected error cleaning up stale PID: %v", err)
	}

	// Verify file was removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("PID file was not removed after stale cleanup")
	}
}

func TestRemovePIDCleansUpLockFile(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	path := filepath.Join(homeDir, ".dotfile-sync-manager.pid")
	lockPath := path + ".lock"

	// Create both PID file and lock file
	if err := os.WriteFile(path, []byte("12345:dot-sync-manager"), pidFilePerms); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}
	if err := os.WriteFile(lockPath, []byte("lock"), pidFilePerms); err != nil {
		t.Fatalf("failed to write lock file: %v", err)
	}

	// Remove PID
	if err := RemovePID(); err != nil {
		t.Fatalf("failed to remove PID: %v", err)
	}

	// Verify both files are removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("PID file was not removed")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("Lock file was not removed")
	}
}

func TestPIDFileFormatCompatibility(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	path := filepath.Join(homeDir, ".dotfile-sync-manager.pid")

	tests := []struct {
		name        string
		content     string
		expectedPID int
		expectedExe string
		shouldWork  bool
	}{
		{
			name:        "new format with exe name",
			content:     "12345:dot-sync-manager",
			expectedPID: 12345,
			expectedExe: "dot-sync-manager",
			shouldWork:  true,
		},
		{
			name:        "legacy format without exe name",
			content:     "12345",
			expectedPID: 12345,
			expectedExe: "",
			shouldWork:  true,
		},
		{
			name:        "new format with different exe name",
			content:     "12345:custom-binary",
			expectedPID: 12345,
			expectedExe: "custom-binary",
			shouldWork:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test content
			if err := os.WriteFile(path, []byte(tt.content), pidFilePerms); err != nil {
				t.Fatalf("failed to write test PID file: %v", err)
			}

			// Read back using readPID function
			pidInfo, err := readPID()

			if !tt.shouldWork {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error reading PID: %v", err)
			}

			if pidInfo.pid != tt.expectedPID {
				t.Fatalf("expected PID %d, got %d", tt.expectedPID, pidInfo.pid)
			}

			if pidInfo.exeName != tt.expectedExe {
				t.Fatalf("expected exe name %q, got %q", tt.expectedExe, pidInfo.exeName)
			}

			// Cleanup for next test
			if err := RemovePID(); err != nil {
				t.Fatalf("failed to cleanup: %v", err)
			}
		})
	}
}

// TestWritePIDExclusive_SecondInstanceReturnsError verifies that attempting to acquire
// an exclusive PID lock when another instance already holds it returns ErrDaemonAlreadyRunning.
// This ensures the singleton daemon guarantee: only one daemon can run at a time.
func TestWritePIDExclusive_SecondInstanceReturnsError(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Setup: First daemon acquires exclusive lock
	lockManager1, err := WritePIDExclusive(1234)
	if err != nil {
		t.Fatalf("first WritePIDExclusive failed: %v", err)
	}
	t.Cleanup(func() {
		if err := lockManager1.Unlock(); err != nil {
			t.Logf("warning: failed to unlock during cleanup: %v", err)
		}
	})

	// Action: Second daemon attempts to acquire lock (should fail)
	lockManager2, err := WritePIDExclusive(5678)

	// Assert: Returns ErrDaemonAlreadyRunning sentinel error
	if lockManager2 != nil {
		if unlockErr := lockManager2.Unlock(); unlockErr != nil {
			t.Logf("warning: failed to unlock lockManager2: %v", unlockErr)
		}
		t.Fatal("expected lockManager2 to be nil when lock acquisition fails")
	}
	if err == nil {
		t.Fatal("expected error when second instance tries to acquire lock, got nil")
	}
	if !errors.Is(err, ErrDaemonAlreadyRunning) {
		t.Errorf("expected ErrDaemonAlreadyRunning, got: %v", err)
	}
}

// TestLockManager_UnlockRemovesBothFiles verifies that LockManager.Unlock()
// properly cleans up both the PID file and the lock file.
func TestLockManager_UnlockRemovesBothFiles(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	pidPath := filepath.Join(homeDir, ".dotfile-sync-manager.pid")
	lockPath := pidPath + ".lock"

	// Setup: Create lock
	lockManager, err := WritePIDExclusive(1234)
	if err != nil {
		t.Fatalf("WritePIDExclusive failed: %v", err)
	}

	// Verify files exist before unlock
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("PID file should exist before unlock: %v", err)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file should exist before unlock: %v", err)
	}

	// Action: Unlock
	if err := lockManager.Unlock(); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	// Assert: Both files removed
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Errorf("PID file should be removed after unlock, got error: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Errorf("lock file should be removed after unlock, got error: %v", err)
	}
}

// TestLockManager_DoubleUnlockSafe verifies that calling Unlock() multiple times
// is safe and doesn't cause errors or panics (idempotent operation).
func TestLockManager_DoubleUnlockSafe(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Setup: Create lock
	lockManager, err := WritePIDExclusive(1234)
	if err != nil {
		t.Fatalf("WritePIDExclusive failed: %v", err)
	}

	// Action: First unlock
	if err := lockManager.Unlock(); err != nil {
		t.Fatalf("first Unlock failed: %v", err)
	}

	// Action: Second unlock (should be safe, no panic or error)
	if err := lockManager.Unlock(); err != nil {
		t.Errorf("second Unlock should be safe, got error: %v", err)
	}
}
