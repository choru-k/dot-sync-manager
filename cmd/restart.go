package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// restartCmd represents the restart command
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart dotfile sync daemon",
	Long: `Restart the dotfile sync daemon. This is equivalent to running 'dsm stop'
followed by 'dsm start'.

Examples:
  dsm restart
  dsm restart --foreground  # Restart in foreground (for debugging)`,
	RunE: runRestart,
}

var restartForeground bool

func init() {
	rootCmd.AddCommand(restartCmd)
	restartCmd.Flags().BoolVar(&restartForeground, "foreground", false, "Run in foreground instead of daemonizing")
}

// runRestart executes the restart command to stop and start the daemon.
// It checks if the daemon is currently running, performs a graceful shutdown,
// waits for completion, then starts the daemon again. Provides detailed
// status feedback throughout the process.
func runRestart(cmd *cobra.Command, args []string) error {
	fmt.Println("🔄 Restarting dotfile sync daemon...")

	// First, stop the daemon if it's running
	if isDaemonRunning() {
		fmt.Println("🛑 Stopping running daemon...")

		// Create a stop command instance
		stopArgs := []string{"--force"}
		stopCmd, _, err := rootCmd.Find([]string{"stop"})
		if err != nil {
			return fmt.Errorf("failed to find stop command: %w", err)
		}
		stopCmd.SetArgs(stopArgs)

		if err := stopCmd.Execute(); err != nil {
			// Don't fail if stop fails, just warn and continue
			fmt.Printf("⚠️  Warning: Failed to stop daemon gracefully: %v\n", err)
			fmt.Println("🔄 Continuing with restart...")
		} else {
			fmt.Println("✅ Daemon stopped successfully")
		}

		// Give the daemon a moment to clean up
		time.Sleep(time.Duration(daemonSleepTime) * time.Second)
	} else {
		fmt.Println("ℹ️  No running daemon found")
	}

	// Now start the daemon
	fmt.Println("🚀 Starting daemon...")

	// Create start command with appropriate flags
	startArgs := []string{}
	if restartForeground {
		startArgs = append(startArgs, "--foreground")
	}

	startCmd, _, err := rootCmd.Find([]string{"start"})
	if err != nil {
		return fmt.Errorf("failed to find start command: %w", err)
	}
	startCmd.SetArgs(startArgs)

	if err := startCmd.Execute(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	return nil
}