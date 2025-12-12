package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

func TestLinksCmd_List(t *testing.T) {
	// Setup
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create source file in repo
	sourceFile := filepath.Join(repoDir, ".bashrc")
	if err := os.WriteFile(sourceFile, []byte("# bash config"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Create target as valid symlink
	validTarget := filepath.Join(targetDir, ".bashrc")
	if err := os.Symlink(sourceFile, validTarget); err != nil {
		t.Fatal(err)
	}

	// Create another target as regular file (not symlink)
	sourceFile2 := filepath.Join(repoDir, ".vimrc")
	if err := os.WriteFile(sourceFile2, []byte("\" vim config"), testFilePerms); err != nil {
		t.Fatal(err)
	}
	invalidTarget := filepath.Join(targetDir, ".vimrc")
	if err := os.WriteFile(invalidTarget, []byte("\" regular file"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Create config with mappings
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.Mappings = map[string]string{
		".bashrc": validTarget,
		".vimrc":  invalidTarget,
	}
	configPath := filepath.Join(repoDir, "config.json")
	cfg.ConfigPath = configPath

	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatal(err)
	}

	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	// Execute command
	rootCmd.SetArgs([]string{"links"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("links command failed: %v", err)
	}

	// Verify output
	output := stdout.String()

	// Should contain both mappings
	if !strings.Contains(output, ".bashrc") {
		t.Error("Output should contain .bashrc mapping")
	}
	if !strings.Contains(output, ".vimrc") {
		t.Error("Output should contain .vimrc mapping")
	}

	// Should contain target paths
	if !strings.Contains(output, validTarget) {
		t.Error("Output should contain valid target path")
	}
	if !strings.Contains(output, invalidTarget) {
		t.Error("Output should contain invalid target path")
	}

	// Should contain status indicators (emojis or status text)
	// Note: May contain "valid", "not_symlink", or emoji equivalents
	if !strings.Contains(output, "valid") && !strings.Contains(output, "✅") {
		t.Error("Output should contain 'valid' status or ✅ emoji")
	}
}

func TestLinksCmd_Verify_AllValid(t *testing.T) {
	// Setup
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create source file and valid symlink
	sourceFile := filepath.Join(repoDir, ".bashrc")
	if err := os.WriteFile(sourceFile, []byte("# bash config"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	validTarget := filepath.Join(targetDir, ".bashrc")
	if err := os.Symlink(sourceFile, validTarget); err != nil {
		t.Fatal(err)
	}

	// Create config with valid mapping
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.Mappings = map[string]string{
		".bashrc": validTarget,
	}
	configPath := filepath.Join(repoDir, "config.json")
	cfg.ConfigPath = configPath

	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatal(err)
	}

	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	// Execute verify command
	rootCmd.SetArgs([]string{"links", "verify"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("links verify should succeed for all valid mappings, got: %v", err)
	}

	// Verify output contains success message
	output := stdout.String()
	if !strings.Contains(output, "All symlink mappings are valid") {
		t.Error("Output should contain success message for all valid")
	}
	if !strings.Contains(output, "Valid:       1") {
		t.Error("Output should show 1 valid mapping")
	}
}

func TestLinksCmd_Verify_BrokenLinks(t *testing.T) {
	// Setup
	repoDir := t.TempDir()
	targetDir := t.TempDir()

	// Create broken symlink (target points to non-existent file)
	sourceFile := filepath.Join(repoDir, ".bashrc")
	brokenTarget := filepath.Join(targetDir, ".bashrc")
	if err := os.Symlink(sourceFile, brokenTarget); err != nil {
		t.Fatal(err)
	}
	// Note: sourceFile was never created, so symlink is broken

	// Create config with broken mapping
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.Mappings = map[string]string{
		".bashrc": brokenTarget,
	}
	configPath := filepath.Join(repoDir, "config.json")
	cfg.ConfigPath = configPath

	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatal(err)
	}

	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	// Execute verify command
	rootCmd.SetArgs([]string{"links", "verify"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	err = rootCmd.Execute()
	if err == nil {
		t.Fatal("links verify should return error for broken mappings")
	}

	// Verify error message
	if !strings.Contains(err.Error(), "broken") {
		t.Errorf("Error should mention 'broken', got: %v", err)
	}

	// Verify output contains summary
	output := stdout.String()
	if !strings.Contains(output, "Broken:") {
		t.Error("Output should contain broken count in summary")
	}
}
