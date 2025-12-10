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
		return fmt.Errorf(
			"symlink: source file does not exist in repo: %s\n"+
				"Expected full path: %s\n"+
				"Hint: Use 'dsm list' to see available files [SYMLINK_SOURCE_NOT_FOUND]",
			source, sourcePath)
	}

	// Validate target parent directory exists
	targetDir := filepath.Dir(target)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return fmt.Errorf("symlink: target parent directory does not exist: %s [SYMLINK_TARGET_PARENT_NOT_FOUND]", targetDir)
	}

	// Check if target already exists (use Lstat to avoid following symlinks)
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("symlink: target already exists: %s [SYMLINK_TARGET_EXISTS]", target)
	}

	// Create symlink
	if err := os.Symlink(sourcePath, target); err != nil {
		return fmt.Errorf("symlink: failed to create symlink: %w [SYMLINK_CREATE_FAILED]", err)
	}

	return nil
}

// RemoveLink removes a symlink at the target path.
// Returns error if target is not a symlink.
func (m *Manager) RemoveLink(target string) error {
	// Check if target exists
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("symlink: target does not exist: %s: %w [SYMLINK_TARGET_NOT_FOUND]", target, err)
		}
		return fmt.Errorf("symlink: failed to stat target: %w [SYMLINK_STAT_FAILED]", err)
	}

	// Verify it's a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("symlink: target is not a symlink: %s [SYMLINK_NOT_A_SYMLINK]", target)
	}

	// Remove the symlink
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("symlink: failed to remove symlink: %w [SYMLINK_REMOVE_FAILED]", err)
	}

	return nil
}

// VerifyMappings checks all configured mappings and returns their status.
// Returns slice of MappingStatus showing state of each mapping:
// - "valid": Symlink exists, points to correct source, source file exists
// - "broken": Symlink exists but source missing or points to wrong location
// - "missing": Symlink doesn't exist at target path
// - "not_symlink": Regular file exists at target path instead of symlink
func (m *Manager) VerifyMappings() []MappingStatus {
	// Handle nil or empty mappings
	if m.cfg.Mappings == nil {
		return nil
	}

	results := make([]MappingStatus, 0, len(m.cfg.Mappings))

	for repoPath, targetPath := range m.cfg.Mappings {
		status := MappingStatus{
			RepoPath:   repoPath,
			TargetPath: targetPath,
		}

		// Check if target exists (use Lstat to avoid following symlinks)
		info, err := os.Lstat(targetPath)
		if os.IsNotExist(err) {
			status.Status = StateMissing
			status.Error = "symlink does not exist"
			results = append(results, status)
			continue
		}
		if err != nil {
			status.Status = StateBroken
			status.Error = err.Error()
			results = append(results, status)
			continue
		}

		// Check if it's a symlink
		if info.Mode()&os.ModeSymlink == 0 {
			status.Status = StateNotSymlink
			status.Error = "target exists but is not a symlink"
			results = append(results, status)
			continue
		}

		// Check if symlink points to correct source
		linkDest, err := os.Readlink(targetPath)
		if err != nil {
			status.Status = StateBroken
			status.Error = fmt.Sprintf("cannot read symlink: %v", err)
			results = append(results, status)
			continue
		}

		// Resolve relative symlinks (Rule 52: Relative symlinks must be resolved)
		resolvedLinkDest := linkDest
		if !filepath.IsAbs(linkDest) {
			resolvedLinkDest = filepath.Clean(filepath.Join(filepath.Dir(targetPath), linkDest))
		}

		expectedSource := filepath.Join(m.cfg.Git.RepoPath, repoPath)
		if resolvedLinkDest != expectedSource {
			status.Status = StateBroken
			status.Error = fmt.Sprintf("symlink points to %s, expected %s", linkDest, expectedSource)
			results = append(results, status)
			continue
		}

		// Check if source file exists in repo
		if _, err := os.Stat(expectedSource); os.IsNotExist(err) {
			status.Status = StateBroken
			status.Error = "source file does not exist in repo"
			results = append(results, status)
			continue
		}

		// All checks passed
		status.Status = StateValid
		results = append(results, status)
	}

	return results
}
