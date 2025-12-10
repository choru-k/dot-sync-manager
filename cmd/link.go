package cmd

import (
	"fmt"
	"os"

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

	// Check if target already exists
	info, err := os.Lstat(targetPath)
	if err == nil {
		if !linkForce {
			return fmt.Errorf("target already exists: %s (use --force to overwrite)", targetPath)
		}

		// Backup existing file unless --no-backup
		if !linkNoBackup {
			backupPath, err := mgr.BackupExisting(targetPath)
			if err != nil {
				return fmt.Errorf("failed to backup existing file: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Backed up to: %s\n", backupPath)
		}

		// Remove existing
		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(targetPath); err != nil {
				return fmt.Errorf("failed to remove existing symlink: %w", err)
			}
		} else {
			if err := os.RemoveAll(targetPath); err != nil {
				return fmt.Errorf("failed to remove existing file/directory: %w", err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check target path: %w", err)
	}

	// Create symlink
	if err := mgr.CreateLink(repoFile, targetPath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	// Add mapping to config
	if err := mgr.AddMapping(repoFile, targetPath); err != nil {
		// Symlink created but mapping failed - warn but don't fail
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to add mapping: %v\n", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created symlink: %s -> %s\n", targetPath, repoFile)
	return nil
}
