// Package symlink provides symlink management functionality for DSM.
package symlink

import (
	"fmt"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

// Manager handles symlink operations for dotfile management.
type Manager struct {
	cfg       *config.SyncConfig
	backupDir string // Reserved for A.3 backup functionality - will be populated when backup features are implemented
}

// MappingStatus represents the status of a symlink mapping.
type MappingStatus struct {
	RepoPath   string // Relative path within dotfiles repo
	TargetPath string // Absolute path on filesystem
	Status     string // "valid", "broken", "missing", "not_symlink"
	Error      string // Error message if any
}

// NewManager creates a new symlink manager.
func NewManager(cfg *config.SyncConfig) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("symlink: config is required [SYMLINK_CONFIG_REQUIRED]")
	}

	return &Manager{
		cfg:       cfg,
		backupDir: "", // Reserved for A.3 backup functionality - will be populated when backup features are implemented
	}, nil
}
