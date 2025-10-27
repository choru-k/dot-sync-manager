package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGracefulShutdown_TimeoutScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout tests in short mode")
	}

	// Create temporary config for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.json")
	repoPath := tempDir

	// Create test configuration with longer timeouts to trigger timeout scenarios
	configContent := `{
		"machine": {
			"name": "test-timeout-scenarios"
		},
		"git": {
			"repo_path": "` + repoPath + `",
			"remote_url": "https://github.com/test/test.git",
			"auth_type": "none"
		},
		"sync": {
			"auto_sync_enabled": false,
			"debounce_seconds": 1
		}
	}`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Get path to current executable
	execPath, err := os.Executable()
	require.NoError(t, err)

	t.Run("Normal Graceful Shutdown", func(t *testing.T) {
		// Test normal graceful shutdown without timeout
		daemonArgs := []string{"test", "--config", configPath, "start", "--foreground"}
		daemonCmd := exec.Command(execPath, daemonArgs...)
		daemonCmd.Env = append(os.Environ(), "GO_TEST_DAEMON=1")

		t.Log("Starting daemon for normal shutdown test...")
		err = daemonCmd.Start()
		require.NoError(t, err)

		// Give daemon time to initialize
		time.Sleep(2 * time.Second)

		// Send SIGTERM for graceful shutdown
		t.Log("Sending SIGTERM for normal shutdown...")
		err = daemonCmd.Process.Signal(syscall.SIGTERM)
		require.NoError(t, err)

		// Wait for process to exit with reasonable timeout
		done := make(chan error, 1)
		go func() {
			done <- daemonCmd.Wait()
		}()

		select {
		case err := <-done:
			t.Logf("Daemon exited normally: %v", err)
			// Should exit cleanly
		case <-time.After(20 * time.Second):
			t.Fatal("Daemon did not exit within timeout for normal shutdown")
		}
	})

	t.Run("Signal Storm Timeout", func(t *testing.T) {
		// Test multiple rapid signals
		daemonArgs := []string{"test", "--config", configPath, "start", "--foreground"}
		daemonCmd := exec.Command(execPath, daemonArgs...)
		daemonCmd.Env = append(os.Environ(), "GO_TEST_DAEMON=1")

		t.Log("Starting daemon for signal storm test...")
		err = daemonCmd.Start()
		require.NoError(t, err)

		// Give daemon time to initialize
		time.Sleep(2 * time.Second)

		// Send multiple signals rapidly
		t.Log("Sending signal storm...")
		for i := 0; i < 5; i++ {
			err = daemonCmd.Process.Signal(syscall.SIGTERM)
			if err != nil {
				t.Logf("Signal %d failed: %v", i+1, err)
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		// Wait for process to exit
		done := make(chan error, 1)
		go func() {
			done <- daemonCmd.Wait()
		}()

		select {
		case err := <-done:
			t.Logf("Daemon handled signal storm, exited: %v", err)
		case <-time.After(20 * time.Second):
			t.Log("Daemon did not exit, forcing kill")
			if daemonCmd.Process != nil {
				if err := daemonCmd.Process.Kill(); err != nil {
					t.Logf("Warning: failed to kill daemon process: %v", err)
				}
			}
			if err := daemonCmd.Wait(); err != nil {
				t.Logf("Warning: daemon wait after kill: %v", err)
			}
			t.Fatal("Daemon should have handled signal storm")
		}
	})
}

func TestGracefulShutdown_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping error handling tests in short mode")
	}

	// Create temporary config with invalid repo path to trigger errors
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-error-config.json")
	invalidRepoPath := "/nonexistent/path/that/does/not/exist"

	configContent := `{
		"machine": {
			"name": "test-error-handling"
		},
		"git": {
			"repo_path": "` + invalidRepoPath + `",
			"remote_url": "https://github.com/test/test.git",
			"auth_type": "none"
		},
		"sync": {
			"auto_sync_enabled": false,
			"debounce_seconds": 1
		}
	}`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Get path to current executable
	execPath, err := os.Executable()
	require.NoError(t, err)

	t.Run("Graceful Shutdown with Errors", func(t *testing.T) {
		daemonArgs := []string{"test", "--config", configPath, "start", "--foreground"}
		daemonCmd := exec.Command(execPath, daemonArgs...)
		daemonCmd.Env = append(os.Environ(), "GO_TEST_DAEMON=1")

		t.Log("Starting daemon with invalid config to test error handling...")
		err = daemonCmd.Start()
		require.NoError(t, err)

		// Give daemon time to attempt initialization
		time.Sleep(2 * time.Second)

		// Send SIGTERM for graceful shutdown
		t.Log("Sending SIGTERM to daemon with errors...")
		err = daemonCmd.Process.Signal(syscall.SIGTERM)
		require.NoError(t, err)

		// Wait for process to exit
		done := make(chan error, 1)
		go func() {
			done <- daemonCmd.Wait()
		}()

		select {
		case err := <-done:
			t.Logf("Daemon with errors exited: %v", err)
			// Should exit even with errors during shutdown
		case <-time.After(15 * time.Second):
			t.Log("Daemon did not exit, forcing kill")
			if daemonCmd.Process != nil {
				if err := daemonCmd.Process.Kill(); err != nil {
					t.Logf("Warning: failed to kill daemon process: %v", err)
				}
			}
			if err := daemonCmd.Wait(); err != nil {
				t.Logf("Warning: daemon wait after kill: %v", err)
			}
			t.Fatal("Daemon should have exited despite errors")
		}
	})
}

func TestGracefulShutdown_CrossPlatform(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cross-platform tests in short mode")
	}

	// Test platform-specific signal handling
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-platform-config.json")
	repoPath := tempDir

	configContent := `{
		"machine": {
			"name": "test-platform-signals"
		},
		"git": {
			"repo_path": "` + repoPath + `",
			"remote_url": "https://github.com/test/test.git",
			"auth_type": "none"
		},
		"sync": {
			"auto_sync_enabled": false,
			"debounce_seconds": 1
		}
	}`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Get path to current executable
	execPath, err := os.Executable()
	require.NoError(t, err)

	t.Run("SIGINT Signal Handling", func(t *testing.T) {
		daemonArgs := []string{"test", "--config", configPath, "start", "--foreground"}
		daemonCmd := exec.Command(execPath, daemonArgs...)
		daemonCmd.Env = append(os.Environ(), "GO_TEST_DAEMON=1")

		t.Log("Starting daemon for SIGINT test...")
		err = daemonCmd.Start()
		require.NoError(t, err)

		// Give daemon time to initialize
		time.Sleep(2 * time.Second)

		// Send SIGINT (Ctrl+C equivalent)
		t.Log("Sending SIGINT signal...")
		err = daemonCmd.Process.Signal(syscall.SIGINT)
		require.NoError(t, err)

		// Wait for process to exit
		done := make(chan error, 1)
		go func() {
			done <- daemonCmd.Wait()
		}()

		select {
		case err := <-done:
			t.Logf("Daemon handled SIGINT, exited: %v", err)
		case <-time.After(15 * time.Second):
			t.Log("Daemon did not exit on SIGINT, forcing kill")
			if daemonCmd.Process != nil {
				if err := daemonCmd.Process.Kill(); err != nil {
					t.Logf("Warning: failed to kill daemon process: %v", err)
				}
			}
			if err := daemonCmd.Wait(); err != nil {
				t.Logf("Warning: daemon wait after kill: %v", err)
			}
			t.Fatal("Daemon should have exited on SIGINT")
		}
	})
}

func TestGracefulShutdown_Coordination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping coordination tests in short mode")
	}

	// Test that shutdown coordination works properly
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-coordination-config.json")
	repoPath := tempDir

	configContent := `{
		"machine": {
			"name": "test-coordination"
		},
		"git": {
			"repo_path": "` + repoPath + `",
			"remote_url": "https://github.com/test/test.git",
			"auth_type": "none"
		},
		"sync": {
			"auto_sync_enabled": false,
			"debounce_seconds": 1
		}
	}`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Get path to current executable
	execPath, err := os.Executable()
	require.NoError(t, err)

	t.Run("Shutdown Coordination Timing", func(t *testing.T) {
		daemonArgs := []string{"test", "--config", configPath, "start", "--foreground"}
		daemonCmd := exec.Command(execPath, daemonArgs...)
		daemonCmd.Env = append(os.Environ(), "GO_TEST_DAEMON=1")

		t.Log("Starting daemon for coordination test...")
		err = daemonCmd.Start()
		require.NoError(t, err)

		// Give daemon time to initialize
		time.Sleep(2 * time.Second)

		startTime := time.Now()

		// Send SIGTERM for graceful shutdown
		t.Log("Sending SIGTERM to test coordination timing...")
		err = daemonCmd.Process.Signal(syscall.SIGTERM)
		require.NoError(t, err)

		// Wait for process to exit
		done := make(chan error, 1)
		go func() {
			done <- daemonCmd.Wait()
		}()

		select {
		case err := <-done:
			shutdownDuration := time.Since(startTime)
			t.Logf("Daemon shutdown completed in %v with status: %v", shutdownDuration, err)
			// Shutdown should complete within reasonable time (less than total timeout)
			assert.Less(t, shutdownDuration, 20*time.Second, "Shutdown should complete within timeout")
		case <-time.After(25 * time.Second):
			t.Log("Daemon coordination test timed out, forcing kill")
			if daemonCmd.Process != nil {
				if err := daemonCmd.Process.Kill(); err != nil {
					t.Logf("Warning: failed to kill daemon process: %v", err)
				}
			}
			if err := daemonCmd.Wait(); err != nil {
				t.Logf("Warning: daemon wait after kill: %v", err)
			}
			t.Fatal("Daemon should have completed shutdown coordination within timeout")
		}
	})
}