package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version information - this will be set during build
	Version   = "dev"
	Commit   = "unknown"
	BuildDate = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long: `Show version information for the Dotfile Sync Manager.
This displays the current version, commit hash, and build date.

Examples:
  dsm version`,
	RunE: runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, args []string) error {
	fmt.Println("Dotfile Sync Manager (DSM)")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Commit: %s\n", Commit)
	fmt.Printf("Built: %s\n", BuildDate)

	if Version == "dev" {
		fmt.Println()
		fmt.Println("💡 This is a development version")
		fmt.Println("   Use 'dsm status' to check daemon status")
		fmt.Println("   Use 'dsm help' for available commands")
	}

	return nil
}