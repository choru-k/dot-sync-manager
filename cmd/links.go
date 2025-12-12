// Package cmd implements CLI commands for the dotfile sync manager.
// This file defines the links command for listing and verifying symlink mappings.
package cmd

import (
	"fmt"
	"sort"

	"github.com/choru-k/dot-sync-manager/internal/symlink"
	"github.com/spf13/cobra"
)

var linksCmd = &cobra.Command{
	Use:   "links [verify]",
	Short: "List and verify symlink mappings",
	Long: `List all symlink mappings with their current status.

Use 'links verify' to check all symlinks are valid and report any issues.

Examples:
  dsm links         # List all mappings with status
  dsm links verify  # Verify all symlinks are valid`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLinks,
}

func init() {
	rootCmd.AddCommand(linksCmd)
}

// runLinks is the entry point for the links command, routing to list or verify subcommands
func runLinks(cmd *cobra.Command, args []string) error {
	if len(args) > 0 && args[0] == "verify" {
		return verifyLinks(cmd)
	}
	return listLinks(cmd)
}

// listLinks displays all symlink mappings with their current status
func listLinks(cmd *cobra.Command) error {
	// Load config
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create symlink manager
	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create symlink manager: %w", err)
	}

	// Get mappings with status
	statuses := mgr.VerifyMappings()

	if len(statuses) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No symlink mappings found.")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Use 'dsm link' to create symlink mappings.")
		return nil
	}

	// Sort by repo path for consistent output
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].RepoPath < statuses[j].RepoPath
	})

	// Print header
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "REPO FILE          TARGET                    STATUS")
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "─────────          ──────                    ──────")

	// Print each mapping
	for _, status := range statuses {
		emoji := statusEmoji(status.Status)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-18s %-25s %s %s\n",
			status.RepoPath,
			status.TargetPath,
			emoji,
			status.Status)
	}

	return nil
}

// verifyLinks checks all symlink mappings and reports any issues
func verifyLinks(cmd *cobra.Command) error {
	// Load config
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create symlink manager
	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create symlink manager: %w", err)
	}

	// Verify all mappings
	statuses := mgr.VerifyMappings()

	if len(statuses) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No symlink mappings to verify.")
		return nil
	}

	// Count status types
	var validCount, brokenCount, missingCount, notSymlinkCount int
	for _, status := range statuses {
		switch status.Status {
		case symlink.StateValid:
			validCount++
		case symlink.StateBroken:
			brokenCount++
		case symlink.StateMissing:
			missingCount++
		case symlink.StateNotSymlink:
			notSymlinkCount++
		}
	}

	// Print summary
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Verification Summary:\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ✅ Valid:       %d\n", validCount)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ❌ Broken:      %d\n", brokenCount)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ⚠️  Missing:     %d\n", missingCount)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ⚠️  Not Symlink: %d\n", notSymlinkCount)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ──────────────────\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Total:         %d\n", len(statuses))

	// Return error if any issues found
	if brokenCount > 0 || missingCount > 0 || notSymlinkCount > 0 {
		return fmt.Errorf("found %d broken, %d missing, %d not symlink mappings", brokenCount, missingCount, notSymlinkCount)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\n✅ All symlink mappings are valid.")
	return nil
}

func statusEmoji(state symlink.MappingState) string {
	switch state {
	case symlink.StateValid:
		return "✅"
	case symlink.StateBroken:
		return "❌"
	case symlink.StateMissing:
		return "⚠️"
	case symlink.StateNotSymlink:
		return "⚠️"
	default:
		return "❓"
	}
}
