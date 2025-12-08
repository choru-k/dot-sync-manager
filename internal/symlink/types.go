package symlink

import (
	"os"
	"path/filepath"
	"time"
)

// BackupInfo represents metadata about a backup file.
type BackupInfo struct {
	OriginalPath string    // Original file path that was backed up
	BackupPath   string    // Path to the backup file
	Timestamp    time.Time // When the backup was created
	Size         int64     // Size in bytes
}

// DefaultBackupDir returns the default backup directory path.
// Returns ~/.dsm/backups/symlink/
func DefaultBackupDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".dsm", "backups", "symlink")
}
