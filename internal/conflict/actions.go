package conflict

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// defaultDirPerms is the default permission mode for directories created during conflict resolution.
const defaultDirPerms = 0755

// defaultFilePerms is the default permission mode for files created during conflict resolution.
const defaultFilePerms = 0644

// UseLocal resolves a conflict by using the local version.
// Copies the local file to the target location and marks as resolved.
func (s *Service) UseLocal(file string) error {
	// Find timestamp dir containing this conflict
	timestampDir, err := s.findLatestConflictDir(file)
	if err != nil {
		return err
	}

	localPath := filepath.Join(timestampDir, file+".local")

	// Check if local file exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		return fmt.Errorf("local version not found for: %s", file)
	}

	// Get target path from config mappings or derive from home
	targetPath, err := s.getTargetPath(file)
	if err != nil {
		return err
	}

	// Copy local to target
	if err := copyFile(localPath, targetPath); err != nil {
		return fmt.Errorf("failed to copy local version: %w", err)
	}

	// Remove conflict artifacts
	if err := s.removeConflictArtifacts(timestampDir, file); err != nil {
		return fmt.Errorf("failed to remove conflict artifacts: %w", err)
	}

	// Fire event
	s.notifyConflictResolved(file)

	// Check if all conflicts are resolved
	if !s.HasConflicts() {
		s.notifyAllConflictsResolved()
	}

	return nil
}

// UseRemote resolves a conflict by using the remote version.
// Copies the remote file to the target location and marks as resolved.
func (s *Service) UseRemote(file string) error {
	// Find timestamp dir containing this conflict
	timestampDir, err := s.findLatestConflictDir(file)
	if err != nil {
		return err
	}

	remotePath := filepath.Join(timestampDir, file+".remote")

	// Check if remote file exists
	if _, err := os.Stat(remotePath); os.IsNotExist(err) {
		return fmt.Errorf("remote version not found for: %s", file)
	}

	// Get target path from config mappings or derive from home
	targetPath, err := s.getTargetPath(file)
	if err != nil {
		return err
	}

	// Copy remote to target
	if err := copyFile(remotePath, targetPath); err != nil {
		return fmt.Errorf("failed to copy remote version: %w", err)
	}

	// Remove conflict artifacts
	if err := s.removeConflictArtifacts(timestampDir, file); err != nil {
		return fmt.Errorf("failed to remove conflict artifacts: %w", err)
	}

	// Fire event
	s.notifyConflictResolved(file)

	// Check if all conflicts are resolved
	if !s.HasConflicts() {
		s.notifyAllConflictsResolved()
	}

	return nil
}

// MarkResolved marks specific conflicts as resolved without copying files.
// Useful when user has manually resolved conflicts.
// Non-existent conflicts are silently skipped (idempotent behavior).
func (s *Service) MarkResolved(files []string) error {
	var errs []error

	for _, file := range files {
		timestampDir, err := s.findLatestConflictDir(file)
		if err != nil {
			// If conflict doesn't exist, skip it (already resolved)
			continue
		}

		if err := s.removeConflictArtifacts(timestampDir, file); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", file, err))
			continue
		}
		s.notifyConflictResolved(file)
	}

	// Check if all conflicts are resolved
	if !s.HasConflicts() {
		s.notifyAllConflictsResolved()
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to mark some conflicts as resolved: %v", errs)
	}

	return nil
}

// MarkAllResolved marks all active conflicts as resolved.
func (s *Service) MarkAllResolved() error {
	conflicts, err := s.GetConflicts()
	if err != nil {
		return err
	}

	if len(conflicts) == 0 {
		return nil
	}

	var files []string
	for _, c := range conflicts {
		files = append(files, c.File)
	}

	return s.MarkResolved(files)
}

// OpenFolder opens the conflict directory in the file manager.
func (s *Service) OpenFolder() error {
	return openInFileManager(s.conflictDir)
}

// OpenBackupFolder opens the backup directory in the file manager.
func (s *Service) OpenBackupFolder() error {
	backupDir := filepath.Join(filepath.Dir(s.conflictDir), "backups")
	return openInFileManager(backupDir)
}

// getTargetPath returns the target file path for a repo file.
func (s *Service) getTargetPath(file string) (string, error) {
	// Check mappings first
	if s.cfg.Mappings != nil {
		if target, ok := s.cfg.Mappings[file]; ok {
			return target, nil
		}
	}

	// Fall back to home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine target path for %s: %w", file, err)
	}
	return filepath.Join(home, file), nil
}

// removeConflictArtifacts removes the conflict files for a file from a timestamp dir.
// If the timestamp directory becomes empty, it is also removed.
func (s *Service) removeConflictArtifacts(timestampDir, file string) error {
	// Remove file.local, file.remote, file.base
	suffixes := []string{".local", ".remote", ".base"}
	for _, suffix := range suffixes {
		path := filepath.Join(timestampDir, file+suffix)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", path, err)
		}
	}

	// Check if timestamp directory is now empty and remove it
	entries, err := os.ReadDir(timestampDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if len(entries) == 0 {
		if err := os.Remove(timestampDir); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

// copyFile copies src to dst, creating directories as needed.
func copyFile(src, dst string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), defaultDirPerms); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultFilePerms)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(dstFile, srcFile)
	closeErr := dstFile.Close()

	// Check copy error first, then close error (close can fail on flush)
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return nil
}

// openInFileManager opens a directory in the system file manager.
func openInFileManager(path string) error {
	// Ensure directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, defaultDirPerms); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{path}
	case "windows":
		cmd = "explorer"
		args = []string{path}
	default: // Linux
		cmd = "xdg-open"
		args = []string{path}
	}

	return exec.Command(cmd, args...).Start()
}
