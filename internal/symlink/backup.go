package symlink

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/util"
)

const (
	defaultDirPerms       os.FileMode = 0o755                    // drwxr-xr-x; standard for created directories, allowing owner to modify and all users to read/enter
	backupTimestampFormat             = "20060102_150405.000000" // Microsecond precision format
	backupFilePrefix                  = "backup"                 // Prefix for backup filenames
)

// generateUniqueBackupPath creates a unique backup path using placeholder files
// to detect collisions. Uses retry loop with counter suffix if name conflicts occur.
// While not fully atomic (TOCTOU between placeholder removal and actual copy),
// microsecond precision timestamps combined with retries provide sufficient uniqueness
// for practical purposes. Maximum 100 retry attempts before returning error.
func generateUniqueBackupPath(backupDir, targetPath string, isDir bool) (string, error) {
	filename := filepath.Base(targetPath)
	baseTimestamp := time.Now().Format(backupTimestampFormat)

	// Try with timestamp alone first
	attempt := 0
	maxAttempts := 100 // Prevent infinite loops

	for attempt < maxAttempts {
		var backupName string
		if attempt == 0 {
			backupName = fmt.Sprintf("%s_%s_%s", backupFilePrefix, baseTimestamp, filename)
		} else {
			// Add collision counter: backup_20250101_120000.000000_001_file
			backupName = fmt.Sprintf("%s_%s_%03d_%s", backupFilePrefix, baseTimestamp, attempt, filename)
		}

		backupPath := filepath.Join(backupDir, backupName)

		// Atomic check: try to create placeholder to claim the name
		if isDir {
			// For directories, attempt os.Mkdir (not MkdirAll) with placeholder
			// We'll remove this and create real directory in copyDir
			placeholderPath := backupPath + ".placeholder"
			err := os.Mkdir(placeholderPath, defaultDirPerms)
			if err == nil {
				// Successfully claimed the name
				_ = os.Remove(placeholderPath) // Clean up placeholder
				return backupPath, nil
			}
			// If error is NOT "exists", it's a real error
			if !os.IsExist(err) {
				return "", fmt.Errorf("failed to check directory availability: %w", err)
			}
		} else {
			// For files, use O_EXCL to atomically claim the name
			f, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err == nil {
				// Successfully claimed the name
				_ = f.Close()
				_ = os.Remove(backupPath) // Clean up placeholder file
				return backupPath, nil
			}
			// If error is NOT "exists", it's a real error
			if !os.IsExist(err) {
				return "", fmt.Errorf("failed to check file availability: %w", err)
			}
		}

		// Path exists, increment and retry
		attempt++
	}

	return "", fmt.Errorf("failed to generate unique backup path after %d attempts", maxAttempts)
}

// BackupExisting creates a timestamped backup of the file or directory at targetPath
// before symlinking operations. The backup is stored in the backup directory with
// format: backup_YYYYMMDD_HHMMSS.microseconds_filename
//
// For files, permissions are preserved via os.FileMode.
// For directories, the entire structure is copied recursively with permissions preserved.
// Symlinks within directories are preserved, with relative symlinks converted to absolute
// paths to ensure backup integrity. Absolute symlinks are preserved as-is.
//
// SECURITY NOTE: Symlink targets are not validated. Ensure source directories are trusted.
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

	// Generate unique backup path atomically
	backupPath, err := generateUniqueBackupPath(m.backupDir, targetPath, info.IsDir())
	if err != nil {
		return "", fmt.Errorf("symlink: failed to generate backup path: %w [BACKUP_COPY_FAILED]", err)
	}

	// Copy file to backup location
	if info.IsDir() {
		if err := copyDir(targetPath, backupPath); err != nil {
			if cleanupErr := os.RemoveAll(backupPath); cleanupErr != nil {
				return "", fmt.Errorf("symlink: failed to backup directory: %w (cleanup also failed: %v) [BACKUP_COPY_FAILED]", err, cleanupErr)
			}
			return "", fmt.Errorf("symlink: failed to backup directory: %w [BACKUP_COPY_FAILED]", err)
		}
	} else {
		if err := copyFile(targetPath, backupPath); err != nil {
			if cleanupErr := os.RemoveAll(backupPath); cleanupErr != nil {
				return "", fmt.Errorf("symlink: failed to backup file: %w (cleanup also failed: %v) [BACKUP_COPY_FAILED]", err, cleanupErr)
			}
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
	defer util.CloseAndCaptureErr(srcFile, &err)

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("copyFile: open destination failed: %w", err)
	}
	defer util.CloseAndCaptureErr(dstFile, &err)

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

		// Handle symlinks by preserving them (convert relative to absolute)
		if entryInfo.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(srcPath)
			if err != nil {
				return fmt.Errorf("copyDir: Readlink %s failed: %w", srcPath, err)
			}
			// Convert relative symlinks to absolute to ensure backup integrity.
			// Relative symlinks like "../actual" would break when the backup directory
			// is moved to a different location (e.g., ~/.dsm/backups/).
			// Trade-offs:
			// - Pros: Backup preserves what symlink points to, restoration will work
			// - Cons: Absolute paths may contain machine-specific info, less portable
			// TODO(A3-03): RestoreBackup will need to handle absolute paths appropriately
			//              for cross-machine restoration scenarios.
			if !filepath.IsAbs(target) {
				target = filepath.Clean(filepath.Join(filepath.Dir(srcPath), target))
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
