package symlink

import (
	"fmt"
	"path/filepath"
	"strings"
)

// checkTargetConflict checks if targetPath is already mapped by another repo.
// excludeRepoPath allows excluding a specific repo (for updates).
// Returns the conflicting repo path and true if conflict exists, empty string and false otherwise.
func (m *Manager) checkTargetConflict(targetPath, excludeRepoPath string) (string, bool) {
	if m.cfg.Mappings == nil {
		return "", false
	}

	targetPathMap := make(map[string]string, len(m.cfg.Mappings))
	for repo, target := range m.cfg.Mappings {
		if repo != excludeRepoPath {
			targetPathMap[target] = repo
		}
	}
	existingRepo, exists := targetPathMap[targetPath]
	return existingRepo, exists
}

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

	// Check for duplicate targetPath
	if existingRepo, exists := m.checkTargetConflict(targetPath, ""); exists {
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

// UpdateMapping updates the target path for an existing mapping.
// Returns error if mapping doesn't exist, validation fails, or target conflicts.
func (m *Manager) UpdateMapping(repoPath, newTargetPath string) error {
	// Check mapping exists
	if m.cfg.Mappings == nil {
		return fmt.Errorf("no mappings exist")
	}
	currentTarget, exists := m.cfg.Mappings[repoPath]
	if !exists {
		return fmt.Errorf("mapping not found: %s", repoPath)
	}

	// Early exit if target unchanged (idempotency)
	if currentTarget == newTargetPath {
		return nil // No-op, avoid unnecessary validation + save
	}

	// Validate new target path
	if err := m.ValidateMapping(repoPath, newTargetPath); err != nil {
		return fmt.Errorf("invalid target path: %w", err)
	}

	// Check target not used by another mapping
	if existingRepo, exists := m.checkTargetConflict(newTargetPath, repoPath); exists {
		return fmt.Errorf("target path already mapped by %s: %s", existingRepo, newTargetPath)
	}

	// Update mapping
	m.cfg.Mappings[repoPath] = newTargetPath

	// Save config
	configPath := m.cfg.GetConfigPath()
	if err := m.cfg.SaveToFile(configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// RemoveMapping removes a mapping from the configuration.
// Returns error if mapping doesn't exist or config can't be saved.
func (m *Manager) RemoveMapping(repoPath string) error {
	// Check mappings exist
	if m.cfg.Mappings == nil {
		return fmt.Errorf("no mappings exist")
	}

	// Check mapping exists
	if _, exists := m.cfg.Mappings[repoPath]; !exists {
		return fmt.Errorf("mapping not found: %s", repoPath)
	}

	// Remove mapping
	delete(m.cfg.Mappings, repoPath)

	// Save config
	configPath := m.cfg.GetConfigPath()
	if err := m.cfg.SaveToFile(configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// ListMappings returns a copy of all mappings.
// Returns empty map (never nil) if no mappings exist.
// Returned map is a copy to prevent external modification.
func (m *Manager) ListMappings() map[string]string {
	// Handle nil or empty mappings
	if m.cfg.Mappings == nil {
		return make(map[string]string)
	}

	// Return copy to prevent modification
	result := make(map[string]string, len(m.cfg.Mappings))
	for k, v := range m.cfg.Mappings {
		result[k] = v
	}
	return result
}
