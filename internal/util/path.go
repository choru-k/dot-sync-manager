package util

import (
	"fmt"
	"io"
	"log"
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

		// Use TrimLeft on path[1:] to normalize any combination of separators to a clean relative suffix.
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

// CreateFileSecurely creates a file with TOCTOU protection using os.O_EXCL flag
// This prevents race conditions between checking existence and creating files
func CreateFileSecurely(filePath string, data []byte, perm os.FileMode) error {
	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	// Use os.O_EXCL to ensure atomic creation and prevent TOCTOU attacks
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("file already exists: %s", filePath)
		}
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer CloseAndCaptureErr(file, &err)

	// Write the data
	if _, err = file.Write(data); err != nil {
		// Try to clean up the file if write fails
		if rmErr := os.Remove(filePath); rmErr != nil {
			log.Printf("Warning: failed to remove file after write error: %v", rmErr)
		}
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

// CreateDirectorySecurely creates a directory with TOCTOU protection
func CreateDirectorySecurely(dirPath string, perm os.FileMode) error {
	// Use os.Mkdir with os.ModePerm to ensure atomic directory creation
	// and prevent race conditions
	if err := os.Mkdir(dirPath, perm); err != nil {
		if os.IsExist(err) {
			// Directory already exists, check if it's actually a directory
			if info, statErr := os.Stat(dirPath); statErr == nil {
				if info.IsDir() {
					return nil // Directory already exists and is a directory
				}
				return fmt.Errorf("path exists but is not a directory: %s", dirPath)
			}
			return fmt.Errorf("failed to stat path: %w", err)
		}
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}
