package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

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

	// Try to find the process using platform-appropriate method
	pid, err := getDaemonPID()
	if err != nil {
		return false
	}

	// Verify the process is actually running
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix systems, we can signal 0 to check if process exists
	// On Windows, this check is more limited
	return process != nil
}

// getDaemonPID finds the PID of the running daemon using platform-appropriate methods
func getDaemonPID() (int, error) {
	// Try to use PID file first (most reliable method)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		pidFile := filepath.Join(homeDir, ".dotfile-sync-manager.pid")
		if data, err := os.ReadFile(pidFile); err == nil {
			pidStr := strings.TrimSpace(string(data))
			if pid, err := strconv.Atoi(pidStr); err == nil {
				return pid, nil
			}
		}
	}

	// Fallback to platform-specific process detection
	return findProcessByName("dotfile-sync-manager")
}

// findProcessByName uses platform-appropriate methods to find a process by name
func findProcessByName(name string) (int, error) {
	switch runtime.GOOS {
	case "windows":
		return findProcessWindows(name)
	case "linux", "darwin", "freebsd", "openbsd":
		return findProcessUnix(name)
	default:
		return 0, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// findProcessUnix uses pgrep or ps to find processes on Unix-like systems
func findProcessUnix(name string) (int, error) {
	// Try pgrep first (more reliable)
	if cmd := exec.Command("pgrep", "-f", name); cmd.Run() == nil {
		if output, err := cmd.Output(); err == nil {
			pidStr := strings.TrimSpace(string(output))
			if strings.Contains(pidStr, "\n") {
				// Take first PID if multiple found
				pidStr = strings.Split(pidStr, "\n")[0]
			}
			if pid, err := strconv.Atoi(pidStr); err == nil {
				return pid, nil
			}
		}
	}

	// Fallback to ps command
	if cmd := exec.Command("ps", "aux"); cmd.Run() == nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, name) && !strings.Contains(line, "grep") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if pid, err := strconv.Atoi(fields[1]); err == nil {
							return pid, nil
						}
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("process not found: %s", name)
}

// findProcessWindows uses tasklist to find processes on Windows
func findProcessWindows(name string) (int, error) {
	// Use tasklist command to find processes
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+name+".exe", "/FO", "CSV")
	output, err := cmd.Output()
	if err != nil {
		// Try without .exe extension
		cmd = exec.Command("tasklist", "/FI", "IMAGENAME eq "+name, "/FO", "CSV")
		output, err = cmd.Output()
		if err != nil {
			return 0, fmt.Errorf("failed to run tasklist: %w", err)
		}
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("process not found: %s", name)
	}

	// Parse CSV output (skip header)
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		// CSV format: "imagename","pid","sessionname","session#","memusage"
		fields := strings.Split(line, "\",\"")
		if len(fields) >= 2 {
			pidStr := strings.Trim(fields[1], "\"")
			if pid, err := strconv.Atoi(pidStr); err == nil {
				return pid, nil
			}
		}
	}

	return 0, fmt.Errorf("process not found: %s", name)
}