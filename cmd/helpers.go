package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/process"
	"github.com/google/shlex"
)

// getDefaultEditor returns the appropriate text editor for the current platform
// It checks the EDITOR environment variable first, then falls back to platform defaults
func getDefaultEditor() (string, error) {
	// Check if we're in a test environment and should avoid launching real editors
	if os.Getenv("DSM_TEST_MODE") != "" || os.Getenv("CI") != "" {
		return "true", nil // Use true as a safe test editor - silent and no GUI
	}

	// Check environment variable first
	if editor := os.Getenv("EDITOR"); editor != "" {
		return validateEditorCommand(editor)
	}

	// Platform-specific fallbacks (these are pre-validated)
	switch runtime.GOOS {
	case "windows":
		return editorNotepad, nil
	case "darwin":
		return editorTextEdit, nil
	default: // Linux and others
		return editorNano, nil
	}
}

// getDefaultEditorForFile returns an appropriate editor for a specific file type
func getDefaultEditorForFile(filePath string) (string, error) {
	// Check if we're in a test environment and should avoid launching real editors
	if os.Getenv("DSM_TEST_MODE") != "" || os.Getenv("CI") != "" {
		return "true", nil // Use true as a safe test editor - silent and no GUI
	}

	// Check environment variable first
	if editor := os.Getenv("EDITOR"); editor != "" {
		return validateEditorCommand(editor)
	}

	// File type-specific editor selection with fallback checking
	switch {
	case strings.HasSuffix(filePath, ".json"), strings.HasSuffix(filePath, ".jsonc"):
		// Prefer VS Code for JSON files, but fall back if not available
		if hasCommand("code") {
			return editorVSCode, nil
		}
		return getDefaultEditor()
	case strings.HasSuffix(filePath, ".md"), strings.HasSuffix(filePath, ".markdown"):
		// Prefer VS Code for Markdown, but fall back if not available
		if hasCommand("code") {
			return editorVSCode, nil
		}
		return getDefaultEditor()
	case strings.HasSuffix(filePath, ".txt"):
		// Prefer VS Code for text files, but fall back if not available
		if hasCommand("code") {
			return editorVSCode, nil
		}
		return getDefaultEditor()
	default:
		return getDefaultEditor()
	}
}


// checkDaemonStatus checks if the daemon is running and provides status information
// Uses the process package with proper fallbacks for cross-platform compatibility
func checkDaemonStatus() (bool, string) {
	// Use the process package's IsDaemonRunning function which has proper fallbacks
	if process.IsDaemonRunning() {
		// Get the PID for more detailed status
		if pid, err := process.GetDaemonPID(); err == nil {
			return true, fmt.Sprintf("Daemon is running (PID: %d)", pid)
		}
		return true, "Daemon is running"
	}

	return false, "Daemon is not running"
}


// parseCommand splits a command string into command and arguments using shlex
// This handles quoted arguments and returns a command and args slice
func parseCommand(cmdStr string) (string, []string) {
	if cmdStr == "" {
		return "", nil
	}

	parts, err := shlex.Split(cmdStr)
	if err != nil {
		// Return empty result on parsing error to prevent command injection
		// rather than falling back to unsafe string splitting
		return "", nil
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

// getSafeEditorResult returns the appropriate editor result based on the test environment
func getSafeEditorResult(editor string) string {
	// Check if we're in a test environment and should avoid launching real editors
	if os.Getenv("DSM_TEST_MODE") != "" || os.Getenv("CI") != "" {
		// In CI/test mode, return "true" for ALL editors to prevent actual execution
		// This is because CI environments typically lack GUI editors and to prevent hanging tests
		return "true"
	}

	// For local development, always validate and return actual editor names
	// This ensures proper security validation while allowing real editor use during development
	return editor
}

// validateEditorCommand validates that an editor command is safe against command injection
func validateEditorCommand(editor string) (string, error) {
	// Strip whitespace and check for empty
	editor = strings.TrimSpace(editor)
	if editor == "" {
		return "", fmt.Errorf("editor command cannot be empty")
	}

	// Check against allowlist of safe editors
	if safeEditors[editor] {
		return getSafeEditorResult(editor), nil
	}

	// For multi-word commands, check the base command
	parts := strings.Fields(editor)
	if len(parts) > 0 && safeEditors[parts[0]] {
		// Additional validation for known safe multi-word commands
		if strings.HasPrefix(editor, "open -a ") {
			// Validate macOS 'open -a AppName' format
			appName := strings.TrimPrefix(editor, "open -a ")
			if len(appName) > 0 && appName[0] != '"' && !strings.ContainsAny(appName, "&|;`$(){}[]<>*?") {
				return getSafeEditorResult(editor), nil
			}
		}
	}

	// Reject any editor containing dangerous characters
	dangerousChars := "&|;`$(){}[]<>*?~\\"
	for _, char := range dangerousChars {
		if strings.Contains(editor, string(char)) {
			return "", fmt.Errorf("editor command contains dangerous character: %q", char)
		}
	}

	// Reject shell metacharacters and command injection patterns
	// Focus only on command injection characters, allow legitimate path traversal
	injectionPatterns := []string{
		// Shell metacharacters and command injection
		"&&", "||", ";", "&", "|", "`", "$(", "${", // Command chaining
		">", ">>", "<", "<<", "2>", "2>>", // Redirection operators
		"$", "`", "\\\"", "\\", "!", "*", "?", "[", "]", "{", "}", // Special characters

		// Dangerous commands and system calls
		"rm ", "del ", "format ", "fdisk ", "mkfs ", // File system operations
		"sudo", "su ", "chmod ", "chown ", "passwd ", // Privilege escalation
		"exec", "eval", "system", "cmd.exe", "powershell", // Command execution
		"nc ", "netcat", "wget", "curl", "ssh", "telnet", // Network commands
		"python", "perl", "ruby", "bash", "sh", "cmd", // Script execution

		// Process and system manipulation
		"kill ", "killall", "pkill ", "taskkill ", // Process termination
		"ps ", "top", "htop", "tasklist", // Process listing
		"nohup ", "disown", "screen", "tmux ", // Session management

		// File and data exfiltration patterns
		"cat ", "type ", "more ", "less ", "head ", "tail ", // File reading
		"grep ", "find ", "locate ", "which ", "whereis ", // File searching
		"tar ", "zip ", "gzip ", "bzip2 ", "7z ", // Archiving/compression
		"scp ", "sftp ", "rsync ", "ftp ", // File transfer

		// Note: Path traversal patterns (.., ~/, etc.) are allowed as they are legitimate for editor paths
	}

	editorLower := strings.ToLower(editor)
	for _, pattern := range injectionPatterns {
		if strings.Contains(editorLower, pattern) {
			return "", fmt.Errorf("editor command contains potentially dangerous pattern: %q", pattern)
		}
	}

	// If we get here, the editor passed basic safety checks but isn't in our allowlist
	// Log a warning but allow it for flexibility in development environments
	printWarning("Editor %q is not in the allowlist of known safe editors", editor)
	return getSafeEditorResult(editor), nil
}

// Emoji helper functions for conditional emoji output

// emoji returns the emoji if emojis are enabled, otherwise returns the alternative text
func emoji(emojiChar, altText string) string {
	if noEmoji {
		return altText
	}
	return emojiChar
}

// printSuccess prints a success message with optional emoji
func printSuccess(format string, args ...interface{}) {
	prefix := emoji("✅", "SUCCESS:")
	fmt.Printf(prefix+" "+format+"\n", args...)
}

// printError prints an error message with optional emoji
func printError(format string, args ...interface{}) {
	prefix := emoji("❌", "ERROR:")
	fmt.Printf(prefix+" "+format+"\n", args...)
}

// printWarning prints a warning message with optional emoji
func printWarning(format string, args ...interface{}) {
	prefix := emoji("⚠️", "WARNING:")
	fmt.Printf(prefix+" "+format+"\n", args...)
}

// printInfo prints an info message with optional emoji
func printInfo(format string, args ...interface{}) {
	prefix := emoji("ℹ️", "INFO:")
	fmt.Printf(prefix+" "+format+"\n", args...)
}

// printStatus prints a status message with optional emoji
func printStatus(emojiChar, altText, format string, args ...interface{}) {
	prefix := emoji(emojiChar, altText+":")
	fmt.Printf(prefix+" "+format+"\n", args...)
}




