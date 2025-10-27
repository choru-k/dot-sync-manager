package cmd

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	git "github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"
)

const (
	// syncTimeout is the maximum time to wait for sync operations
	syncTimeout = 5 * time.Minute
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

	if isDaemonRunning() {
		fmt.Println("ℹ️  Note: Daemon is running; manual sync will run alongside background sync")
	}

	gmCfg := cfg.ToGitManagerConfig()
	ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
	defer cancel()

	gitMgr, err := gitmanager.NewGitManager(ctx, gmCfg)
	if err != nil {
		return fmt.Errorf("failed to prepare git repository: %w", err)
	}

	if dryRun {
		fmt.Println("🔍 Dry run mode - no changes will be made")

		// Open the worktree and get status
		worktree, err := gitMgr.Repo().Worktree()
		if err != nil {
			return fmt.Errorf("failed to open worktree: %w", err)
		}

		status, err := worktree.Status()
		if err != nil {
			return fmt.Errorf("failed to read repository status: %w", err)
		}

		// Handle the clean case
		if status.IsClean() {
			fmt.Println("✅ No changes to sync")
			return nil
		}

		// Collect changed paths into a slice
		var changedPaths []string
		for path, fileStatus := range status {
			if fileStatus.Worktree == git.Unmodified && fileStatus.Staging == git.Unmodified {
				continue
			}
			changedPaths = append(changedPaths, path)
		}

		// Sort the paths for deterministic output
		sort.Strings(changedPaths)

		// Print the sorted list
		for _, path := range changedPaths {
			fmt.Printf(" • %s\n", path)
		}

		if cfg.Git.RemoteURL != "" {
			fmt.Println("📤 Would push to remote repository")
		}
		return nil
	}

	// Stage and commit changes
	changed, err := gitMgr.StageAndCommit(ctx, timeNow())
	if err != nil {
		return fmt.Errorf("failed to stage or commit changes: %w", err)
	}

	if len(changed) == 0 {
		fmt.Println("✅ No changes to sync")
		return nil
	}

	fmt.Println("✅ Manual sync completed")
	fmt.Println("📋 Committed files:")
	for _, file := range changed {
		fmt.Printf(" • %s\n", file)
	}

	// Push only if remote URL is configured
	if cfg.Git.RemoteURL != "" {
		if err := gitMgr.Push(ctx); err != nil {
			return fmt.Errorf("failed to push changes: %w", err)
		}
	} else {
		fmt.Println("ℹ️  No remote configured, skipping push")
	}

	return nil
}
