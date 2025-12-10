package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/choru-k/dot-sync-manager/internal/symlink"
	"github.com/choru-k/dot-sync-manager/internal/util"
	"github.com/spf13/cobra"
)

var unlinkRestore bool

var unlinkCmd = &cobra.Command{
	Use:   "unlink <target-path>",
	Short: "Remove symlink and optionally restore original",
	Long: `Remove a symlink that was created by 'dsm link'.

The target-path should be the location of the symlink to remove.

If a backup exists and --restore is used, the original file will be restored.
Note: --restore matches backups by filename only. If you have multiple linked
files with the same name in different directories, ensure you restore the
correct one by checking the timestamp.

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

	// Find mapping for this target
	repoPath := ""
	for rp, tp := range mgr.ListMappings() {
		if tp == targetPath {
			repoPath = rp
			break
		}
	}

	if repoPath == "" {
		// Not in mappings - still try to remove if it's a symlink, but warn the user.
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

		// NOTE: Backup matching limitation
		// Currently matches by basename only (filepath.Base), which could
		// incorrectly match backups from different directories with the same filename.
		// Example: ~/project1/config.yaml and ~/project2/config.yaml both create
		// backups named "config.yaml" and could be mixed up during restore.
		// TODO(Phase B): Fix backup system to match by full target path.
		// See PR #100 review feedback for details.

		// Find most recent backup for this target
		targetName := filepath.Base(targetPath)
		var backupPath string
		matchCount := 0
		for _, b := range backups {
			if b.OriginalPath == targetName {
				if matchCount == 0 {
					backupPath = b.BackupPath
				}
				matchCount++
			}
		}

		if matchCount > 1 {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: found %d backups for '%s', using most recent\n", matchCount, targetName)
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
