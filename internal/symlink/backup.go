package symlink

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultDirPerms os.FileMode = 0o755 // drwxr-xr-x - standard directory permissions
)

// BackupExisting creates a timestamped backup of the file or directory at targetPath
// before symlinking operations. The backup is stored in the backup directory with
// format: backup_YYYYMMDD_HHMMSS_filename
//
// For files, permissions are preserved via os.FileMode.
// For directories, the entire structure is copied recursively with permissions preserved.
// Symlinks within directories are preserved as symlinks.
//
// Returns the absolute path to the backup file/directory and any error encountered.
// Returns BACKUP_SOURCE_NOT_FOUND if targetPath doesn't exist.
// Returns BACKUP_COPY_FAILED if the copy operation fails.
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
	if err := os.MkdirAll(m.backupDir, defaultDirPerms); err != nil {
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

// copyFile copies a single file from src to dst, preserving the source file's permissions.
// This is used during backup operations to ensure backed-up files maintain their original
// permission settings for accurate restoration.
func copyFile(src, dst string) (err error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copyFile: stat source failed: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copyFile: open source failed: %w", err)
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("copyFile: close source failed: %w", closeErr)
		}
	}()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("copyFile: open destination failed: %w", err)
	}
	defer func() {
		if closeErr := dstFile.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("copyFile: close destination failed: %w", closeErr)
		}
	}()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copyFile: io.Copy failed: %w", err)
	}

	return nil
}

// copyDir recursively copies a directory from src to dst, preserving the directory structure
// and all file/directory permissions. Symlinks are preserved as symlinks.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copyDir: stat %s failed: %w", src, err)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("copyDir: MkdirAll %s failed: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("copyDir: ReadDir %s failed: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Use Lstat to detect symlinks (doesn't follow them)
		entryInfo, err := os.Lstat(srcPath)
		if err != nil {
			return fmt.Errorf("copyDir: Lstat %s failed: %w", srcPath, err)
		}

		// Handle symlinks by preserving them
		if entryInfo.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(srcPath)
			if err != nil {
				return fmt.Errorf("copyDir: Readlink %s failed: %w", srcPath, err)
			}
			if err := os.Symlink(target, dstPath); err != nil {
				return fmt.Errorf("copyDir: Symlink %s failed: %w", dstPath, err)
			}
			continue
		}

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err // Already wrapped from recursive call
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err // Already wrapped from copyFile
			}
		}
	}

	return nil
}
