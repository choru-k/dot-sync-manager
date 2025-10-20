package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// conflictsCmd represents the conflicts command
var conflictsCmd = &cobra.Command{
	Use:   "conflicts",
	Short: "Show detailed conflict information",
	Long: `Show detailed information about any merge conflicts in the dotfiles repository.
This command provides in-depth analysis of conflicts and helps with resolution.

Examples:
  dsm conflicts
  dsm conflicts --verbose  # Show detailed conflict information`,
	RunE: runConflicts,
}

var conflictsVerbose bool

func init() {
	rootCmd.AddCommand(conflictsCmd)
	conflictsCmd.Flags().BoolVar(&conflictsVerbose, "verbose", false, "Show detailed conflict information")
}

func runConflicts(cmd *cobra.Command, args []string) error {
	// conflicts command should not accept any arguments
	if len(args) > 0 {
		return fmt.Errorf("conflicts command accepts no arguments")
	}

	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("🔍 Checking for merge conflicts...")
	fmt.Printf("📁 Repository: %s\n", cfg.Git.RepoPath)

	// Check for conflict artifacts in .dsm/conflicts directory
	conflictDir := filepath.Join(cfg.Git.RepoPath, ".dsm", "conflicts")
	stat, err := os.Stat(conflictDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("✅ No conflict artifacts found")
			return nil
		}
		return fmt.Errorf("failed to access conflict directory: %w", err)
	}
	if !stat.IsDir() {
		fmt.Println("✅ No conflict artifacts found")
		return nil
	}

	// List conflict directories
	entries, err := os.ReadDir(conflictDir)
	if err != nil {
		return fmt.Errorf("failed to read conflict directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("✅ No conflict artifacts found")
		return nil
	}

	// Sort entries by name (timestamp)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	fmt.Printf("❌ Found %d conflict artifact(s):\n\n", len(entries))

	for i, entry := range entries {
		conflictPath := filepath.Join(conflictDir, entry.Name())
		fmt.Printf("Conflict %d/%d: %s\n", i+1, len(entries), entry.Name())
		fmt.Println(strings.Repeat("-", 40))

		if conflictsVerbose {
			// Show details about conflict artifacts
			if err := showConflictDetails(conflictPath); err != nil {
				fmt.Printf("⚠️  Could not show details: %v\n", err)
			}
		} else {
			fmt.Printf("💡 Run 'dsm conflicts --verbose' for detailed information\n")
		}

		fmt.Println()
	}

	// Show resolution guidance
	fmt.Println("📋 Resolution steps:")
	fmt.Println("1. Open the conflict directory to see artifact files")
	fmt.Println("2. Review .local, .remote, and .base files")
	fmt.Println("3. Manually resolve conflicts in the actual files")
	fmt.Println("4. Stage the resolved files: git add <file>")
	fmt.Println("5. Commit the resolution: git commit")
	fmt.Println("6. Run 'dsm resolve' to mark conflicts as resolved")
	fmt.Println()
	fmt.Printf("💡 Conflict artifacts directory: %s\n", conflictDir)

	// Check if daemon is running (it should be paused during conflicts)
	if isDaemonRunning() {
		fmt.Println("⚠️  Warning: Daemon is still running")
		fmt.Println("   Consider running 'dsm stop' while resolving conflicts")
	} else {
		fmt.Println("✅ Daemon is not running (good for conflict resolution)")
	}

	return nil
}

func showConflictDetails(conflictPath string) error {
	fmt.Printf("Directory: %s\n", conflictPath)

	// List conflict artifact files
	entries, err := os.ReadDir(conflictPath)
	if err != nil {
		return fmt.Errorf("failed to read conflict directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("   (empty)")
		return nil
	}

	fmt.Println("   Artifact files:")
	for _, entry := range entries {
		if !entry.IsDir() {
			fullPath := filepath.Join(conflictPath, entry.Name())
			if info, err := os.Stat(fullPath); err == nil {
				fmt.Printf("   • %s (%d bytes)\n", entry.Name(), info.Size())
			} else {
				fmt.Printf("   • %s (error getting size)\n", entry.Name())
			}
		}
	}

	return nil
}