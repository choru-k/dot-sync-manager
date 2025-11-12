package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/status"
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
	fmt.Println("📊 Dotfile Sync Manager Status")
	fmt.Println("=" + strings.Repeat("=", statusSeparatorWidth))

	// Try to get rich status from Unix socket first
	daemonStatus, err := status.GetStatusFromSocket()
	if err == nil && daemonStatus != nil {
		// Daemon is running with status server
		showRichDaemonStatus(daemonStatus)

		// Load configuration for additional info
		cfg, err := getConfig()
		if err != nil {
			fmt.Printf("\n⚠️  Could not load configuration: %v\n", err)
			return nil
		}

		showConfigurationInfo(cfg)
		showGitStatus(cfg.Git.RepoPath)
		showFileMappings(cfg.Mappings)
		return nil
	}

	// Fallback to basic status checking
	fmt.Println("🔴 Daemon Status Server: Not available")

	// Check if daemon is running using old method
	if isDaemonRunning() {
		fmt.Println("🟡 Daemon: Running (legacy mode)")
		pid, err := getDaemonPID()
		if err == nil {
			fmt.Printf("📋 PID: %d\n", pid)
		}
		fmt.Println("💡 Tip: Restart daemon to get rich status reporting")
	} else {
		fmt.Println("🔴 Daemon: Not running")
	}

	// Load configuration and show basic info
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	showConfigurationInfo(cfg)
	showGitStatus(cfg.Git.RepoPath)
	showFileMappings(cfg.Mappings)

	return nil
}

// showRichDaemonStatus displays detailed daemon status from the Unix socket
func showRichDaemonStatus(daemonStatus *status.DaemonStatus) {
	// Daemon status with state-specific emoji
	var stateEmoji string
	switch daemonStatus.CurrentState {
	case status.StateRunning:
		stateEmoji = "🟢"
	case status.StateSyncing:
		stateEmoji = "🔄"
	case status.StateIdle:
		stateEmoji = "⏸️"
	case status.StateError:
		stateEmoji = "🔴"
	case status.StateStarting:
		stateEmoji = "🚀"
	case status.StateStopping:
		stateEmoji = "🛑"
	default:
		stateEmoji = "❓"
	}

	fmt.Printf("%s Daemon: %s (PID: %d)\n", stateEmoji, daemonStatus.CurrentState, daemonStatus.PID)
	fmt.Printf("⏱️  Uptime: %s\n", formatDuration(daemonStatus.Uptime))
	fmt.Printf("📦 Version: %s\n", daemonStatus.Version)

	// Sync statistics
	fmt.Printf("\n📈 Sync Statistics:\n")
	fmt.Printf("🔄 Total syncs: %d\n", daemonStatus.SyncCount)
	fmt.Printf("❌ Error count: %d\n", daemonStatus.ErrorCount)
	fmt.Printf("📁 Files synced: %d\n", daemonStatus.FilesSynced)

	// Last sync information
	if !daemonStatus.LastSync.IsZero() {
		fmt.Printf("🕐 Last sync: %s ago\n", formatDuration(time.Since(daemonStatus.LastSync)))
		fmt.Printf("📝 Last result: %s\n", daemonStatus.LastSyncResult)
	} else {
		fmt.Printf("🕐 Last sync: Never\n")
	}

	// Error information
	if daemonStatus.CurrentState == status.StateError && daemonStatus.LastError != "" {
		fmt.Printf("\n⚠️  Last Error: %s\n", daemonStatus.LastError)
	}

	// Watched paths
	if len(daemonStatus.WatchedPaths) > 0 {
		fmt.Printf("\n👁️  Watching %d paths:\n", len(daemonStatus.WatchedPaths))
		for i, path := range daemonStatus.WatchedPaths {
			if i >= 5 { // Limit to first 5 paths
				fmt.Printf("  ... and %d more paths\n", len(daemonStatus.WatchedPaths)-5)
				break
			}
			fmt.Printf("  📂 %s\n", path)
		}
	}

	fmt.Printf("\n⚙️  Configuration: %s\n", daemonStatus.ConfigPath)
}

// showConfigurationInfo displays configuration information
func showConfigurationInfo(cfg *config.SyncConfig) {
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
}

// showFileMappings displays file mappings
func showFileMappings(mappings map[string]string) {
	if len(mappings) > 0 {
		fmt.Printf("\n🔗 File Mappings (%d):\n", len(mappings))
		sources := make([]string, 0, len(mappings))
		for source := range mappings {
			sources = append(sources, source)
		}
		sort.Strings(sources)

		for _, source := range sources {
			fmt.Printf("  %s -> %s\n", source, mappings[source])
		}
	}
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), float64(int(d.Seconds())%60))
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}

func showGitStatus(repoPath string) {
	fmt.Printf("\n📂 Repository Status:\n")
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
	}
}
