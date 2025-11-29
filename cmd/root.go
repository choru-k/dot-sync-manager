package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/process"
	syncservice "github.com/choru-k/dot-sync-manager/internal/sync"
	"github.com/choru-k/dot-sync-manager/internal/util"
	"github.com/spf13/cobra"
)

var (
	configFile   string
	verbose      bool
	noEmoji      bool
	globalDryRun bool

	configFileMu sync.RWMutex
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "dsm",
	Short: "Dotfile Sync Manager",
	Long: `Dotfile Sync Manager (DSM) automatically syncs dotfiles between machines
using Git. Manage your dotfiles with simple commands.

Use "dsm help <command>" for more information about a command.`,
	RunE: runRoot,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Path to configuration file")
	// TODO(future): Implement verbose logging throughout commands when verbose flag is set
	// This would involve adding detailed logging statements in all command functions
	// and using a proper logging framework (like logrus or zap) with configurable levels
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().BoolVar(&noEmoji, "no-emoji", false, "Disable emoji output")
	rootCmd.PersistentFlags().BoolVar(&globalDryRun, "dry-run", false, "Show what would be done without executing")
}

// getConfig loads configuration using proper discovery logic.
// If --config flag is provided, it loads from that path (with tilde expansion).
// Otherwise, it searches default locations (~/dotfiles/{ConfigFileName}, ~/.dotfile-sync.json).
// Returns error if explicit config file doesn't exist or has invalid JSON.
func getConfig() (*config.SyncConfig, error) {
	var cfg *config.SyncConfig
	var err error

	if cfgPath := getConfigFile(); cfgPath != "" {
		// Expand path for tilde and other user path shortcuts
		expandedPath, err := util.ExpandPath(cfgPath)
		if err != nil {
			return nil, fmt.Errorf("failed to expand config path %s: %w", cfgPath, err)
		}
		setConfigFile(expandedPath)

		// When an explicit config file is given, it must exist
		if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("configuration file not found at %s", expandedPath)
		}

		cfg, err = config.LoadFromFile(expandedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load configuration from %s: %w\nHint: Check that the file is valid JSON", expandedPath, err)
		}
	} else {
		cfg, err = config.LoadFromDefaultLocation()
		if err != nil {
			return nil, formatConfigNotFoundError(err)
		}
	}

	return cfg, nil
}

// formatConfigNotFoundError formats a helpful error message when config is not found.
// It attempts to show expected config locations, falling back to a generic message
// if the home directory cannot be determined.
func formatConfigNotFoundError(err error) error {
	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		// Can't show expected paths if we can't get home directory
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	prdPath := filepath.Join(homeDir, "dotfiles", ConfigFileName)
	legacyPath := filepath.Join(homeDir, LegacyConfigFileName)

	return fmt.Errorf(`failed to load configuration: %w

Expected config at:
  - %s (PRD location)
  - %s (legacy location)

Run 'dsm init' to create a new configuration`, err, prdPath, legacyPath)
}

func isDaemonRunning() bool {
	return process.IsDaemonRunning()
}

func getConfigFile() string {
	configFileMu.RLock()
	defer configFileMu.RUnlock()
	return configFile
}

func setConfigFile(path string) {
	configFileMu.Lock()
	defer configFileMu.Unlock()
	configFile = path
}

// getDaemonPID finds the PID of the running daemon.
func getDaemonPID() (int, error) {
	return process.GetDaemonPID()
}

func runRoot(cmd *cobra.Command, args []string) error {
	// Prevent accidental invocation with unexpected positional arguments.
	if len(args) > 0 {
		return cmd.Help()
	}

	cfg, err := getConfig()
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	gitCfg := cfg.ToGitManagerConfig()
	// gitCfg is a struct, not a pointer, so we validate its fields instead of checking for nil
	if gitCfg.RepoPath == "" {
		return fmt.Errorf("failed to create git manager configuration: invalid repository path")
	}
	gitMgr, err := gitmanager.NewGitManager(ctx, gitCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize git manager: %w", err)
	}

	// Create sync service with version and config path for status reporting
	syncCfg := cfg.ToSyncConfig(Version, cfg.GetConfigPath())
	service, err := syncservice.New(gitMgr, syncCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize sync service: %w", err)
	}

	if err := service.Start(); err != nil {
		return fmt.Errorf("failed to start sync service: %w", err)
	}

	// Acquire PID file lock for the daemon's entire lifetime
	lockManager, err := process.WritePIDExclusive(os.Getpid())
	if err != nil {
		if service != nil {
			if stopErr := service.Stop(); stopErr != nil {
				return fmt.Errorf("failed to write PID file: %w; also failed to stop service: %w", err, stopErr)
			}
		}
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	signalCh := make(chan os.Signal, 1)
	signals := []os.Signal{os.Interrupt}
	if runtime.GOOS != "windows" {
		signals = append(signals, syscall.SIGTERM)
	}
	signal.Notify(signalCh, signals...)

	defer func() {
		signal.Stop(signalCh)
		if service != nil {
			if err := service.Stop(); err != nil {
				log.Printf("sync: warning - failed to stop service gracefully: %v", err)
			}
		}
		if err := lockManager.Unlock(); err != nil {
			log.Printf("process: warning - failed to cleanup PID file and lock: %v", err)
		}
	}()

	fmt.Println("Dotfile Sync Manager daemon started. Press Ctrl+C to stop.")

	select {
	case <-ctx.Done():
		fmt.Println("Context cancelled; stopping daemon...")
	case sig := <-signalCh:
		fmt.Printf("Received %s; stopping daemon...\n", sig)
	}

	return nil
}
