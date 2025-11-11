package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

// openCmd represents the open command
var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open dotfiles repository directory",
	Long: `Open the dotfiles repository directory in the default file manager or application.
This makes it easy to navigate to your dotfiles for manual editing.

Examples:
  dsm open
  dsm open --editor  # Open in default editor instead of file manager`,
	RunE: runOpen,
}

var openEditor bool

func init() {
	rootCmd.AddCommand(openCmd)
	openCmd.Flags().BoolVarP(&openEditor, "editor", "e", false, "Open in default editor instead of file manager")
}

// runOpen executes the open command to launch the dotfiles repository.
// It can open the repository directory in the default file manager or
// open it in a text editor, with cross-platform support for different operating systems.
func runOpen(cmd *cobra.Command, args []string) error {
	// open command accepts at most one argument (optional path to open)
	if len(args) > 1 {
		return fmt.Errorf("open command accepts at most one argument (path to open)")
	}

	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	var repoPath string
	if len(args) == 1 {
		// Use the provided path
		repoPath = args[0]
	} else {
		repoPath = cfg.Git.RepoPath
	}

	// Check if repository exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("dotfiles repository not found at: %s", repoPath)
	}

	fmt.Printf("📂 Opening dotfiles repository: %s\n", repoPath)

	// Check if we're in a test environment and should avoid launching real file managers
	if os.Getenv("DSM_TEST_MODE") != "" || os.Getenv("CI") != "" {
		fmt.Printf("🧪 Test mode: Skipping actual directory opening\n")
		return nil
	}

	// Choose the appropriate open command based on platform and flag
	var openCmd string
	var openArgs []string

	switch {
	case openEditor:
		// Use centralized editor selection logic
		editor := getDefaultEditor()
		openCmd, openArgs = parseCommand(editor)

	default:
		// Use file manager
		switch runtime.GOOS {
		case "windows":
			openCmd = "explorer"
			openArgs = []string{repoPath}
		case "darwin":
			openCmd = "open"
			openArgs = []string{repoPath}
		default: // Linux and others
			if hasCommand("xdg-open") {
				openCmd = "xdg-open"
				openArgs = []string{repoPath}
			} else if hasCommand("gnome-open") {
				openCmd = "gnome-open"
				openArgs = []string{repoPath}
			} else {
				return fmt.Errorf("no suitable file manager found (tried xdg-open, gnome-open)")
			}
		}
	}

	// Execute the open command
	execCmd := exec.Command(openCmd, openArgs...)
	if output, err := execCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to open directory: %w\nOutput: %s", err, string(output))
	}

	return nil
}
