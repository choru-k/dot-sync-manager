package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CloseAndCaptureErr captures close errors only when no other error exists.
// This helper function prevents error shadowing in defer blocks.
func CloseAndCaptureErr(c io.Closer, err *error) {
	if closeErr := c.Close(); closeErr != nil && *err == nil {
		*err = closeErr
	}
}

// ExpandPath expands ~ to user home directory and resolves relative paths.
// Returns an error if path expansion fails.
func ExpandPath(path string) (string, error) {
	// Handle tilde expansion
	if path == "~" || strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		remainder := strings.TrimLeft(path[1:], "/\\")
		if remainder == "" {
			return homeDir, nil
		}
		return filepath.Join(homeDir, remainder), nil
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
