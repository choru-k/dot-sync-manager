package cmd

import (
	"bufio"
	"fmt"
	"io"
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
	fileInfo, err := os.Lstat(filePath) // Use Lstat to detect symlinks
	if os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if it's a directory
	if fileInfo.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s\nHint: Use symlinks for directories or add individual files within the directory", filePath)
	}

	// Check if file is already a symlink
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		linkTarget, _ := os.Readlink(filePath)
		return fmt.Errorf("file is already a symlink: %s -> %s\nHint: Only add actual files, not symlinks", filePath, linkTarget)
	}

	// Warn if file appears to be sensitive
	if isSensitiveFile(filePath) {
		fmt.Printf("⚠️  WARNING: This file may contain sensitive data:\n")
		fmt.Printf("   %s\n\n", filePath)
		fmt.Printf("   Sensitive files should NOT be added to dotfiles repositories as they will be:\n")
		fmt.Printf("   - Stored in Git history (cannot be fully removed)\n")
		fmt.Printf("   - Potentially pushed to remote repositories\n")
		fmt.Printf("   - Accessible to anyone with repository access\n\n")
		fmt.Printf("   Common sensitive files include:\n")
		fmt.Printf("   - SSH private keys (.ssh/id_*, .ssh/*.pem)\n")
		fmt.Printf("   - Cloud credentials (.aws/credentials, .gcp/*, .azure/*)\n")
		fmt.Printf("   - GPG private keys (.gnupg/private-keys-v1.d/*, *.key)\n")
		fmt.Printf("   - Environment files (.env, .env.local, .env.production)\n")
		fmt.Printf("   - Database credentials and API tokens\n\n")
		fmt.Printf("Type 'yes' to continue anyway, or anything else to cancel: ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}
		response = strings.TrimSpace(response)
		if response != "yes" {
			return fmt.Errorf("operation cancelled by user")
		}
		fmt.Println()
	}

	// Load configuration
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get absolute paths for both file and repo
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	absRepo, err := filepath.Abs(cfg.Git.RepoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve repository path: %w", err)
	}

	// Check if file is already inside the dotfiles repository
	relPath, err := filepath.Rel(absRepo, absPath)
	if err != nil {
		// filepath.Rel can fail on Windows with different drives
		// In this case, they're on different volumes, so file is definitely outside repo
	} else if !strings.HasPrefix(relPath, "..") && relPath != "." {
		// File is inside or equal to repo directory
		return fmt.Errorf("file is already inside dotfiles repository: %s\nHint: Only add files from outside the repository", absPath)
	}

	// Determine target path in dotfiles repository
	targetPath, err := getTargetPath(cfg.Git.RepoPath, absPath)
	if err != nil {
		return fmt.Errorf("failed to determine target path: %w", err)
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve target path: %w", err)
	}

	// Validate target is inside repository (absRepo already computed above)
	relTarget, err := filepath.Rel(absRepo, absTarget)
	if err != nil || strings.HasPrefix(relTarget, "..") || relTarget == "." {
		return fmt.Errorf("target path is outside repository: %s", absTarget)
	}

	// Check if target already exists
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("target file already exists in dotfiles: %s\nThis file may have been added previously", targetPath)
	}

	// Create target directory if needed
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Track backup path for potential rollback
	var backupPath string
	var backupCreated bool

	// Backup original file if it exists and is not a symlink
	if fileInfo, err := os.Lstat(filePath); err == nil && fileInfo.Mode()&os.ModeSymlink == 0 {
		backupDir := cfg.ConflictResolution.BackupDir
		if backupDir == "" {
			backupDir = filepath.Join(cfg.Git.RepoPath, ".backup")
		}

		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory: %w", err)
		}

		timestamp := time.Now().Format("20060102-150405")
		filename := filepath.Base(filePath)
		backupPath = filepath.Join(backupDir, fmt.Sprintf("%s-%s", filename, timestamp))

		if err := copyFile(filePath, backupPath); err != nil {
			return fmt.Errorf("failed to backup original file: %w", err)
		}
		backupCreated = true
		fmt.Printf("📦 Backed up original file to: %s\n", backupPath)
	}

	// Move file to dotfiles directory using copy-then-remove for cross-filesystem compatibility
	if err := copyFile(filePath, targetPath); err != nil {
		if backupCreated {
			if removeErr := os.Remove(backupPath); removeErr != nil {
				fmt.Printf("⚠️  Warning: failed to remove backup file %s after copy error: %v\n", backupPath, removeErr)
			}
		}
		return fmt.Errorf("failed to copy file to dotfiles: %w", err)
	}

	// Remove original file after successful copy
	if err := os.Remove(filePath); err != nil {
		// Rollback: remove the copied file
		if removeErr := os.Remove(targetPath); removeErr != nil {
			fmt.Printf("❌ Failed to remove copied file during rollback: %v\n", removeErr)
		}
		if backupCreated {
			if removeErr := os.Remove(backupPath); removeErr != nil {
				fmt.Printf("⚠️  Warning: failed to remove backup file %s after remove error: %v\n", backupPath, removeErr)
			}
		}
		return fmt.Errorf("failed to remove original file: %w", err)
	}

	// Create symlink
	if err := os.Symlink(targetPath, filePath); err != nil {
		// Rollback: restore from backup or from target
		restoreSuccessful := false
		if backupCreated {
			// Restore from backup
			if restoreErr := copyFile(backupPath, filePath); restoreErr != nil {
				fmt.Printf("❌ Failed to restore from backup: %v\n", restoreErr)
				fmt.Printf("⚠️  Backup retained at %s for manual recovery\n", backupPath)
			} else {
				restoreSuccessful = true
				// Clean up backup after successful restore
				os.Remove(backupPath)
			}
		}

		// Remove the file from dotfiles directory
		if removeErr := os.Remove(targetPath); removeErr != nil {
			fmt.Printf("⚠️  Warning: failed to remove file from dotfiles during rollback: %v\n", removeErr)
		}

		if restoreSuccessful {
			return fmt.Errorf("failed to create symlink: %w\nOriginal file restored to %s", err, filePath)
		}
		if backupCreated {
			return fmt.Errorf("failed to create symlink: %w\nBackup available at %s", err, backupPath)
		}
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	// Success! Clean up the backup since operation completed successfully
	if backupCreated {
		if err := os.Remove(backupPath); err != nil {
			fmt.Printf("⚠️  Warning: failed to remove backup file %s: %v\n", backupPath, err)
		}
	}

	// Update mappings in configuration
	sourceRelative, err := filepath.Rel(cfg.Git.RepoPath, targetPath)
	if err != nil {
		return fmt.Errorf("failed to compute mapping path: %w", err)
	}

	if cfg.Mappings == nil {
		cfg.Mappings = make(map[string]string)
	}
	cfg.Mappings[sourceRelative] = filePath

	// Save updated configuration to the original location
	configPath := cfg.GetConfigPath()
	if err := cfg.SaveToFile(configPath); err != nil {
		fmt.Printf("⚠️  Warning: failed to update configuration: %v\n", err)
	}

	fmt.Printf("✅ Added file to dotfiles\n")
	fmt.Printf("📄 Source: %s\n", targetPath)
	fmt.Printf("🔗 Symlink: %s\n", filePath)
	fmt.Printf("📂 Repository: %s\n", cfg.Git.RepoPath)

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
		relativePath = strings.TrimPrefix(relativePath, string(os.PathSeparator))
		relativePath = filepath.Clean(relativePath)

		parts := strings.Split(relativePath, string(os.PathSeparator))
		if len(parts) > 0 {
			parts[0] = strings.TrimPrefix(parts[0], ".")
		}
		normalized := filepath.Join(parts...)

		return filepath.Join(repoPath, normalized), nil
	}

	// For files outside home directory, use the full path structure
	relativePath := strings.TrimPrefix(sourcePath, string(os.PathSeparator))
	relativePath = filepath.Clean(relativePath)
	return filepath.Join(repoPath, relativePath), nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

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
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}

// isSensitiveFile checks if a file path matches patterns for sensitive files
func isSensitiveFile(path string) bool {
	// Normalize path separators
	path = filepath.ToSlash(path)
	baseName := filepath.Base(path)

	// Sensitive file patterns
	sensitivePatterns := []string{
		// SSH keys
		".ssh/id_rsa", ".ssh/id_dsa", ".ssh/id_ecdsa", ".ssh/id_ed25519",
		".ssh/identity",
		// Environment files
		".env", ".env.local", ".env.production", ".env.development", ".env.test",
		// Cloud credentials
		".aws/credentials", ".aws/config",
		".gcp/credentials", ".gcp/key.json",
		".azure/credentials",
		// GPG keys
		".gnupg/secring.gpg", ".gnupg/pubring.gpg",
		// Database files
		".mysql_history", ".psql_history", ".pgpass",
		// Docker secrets
		".docker/config.json",
	}

	// Sensitive filename patterns
	sensitiveNames := []string{
		"credentials", "secrets", "secret", "password", "passwd",
		"token", "auth", "private", "privatekey",
	}

	// Sensitive extensions
	sensitiveExtensions := []string{
		".pem", ".key", ".p12", ".pfx", ".jks", ".keystore",
	}

	// Check exact patterns
	for _, pattern := range sensitivePatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check if basename contains sensitive keywords
	lowerBaseName := strings.ToLower(baseName)
	for _, name := range sensitiveNames {
		if strings.Contains(lowerBaseName, name) {
			return true
		}
	}

	// Check extensions
	for _, ext := range sensitiveExtensions {
		if strings.HasSuffix(lowerBaseName, ext) {
			return true
		}
	}

	// Check for files inside .gnupg/private-keys-v1.d/
	if strings.Contains(path, ".gnupg/private-keys-v1.d/") {
		return true
	}

	return false
}
