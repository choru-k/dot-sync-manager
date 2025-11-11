// Package cmd provides command-line interface commands for the Dotfile Sync Manager.
//
// This file implements the validate-config command which provides comprehensive
// configuration validation to help users identify and fix common issues.
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/spf13/cobra"
)

const (
	validateTimeout = 30 * time.Second
)

// validateCmd represents the validate-config command
var validateCmd = &cobra.Command{
	Use:   "validate-config",
	Short: "Validate configuration file",
	Long: `Validate the dotfile sync configuration file for common issues.

This command performs comprehensive validation of the configuration file,
checking for missing required fields, invalid paths, and accessibility issues.

Examples:
  dsm validate-config
  dsm validate-config --config /path/to/config.json`,
	RunE: runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

// runValidate executes the configuration validation
func runValidate(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Print header
	printStatus("🔍", "VALIDATING", "Validating configuration file...")
	if noEmoji {
		fmt.Printf("Configuration: %s\n", cfg.GetConfigPath())
	} else {
		fmt.Printf("📄 Configuration: %s\n", cfg.GetConfigPath())
	}

	// Track validation results
	allIssues := []string{}
	allWarnings := []string{}

	// Run all validations
	issues, warnings := validateBasicStructure(cfg)
	allIssues = append(allIssues, issues...)
	allWarnings = append(allWarnings, warnings...)

	issues, warnings = validateRequiredFields(cfg)
	allIssues = append(allIssues, issues...)
	allWarnings = append(allWarnings, warnings...)

	issues, warnings = validateGitIntegration(cfg)
	allIssues = append(allIssues, issues...)
	allWarnings = append(allWarnings, warnings...)

	issues, warnings = validateSecuritySettings(cfg)
	allIssues = append(allIssues, issues...)
	allWarnings = append(allWarnings, warnings...)

	// Print summary
	printValidationSummary(allIssues, allWarnings)

	// Return appropriate exit code
	if len(allIssues) > 0 {
		return fmt.Errorf("found %d validation issue(s) that need to be fixed", len(allIssues))
	}

	if len(allWarnings) > 0 {
		fmt.Println("\n💡 Configuration is valid but has some warnings.")
		fmt.Println("   Consider addressing these warnings for optimal experience.")
	} else {
		fmt.Println("\n✅ Configuration is valid and ready to use!")
	}

	return nil
}

// validateBasicStructure checks the basic structure and format of the configuration
func validateBasicStructure(cfg *config.SyncConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	// Check if config is properly loaded
	if cfg == nil {
		issues = append(issues, "Configuration file could not be loaded")
		return issues, warnings
	}

	// Check if machine name is set
	if cfg.Machine.Name == "" {
		issues = append(issues, "Machine name is empty - required for multi-machine sync")
	} else {
		if verbose {
			fmt.Printf("   Machine name: %s\n", cfg.Machine.Name)
		}
	}

	return issues, warnings
}

// validateRequiredFields checks all required configuration fields
func validateRequiredFields(cfg *config.SyncConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	// Git repository path is required
	if cfg.Git.RepoPath == "" {
		issues = append(issues, "Git repository path (git.repo_path) is required")
	} else {
		issues, warnings = validateRepoPath(cfg.Git.RepoPath, issues, warnings)
	}

	// Check sync configuration
	if cfg.Sync.DebounceSeconds <= 0 {
		warnings = append(warnings, "Sync debounce time is very short (may cause excessive commits)")
	}

	if cfg.Sync.PullIntervalSeconds <= 0 {
		warnings = append(warnings, "Pull interval should be positive for regular syncing")
	}

	return issues, warnings
}

// validatePaths checks if all configured paths exist and are accessible
func validateRepoPath(repoPath string, issues, warnings []string) ([]string, []string) {
	// Expand and check repository path
	expandedPath, err := validatePathExists(repoPath)
	if err != nil {
		issues = append(issues, fmt.Sprintf("Git repository path validation failed: %v", err))
		return issues, warnings
	}

	// Check if it's actually a git repository
	gitDir := filepath.Join(expandedPath, ".git")
	if stat, err := os.Stat(gitDir); err != nil || !stat.IsDir() {
		issues = append(issues, fmt.Sprintf("Not a valid git repository: %s", expandedPath))
	} else {
		if verbose {
			fmt.Printf("   Git repository found at: %s\n", expandedPath)
		}
	}

	return issues, warnings
}

// validateGitIntegration checks if git integration is working
func validateGitIntegration(cfg *config.SyncConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	ctx, cancel := context.WithTimeout(context.Background(), validateTimeout)
	defer cancel()

	// Try to initialize GitManager with current config
	gmCfg := cfg.ToGitManagerConfig()
	_, err := gitmanager.NewGitManager(ctx, gmCfg)
	if err != nil {
		if strings.Contains(err.Error(), "ssh auth requires private key path") {
			warnings = append(warnings, "SSH authentication requires private key path configuration")
			warnings = append(warnings, "   Set git.ssh_key_path to your SSH private key file")
		} else {
			issues = append(issues, fmt.Sprintf("Git manager initialization failed: %v", err))
		}
	} else {
		if verbose {
			fmt.Printf("   Git integration configured successfully\n")
		}
	}

	// Check remote URL configuration
	if cfg.Git.RemoteURL == "" {
		warnings = append(warnings, "No remote URL configured - sync will be local only")
	} else {
		if strings.HasPrefix(cfg.Git.RemoteURL, "git@") {
			if verbose {
				fmt.Printf("   SSH remote URL: %s\n", cfg.Git.RemoteURL)
			}
		} else if strings.HasPrefix(cfg.Git.RemoteURL, "https://") {
			warnings = append(warnings, "HTTPS remote detected - ensure credentials are configured")
		} else {
			warnings = append(warnings, "Remote URL format may be invalid")
		}
	}

	return issues, warnings
}

// validateSecuritySettings checks security-related configuration
func validateSecuritySettings(cfg *config.SyncConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	// Check auth type
	if cfg.Git.AuthType == "" {
		warnings = append(warnings, "Git authentication type not explicitly set")
	} else {
		if verbose {
			fmt.Printf("   Git auth type: %s\n", cfg.Git.AuthType)
		}

		// Validate SSH key path if SSH auth is used
		if cfg.Git.AuthType == "ssh" {
			if cfg.Git.SSHKeyPath == "" {
				warnings = append(warnings, "SSH key path not configured - may use default SSH agent")
			} else {
				expandedPath, err := validatePathExists(cfg.Git.SSHKeyPath)
				if err != nil {
					issues = append(issues, fmt.Sprintf("SSH key file not found: %s", expandedPath))
				}
			}
		}
	}

	// Note: Ignore patterns are handled by .syncignore file in the repository
	if verbose {
		fmt.Printf("   Sync settings: auto_sync=%v, debounce=%ds, pull_interval=%ds\n",
			cfg.Sync.AutoSyncEnabled, cfg.Sync.DebounceSeconds, cfg.Sync.PullIntervalSeconds)
	}

	return issues, warnings
}

// printValidationSummary prints the final validation summary
func printValidationSummary(issues, warnings []string) {
	fmt.Println("\n" + strings.Repeat("=", 50))

	if len(issues) > 0 {
		printError("❌ Found %d validation issue(s):", len(issues))
		for _, issue := range issues {
			fmt.Printf("   • %s\n", issue)
		}
	}

	if len(warnings) > 0 {
		printWarning("⚠️  Found %d warning(s):", len(warnings))
		for _, warning := range warnings {
			fmt.Printf("   • %s\n", warning)
		}
	}

	fmt.Println()
}