package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// ignoreCmd represents the ignore command
var ignoreCmd = &cobra.Command{
	Use:   "ignore",
	Short: "Edit .syncignore file",
	Long: `Edit the .syncignore file to control which files and directories
should be excluded from automatic syncing. This uses the same syntax
as gitignore files.

Patterns:
  *.log              Ignore all .log files
  temp/              Ignore the temp directory
  !important.log     Don't ignore this specific file
  **/cache           Ignore cache directories at any level
  build/             Ignore build directories
  .env               Ignore .env files

Examples:
  dsm ignore          Edit .syncignore in default editor
  dsm ignore --show   Show current .syncignore contents`,
	RunE: runIgnore,
}

var ignoreShow bool

func init() {
	rootCmd.AddCommand(ignoreCmd)
	ignoreCmd.Flags().BoolVar(&ignoreShow, "show", false, "Show current .syncignore contents instead of editing")
}

// runIgnore executes the ignore command to manage .syncignore files.
// It creates a new .syncignore file with default patterns if it doesn't exist,
// or opens an existing file in the user's preferred editor for manual editing.
// The --show flag displays current contents without editing.
func runIgnore(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Determine .syncignore file location
	repoPath := cfg.Git.RepoPath
	ignoreFile := filepath.Join(repoPath, ".syncignore")

	// Show current contents if requested
	if ignoreShow {
		return showIgnoreFile(ignoreFile)
	}

	// Create .syncignore file if it doesn't exist
	if _, err := os.Stat(ignoreFile); os.IsNotExist(err) {
		// Create with helpful comments and default patterns
		if err := os.WriteFile(ignoreFile, []byte(defaultIgnoreContent), defaultFilePerms); err != nil {
			return fmt.Errorf("failed to create .syncignore file: %w", err)
		}
		fmt.Printf("📝 Created new .syncignore file: %s\n", ignoreFile)
		fmt.Printf("💡 Added default exclusion patterns\n")
	}

	// Open .syncignore file in default editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Fallback editors by platform
		switch {
		case strings.Contains(ignoreFile, ".txt"):
			editor = "code" // Try VS Code
		default:
			switch runtime.GOOS {
			case "windows":
				editor = "notepad"
			case "darwin":
				editor = "open -a TextEdit"
			default: // Linux
				editor = "nano"
			}
		}
	}

	fmt.Printf("📝 Opening .syncignore file: %s\n", ignoreFile)
	fmt.Printf("Using editor: %s\n", editor)

	execCmd := exec.Command(editor, ignoreFile)
	if output, err := execCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to open editor: %w\nOutput: %s", err, string(output))
	}

	fmt.Println("✅ .syncignore file updated")
	fmt.Println("💡 Changes will take effect on the next sync cycle")

	return nil
}

func showIgnoreFile(ignoreFile string) error {
	fmt.Printf("📝 .syncignore file: %s\n", ignoreFile)

	// Check if file exists
	if _, err := os.Stat(ignoreFile); os.IsNotExist(err) {
		fmt.Println("(file does not exist)")
		fmt.Println()
		fmt.Println("💡 Run 'dsm ignore' to create and edit .syncignore file")
		return nil
	}

	// Read and display contents
	content, err := os.ReadFile(ignoreFile)
	if err != nil {
		return fmt.Errorf("failed to read .syncignore file: %w", err)
	}

	if len(content) == 0 {
		fmt.Println("(file is empty)")
	} else {
		fmt.Println(strings.Repeat("=", 50))
		fmt.Print(string(content))
		if !strings.HasSuffix(string(content), "\n") {
			fmt.Println()
		}
		fmt.Println(strings.Repeat("=", 50))
	}

	return nil
}