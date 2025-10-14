package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/process"
)

func setupTestConfig(t *testing.T) (*config.SyncConfig, string, string) {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	repoPath := filepath.Join(homeDir, "dotfiles")
	configPath := filepath.Join(repoPath, ".sync-config.json")

	cfg := config.DefaultConfig()
	cfg.Git.RepoPath = repoPath
	cfg.Git.RemoteURL = ""
	cfg.Git.RemoteName = "origin"
	cfg.Git.AuthType = gitmanager.AuthStrategyNone
	cfg.Git.AuthorName = "Test User"
	cfg.Git.AuthorEmail = "test@example.com"
	cfg.Machine.Name = "test-machine"
	cfg.ConfigPath = configPath

	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	configFile = configPath
	t.Cleanup(func() { configFile = "" })

	return cfg, homeDir, repoPath
}

func TestRunAddCreatesSymlink(t *testing.T) {
	cfg, homeDir, repoPath := setupTestConfig(t)

	sourcePath := filepath.Join(homeDir, ".bashrc")
	if err := os.WriteFile(sourcePath, []byte("export TEST_VAR=1\n"), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	if err := runAdd(nil, []string{sourcePath}); err != nil {
		t.Fatalf("runAdd failed: %v", err)
	}

	info, err := os.Lstat(sourcePath)
	if err != nil {
		t.Fatalf("failed to stat source after add: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected source to be symlink, got mode %v", info.Mode())
	}

	targetPath := filepath.Join(repoPath, "bashrc")
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read target file: %v", err)
	}
	if string(content) != "export TEST_VAR=1\n" {
		t.Fatalf("unexpected target content: %q", string(content))
	}

	reloaded, err := config.LoadFromFile(cfg.ConfigPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	mapping, ok := reloaded.Mappings["bashrc"]
	if !ok {
		t.Fatalf("expected mapping for bashrc to exist")
	}
	if mapping != sourcePath {
		t.Fatalf("unexpected mapping target: %s", mapping)
	}

	backupDir := filepath.Join(repoPath, ".backup")
	entries, err := os.ReadDir(backupDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to read backup dir: %v", err)
	}
	if err == nil && len(entries) != 0 {
		t.Fatalf("expected backup directory to be empty, found %d entries", len(entries))
	}
}

func TestRunForegroundDaemonLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("foreground daemon signal handling not supported on Windows in tests")
	}

	cfg, homeDir, _ := setupTestConfig(t)

	done := make(chan error, 1)
	go func() {
		done <- runForegroundDaemon(cfg)
	}()

	pidPath := filepath.Join(homeDir, ".dotfile-sync-manager.pid")
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(pidPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pid file was not created")
		}
		time.Sleep(50 * time.Millisecond)
	}

	sig := daemonSignals()[0]
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("failed to find current process: %v", err)
	}
	if err := proc.Signal(sig); err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runForegroundDaemon returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("foreground daemon did not stop after interrupt")
	}

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("expected pid file to be removed, got err=%v", err)
	}
}

func TestRunAddBackupAndRollbackOnSymlinkFailure(t *testing.T) {
	_, homeDir, repoPath := setupTestConfig(t)

	sourcePath := filepath.Join(homeDir, ".bashrc")
	originalContent := "export TEST_VAR=original\n"
	if err := os.WriteFile(sourcePath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Create a directory at the target path to force symlink creation failure
	targetPath := filepath.Join(repoPath, "bashrc")
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatalf("failed to create conflicting directory: %v", err)
	}

	err := runAdd(nil, []string{sourcePath})
	if err == nil {
		t.Fatalf("expected runAdd to fail, but it succeeded")
	}

	// The original file should still exist since the move failed before symlink creation
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("failed to read source file after failed add: %v", err)
	}
	if string(content) != originalContent {
		t.Fatalf("expected original content to be preserved, got %q", string(content))
	}

	// Verify source is not a symlink
	info, err := os.Lstat(sourcePath)
	if err != nil {
		t.Fatalf("failed to stat source after failed add: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected source to be regular file after failed add, got symlink")
	}
}

func TestRunAddBackupCleanupOnSuccess(t *testing.T) {
	_, homeDir, repoPath := setupTestConfig(t)

	sourcePath := filepath.Join(homeDir, ".bashrc")
	originalContent := "export TEST_VAR=original\n"
	if err := os.WriteFile(sourcePath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	if err := runAdd(nil, []string{sourcePath}); err != nil {
		t.Fatalf("runAdd failed: %v", err)
	}

	// Verify backup directory is empty after successful operation
	backupDir := filepath.Join(repoPath, ".backup")
	entries, err := os.ReadDir(backupDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to read backup dir: %v", err)
	}
	if err == nil && len(entries) != 0 {
		t.Fatalf("expected backup directory to be empty after success, found %d entries", len(entries))
	}

	// Verify symlink was created correctly
	info, err := os.Lstat(sourcePath)
	if err != nil {
		t.Fatalf("failed to stat source after add: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected source to be symlink, got mode %v", info.Mode())
	}
}

func TestProcessDetectionRaceCondition(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific process detection test")
	}

	// Test signal 0 check for non-existent PID
	nonExistentPID := 99999
	if processExists(nonExistentPID) {
		t.Fatalf("processExists returned true for non-existent PID %d", nonExistentPID)
	}

	// Test signal 0 check for current PID
	currentPID := os.Getpid()
	if !processExists(currentPID) {
		t.Fatalf("processExists returned false for current PID %d", currentPID)
	}

	// Test invalid PID handling
	if processExists(0) {
		t.Fatalf("processExists returned true for PID 0")
	}

	if processExists(-1) {
		t.Fatalf("processExists returned true for negative PID")
	}
}

func TestProcessExistsImplementation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Test the actual syscall.Kill(pid, 0) implementation
	currentPID := os.Getpid()

	// This should succeed (process exists)
	err := syscall.Kill(currentPID, 0)
	if err != nil {
		t.Fatalf("syscall.Kill(currentPID, 0) failed: %v", err)
	}

	// This should fail (process doesn't exist)
	nonExistentPID := 99999
	err = syscall.Kill(nonExistentPID, 0)
	if err == nil {
		t.Fatalf("syscall.Kill(nonExistentPID, 0) should have failed")
	}
}

func TestPIDFileManagement(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	originalPID := 12345
	if err := process.WritePID(originalPID); err != nil {
		t.Fatalf("failed to write PID: %v", err)
	}

	pidPath := filepath.Join(homeDir, ".dotfile-sync-manager.pid")

	// Verify PID file was created
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("PID file was not created: %v", err)
	}

	// Test PID removal
	if err := process.RemovePID(); err != nil {
		t.Fatalf("failed to remove PID file: %v", err)
	}

	// Verify PID file was removed
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("PID file was not removed: %v", err)
	}
}

// Helper function for process testing
func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
