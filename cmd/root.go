package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/util"
	"github.com/spf13/cobra"
)

var (
	configFile string
	verbose    bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "dsm",
	Short: "Dotfile Sync Manager",
	Long: `Dotfile Sync Manager (DSM) automatically syncs dotfiles between machines
using Git. Manage your dotfiles with simple commands.

Use "dsm help <command>" for more information about a command.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Path to configuration file")
	// TODO(future): Implement verbose logging throughout commands when verbose flag is set
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
}

// getConfig loads configuration using proper discovery logic.
// If --config flag is provided, it loads from that path (with tilde expansion).
// Otherwise, it searches default locations (~/dotfiles/{ConfigFileName}, ~/.dotfile-sync.json).
// Returns error if explicit config file doesn't exist or has invalid JSON.
func getConfig() (*config.SyncConfig, error) {
	var cfg *config.SyncConfig
	var err error

	if configFile != "" {
		// Expand path for tilde and other user path shortcuts
		expandedPath, err := util.ExpandPath(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to expand config path %s: %w", configFile, err)
		}
		configFile = expandedPath

		// When an explicit config file is given, it must exist
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("configuration file not found at %s", configFile)
		}

		cfg, err = config.LoadFromFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load configuration from %s: %w\nHint: Check that the file is valid JSON", configFile, err)
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
	legacyPath := filepath.Join(homeDir, ".dotfile-sync.json")

	return fmt.Errorf(`failed to load configuration: %w

Expected config at:
  - %s (PRD location)
  - %s (legacy location)

Run 'dsm init' to create a new configuration`, err, prdPath, legacyPath)
}

// isDaemonRunning checks if the daemon is already running.
// TODO(PR3): Implement actual daemon detection via PID file or process lookup.
func isDaemonRunning() bool {
	return false
}

// getDaemonPID finds the PID of the running daemon.
// TODO(PR3): Implement by reading PID file from ~/.dotfile-sync.pid.
func getDaemonPID() (int, error) {
	return 0, fmt.Errorf("daemon not running")
}
