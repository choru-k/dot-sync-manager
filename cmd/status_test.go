package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/status"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShowRichDaemonStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   *status.DaemonStatus
		expected []string
	}{
		{
			name: "running daemon with syncs",
			status: &status.DaemonStatus{
				PID:            1234,
				CurrentState:   status.StateRunning,
				Uptime:         5*time.Minute + 30*time.Second,
				Version:        "1.0.0",
				SyncCount:      10,
				ErrorCount:     1,
				FilesSynced:    25,
				LastSync:       time.Now().Add(-2 * time.Minute),
				LastSyncResult: "synced 3 files",
				ConfigPath:     "/test/config.json",
				WatchedPaths:   []string{"/path1", "/path2", "/path3"},
			},
			expected: []string{
				"🟢 Daemon: running (PID: 1234)",
				"⏱️  Uptime:", // Don't check exact uptime due to timing
				"📦 Version: 1.0.0",
				"🔄 Total syncs: 10",
				"❌ Error count: 1",
				"📁 Files synced: 25",
				"🕐 Last sync: 2m",
				"📝 Last result: synced 3 files",
				"👁️  Watching 3 paths:",
				"⚙️  Configuration: /test/config.json",
			},
		},
		{
			name: "syncing daemon",
			status: &status.DaemonStatus{
				PID:          5678,
				CurrentState: status.StateSyncing,
				Uptime:       1 * time.Hour,
				Version:      "2.0.0",
				SyncCount:    0,
				ErrorCount:   0,
				FilesSynced:  0,
				ConfigPath:   "/test/config.json",
			},
			expected: []string{
				"🔄 Daemon: syncing (PID: 5678)",
				"⏱️  Uptime:", // Don't check exact uptime due to timing
				"📦 Version: 2.0.0",
				"🔄 Total syncs: 0",
				"❌ Error count: 0",
				"📁 Files synced: 0",
				"🕐 Last sync: Never",
			},
		},
		{
			name: "daemon with error",
			status: &status.DaemonStatus{
				PID:          9999,
				CurrentState: status.StateError,
				Uptime:       30 * time.Minute,
				Version:      "1.5.0",
				SyncCount:    5,
				ErrorCount:   2,
				FilesSynced:  12,
				LastError:    "git push failed: remote rejected",
				ConfigPath:   "/test/config.json",
			},
			expected: []string{
				"🔴 Daemon: error (PID: 9999)",
				"⏱️  Uptime:", // Don't check exact uptime due to timing
				"🔄 Total syncs: 5",
				"❌ Error count: 2",
				"⚠️  Last Error: git push failed: remote rejected",
			},
		},
		{
			name: "daemon with many watched paths",
			status: &status.DaemonStatus{
				PID:          1111,
				CurrentState: status.StateRunning,
				Uptime:       2 * time.Hour + 15*time.Minute,
				Version:      "1.0.0",
				ConfigPath:   "/test/config.json",
				WatchedPaths: []string{
					"/path1", "/path2", "/path3", "/path4", "/path5",
					"/path6", "/path7", "/path8", "/path9", "/path10",
				},
			},
			expected: []string{
				"👁️  Watching 10 paths:",
				"📂 /path1",
				"📂 /path2",
				"📂 /path3",
				"📂 /path4",
				"📂 /path5",
				"... and 5 more paths",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)
			os.Stdout = w

			// Call the function
			showRichDaemonStatus(tt.status)

			// Close writer and read output
			_ = w.Close()
			os.Stdout = oldStdout
			output, err := io.ReadAll(r)
			require.NoError(t, err)

			outputStr := string(output)

			// Check that all expected substrings are present
			for _, expected := range tt.expected {
				assert.Contains(t, outputStr, expected, "Output should contain: %s", expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"seconds", 30 * time.Second, "30s"},
		{"minutes and seconds", 2*time.Minute + 30*time.Second, "2m 30s"},
		{"hours and minutes", 2*time.Hour + 30*time.Minute, "2h 30m"},
		{"days and hours", 2*24*time.Hour + 5*time.Hour, "2d 5h"},
		{"exactly one minute", 1 * time.Minute, "1m 0s"},
		{"exactly one hour", 1 * time.Hour, "1h 0m"},
		{"exactly one day", 24 * time.Hour, "1d 0h"},
		{"zero duration", 0, "0s"},
		{"less than one second", 500 * time.Millisecond, "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRunStatus_WithDaemon(t *testing.T) {
	// This test would require mocking the Unix socket connection
	// For now, we'll test the fallback behavior
	t.Skip("Requires Unix socket mocking - test fallback behavior instead")
}

func TestRunStatus_FallbackBehavior(t *testing.T) {
	// Create a mock command
	cmd := &cobra.Command{}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	// Call runStatus - this will use fallback behavior since no daemon is running
	_ = runStatus(cmd, []string{})

	// Close writer and read output
	_ = w.Close()
	os.Stdout = oldStdout
	output, err := io.ReadAll(r)
	require.NoError(t, err)

	outputStr := string(output)

	// Should contain fallback indicators
	assert.Contains(t, outputStr, "🔴 Daemon Status Server: Not available")
	assert.Contains(t, outputStr, "🔴 Daemon: Not running")
	assert.Contains(t, outputStr, "📊 Dotfile Sync Manager Status")
}

func TestShowMappings(t *testing.T) {
	tests := []struct {
		name     string
		mappings map[string]string
		expected []string
	}{
		{
			name:     "no mappings",
			mappings: map[string]string{},
			expected: []string{},
		},
		{
			name: "single mapping",
			mappings: map[string]string{
				"~/.bashrc": "dotfiles/bashrc",
			},
			expected: []string{
				"🔗 File Mappings (1):",
				"~/.bashrc -> dotfiles/bashrc",
			},
		},
		{
			name: "multiple mappings",
			mappings: map[string]string{
				"~/.bashrc": "dotfiles/bashrc",
				"~/.vimrc":  "dotfiles/vimrc",
				"~/.zshrc":  "dotfiles/zshrc",
			},
			expected: []string{
				"🔗 File Mappings (3):",
				"~/.bashrc -> dotfiles/bashrc",
				"~/.vimrc -> dotfiles/vimrc",
				"~/.zshrc -> dotfiles/zshrc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)
			os.Stdout = w

			// Call the function
			showFileMappings(tt.mappings)

			// Close writer and read output
			_ = w.Close()
			os.Stdout = oldStdout
			output, err := io.ReadAll(r)
			require.NoError(t, err)

			outputStr := string(output)

			if len(tt.expected) == 0 {
				assert.Empty(t, strings.TrimSpace(outputStr))
			} else {
				for _, expected := range tt.expected {
					assert.Contains(t, outputStr, expected, "Output should contain: %s", expected)
				}
			}
		})
	}
}

// Test helper function to capture command output
func captureCommandOutput(cmd *cobra.Command, _ []string) (string, error) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	// Set command output to our pipe
	cmd.SetOut(w)
	cmd.SetErr(w)

	// Execute command
	_ = cmd.Execute()

	// Close writer and read output
	_ = w.Close()
	os.Stdout = oldStdout
	output, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// Test integration with status command
func TestStatusCommand_Integration(t *testing.T) {
	// Test the status command directly
	cmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {
			err := runStatus(cmd, args)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		},
	}

	// Set up command args
	cmd.SetArgs([]string{})

	// Capture output
	output, err := captureCommandOutput(cmd, []string{})

	// Should not error and should show basic output
	require.NoError(t, err)
	assert.Contains(t, output, "📊 Dotfile Sync Manager Status")
	assert.Contains(t, output, "🔴 Daemon Status Server: Not available")
}