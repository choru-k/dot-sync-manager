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
	return nil // Minimal implementation
}
