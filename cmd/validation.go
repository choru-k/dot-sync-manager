package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/util"
)

// Constants for validation
const (
	maxConfigKeyLength = 100
)

// Safe characters for configuration keys
var configKeyPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.]*$`)

// validateConfigKey validates a configuration key for security and format
func validateConfigKey(key string) error {
	if key == "" {
		return fmt.Errorf("configuration key cannot be empty")
	}

	if len(key) > maxConfigKeyLength {
		return fmt.Errorf("configuration key too long (max %d characters): %s", maxConfigKeyLength, key)
	}

	if !configKeyPattern.MatchString(key) {
		return fmt.Errorf("invalid configuration key format: %s (must start with letter and contain only letters, numbers, underscores, and dots)", key)
	}

	// Prevent configuration keys that could be dangerous
	dangerousKeys := []string{
		"password", "passwd", "secret", "key", "token", "auth",
		"private_key", "ssh_key", "credential", "cert",
	}

	keyLower := strings.ToLower(key)
	for _, dangerous := range dangerousKeys {
		if strings.Contains(keyLower, dangerous) {
			return fmt.Errorf("configuration key contains potentially dangerous term: %s", key)
		}
	}

	return nil
}

// validatePathExists expands a path and checks if it exists
// Returns the expanded path and any error encountered
func validatePathExists(rawPath string) (string, error) {
	if rawPath == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	expandedPath, err := util.ExpandPath(rawPath)
	if err != nil {
		return "", fmt.Errorf("failed to expand file path %s: %w", rawPath, err)
	}

	// Check if file/symlink exists
	_, err = os.Lstat(expandedPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", expandedPath)
	}
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	return expandedPath, nil
}