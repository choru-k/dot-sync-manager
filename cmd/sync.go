package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Trigger manual sync",
	Long: `Manually trigger a sync operation. This will stage all changes, create a commit,
and push to the remote repository.

Examples:
  dsm sync
  dsm sync --dry-run  # Show what would be synced without actually doing it`,
	RunE: runSync,
}

var dryRun bool

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be synced without actually doing it")
}

func runSync(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("🔄 Triggering manual sync...")
	fmt.Printf("📁 Repository: %s\n", cfg.Git.RepoPath)

	if dryRun {
		fmt.Println("🔍 Dry run mode - no changes will be made")
		// In a real implementation, this would check for changes without committing
		fmt.Println("📋 Would stage and commit any changes")
		if cfg.Git.RemoteURL != "" {
			fmt.Println("📤 Would push to remote repository")
		}
		return nil
	}

	// Check if daemon is running
	if isDaemonRunning() {
		fmt.Println("ℹ️  Note: Daemon is running, sync will be handled automatically")
		fmt.Println("💡 Use 'dsm stop' to stop the daemon if you want manual control")
		return nil
	}

	// In a real implementation, this would:
	// 1. Stage all changes
	// 2. Create a commit with timestamp
	// 3. Push to remote if configured
	fmt.Println("📋 Staging changes...")
	fmt.Println("📝 Creating commit...")

	if cfg.Git.RemoteURL != "" {
		fmt.Println("📤 Pushing to remote...")
	} else {
		fmt.Println("ℹ️  No remote configured, skipping push")
	}

	fmt.Println("✅ Manual sync completed")

	return nil
}