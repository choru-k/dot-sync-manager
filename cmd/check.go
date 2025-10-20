package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/spf13/cobra"
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

func runCheck(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("🔍 Checking dotfiles repository...")
	fmt.Printf("📁 Repository: %s\n", cfg.Git.RepoPath)
	fmt.Printf("🖥️  Machine: %s\n", cfg.Machine.Name)

	issues := []string{}
	warnings := []string{}

	// Check daemon status
	if isDaemonRunning() {
		fmt.Println("✅ Daemon is running")
		if checkVerbose {
			if pid, err := getDaemonPID(); err == nil {
				fmt.Printf("   PID: %d\n", pid)
			}
		}
	} else {
		warnings = append(warnings, "Daemon is not running")
	}

	// Check git repository status
	ctx := context.Background()
	gmCfg := cfg.ToGitManagerConfig()
	gitMgr, err := gitmanager.NewGitManager(ctx, gmCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize git manager: %w", err)
	}

	// Check if repository exists and is accessible
	repo := gitMgr.Repo()
	if repo == nil {
		issues = append(issues, "Git repository is not accessible")
	} else {
		fmt.Println("✅ Git repository is accessible")

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
					fmt.Println("✅ Working tree is clean")
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

	// Check remote configuration
	if cfg.Git.RemoteURL != "" {
		fmt.Println("🌐 Remote URL configured")
		if checkVerbose {
			fmt.Printf("   Remote URL: %s\n", cfg.Git.RemoteURL)
		}
	} else {
		warnings = append(warnings, "No remote URL configured")
	}

	// Check configuration
	fmt.Println("⚙️  Checking configuration...")
	if cfg.Sync.AutoSyncEnabled {
		fmt.Println("✅ Auto-sync is enabled")
		if checkVerbose {
			fmt.Printf("   Pull interval: %d seconds\n", cfg.Sync.PullIntervalSeconds)
			fmt.Printf("   Debounce: %d seconds\n", cfg.Sync.DebounceSeconds)
		}
	} else {
		warnings = append(warnings, "Auto-sync is disabled")
	}

	// Check file mappings
	if len(cfg.Mappings) > 0 {
		fmt.Printf("✅ %d file mappings configured\n", len(cfg.Mappings))
		if checkVerbose {
			for source, target := range cfg.Mappings {
				fmt.Printf("   %s -> %s\n", source, target)
			}
		}
	} else {
		warnings = append(warnings, "No file mappings configured")
	}

	// Check for conflicts in .dsm/conflicts directory
	conflictDir := filepath.Join(cfg.Git.RepoPath, ".dsm", "conflicts")
	if stat, err := os.Stat(conflictDir); err == nil && stat.IsDir() {
		entries, err := os.ReadDir(conflictDir)
		if err == nil && len(entries) > 0 {
			issues = append(issues, fmt.Sprintf("Found conflict artifacts in %s", conflictDir))
			fmt.Printf("❌ Conflict artifacts found: %s\n", conflictDir)
			fmt.Println("💡 Run 'dsm conflicts' for more details")
			fmt.Println("💡 Run 'dsm resolve' after manually resolving conflicts")
		} else {
			fmt.Println("✅ No conflict artifacts found")
		}
	} else {
		fmt.Println("✅ No conflict artifacts found")
	}

	// Summary
	fmt.Println("\n" + strings.Repeat("=", 50))
	if len(issues) > 0 {
		fmt.Printf("❌ Found %d issue(s):\n", len(issues))
		for _, issue := range issues {
			fmt.Printf("   • %s\n", issue)
		}
	}

	if len(warnings) > 0 {
		fmt.Printf("⚠️  Found %d warning(s):\n", len(warnings))
		for _, warning := range warnings {
			fmt.Printf("   • %s\n", warning)
		}
	}

	if len(issues) == 0 && len(warnings) == 0 {
		fmt.Println("✅ Everything looks good!")
	} else if len(issues) == 0 {
		fmt.Println("✅ No critical issues found (some warnings present)")
	}

	// Return appropriate exit code
	if len(issues) > 0 {
		return fmt.Errorf("found %d issue(s) that need attention", len(issues))
	}

	return nil
}