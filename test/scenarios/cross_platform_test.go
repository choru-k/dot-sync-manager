package scenarios

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCrossPlatformCompatibility tests DSM across different platforms and path handling
func TestCrossPlatformCompatibility(t *testing.T) {
	testID := RequireTestID(t)

	t.Logf("Running cross-platform compatibility test with ID: %s", testID)
	t.Logf("Running on platform: %s, architecture: %s", runtime.GOOS, runtime.GOARCH)

	// Setup test environment with dynamic paths
	sourceDir, targetDir := CreateTestEnvironment(t, testID+"_crossplatform")

	// Initialize DSM with dynamic config
	crossPlatformConfigPath := setupDSMForCrossPlatformTest(t, sourceDir, targetDir)

	t.Run("TestPathHandling", func(t *testing.T) {
		// Test different path formats and separators
		testCases := []struct {
			filename    string
			description string
		}{
			{".simpleconfig", "Simple dotfile"},
			{"config.json", "JSON config file"},
			{"nested/deep/config.conf", "Nested path"},
			{"file-with-dashes", "Filename with dashes"},
			{"file_with_underscores", "Filename with underscores"},
			{"file.with.dots", "Filename with multiple dots"},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				// Create source directory if nested
				sourceFile := filepath.Join(sourceDir, tc.filename)
				sourceDirOfFile := filepath.Dir(sourceFile)

				err := os.MkdirAll(sourceDirOfFile, dirPermissions)
				require.NoError(t, err)

				// Create test file
				content := `# Cross-platform test file
DESCRIPTION=%s
PLATFORM=%s
ARCH=%s
TIMESTAMP=$(date +%s)
PATH_SEPARATOR=%s
`
				separator := string(filepath.Separator)
				formattedContent := fmt.Sprintf(content, tc.description, runtime.GOOS, runtime.GOARCH, separator)

				writeErr := os.WriteFile(sourceFile, []byte(formattedContent), filePermissions)
				require.NoError(t, writeErr)

				// Add file to DSM
				addFileToDSMWithConfig(t, crossPlatformConfigPath, sourceFile)

				// Sync the file
				syncChangesWithConfig(t, crossPlatformConfigPath)

				// Verify file exists in target with correct path
				// DSM normalizes filenames by stripping leading '.' from files in home directory
				// With our fix, DSM now preserves nested path structure instead of flattening to basename
				expectedTargetName := strings.TrimPrefix(tc.filename, ".") // Remove leading dot but keep path structure
				targetFile := filepath.Join(targetDir, expectedTargetName)
				assert.FileExists(t, targetFile, "File should exist in target with path: %s", expectedTargetName)

				// Verify content
				targetContent, err := os.ReadFile(targetFile)
				require.NoError(t, err, "Should be able to read target file: %s", expectedTargetName)

				contentStr := string(targetContent)
				assert.Contains(t, contentStr, tc.description, "Content should match description")
				assert.Contains(t, contentStr, runtime.GOOS, "Content should contain platform")
				assert.Contains(t, contentStr, separator, "Content should contain path separator")

				t.Logf("✅ Path handling test passed for: %s", tc.filename)
			})
		}
	})

	t.Run("TestSymlinkHandling", func(t *testing.T) {
		// Create a source file
		sourceFile := filepath.Join(sourceDir, ".symlink-source")
		sourceContent := `# Symlink source file
TYPE=source
TIMESTAMP=$(date +%s)
PLATFORM=%s
`
		formattedContent := fmt.Sprintf(sourceContent, runtime.GOOS)

		err := os.WriteFile(sourceFile, []byte(formattedContent), filePermissions)
		require.NoError(t, err)

		// Create a symlink
		symlinkPath := filepath.Join(sourceDir, ".symlink-target")

		// Note: Symlink creation might not work in all container environments
		// We'll test both symlink creation and fallback behavior
		err = os.Symlink(sourceFile, symlinkPath)
		if err != nil {
			t.Logf("Symlink creation failed (expected in some environments): %v", err)
			t.Skip("Symlink creation not supported in this environment")
		}

		// Add symlink to DSM
		addFileToDSMWithConfig(t, crossPlatformConfigPath, ".symlink-target")

		// Sync changes
		syncChangesWithConfig(t, crossPlatformConfigPath)

		// Verify behavior (either symlink is preserved or resolved to target)
		targetSymlink := filepath.Join(targetDir, ".symlink-target")
		targetSource := filepath.Join(targetDir, ".symlink-source")

		// At least one of these should exist depending on symlink handling
		hasSymlink := fileExists(targetSymlink)
		hasSource := fileExists(targetSource)

		assert.True(t, hasSymlink || hasSource, "Either symlink or source should exist in target")

		if hasSymlink {
			t.Logf("✅ Symlink preserved in target")
		} else if hasSource {
			t.Logf("✅ Symlink resolved to source in target")
		}
	})

	t.Run("TestSpecialCharacters", func(t *testing.T) {
		// Test files with special characters in names
		specialCases := []struct {
			filename    string
			description string
		}{
			{"file with spaces.txt", "Filename with spaces"},
			{"file'with'quotes.txt", "Filename with quotes"},
			{"file#hash.txt", "Filename with hash"},
			{"file@at.txt", "Filename with at symbol"},
		}

		for _, tc := range specialCases {
			t.Run(tc.description, func(t *testing.T) {
				sourceFile := filepath.Join(sourceDir, tc.filename)

				// Skip if filename contains unsupported characters for current platform
				if !isValidFilename(tc.filename) {
					t.Skipf("Filename %s not supported on platform %s", tc.filename, runtime.GOOS)
				}

				content := `# Special character test
FILENAME=%s
DESCRIPTION=%s
PLATFORM=%s
`
				formattedContent := fmt.Sprintf(content, tc.filename, tc.description, runtime.GOOS)

				err := os.WriteFile(sourceFile, []byte(formattedContent), filePermissions)
				require.NoError(t, err)

				// Add file to DSM
				addFileToDSMWithConfig(t, crossPlatformConfigPath, tc.filename)

				// Sync changes
				syncChangesWithConfig(t, crossPlatformConfigPath)

				// Verify file exists
				targetFile := filepath.Join(targetDir, tc.filename)
				assert.FileExists(t, targetFile, "Special character file should exist: %s", tc.filename)

				t.Logf("✅ Special character test passed for: %s", tc.filename)
			})
		}
	})

	t.Run("TestLargeFilePath", func(t *testing.T) {
		// Test very long file paths
		longDirName := strings.Repeat("very_long_directory_name_", 5)
		longFileName := strings.Repeat("very_long_file_name_", 3) + ".conf"

		sourceDirPath := filepath.Join(sourceDir, longDirName)
		err := os.MkdirAll(sourceDirPath, dirPermissions)
		require.NoError(t, err)

		sourceFile := filepath.Join(sourceDirPath, longFileName)

		// Skip if path would be too long for current platform
		if len(sourceFile) > 255 && runtime.GOOS == "windows" {
			t.Skip("Path too long for Windows")
		}

		content := `# Long path test
PATH_LENGTH=%d
DIRECTORY_NAME=%s
FILE_NAME=%s
PLATFORM=%s
`
		formattedContent := fmt.Sprintf(content, len(sourceFile), longDirName, longFileName, runtime.GOOS)

		err = os.WriteFile(sourceFile, []byte(formattedContent), 0644)
		require.NoError(t, err)

		// Add to DSM
		relativeFilePath := filepath.Join(longDirName, longFileName)
		addFileToDSMWithConfig(t, crossPlatformConfigPath, relativeFilePath)

		// Sync changes
		syncChangesWithConfig(t, crossPlatformConfigPath)

		// Verify file exists
		targetFile := filepath.Join(targetDir, longDirName, longFileName)
		assert.FileExists(t, targetFile, "Long path file should exist")

		t.Logf("✅ Long path test passed (path length: %d)", len(sourceFile))
	})

	t.Run("TestPlatformSpecificPaths", func(t *testing.T) {
		// Test platform-specific path patterns
		platformPaths := getPlatformSpecificPaths()

		for _, pathInfo := range platformPaths {
			t.Run(pathInfo.description, func(t *testing.T) {
				// Skip if not applicable to current platform
				if pathInfo.platform != "" && pathInfo.platform != runtime.GOOS {
					t.Skipf("Path %s not applicable to platform %s", pathInfo.path, runtime.GOOS)
				}

				sourceFile := filepath.Join(sourceDir, pathInfo.path)
				sourceDirOfFile := filepath.Dir(sourceFile)

				err := os.MkdirAll(sourceDirOfFile, dirPermissions)
				require.NoError(t, err)

				content := `# Platform-specific path test
PATH=%s
PLATFORM=%s
DESCRIPTION=%s
`
				formattedContent := fmt.Sprintf(content, pathInfo.path, runtime.GOOS, pathInfo.description)

				err = os.WriteFile(sourceFile, []byte(formattedContent), 0644)
				require.NoError(t, err)

				// Add to DSM
				addFileToDSMWithConfig(t, crossPlatformConfigPath, pathInfo.path)

				// Sync changes
				syncChangesWithConfig(t, crossPlatformConfigPath)

				// Verify file exists
				targetFile := filepath.Join(targetDir, pathInfo.path)
				assert.FileExists(t, targetFile, "Platform-specific file should exist: %s", pathInfo.path)

				t.Logf("✅ Platform-specific path test passed: %s", pathInfo.path)
			})
		}
	})

	t.Log("✅ Cross-platform compatibility test completed successfully")
}

// setupDSMForCrossPlatformTest initializes DSM for cross-platform tests
func setupDSMForCrossPlatformTest(t *testing.T, sourceDir, targetDir string) string {
	configPath := writeConfigFromTemplate(t, "cross_platform", map[string]interface{}{
		"SourceDir": sourceDir,
		"TargetDir": targetDir,
	})

	// Initialize git repository in target directory before DSM init (like basic_sync test)
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	// Initialize bare git repository
	cmd := execCommandContext(ctx, "git", "init", targetDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Git init output: %s", string(output))
		t.Logf("Git init error: %v", err)
	}
	require.NoError(t, err, "Git repository initialization should succeed")

	// Configure git user
	cmd = execCommandContext(ctx, "git", "-C", targetDir, "config", "user.name", "Test User")
	err = cmd.Run()
	require.NoError(t, err, "Git user config should succeed")

	cmd = execCommandContext(ctx, "git", "-C", targetDir, "config", "user.email", "test@example.com")
	err = cmd.Run()
	require.NoError(t, err, "Git email config should succeed")

	// Verify DSM can read configuration correctly (this confirms DSM is properly initialized)
	cmd = execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "status")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Logf("DSM status output: %s", string(output))
		t.Logf("DSM status error: %v", err)
	}
	require.NoError(t, err, "DSM should be able to read configuration")
	assert.Contains(t, string(output), "Repository: "+targetDir)

	t.Log("✅ DSM initialization verified through successful status command")
	return configPath
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

// platformPathInfo represents a platform-specific path for testing
type platformPathInfo struct {
	path        string
	description string
	platform    string // empty string means all platforms
}

// getPlatformSpecificPaths returns platform-specific paths to test
func getPlatformSpecificPaths() []platformPathInfo {
	return []platformPathInfo{
		{
			path:        ".config/application/config.json",
			description: "Nested config directory",
		},
		{
			path:        ".local/share/application/data.txt",
			description: "Local share directory",
		},
		{
			path:        ".cache/app/cache.tmp",
			description: "Cache directory",
		},
		{
			path:        "Library/Preferences/app.plist",
			description: "macOS preferences",
			platform:    "darwin",
		},
		{
			path:        "AppData/Local/app/config.ini",
			description: "Windows AppData",
			platform:    "windows",
		},
		{
			path:        ".config/wayland-app/config",
			description: "Wayland config",
			platform:    "linux",
		},
	}
}
