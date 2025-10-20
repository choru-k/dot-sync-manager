package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestLogCmd_Sanity(t *testing.T) {
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
			name:        "single file argument should work",
			args:        []string{"/tmp/test.log"},
			expectError: false,
		},
		{
			name:        "multiple arguments should error",
			args:        []string{"/tmp/test.log", "/tmp/test2.log"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "log"}
			err := runLog(cmd, tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("runLog() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestGetLogFile(t *testing.T) {
	tests := []struct {
		name           string
		configPath     string
		expectedResult string
		expectError    bool
	}{
		{
			name:           "default log path",
			configPath:     "",
			expectedResult: "/tmp/dsm.log", // Default expected location
			expectError:    false,
		},
		{
			name:        "non-existent config",
			configPath:  "/tmp/non-existent-config.json",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logFile, err := getLogFile()

			if (err != nil) != tt.expectError {
				t.Errorf("getLogFile() error = %v, expectError %v", err, tt.expectError)
			}

			if !tt.expectError && logFile == "" {
				t.Error("expected non-empty log file path")
			}
		})
	}
}

func TestTailLogFile(t *testing.T) {
	tests := []struct {
		name        string
		setupLog    func(t *testing.T) string
		lines       int
		expectError bool
	}{
		{
			name: "non-existent log file",
			setupLog: func(t *testing.T) string {
				return "/tmp/non-existent-test.log"
			},
			lines:       10,
			expectError: true,
		},
		{
			name: "empty log file",
			setupLog: func(t *testing.T) string {
				logFile := filepath.Join(t.TempDir(), "test.log")
				createTestFile(t, logFile, "")
				return logFile
			},
			lines:       10,
			expectError: false,
		},
		{
			name: "log file with content",
			setupLog: func(t *testing.T) string {
				logFile := filepath.Join(t.TempDir(), "test.log")
				content := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n"
				createTestFile(t, logFile, content)
				return logFile
			},
			lines:       3,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logFile := tt.setupLog(t)
			err := tailLogFile(logFile, tt.lines)

			if (err != nil) != tt.expectError {
				t.Errorf("tailLogFile() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestParseLogLines(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		lines       int
		expectCount int
		expectError bool
	}{
		{
			name:        "empty content",
			content:     "",
			lines:       10,
			expectCount: 0,
			expectError: false,
		},
		{
			name:        "single line",
			content:     "Single log line",
			lines:       10,
			expectCount: 1,
			expectError: false,
		},
		{
			name:        "multiple lines",
			content:     "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n",
			lines:       3,
			expectCount: 3,
			expectError: false,
		},
		{
			name:        "more lines than requested",
			content:     "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\n",
			lines:       3,
			expectCount: 3,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logLines := parseLogLines(tt.content, tt.lines)

			if len(logLines) != tt.expectCount {
				t.Errorf("parseLogLines() returned %d lines, expected %d", len(logLines), tt.expectCount)
			}

			// Verify that returned lines are the last N lines
			allLines := splitLines(tt.content)
			if len(allLines) > 0 && len(logLines) > 0 {
				expectedStart := len(allLines) - min(len(allLines), tt.lines)
				for i, line := range logLines {
					if expectedStart+i < len(allLines) && line != allLines[expectedStart+i] {
						t.Errorf("log line mismatch at index %d: got %q, expected %q", i, line, allLines[expectedStart+i])
					}
				}
			}
		})
	}
}

func TestLogCmd_Integration(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Create a log file with test content
	logFile := filepath.Join(testConfig.RepoPath, "dsm.log")
	logContent := `2024-01-15T10:00:00Z INFO Starting dotfile sync daemon
2024-01-15T10:00:01Z INFO Watching 5 files
2024-01-15T10:00:02Z INFO Changes detected in .bashrc
2024-01-15T10:00:03Z INFO Syncing changes to repository
2024-01-15T10:00:04Z INFO Sync completed successfully
2024-01-15T10:00:05Z ERROR Failed to sync .vimrc: permission denied`
	createTestFile(t, logFile, logContent)

	// Test log command
	cmd := &cobra.Command{Use: "log"}
	err := runLog(cmd, []string{logFile})
	if err != nil {
		t.Errorf("runLog() failed: %v", err)
	}
}

func TestLogCmd_CustomLines(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Create a log file with many lines
	logFile := filepath.Join(testConfig.RepoPath, "dsm.log")
	var logContent string
	for i := 1; i <= 20; i++ {
		logContent += fmt.Sprintf("2024-01-15T10:%02d:%02dZ INFO Log line %d\n", i/60, i%60, i)
	}
	createTestFile(t, logFile, logContent)

	// Test log command with custom line count
	cmd := &cobra.Command{Use: "log"}
	if err := cmd.Flags().Set("lines", "5"); err != nil {
		t.Fatalf("failed to set lines flag: %v", err)
	}

	err := runLog(cmd, []string{logFile})
	if err != nil {
		t.Errorf("runLog() failed: %v", err)
	}
}

func TestLogCmd_FollowFlag(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Create a log file
	logFile := filepath.Join(testConfig.RepoPath, "dsm.log")
	createTestFile(t, logFile, "Initial log line\n")

	// Test log command with follow flag
	cmd := &cobra.Command{Use: "log"}
	if err := cmd.Flags().Set("follow", "true"); err != nil {
		t.Fatalf("failed to set follow flag: %v", err)
	}

	// Run in a goroutine since follow mode blocks
	done := make(chan error, 1)
	go func() {
		done <- runLog(cmd, []string{logFile})
	}()

	// Add a new line to the log file after a short delay
	time.Sleep(100 * time.Millisecond)
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, defaultFilePerms)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Logf("warning: failed to close log file: %v", err)
		}
	}()

	_, err = file.WriteString("New log line\n")
	if err != nil {
		t.Fatalf("failed to write to log file: %v", err)
	}

	// Wait a bit more then signal completion
	time.Sleep(100 * time.Millisecond)
	// In a real test, we'd need a way to stop the follow mode
	// For now, just wait and check if it's still running
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("runLog() with follow failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		// Expected - follow mode would block
		t.Log("follow mode still running (expected)")
	}
}

func TestLogCmd_NonExistentFile(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Test log command with non-existent file
	cmd := &cobra.Command{Use: "log"}
	err := runLog(cmd, []string{"/tmp/non-existent-test.log"})
	if err == nil {
		t.Error("expected runLog() to fail with non-existent file")
	}
}

func TestLogCmd_EmptyFile(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Create empty log file
	logFile := filepath.Join(testConfig.RepoPath, "dsm.log")
	createTestFile(t, logFile, "")

	// Test log command with empty file
	cmd := &cobra.Command{Use: "log"}
	err := runLog(cmd, []string{logFile})
	if err != nil {
		t.Errorf("runLog() failed with empty file: %v", err)
	}
}

func TestLogPathExpansion(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	defer cleanupTestEnvironment(testConfig)

	// Create log file with ~ path
	homeLogFile := filepath.Join(testConfig.HomeDir, "dsm.log")
	createTestFile(t, homeLogFile, "Log file in home directory")

	// Test log command with tilde path
	cmd := &cobra.Command{Use: "log"}
	err := runLog(cmd, []string{"~/dsm.log"})
	if err != nil {
		t.Errorf("runLog() failed with tilde path: %v", err)
	}
}

func TestValidateLogFile(t *testing.T) {
	tests := []struct {
		name        string
		logFile     string
		expectError bool
	}{
		{
			name:        "empty path",
			logFile:     "",
			expectError: true,
		},
		{
			name:        "non-existent file",
			logFile:     "/tmp/non-existent.log",
			expectError: true,
		},
		{
			name:        "existing file",
			logFile:     func() string { file := filepath.Join(t.TempDir(), "test.log"); createTestFile(t, file, "test"); return file }(),
			expectError: false,
		},
		{
			name:        "directory instead of file",
			logFile:     t.TempDir(),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLogFile(tt.logFile)
			if (err != nil) != tt.expectError {
				t.Errorf("validateLogFile() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

// Helper functions
func splitLines(content string) []string {
	if content == "" {
		return []string{}
	}
	lines := []string{}
	for _, line := range strings.Split(content, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

