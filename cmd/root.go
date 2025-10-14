package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/choru-k/dot-sync-manager/internal/config"
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
			return nil, fmt.Errorf("failed to load configuration from %s: %w", configFile, err)
		}
	} else {
		// Use default location discovery (PRD-compliant)
		cfg, err = config.LoadFromDefaultLocation()
		if err != nil {
			return nil, fmt.Errorf("failed to load configuration: %w", err)
		}
	}

	return cfg, nil
}

// expandPath expands ~ to user home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		if homeDir, err := os.UserHomeDir(); err == nil {
			if len(path) == 1 {
				return homeDir
			}
			return filepath.Join(homeDir, path[1:])
		}
	}
	return path
}

// isDaemonRunning checks if the daemon is already running
func isDaemonRunning() bool {
	// This is a simplified check. In a real implementation, you would:
	// 1. Check for a PID file
	// 2. Verify the process is still running
	// 3. Check that it's the correct process

	// For now, we'll use a simple approach by checking if we can find the process
	cmd := exec.Command("pgrep", "-f", "dotfile-sync-manager")
	err := cmd.Run()
	return err == nil
}

// getDaemonPID finds the PID of the running daemon
func getDaemonPID() (int, error) {
	// Use pgrep to find the dotfile-sync-manager process
	cmd := exec.Command("pgrep", "-f", "dotfile-sync-manager")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to find daemon process: %w", err)
	}

	pidStr := string(output)
	pidStr = pidStr[:len(pidStr)-1] // Remove trailing newline

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse PID: %w", err)
	}

	return pid, nil
}