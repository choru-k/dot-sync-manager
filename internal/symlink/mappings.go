package symlink

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateMapping validates a mapping entry for correctness.
// repoPath must be relative (within repository), targetPath must be absolute.
// Returns error if validation fails, nil otherwise.
func (m *Manager) ValidateMapping(repoPath, targetPath string) error {
	// Validation 1: repoPath must be relative (no leading /)
	if repoPath == "" || filepath.IsAbs(repoPath) {
		return fmt.Errorf("repoPath must be a non-empty relative path, but got: '%s'", repoPath)
	}

	// Validation 2: repoPath cannot escape repository (no ..)
	cleanPath := filepath.Clean(repoPath)
	if strings.HasPrefix(cleanPath, "..") {
		return fmt.Errorf("repoPath cannot escape repository: %s", repoPath)
	}

	// Validation 3: targetPath must be absolute
	if !filepath.IsAbs(targetPath) {
		return fmt.Errorf("targetPath must be an absolute path, but got: '%s'", targetPath)
	}

	// Validation 4: No circular reference (target cannot be inside repo)
	// Note: On Windows, filepath.Rel fails for cross-drive paths (e.g., C:\ vs D:\),
	// which correctly indicates no circular reference is possible.
	repoAbsPath := filepath.Clean(m.cfg.Git.RepoPath)
	cleanTarget := filepath.Clean(targetPath)

	relPath, err := filepath.Rel(repoAbsPath, cleanTarget)
	if err == nil && !strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("targetPath cannot be inside repository: %s is inside %s",
			targetPath, repoAbsPath)
	}

	return nil
}

// AddMapping adds a new mapping to the configuration.
// Both paths are validated before adding. Returns error if:
// - Validation fails (invalid paths)
// - Mapping already exists for repoPath
// - Target path is already used by another mapping
// - Config cannot be saved
func (m *Manager) AddMapping(repoPath, targetPath string) error {
	// Validate paths first
	if err := m.ValidateMapping(repoPath, targetPath); err != nil {
		return fmt.Errorf("invalid mapping: %w", err)
	}

	// Initialize mappings if needed
	if m.cfg.Mappings == nil {
		m.cfg.Mappings = make(map[string]string)
	}

	// Check for duplicate repoPath
	if _, exists := m.cfg.Mappings[repoPath]; exists {
		return fmt.Errorf("mapping already exists for: %s", repoPath)
	}

	// Check for duplicate targetPath (use map for O(1) lookup per style guide Rule 12)
	targetPathMap := make(map[string]string, len(m.cfg.Mappings))
	for repo, target := range m.cfg.Mappings {
		targetPathMap[target] = repo
	}
	if existingRepo, exists := targetPathMap[targetPath]; exists {
		return fmt.Errorf("target path already mapped by %s: %s", existingRepo, targetPath)
	}

	// Add mapping
	m.cfg.Mappings[repoPath] = targetPath

	// Save config
	configPath := m.cfg.GetConfigPath()
	if err := m.cfg.SaveToFile(configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}
