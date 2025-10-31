package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/process"
	syncservice "github.com/choru-k/dot-sync-manager/internal/sync"
	"github.com/spf13/cobra"
)

const (
	daemonStartupTimeout      = 5 * time.Second
	daemonStartupPollInterval = 100 * time.Millisecond
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

	// Prepare command arguments
	flagArgs := []string{}
	if configFile != "" {
		flagArgs = append(flagArgs, "--config", cfg.GetConfigPath())
	}
	if verbose {
		flagArgs = append(flagArgs, "--verbose")
	}

	if foreground {
		return runForegroundDaemon(cfg)
	}

	// Check if daemon is already running (only for non-foreground mode)
	// Foreground mode is spawned by the parent process, so we skip this check
	if isDaemonRunning() {
		return fmt.Errorf("dotfile sync daemon is already running")
	}

	// Run as daemon
	fmt.Println("Starting dotfile sync daemon in background...")

	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create command for daemon
	daemonArgs := append(flagArgs, "start", "--foreground")
	daemonCmd := exec.Command(execPath, daemonArgs...)

	if attrs := daemonProcAttr(); attrs != nil {
		daemonCmd.SysProcAttr = attrs
	}

	// Start daemon
	if err := daemonCmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for daemon to fully initialize by checking for PID file
	// This ensures the daemon successfully started before we report success
	if err := waitForDaemonStartup(daemonStartupTimeout); err != nil {
		return fmt.Errorf("daemon failed to start: %w", err)
	}

	// Get the actual daemon PID from the PID file (not the spawned process PID)
	pid, err := getDaemonPID()
	if err != nil {
		// Daemon started but we couldn't read PID - still report success
		fmt.Println("✅ Dotfile sync daemon started")
	} else {
		fmt.Printf("✅ Dotfile sync daemon started (PID: %d)\n", pid)
	}

	fmt.Printf("📁 Repository: %s\n", cfg.Git.RepoPath)
	fmt.Printf("📊 Machine: %s\n", cfg.Machine.Name)
	fmt.Printf("⚙️  Auto-sync: %v\n", cfg.Sync.AutoSyncEnabled)

	fmt.Println("\n💡 Use 'dsm stop' to stop the daemon")
	fmt.Println("💡 Use 'dsm status' to check daemon status")

	return nil
}

// waitForDaemonStartup polls for the daemon to fully initialize by checking for PID file
func waitForDaemonStartup(timeout time.Duration) error {
	deadline := timeNow().Add(timeout)
	ticker := time.NewTicker(daemonStartupPollInterval)
	defer ticker.Stop()

	for timeNow().Before(deadline) {
		// Check if daemon is running by checking PID file and process existence
		if isDaemonRunning() {
			return nil
		}
		<-ticker.C
	}

	return fmt.Errorf("daemon did not start within %v", timeout)
}

func runForegroundDaemon(cfg *config.SyncConfig) error {
	fmt.Println("Starting dotfile sync daemon in foreground...")
	fmt.Println("Press Ctrl+C to stop")

	// Create context with signal handling first so it can be used throughout
	signalCtx, cancel := signal.NotifyContext(context.Background(), daemonSignals()...)
	defer cancel()

	gmCfg := cfg.ToGitManagerConfig()

	// Use signal context for git manager so it can be cancelled on shutdown
	gitMgr, err := gitmanager.NewGitManager(signalCtx, gmCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize git manager: %w", err)
	}

	syncSvc, err := syncservice.New(gitMgr, cfg.ToSyncConfig())
	if err != nil {
		return fmt.Errorf("failed to create sync service: %w", err)
	}

	if err := syncSvc.Start(); err != nil {
		return fmt.Errorf("failed to start sync service: %w", err)
	}
	defer func() {
		if err := syncSvc.Stop(); err != nil {
			// Log error but don't fail shutdown
			log.Printf("sync: warning - failed to stop service: %v", err)
		}
	}()

	if err := process.WritePIDExclusive(os.Getpid()); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer func() {
		if err := process.RemovePID(); err != nil {
			fmt.Printf("⚠️  Warning: failed to remove PID file: %v\n", err)
		}
	}()

	fmt.Printf("🚀 Watching repository: %s\n", cfg.Git.RepoPath)
	fmt.Printf("📊 Machine: %s\n", cfg.Machine.Name)
	fmt.Printf("⚙️  Auto-sync enabled: %v\n", cfg.Sync.AutoSyncEnabled)

	<-signalCtx.Done()
	fmt.Println("\n🛑 Stopping sync service...")

	return nil
}
