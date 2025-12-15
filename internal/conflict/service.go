package conflict

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
)

// NewService creates a new conflict service.
func NewService(gitMgr *gitmanager.GitManager, cfg *config.SyncConfig) *Service {
	repoPath := cfg.Git.RepoPath
	conflictDir := filepath.Join(repoPath, ".dsm", "conflicts")

	return &Service{
		gitMgr:      gitMgr,
		cfg:         cfg,
		conflictDir: conflictDir,
	}
}

// GetConflicts returns all active conflicts.
func (s *Service) GetConflicts() []ConflictInfo {
	conflicts := []ConflictInfo{}

	// Read conflict directory
	entries, err := os.ReadDir(s.conflictDir)
	if os.IsNotExist(err) {
		return conflicts
	}
	if err != nil {
		return conflicts
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Read metadata.json for each conflict
		metadataPath := filepath.Join(s.conflictDir, entry.Name(), "metadata.json")
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			continue
		}

		var info ConflictInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		conflicts = append(conflicts, info)
	}

	return conflicts
}

// HasConflicts returns true if there are any active conflicts.
func (s *Service) HasConflicts() bool {
	entries, err := os.ReadDir(s.conflictDir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// GetConflictDir returns the conflict directory path.
func (s *Service) GetConflictDir() string {
	return s.conflictDir
}

// GetConflictDetails returns the full content of conflicting versions.
func (s *Service) GetConflictDetails(file string) (*ConflictDetails, error) {
	conflictPath := filepath.Join(s.conflictDir, file)

	// Check if conflict exists
	if _, err := os.Stat(conflictPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("conflict not found: %s", file)
	}

	details := &ConflictDetails{
		File:       file,
		LocalPath:  filepath.Join(conflictPath, "local"),
		RemotePath: filepath.Join(conflictPath, "remote"),
		BasePath:   filepath.Join(conflictPath, "base"),
	}

	// Read local version
	if data, err := os.ReadFile(details.LocalPath); err == nil {
		details.LocalContent = data
	}

	// Read remote version
	if data, err := os.ReadFile(details.RemotePath); err == nil {
		details.RemoteContent = data
	}

	// Read base version (optional)
	if data, err := os.ReadFile(details.BasePath); err == nil {
		details.BaseContent = data
	}

	return details, nil
}
