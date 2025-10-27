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
	// checkTimeout is the maximum time to wait for check operations
	checkTimeout = 30 * time.Second
)

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for conflicts and issues",
	Long: `Check the dotfiles repository for conflicts, sync issues, and other problems.
This command examines the repository status and reports any issues that need attention.

Examples:
  dsm check
  dsm check --verbose  # Show detailed information`,
	RunE: runCheck,
}

var checkVerbose bool

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().BoolVar(&checkVerbose, "verbose", false, "Show detailed information")
}

// checkArguments validates that no arguments are provided to check command
func checkArguments(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("check command accepts no arguments")
	}
	return nil
}

// checkDaemonStatus checks if the DSM daemon is running
func checkDaemonStatus() ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	if isDaemonRunning() {
		printSuccess("Daemon is running")
		if checkVerbose {
			if pid, err := getDaemonPID(); err == nil {
				fmt.Printf("   PID: %d\n", pid)
			}
		}
	} else {
		warnings = append(warnings, "Daemon is not running")
	}

	return issues, warnings
}

// checkGitRepository checks the git repository status and accessibility
func checkGitRepository(cfg *config.SyncConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	gmCfg := cfg.ToGitManagerConfig()
	gitMgr, err := gitmanager.NewGitManager(ctx, gmCfg)
	if err != nil {
		issues = append(issues, fmt.Sprintf("Failed to initialize git manager: %v", err))
		return issues, warnings
	}

	// Check if repository exists and is accessible
	repo := gitMgr.Repo()
	if repo == nil {
		issues = append(issues, "Git repository is not accessible")
	} else {
		printSuccess("Git repository is accessible")

		// Check working tree status
		worktree, err := repo.Worktree()
		if err != nil {
			issues = append(issues, fmt.Sprintf("Failed to access working tree: %v", err))
		} else {
			status, err := worktree.Status()
			if err != nil {
				issues = append(issues, fmt.Sprintf("Failed to get repository status: %v", err))
			} else {
				if status.IsClean() {
					printSuccess("Working tree is clean")
				} else {
					warnings = append(warnings, fmt.Sprintf("Working tree has %d uncommitted changes", len(status)))
					if checkVerbose {
						fmt.Println("   Uncommitted files:")
						for path := range status {
							fmt.Printf("   • %s\n", path)
						}
					}
				}
			}
		}
	}

	return issues, warnings
}

// checkRemoteConfiguration checks the git remote configuration
func checkRemoteConfiguration(cfg *config.SyncConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	if cfg == nil {
		return issues, warnings // Should handle nil gracefully
	}

	if cfg.Git.RemoteURL != "" {
		printStatus("🌐", "REMOTE", "Remote URL configured")
		if checkVerbose {
			fmt.Printf("   Remote URL: %s\n", cfg.Git.RemoteURL)
		}
	} else {
		warnings = append(warnings, "No remote URL configured")
	}

	return issues, warnings
}

// checkSyncConfiguration checks the sync-related configuration
func checkSyncConfiguration(cfg *config.SyncConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	if cfg == nil {
		return issues, warnings // Should handle nil gracefully
	}

	printStatus("⚙️", "CONFIG", "Checking configuration...")
	if cfg.Sync.AutoSyncEnabled {
		printSuccess("Auto-sync is enabled")
		if checkVerbose {
			fmt.Printf("   Pull interval: %d seconds\n", cfg.Sync.PullIntervalSeconds)
			fmt.Printf("   Debounce: %d seconds\n", cfg.Sync.DebounceSeconds)
		}
	} else {
		warnings = append(warnings, "Auto-sync is disabled")
	}

	return issues, warnings
}

// checkFileMappings checks the file mappings configuration
func checkFileMappings(cfg *config.SyncConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	if cfg == nil {
		return issues, warnings // Should handle nil gracefully
	}

	if len(cfg.Mappings) > 0 {
		printSuccess("%d file mappings configured", len(cfg.Mappings))
		if checkVerbose {
			for source, target := range cfg.Mappings {
				fmt.Printf("   %s -> %s\n", source, target)
			}
		}
	} else {
		warnings = append(warnings, "No file mappings configured")
	}

	return issues, warnings
}

// checkConflicts checks for conflict artifacts in the .dsm/conflicts directory
func checkConflicts(cfg *config.SyncConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	if cfg == nil {
		return issues, warnings // Should handle nil gracefully
	}

	conflictDir := filepath.Join(cfg.Git.RepoPath, ".dsm", "conflicts")
	if stat, err := os.Stat(conflictDir); err == nil && stat.IsDir() {
		entries, err := os.ReadDir(conflictDir)
		if err == nil && len(entries) > 0 {
			issues = append(issues, fmt.Sprintf("Found conflict artifacts in %s", conflictDir))
			printError("Conflict artifacts found: %s", conflictDir)
			printInfo("Run 'dsm conflicts' for more details")
			printInfo("Run 'dsm resolve' after manually resolving conflicts")
		} else {
			printSuccess("No conflict artifacts found")
		}
	} else {
		printSuccess("No conflict artifacts found")
	}

	return issues, warnings
}

// printCheckSummary prints the final summary of issues and warnings
func printCheckSummary(issues, warnings []string) {
	fmt.Println("\n" + strings.Repeat("=", 50))
	if len(issues) > 0 {
		printError("Found %d issue(s):", len(issues))
		for _, issue := range issues {
			fmt.Printf("   • %s\n", issue)
		}
	}

	if len(warnings) > 0 {
		printWarning("Found %d warning(s):", len(warnings))
		for _, warning := range warnings {
			fmt.Printf("   • %s\n", warning)
		}
	}

	if len(issues) == 0 && len(warnings) == 0 {
		printSuccess("Everything looks good!")
	} else if len(issues) == 0 {
		printSuccess("No critical issues found (some warnings present)")
	}
}

func runCheck(cmd *cobra.Command, args []string) error {
	// Validate arguments
	if err := checkArguments(args); err != nil {
		return err
	}

	// Load configuration
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Print header
	printStatus("🔍", "CHECKING", "Checking dotfiles repository...")
	if noEmoji {
		fmt.Printf("Repository: %s\n", cfg.Git.RepoPath)
		fmt.Printf("Machine: %s\n", cfg.Machine.Name)
	} else {
		fmt.Printf("📁 Repository: %s\n", cfg.Git.RepoPath)
		fmt.Printf("🖥️  Machine: %s\n", cfg.Machine.Name)
	}

	// Collect all issues and warnings
	allIssues := []string{}
	allWarnings := []string{}

	// Run all checks
	issues, warnings := checkDaemonStatus()
	allIssues = append(allIssues, issues...)
	allWarnings = append(allWarnings, warnings...)

	issues, warnings = checkGitRepository(cfg)
	allIssues = append(allIssues, issues...)
	allWarnings = append(allWarnings, warnings...)

	issues, warnings = checkRemoteConfiguration(cfg)
	allIssues = append(allIssues, issues...)
	allWarnings = append(allWarnings, warnings...)

	issues, warnings = checkSyncConfiguration(cfg)
	allIssues = append(allIssues, issues...)
	allWarnings = append(allWarnings, warnings...)

	issues, warnings = checkFileMappings(cfg)
	allIssues = append(allIssues, issues...)
	allWarnings = append(allWarnings, warnings...)

	issues, warnings = checkConflicts(cfg)
	allIssues = append(allIssues, issues...)
	allWarnings = append(allWarnings, warnings...)

	// Print summary
	printCheckSummary(allIssues, allWarnings)

	// Return appropriate exit code
	if len(allIssues) > 0 {
		return fmt.Errorf("found %d issue(s) that need attention", len(allIssues))
	}

	return nil
}