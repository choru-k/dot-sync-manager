// Package cmd provides command-line interface commands for the Dotfile Sync Manager.
//
// This package contains all CLI commands including initialization, file management,
// configuration handling, and system operations. The commands are built using
// the Cobra library and follow standard CLI conventions.
//
// Key features:
// - Secure file validation and path handling
// - Configuration management with nested key access
// - Interactive prompts with safety confirmations
// - Comprehensive error handling and user guidance
// - Cross-platform compatibility (Windows, macOS, Linux)
//
// Security considerations:
// - All file paths are expanded and validated before use
// - Sensitive file detection helps prevent accidental credential exposure
// - Editor commands are validated against an allowlist to prevent injection
// - TOCTOU vulnerabilities are documented and mitigated where possible
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

	// Validate that the section is valid (whitelist approach)
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return fmt.Errorf("configuration key must have at least one section and field (e.g., 'machine.name')")
	}

	section := parts[0]
	validSections := map[string]bool{
		"machine":            true,
		"git":                true,
		"sync":               true,
		"notifications":      true,
		"conflict_resolution": true,
		"ui":                 true,
		"advanced":           true,
	}

	if !validSections[section] {
		return fmt.Errorf("invalid configuration section: %s (valid sections: machine, git, sync, notifications, conflict_resolution, ui, advanced)", section)
	}

	// Prevent configuration keys that could be dangerous
	// Use specific patterns to avoid false positives with legitimate fields
	// Allow git.password and git.ssh_key_passphrase as they are required per PRD
	dangerousPatterns := []string{
		`(^|\.|_)passwd($|\.|_)`, `(^|\.|_)secret($|\.|_)`, `(^|\.|_)private_key($|\.|_)`, // blocked
		`(^|\.|_)credential($|\.|_)`, `(^|\.|_)cert($|\.|_)`, `(^|\.|_)token($|\.|_)`, // blocked
		`(^|\.|_)api_key($|\.|_)`, `(^|\.|_)auth_key($|\.|_)`, // blocked
		// Note: git.password and git.ssh_key_passphrase are explicitly allowed per PRD requirements
	}

	keyLower := strings.ToLower(key)
	for _, pattern := range dangerousPatterns {
		matched, err := regexp.MatchString(pattern, keyLower)
		if err == nil && matched {
			return fmt.Errorf("configuration key contains potentially dangerous term: %s", key)
		}
	}

	return nil
}

// validatePathExists expands a path and checks if it exists
// Returns the expanded path and any error encountered
//
// SECURITY NOTE: This function has a minor TOCTOU (Time-of-Check-Time-of-Use)
// vulnerability window between path expansion and the existence check.
// For most use cases this is acceptable, but for high-security scenarios,
// consider using atomic file operations from util/path.go.
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