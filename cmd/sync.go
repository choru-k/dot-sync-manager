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

// displaySyncSummary shows summary statistics for dry-run mode
func displaySyncSummary(addedCount, modifiedCount, deletedCount int) {
	totalFiles := addedCount + modifiedCount + deletedCount
	fmt.Println("\nSummary:")
	fmt.Printf("  %d files total\n", totalFiles)
	if addedCount > 0 {
		fmt.Printf("  %d added\n", addedCount)
	}
	if modifiedCount > 0 {
		fmt.Printf("  %d modified\n", modifiedCount)
	}
	if deletedCount > 0 {
		fmt.Printf("  %d deleted\n", deletedCount)
	}
}

// categorizeFilesByOperation groups files by their git operation type
func categorizeFilesByOperation(status git.Status) (added, modified, deleted []string) {
	for path, fileStatus := range status {
		// Skip unmodified files
		if fileStatus.Worktree == git.Unmodified && fileStatus.Staging == git.Unmodified {
			continue
		}

		// Determine operation type
		switch {
		case fileStatus.Worktree == git.Untracked:
			// Untracked files are new additions
			added = append(added, path)
		case fileStatus.Worktree == git.Added || fileStatus.Staging == git.Added:
			added = append(added, path)
		case fileStatus.Worktree == git.Deleted || fileStatus.Staging == git.Deleted:
			deleted = append(deleted, path)
		default:
			modified = append(modified, path)
		}
	}

	// Sort for deterministic output
	sort.Strings(added)
	sort.Strings(modified)
	sort.Strings(deleted)

	return added, modified, deleted
}

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Trigger manual sync",
	Long: `Manually trigger a sync operation. This will stage all changes, create a commit,
and push to the remote repository.

Examples:
  dsm sync
  dsm --dry-run sync  # Show what would be synced without actually doing it`,
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
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
		return fmt.Errorf("failed to prepare git repository: %w\nHint: Check that the repository path exists and is a valid git repository\nRun 'dsm validate-config' to verify your configuration", err)
	}

	if isDryRun() {
		PrintDryRun("Dry run mode - no changes will be made")

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

		// Categorize files by operation type
		addedFiles, modifiedFiles, deletedFiles := categorizeFilesByOperation(status)

		// Display files by operation type
		if len(addedFiles) > 0 {
			fmt.Println("\nWould add:")
			for _, path := range addedFiles {
				fmt.Printf(" • %s\n", path)
			}
		}

		if len(modifiedFiles) > 0 {
			fmt.Println("\nWould modify:")
			for _, path := range modifiedFiles {
				fmt.Printf(" • %s\n", path)
			}
		}

		if len(deletedFiles) > 0 {
			fmt.Println("\nWould delete:")
			for _, path := range deletedFiles {
				fmt.Printf(" • %s\n", path)
			}
		}

		// Build and display commit message preview
		// Combine all changed files for commit message
		allChangedFiles := make([]string, 0, len(addedFiles)+len(modifiedFiles)+len(deletedFiles))
		allChangedFiles = append(allChangedFiles, addedFiles...)
		allChangedFiles = append(allChangedFiles, modifiedFiles...)
		allChangedFiles = append(allChangedFiles, deletedFiles...)
		sort.Strings(allChangedFiles)

		commitMessage := gitmanager.BuildAutoCommitMessage(time.Now(), allChangedFiles)
		fmt.Println("\nWould create commit:")
		fmt.Println(commitMessage)

		// Display statistics summary
		displaySyncSummary(len(addedFiles), len(modifiedFiles), len(deletedFiles))

		if cfg.Git.RemoteURL != "" {
			fmt.Println("\n📤 Would push to remote repository")
			fmt.Printf("   Remote: %s\n", cfg.Git.RemoteURL)
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
