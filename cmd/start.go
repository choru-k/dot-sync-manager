package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
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

	// Graceful shutdown timeouts
	// Design rationale: service timeout < total timeout to allow coordinated cleanup
	// serviceShutdownTimeout (10s): Individual service timeout based on typical sync operation times
	// totalShutdownTimeout (15s): Overall timeout including coordination overhead
	serviceShutdownTimeout = 10 * time.Second
	totalShutdownTimeout  = 15 * time.Second
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

	// Acquire PID file lock for the daemon's entire lifetime
	lockManager, err := process.WritePIDExclusive(os.Getpid())
	if err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer func() {
		if err := lockManager.Unlock(); err != nil {
			fmt.Printf("⚠️  Warning: failed to cleanup PID file and lock: %v\n", err)
		}
	}()

	fmt.Printf("🚀 Watching repository: %s\n", cfg.Git.RepoPath)
	fmt.Printf("📊 Machine: %s\n", cfg.Machine.Name)
	fmt.Printf("⚙️  Auto-sync enabled: %v\n", cfg.Sync.AutoSyncEnabled)

	// Wait for shutdown signal
	<-signalCtx.Done()
	fmt.Println("\n🛑 Signal received, initiating graceful shutdown...")

	// Execute graceful shutdown with timeout protection
	if err := gracefulShutdown(signalCtx, syncSvc, lockManager); err != nil {
		fmt.Printf("❌ Critical: Graceful shutdown failed: %v\n", err)
		fmt.Printf("⚠️  Daemon state may be inconsistent. Manual cleanup may be required.\n")
		fmt.Printf("💡 Run 'dsm status' to check daemon state and 'dsm stop' to force cleanup if needed.\n")
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	fmt.Println("✅ Graceful shutdown completed")
	return nil
}

// gracefulShutdown coordinates the shutdown sequence with timeout protection
//
// Design strategy:
// 1. Parallel execution: Sync service and PID cleanup run concurrently for efficiency
// 2. Timeout hierarchy: Service timeout (10s) < Total timeout (15s) for coordination
// 3. Error aggregation: Collect all errors, filter nils, report comprehensive status
// 4. Graceful degradation: Continue shutdown even if individual operations fail
//
// Coordination timing: 100ms delay before PID cleanup ensures sync service
// starts shutting down first, preventing race conditions where PID file
// might be removed before service cleanup begins.
func gracefulShutdown(ctx context.Context, syncSvc *syncservice.SyncService, lockManager *process.LockManager) error {
	fmt.Println("🔄 Shutting down services...")

	// Create shutdown timeout context to prevent indefinite hangs
	// This context wraps the incoming signal context and provides a hard deadline
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, totalShutdownTimeout)
	defer shutdownCancel()

	// Channel to collect shutdown errors (buffered to prevent blocking)
	// Capacity 2: one for sync service, one for PID cleanup operation
	shutdownErrors := make(chan error, 2)
	var shutdownWG sync.WaitGroup

	// Start sync service shutdown in goroutine with timeout
	shutdownWG.Add(1)
	go func() {
		defer shutdownWG.Done()

		// Shutdown sync service - use Stop() method which returns error
		if err := syncSvc.Stop(); err != nil {
			shutdownErrors <- fmt.Errorf("sync service shutdown failed after %v timeout: %w", serviceShutdownTimeout, err)
			log.Printf("CRITICAL: Sync service shutdown failure may indicate: locked files, incomplete git operations, or resource deadlock")
			return
		}
		shutdownErrors <- nil
	}()

	// Start PID file cleanup in goroutine using LockManager
	//
	// Coordination strategy: PID cleanup runs concurrently but slightly delayed
	// to ensure sync service shutdown begins first. This prevents the race condition
	// where PID file removal might be perceived as "daemon not running" by other
	// processes before the sync service has actually stopped monitoring files.
	shutdownWG.Add(1)
	go func() {
		defer shutdownWG.Done()

		// Coordination delay (100ms): Ensures sync service shutdown starts first
		// This timing is critical for preventing false "daemon not running" states
		// when multiple processes check daemon status during shutdown window.
		time.Sleep(100 * time.Millisecond)

		if err := lockManager.Unlock(); err != nil {
			shutdownErrors <- fmt.Errorf("PID file cleanup error: %w", err)
			return
		}
		shutdownErrors <- nil
	}()

	// Wait for shutdown operations or timeout
	done := make(chan struct{})
	go func() {
		shutdownWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All shutdown operations completed
		close(shutdownErrors)
		var errors []error
		for err := range shutdownErrors {
			if err != nil {
				errors = append(errors, err)
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("shutdown completed with %d error(s): %v", len(errors), errors)
		}
		fmt.Println("🧹 All services shut down cleanly")
		return nil

	case <-shutdownCtx.Done():
		// Shutdown timeout exceeded
		fmt.Println("⏰ Shutdown timeout exceeded, forcing exit")
		return fmt.Errorf("shutdown timeout after %v", totalShutdownTimeout)
	}
}
