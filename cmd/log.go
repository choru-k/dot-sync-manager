package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/util"
	"github.com/spf13/cobra"
)

// logCmd represents the log command
var logCmd = &cobra.Command{
	Use:   "log [flags]",
	Short: "View sync log file",
	Long: `View the dotfile sync log file. This shows recent daemon activity,
sync operations, and any errors that occurred.

Examples:
  dsm log
  dsm log -n 20    # Show last 20 lines
  dsm log -f       # Follow log in real-time`,
	RunE: runLog,
}

var (
	logLines    int
	logFollow   bool
	logFile    string
)

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().IntVarP(&logLines, "lines", "n", defaultLogLines, "Number of lines to show from end of log")
	logCmd.Flags().BoolVarP(&logFollow, "follow", "f", false, "Follow log output (like tail -f)")
}

// runLog executes the log command to view sync daemon activity.
// It determines the log file location, handles user path expansion,
// and provides options for showing recent lines or following the log in real-time.
func runLog(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Determine log file location
	if logFile == "" {
		logFile = cfg.Advanced.LogFile
	}
	if logFile == "" {
		// Default log file location
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to determine home directory: %w", err)
		}
		logFile = filepath.Join(homeDir, ".dotfile-sync.log")
	}

	// Expand user path in log file
	if strings.HasPrefix(logFile, "~") {
		expandedPath, err := util.ExpandPath(logFile)
		if err != nil {
			return fmt.Errorf("failed to expand log file path: %w", err)
		}
		logFile = expandedPath
	}

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Printf("📝 Log file not found: %s\n", logFile)
		fmt.Println("💡 Log file will be created when the daemon runs")
		return nil
	}

	fmt.Printf("📝 Viewing log file: %s\n", logFile)

	if logFollow {
		// Follow mode (like tail -f)
		return followLog(logFile)
	}

	// Show last N lines
	return showLastLines(logFile, logLines)
}

func showLastLines(logFile string, lines int) (err error) {
	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer util.CloseAndCaptureErr(file, &err)

	// Read all lines
	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	if len(allLines) == 0 {
		fmt.Println("(log file is empty)")
		return nil
	}

	// Show last N lines
	start := 0
	if len(allLines) > lines {
		start = len(allLines) - lines
	}

	fmt.Println(strings.Repeat("=", 50))
	for i := start; i < len(allLines); i++ {
		fmt.Println(allLines[i])
	}
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Showing last %d of %d total lines\n", len(allLines)-start, len(allLines))

	return nil
}

func followLog(logFile string) (err error) {
	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer util.CloseAndCaptureErr(file, &err)

	// Seek to end of file
	if _, err := file.Seek(0, 2); err != nil {
		return fmt.Errorf("failed to seek to end of log file: %w", err)
	}

	fmt.Println("👁️  Following log output (Ctrl+C to stop)...")
	fmt.Println(strings.Repeat("=", 50))

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	return scanner.Err()
}