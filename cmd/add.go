package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add <filepath>",
	Short: "Add file to dotfiles",
	Long: `Add a file to your dotfiles repository. The file will be moved to the dotfiles
directory and a symlink will be created in its original location.

Examples:
  dsm add ~/.bashrc
  dsm add ~/.vimrc
  dsm add ~/.config/nvim/init.vim`,
	Args: cobra.ExactArgs(1),
	RunE: runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	// Expand path
	filePath = expandPath(filePath)

	// Check if file exists and validate it's a file (not a directory)
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	if fileInfo.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s\nHint: Use symlinks for directories or add individual files within the directory", filePath)
	}

	// Load configuration
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Determine target path in dotfiles repository
	targetPath, err := getTargetPath(cfg.Git.RepoPath, absPath)
	if err != nil {
		return fmt.Errorf("failed to determine target path: %w", err)
	}

	// Check if target already exists
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("target file already exists in dotfiles: %s", targetPath)
	}

	// Create target directory if needed
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Track backup path for potential rollback
	var backupPath string

	// Backup original file if it exists and is not a symlink
	if fileInfo, err := os.Lstat(filePath); err == nil && fileInfo.Mode()&os.ModeSymlink == 0 {
		// Use configured backup directory if available
		backupDir := cfg.ConflictResolution.BackupDir
		if backupDir == "" {
			backupDir = filepath.Join(cfg.Git.RepoPath, ".backup")
		}

		// Create backup directory if needed
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory: %w", err)
		}

		// Generate backup filename with timestamp
		timestamp := time.Now().Format("20060102-150405")
		filename := filepath.Base(filePath)
		backupPath = filepath.Join(backupDir, fmt.Sprintf("%s-%s", filename, timestamp))

		if err := os.Rename(filePath, backupPath); err != nil {
			return fmt.Errorf("failed to backup original file: %w", err)
		}
		fmt.Printf("📦 Backed up original file to: %s\n", backupPath)
	}

	// Move file to dotfiles directory
	if err := os.Rename(filePath, targetPath); err != nil {
		// Rollback: Restore backup if it was created
		if backupPath != "" {
			_ = os.Rename(backupPath, filePath)
		}
		return fmt.Errorf("failed to move file to dotfiles: %w", err)
	}

	// Create symlink
	if err := os.Symlink(targetPath, filePath); err != nil {
		// Rollback: Move file back from dotfiles
		_ = os.Rename(targetPath, filePath)
		// Rollback: Restore the original backup if it was created
		if backupPath != "" {
			_ = os.Rename(filePath, backupPath)
		}
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	// Success! Clean up the backup since operation completed successfully
	if backupPath != "" {
		_ = os.Remove(backupPath)
	}

	// Update mappings in configuration
	sourceRelative := strings.TrimPrefix(targetPath, cfg.Git.RepoPath)
	sourceRelative = strings.TrimPrefix(sourceRelative, "/")

	if cfg.Mappings == nil {
		cfg.Mappings = make(map[string]string)
	}
	cfg.Mappings[sourceRelative] = filePath

	// Save updated configuration
	configPath := filepath.Join(cfg.Git.RepoPath, ".sync-config.json")
	if err := cfg.SaveToFile(configPath); err != nil {
		fmt.Printf("⚠️  Warning: failed to update configuration: %v\n", err)
	}

	fmt.Printf("✅ Added file to dotfiles\n")
	fmt.Printf("📄 Source: %s\n", targetPath)
	fmt.Printf("🔗 Symlink: %s\n", filePath)
	fmt.Printf("📂 Repository: %s\n", cfg.Git.RepoPath)

	// TODO: Git add and commit
	// This would integrate with the gitmanager package to stage and commit the changes
	fmt.Printf("\n📝 Note: File staged for git commit. Run 'dsm sync' or wait for auto-sync.\n")

	return nil
}

// getTargetPath determines where the file should be placed in the dotfiles repository
func getTargetPath(repoPath, sourcePath string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// If source is in home directory, strip the home directory prefix
	if strings.HasPrefix(sourcePath, homeDir) {
		relativePath := strings.TrimPrefix(sourcePath, homeDir)
		relativePath = strings.TrimPrefix(relativePath, "/")

		// Remove leading dot from filename (as per PRD repository structure)
		if strings.HasPrefix(relativePath, ".") {
			relativePath = relativePath[1:]
		}

		return filepath.Join(repoPath, relativePath), nil
	}

	// For files outside home directory, use the full path structure
	relativePath := strings.TrimPrefix(sourcePath, "/")
	return filepath.Join(repoPath, relativePath), nil
}