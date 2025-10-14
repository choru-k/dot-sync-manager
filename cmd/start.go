package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start dotfile sync daemon",
	Long: `Start the dotfile sync daemon in the background. The daemon will watch for
file changes and automatically sync them to the Git repository.

Examples:
  dsm start
  dsm start --foreground  # Run in foreground (for debugging)`,
	RunE: runStart,
}

var foreground bool

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolVar(&foreground, "foreground", false, "Run in foreground instead of daemonizing")
}

func runStart(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Check if daemon is already running
	if isDaemonRunning() {
		return fmt.Errorf("dotfile sync daemon is already running")
	}

	// Prepare command arguments
	cmdArgs := []string{"-config", cfg.GetConfigPath()}
	if verbose {
		cmdArgs = append(cmdArgs, "-verbose")
	}

	if foreground {
		// Run in foreground
		fmt.Println("Starting dotfile sync daemon in foreground...")
		fmt.Println("Press Ctrl+C to stop")

		// This would normally start the sync service directly
		// For now, we'll simulate it
		fmt.Printf("🚀 Starting sync service for repository: %s\n", cfg.Git.RepoPath)
		fmt.Printf("📊 Machine: %s\n", cfg.Machine.Name)
		fmt.Printf("⚙️  Auto-sync: %v\n", cfg.Sync.AutoSyncEnabled)
		fmt.Printf("⏱️  Pull interval: %d seconds\n", cfg.Sync.PullIntervalSeconds)
		fmt.Printf("⏱️  Debounce: %d seconds\n", cfg.Sync.DebounceSeconds)

		// In a real implementation, this would start the actual sync service
		fmt.Println("\n🔄 Monitoring for file changes...")
		fmt.Println("(This is a simulation - actual daemon would run indefinitely)")

		return nil
	}

	// Run as daemon
	fmt.Println("Starting dotfile sync daemon in background...")

	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create command for daemon
	daemonCmd := exec.Command(execPath, cmdArgs...)

	// Set up process attributes for daemon
	daemonCmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session
	}

	// Start daemon
	if err := daemonCmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	fmt.Printf("✅ Dotfile sync daemon started (PID: %d)\n", daemonCmd.Process.Pid)
	fmt.Printf("📁 Repository: %s\n", cfg.Git.RepoPath)
	fmt.Printf("📊 Machine: %s\n", cfg.Machine.Name)
	fmt.Printf("⚙️  Auto-sync: %v\n", cfg.Sync.AutoSyncEnabled)

	fmt.Println("\n💡 Use 'dsm stop' to stop the daemon")
	fmt.Println("💡 Use 'dsm status' to check daemon status")

	return nil
}

