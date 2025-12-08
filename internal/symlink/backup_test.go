package symlink

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

const (
	testFilePerms os.FileMode = 0644  // -rw-r--r--
	testDirPerms  os.FileMode = 0o755 // drwxr-xr-x
)

func TestBackup_BackupExisting_File(t *testing.T) {
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	// Create file to backup
	targetFile := filepath.Join(targetDir, ".bashrc")
	content := []byte("# original content")
	if err := os.WriteFile(targetFile, content, testFilePerms); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}
	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}
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

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}
	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}
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
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}
	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}
	mgr.SetBackupDir(t.TempDir())

	_, err = mgr.BackupExisting("/nonexistent/file")
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

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}
	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}
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
