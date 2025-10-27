package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/util"
)

// Test constants for consistent testing across all CLI command tests
const (
	// testFilePerms allows owner read/write access with group/others read
	testFilePerms = 0644 // -rw-r--r--

	// testDirPerms grants owner full access with group/others read/execute
	testDirPerms = 0755 // rwxr-xr-x, standard permissions for directories allowing owner full access and others to traverse.

	// testRepoName is used for test repositories
	testRepoName = "test-dotfiles"

	// testMachineName is used for test configurations
	testMachineName = "test-machine"

	// testAuthorName and testAuthorEmail are used for git configurations
	testAuthorName  = "Test User"
	testAuthorEmail = "test@example.com"
)

// TestConfig represents a test configuration setup
type TestConfig struct {
	HomeDir    string
	RepoPath   string
	ConfigPath  string
	Config     *config.SyncConfig
}

// setupTestEnvironment creates a temporary test environment with proper configuration
func setupTestEnvironment(t *testing.T) *TestConfig {
	t.Helper()

	// Create temporary home directory
	homeDir := t.TempDir()
	if err := os.MkdirAll(homeDir, testDirPerms); err != nil {
		t.Fatalf("failed to create test home directory: %v", err)
	}

	// Set HOME environment variable for the test
	t.Setenv("HOME", homeDir)

	// Set test mode to prevent GUI editor launches during tests
	t.Setenv("DSM_TEST_MODE", "1")

	// Create repository directory
	repoPath := filepath.Join(homeDir, testRepoName)
	if err := os.MkdirAll(repoPath, testDirPerms); err != nil {
		t.Fatalf("failed to create test repo directory: %v", err)
	}

	// Create default configuration
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}

	// Configure test-specific values
	cfg.Machine.Name = testMachineName
	cfg.Git.RepoPath = repoPath
	cfg.Git.AuthorName = testAuthorName
	cfg.Git.AuthorEmail = testAuthorEmail
	cfg.Git.AuthType = "none" // Use "none" auth for local tests to avoid SSH key requirements
	cfg.Git.RemoteURL = ""     // No remote for local tests
	cfg.ConflictResolution.BackupDir = filepath.Join(repoPath, ".backup")
	cfg.Mappings = make(map[string]string)
	cfg.ConfigPath = filepath.Join(repoPath, ".sync-config.json")

	// Save configuration to file
	if err := cfg.SaveToFile(cfg.ConfigPath); err != nil {
		t.Fatalf("failed to save test config: %v", err)
	}

	return &TestConfig{
		HomeDir:    homeDir,
		RepoPath:   repoPath,
		ConfigPath:  cfg.ConfigPath,
		Config:     cfg,
	}
}

// createTestFile creates a test file with specified content
func createTestFile(t *testing.T, path string, content string) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, testDirPerms); err != nil {
		t.Fatalf("failed to create directory for test file: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), testFilePerms); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
}

// createTestSymlink creates a test symlink
func createTestSymlink(t *testing.T, target, link string) {
	t.Helper()

	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation not supported on Windows: %v", err)
		}
		t.Fatalf("failed to create test symlink: %v", err)
	}
}

// requireSymlinkSupport skips tests on platforms that don't support symlinks
func requireSymlinkSupport(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src")
	dst := filepath.Join(tempDir, "dst")

	// Create source file
	if err := os.WriteFile(src, []byte("test"), testFilePerms); err != nil {
		t.Fatalf("failed to create test source file: %v", err)
	}

	// Test symlink creation
	if err := os.Symlink(src, dst); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation not supported on Windows: %v", err)
		}
		t.Fatalf("symlink support required for this test: %v", err)
	}

	// Cleanup
	if err := os.Remove(dst); err != nil {
		t.Fatalf("failed to cleanup test symlink: %v", err)
	}
}


// assertFileExists checks if a file exists and fails the test if it doesn't
func assertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file to exist: %s", path)
	}
}

// assertFileNotExists checks if a file doesn't exist and fails the test if it does
func assertFileNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected file to not exist: %s", path)
	}
}


// cleanupTestEnvironment cleans up a test environment
func cleanupTestEnvironment(config *TestConfig) {
	// No explicit cleanup needed when using t.TempDir()
	// This function exists for compatibility with existing test code
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
func validateResolutionState(repoPath string) error {
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
	// First check if the repository path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return false, nil, fmt.Errorf("repository path does not exist: %s", repoPath)
	}

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

	text := string(content)

	// Check if file contains conflict markers
	hasConflictMarkers := strings.Contains(text, "<<<<<<< HEAD") ||
		strings.Contains(text, "=======") ||
		strings.Contains(text, ">>>>>>> ")

	if !hasConflictMarkers {
		return []string{}, nil
	}

	// Parse conflict sections
	lines := strings.Split(text, "\n")
	var sections []string
	var currentSection strings.Builder
	inConflict := false

	for _, line := range lines {
		if strings.HasPrefix(line, "<<<<<<< ") {
			// Start of conflict - begin new section
			inConflict = true
			currentSection.Reset()
		} else if line == "=======" && inConflict {
			// Separator - save current section and start new one
			sections = append(sections, strings.TrimSpace(currentSection.String()))
			currentSection.Reset()
		} else if strings.HasPrefix(line, ">>>>>>> ") && inConflict {
			// End of conflict - save final section
			sections = append(sections, strings.TrimSpace(currentSection.String()))
			inConflict = false
		} else if inConflict {
			// Add content to current section
			if currentSection.Len() > 0 {
				currentSection.WriteString("\n")
			}
			currentSection.WriteString(line)
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
	// Check if repository exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return false, fmt.Errorf("repository does not exist: %s", repoPath)
	}

	// Check if .git directory exists
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return false, fmt.Errorf("not a git repository: %s", repoPath)
	}

	// Check for common indicators of unmerged files in test environment
	// Look for conflict markers or unmerged file indicators
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return false, err
	}

	// Simple heuristic: check for files with conflict markers or common unmerged patterns
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip .git and hidden directories
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		// Check for common conflict file patterns
		if strings.Contains(entry.Name(), ".conflict") ||
		   strings.Contains(entry.Name(), ".merge") ||
		   strings.Contains(entry.Name(), ".rej") {
			return true, nil
		}
	}

	// In a real implementation, this would run `git status --porcelain` and parse
	// for unmerged files (marked with 'UU' in git status output)
	// For test purposes, we'll assume no unmerged files unless we find obvious indicators
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
	// Check if repository exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return false, fmt.Errorf("repository does not exist: %s", repoPath)
	}

	// Check if .git directory exists
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return false, fmt.Errorf("not a git repository: %s", repoPath)
	}

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

	// Check if target is a relative path (doesn't start with / or ~)
	if !strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "~") {
		return fmt.Errorf("target must be an absolute path")
	}

	expandedPath, err := util.ExpandPath(target)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}

	// For open command, we just validate the path format, not existence
	// The actual open command will handle non-existent paths appropriately
	_ = expandedPath
	return nil
}

func getEditorCommand(filePath string) string {
	return getDefaultEditorForFile(filePath)
}

// Helper functions for log tests
func getLogFile() (string, error) {
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
	// Basic validation - check for dangerous patterns that could indicate command injection
	// Only look for patterns that are clearly dangerous in gitignore context
	dangerousPatterns := []string{
		"rm -rf /",
		"sudo rm -rf",
		":(){ :|:& };:", // fork bomb
		"&& rm",
		"; rm",
		"| rm",
		"> /dev/null",
		"`", // backticks for command substitution
		"$(", // command substitution
	}

	contentLower := strings.ToLower(content)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(contentLower, pattern) {
			return fmt.Errorf("dangerous pattern detected in ignore content")
		}
	}

	return nil
}

func editIgnoreFile(ignorePath string) error {
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
	if content == "" {
		return []string{}
	}

	lines := strings.Split(content, "\n")
	var patterns []string

	for _, line := range lines {
		// Skip empty lines but keep whitespace-only lines and comments
		if line != "" {
			patterns = append(patterns, line)
		}
	}

	return patterns
}