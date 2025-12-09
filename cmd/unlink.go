package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/choru-k/dot-sync-manager/internal/symlink"
	"github.com/spf13/cobra"
)

var unlinkRestore bool

var unlinkCmd = &cobra.Command{
	Use:   "unlink <target-path>",
	Short: "Remove symlink and optionally restore original",
	Long: `Remove a symlink that was created by 'dsm link'.

The target-path should be the location of the symlink to remove.
If a backup exists and --restore is used, the original file will be restored.

Example:
  dsm unlink ~/.bashrc
  dsm unlink ~/.bashrc --restore`,
	Args: cobra.ExactArgs(1),
	RunE: runUnlink,
}

func init() {
	rootCmd.AddCommand(unlinkCmd)
	unlinkCmd.Flags().BoolVarP(&unlinkRestore, "restore", "r", false, "Restore original file from backup")
}

func runUnlink(cmd *cobra.Command, args []string) error {
	targetPath := args[0]

	// Expand ~ in target path
	if len(targetPath) > 0 && targetPath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		if len(targetPath) == 1 {
			targetPath = home
		} else if targetPath[1] == filepath.Separator {
			targetPath = filepath.Join(home, targetPath[2:])
		}
	}

	// Make target absolute
	var err error
	targetPath, err = filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve target path: %w", err)
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

	// Find mapping for this target
	repoPath := ""
	for rp, tp := range mgr.ListMappings() {
		if tp == targetPath {
			repoPath = rp
			break
		}
	}

	if repoPath == "" {
		// Not in mappings - still try to remove if it's a symlink
		info, err := os.Lstat(targetPath)
		if os.IsNotExist(err) {
			return fmt.Errorf("target does not exist: %s", targetPath)
		}
		if err != nil {
			return fmt.Errorf("failed to check target: %w", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("target is not a symlink: %s", targetPath)
		}
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: target not found in mappings\n")
	}

	// Remove the symlink
	if err := mgr.RemoveLink(targetPath); err != nil {
		return fmt.Errorf("failed to remove symlink: %w", err)
	}

	// Restore from backup if requested
	if unlinkRestore {
		backups, err := mgr.ListBackups()
		if err != nil {
			return fmt.Errorf("failed to list backups: %w", err)
		}

		// Find most recent backup for this target
		targetName := filepath.Base(targetPath)
		var backupPath string
		for _, b := range backups {
			if b.OriginalPath == targetName {
				backupPath = b.BackupPath
				break // Already sorted by newest first
			}
		}

		if backupPath == "" {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: no backup found for %s\n", targetName)
		} else {
			if err := mgr.RestoreFromBackup(backupPath, targetPath); err != nil {
				return fmt.Errorf("failed to restore backup: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Restored from: %s\n", backupPath)
		}
	}

	// Remove mapping from config
	if repoPath != "" {
		if err := mgr.RemoveMapping(repoPath); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to remove mapping: %v\n", err)
		}
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed symlink: %s\n", targetPath)
	return nil
}
