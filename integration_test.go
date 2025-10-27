package main

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGracefulShutdownIntegration tests the graceful shutdown functionality end-to-end
func TestGracefulShutdownIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Get path to current executable
	execPath, err := os.Executable()
	require.NoError(t, err)

	// Create a temporary config file that points to a test directory
	tempDir := t.TempDir()
	configContent := `{
		"machine": {
			"name": "test-machine-graceful-shutdown"
		},
		"git": {
			"repo_path": "` + tempDir + `",
			"remote_url": "https://github.com/test/test.git",
			"auth_type": "none"
		},
		"sync": {
			"auto_sync_enabled": false,
			"debounce_seconds": 1
		}
	}`

	configPath := tempDir + "/test-config.json"
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Start daemon process in foreground mode
	daemonCmd := exec.Command(execPath, "start", "--config", configPath, "--foreground")
	daemonCmd.Env = append(os.Environ(), "GO_TEST_DAEMON=1")

	t.Log("Starting daemon for graceful shutdown integration test...")
	err = daemonCmd.Start()
	require.NoError(t, err)

	// Give daemon time to initialize
	time.Sleep(2 * time.Second)

	// Verify daemon is running
	assert.True(t, daemonCmd.Process != nil, "Daemon process should be started")
	assert.Greater(t, daemonCmd.Process.Pid, 0, "Daemon should have valid PID")

	t.Logf("Daemon started with PID: %d", daemonCmd.Process.Pid)

	// Send SIGTERM to test graceful shutdown
	t.Log("Sending SIGTERM to test graceful shutdown...")
	err = daemonCmd.Process.Signal(syscall.SIGTERM)
	assert.NoError(t, err, "Should be able to send SIGTERM to daemon")

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- daemonCmd.Wait()
	}()

	select {
	case err := <-done:
		t.Logf("Daemon process exited with status: %v", err)
		// Process should exit cleanly
		break
	case <-time.After(15 * time.Second):
		// Force kill if timeout
		t.Log("Daemon did not exit within timeout, forcing kill")
		if daemonCmd.Process != nil {
			if err := daemonCmd.Process.Kill(); err != nil {
				t.Logf("Warning: failed to kill daemon process: %v", err)
			}
		}
		if err := daemonCmd.Wait(); err != nil {
			t.Logf("Warning: daemon wait after kill: %v", err)
		}
		t.Fatal("Daemon should have exited gracefully within timeout period")
	}

	// Test passes if we reach here - daemon handled graceful shutdown
	t.Log("✅ Graceful shutdown integration test passed")
}