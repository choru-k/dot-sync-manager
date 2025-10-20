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
	// log command should not accept any arguments
	if len(args) > 0 {
		return fmt.Errorf("log command accepts no arguments")
	}

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

func showLastLines(logFile string, lines int) error {
	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close log file: %v\n", closeErr)
		}
	}()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if stat.Size() == 0 {
		fmt.Println("(log file is empty)")
		return nil
	}

	// Seek to end and read backwards to find lines
	result, err := readLastLines(file, lines)
	if err != nil {
		return fmt.Errorf("failed to read last lines: %w", err)
	}

	if len(result.lines) == 0 {
		fmt.Println("(log file is empty)")
		return nil
	}

	fmt.Println(strings.Repeat("=", 50))
	for _, line := range result.lines {
		fmt.Println(line)
	}
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Showing last %d of %d total lines\n", len(result.lines), result.totalLines)

	return nil
}

// readLastLines efficiently reads the last N lines from a file by reading backwards
func readLastLines(file *os.File, n int) (*LogResult, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	size := stat.Size()
	if size == 0 {
		return &LogResult{lines: []string{}, totalLines: 0}, nil
	}

	// Start reading from the end
	const chunkSize = 4096
	var lines []string
	var buf []byte
	var pos = size
	var lineCount int

	for pos > 0 && lineCount < n {
		// Calculate chunk start position
		chunkStart := pos - chunkSize
		if chunkStart < 0 {
			chunkStart = 0
		}

		// Read chunk
		chunk := make([]byte, pos-chunkStart)
		_, err := file.ReadAt(chunk, chunkStart)
		if err != nil {
			return nil, err
		}

		// Prepend to buffer
		buf = append(chunk, buf...)
		pos = chunkStart

		// Count newlines in combined buffer
		newlines := 0
		for _, b := range buf {
			if b == '\n' {
				newlines++
			}
		}

		// If we have enough lines, extract them
		if newlines >= n {
			break
		}
	}

	// Extract the last N lines from buffer
	if len(buf) == 0 {
		return &LogResult{lines: []string{}, totalLines: 0}, nil
	}

	// Split buffer into lines
	allLines := strings.Split(string(buf), "\n")

	// Remove empty string at the end if file ends with newline
	if len(allLines) > 0 && allLines[len(allLines)-1] == "" {
		allLines = allLines[:len(allLines)-1]
	}

	// Get the last N lines
	start := 0
	if len(allLines) > n {
		start = len(allLines) - n
	}

	lines = allLines[start:]
	totalLines := len(allLines)

	return &LogResult{lines: lines, totalLines: totalLines}, nil
}

// LogResult holds the result of reading log lines
type LogResult struct {
	lines      []string
	totalLines int
}

func followLog(logFile string) error {
	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close log file: %v\n", closeErr)
		}
	}()

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