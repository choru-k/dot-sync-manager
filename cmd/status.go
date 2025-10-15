package cmd

import (
	"fmt"
	"strings"

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
	showGitStatus(cfg.Git.RepoPath)

	// Show mappings if any
	if len(cfg.Mappings) > 0 {
		fmt.Printf("\n🔗 File Mappings (%d):\n", len(cfg.Mappings))
		for source, target := range cfg.Mappings {
			fmt.Printf("  %s -> %s\n", source, target)
		}
	}

	return nil
}

func showGitStatus(repoPath string) {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		fmt.Printf("❓ Git status unavailable: %v\n", err)
		return
	}

	w, err := r.Worktree()
	if err != nil {
		fmt.Printf("❓ Git status unavailable: %v\n", err)
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
