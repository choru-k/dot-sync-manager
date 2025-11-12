package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/process"
	"github.com/spf13/cobra"
)

const (
	daemonStartupTimeout      = 15 * time.Second
	daemonStartupPollInterval = 100 * time.Millisecond

	// Graceful shutdown timeouts
	// Design rationale: service timeout < total timeout to allow coordinated cleanup
	// serviceShutdownTimeout (10s): Individual service timeout based on typical sync operation times
	// totalShutdownTimeout (15s): Overall timeout including coordination overhead
	serviceShutdownTimeout = 10 * time.Second
	totalShutdownTimeout   = 15 * time.Second
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
var startLogFile string

var daemonExeName string

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolVar(&foreground, "foreground", false, "Run in foreground instead of daemonizing")
	startCmd.Flags().StringVar(&startLogFile, "log-file", "", "Redirect daemon output to a log file")
	if err := startCmd.Flags().MarkHidden("log-file"); err != nil {
		// Log warning but don't fail startup - log-file flag is optional
		log.Printf("warning: failed to hide log-file flag: %v", err)
	}
	startCmd.Flags().StringVar(&daemonExeName, "daemon-exe-name", "", "Explicitly set the daemon executable name (for testing)")
	if err := startCmd.Flags().MarkHidden("daemon-exe-name"); err != nil {
		log.Printf("warning: failed to hide daemon-exe-name flag: %v", err)
	}
}

func runStart(cmd *cobra.Command, args []string) error {
	if daemonExeName != "" {
		process.SetExplicitProcessName(daemonExeName)
	}

	// Load configuration
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Prepare command arguments
	flagArgs := []string{}
	if getConfigFile() != "" {
		flagArgs = append(flagArgs, "--config", cfg.GetConfigPath())
	}
	if verbose {
		flagArgs = append(flagArgs, "--verbose")
	}
	if startLogFile != "" {
		flagArgs = append(flagArgs, "--log-file", startLogFile)
	}

	if foreground {
		if startLogFile != "" {
			f, err := os.OpenFile(startLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				return fmt.Errorf("failed to open log file: %w", err)
			}
			log.SetOutput(f)
		}
		return runForegroundDaemon(cfg, startLogFile)
	}

	startLock, err := process.AcquireCommandLock("start")
	if err != nil {
		if errors.Is(err, process.ErrCommandLockHeld) {
			return fmt.Errorf("dotfile sync daemon start already in progress")
		}
		return fmt.Errorf("failed to lock start command: %w", err)
	}
	defer func() {
		if releaseErr := startLock.Release(); releaseErr != nil {
			log.Printf("warning: failed to release start lock: %v", releaseErr)
		}
	}()

	// Check if daemon is already running (only for non-foreground mode)
	// Foreground mode is spawned by the parent process, so we skip this check
	if process.IsDaemonRunningPIDOnly() {
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
		return fmt.Errorf("failed to start daemon: %w\nHint: Check if another dsm daemon is already running with 'dsm status'\nEnsure the dotfiles repository exists and is accessible", err)
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
		// Check if daemon is running by verifying the PID file state
		if process.IsDaemonRunningPIDOnly() {
			return nil
		}
		<-ticker.C
	}

	return fmt.Errorf("daemon did not start within %v", timeout)
}

func runForegroundDaemon(cfg *config.SyncConfig, logFile string) error {
	fmt.Println("Starting dotfile sync daemon in foreground...")
	fmt.Println("Press Ctrl+C to stop")

	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		log.SetOutput(f)
	}

	// Create context with signal handling first so it can be used throughout
	signalCtx, cancel := signal.NotifyContext(context.Background(), daemonSignals()...)
	defer cancel()

	// Acquire PID file lock for the daemon's entire lifetime
	lockManager, err := process.WritePIDExclusive(os.Getpid())
	if err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	// PID lock is released by the gracefulShutdown function.

	fmt.Printf("🚀 Watching repository: %s\n", cfg.Git.RepoPath)
	fmt.Printf("📊 Machine: %s\n", cfg.Machine.Name)
	fmt.Printf("⚙️  Auto-sync enabled: %v\n", cfg.Sync.AutoSyncEnabled)

	// Wait for shutdown signal
	<-signalCtx.Done()
	fmt.Println("\n🛑 Signal received, initiating graceful shutdown...")

	// Execute graceful shutdown with timeout protection
	if err := gracefulShutdown(signalCtx, lockManager); err != nil {
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
func gracefulShutdown(_ context.Context, lockManager *process.LockManager) error {
	fmt.Println("🔄 Shutting down services...")

	// Create shutdown timeout context to prevent indefinite hangs
	// This context wraps the incoming signal context and provides a hard deadline
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), totalShutdownTimeout)
	defer shutdownCancel()

	// Channel to collect shutdown errors (buffered to prevent blocking)
	// Capacity 2: one for sync service, one for PID cleanup operation
	shutdownErrors := make(chan error, 2)
	var shutdownWG sync.WaitGroup

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
