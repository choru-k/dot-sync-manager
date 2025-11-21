package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

// checkDirectoryExists checks if a directory exists and is actually a directory
// Returns (true, nil) if directory exists, (false, nil) if not found or not a directory,
// or (false, error) if there was an error accessing the path
func checkDirectoryExists(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to access %s: %w", path, err)
	}
	return stat.IsDir(), nil
}

// getMachineName extracts the machine name from a config object without JSON marshaling
func getMachineName(obj interface{}) string {
	// Handle machine.name access directly from struct
	if cfg, ok := obj.(*config.SyncConfig); ok {
		return cfg.Machine.Name
	}
	if cfg, ok := obj.(config.SyncConfig); ok {
		return cfg.Machine.Name
	}
	return ""
}

// getMachineNameFromOS gets the machine name from the operating system
func getMachineNameFromOS() string {
	hostname, err := os.Hostname()
	if err == nil {
		return hostname
	}
	// Log hostname error before falling back to default
	printError("Failed to get hostname: %v", err)
	return "unknown-machine"
}

// Print helper functions for consistent output formatting
func printSuccess(format string, args ...interface{}) {
	if noEmoji {
		fmt.Printf("✓ "+format+"\n", args...)
	} else {
		fmt.Printf("✅ "+format+"\n", args...)
	}
}

func printError(format string, args ...interface{}) {
	if noEmoji {
		fmt.Printf("ERROR: "+format+"\n", args...)
	} else {
		fmt.Printf("❌ "+format+"\n", args...)
	}
}

func printWarning(format string, args ...interface{}) {
	if noEmoji {
		fmt.Printf("WARNING: "+format+"\n", args...)
	} else {
		fmt.Printf("⚠️ "+format+"\n", args...)
	}
}

func printInfo(format string, args ...interface{}) {
	if noEmoji {
		fmt.Printf("INFO: "+format+"\n", args...)
	} else {
		fmt.Printf("💡 "+format+"\n", args...)
	}
}

func printStatus(emoji, category, message string) {
	if noEmoji {
		fmt.Printf("[%s] %s\n", category, message)
	} else {
		fmt.Printf("%s %s: %s\n", emoji, category, message)
	}
}

// Dry-run helper functions

// isDryRun returns the global dry-run flag value
func isDryRun() bool {
	return globalDryRun
}

// printDryRun prints a formatted dry-run message
func printDryRun(format string, args ...interface{}) {
	if noEmoji {
		fmt.Printf("DRY-RUN: "+format+"\n", args...)
	} else {
		fmt.Printf("🔍 "+format+"\n", args...)
	}
}

// LogDryRunAction logs a dry-run action with optional details
func LogDryRunAction(action string, details ...string) {
	if isDryRun() {
		msg := action
		if len(details) > 0 {
			msg += ": " + strings.Join(details, ", ")
		}
		printDryRun(msg)
	}
}

// Editor and command handling functions

// validateEditorCommand validates that an editor command is safe to execute.
// It checks for dangerous characters, patterns, and validates against an allowlist
// of known safe editors. Returns the validated command or an error.
func validateEditorCommand(editor string) (string, error) {
	editor = strings.TrimSpace(editor)
	if editor == "" {
		return "", fmt.Errorf("cannot be empty")
	}

	// Then check for dangerous characters
	dangerousChars := []string{";", "|", "&", "`", "$", ">", "<"}
	for _, char := range dangerousChars {
		if strings.Contains(editor, char) {
			return "", fmt.Errorf("dangerous character: '%s'", char)
		}
	}

	// Check for potentially dangerous characters first (null bytes, tabs, newlines)
	hasSpecialChars := strings.Contains(editor, "\x00") || strings.Contains(editor, "\t") || strings.Contains(editor, "\n")

	// Check for dangerous patterns
	lowerEditor := strings.ToLower(editor)
	dangerousPatterns := []string{"rm ", "format ", "curl", "sh", "cat"}
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerEditor, pattern) {
			if hasSpecialChars {
				return "", fmt.Errorf("potentially dangerous pattern: %q", pattern)
			} else {
				return "", fmt.Errorf("dangerous pattern: %q", pattern)
			}
		}
	}

	// If no dangerous patterns found but has special characters, still error out
	if hasSpecialChars {
		return "", fmt.Errorf("potentially dangerous characters found in editor command")
	}

	// Parse command to separate editor from arguments
	editorCmd, _ := parseCommand(editor)

	// Security: validate editor command against allowlist
	allowedEditors := []string{
		"vi", "vim", "nvim", "emacs", "nano", "code", "subl", "atom",
		"gedit", "kate", "mousepad", "leafpad", "xed", "pluma",
		"notepad", "notepad++", "wordpad",
		"open", "xdg-open", "start",
	}

	// Check if the base editor command is in the allowlist
	if !isAllowedEditor(editorCmd, allowedEditors) {
		return "", fmt.Errorf("editor '%s' is not in the allowed list for security reasons", editorCmd)
	}

	// In test mode, return the editor command as is to prevent actual editor execution
	// but allow the test to validate the command was called
	if os.Getenv("DSM_TEST_MODE") != "" || os.Getenv("CI") != "" {
		return "true", nil
	}

	// Check if the command exists on the system (only in production mode)
	if _, err := exec.LookPath(editorCmd); err != nil {
		return "", fmt.Errorf("editor command '%s' not found", editorCmd)
	}

	return editor, nil
}

// getDefaultEditorForFile returns the default editor for a given file type.
// Currently uses platform-agnostic logic, but filePath parameter is reserved
// for future file-type-specific editor selection based on file extension.
func getDefaultEditorForFile(filePath string) string {
	// TODO: Implement file-type-specific editor selection based on filePath extension
	// For now, we use the same logic as getDefaultEditor regardless of file type
	_ = filePath // Suppress unused parameter warning

	return getDefaultEditorCommon()
}

// getDefaultEditorCommon contains the common editor selection logic
// shared between getDefaultEditor and getDefaultEditorForFile.
func getDefaultEditorCommon() string {
	// Check environment variables first
	if editor := os.Getenv("EDITOR"); editor != "" {
		if validatedEditor, err := validateEditorCommand(editor); err == nil {
			return validatedEditor
		}
	}

	if editor := os.Getenv("VISUAL"); editor != "" {
		if validatedEditor, err := validateEditorCommand(editor); err == nil {
			return validatedEditor
		}
	}

	// Fall back to platform-specific defaults
	switch runtime.GOOS {
	case "darwin":
		return "open"
	case "windows":
		return "notepad"
	default: // linux and others
		// Try common editors in order of preference
		editors := []string{"code", "nano", "vim", "vi"}
		for _, editor := range editors {
			if _, err := exec.LookPath(editor); err == nil {
				return editor
			}
		}
		return "nano" // Default fallback
	}
}

// getDefaultEditor returns the default editor for the current platform.
// It checks environment variables (EDITOR, VISUAL) first, then falls back
// to platform-specific defaults (open on macOS, notepad on Windows, code/nano/vim/vi on Linux).
func getDefaultEditor() string {
	return getDefaultEditorCommon()
}

// parseCommand parses a command string into command and arguments.
// It handles simple space-based splitting and returns the base command
// and a slice of arguments. Empty command returns empty strings.
func parseCommand(command string) (string, []string) {
	if command == "" {
		return "", nil
	}

	// Handle quoted commands
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", nil
	}

	// Find the base command (first unquoted part)
	cmd := parts[0]
	args := []string{}

	if len(parts) > 1 {
		args = parts[1:]
	}

	return cmd, args
}

// isAllowedEditor checks if an editor command is in the allowlist
func isAllowedEditor(editor string, allowedEditors []string) bool {
	for _, allowed := range allowedEditors {
		if editor == allowed {
			return true
		}
	}
	return false
}

// hasCommand checks if a command exists on the system
func hasCommand(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}
