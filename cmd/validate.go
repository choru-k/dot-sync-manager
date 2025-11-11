package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate-config",
	Short: "Validate configuration file",
	Long: `Validate the dotfile sync manager configuration file for common issues
and potential problems.

Examples:
  dsm validate-config
  dsm validate-config --verbose`,
	RunE: runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("🔍 Validating configuration...")
	fmt.Printf("📁 Config file: %s\n", cfg.GetConfigPath())

	issues := []string{}
	warnings := []string{}

	// Basic structure validation
	if cfg.Machine.Name == "" {
		issues = append(issues, "Machine name is required")
	}
	if cfg.Git.RepoPath == "" {
		issues = append(issues, "Git repository path is required")
	} else {
		// Check if repo path exists
		if _, err := os.Stat(cfg.Git.RepoPath); os.IsNotExist(err) {
			issues = append(issues, fmt.Sprintf("Git repository path does not exist: %s", cfg.Git.RepoPath))
		}
	}

	// Git configuration validation
	if cfg.Git.RemoteURL != "" {
		if !strings.HasPrefix(cfg.Git.RemoteURL, "git@") && !strings.HasPrefix(cfg.Git.RemoteURL, "https://") {
			warnings = append(warnings, "Remote URL should use SSH (git@) or HTTPS (https://)")
		}
	}

	// SSH key validation for SSH remotes
	if strings.HasPrefix(cfg.Git.RemoteURL, "git@") && cfg.Git.AuthType == "ssh" {
		sshPath := filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
		if err := validatePathExists(sshPath); err != nil {
			warnings = append(warnings, fmt.Sprintf("SSH key not found at %s", sshPath))
		}
	}

	// Print results
	if len(issues) == 0 && len(warnings) == 0 {
		fmt.Println("✅ Configuration is valid")
		return nil
	}

	if len(issues) > 0 {
		fmt.Println("\n❌ Issues found:")
		for _, issue := range issues {
			fmt.Printf("  • %s\n", issue)
		}
	}

	if len(warnings) > 0 {
		fmt.Println("\n⚠️  Warnings:")
		for _, warning := range warnings {
			fmt.Printf("  • %s\n", warning)
		}
	}

	if verbose {
		fmt.Printf("\n📊 Configuration Summary:\n")
		fmt.Printf("  Machine: %s\n", cfg.Machine.Name)
		fmt.Printf("  Repository: %s\n", cfg.Git.RepoPath)
		fmt.Printf("  Remote: %s\n", cfg.Git.RemoteURL)
		fmt.Printf("  Auth Type: %s\n", cfg.Git.AuthType)
		fmt.Printf("  Auto-sync: %v\n", cfg.Sync.AutoSyncEnabled)
	}

	if len(issues) > 0 {
		return fmt.Errorf("configuration has %d issue(s) to fix", len(issues))
	}

	return nil
}