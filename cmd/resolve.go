package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// resolveCmd represents the resolve command
var resolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Mark conflicts as resolved and resume sync",
	Long: `Mark merge conflicts as resolved and resume automatic syncing. This command
should be run after manually resolving all merge conflicts.

Examples:
  dsm resolve
  dsm resolve --start  # Also restart the daemon after resolving`,
	RunE: runResolve,
}

var resolveStart bool

func init() {
	rootCmd.AddCommand(resolveCmd)
	resolveCmd.Flags().BoolVar(&resolveStart, "start", false, "Start daemon after resolving conflicts")
}

func runResolve(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("🔍 Checking for conflicts...")

	// Check for conflict artifacts
	conflictDir := filepath.Join(cfg.Git.RepoPath, ".dsm", "conflicts")
	stat, err := os.Stat(conflictDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("✅ No conflict artifacts found")
		} else {
			return fmt.Errorf("failed to access conflict directory: %w", err)
		}
	} else if !stat.IsDir() {
		fmt.Println("✅ No conflict artifacts found")
	} else {
		entries, err := os.ReadDir(conflictDir)
		if err != nil {
			return fmt.Errorf("failed to read conflict directory: %w", err)
		}

		if len(entries) > 0 {
			fmt.Printf("⚠️  Found %d conflict artifact(s) still present:\n", len(entries))
			for _, entry := range entries {
				fmt.Printf("   • %s\n", entry.Name())
			}
			fmt.Println()
			fmt.Println("💡 Consider cleaning up resolved conflict artifacts")
			fmt.Printf("💡 Conflict artifacts directory: %s\n", conflictDir)
			fmt.Println()
		} else {
			fmt.Println("✅ No conflict artifacts found")
		}
	}

	// Check daemon status
	daemonWasRunning := isDaemonRunning()
	if daemonWasRunning {
		fmt.Println("⚠️  Warning: Daemon is currently running")
		fmt.Println("   It's recommended to restart the daemon after conflict resolution")
	} else {
		fmt.Println("✅ Daemon is not running")
	}

	fmt.Println()
	fmt.Println("🎉 Conflict resolution check completed!")

	// Offer to start daemon
	if resolveStart && !daemonWasRunning {
		fmt.Println("🚀 Starting daemon...")
		startArgs := []string{}
		startCmd, _, err := rootCmd.Find([]string{"start"})
		if err != nil {
			return fmt.Errorf("failed to find start command: %w", err)
		}
		startCmd.SetArgs(startArgs)

		if err := startCmd.Execute(); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}
	} else if !daemonWasRunning {
		fmt.Println("💡 Run 'dsm start' to resume automatic syncing")
	} else {
		fmt.Println("💡 Run 'dsm restart' to restart the daemon with clean state")
	}

	return nil
}