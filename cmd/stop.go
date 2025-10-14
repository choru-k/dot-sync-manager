package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

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
		return fmt.Errorf("dotfile sync daemon is not running")
	}

	fmt.Println("Stopping dotfile sync daemon...")

	// Get PID of the daemon process
	pid, err := getDaemonPID()
	if err != nil {
		if stopForce {
			fmt.Printf("⚠️  Could not determine daemon PID: %v\n", err)
			fmt.Println("Attempting to stop all dotfile-sync-manager processes...")
			return stopAllDaemons()
		}
		return fmt.Errorf("failed to get daemon PID: %w", err)
	}

	// Find the process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Send termination signal using platform-appropriate method
	if err := terminateProcess(process); err != nil {
		if stopForce {
			fmt.Printf("⚠️  Failed to send graceful shutdown signal: %v\n", err)
			fmt.Println("Attempting force shutdown...")
			return process.Kill()
		}
		return fmt.Errorf("failed to send shutdown signal: %w", err)
	}

	fmt.Printf("✅ Sent graceful shutdown signal to daemon (PID: %d)\n", pid)
	fmt.Println("💡 The daemon will finish any pending operations before stopping")

	return nil
}


// terminateProcess sends appropriate termination signal based on platform
func terminateProcess(process *os.Process) error {
	switch runtime.GOOS {
	case "windows":
		// On Windows, we can only use process.Kill() as there's no graceful shutdown signal
		return process.Kill()
	default:
		// On Unix-like systems, we can use SIGTERM for graceful shutdown
		return process.Signal(os.Interrupt) // Use SIGINT (Ctrl+C) instead of SIGTERM
	}
}

// stopAllDaemons attempts to stop all dotfile-sync-manager processes using platform-appropriate method
func stopAllDaemons() error {
	switch runtime.GOOS {
	case "windows":
		// Use taskkill to stop all dotfile-sync-manager processes
		cmd := exec.Command("taskkill", "/F", "/IM", "dotfile-sync-manager.exe")
		if err := cmd.Run(); err != nil {
			// Try without .exe extension
			cmd = exec.Command("taskkill", "/F", "/IM", "dotfile-sync-manager")
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to stop daemon processes: %w", err)
			}
		}
	default:
		// Use pkill to stop all dotfile-sync-manager processes
		cmd := exec.Command("pkill", "-f", "dotfile-sync-manager")
		if err := cmd.Run(); err != nil {
			// Fallback to kill command
			cmd = exec.Command("killall", "dotfile-sync-manager")
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to stop daemon processes: %w", err)
			}
		}
	}

	fmt.Println("✅ Sent stop signal to all dotfile-sync-manager processes")
	return nil
}