package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/config"
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
	deleteAll bool
)

func init() {
	rootCmd.AddCommand(removeCmd)
	removeCmd.Flags().BoolVar(&deleteAll, "delete-all", false, "Delete file from both home and repository")
}

func runRemove(cmd *cobra.Command, args []string) error {
	// Validate argument count even though Cobra should enforce this via Args: cobra.ExactArgs(1)
	if len(args) == 0 {
		return fmt.Errorf("remove command requires exactly one argument: <filepath>")
	}
	if len(args) > 1 {
		return fmt.Errorf("remove command accepts only one argument, got %d", len(args))
	}

	// No conflicting flags to validate since --keep-repo was removed

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
	if err := executeRemoval(filePath, trackedFile); err != nil {
		return err
	}

	// Update configuration
	if err := updateConfigAfterRemoval(cfg, mappingKey); err != nil {
		return fmt.Errorf("removed files but failed to update configuration: %w", err)
	}

	fmt.Printf("✅ Removed file from dotfiles tracking\n")
	fmt.Printf("🔗 Symlink removed: %s\n", filePath)
	if !deleteAll {
		fmt.Printf("📄 Original file restored to: %s\n", filePath)
		fmt.Printf("📂 File preserved in repository: %s\n", trackedFile)
	}
	fmt.Printf("📂 Repository: %s\n", cfg.Git.RepoPath)

	fmt.Printf("\n📝 Note: Changes staged for git commit. Run 'dsm sync' or wait for auto-sync.\n")

	return nil
}

func validateRemoveTarget(rawPath string) (string, error) {
	expandedPath, err := validatePathExists(rawPath)
	if err != nil {
		return "", err
	}

	// Get file info for symlink check
	fileInfo, err := os.Lstat(expandedPath)
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

	// Create reverse map for O(1) lookup: target path -> source path
	reverseMap := make(map[string]string)
	for sourceRepoPath, targetHomePath := range cfg.Mappings {
		absTarget, err := filepath.Abs(targetHomePath)
		if err != nil {
			continue // skip this mapping, continue checking others
		}
		reverseMap[absTarget] = sourceRepoPath
	}

	// O(1) lookup in reverse map
	if sourceRepoPath, found := reverseMap[absSymlink]; found {
		absRepo, err := filepath.Abs(cfg.Git.RepoPath)
		if err != nil {
			return "", "", false, fmt.Errorf("failed to resolve repository path: %w", err)
		}
		sourceFile := filepath.Join(absRepo, sourceRepoPath)
		return sourceRepoPath, sourceFile, true, nil
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
		action = "remove symlink and restore original file (keeping tracked file)"
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

func executeRemoval(symlinkPath, trackedFile string) error {
	// If not deleting all, restore the original file from repo to home location
	if !deleteAll {
		if err := restoreOriginalFile(trackedFile, symlinkPath); err != nil {
			return fmt.Errorf("failed to restore original file: %w", err)
		}
	}

	// Remove symlink
	if err := os.Remove(symlinkPath); err != nil {
		return fmt.Errorf("failed to remove symlink: %w", err)
	}

	// Optionally delete the tracked file
	if deleteAll {
		if err := os.Remove(trackedFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove tracked file: %w", err)
		}
	}

	return nil
}

func restoreOriginalFile(trackedFile, symlinkPath string) error {
	// Get file info to preserve permissions
	sourceInfo, err := os.Stat(trackedFile)
	if err != nil {
		return fmt.Errorf("failed to stat tracked file: %w", err)
	}

	// Copy the file from repository back to original home location
	if err := copyFile(trackedFile, symlinkPath); err != nil {
		return fmt.Errorf("failed to copy file from repository to home: %w", err)
	}

	// Preserve original file permissions
	if err := os.Chmod(symlinkPath, sourceInfo.Mode()); err != nil {
		// Non-fatal error - file was copied successfully
		fmt.Printf("⚠️  Warning: could not preserve original file permissions: %v\n", err)
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