package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/util"
	"github.com/google/shlex"
)

// getDefaultEditor returns the appropriate text editor for the current platform
// It checks the EDITOR environment variable first, then falls back to platform defaults
func getDefaultEditor() string {
	// Check environment variable first
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	// Platform-specific fallbacks
	switch runtime.GOOS {
	case "windows":
		return editorNotepad
	case "darwin":
		return editorTextEdit
	default: // Linux and others
		return editorNano
	}
}

// getDefaultEditorForFile returns an appropriate editor for a specific file type
func getDefaultEditorForFile(filePath string) string {
	// Check environment variable first
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	// File type-specific editor selection
	switch {
	case strings.HasSuffix(filePath, ".json"), strings.HasSuffix(filePath, ".jsonc"):
		return editorVSCode // Prefer VS Code for JSON files
	case strings.HasSuffix(filePath, ".md"), strings.HasSuffix(filePath, ".markdown"):
		return editorVSCode // Prefer VS Code for Markdown
	case strings.HasSuffix(filePath, ".txt"):
		return editorVSCode // Prefer VS Code for text files
	default:
		return getDefaultEditor()
	}
}


// checkDaemonStatus checks if the daemon is running and provides status information
func checkDaemonStatus() (bool, string, error) {
	pidFile := "/tmp/dsm-daemon.pid"

	// Check if PID file exists
	if _, err := os.Stat(pidFile); err != nil {
		if os.IsNotExist(err) {
			return false, "Daemon is not running", nil
		}
		return false, fmt.Sprintf("Unable to check daemon status: %v", err), err
	}

	// Read PID from file
	content, err := os.ReadFile(pidFile)
	if err != nil {
		return false, fmt.Sprintf("Unable to read daemon PID file: %v", err), err
	}

	pidStr := strings.TrimSpace(string(content))
	if pidStr == "" {
		return false, "Daemon PID file is empty", nil
	}

	// Check if process is running
	cmd := exec.Command("ps", "-p", pidStr)
	if err := cmd.Run(); err != nil {
		return false, "Daemon is not running", nil
	}

	return true, fmt.Sprintf("Daemon is running (PID: %s)", pidStr), nil
}


// parseCommand splits a command string into command and arguments using shlex
// This handles quoted arguments and returns a command and args slice
func parseCommand(cmdStr string) (string, []string) {
	if cmdStr == "" {
		return "", nil
	}

	parts, err := shlex.Split(cmdStr)
	if err != nil {
		// Fallback to simple split if shlex fails
		parts = strings.Fields(cmdStr)
	}

	if len(parts) == 0 {
		return "", nil
	}

	return parts[0], parts[1:]
}

// hasCommand checks if a command exists in the system PATH
func hasCommand(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}


// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		   (s == substr ||
		    len(substr) == 0 ||
		    (len(s) > len(substr) && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

