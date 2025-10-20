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


// parseCommand splits a command string into command and arguments
// This handles quoted arguments and returns a command and args slice
func parseCommand(cmdStr string) (string, []string) {
	if cmdStr == "" {
		return "", nil
	}

	var parts []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)

	for i, char := range cmdStr {
		switch {
		case (char == '\'' || char == '"') && (i == 0 || cmdStr[i-1] != '\\'):
			if inQuotes && byte(char) == quoteChar {
				// End of quoted section
				inQuotes = false
			} else if !inQuotes {
				// Start of quoted section
				inQuotes = true
				quoteChar = byte(char)
			} else {
				// Quote inside quoted string
				current.WriteByte(byte(char))
			}
		case char == ' ' && !inQuotes:
			// End of argument
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(byte(char))
		}
	}

	// Add the last argument
	if current.Len() > 0 {
		parts = append(parts, current.String())
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

// Additional helper functions for tests

func stopDaemon() error {
	pidFile := "/tmp/dsm-daemon.pid"
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return nil // Daemon not running
	}
	return os.Remove(pidFile)
}

func removeFromConfig(key string, config *config.SyncConfig) {
	if config.Mappings != nil {
		delete(config.Mappings, key)
	}
}

func removeSymlink(symlinkPath string) error {
	return os.Remove(symlinkPath)
}

// Helper functions for restart tests
func restartDaemonAfterResolve() error {
	if err := stopDaemon(); err != nil {
		return err
	}
	// In a real implementation, this would start the daemon again
	return nil
}

// Helper functions for check tests
func validateResolutionState(repoPath, conflictsDir string) error {
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository not found")
	}
	return nil
}

func getResolutionStatus(repoPath, conflictsDir string) string {
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return "Repository not found"
	}

	if conflictsDir != "" {
		if files, _ := os.ReadDir(conflictsDir); len(files) > 0 {
			return "Conflicts still exist"
		}
	}

	return "No conflicts found"
}


// Missing conflict helper functions for tests
func detectConflicts(repoPath string) (bool, []string, error) {
	conflictsDir := filepath.Join(repoPath, ".dsm", "conflicts")
	if _, err := os.Stat(conflictsDir); os.IsNotExist(err) {
		return false, nil, nil
	}

	files, err := os.ReadDir(conflictsDir)
	if err != nil {
		return false, nil, err
	}

	var conflicts []string
	for _, file := range files {
		conflicts = append(conflicts, file.Name())
	}

	return len(conflicts) > 0, conflicts, nil
}

func listConflictFiles(conflictsDir string) ([]string, error) {
	if _, err := os.Stat(conflictsDir); os.IsNotExist(err) {
		return nil, nil
	}

	files, err := os.ReadDir(conflictsDir)
	if err != nil {
		return nil, err
	}

	var conflictFiles []string
	for _, file := range files {
		conflictFiles = append(conflictFiles, file.Name())
	}

	return conflictFiles, nil
}

func parseConflictFile(conflictFile string) ([]string, error) {
	content, err := os.ReadFile(conflictFile)
	if err != nil {
		return nil, err
	}

	// Simple parsing - just split by lines
	lines := strings.Split(string(content), "\n")
	var sections []string

	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			sections = append(sections, line)
		}
	}

	return sections, nil
}

func getConflictStatus(conflictsDir string) string {
	if _, err := os.Stat(conflictsDir); os.IsNotExist(err) {
		return "No conflicts detected"
	}

	files, err := os.ReadDir(conflictsDir)
	if err != nil {
		return "Error checking conflicts"
	}

	if len(files) == 0 {
		return "No conflicts detected"
	}

	return "Conflicts detected"
}

func checkUnmergedFiles(repoPath string) (bool, error) {
	// Simplified implementation - in real code would check git status
	return false, nil
}

func cleanupConflictFiles(conflictsDir string) error {
	if _, err := os.Stat(conflictsDir); os.IsNotExist(err) {
		return nil
	}

	return os.RemoveAll(conflictsDir)
}

// Helper functions for resolve tests
func verifyConflictResolution(repoPath string) (bool, error) {
	conflictsDir := filepath.Join(repoPath, ".dsm", "conflicts")
	if _, err := os.Stat(conflictsDir); os.IsNotExist(err) {
		return true, nil
	}

	files, err := os.ReadDir(conflictsDir)
	if err != nil {
		return false, err
	}

	return len(files) == 0, nil
}

// Helper functions for version tests
func getVersionInfo() map[string]string {
	return map[string]string{
		"Version":   Version,
		"Commit":    Commit,
		"BuildDate": BuildDate,
	}
}

func formatVersionOutput(info map[string]string) string {
	var output strings.Builder
	output.WriteString("Version Information:\n")
	for key, value := range info {
		output.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
	}
	return output.String()
}

// Helper functions for open tests
func getOpenCommand(target string) (string, error) {
	if err := validateOpenTarget(target); err != nil {
		return "", err
	}

	editor := getDefaultEditorForFile(target)
	if editor == "" {
		return "", fmt.Errorf("no editor available")
	}

	return fmt.Sprintf("%s %s", editor, target), nil
}

func getPlatformOpenCommand(targetPath string) string {
	switch runtime.GOOS {
	case "darwin":
		return fmt.Sprintf("open %s", targetPath)
	case "windows":
		return fmt.Sprintf("start %s", targetPath)
	default:
		return fmt.Sprintf("xdg-open %s", targetPath)
	}
}

func validateOpenTarget(target string) error {
	if target == "" {
		return fmt.Errorf("target cannot be empty")
	}

	expandedPath, err := util.ExpandPath(target)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}

	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return fmt.Errorf("target does not exist: %s", expandedPath)
	}

	return nil
}

func getEditorCommand(filePath string) string {
	return getDefaultEditorForFile(filePath)
}

// Helper functions for log tests
func getLogFile(configPath string) (string, error) {
	if configPath == "" {
		return "/tmp/dsm.log", nil
	}

	// In a real implementation, would read config to get log file path
	return "/tmp/dsm.log", nil
}

func tailLogFile(logFile string, lines int) error {
	content, err := os.ReadFile(logFile)
	if err != nil {
		return err
	}

	logLines := strings.Split(string(content), "\n")

	// Get last N lines
	if len(logLines) > lines {
		logLines = logLines[len(logLines)-lines:]
	}

	for _, line := range logLines {
		if strings.TrimSpace(line) != "" {
			fmt.Println(line)
		}
	}

	return nil
}

func parseLogLines(content string, lines int) []string {
	allLines := strings.Split(content, "\n")

	// Filter out empty lines
	var nonEmptyLines []string
	for _, line := range allLines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	// Get last N lines
	if len(nonEmptyLines) > lines {
		start := len(nonEmptyLines) - lines
		return nonEmptyLines[start:]
	}

	return nonEmptyLines
}

func validateLogFile(logFile string) error {
	if logFile == "" {
		return fmt.Errorf("log file path cannot be empty")
	}

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return fmt.Errorf("log file does not exist: %s", logFile)
	}

	fileInfo, err := os.Stat(logFile)
	if err != nil {
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("log file path is a directory: %s", logFile)
	}

	return nil
}


func createConfigBackup(configPath string) (string, error) {
	backupPath := configPath + ".backup"
	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	return backupPath, os.WriteFile(backupPath, content, defaultFilePerms)
}

func parseConfigKey(key string) ([]string, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	return strings.Split(key, "."), nil
}

func formatConfigValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		return fmt.Sprintf("%t", v)
	case int, int32, int64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%f", v)
	default:
		// For complex types like maps, use JSON
		if jsonBytes, err := json.Marshal(value); err == nil {
			return string(jsonBytes)
		}
		return fmt.Sprintf("%v", v)
	}
}

// Helper functions for ignore tests
func getIgnoreFilePath(repoPath string) (string, error) {
	if repoPath == "" {
		return "", fmt.Errorf("repo path cannot be empty")
	}

	return filepath.Join(repoPath, ".syncignore"), nil
}

func createIgnoreFile(ignorePath, content string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(ignorePath)
	if err := os.MkdirAll(dir, testDirPerms); err != nil {
		return err
	}

	return os.WriteFile(ignorePath, []byte(content), defaultFilePerms)
}

func validateIgnoreContent(content string) error {
	// Basic validation - check for dangerous patterns
	dangerousPatterns := []string{
		"rm -rf",
		"sudo rm",
		"$(sudo",
		"`sudo",
		"$(",
		"`",
	}

	contentLower := strings.ToLower(content)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(contentLower, pattern) {
			return fmt.Errorf("dangerous pattern detected in ignore content")
		}
	}

	return nil
}

func editIgnoreFile(ignorePath, editor string) error {
	// Simplified implementation - would actually launch editor in real code
	return validateIgnoreFile(ignorePath)
}

func validateIgnoreFile(ignorePath string) error {
	content, err := os.ReadFile(ignorePath)
	if err != nil {
		return err
	}

	return validateIgnoreContent(string(content))
}

func createIgnoreFileBackup(ignorePath string) (string, error) {
	backupPath := ignorePath + ".backup"
	content, err := os.ReadFile(ignorePath)
	if err != nil {
		return "", err
	}

	return backupPath, os.WriteFile(backupPath, content, defaultFilePerms)
}

func getDefaultIgnoreContent() string {
	return defaultIgnoreContent
}

func parseIgnorePatterns(content string) []string {
	lines := strings.Split(content, "\n")
	var patterns []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			patterns = append(patterns, trimmed)
		}
	}

	return patterns
}