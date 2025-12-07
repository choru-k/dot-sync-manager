package symlink_test

import (
	"path/filepath"
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

func TestManager_AddMapping_Success(t *testing.T) {
	repoDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.json")

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.ConfigPath = configPath

	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	// Create cross-platform test paths
	homeDir := t.TempDir()
	bashrcPath := filepath.Join(homeDir, ".bashrc")
	vimrcPath := filepath.Join(homeDir, ".vimrc")

	// Add first mapping
	err = mgr.AddMapping(".bashrc", bashrcPath)
	if err != nil {
		t.Fatalf("AddMapping() failed: %v", err)
	}

	// Verify mapping was added in memory
	if cfg.Mappings[".bashrc"] != bashrcPath {
		t.Errorf("Mapping not found in config: got %v", cfg.Mappings)
	}

	// Add second mapping
	err = mgr.AddMapping(".vimrc", vimrcPath)
	if err != nil {
		t.Fatalf("AddMapping() second failed: %v", err)
	}

	// Verify both mappings exist in memory
	if len(cfg.Mappings) != 2 {
		t.Errorf("Expected 2 mappings in memory, got %d", len(cfg.Mappings))
	}

	// Verify persistence by reloading from disk
	reloadedCfg, err := config.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to reload config from file: %v", err)
	}
	if len(reloadedCfg.Mappings) != 2 {
		t.Errorf("Expected 2 mappings after reloading from file, got %d", len(reloadedCfg.Mappings))
	}
	if reloadedCfg.Mappings[".vimrc"] != vimrcPath {
		t.Errorf("Second mapping not found after reloading")
	}
}

func TestManager_AddMapping_Conflict(t *testing.T) {
	repoDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.json")

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}
	cfg.Git.RepoPath = repoDir
	cfg.ConfigPath = configPath

	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	// Create cross-platform test paths
	homeDir := t.TempDir()
	bashrcPath := filepath.Join(homeDir, ".bashrc")
	otherPath := filepath.Join(homeDir, "other", "path")

	// Add initial mapping
	err = mgr.AddMapping(".bashrc", bashrcPath)
	if err != nil {
		t.Fatalf("AddMapping() setup failed: %v", err)
	}

	// Try to add duplicate repo path
	err = mgr.AddMapping(".bashrc", otherPath)
	if err == nil {
		t.Error("Expected error for duplicate repo path")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' error, got: %v", err)
	}

	// Try to add duplicate target path
	err = mgr.AddMapping(".other", bashrcPath)
	if err == nil {
		t.Error("Expected error for duplicate target path")
	}
	if !strings.Contains(err.Error(), "already mapped") {
		t.Errorf("Expected 'already mapped' error, got: %v", err)
	}
}

func TestManager_UpdateMapping(t *testing.T) {
	// Create cross-platform test paths
	homeDir := t.TempDir()
	bashrcPath := filepath.Join(homeDir, ".bashrc")
	newBashrcPath := filepath.Join(homeDir, "new", ".bashrc")
	vimrcPath := filepath.Join(homeDir, ".vimrc")

	tests := map[string]struct {
		setupMappings map[string]string
		repoPath      string
		newTargetPath string
		wantErr       bool
		errContains   string
	}{
		"when mapping exists": {
			setupMappings: map[string]string{".bashrc": bashrcPath},
			repoPath:      ".bashrc",
			newTargetPath: newBashrcPath,
			wantErr:       false,
		},
		"when mapping not found": {
			setupMappings: map[string]string{},
			repoPath:      ".nonexistent",
			newTargetPath: newBashrcPath,
			wantErr:       true,
			errContains:   "not found",
		},
		"when target conflicts": {
			setupMappings: map[string]string{".bashrc": bashrcPath, ".vimrc": vimrcPath},
			repoPath:      ".vimrc",
			newTargetPath: bashrcPath, // Conflict with .bashrc
			wantErr:       true,
			errContains:   "already mapped",
		},
		"when path invalid": {
			setupMappings: map[string]string{".bashrc": bashrcPath},
			repoPath:      ".bashrc",
			newTargetPath: "relative/path",
			wantErr:       true,
			errContains:   "invalid",
		},
		"when target unchanged (idempotent)": {
			setupMappings: map[string]string{".bashrc": bashrcPath},
			repoPath:      ".bashrc",
			newTargetPath: bashrcPath, // Same value - no-op update
			wantErr:       false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			repoDir := t.TempDir()
			configPath := filepath.Join(t.TempDir(), "config.json")

			cfg, err := config.DefaultConfig()
			if err != nil {
				t.Fatalf("DefaultConfig() failed: %v", err)
			}
			cfg.Git.RepoPath = repoDir
			cfg.ConfigPath = configPath

			// Setup initial mappings
			if len(tt.setupMappings) > 0 {
				cfg.Mappings = make(map[string]string)
				for repo, target := range tt.setupMappings {
					cfg.Mappings[repo] = target
				}
				if err := cfg.SaveToFile(configPath); err != nil {
					t.Fatalf("Failed to save initial config: %v", err)
				}
			}

			mgr, err := symlink.NewManager(cfg)
			if err != nil {
				t.Fatalf("NewManager() failed: %v", err)
			}

			// Execute UpdateMapping
			err = mgr.UpdateMapping(tt.repoPath, tt.newTargetPath)

			// Assert error or success
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Expected error containing %q, got nil", tt.errContains)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}

				// Verify mapping updated in memory
				if cfg.Mappings[tt.repoPath] != tt.newTargetPath {
					t.Errorf("Mapping not updated: expected %s, got %s", tt.newTargetPath, cfg.Mappings[tt.repoPath])
				}

				// Verify persistence by reloading from disk
				reloadedCfg, err := config.LoadFromFile(configPath)
				if err != nil {
					t.Fatalf("Failed to reload config: %v", err)
				}
				if reloadedCfg.Mappings[tt.repoPath] != tt.newTargetPath {
					t.Errorf("Mapping not persisted: expected %s, got %s", tt.newTargetPath, reloadedCfg.Mappings[tt.repoPath])
				}
			}
		})
	}
}

func TestManager_RemoveMapping(t *testing.T) {
	// Create cross-platform test path
	homeDir := t.TempDir()
	bashrcPath := filepath.Join(homeDir, ".bashrc")

	tests := map[string]struct {
		setupMappings map[string]string
		nilMappings   bool
		repoPath      string
		wantErr       bool
		errContains   string
	}{
		"when mapping exists": {
			setupMappings: map[string]string{".bashrc": bashrcPath},
			repoPath:      ".bashrc",
			wantErr:       false,
		},
		"when mapping not found": {
			setupMappings: map[string]string{},
			repoPath:      ".nonexistent",
			wantErr:       true,
			errContains:   "not found",
		},
		"when mappings nil": {
			nilMappings: true,
			repoPath:    ".bashrc",
			wantErr:     true,
			errContains: "no mappings",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			repoDir := t.TempDir()
			configPath := filepath.Join(t.TempDir(), "config.json")

			cfg, err := config.DefaultConfig()
			if err != nil {
				t.Fatalf("DefaultConfig() failed: %v", err)
			}
			cfg.Git.RepoPath = repoDir
			cfg.ConfigPath = configPath

			// Setup initial mappings
			if tt.nilMappings {
				cfg.Mappings = nil
			} else {
				for repo, target := range tt.setupMappings {
					if cfg.Mappings == nil {
						cfg.Mappings = make(map[string]string)
					}
					cfg.Mappings[repo] = target
				}
				if len(tt.setupMappings) > 0 {
					if err := cfg.SaveToFile(configPath); err != nil {
						t.Fatalf("Failed to save initial config: %v", err)
					}
				}
			}

			mgr, err := symlink.NewManager(cfg)
			if err != nil {
				t.Fatalf("NewManager() failed: %v", err)
			}

			// Execute RemoveMapping
			err = mgr.RemoveMapping(tt.repoPath)

			// Assert error or success
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Expected error containing %q, got nil", tt.errContains)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}

				// Verify mapping no longer exists in memory
				if _, exists := cfg.Mappings[tt.repoPath]; exists {
					t.Error("Mapping should not exist after removal")
				}

				// Verify persistence by reloading from disk
				reloadedCfg, err := config.LoadFromFile(configPath)
				if err != nil {
					t.Fatalf("Failed to reload config: %v", err)
				}
				if _, exists := reloadedCfg.Mappings[tt.repoPath]; exists {
					t.Error("Mapping should not be persisted after removal")
				}
			}
		})
	}
}

func TestManager_ListMappings(t *testing.T) {
	// Create cross-platform test paths
	homeDir := t.TempDir()
	bashrcPath := filepath.Join(homeDir, ".bashrc")
	vimrcPath := filepath.Join(homeDir, ".vimrc")

	tests := map[string]struct {
		setupMappings    map[string]string
		testImmutability bool
		expectedCount    int
		expectedNonNil   bool
	}{
		"when mappings empty": {
			setupMappings:  map[string]string{},
			expectedCount:  0,
			expectedNonNil: true, // Should return empty map, not nil
		},
		"when mappings exist": {
			setupMappings: map[string]string{
				".bashrc": bashrcPath,
				".vimrc":  vimrcPath,
			},
			expectedCount: 2,
		},
		"when external modification attempted": {
			setupMappings: map[string]string{
				".bashrc": bashrcPath,
			},
			testImmutability: true,
			expectedCount:    1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			repoDir := t.TempDir()
			configPath := filepath.Join(t.TempDir(), "config.json")

			cfg, err := config.DefaultConfig()
			if err != nil {
				t.Fatalf("DefaultConfig() failed: %v", err)
			}
			cfg.Git.RepoPath = repoDir
			cfg.ConfigPath = configPath

			mgr, err := symlink.NewManager(cfg)
			if err != nil {
				t.Fatalf("NewManager() failed: %v", err)
			}

			// Add initial mappings
			for repo, target := range tt.setupMappings {
				err = mgr.AddMapping(repo, target)
				if err != nil {
					t.Fatalf("AddMapping(%s) setup failed: %v", repo, err)
				}
			}

			// Test ListMappings
			result := mgr.ListMappings()

			// Verify non-nil for empty case
			if tt.expectedNonNil && result == nil {
				t.Error("ListMappings() should return empty map, not nil")
			}

			// Verify count
			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d mappings, got %d", tt.expectedCount, len(result))
			}

			// Verify correct mappings when they exist
			if !tt.testImmutability {
				for repo, expectedTarget := range tt.setupMappings {
					if result[repo] != expectedTarget {
						t.Errorf("Expected %s → %s, got %s", repo, expectedTarget, result[repo])
					}
				}
			}

			// Test immutability (defensive copy)
			if tt.testImmutability {
				result[".vimrc"] = "/modified/path"
				result2 := mgr.ListMappings()

				if len(result2) != tt.expectedCount {
					t.Errorf("External modification affected internal state: expected %d mapping, got %d", tt.expectedCount, len(result2))
				}
				if _, exists := result2[".vimrc"]; exists {
					t.Error("External modification affected internal state: .vimrc should not exist")
				}
			}
		})
	}
}
