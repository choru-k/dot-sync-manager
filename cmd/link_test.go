package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

func TestLinkCmd_Success(t *testing.T) {
	// Setup
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create source file in repo
	sourceFile := filepath.Join(repoDir, ".bashrc")
	if err := os.WriteFile(sourceFile, []byte("# bash config"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Create config
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.Mappings = make(map[string]string)
	configPath := filepath.Join(repoDir, "config.json")
	cfg.ConfigPath = configPath

	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatal(err)
	}

	// Set config location for test
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	// Execute command
	targetPath := filepath.Join(targetDir, ".bashrc")
	rootCmd.SetArgs([]string{"link", ".bashrc", targetPath})

	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Verify symlink created
	info, err := os.Lstat(targetPath)
	if err != nil {
		t.Fatal("Target should exist")
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Target should be a symlink")
	}

	// Verify output
	if !bytes.Contains(stdout.Bytes(), []byte("Created symlink")) {
		t.Error("Should show success message")
	}
}

func TestLinkCmd_ForceOverwrite(t *testing.T) {
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create source file
	sourceFile := filepath.Join(repoDir, ".bashrc")
	if err := os.WriteFile(sourceFile, []byte("# bash config"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Create existing file at target
	targetPath := filepath.Join(targetDir, ".bashrc")
	if err := os.WriteFile(targetPath, []byte("# existing"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Setup config
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.Mappings = make(map[string]string)
	configPath := filepath.Join(repoDir, "config.json")
	cfg.ConfigPath = configPath

	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatal(err)
	}
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	// Without --force should fail
	rootCmd.SetArgs([]string{"link", ".bashrc", targetPath})
	if err := rootCmd.Execute(); err == nil {
		t.Error("Should fail without --force when target exists")
	}

	// With --force should succeed
	rootCmd.SetArgs([]string{"link", ".bashrc", targetPath, "--force"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("link --force failed: %v", err)
	}

	// Verify it's now a symlink
	info, err := os.Lstat(targetPath)
	if err != nil {
		t.Fatal("Target should exist after --force")
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Target should be a symlink after --force")
	}
}

func TestLinkCmd_MissingConfig(t *testing.T) {
	// This test verifies graceful error handling when config doesn't exist
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get home directory: %v", err)
	}

	targetPath := filepath.Join(home, ".test-link-missing-config")
	// Clean up if exists from previous test
	_ = os.Remove(targetPath)

	// This will fail because config doesn't exist, but validates error handling
	rootCmd.SetArgs([]string{"link", ".bashrc", targetPath})
	err = rootCmd.Execute()
	if err == nil {
		t.Fatal("expected an error due to missing config, but got nil")
	}
	if !strings.Contains(err.Error(), "configuration file not found") {
		t.Errorf("expected config not found error, but got: %v", err)
	}

	// Cleanup
	_ = os.Remove(targetPath)
}

func TestUnlinkCmd_Success(t *testing.T) {
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create source file
	sourceFile := filepath.Join(repoDir, ".bashrc")
	if err := os.WriteFile(sourceFile, []byte("# bash"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Create symlink
	targetPath := filepath.Join(targetDir, ".bashrc")
	if err := os.Symlink(sourceFile, targetPath); err != nil {
		t.Fatal(err)
	}

	// Setup config with mapping
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.Mappings = map[string]string{".bashrc": targetPath}
	configPath := filepath.Join(repoDir, "config.json")
	cfg.ConfigPath = configPath

	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatal(err)
	}
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	// Execute unlink
	rootCmd.SetArgs([]string{"unlink", targetPath})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unlink command failed: %v", err)
	}

	// Verify symlink removed
	if _, err := os.Lstat(targetPath); !os.IsNotExist(err) {
		t.Error("Symlink should be removed")
	}

	// Verify output
	if !bytes.Contains(stdout.Bytes(), []byte("Removed symlink")) {
		t.Error("Should show success message")
	}
}

func TestUnlinkCmd_RestoreBackup(t *testing.T) {
	// TODO(A4R2-06): Backup restore test requires integration test with real backup directory setup
	// Should verify: backup discovery, full path matching, basename fallback, restore success
	t.Skip("Backup restore test requires integration test with real backup directory setup")
}

func TestUnlinkCmd_NotSymlink(t *testing.T) {
	targetDir := t.TempDir()

	// Create regular file (not symlink)
	targetPath := filepath.Join(targetDir, ".bashrc")
	if err := os.WriteFile(targetPath, []byte("# regular file"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Setup minimal config
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Git.RepoPath = t.TempDir()
	cfg.Mappings = make(map[string]string)
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg.ConfigPath = configPath

	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatal(err)
	}
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	rootCmd.SetArgs([]string{"unlink", targetPath})
	err = rootCmd.Execute()

	if err == nil {
		t.Error("Should fail for non-symlink target")
	}
	if err != nil && !bytes.Contains([]byte(err.Error()), []byte("not a symlink")) {
		t.Errorf("Error should mention non-symlink, got: %v", err)
	}
}
