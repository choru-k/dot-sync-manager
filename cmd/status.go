package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/config"
	git "github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"
)

const (
	// statusSeparatorWidth defines the width of separator lines in status output
	statusSeparatorWidth = 30
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	Long: `Show the current status of the dotfile sync daemon and repository.

Examples:
  dsm status`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("📊 Dotfile Sync Manager Status")
	fmt.Println("=" + strings.Repeat("=", statusSeparatorWidth))

	// Check daemon status
	if isDaemonRunning() {
		fmt.Println("🟢 Daemon: Running")
		pid, err := getDaemonPID()
		if err == nil {
			fmt.Printf("📋 PID: %d\n", pid)
		}
	} else {
		fmt.Println("🔴 Daemon: Not running")
	}

	// Show configuration info
	fmt.Printf("\n⚙️  Configuration:\n")
	fmt.Printf("📁 Repository: %s\n", cfg.Git.RepoPath)
	fmt.Printf("🖥️  Machine: %s\n", cfg.Machine.Name)
	fmt.Printf("🔄 Auto-sync: %v\n", cfg.Sync.AutoSyncEnabled)
	fmt.Printf("⏱️  Pull interval: %d seconds\n", cfg.Sync.PullIntervalSeconds)
	fmt.Printf("⏱️  Debounce: %d seconds\n", cfg.Sync.DebounceSeconds)

	if cfg.Git.RemoteURL != "" {
		fmt.Printf("🌐 Remote: %s\n", cfg.Git.RemoteURL)
		fmt.Printf("🌿 Branch: %s\n", cfg.Git.Branch)
	} else {
		fmt.Println("🌐 Remote: Not configured")
	}

	// Show git status (simplified)
	fmt.Printf("\n📂 Repository Status:\n")
	showGitStatus(cfg.Git.RepoPath, checkVerbose)

	// Show mappings if any
	if len(cfg.Mappings) > 0 {
		fmt.Printf("\n🔗 File Mappings (%d):\n", len(cfg.Mappings))
		sources := make([]string, 0, len(cfg.Mappings))
		for source := range cfg.Mappings {
			sources = append(sources, source)
		}
		sort.Strings(sources)

		for _, source := range sources {
			fmt.Printf("  %s -> %s\n", source, cfg.Mappings[source])
		}
	}

	// Show verbose information
	if verbose {
		showVerboseStatus(cfg)
	}

	return nil
}

func showGitStatus(repoPath string, verbose bool) {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		if strings.Contains(err.Error(), "repository not found") {
			fmt.Printf("❌ Not a git repository: %s\n", repoPath)
		} else {
			fmt.Printf("❓ Git status unavailable: %v\n", err)
		}
		return
	}

	w, err := r.Worktree()
	if err != nil {
		fmt.Printf("❓ Git worktree unavailable: %v\n", err)
		return
	}

	status, err := w.Status()
	if err != nil {
		fmt.Printf("❓ Git status unavailable: %v\n", err)
		return
	}

	if status.IsClean() {
		fmt.Println("✅ Repository is clean")
	} else {
		fmt.Printf("📝 %d files have changes\n", len(status))

		// Show detailed file status in verbose mode
		if verbose {
			for filepath, fileStatus := range status {
				var statusSymbol string
				switch fileStatus.Worktree {
				case git.Modified:
					statusSymbol = "📝 Modified"
				case git.Added:
					statusSymbol = "➕ Added"
				case git.Deleted:
					statusSymbol = "❌ Deleted"
				case git.Renamed:
					statusSymbol = "🔄 Renamed"
				case git.Copied:
					statusSymbol = "📋 Copied"
				case git.Untracked:
					statusSymbol = "❓ Untracked"
				default:
					statusSymbol = "📍 Changed"
				}
				fmt.Printf("   %s: %s\n", statusSymbol, filepath)
			}
		}
	}
}

func showVerboseStatus(cfg *config.SyncConfig) {
	fmt.Printf("\n🔍 Detailed Information:\n")

	// Configuration file location
	fmt.Printf("📄 Config file: %s\n", cfg.GetConfigPath())

	// Authentication details
	fmt.Printf("🔐 Authentication: %s", cfg.Git.AuthType)
	if cfg.Git.AuthType == "ssh" {
		if cfg.Git.SSHKeyPath != "" {
			fmt.Printf(" (key: %s)", cfg.Git.SSHKeyPath)
		} else {
			fmt.Printf(" (using SSH agent)")
		}
	}
	fmt.Println()

	// Extended sync settings
	fmt.Printf("⚙️  Extended Sync Settings:\n")
	fmt.Printf("   Auto-commit: %v\n", cfg.Sync.AutoCommit)
	fmt.Printf("   Auto-push: %v\n", cfg.Sync.AutoPush)
	fmt.Printf("   Auto-pull: %v\n", cfg.Sync.AutoPull)

	// Backoff settings if configured
	if cfg.Sync.Backoff != nil {
		fmt.Printf("⏱️  Backoff Settings:\n")
		fmt.Printf("   Max delay: %ds\n", cfg.Sync.Backoff.MaxDelaySeconds)
		fmt.Printf("   Churn threshold: %d changes\n", cfg.Sync.Backoff.ChurnThreshold)
		fmt.Printf("   Churn window: %ds\n", cfg.Sync.Backoff.ChurnWindowSeconds)
		fmt.Printf("   Decay reset: %ds\n", cfg.Sync.Backoff.DecayResetSeconds)
	}
}
