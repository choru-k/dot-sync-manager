package symlink_test

import (
	"strings"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/symlink"
)

func TestManager_ValidateMapping(t *testing.T) {
	// Setup temp directory for repo path
	repoDir := t.TempDir()

	tests := map[string]struct {
		repoPath    string
		targetPath  string
		wantErr     bool
		errContains string
	}{
		// Happy path
		"valid mapping": {
			repoPath:   ".bashrc",
			targetPath: "/home/user/.bashrc",
			wantErr:    false,
		},
		"valid nested path": {
			repoPath:   "config/nvim/init.vim",
			targetPath: "/home/user/.config/nvim/init.vim",
			wantErr:    false,
		},

		// Validation 1: repoPath must be relative
		"absolute repoPath": {
			repoPath:    "/etc/config",
			targetPath:  "/home/user/.config",
			wantErr:     true,
			errContains: "must be a non-empty relative path",
		},

		// Validation 2: repoPath cannot escape repository
		"repoPath with parent reference": {
			repoPath:    "../outside/file",
			targetPath:  "/home/user/.config",
			wantErr:     true,
			errContains: "escape repository",
		},
		"repoPath with embedded parent": {
			repoPath:    "config/../../escape",
			targetPath:  "/home/user/.config",
			wantErr:     true,
			errContains: "escape repository",
		},

		// Validation 3: targetPath must be absolute
		"relative targetPath": {
			repoPath:    ".bashrc",
			targetPath:  "home/user/.bashrc",
			wantErr:     true,
			errContains: "must be an absolute path",
		},

		// Validation 4: circular reference (target inside repo)
		"targetPath inside repo": {
			repoPath:    ".bashrc",
			targetPath:  repoDir + "/inside/file",
			wantErr:     true,
			errContains: "inside repository",
		},

		// Edge cases
		"empty repoPath": {
			repoPath:    "",
			targetPath:  "/home/user/.bashrc",
			wantErr:     true,
			errContains: "must be a non-empty relative path",
		},
		"empty targetPath": {
			repoPath:    ".bashrc",
			targetPath:  "",
			wantErr:     true,
			errContains: "must be an absolute path",
		},
		"repoPath with dot prefix": {
			repoPath:   "./.bashrc",
			targetPath: "/home/user/.bashrc",
			wantErr:    false,
		},
		"repoPath is just dot": {
			repoPath:   ".",
			targetPath: "/home/user/.bashrc",
			wantErr:    false,
		},
		"repoPath with double slash": {
			repoPath:   "dotfiles//config",
			targetPath: "/home/user/.config",
			wantErr:    false,
		},
		"targetPath with trailing slash": {
			repoPath:   ".bashrc",
			targetPath: "/home/user/.bashrc/",
			wantErr:    false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Use DefaultConfig as per project patterns
			cfg, err := config.DefaultConfig()
			if err != nil {
				t.Fatalf("DefaultConfig() failed: %v", err)
			}
			cfg.Git.RepoPath = repoDir

			mgr, err := symlink.NewManager(cfg)
			if err != nil {
				t.Fatalf("NewManager failed: %v", err)
			}

			err = mgr.ValidateMapping(tt.repoPath, tt.targetPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateMapping() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateMapping() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateMapping() unexpected error = %v", err)
				}
			}
		})
	}
}
