// Package symlink provides symlink management functionality for DSM.
package symlink

import (
	"fmt"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

// Manager handles symlink operations for dotfile management.
type Manager struct {
	cfg       *config.SyncConfig
	backupDir string // TODO(A.3): Reserved for backup functionality - will be populated when backup features are implemented
}

// MappingState represents the state of a symlink mapping.
type MappingState string

const (
	StateValid      MappingState = "valid"
	StateBroken     MappingState = "broken"
	StateMissing    MappingState = "missing"
	StateNotSymlink MappingState = "not_symlink"
)

// MappingStatus represents the status of a symlink mapping.
type MappingStatus struct {
	RepoPath   string       // Relative path within dotfiles repo
	TargetPath string       // Absolute path on filesystem
	Status     MappingState // Current mapping state
	Error      string       // Error message if any
}

// NewManager creates a new symlink manager.
func NewManager(cfg *config.SyncConfig) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("symlink: config is required [SYMLINK_CONFIG_REQUIRED]")
	}

	return &Manager{
		cfg:       cfg,
		backupDir: DefaultBackupDir(),
	}, nil
}

// SetBackupDir sets a custom backup directory (useful for testing).
func (m *Manager) SetBackupDir(dir string) {
	m.backupDir = dir
}

// GetBackupDir returns the current backup directory.
func (m *Manager) GetBackupDir() string {
	return m.backupDir
}
