package cmd

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/choru-k/dot-sync-manager/internal/conflict"
	"github.com/spf13/cobra"
)

// conflictsCmd represents the conflicts command
var conflictsCmd = &cobra.Command{
	Use:   "conflicts",
	Short: "List all active conflicts",
	Long: `Display all active file conflicts that need resolution.

Use --json for machine-readable output.

Examples:
  dsm conflicts
  dsm conflicts --json`,
	RunE: runConflicts,
}

var conflictsJSON bool

func init() {
	rootCmd.AddCommand(conflictsCmd)
	conflictsCmd.Flags().BoolVar(&conflictsJSON, "json", false, "Output in JSON format")
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

	// Create conflict service
	svc := conflict.NewService(nil, cfg)
	conflicts, err := svc.CheckForConflicts()
	if err != nil {
		return fmt.Errorf("failed to check for conflicts: %w", err)
	}

	if conflictsJSON {
		return outputConflictsJSON(cmd, conflicts)
	}

	return outputConflictsTable(cmd, conflicts)
}

// conflictsJSONOutput represents the JSON output structure for conflicts command.
type conflictsJSONOutput struct {
	Count     int                     `json:"count"`
	Conflicts []conflict.ConflictInfo `json:"conflicts"`
}

func outputConflictsJSON(cmd *cobra.Command, conflicts []conflict.ConflictInfo) error {
	output := conflictsJSONOutput{
		Count:     len(conflicts),
		Conflicts: conflicts,
	}

	// Ensure empty array instead of null
	if output.Conflicts == nil {
		output.Conflicts = []conflict.ConflictInfo{}
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func outputConflictsTable(cmd *cobra.Command, conflicts []conflict.ConflictInfo) error {
	if len(conflicts) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No active conflicts.")
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Found %d conflict(s):\n\n", len(conflicts))

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "FILE\tDETECTED\tHAS BASE")

	for _, c := range conflicts {
		hasBase := "No"
		if c.HasBase {
			hasBase = "Yes"
		}
		detected := c.DetectedAt.Format("2006-01-02 15:04")
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", c.File, detected, hasBase)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout())
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Use 'dsm resolve' to resolve conflicts.")
	return nil
}
