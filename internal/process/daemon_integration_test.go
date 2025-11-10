package process_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/process"
)

// TestDaemonLifecycleIntegration tests the complete daemon lifecycle
// including startup, concurrent execution prevention, and cleanup.
func TestDaemonLifecycleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a temporary home directory
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Create a minimal git repository for testing
	dotfilesDir := filepath.Join(tempHome, "dotfiles")
	if err := os.MkdirAll(dotfilesDir, 0755); err != nil {
		t.Fatalf("failed to create dotfiles directory: %v", err)
	}

	// Create a minimal configuration file for testing
	configPath := filepath.Join(dotfilesDir, ".sync-config.json")
	minimalConfig := `{
		"version": "1.0",
		"machine": {
			"name": "test-machine"
		},
		"git": {
			"repo_path": "` + tempHome + `/dotfiles",
			"remote_url": "",
			"auth_type": "none"
		},
		"sync": {
			"auto_sync_enabled": false,
			"debounce_seconds": 1
		}
	}`
	if err := os.WriteFile(configPath, []byte(minimalConfig), 0644); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	// Initialize git repository
	if err := exec.Command("git", "init", dotfilesDir).Run(); err != nil {
		t.Fatalf("failed to initialize git repository: %v", err)
	}

	// Configure git user for the test repo
	if err := exec.Command("git", "-C", dotfilesDir, "config", "user.name", "Test User").Run(); err != nil {
		t.Fatalf("failed to configure git user: %v", err)
	}
	if err := exec.Command("git", "-C", dotfilesDir, "config", "user.email", "test@example.com").Run(); err != nil {
		t.Fatalf("failed to configure git email: %v", err)
	}

	// Build the test binary in the project directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	projectRoot := filepath.Join(cwd, "../..")
	binPath := filepath.Join(projectRoot, "test-dsm-"+t.Name())
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	// Use go build to create a test binary
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = projectRoot  // Go to project root from internal/process/
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build test binary: %v", err)
	}

	// Make the binary executable
	if err := os.Chmod(binPath, 0755); err != nil {
		t.Fatalf("failed to make test binary executable: %v", err)
	}

	// Clean up the binary after test
	t.Cleanup(func() {
		if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
			t.Logf("warning: failed to remove test binary: %v", err)
		}
	})

	// Test 1: Start daemon successfully
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logFilePath := filepath.Join(tempHome, "daemon.log")
	startCmd := exec.CommandContext(ctx, binPath, "start", "--log-file", logFilePath)
	output, err := startCmd.CombinedOutput()
	if err != nil {
		logContent, _ := os.ReadFile(logFilePath)
		t.Fatalf("failed to start daemon: %v\nOutput:\n%s\nLog:\n%s", err, string(output), string(logContent))
	}

	// Poll for daemon to start (more reliable than fixed sleep)
	deadline := time.Now().Add(10 * time.Second)
	var daemonRunning bool
	for time.Now().Before(deadline) {
		if process.IsDaemonRunning() {
			daemonRunning = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !daemonRunning {
		t.Fatal("daemon should be running after start command")
	}

	// Test 2: Attempt to start second daemon (should fail)
	startCmd2 := exec.CommandContext(ctx, binPath, "start")
	if err := startCmd2.Run(); err == nil {
		t.Fatal("second daemon start should have failed")
	}

	// Test 3: Stop daemon
	stopCmd := exec.CommandContext(ctx, binPath, "stop")
	if err := stopCmd.Run(); err != nil {
		t.Fatalf("failed to stop daemon: %v", err)
	}

	// Poll for daemon to stop
	deadline = time.Now().Add(10 * time.Second)
	var daemonStillRunning bool
	for time.Now().Before(deadline) {
		if !process.IsDaemonRunning() {
			break
		}
		daemonStillRunning = true
		time.Sleep(100 * time.Millisecond)
	}

	if daemonStillRunning && process.IsDaemonRunning() {
		t.Fatal("daemon should not be running after stop command")
	}

	// Test 4: Verify PID file cleanup
	pidPath := filepath.Join(tempHome, ".dotfile-sync-manager.pid")
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("PID file should be cleaned up after daemon stops")
	}

	// Test 5: Verify lock file cleanup
	lockPath := pidPath + ".lock"
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("lock file should be cleaned up after daemon stops")
	}
}

// TestDaemonCrashRecovery tests daemon recovery after crashes
func TestDaemonCrashRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Create a fake PID file for a non-existent process
	pidPath := filepath.Join(tempHome, ".dotfile-sync-manager.pid")
	pidContent := "99999:dot-sync-manager" // Non-existent PID
	if err := os.WriteFile(pidPath, []byte(pidContent), 0600); err != nil {
		t.Fatalf("failed to write fake PID file: %v", err)
	}

	// Verify IsDaemonRunning properly cleans up stale PID
	if process.IsDaemonRunning() {
		t.Fatal("daemon should not be running with stale PID")
	}

	// Verify stale PID file was cleaned up
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("stale PID file should be cleaned up")
	}
}

// TestConcurrentDaemonStart tests that concurrent daemon startups are properly serialized
func TestConcurrentDaemonStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Build the test binary in the project directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	projectRoot := filepath.Join(cwd, "../..")
	binPath := filepath.Join(projectRoot, "test-dsm-"+t.Name())
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = projectRoot  // Go to project root from internal/process/
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build test binary: %v", err)
	}

	// Make the binary executable
	if err := os.Chmod(binPath, 0755); err != nil {
		t.Fatalf("failed to make test binary executable: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
			t.Logf("warning: failed to remove test binary: %v", err)
		}
	})

	const numGoroutines = 5
	results := make(chan error, numGoroutines)

	// Try to start multiple daemons concurrently
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			logFilePath := filepath.Join(tempHome, fmt.Sprintf("daemon-%d.log", i))
			startCmd := exec.CommandContext(ctx, binPath, "start", "--log-file", logFilePath)
			output, err := startCmd.CombinedOutput()
			if err != nil {
				logContent, _ := os.ReadFile(logFilePath)
				results <- fmt.Errorf("error: %w, output: %s, log: %s", err, string(output), string(logContent))
			} else {
				results <- nil
			}
		}(i)
	}

	// Wait for all results
	successCount := 0
	errorCount := 0
	for i := 0; i < numGoroutines; i++ {
		if err := <-results; err != nil {
			errorCount++
			t.Logf("Concurrent start failed as expected: %v", err)
		} else {
			successCount++
		}
	}

	// Exactly one should succeed
	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful start, got %d successful and %d failed", successCount, errorCount)
	}

	// Clean up: stop the daemon that started
	stopCmd := exec.CommandContext(ctx, binPath, "stop")
	if err := stopCmd.Run(); err != nil {
		t.Logf("warning: failed to stop daemon during cleanup: %v", err)
	}
}

// TestPIDFileFormatCompatibility tests that both old and new PID file formats work
func TestPIDFileFormatCompatibility(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	pidPath := filepath.Join(tempHome, ".dotfile-sync-manager.pid")

	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "legacy format",
			content:  "12345",
			expected: true,
		},
		{
			name:     "new format",
			content:  "12345:dot-sync-manager",
			expected: true,
		},
		{
			name:     "invalid format",
			content:  "invalid",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test PID file
			if err := os.WriteFile(pidPath, []byte(tt.content), 0600); err != nil {
				t.Fatalf("failed to write test PID file: %v", err)
			}

			// Check if daemon detection works as expected
			running := process.IsDaemonRunning()

			if tt.expected && running {
				// For valid formats with non-existent PIDs, should return false and clean up
				if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
					t.Errorf("PID file should have been cleaned up for valid format with non-existent PID")
				}
			} else if !tt.expected && running {
				t.Errorf("daemon should not be detected as running for invalid format")
			}
		})
	}
}

// TestCrossPlatformCompatibility tests that PID file management works across platforms
func TestCrossPlatformCompatibility(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Test basic PID file operations
	const testPID = 12345

	// Write PID exclusively and get lock manager
	lockManager, err := process.WritePIDExclusive(testPID)
	if err != nil {
		t.Fatalf("failed to write PID exclusively: %v", err)
	}

	// Verify PID file exists
	pidPath := filepath.Join(tempHome, ".dotfile-sync-manager.pid")
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("PID file should exist: %v", err)
	}

	// Note: lock file should be held while lockManager exists
	lockPath := pidPath + ".lock"
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("Lock file should exist while lock manager is held: %v", err)
	}

	// Release the lock to simulate daemon shutdown
	if err := lockManager.Unlock(); err != nil {
		t.Fatalf("failed to unlock PID file: %v", err)
	}

	// Verify lock file is cleaned up after unlock
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("Lock file should be removed after unlock")
	}

	// Try to write PID exclusively again with a stale PID (should succeed after cleanup)
	lockManager2, err := process.WritePIDExclusive(testPID + 1)
	if err != nil {
		t.Fatalf("expected success when writing after stale PID cleanup, got error: %v", err)
	}
	// Verify cleanup
	if err := lockManager2.Unlock(); err != nil {
		t.Fatalf("failed to cleanup second lock manager: %v", err)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("PID file should be removed after unlock")
	}
}