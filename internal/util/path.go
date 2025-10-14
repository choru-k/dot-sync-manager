package util

import (
"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandPath expands ~ to user home directory and resolves relative paths.
// Returns an error if path expansion fails.
func ExpandPath(path string) (string, error) {
	// Handle tilde expansion
	if path == "~" || strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		// For "~", use empty string slice which will result in just homeDir
		// For "~/path", use the path after the "~/" prefix
		if len(path) > 2 {
			return filepath.Join(homeDir, path[2:]), nil
		}
		return homeDir, nil
	}

	// Convert to absolute path if it's not already
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path: %w", err)
		}
		return absPath, nil
	}

	return path, nil
}
