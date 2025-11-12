package scenarios

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCrossPlatformCompatibility orchestrates focused cross-platform tests
func TestCrossPlatformCompatibility(t *testing.T) {
	testID := RequireTestID(t)

	t.Logf("Running cross-platform compatibility test with ID: %s", testID)
	t.Logf("Running on platform: %s, architecture: %s", runtime.GOOS, runtime.GOARCH)

	// Create isolated test environment for orchestration
	sourceDir, targetDir := CreateTestEnvironment(t, testID+"_crossplatform")

	// Initialize DSM with dynamic config for cross-platform tests
	crossPlatformConfigPath := writeConfigFromTemplate(t, "cross_platform", map[string]interface{}{
		"SourceDir": sourceDir,
		"TargetDir": targetDir,
	})

	// Use shared helper to initialize git and verify DSM
	setupDSMWithGit(t, crossPlatformConfigPath)

	// Verify that cross-platform functionality works as expected
	// The actual testing happens in the focused test functions
	t.Log("✅ Cross-platform compatibility test setup completed")

	// Clean up test environment
	_ = os.RemoveAll(sourceDir) // Ignore cleanup errors in tests
	_ = os.RemoveAll(targetDir) // Ignore cleanup errors in tests
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isValidFilename checks if a filename is valid for the current platform
func isValidFilename(filename string) bool {
	// Check for invalid characters
	invalidChars := "<>:\"|?*"
	for _, char := range invalidChars {
		if strings.Contains(filename, string(char)) {
			return false
		}
	}

	// Check for reserved names (Windows-specific)
	reservedNames := []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}

	base := strings.ToUpper(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	for _, reserved := range reservedNames {
		if base == reserved {
			return false
		}
	}

	return true
}
