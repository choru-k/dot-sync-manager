package util

import (
	"os"
	"path/filepath"
)

// ExpandPath expands ~ to user home directory and resolves relative paths
func ExpandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		if homeDir, err := os.UserHomeDir(); err == nil {
			if len(path) == 1 {
				return homeDir
			}
			return filepath.Join(homeDir, path[1:])
		}
	}

	// Convert to absolute path if it's not already
	if !filepath.IsAbs(path) {
		if absPath, err := filepath.Abs(path); err == nil {
			return absPath
		}
	}

	return path
}