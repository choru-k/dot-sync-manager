package symlink

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

const (
	testFilePerms os.FileMode = 0o644 // -rw-r--r--
	testDirPerms  os.FileMode = 0o755 // drwxr-xr-x
)

// newTestManager creates a Manager instance for testing
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}
	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}
	return mgr
}

func TestBackup_BackupExisting_File(t *testing.T) {
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	// Create file to backup
	targetFile := filepath.Join(targetDir, ".bashrc")
	content := []byte("# original content")
	if err := os.WriteFile(targetFile, content, testFilePerms); err != nil {
		t.Fatal(err)
	}

	mgr := newTestManager(t)
	mgr.SetBackupDir(backupDir)

	// Backup
	backupPath, err := mgr.BackupExisting(targetFile)
	if err != nil {
		t.Fatalf("BackupExisting failed: %v", err)
	}

	// Verify backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatal("Backup file should exist")
	}

	// Verify content matches
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backupContent) != string(content) {
		t.Error("Backup content doesn't match original")
	}

	// Verify naming pattern
	if !strings.Contains(backupPath, "backup_") {
		t.Error("Backup path should contain 'backup_' prefix")
	}
	if !strings.Contains(backupPath, ".bashrc") {
		t.Error("Backup path should contain original filename")
	}
}

func TestBackup_BackupExisting_Directory(t *testing.T) {
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	// Create directory with files
	configDir := filepath.Join(targetDir, ".config")
	if err := os.MkdirAll(filepath.Join(configDir, "nvim"), testDirPerms); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "nvim", "init.vim"), []byte("# vim"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	mgr := newTestManager(t)
	mgr.SetBackupDir(backupDir)

	// Backup
	backupPath, err := mgr.BackupExisting(configDir)
	if err != nil {
		t.Fatalf("BackupExisting directory failed: %v", err)
	}

	// Verify backup structure
	nestedFile := filepath.Join(backupPath, "nvim", "init.vim")
	if _, err := os.Stat(nestedFile); os.IsNotExist(err) {
		t.Error("Nested file should exist in backup")
	}
}

func TestBackup_BackupExisting_NotExists(t *testing.T) {
	mgr := newTestManager(t)
	mgr.SetBackupDir(t.TempDir())

	// Use cross-platform path construction
	_, err := mgr.BackupExisting(filepath.Join(t.TempDir(), "nonexistent-file"))
	if err == nil {
		t.Fatal("Expected error for non-existent file, got nil")
	}
	if !strings.Contains(err.Error(), "[BACKUP_SOURCE_NOT_FOUND]") {
		t.Errorf("Expected error to contain [BACKUP_SOURCE_NOT_FOUND], got: %v", err)
	}
}

func TestBackup_BackupExisting_WithSymlinks(t *testing.T) {
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	// Create directory with symlinked subdirectory
	configDir := filepath.Join(targetDir, ".config")
	if err := os.MkdirAll(configDir, testDirPerms); err != nil {
		t.Fatal(err)
	}

	// Create actual directory and symlink to it
	actualDir := filepath.Join(targetDir, "actual")
	if err := os.MkdirAll(actualDir, testDirPerms); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(actualDir, "file.txt"), []byte("content"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(configDir, "link")
	if err := os.Symlink(actualDir, symlinkPath); err != nil {
		t.Fatal(err)
	}

	mgr := newTestManager(t)
	mgr.SetBackupDir(backupDir)

	// Backup directory containing symlink
	backupPath, err := mgr.BackupExisting(configDir)
	if err != nil {
		t.Fatalf("BackupExisting failed: %v", err)
	}

	// Verify symlink was preserved
	backupSymlink := filepath.Join(backupPath, "link")
	linkInfo, err := os.Lstat(backupSymlink)
	if err != nil {
		t.Fatal(err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Error("Backup should preserve symlink, but found regular file/dir")
	}

	// Verify symlink target
	target, err := os.Readlink(backupSymlink)
	if err != nil {
		t.Fatal(err)
	}
	if target != actualDir {
		t.Errorf("Symlink target mismatch: got %s, want %s", target, actualDir)
	}
}

func TestBackup_BackupExisting_WithRelativeSymlink(t *testing.T) {
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	// Create directory structure: targetDir/.config/link -> ../actual/file.txt
	configDir := filepath.Join(targetDir, ".config")
	if err := os.MkdirAll(configDir, testDirPerms); err != nil {
		t.Fatal(err)
	}

	actualDir := filepath.Join(targetDir, "actual")
	if err := os.MkdirAll(actualDir, testDirPerms); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(actualDir, "file.txt")
	if err := os.WriteFile(testFile, []byte("content"), testFilePerms); err != nil {
		t.Fatal(err)
	}

	// Create RELATIVE symlink: .config/link -> ../actual
	symlinkPath := filepath.Join(configDir, "link")
	if err := os.Symlink("../actual", symlinkPath); err != nil {
		t.Fatal(err)
	}

	mgr := newTestManager(t)
	mgr.SetBackupDir(backupDir)

	// Backup directory containing relative symlink
	backupPath, err := mgr.BackupExisting(configDir)
	if err != nil {
		t.Fatalf("BackupExisting failed: %v", err)
	}

	// Verify symlink was preserved and target is now absolute
	backupSymlink := filepath.Join(backupPath, "link")
	linkInfo, err := os.Lstat(backupSymlink)
	if err != nil {
		t.Fatal(err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Error("Backup should preserve symlink")
	}

	// Verify target is absolute (converted from relative)
	target, err := os.Readlink(backupSymlink)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(target) {
		t.Errorf("Relative symlink should be converted to absolute, got: %s", target)
	}

	// Verify target points to correct location
	expectedTarget := actualDir // Should resolve to absolute path
	if target != expectedTarget {
		t.Errorf("Symlink target mismatch: got %s, want %s", target, expectedTarget)
	}
}

func TestBackup_BackupExisting_TimestampUniqueness(t *testing.T) {
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	// Create test file
	targetFile := filepath.Join(targetDir, ".bashrc")
	content := []byte("# test")
	if err := os.WriteFile(targetFile, content, testFilePerms); err != nil {
		t.Fatal(err)
	}

	mgr := newTestManager(t)
	mgr.SetBackupDir(backupDir)

	// Create two rapid backups
	backup1, err := mgr.BackupExisting(targetFile)
	if err != nil {
		t.Fatalf("First backup failed: %v", err)
	}

	backup2, err := mgr.BackupExisting(targetFile)
	if err != nil {
		t.Fatalf("Second backup failed: %v", err)
	}

	// Verify both backups exist (no overwrite)
	if backup1 == backup2 {
		t.Error("Rapid backups should have different paths")
	}

	if _, err := os.Stat(backup1); os.IsNotExist(err) {
		t.Error("First backup was overwritten")
	}
	if _, err := os.Stat(backup2); os.IsNotExist(err) {
		t.Error("Second backup doesn't exist")
	}
}

func TestBackup_BackupExisting_ConcurrentSafety(t *testing.T) {
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	// Create test file
	targetFile := filepath.Join(targetDir, ".bashrc")
	content := []byte("# test")
	if err := os.WriteFile(targetFile, content, testFilePerms); err != nil {
		t.Fatal(err)
	}

	mgr := newTestManager(t)
	mgr.SetBackupDir(backupDir)

	// Launch 10 concurrent backups
	const numBackups = 10
	results := make(chan string, numBackups)
	errors := make(chan error, numBackups)

	for i := 0; i < numBackups; i++ {
		go func() {
			path, err := mgr.BackupExisting(targetFile)
			if err != nil {
				errors <- err
				return
			}
			results <- path
		}()
	}

	// Collect results
	var backupPaths []string
	for i := 0; i < numBackups; i++ {
		select {
		case path := <-results:
			backupPaths = append(backupPaths, path)
		case err := <-errors:
			t.Fatalf("Backup failed: %v", err)
		}
	}

	// Verify all paths are unique (no overwrites)
	pathSet := make(map[string]bool)
	for _, path := range backupPaths {
		if pathSet[path] {
			t.Errorf("Duplicate backup path detected: %s", path)
		}
		pathSet[path] = true

		// Verify file exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Backup file doesn't exist: %s", path)
		}
	}

	// Verify we have all 10 unique backups
	if len(pathSet) != numBackups {
		t.Errorf("Expected %d unique backups, got %d", numBackups, len(pathSet))
	}
}
