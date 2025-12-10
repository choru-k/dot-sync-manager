// Package cmd implements CLI commands for the dotfile sync manager.
// This file defines the link command for creating symlinks from repository files to target locations.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/choru-k/dot-sync-manager/internal/symlink"
	"github.com/choru-k/dot-sync-manager/internal/util"
	"github.com/spf13/cobra"
)

var (
	linkForce    bool
	linkNoBackup bool
)

var linkCmd = &cobra.Command{
	Use:   "link <repo-file> <target-path>",
	Short: "Create symlink from repo file to target",
	Long: `Create a symlink from a file in your dotfiles repository to a target location.

The repo-file should be a path relative to your dotfiles repository.
The target-path should be an absolute path where the symlink will be created.

Example:
  dsm link .bashrc ~/.bashrc
  dsm link config/nvim ~/.config/nvim`,
	Args: cobra.ExactArgs(2),
	RunE: runLink,
}

func init() {
	rootCmd.AddCommand(linkCmd)
	linkCmd.Flags().BoolVarP(&linkForce, "force", "f", false, "Overwrite existing file without prompt")
	linkCmd.Flags().BoolVar(&linkNoBackup, "no-backup", false, "Skip backup of existing file")
}

func runLink(cmd *cobra.Command, args []string) error {
	repoFile := args[0]
	targetPath := args[1]

	// Expand ~ in target path and make absolute
	var err error
	targetPath, err = util.ExpandPath(targetPath)
	if err != nil {
		return fmt.Errorf("failed to expand target path: %w", err)
	}

	// Validate result is absolute (per CODING_RULES.md)
	if !filepath.IsAbs(targetPath) {
		return fmt.Errorf("target path must be absolute: %s", targetPath)
	}

	// Load config
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create symlink manager
	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create symlink manager: %w", err)
	}

	// Validate source file exists in repository BEFORE any operations
	sourcePath := filepath.Join(cfg.Git.RepoPath, repoFile)
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf(
			"source file does not exist in repository: %s\n"+
				"Expected path: %s\n"+
				"Hint: Check repository path with 'dsm config' and verify file exists",
			repoFile, sourcePath)
	} else if err != nil {
		return fmt.Errorf("failed to check source file: %w", err)
	}

	// Check if target already exists
	info, err := os.Lstat(targetPath)
	if err == nil {
		if !linkForce {
			typeStr := "file"
			if info.IsDir() {
				typeStr = "directory"
			} else if info.Mode()&os.ModeSymlink != 0 {
				typeStr = "symlink"
			}
			return fmt.Errorf("target already exists (%s): %s (use --force to overwrite)", typeStr, targetPath)
		}

		// Transaction: Backup → Remove → Create (with rollback on failure)
		var backupPath string
		if !linkNoBackup {
			backupPath, err = mgr.BackupExisting(targetPath)
			if err != nil {
				return fmt.Errorf("failed to backup existing file: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Backed up to: %s\n", backupPath)
		}

		// Remove existing (point of no return)
		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(targetPath); err != nil {
				return fmt.Errorf("failed to remove existing symlink: %w", err)
			}
		} else {
			if err := os.RemoveAll(targetPath); err != nil {
				return fmt.Errorf("failed to remove existing file/directory: %w", err)
			}
		}

		// Create symlink with rollback on failure
		if err := mgr.CreateLink(repoFile, targetPath); err != nil {
			// ROLLBACK: Restore from backup if available
			if backupPath != "" {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "ERROR: Symlink creation failed: %v\n", err)
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Attempting to restore original file from backup...\n")

				if restoreErr := mgr.RestoreFromBackup(backupPath, targetPath); restoreErr != nil {
					// Double failure - both symlink and restore failed
					return fmt.Errorf(
						"CRITICAL: symlink creation failed AND automatic restoration failed.\n"+
							"Original file backed up at: %s\n"+
							"Symlink error: %w\n"+
							"Restore error: %v\n"+
							"Please manually restore from backup location",
						backupPath, err, restoreErr)
				}

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Successfully restored original file from backup.\n")
				return fmt.Errorf("symlink creation failed (original file restored): %w", err)
			}

			// No backup available (--no-backup used)
			return fmt.Errorf("symlink creation failed (no backup to restore): %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check target path: %w", err)
	} else {
		// Target doesn't exist, create symlink directly
		if err := mgr.CreateLink(repoFile, targetPath); err != nil {
			return fmt.Errorf("failed to create symlink: %w", err)
		}
	}

	// Add mapping to config
	if err := mgr.AddMapping(repoFile, targetPath); err != nil {
		// Symlink created but mapping failed - warn but don't fail
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to add mapping: %v\n", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created symlink: %s -> %s\n", targetPath, repoFile)
	return nil
}
