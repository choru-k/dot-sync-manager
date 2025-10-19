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
	// Handle tilde pointing to the current user's home directory
	if path == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return homeDir, nil
	}

	// Support both Unix (~/<path>) and Windows (~\<path>) separators.
	if len(path) >= 2 && path[0] == '~' && (path[1] == '/' || path[1] == '\\') {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}

		// Use path[2:] to avoid losing the home directory when joining (e.g., filepath.Join("~/", "/foo"))
		remainder := strings.TrimLeft(path[2:], "/\\")
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
