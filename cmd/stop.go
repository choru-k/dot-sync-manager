package cmd

import (
	"fmt"
	"os"

	"github.com/choru-k/dot-sync-manager/internal/process"
	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop dotfile sync daemon",
	Long: `Stop the running dotfile sync daemon.

Examples:
  dsm stop
  dsm stop --force  # Force stop even if daemon appears unresponsive`,
	RunE: runStop,
}

var stopForce bool

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().BoolVar(&stopForce, "force", false, "Force stop the daemon")
}

func runStop(cmd *cobra.Command, args []string) error {
	// Check if daemon is running
	if !isDaemonRunning() {
		return fmt.Errorf(`dotfile sync daemon is not running

Hint: Use 'dsm start' to start the daemon`)
	}

	fmt.Println("Stopping dotfile sync daemon...")

	// Get PID of the daemon process
	pid, err := getDaemonPID()
	if err != nil {
		if stopForce {
			fmt.Printf("⚠️  Could not determine daemon PID: %v\n", err)
			fmt.Println("Attempting to stop all dotfile-sync-manager processes...")
			if err := process.StopAllDaemons(); err != nil {
				return err
			}
			fmt.Println("✅ Sent stop signal to all dotfile-sync-manager processes")
			return nil
		}
		return fmt.Errorf("failed to get daemon PID: %w", err)
	}

	// Find the process
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Send termination signal using platform-appropriate method
	if err := process.TerminateProcess(proc); err != nil {
		if stopForce {
			fmt.Printf("⚠️  Failed to send graceful shutdown signal: %v\n", err)
			fmt.Println("Attempting force shutdown...")
			return proc.Kill()
		}
		return fmt.Errorf("failed to send shutdown signal: %w", err)
	}

	fmt.Printf("✅ Sent graceful shutdown signal to daemon (PID: %d)\n", pid)
	fmt.Println("💡 The daemon will finish any pending operations before stopping")
	fmt.Println("💡 The daemon will clean up its own PID file upon shutdown")

	return nil
}
