package symlink

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/util"
)

const (
	defaultDirPerms       os.FileMode = 0o755                    // drwxr-xr-x; standard for created directories, allowing owner to modify and all users to read/enter
	backupTimestampFormat             = "20060102_150405.000000" // Microsecond precision format
	backupFilePrefix                  = "backup"                 // Prefix for backup filenames
)

// backupPattern matches backup filenames in both old and new formats:
// Old format: backup_YYYYMMDD_HHMMSS.microseconds_filename (basename only)
// New format: backup_YYYYMMDD_HHMMSS.microseconds_/full/path/to/file (encoded full path)
// Accounts for microsecond timestamps, optional collision counter (_001, _002), and path
var backupPattern = regexp.MustCompile(`^backup_(\d{8}_\d{6}(?:\.\d{6})?)(?:_\d{3})?_(.+)$`)

// generateUniqueBackupPath creates a unique backup path by atomically claiming a name.
// Uses OS-level atomic operations (O_EXCL for files, Mkdir for directories) to ensure
// only one process can claim each path. The created placeholder remains in place and
// is reused as the actual backup location (copyFile overwrites with O_TRUNC, copyDir
// uses MkdirAll). If collision occurs, retries with incremented counter suffix.
// Maximum 100 retry attempts before returning error.
func generateUniqueBackupPath(backupDir, targetPath string, isDir bool) (string, error) {
	// Encode full target path for backup filename
	// Replace path separator with underscore to avoid directory creation
	encodedPath := strings.ReplaceAll(targetPath, string(filepath.Separator), "_")

	baseTimestamp := time.Now().Format(backupTimestampFormat)

	// Try with timestamp alone first
	attempt := 0
	maxAttempts := 100 // Prevent infinite loops

	for attempt < maxAttempts {
		var backupName string
		if attempt == 0 {
			backupName = fmt.Sprintf("%s_%s_%s", backupFilePrefix, baseTimestamp, encodedPath)
		} else {
			// Add collision counter: backup_20250101_120000.000000_001_/path/to/file
			backupName = fmt.Sprintf("%s_%s_%03d_%s", backupFilePrefix, baseTimestamp, attempt, encodedPath)
		}

		backupPath := filepath.Join(backupDir, backupName)

		// Atomic check: try to create placeholder to claim the name
		if isDir {
			// Create directory at actual backup path (not .placeholder suffix)
			// Don't remove it - copyDir() will use MkdirAll which succeeds on existing dirs
			err := os.Mkdir(backupPath, defaultDirPerms)
			if err == nil {
				// Successfully claimed the name - leave directory in place
				return backupPath, nil
			}
			// If error is NOT "exists", it's a real error
			if !os.IsExist(err) {
				return "", fmt.Errorf("failed to check directory availability: %w", err)
			}
		} else {
			// Create empty file at actual backup path
			// Don't remove it - copyFile() will open with O_TRUNC to overwrite
			f, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err == nil {
				// Successfully claimed the name - leave file in place
				_ = f.Close()
				return backupPath, nil
			}
			// Defensive close (f is always nil when err != nil in Go stdlib, but be explicit)
			if f != nil {
				_ = f.Close()
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

// RestoreFromBackup restores a file or directory from a backup to the target path.
// If the target already exists (file, directory, or symlink), it will be removed first.
// Parent directories are created if they don't exist. Permissions are preserved.
//
// Returns RESTORE_BACKUP_NOT_FOUND if the backup doesn't exist.
// Returns RESTORE_TARGET_REMOVE_FAILED if removing existing target fails.
// Returns RESTORE_DIR_CREATE_FAILED if creating parent directories fails.
// Returns RESTORE_COPY_FAILED if copying the backup content fails.
func (m *Manager) RestoreFromBackup(backupPath, targetPath string) error {
	// Check backup exists
	backupInfo, err := os.Stat(backupPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("symlink: backup does not exist: %s [RESTORE_BACKUP_NOT_FOUND]", backupPath)
	}
	if err != nil {
		return fmt.Errorf("symlink: failed to stat backup: %w [RESTORE_BACKUP_NOT_FOUND]", err)
	}

	// Remove existing target if present (use Lstat to detect symlinks without following)
	if _, err := os.Lstat(targetPath); err == nil {
		if err := os.RemoveAll(targetPath); err != nil {
			return fmt.Errorf("symlink: failed to remove existing target: %w [RESTORE_TARGET_REMOVE_FAILED]", err)
		}
	}

	// Create target parent directory if needed
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, defaultDirPerms); err != nil {
		return fmt.Errorf("symlink: failed to create target directory: %w [RESTORE_DIR_CREATE_FAILED]", err)
	}

	// Copy backup to target
	if backupInfo.IsDir() {
		if err := copyDir(backupPath, targetPath); err != nil {
			return fmt.Errorf("symlink: failed to restore directory: %w [RESTORE_COPY_FAILED]", err)
		}
	} else {
		if err := copyFile(backupPath, targetPath); err != nil {
			return fmt.Errorf("symlink: failed to restore file: %w [RESTORE_COPY_FAILED]", err)
		}
	}

	return nil
}

// CleanupOldBackups removes backups older than retentionDays.
// Returns the number of backups deleted and any error encountered.
// If the backup directory doesn't exist, returns (0, nil).
// Individual deletion failures are silently skipped to allow partial cleanup.
func (m *Manager) CleanupOldBackups(retentionDays int) (int, error) {
	if retentionDays < 0 {
		return 0, fmt.Errorf("symlink: retention days must be non-negative [CLEANUP_INVALID_RETENTION]")
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	deleted := 0

	entries, err := os.ReadDir(m.backupDir)
	if os.IsNotExist(err) {
		return 0, nil // No backup dir yet
	}
	if err != nil {
		return 0, fmt.Errorf("symlink: failed to read backup directory: %w [CLEANUP_DIR_READ_FAILED]", err)
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip entries we can't stat
		}

		if info.ModTime().Before(cutoff) {
			backupPath := filepath.Join(m.backupDir, entry.Name())
			if err := os.RemoveAll(backupPath); err != nil {
				continue // Skip deletion failures but continue cleanup
			}
			deleted++
		}
	}

	return deleted, nil
}

// ListBackups returns metadata for all backups in the backup directory.
// Backups are sorted by timestamp (newest first).
// If the backup directory doesn't exist, returns (nil, nil).
// Individual entry errors are silently skipped.
func (m *Manager) ListBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(m.backupDir)
	if os.IsNotExist(err) {
		return nil, nil // No backups yet
	}
	if err != nil {
		return nil, fmt.Errorf("symlink: failed to read backup directory: %w [LIST_DIR_READ_FAILED]", err)
	}

	backups := make([]BackupInfo, 0, len(entries))

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip entries we can't stat
		}

		backupPath := filepath.Join(m.backupDir, entry.Name())

		// Parse backup name for original path
		originalPath := ""
		if matches := backupPattern.FindStringSubmatch(entry.Name()); len(matches) == 3 {
			// Decode path: reverse the encoding from generateUniqueBackupPath
			encodedPath := matches[2]
			originalPath = strings.ReplaceAll(encodedPath, "_", string(filepath.Separator))

			// If path doesn't start with separator, it's old format (basename only)
			// Keep it as-is for backward compatibility
			if !filepath.IsAbs(originalPath) {
				originalPath = encodedPath // Keep basename for old backups
			}
		}

		backups = append(backups, BackupInfo{
			OriginalPath: originalPath,
			BackupPath:   backupPath,
			Timestamp:    info.ModTime(),
			Size:         info.Size(),
		})
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
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
