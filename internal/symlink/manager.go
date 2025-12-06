package symlink

import (
	"fmt"
	"os"
	"path/filepath"
)

// CreateLink creates a symlink from source (in repo) to target (on filesystem).
// source: relative path within dotfiles repo
// target: absolute path where symlink should be created
func (m *Manager) CreateLink(source, target string) error {
	// Validate source exists in repo
	sourcePath := filepath.Join(m.cfg.Git.RepoPath, source)
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("symlink: source file does not exist in repo: %s [SYMLINK_SOURCE_NOT_FOUND]", source)
	}

	// Validate target parent directory exists
	targetDir := filepath.Dir(target)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return fmt.Errorf("symlink: target parent directory does not exist: %s [SYMLINK_TARGET_PARENT_NOT_FOUND]", targetDir)
	}

	// Check if target already exists (use Lstat to avoid following symlinks)
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("symlink: target already exists: %s [SYMLINK_TARGET_EXISTS]", target)
	}

	// Create symlink
	if err := os.Symlink(sourcePath, target); err != nil {
		return fmt.Errorf("symlink: failed to create symlink: %w [SYMLINK_CREATE_FAILED]", err)
	}

	return nil
}

// RemoveLink removes a symlink at the target path.
// Returns error if target is not a symlink.
func (m *Manager) RemoveLink(target string) error {
	// Check if target exists
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("symlink: target does not exist: %s: %w [SYMLINK_TARGET_NOT_FOUND]", target, err)
		}
		return fmt.Errorf("symlink: failed to stat target: %w [SYMLINK_STAT_FAILED]", err)
	}

	// Verify it's a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("symlink: target is not a symlink: %s [SYMLINK_NOT_A_SYMLINK]", target)
	}

	// Remove the symlink
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("symlink: failed to remove symlink: %w [SYMLINK_REMOVE_FAILED]", err)
	}

	return nil
}
