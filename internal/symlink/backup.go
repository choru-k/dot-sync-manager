package symlink

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// BackupExisting creates a backup of the file at targetPath.
// Returns the backup path and any error.
// Backup naming: backup_YYYYMMDD_HHMMSS_filename
func (m *Manager) BackupExisting(targetPath string) (string, error) {
	// Check if file exists
	info, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("symlink: file does not exist: %s [BACKUP_SOURCE_NOT_FOUND]", targetPath)
	}
	if err != nil {
		return "", fmt.Errorf("symlink: failed to stat file: %w [BACKUP_COPY_FAILED]", err)
	}

	// Create backup directory if needed
	if err := os.MkdirAll(m.backupDir, 0755); err != nil {
		return "", fmt.Errorf("symlink: failed to create backup directory: %w [BACKUP_DIR_CREATE_FAILED]", err)
	}

	// Generate backup filename
	filename := filepath.Base(targetPath)
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("backup_%s_%s", timestamp, filename)
	backupPath := filepath.Join(m.backupDir, backupName)

	// Copy file to backup location
	if info.IsDir() {
		if err := copyDir(targetPath, backupPath); err != nil {
			return "", fmt.Errorf("symlink: failed to backup directory: %w [BACKUP_COPY_FAILED]", err)
		}
	} else {
		if err := copyFile(targetPath, backupPath); err != nil {
			return "", fmt.Errorf("symlink: failed to backup file: %w [BACKUP_COPY_FAILED]", err)
		}
	}

	return backupPath, nil
}

// copyFile copies a single file preserving permissions.
func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

// copyDir recursively copies a directory preserving permissions.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
