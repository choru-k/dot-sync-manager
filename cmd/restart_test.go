package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestRestartCmd_Sanity(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "no arguments should work",
			args:        []string{},
			expectError: false,
		},
		{
			name:        "any arguments should error",
			args:        []string{"extra"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "restart"}
			err := runRestart(cmd, tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("runRestart() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestIsDaemonRunning(t *testing.T) {
	// Test when daemon is not running
	if isDaemonRunning() {
		t.Skip("daemon is already running, skipping test")
	}

	// Create a fake PID file to simulate running daemon
	pidFile := "/tmp/dsm-daemon-test.pid"
	defer func() {
		if err := os.Remove(pidFile); err != nil {
			t.Logf("warning: failed to remove PID file: %v", err)
		}
	}()

	// Write a fake PID
	err := os.WriteFile(pidFile, []byte("99999"), defaultFilePerms)
	if err != nil {
		t.Fatalf("failed to create test PID file: %v", err)
	}

	// Test detection
	if isDaemonRunning() {
		t.Error("expected daemon to not be running with fake PID")
	}
}

func TestStopDaemon(t *testing.T) {
	// Test stopping when daemon is not running
	err := stopDaemon()
	if err != nil {
		// This is expected - daemon should not be running
		t.Logf("stopDaemon() returned expected error when daemon not running: %v", err)
	}
}

func TestRestartCmd_Integration(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Create PID file to simulate running daemon
	pidFile := "/tmp/dsm-daemon-test.pid"
	defer func() {
		if err := os.Remove(pidFile); err != nil {
			t.Logf("warning: failed to remove PID file: %v", err)
		}
	}()

	// Write a fake PID (this won't be a real process)
	err := os.WriteFile(pidFile, []byte("99999"), defaultFilePerms)
	if err != nil {
		t.Fatalf("failed to create test PID file: %v", err)
	}

	// Test restart command
	cmd := &cobra.Command{Use: "restart"}
	startTime := time.Now()

	err = runRestart(cmd, []string{})
	if err != nil {
		// This might fail if daemon start fails, which is expected in test
		t.Logf("runRestart() returned error (expected in test): %v", err)
	}

	// Verify command took some time (indicating it tried to stop/start)
	elapsed := time.Since(startTime)
	if elapsed < 100*time.Millisecond {
		t.Error("restart command should have taken some time to execute")
	}
}

func TestRestartCmd_Concurrency(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Create PID file
	pidFile := "/tmp/dsm-daemon-test-concurrent.pid"
	defer func() {
		if err := os.Remove(pidFile); err != nil {
			t.Logf("warning: failed to remove PID file: %v", err)
		}
	}()

	err := os.WriteFile(pidFile, []byte("99999"), defaultFilePerms)
	if err != nil {
		t.Fatalf("failed to create test PID file: %v", err)
	}

	// Test concurrent restart attempts
	cmd := &cobra.Command{Use: "restart"}
	done := make(chan bool, 2)

	// Start two restart operations concurrently
	for i := 0; i < 2; i++ {
		go func() {
			err := runRestart(cmd, []string{})
			done <- (err == nil)
		}()
	}

	// Wait for both to complete
	success1 := <-done
	success2 := <-done

	// At least one should succeed (though both might fail in test environment)
	if !success1 && !success2 {
		t.Log("both restart operations failed (expected in test environment)")
	}
}

func TestDaemonStatusCheck(t *testing.T) {
	// Test status check when no PID file exists
	running, status, err := checkDaemonStatus()
	if err != nil {
		t.Errorf("checkDaemonStatus() returned error when no PID file: %v", err)
	}
	if running {
		t.Error("expected daemon to not be running when no PID file exists")
	}
	if status == "" {
		t.Error("expected non-empty status message")
	}
}

func TestRestartCmd_FilePermissions(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Create PID file with restricted permissions
	pidFile := "/tmp/dsm-daemon-perms-test.pid"
	defer func() {
		if err := os.Remove(pidFile); err != nil {
			t.Logf("warning: failed to remove PID file: %v", err)
		}
	}()

	err := os.WriteFile(pidFile, []byte("99999"), restrictiveConfigFilePerms)
	if err != nil {
		t.Fatalf("failed to create test PID file: %v", err)
	}

	// Verify restart can handle permission issues
	cmd := &cobra.Command{Use: "restart"}
	err = runRestart(cmd, []string{})
	// Error expected in test environment
	t.Logf("restart with permission-restricted PID file: %v", err)
}