package cmd

import (
	"fmt"

	"github.com/choru-k/dot-sync-manager/internal/conflict"
	"github.com/spf13/cobra"
)

// resolveCmd represents the resolve command.
var resolveCmd = &cobra.Command{
	Use:   "resolve [file]",
	Short: "Resolve file conflicts",
	Long: `Resolve file conflicts using local or remote versions.

Without flags, marks specified file as resolved (manual resolution assumed).
With --use-local or --use-remote, applies the chosen version.
Without arguments and with flags, resolves all conflicts.

Examples:
  dsm resolve .bashrc            # Mark specific file as resolved
  dsm resolve .bashrc --use-local  # Use local version for specific file
  dsm resolve --use-local        # Use local versions for all conflicts
  dsm resolve --use-remote       # Use remote versions for all conflicts
  dsm resolve --all              # Mark all conflicts as resolved`,
	RunE: runResolve,
}

var (
	resolveUseLocal  bool
	resolveUseRemote bool
	resolveAll       bool
)

func init() {
	rootCmd.AddCommand(resolveCmd)
	resolveCmd.Flags().BoolVar(&resolveUseLocal, "use-local", false, "Use local version for conflicts")
	resolveCmd.Flags().BoolVar(&resolveUseRemote, "use-remote", false, "Use remote version for conflicts")
	resolveCmd.Flags().BoolVar(&resolveAll, "all", false, "Mark all conflicts as resolved")
}

func runResolve(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create conflict service. Passing nil for git manager is safe here
	// because the file-based resolution methods don't require git operations.
	svc := conflict.NewService(nil, cfg)
	conflicts, err := svc.CheckForConflicts()
	if err != nil {
		return fmt.Errorf("failed to check for conflicts: %w", err)
	}

	if len(conflicts) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No conflicts to resolve.")
		return nil
	}

	// Validate mutually exclusive flags
	flagCount := 0
	if resolveUseLocal {
		flagCount++
	}
	if resolveUseRemote {
		flagCount++
	}
	if resolveAll {
		flagCount++
	}
	if flagCount > 1 {
		return fmt.Errorf("flags --use-local, --use-remote, and --all are mutually exclusive")
	}

	// Handle specific file argument
	if len(args) > 0 {
		return resolveSpecificFile(cmd, svc, args[0])
	}

	// Handle flags for all conflicts
	if resolveUseLocal {
		return resolveAllWithLocal(cmd, svc, conflicts)
	}
	if resolveUseRemote {
		return resolveAllWithRemote(cmd, svc, conflicts)
	}
	if resolveAll {
		return markAllResolved(cmd, svc)
	}

	// Default: show conflicts and hint for flags
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Found %d conflict(s):\n", len(conflicts))
	for _, c := range conflicts {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", c.File)
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Use --use-local or --use-remote to resolve all, or specify a file.")
	return nil
}

func resolveSpecificFile(cmd *cobra.Command, svc *conflict.Service, file string) error {
	if resolveUseLocal {
		if err := svc.UseLocal(file); err != nil {
			return fmt.Errorf("failed to use local: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Resolved %s with local version.\n", file)
		return nil
	}
	if resolveUseRemote {
		if err := svc.UseRemote(file); err != nil {
			return fmt.Errorf("failed to use remote: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Resolved %s with remote version.\n", file)
		return nil
	}

	// Mark as resolved (manual resolution assumed)
	if err := svc.MarkResolved([]string{file}); err != nil {
		return fmt.Errorf("failed to mark resolved: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Marked %s as resolved.\n", file)
	return nil
}

func resolveAllWithLocal(cmd *cobra.Command, svc *conflict.Service, conflicts []conflict.ConflictInfo) error {
	var resolved int
	var hasErrors bool
	for _, c := range conflicts {
		if err := svc.UseLocal(c.File); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to resolve %s: %v\n", c.File, err)
			hasErrors = true
			continue
		}
		resolved++
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Resolved %d conflict(s) with local versions.\n", resolved)
	if hasErrors {
		return fmt.Errorf("one or more files failed to resolve")
	}
	return nil
}

func resolveAllWithRemote(cmd *cobra.Command, svc *conflict.Service, conflicts []conflict.ConflictInfo) error {
	var resolved int
	var hasErrors bool
	for _, c := range conflicts {
		if err := svc.UseRemote(c.File); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to resolve %s: %v\n", c.File, err)
			hasErrors = true
			continue
		}
		resolved++
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Resolved %d conflict(s) with remote versions.\n", resolved)
	if hasErrors {
		return fmt.Errorf("one or more files failed to resolve")
	}
	return nil
}

func markAllResolved(cmd *cobra.Command, svc *conflict.Service) error {
	if err := svc.MarkAllResolved(); err != nil {
		return fmt.Errorf("failed to mark all resolved: %w", err)
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Marked all conflicts as resolved.")
	return nil
}
