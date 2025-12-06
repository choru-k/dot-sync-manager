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

	return nil
}
