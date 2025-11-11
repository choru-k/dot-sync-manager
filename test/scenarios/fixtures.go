package scenarios

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// CopySampleDotfiles copies sample dotfiles from fixtures to the specified destination directory
func CopySampleDotfiles(t *testing.T, destDir string) {
	t.Helper()

	// Initialize path resolution
	initPathResolution()

	// Source fixtures directory
	fixturesDir := filepath.Join(GetFixturesDir(), "sample_dotfiles")

	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, dirPermissions); err != nil {
		t.Fatalf("Failed to create destination directory %s: %v", destDir, err)
	}

	// List of sample files to copy
	sampleFiles := []string{
		".bashrc",
		".vimrc",
		".gitconfig",
	}

	for _, filename := range sampleFiles {
		srcPath := filepath.Join(fixturesDir, filename)
		dstPath := filepath.Join(destDir, filename)

		// Check if source file exists
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			// Create a minimal file if it doesn't exist
			content := fmt.Sprintf("# Auto-generated %s for DSM testing\nCreated at: test time\n", filename)
			if err := os.WriteFile(dstPath, []byte(content), filePermissions); err != nil {
				t.Fatalf("Failed to create %s: %v", filename, err)
			}
			t.Logf("Created minimal %s: %s", filename, dstPath)
			continue
		}

		// Read source file
		content, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("Failed to read sample file %s: %v", filename, err)
		}

		// Write destination file
		if err := os.WriteFile(dstPath, content, filePermissions); err != nil {
			t.Fatalf("Failed to copy sample file %s: %v", filename, err)
		}

		t.Logf("Copied sample file: %s -> %s", srcPath, dstPath)
	}
}

// EnsureRepoFixtures ensures all necessary repository fixtures exist in the specified repo directory
func EnsureRepoFixtures(t *testing.T, repoDir string) {
	t.Helper()

	// Initialize path resolution
	initPathResolution()

	// Ensure git repository is properly initialized
	if !isGitRepository(repoDir) {
		setupGitRepository(t, repoDir)
	}

	// Create additional fixture files if needed
	additionalFixtures := map[string]string{
		".DS_Store":                 "Binary file that should be ignored",
		"temporary.tmp":             "Temporary file that should be ignored",
		"no_sync.log":               "Log file that should be ignored",
		"README.md":                 "# Test Repository\n\nThis is a test repository for DSM E2E testing.",
		"test-data/sample.json":     `{"test": true, "mode": "e2e"}`,
		"config/testing.conf":       "# Test configuration file\ndebug=true\nmode=test",
		".config/app/settings.json": `{"app_name": "test", "version": "1.0.0"}`,
		".local/share/app/data.txt": "Local application data",
		".cache/app/cache.tmp":      "Application cache",
	}

	for relPath, content := range additionalFixtures {
		fullPath := filepath.Join(repoDir, relPath)

		// Create parent directories if needed
		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, dirPermissions); err != nil {
			t.Fatalf("Failed to create parent directory for %s: %v", relPath, err)
		}

		// Skip if file already exists
		if _, err := os.Stat(fullPath); err == nil {
			continue
		}

		// Write the file
		if err := os.WriteFile(fullPath, []byte(content), filePermissions); err != nil {
			t.Fatalf("Failed to create fixture file %s: %v", relPath, err)
		}

		t.Logf("Created fixture file: %s", relPath)
	}

	// Create .gitignore file if it doesn't exist
	gitignorePath := filepath.Join(repoDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		gitignoreContent := `# Files that should be ignored by DSM tests
.DS_Store
*.tmp
*.log
no_sync.*
.cache/
.local/share/
*.swp
*.swo
*~
`
		if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), filePermissions); err != nil {
			t.Fatalf("Failed to create .gitignore: %v", err)
		}
		t.Logf("Created .gitignore file")
	}

	// Create .syncignore file for DSM tests
	syncignorePath := filepath.Join(repoDir, ".syncignore")
	if _, err := os.Stat(syncignorePath); os.IsNotExist(err) {
		syncignoreContent := `# DSM-specific ignore patterns
*.tmp
*.log
.cache/
.DS_Store
no_sync.*
*.swp
*.swo
*~
*.backup
*.bak
`
		if err := os.WriteFile(syncignorePath, []byte(syncignoreContent), filePermissions); err != nil {
			t.Fatalf("Failed to create .syncignore: %v", err)
		}
		t.Logf("Created .syncignore file")
	}
}

// isGitRepository checks if a directory is a git repository
func isGitRepository(repoDir string) bool {
	gitDir := filepath.Join(repoDir, ".git")
	if stat, err := os.Stat(gitDir); err == nil {
		return stat.IsDir()
	}
	return false
}

// CreateTestEnvironment creates a complete test environment with source and target directories
func CreateTestEnvironment(t *testing.T, testID string) (sourceDir, targetDir string) {
	t.Helper()

	// Initialize path resolution
	initPathResolution()

	// Create test-specific directories
	testDataDir := GetTestDataDir()
	sourceDir = filepath.Join(testDataDir, "source_dotfiles_"+testID)
	targetDir = filepath.Join(testDataDir, "dotfiles-test_"+testID)

	// Clean up any existing test data
	os.RemoveAll(sourceDir)
	os.RemoveAll(targetDir)

	// Create directories
	if err := os.MkdirAll(sourceDir, dirPermissions); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}
	if err := os.MkdirAll(targetDir, dirPermissions); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	// Copy sample dotfiles
	CopySampleDotfiles(t, sourceDir)

	// Setup git repository in target directory
	setupGitRepository(t, targetDir)

	// Ensure repository fixtures
	EnsureRepoFixtures(t, targetDir)

	t.Logf("Created test environment: source=%s, target=%s", sourceDir, targetDir)
	return sourceDir, targetDir
}

// GetStandardTestDirectories returns the standard source and target directories for tests
func GetStandardTestDirectories() (sourceDir, targetDir string) {
	initPathResolution()
	sourceDir = filepath.Join(GetTestDataDir(), "source_dotfiles")
	targetDir = filepath.Join(GetTestDataDir(), "dotfiles-test")
	return sourceDir, targetDir
}

// CleanupTestEnvironment cleans up a test environment
func CleanupTestEnvironment(t *testing.T, sourceDir, targetDir string) {
	t.Helper()

	// Remove test directories
	if err := os.RemoveAll(sourceDir); err != nil {
		t.Logf("Warning: Failed to remove source directory %s: %v", sourceDir, err)
	}
	if err := os.RemoveAll(targetDir); err != nil {
		t.Logf("Warning: Failed to remove target directory %s: %v", targetDir, err)
	}

	t.Logf("Cleaned up test environment")
}
