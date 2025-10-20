package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/util"
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

const (
	// filePerms allows owner read/write access while providing read access to group and others, appropriate for most dotfiles.
	filePerms = 0644 // rw-r--r-- standard readable file permissions for dotfiles
	// sensitiveFilePerms restricts access to owner only, suitable for files containing secrets or private keys.
	sensitiveFilePerms = 0600 // rw------- private file permissions for sensitive data
	// backupTimestampFormat ensures backups are timestamped consistently for easy sorting.
	backupTimestampFormat = "20060102-150405"
	// defaultBackupDirName provides the fallback directory when no custom backup path is configured.
	defaultBackupDirName = ".backup"
	// tempConfigSuffix isolates temporary config files so atomic renames remain predictable.
	tempConfigSuffix = ".tmp"
)

var (
	mkdirAllFunc = os.MkdirAll
	removeFunc   = os.Remove
	symlinkFunc  = os.Symlink
	renameFunc   = os.Rename

	copyFileFunc = copyFile
	saveConfigFn = func(cfg *config.SyncConfig, path string) error {
		return cfg.SaveToFile(path)
	}
)

var (
	// sensitivePathPatterns lists path fragments that indicate files likely contain secrets.
	sensitivePathPatterns = []string{
		// SSH keys
		"/.ssh/id_rsa", "/.ssh/id_dsa", "/.ssh/id_ecdsa", "/.ssh/id_ed25519",
		"/.ssh/identity",
		// Environment files
		"/.env", "/.env.local", "/.env.production", "/.env.development", "/.env.test",
		// Cloud credentials
		"/.aws/credentials", "/.aws/config",
		"/.gcp/credentials", "/.gcp/key.json",
		"/.azure/credentials",
		// GPG keys
		"/.gnupg/secring.gpg", "/.gnupg/pubring.gpg",
		// Database files
		"/.mysql_history", "/.psql_history", "/.pgpass",
		// Docker secrets
		"/.docker/config.json",
	}

	// sensitiveFilenames holds substrings that often denote secret material.
	sensitiveFilenames = []string{
		"credentials", "secrets", "secret", "password", "passwd",
		"token", "auth", "private", "privatekey",
	}

	// sensitiveExtensions contains file suffixes commonly used for private keys or certificates.
	sensitiveExtensions = []string{
		".pem", ".key", ".p12", ".pfx", ".jks", ".keystore",
	}
)

// runAdd is the cobra entry point for the `dsm add` command; it orchestrates validation,
// file relocation, symlink creation, and configuration updates for a single source file.
func runAdd(cmd *cobra.Command, args []string) error {
	filePath, err := validateSourceFile(cmd, args[0])
	if err != nil {
		return fmt.Errorf("failed to validate source file: %w", err)
	}

	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	targetPath, err := prepareTargetPath(cfg, filePath)
	if err != nil {
		return fmt.Errorf("failed to prepare target path: %w", err)
	}

	backupPath, backupCreated, err := executeAddTransaction(cfg, filePath, targetPath)
	if err != nil {
		return fmt.Errorf("failed to execute add transaction: %w", err)
	}

	if err := updateAndSaveConfig(cfg, targetPath, filePath, backupPath, backupCreated); err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	if backupCreated && backupPath != "" {
		if err := removeFunc(backupPath); err != nil {
			fmt.Printf("⚠️  Warning: failed to remove backup file %s: %v\n", backupPath, err)
		}
	}

	fmt.Printf("✅ Added file to dotfiles\n")
	fmt.Printf("📄 Source: %s\n", targetPath)
	fmt.Printf("🔗 Symlink: %s\n", filePath)
	fmt.Printf("📂 Repository: %s\n", cfg.Git.RepoPath)

	fmt.Printf("\n📝 Note: File staged for git commit. Run 'dsm sync' or wait for auto-sync.\n")

	return nil
}

func validateSourceFile(cmd *cobra.Command, rawPath string) (string, error) {
	expandedPath, err := validatePathExists(rawPath)
	if err != nil {
		return "", err
	}

	fileInfo, err := os.Lstat(expandedPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	if fileInfo.IsDir() {
		return "", fmt.Errorf(`path is a directory, not a file: %s
Hint: Use symlinks for directories or add individual files within the directory`, expandedPath)
	}

	if fileInfo.Mode()&os.ModeSymlink != 0 {
		linkTarget, err := os.Readlink(expandedPath)
		if err != nil {
			return "", fmt.Errorf("file is a symlink, but could not read its target: %w", err)
		}
		return "", fmt.Errorf(`file is already a symlink: %s -> %s
Hint: Only add actual files, not symlinks`, expandedPath, linkTarget)
	}

	if isSensitiveFile(expandedPath) {
		if err := confirmSensitiveAdd(cmd, expandedPath); err != nil {
			return "", err
		}
	}

	return expandedPath, nil
}

func confirmSensitiveAdd(cmd *cobra.Command, filePath string) error {
	fmt.Printf(`⚠️  WARNING: This file may contain sensitive data:
   %s

   Sensitive files should NOT be added to dotfiles repositories as they will be:
   - Stored in Git history (cannot be fully removed)
   - Potentially pushed to remote repositories
   - Accessible to anyone with repository access

   Common sensitive files include:
   - SSH private keys (.ssh/id_*, .ssh/*.pem)
   - Cloud credentials (.aws/credentials, .gcp/*, .azure/*)
   - GPG private keys (.gnupg/private-keys-v1.d/*, *.key)
   - Environment files (.env, .env.local, .env.production)
   - Database credentials and API tokens

Type 'yes' to continue anyway, or anything else to cancel: `, filePath)

	reader := bufio.NewReader(cmd.InOrStdin())
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}
	response = strings.TrimSpace(response)
	if response != "yes" {
		return fmt.Errorf("operation cancelled by user")
	}
	fmt.Println()
	return nil
}

func prepareTargetPath(cfg *config.SyncConfig, filePath string) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	absRepo, err := filepath.Abs(cfg.Git.RepoPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve repository path: %w", err)
	}

	relPath, err := filepath.Rel(absRepo, absPath)
	if err == nil && !strings.HasPrefix(relPath, "..") && relPath != "." {
		return "", fmt.Errorf(`file is already inside dotfiles repository: %s
Hint: Only add files from outside the repository`, absPath)
	}

	targetPath, err := getTargetPath(cfg.Git.RepoPath, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to determine target path: %w", err)
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve target path: %w", err)
	}

	relTarget, err := filepath.Rel(absRepo, absTarget)
	if err != nil || strings.HasPrefix(relTarget, "..") || relTarget == "." {
		return "", fmt.Errorf("target path is outside repository: %s", absTarget)
	}

	if _, err := os.Stat(targetPath); err == nil {
		return "", fmt.Errorf(`target file already exists in dotfiles: %s
This file may have been added previously`, targetPath)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check target path: %w", err)
	}

	if err := mkdirAllFunc(filepath.Dir(targetPath), dirPerms); err != nil {
		return "", fmt.Errorf("failed to create target directory: %w", err)
	}

	return targetPath, nil
}

func executeAddTransaction(cfg *config.SyncConfig, filePath, targetPath string) (string, bool, error) {
	backupDir := cfg.ConflictResolution.BackupDir
	if backupDir == "" {
		backupDir = filepath.Join(cfg.Git.RepoPath, defaultBackupDirName)
	}

	if err := mkdirAllFunc(backupDir, dirPerms); err != nil {
		return "", false, fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := timeNow().Format(backupTimestampFormat)
	filename := filepath.Base(filePath)
	backupPath := filepath.Join(backupDir, fmt.Sprintf("%s-%s", filename, timestamp))

	if err := copyFileFunc(filePath, backupPath); err != nil {
		return "", false, fmt.Errorf("failed to backup original file: %w", err)
	}
	backupCreated := true
	fmt.Printf("📦 Backed up original file to: %s\n", backupPath)

	if err := copyFileFunc(filePath, targetPath); err != nil {
		if backupCreated {
			fmt.Printf("⚠️  Failed to copy file to dotfiles. Backup of original file retained at: %s\n", backupPath)
		}
		return backupPath, backupCreated, fmt.Errorf("failed to copy file to dotfiles: %w", err)
	}

	if err := removeFunc(filePath); err != nil {
		if removeErr := removeFunc(targetPath); removeErr != nil {
			fmt.Printf("❌ Failed to remove copied file during rollback: %v\n", removeErr)
		}
		if backupCreated {
			fmt.Printf("⚠️  Failed to remove original file. The file was copied to the repository, but the original was not removed. Backup retained at: %s for manual recovery.\n", backupPath)
		}
		return backupPath, backupCreated, fmt.Errorf("failed to remove original file: %w", err)
	}

	relSymlinkPath, err := filepath.Rel(filepath.Dir(filePath), targetPath)
	symlinkTarget := relSymlinkPath
	if err != nil {
		symlinkTarget = targetPath
	}

	if err := symlinkFunc(symlinkTarget, filePath); err != nil {
		restoreSuccessful := false
		if backupCreated {
			if restoreErr := copyFileFunc(backupPath, filePath); restoreErr != nil {
				fmt.Printf("❌ Failed to restore from backup: %v\n", restoreErr)
				fmt.Printf("⚠️  Backup retained at %s for manual recovery\n", backupPath)
			} else {
				restoreSuccessful = true
				backupCreated = false
				if removeErr := removeFunc(backupPath); removeErr != nil {
					fmt.Printf("⚠️  Warning: failed to remove backup file %s after restore: %v\n", backupPath, removeErr)
					backupCreated = true
				}
			}
		}

		if removeErr := removeFunc(targetPath); removeErr != nil {
			fmt.Printf("⚠️  Warning: failed to remove file from dotfiles during rollback: %v\n", removeErr)
		}

		if restoreSuccessful {
			return backupPath, backupCreated, fmt.Errorf("failed to create symlink: %w\nOriginal file restored to %s", err, filePath)
		}
		if backupCreated {
			return backupPath, backupCreated, fmt.Errorf("failed to create symlink: %w\nBackup available at %s", err, backupPath)
		}
		return backupPath, backupCreated, fmt.Errorf("failed to create symlink: %w", err)
	}

	return backupPath, backupCreated, nil
}

func updateAndSaveConfig(cfg *config.SyncConfig, targetPath, filePath, backupPath string, backupCreated bool) error {
	sourceRelative, err := filepath.Rel(cfg.Git.RepoPath, targetPath)
	if err != nil {
		return fmt.Errorf("failed to compute mapping path: %w", err)
	}

	if cfg.Mappings == nil {
		cfg.Mappings = make(map[string]string)
	}
	cfg.Mappings[sourceRelative] = filePath

	configPath := cfg.GetConfigPath()
	tempConfigPath := configPath + tempConfigSuffix
	if err := saveConfigFn(cfg, tempConfigPath); err != nil {
		restoredOriginal, backupRetained := rollbackAfterConfigFailure(filePath, targetPath, backupPath, backupCreated)
		if removeErr := removeFunc(tempConfigPath); removeErr != nil && !os.IsNotExist(removeErr) {
			fmt.Printf("⚠️  Warning: failed to remove temp config file %s: %v\n", tempConfigPath, removeErr)
		}
		return fmt.Errorf("file moved and symlinked, but failed to prepare configuration update: %w.%s Please check %s for correctness", err, rollbackOutcomeMessage(restoredOriginal, backupRetained, backupPath), configPath)
	}

	if err := renameFunc(tempConfigPath, configPath); err != nil {
		restoredOriginal, backupRetained := rollbackAfterConfigFailure(filePath, targetPath, backupPath, backupCreated)
		if removeErr := removeFunc(tempConfigPath); removeErr != nil && !os.IsNotExist(removeErr) {
			fmt.Printf("⚠️  Warning: failed to remove temp config file %s: %v\n", tempConfigPath, removeErr)
		}
		return fmt.Errorf("file moved and symlinked, but failed to finalize configuration: %w.%s Please check %s for correctness", err, rollbackOutcomeMessage(restoredOriginal, backupRetained, backupPath), configPath)
	}

	return nil
}

// rollbackAfterConfigFailure attempts to restore the user's original file when a configuration
// write fails after the file has been moved and symlinked. It returns whether the original file was
// restored and whether a backup copy remains for manual recovery.
func rollbackAfterConfigFailure(filePath, targetPath, backupPath string, backupCreated bool) (bool, bool) {
	restored := false
	backupRetained := backupCreated

	if err := removeFunc(filePath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("⚠️  Warning: failed to remove symlink during rollback: %v\n", err)
	}

	if backupCreated {
		if err := copyFileFunc(backupPath, filePath); err != nil {
			fmt.Printf("❌ Failed to restore original file from backup: %v\n", err)
		} else {
			restored = true
			backupRetained = false
			if removeErr := removeFunc(backupPath); removeErr != nil {
				fmt.Printf("⚠️  Warning: failed to remove backup file %s after restore: %v\n", backupPath, removeErr)
				backupRetained = true
			}
		}
	}

	if !restored {
		if err := copyFileFunc(targetPath, filePath); err != nil {
			fmt.Printf("❌ Failed to restore original file from dotfiles copy: %v\n", err)
		} else {
			restored = true
		}
	}

	if restored {
		if removeErr := removeFunc(targetPath); removeErr != nil && !os.IsNotExist(removeErr) {
			fmt.Printf("⚠️  Warning: failed to remove dotfiles copy during rollback: %v\n", removeErr)
		}
	}

	return restored, backupRetained
}

// rollbackOutcomeMessage produces user-facing guidance based on rollback results.
func rollbackOutcomeMessage(restored, backupRetained bool, backupPath string) string {
	if restored && backupRetained && backupPath != "" {
		return fmt.Sprintf(" Original file restored. Backup retained at %s.", backupPath)
	}
	if restored {
		return " Original file restored."
	}
	if backupRetained && backupPath != "" {
		return fmt.Sprintf(" Original file not fully restored. Backup available at %s.", backupPath)
	}
	return " Original file could not be restored automatically."
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
	// Strip volume name (e.g., "C:" on Windows) to ensure relative path
	relativePath := strings.TrimPrefix(sourcePath, filepath.VolumeName(sourcePath))
	relativePath = strings.TrimPrefix(relativePath, string(os.PathSeparator))
	return filepath.Join(repoPath, filepath.Clean(relativePath)), nil
}

// copyFile copies the file from src to dst while preserving the original permissions.
func copyFile(src, dst string) (err error) {
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

// isSensitiveFile checks if a file path matches patterns for sensitive files that should not
// be stored in version control. This includes SSH keys, credentials, GPG keys, and other
// sensitive configuration files. Returns true if the file matches any sensitive pattern.
func isSensitiveFile(path string) bool {
	// Normalize path separators
	path = filepath.ToSlash(path)
	baseName := filepath.Base(path)

	// Check exact patterns
	for _, pattern := range sensitivePathPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check if basename contains sensitive keywords
	lowerBaseName := strings.ToLower(baseName)
	for _, name := range sensitiveFilenames {
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
	if strings.Contains(path, "/.gnupg/private-keys-v1.d/") {
		return true
	}

	return false
}
