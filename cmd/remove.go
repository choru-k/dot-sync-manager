package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/util"
	"github.com/spf13/cobra"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove <filepath>",
	Short: "Remove file from dotfiles tracking",
	Long: `Remove a file from dotfiles tracking. The symlink will be removed and the original
file in the dotfiles repository will be preserved by default.

Examples:
  dsm remove ~/.bashrc
  dsm remove ~/.vimrc
  dsm remove ~/.config/nvim/init.vim`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

var (
	keepRepoFile   bool
	deleteAll     bool
	removeNoCommit bool
)

func init() {
	rootCmd.AddCommand(removeCmd)
	removeCmd.Flags().BoolVar(&keepRepoFile, "keep-repo", true, "Keep file in dotfiles repository (default)")
	removeCmd.Flags().BoolVar(&deleteAll, "delete-all", false, "Delete file from both home and repository")
	removeCmd.Flags().BoolVar(&removeNoCommit, "no-commit", false, "Skip automatic git commit of removed files")
}

func runRemove(cmd *cobra.Command, args []string) error {
	// Validate flags
	if deleteAll && keepRepoFile {
		return fmt.Errorf("cannot use both --keep-repo and --delete-all flags")
	}

	filePath, err := validateRemoveTarget(args[0])
	if err != nil {
		return err
	}

	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	mappingKey, trackedFile, isTracked, err := findTrackedFile(cfg, filePath)
	if err != nil {
		return err
	}

	if !isTracked {
		return fmt.Errorf("file is not tracked by dotfiles manager: %s", filePath)
	}

	// Confirm removal operation
	if !confirmRemoval(filePath, trackedFile, mappingKey) {
		fmt.Println("Operation cancelled")
		return nil
	}

	// Execute removal
	if err := executeRemoval(cfg, filePath, trackedFile, mappingKey); err != nil {
		return err
	}

	// Update configuration
	if err := updateConfigAfterRemoval(cfg, mappingKey); err != nil {
		return fmt.Errorf("removed files but failed to update configuration: %w", err)
	}

	fmt.Printf("✅ Removed file from dotfiles tracking\n")
	fmt.Printf("🔗 Symlink removed: %s\n", filePath)
	if !deleteAll {
		fmt.Printf("📄 File restored to original location: %s\n", filePath)
	} else {
		fmt.Printf("📄 File deleted from repository: %s\n", trackedFile)
	}
	fmt.Printf("📂 Repository: %s\n", cfg.Git.RepoPath)

	// Perform git operations unless --no-commit flag is specified
	if !removeNoCommit {
		if err := commitRemovedFile(cmd, cfg, trackedFile, filePath); err != nil {
			// Git operation failed, but file operations succeeded
			fmt.Printf("\n⚠️  Warning: Git commit failed: %v\n", err)
			fmt.Printf("📝 Note: Changes were made but not committed. Run 'dsm sync' or wait for auto-sync to commit.\n")
		} else {
			fmt.Printf("✅ Committed changes to git repository\n")
		}
	} else {
		fmt.Printf("\n📝 Note: --no-commit specified. Run 'dsm sync' or wait for auto-sync to commit changes.\n")
	}

	return nil
}

func validateRemoveTarget(rawPath string) (string, error) {
	expandedPath, err := util.ExpandPath(rawPath)
	if err != nil {
		return "", fmt.Errorf("failed to expand file path %s: %w", rawPath, err)
	}

	// Check if file/symlink exists
	fileInfo, err := os.Lstat(expandedPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", expandedPath)
	}
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if it's a symlink
	if fileInfo.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("file is not a symlink: %s\nHint: 'dsm remove' only removes symlinks created by 'dsm add'", expandedPath)
	}

	return expandedPath, nil
}

func findTrackedFile(cfg *config.SyncConfig, symlinkPath string) (string, string, bool, error) {
	absSymlink, err := filepath.Abs(symlinkPath)
	if err != nil {
		return "", "", false, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if symlink target is in our mappings
	for sourceRepoPath, targetHomePath := range cfg.Mappings {
		absTarget, err := filepath.Abs(targetHomePath)
		if err != nil {
			continue // skip this mapping, continue checking others
		}

		if absSymlink == absTarget {
			absRepo, err := filepath.Abs(cfg.Git.RepoPath)
			if err != nil {
				return "", "", false, fmt.Errorf("failed to resolve repository path: %w", err)
			}
			sourceFile := filepath.Join(absRepo, sourceRepoPath)
			return sourceRepoPath, sourceFile, true, nil
		}
	}

	// Not found in mappings, check if symlink points to repository
	linkTarget, err := os.Readlink(symlinkPath)
	if err != nil {
		return "", "", false, fmt.Errorf("failed to read symlink: %w", err)
	}

	absRepo, err := filepath.Abs(cfg.Git.RepoPath)
	if err != nil {
		return "", "", false, fmt.Errorf("failed to resolve repository path: %w", err)
	}

	// Convert to absolute if relative
	if !filepath.IsAbs(linkTarget) {
		linkTarget = filepath.Join(filepath.Dir(symlinkPath), linkTarget)
		linkTarget = filepath.Clean(linkTarget)
	}

	if strings.HasPrefix(linkTarget, absRepo) {
		relPath, err := filepath.Rel(absRepo, linkTarget)
		if err != nil {
			return "", "", false, fmt.Errorf("failed to compute relative path: %w", err)
		}
		return relPath, linkTarget, true, nil
	}

	return "", "", false, nil
}

func confirmRemoval(symlinkPath, trackedFile, mappingKey string) bool {
	fmt.Printf("Found tracked file:\n")
	fmt.Printf("🔗 Symlink: %s\n", symlinkPath)
	fmt.Printf("📄 Tracked file: %s\n", trackedFile)
	fmt.Printf("🔑 Mapping: %s\n", mappingKey)

	var action string
	if deleteAll {
		action = "delete both symlink and tracked file"
	} else {
		action = "remove symlink (keeping tracked file)"
	}

	fmt.Printf("\n⚠️  This will %s\n", action)
	fmt.Printf("Continue? (y/N): ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

func executeRemoval(cfg *config.SyncConfig, symlinkPath, trackedFile, mappingKey string) error {
	// Remove symlink
	if err := os.Remove(symlinkPath); err != nil {
		return fmt.Errorf("failed to remove symlink: %w", err)
	}

	// If not deleting all, copy the file back to original location
	if !deleteAll {
		// Ensure the original directory exists
		if err := os.MkdirAll(filepath.Dir(symlinkPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for file restoration: %w", err)
		}

		// Copy file from repository back to original location
		if err := copyFileForRemove(trackedFile, symlinkPath); err != nil {
			return fmt.Errorf("failed to restore file to original location: %w", err)
		}
	} else {
		// Delete the tracked file from repository
		if err := os.Remove(trackedFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove tracked file: %w", err)
		}
	}

	return nil
}

func updateConfigAfterRemoval(cfg *config.SyncConfig, mappingKey string) error {
	if cfg.Mappings == nil {
		return nil // nothing to remove
	}

	delete(cfg.Mappings, mappingKey)

	configPath := cfg.GetConfigPath()
	if err := cfg.SaveToFile(configPath); err != nil {
		return fmt.Errorf("failed to save updated configuration: %w", err)
	}

	return nil
}

// copyFileForRemove copies the file from src to dst while preserving the original permissions.
func copyFileForRemove(src, dst string) (err error) {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer util.CloseAndCaptureErr(sourceFile, &err)

	// Get source file info to preserve permissions
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	// Create destination file with source file's permissions
	destFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode())
	if err != nil {
		return err
	}
	defer util.CloseAndCaptureErr(destFile, &err)

	if _, err = io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}

// commitRemovedFile stages and commits the removal changes to git using GitManager
func commitRemovedFile(cmd *cobra.Command, cfg *config.SyncConfig, trackedFile, filePath string) error {
	// Create GitManager instance
	gmCfg := cfg.ToGitManagerConfig()

	gm, err := gitmanager.NewGitManager(cmd.Context(), gmCfg)
	if err != nil {
		return fmt.Errorf("failed to create git manager: %w", err)
	}

	// Stage and commit the changes
	changedFiles, err := gm.StageAndCommit(cmd.Context(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to stage and commit changes: %w", err)
	}

	// Only push if there's a remote configured
	if cfg.Git.RemoteURL != "" {
		if err := gm.Push(cmd.Context()); err != nil {
			// Commit succeeded but push failed - still consider operation successful
			fmt.Printf("⚠️  Warning: Git push failed: %v\n", err)
			fmt.Printf("📝 Note: Changes were committed locally but not pushed. Run 'dsm sync' or wait for auto-sync to push.\n")
		}
	}

	// Log which files were committed for transparency
	if len(changedFiles) > 0 {
		fmt.Printf("📝 Committed changes:\n")
		for _, file := range changedFiles {
			fmt.Printf("   - %s\n", file)
		}
	}

	return nil
}