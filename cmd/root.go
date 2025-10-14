package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/process"
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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Set up logging based on verbose flag
		if verbose {
			// Enable verbose logging
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Path to configuration file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
}

// getConfig loads configuration using proper discovery
func getConfig() (*config.SyncConfig, error) {
	var cfg *config.SyncConfig
	var err error

	if configFile != "" {
		// Explicit config file path provided
		cfg, err = config.LoadFromFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load configuration from %s: %w\nHint: Check that the file exists and is valid JSON", configFile, err)
		}
	} else {
		// Use default location discovery (PRD-compliant)
		cfg, err = config.LoadFromDefaultLocation()
		if err != nil {
			homeDir, _ := os.UserHomeDir()
			prdPath := filepath.Join(homeDir, "dotfiles", ".sync-config.json")
			legacyPath := filepath.Join(homeDir, ".dotfile-sync.json")
			return nil, fmt.Errorf("failed to load configuration: %w\n\nExpected config at:\n  - %s (PRD location)\n  - %s (legacy location)\n\nRun 'dsm init' to create a new configuration", err, prdPath, legacyPath)
		}
	}

	return cfg, nil
}

// expandPath expands ~ to user home directory using shared utility
func expandPath(path string) string {
	return util.ExpandPath(path)
}

// isDaemonRunning checks if the daemon is already running
// Uses the process package which properly detects the actual binary name
func isDaemonRunning() bool {
	return process.IsDaemonRunning()
}

// getDaemonPID finds the PID of the running daemon using platform-appropriate methods
// Uses the process package which properly handles PID file and process name detection
func getDaemonPID() (int, error) {
	return process.GetDaemonPID()
}
