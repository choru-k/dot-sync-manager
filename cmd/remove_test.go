package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestRemoveCmd_Sanity(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		flags       map[string]string
		expectError bool
	}{
		{
			name:        "no arguments",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "regular file (should error - remove only works with symlinks)",
			args:        []string{"/tmp/test.txt"},
			expectError: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"/tmp/test.txt", "/tmp/test2.txt"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "remove"}
			flags := cmd.Flags()

			// Set test flags
			if deleteAll, ok := tt.flags["delete-all"]; ok {
				if err := flags.Set("delete-all", deleteAll); err != nil {
					t.Fatalf("failed to set delete-all flag: %v", err)
				}
			}
			if keepRepoFile, ok := tt.flags["keep-repo"]; ok {
				if err := flags.Set("keep-repo", keepRepoFile); err != nil {
					t.Fatalf("failed to set keep-repo flag: %v", err)
				}
			}

			err := runRemove(cmd, tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("runRemove() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestValidateRemoveTarget(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "empty path",
			path:        "",
			expectError: true,
		},
		{
			name:        "relative path",
			path:        "relative/path.txt",
			expectError: true,
		},
		{
			name:        "absolute path",
			path:        "/tmp/test.txt",
			expectError: true, // File doesn't exist, should error
		},
		{
			name:        "home directory path",
			path:        "~/test.txt",
			expectError: true, // File doesn't exist, should error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateRemoveTarget(tt.path)
			if (err != nil) != tt.expectError {
				t.Errorf("validateRemoveTarget() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestRemoveCmd_Integration(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Create test files
	homeFile := filepath.Join(testConfig.HomeDir, ".bashrc")
	repoFile := filepath.Join(testConfig.RepoPath, "bashrc")
	linkFile := filepath.Join(testConfig.HomeDir, ".bashrc")

	createTestFile(t, homeFile, "# Bash config")
	createTestFile(t, repoFile, "# Bash config")

	// Create symlink
	if err := os.Symlink(repoFile, linkFile); err != nil {
		t.Skipf("symlink creation not supported: %v", err)
	}

	// Update config to track the file
	testConfig.Config.Mappings[".bashrc"] = "bashrc"
	if err := testConfig.Config.SaveToFile(testConfig.ConfigPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Test remove command
	cmd := &cobra.Command{Use: "remove"}
	err := runRemove(cmd, []string{homeFile})
	if err != nil {
		t.Fatalf("runRemove() failed: %v", err)
	}

	// Verify file is preserved in home directory
	assertFileExists(t, homeFile)
	assertFileNotExists(t, linkFile)
}

func TestRemoveCmd_KeepRepoFlag(t *testing.T) {
	requireSymlinkSupport(t)

	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Create test files
	homeFile := filepath.Join(testConfig.HomeDir, ".bashrc")
	repoFile := filepath.Join(testConfig.RepoPath, "bashrc")

	createTestFile(t, repoFile, "# Bash config")

	// Create symlink from home to repo (simulate dsm add)
	createTestSymlink(t, repoFile, homeFile)

	// Set the global configFile variable
	originalConfigFile := configFile
	configFile = testConfig.ConfigPath
	defer func() { configFile = originalConfigFile }()

	// Update config
	testConfig.Config.Mappings[".bashrc"] = "bashrc"
	if err := testConfig.Config.SaveToFile(testConfig.ConfigPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Test remove with keep-repo flag - use the actual removeCmd with pre-bound flags
	if err := removeCmd.Flags().Set("keep-repo", "true"); err != nil {
		t.Fatalf("failed to set keep-repo flag: %v", err)
	}

	err := runRemove(removeCmd, []string{homeFile})
	if err != nil {
		t.Fatalf("runRemove() failed: %v", err)
	}

	// Verify repo file is preserved
	assertFileExists(t, repoFile)
}

func TestRemoveCmd_DeleteAllFlag(t *testing.T) {
	requireSymlinkSupport(t)

	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Create test files
	homeFile := filepath.Join(testConfig.HomeDir, ".bashrc")
	repoFile := filepath.Join(testConfig.RepoPath, "bashrc")

	createTestFile(t, repoFile, "# Bash config")

	// Create symlink from home to repo (simulate dsm add)
	createTestSymlink(t, repoFile, homeFile)

	// Set the global configFile variable
	originalConfigFile := configFile
	configFile = testConfig.ConfigPath
	defer func() { configFile = originalConfigFile }()

	// Update config
	testConfig.Config.Mappings[".bashrc"] = "bashrc"
	if err := testConfig.Config.SaveToFile(testConfig.ConfigPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Test remove with delete-all flag - use the actual removeCmd with pre-bound flags
	if err := removeCmd.Flags().Set("delete-all", "true"); err != nil {
		t.Fatalf("failed to set delete-all flag: %v", err)
	}

	// Verify the flag was set
	if val, _ := removeCmd.Flags().GetBool("delete-all"); !val {
		t.Fatal("delete-all flag was not set correctly")
	}

	err := runRemove(removeCmd, []string{homeFile})
	if err != nil {
		t.Fatalf("runRemove() failed: %v", err)
	}

	// Verify both files are removed
	assertFileNotExists(t, homeFile)
	assertFileNotExists(t, repoFile)
}

func TestRemoveCmd_ConflictingFlags(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	homeFile := filepath.Join(testConfig.HomeDir, ".bashrc")
	createTestFile(t, homeFile, "# Bash config")

	// Test conflicting flags
	cmd := &cobra.Command{Use: "remove"}
	cmd.Flags().Bool("keep-repo", false, "Keep file in repository (default behavior)")
	cmd.Flags().Bool("delete-all", false, "Delete file from both home and repository")
	if err := cmd.Flags().Set("keep-repo", "true"); err != nil {
					t.Fatalf("failed to set keep-repo flag: %v", err)
				}
	if err := cmd.Flags().Set("delete-all", "true"); err != nil {
					t.Fatalf("failed to set delete-all flag: %v", err)
				}

	err := runRemove(cmd, []string{homeFile})
	if err == nil {
		t.Error("expected error for conflicting flags")
	}
}

func TestRemoveFromConfig(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Setup config with mappings
	testConfig.Config.Mappings[".bashrc"] = "bashrc"
	testConfig.Config.Mappings[".vimrc"] = "vimrc"
	if err := testConfig.Config.SaveToFile(testConfig.ConfigPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Remove from config
	removeFromConfig(".bashrc", testConfig.Config)

	// Verify removal
	if _, exists := testConfig.Config.Mappings[".bashrc"]; exists {
		t.Error("expected .bashrc to be removed from config")
	}
	if _, exists := testConfig.Config.Mappings[".vimrc"]; !exists {
		t.Error("expected .vimrc to still exist in config")
	}
}

func TestRemoveSymlink(t *testing.T) {
	requireSymlinkSupport(t)

	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Create test symlink
	target := filepath.Join(testConfig.RepoPath, "bashrc")
	link := filepath.Join(testConfig.HomeDir, ".bashrc")

	createTestFile(t, target, "# Bash config")
	createTestSymlink(t, target, link)

	// Remove symlink
	err := removeSymlink(link)
	if err != nil {
		t.Fatalf("removeSymlink() failed: %v", err)
	}

	// Verify symlink is removed
	assertFileNotExists(t, link)
	// Verify target is preserved
	assertFileExists(t, target)
}