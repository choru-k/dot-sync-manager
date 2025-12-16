package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/conflict"
)

// testConflictTimestampFormat matches the format used by gitmanager for conflict directories.
const testConflictTimestampFormat = "20060102T150405Z0700"

func setupConflictTestEnv(t *testing.T) (repoDir string, cleanup func()) {
	t.Helper()

	// Reset flags to default values before each test
	conflictsJSON = false

	repoDir = t.TempDir()
	conflictDir := filepath.Join(repoDir, ".dsm", "conflicts")
	if err := os.MkdirAll(conflictDir, 0755); err != nil {
		t.Fatalf("failed to create conflict dir: %v", err)
	}

	// Create config
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	cfg.Git.RepoPath = repoDir
	configPath := filepath.Join(repoDir, "config.json")
	cfg.ConfigPath = configPath

	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	setConfigFile(configPath)

	cleanup = func() {
		setConfigFile("")
		conflictsJSON = false
	}

	return repoDir, cleanup
}

func createConflictArtifact(t *testing.T, repoDir, filename string, hasBase bool) {
	t.Helper()

	timestamp := time.Now().Format(testConflictTimestampFormat)
	timestampDir := filepath.Join(repoDir, ".dsm", "conflicts", timestamp)
	if err := os.MkdirAll(timestampDir, 0755); err != nil {
		t.Fatalf("failed to create timestamp dir: %v", err)
	}

	// Create .local file
	localPath := filepath.Join(timestampDir, filename+".local")
	if err := os.WriteFile(localPath, []byte("local content"), 0644); err != nil {
		t.Fatalf("failed to create local file: %v", err)
	}

	// Create .remote file
	remotePath := filepath.Join(timestampDir, filename+".remote")
	if err := os.WriteFile(remotePath, []byte("remote content"), 0644); err != nil {
		t.Fatalf("failed to create remote file: %v", err)
	}

	// Optionally create .base file
	if hasBase {
		basePath := filepath.Join(timestampDir, filename+".base")
		if err := os.WriteFile(basePath, []byte("base content"), 0644); err != nil {
			t.Fatalf("failed to create base file: %v", err)
		}
	}
}

func TestConflictsCmd_JSONOutput(t *testing.T) {
	repoDir, cleanup := setupConflictTestEnv(t)
	defer cleanup()

	// Create a conflict artifact with base file
	createConflictArtifact(t, repoDir, ".bashrc", true)

	// Execute command with --json flag
	rootCmd.SetArgs([]string{"conflicts", "--json"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("conflicts --json failed: %v", err)
	}

	output := stdout.String()

	// Verify it's valid JSON
	var result struct {
		Count     int                     `json:"count"`
		Conflicts []conflict.ConflictInfo `json:"conflicts"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput was: %s", err, output)
	}

	if result.Count != 1 {
		t.Errorf("expected count 1, got %d", result.Count)
	}

	if len(result.Conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(result.Conflicts))
	}

	if result.Conflicts[0].File != ".bashrc" {
		t.Errorf("expected file .bashrc, got %s", result.Conflicts[0].File)
	}

	if !result.Conflicts[0].HasBase {
		t.Error("expected HasBase to be true")
	}
}

func TestConflictsCmd_TableOutput(t *testing.T) {
	repoDir, cleanup := setupConflictTestEnv(t)
	defer cleanup()

	// Create a conflict artifact without base file
	createConflictArtifact(t, repoDir, ".vimrc", false)

	// Execute command (default table output)
	rootCmd.SetArgs([]string{"conflicts"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("conflicts failed: %v", err)
	}

	output := stdout.String()

	// Should contain the filename
	if !strings.Contains(output, ".vimrc") {
		t.Error("output should contain .vimrc")
	}

	// Should contain header
	if !strings.Contains(output, "FILE") {
		t.Error("output should contain FILE header")
	}

	// Should show conflict count
	if !strings.Contains(output, "1 conflict") {
		t.Error("output should show '1 conflict'")
	}

	// Should show "No" for HasBase since we didn't create base file
	if !strings.Contains(output, "No") {
		t.Error("output should contain 'No' for HasBase")
	}

	// Should show resolution hint
	if !strings.Contains(output, "dsm resolve") {
		t.Error("output should contain 'dsm resolve' hint")
	}
}

func TestConflictsCmd_NoConflicts(t *testing.T) {
	_, cleanup := setupConflictTestEnv(t)
	defer cleanup()

	// Execute command without any conflicts
	rootCmd.SetArgs([]string{"conflicts"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("conflicts failed: %v", err)
	}

	output := stdout.String()

	// Should show no conflicts message
	if !strings.Contains(output, "No active conflicts") {
		t.Errorf("should show 'No active conflicts', got: %s", output)
	}
}

func TestConflictsCmd_JSONNoConflicts(t *testing.T) {
	_, cleanup := setupConflictTestEnv(t)
	defer cleanup()

	// Execute command with --json flag when no conflicts exist
	rootCmd.SetArgs([]string{"conflicts", "--json"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("conflicts --json failed: %v", err)
	}

	output := stdout.String()

	// Verify it's valid JSON with empty array
	var result struct {
		Count     int                     `json:"count"`
		Conflicts []conflict.ConflictInfo `json:"conflicts"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput was: %s", err, output)
	}

	if result.Count != 0 {
		t.Errorf("expected count 0, got %d", result.Count)
	}

	// Should be empty array, not null
	if result.Conflicts == nil {
		t.Error("conflicts should be empty array, not null")
	}
}

func TestConflictsCmd_RejectsArguments(t *testing.T) {
	_, cleanup := setupConflictTestEnv(t)
	defer cleanup()

	// Execute command with unexpected arguments
	rootCmd.SetArgs([]string{"conflicts", "unexpected"})
	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)

	err := rootCmd.Execute()
	if err == nil {
		t.Error("conflicts should reject arguments")
	}

	if !strings.Contains(err.Error(), "accepts no arguments") {
		t.Errorf("error should mention 'accepts no arguments', got: %v", err)
	}
}
