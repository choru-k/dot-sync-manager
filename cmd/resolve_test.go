package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

// testResolveTimestampFormat matches the format used by gitmanager for conflict directories.
const testResolveTimestampFormat = "20060102T150405Z0700"

func setupResolveTestEnv(t *testing.T) (repoDir, targetDir string, cleanup func()) {
	t.Helper()

	// Reset flags to default values before each test
	resolveUseLocal = false
	resolveUseRemote = false
	resolveAll = false

	repoDir = t.TempDir()
	targetDir = t.TempDir()
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts")
	if err := os.MkdirAll(conflictDir, 0755); err != nil {
		t.Fatalf("failed to create conflict dir: %v", err)
	}

	// Create config with mapping
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.Mappings = map[string]string{
		".bashrc": filepath.Join(targetDir, ".bashrc"),
		".vimrc":  filepath.Join(targetDir, ".vimrc"),
	}
	configPath := filepath.Join(repoDir, "config.json")
	cfg.ConfigPath = configPath

	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	setConfigFile(configPath)

	cleanup = func() {
		setConfigFile("")
		resolveUseLocal = false
		resolveUseRemote = false
		resolveAll = false
	}

	return repoDir, targetDir, cleanup
}

func createResolveConflictArtifact(t *testing.T, repoDir, filename string) {
	t.Helper()

	timestamp := time.Now().Format(testResolveTimestampFormat)
	timestampDir := filepath.Join(repoDir, ".dsm", "conflicts", timestamp)
	if err := os.MkdirAll(timestampDir, 0755); err != nil {
		t.Fatalf("failed to create timestamp dir: %v", err)
	}

	// Create .local file
	localPath := filepath.Join(timestampDir, filename+".local")
	if err := os.WriteFile(localPath, []byte("local content for "+filename), 0644); err != nil {
		t.Fatalf("failed to create local file: %v", err)
	}

	// Create .remote file
	remotePath := filepath.Join(timestampDir, filename+".remote")
	if err := os.WriteFile(remotePath, []byte("remote content for "+filename), 0644); err != nil {
		t.Fatalf("failed to create remote file: %v", err)
	}
}

func TestResolveCmd_UseLocal(t *testing.T) {
	repoDir, targetDir, cleanup := setupResolveTestEnv(t)
	defer cleanup()

	// Create conflict artifact
	createResolveConflictArtifact(t, repoDir, ".bashrc")

	// Execute command with --use-local flag
	rootCmd.SetArgs([]string{"resolve", "--use-local"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("resolve --use-local failed: %v", err)
	}

	output := stdout.String()

	// Verify output confirms resolution
	if !strings.Contains(output, "local versions") {
		t.Errorf("should confirm local resolution, got: %s", output)
	}

	// Verify target file has local content
	targetPath := filepath.Join(targetDir, ".bashrc")
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read target file: %v", err)
	}

	if !strings.Contains(string(content), "local content") {
		t.Errorf("target should have local content, got: %s", content)
	}
}

func TestResolveCmd_UseRemote(t *testing.T) {
	repoDir, targetDir, cleanup := setupResolveTestEnv(t)
	defer cleanup()

	// Create conflict artifact
	createResolveConflictArtifact(t, repoDir, ".vimrc")

	// Execute command with --use-remote flag
	rootCmd.SetArgs([]string{"resolve", "--use-remote"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("resolve --use-remote failed: %v", err)
	}

	output := stdout.String()

	// Verify output confirms resolution
	if !strings.Contains(output, "remote versions") {
		t.Errorf("should confirm remote resolution, got: %s", output)
	}

	// Verify target file has remote content
	targetPath := filepath.Join(targetDir, ".vimrc")
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read target file: %v", err)
	}

	if !strings.Contains(string(content), "remote content") {
		t.Errorf("target should have remote content, got: %s", content)
	}
}

func TestResolveCmd_All(t *testing.T) {
	repoDir, _, cleanup := setupResolveTestEnv(t)
	defer cleanup()

	// Create conflict artifact
	createResolveConflictArtifact(t, repoDir, ".bashrc")

	// Execute command with --all flag
	rootCmd.SetArgs([]string{"resolve", "--all"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("resolve --all failed: %v", err)
	}

	output := stdout.String()

	// Verify output confirms all resolved
	if !strings.Contains(output, "Marked all conflicts as resolved") {
		t.Errorf("should confirm all resolved, got: %s", output)
	}

	// Verify conflict directory is empty or timestamp dir removed
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts")
	entries, err := os.ReadDir(conflictDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to read conflict dir: %v", err)
	}

	// Either no entries or the timestamp dir should be gone
	if len(entries) > 0 {
		t.Errorf("conflict directory should be empty after --all, found %d entries", len(entries))
	}
}

func TestResolveCmd_MutuallyExclusiveFlags(t *testing.T) {
	repoDir, _, cleanup := setupResolveTestEnv(t)
	defer cleanup()

	// Create conflict artifact to ensure there are conflicts to resolve
	createResolveConflictArtifact(t, repoDir, ".bashrc")

	// Execute command with mutually exclusive flags
	rootCmd.SetArgs([]string{"resolve", "--use-local", "--use-remote"})
	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)

	err := rootCmd.Execute()
	if err == nil {
		t.Error("should error with mutually exclusive flags")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutually exclusive, got: %v", err)
	}
}

func TestResolveCmd_NoConflicts(t *testing.T) {
	_, _, cleanup := setupResolveTestEnv(t)
	defer cleanup()

	// Execute command without any conflicts
	rootCmd.SetArgs([]string{"resolve"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	output := stdout.String()

	// Should show no conflicts message
	if !strings.Contains(output, "No conflicts to resolve") {
		t.Errorf("should show 'No conflicts to resolve', got: %s", output)
	}
}

func TestResolveCmd_SpecificFile(t *testing.T) {
	repoDir, _, cleanup := setupResolveTestEnv(t)
	defer cleanup()

	// Create conflict artifact
	createResolveConflictArtifact(t, repoDir, ".bashrc")

	// Execute command with specific file (mark as resolved)
	rootCmd.SetArgs([]string{"resolve", ".bashrc"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("resolve .bashrc failed: %v", err)
	}

	output := stdout.String()

	// Verify output confirms specific file resolved
	if !strings.Contains(output, "Marked .bashrc as resolved") {
		t.Errorf("should confirm specific file resolved, got: %s", output)
	}
}

func TestResolveCmd_SpecificFileWithLocalFlag(t *testing.T) {
	repoDir, targetDir, cleanup := setupResolveTestEnv(t)
	defer cleanup()

	// Create conflict artifact
	createResolveConflictArtifact(t, repoDir, ".bashrc")

	// Execute command with specific file and --use-local flag
	rootCmd.SetArgs([]string{"resolve", ".bashrc", "--use-local"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("resolve .bashrc --use-local failed: %v", err)
	}

	output := stdout.String()

	// Verify output confirms resolution
	if !strings.Contains(output, "Resolved .bashrc with local version") {
		t.Errorf("should confirm local resolution for specific file, got: %s", output)
	}

	// Verify target file has local content
	targetPath := filepath.Join(targetDir, ".bashrc")
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read target file: %v", err)
	}

	if !strings.Contains(string(content), "local content") {
		t.Errorf("target should have local content, got: %s", content)
	}
}

func TestResolveCmd_DefaultShowsConflicts(t *testing.T) {
	repoDir, _, cleanup := setupResolveTestEnv(t)
	defer cleanup()

	// Create conflict artifact
	createResolveConflictArtifact(t, repoDir, ".bashrc")

	// Execute command without flags or args
	rootCmd.SetArgs([]string{"resolve"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	output := stdout.String()

	// Should list conflicts and show hint
	if !strings.Contains(output, ".bashrc") {
		t.Error("should list conflict file")
	}

	if !strings.Contains(output, "--use-local") || !strings.Contains(output, "--use-remote") {
		t.Error("should show flag hints")
	}
}
