package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/util"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tracked dotfiles",
	Long: `List all files and directories currently tracked by the dotfile sync manager.

Examples:
  dsm list
  dsm list --mappings  # Show source-to-target mappings`,
	RunE: runList,
}

var showMappings bool

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVar(&showMappings, "mappings", false, "Show source-to-target mappings")
}

func runList(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("📂 Tracked Dotfiles")
	fmt.Println("=" + strings.Repeat("=", 30))

	// List tracked files from cfg.Mappings (authoritative source)
	if len(cfg.Mappings) == 0 {
		fmt.Println("No files tracked yet.")
		fmt.Println("\n💡 Use 'dsm add <filepath>' to add files to tracking")
		return nil
	}

	if showMappings {
		fmt.Printf("File Mappings (%d):\n\n", len(cfg.Mappings))
		// Sort source keys for deterministic output
		sources := make([]string, 0, len(cfg.Mappings))
		for source := range cfg.Mappings {
			sources = append(sources, source)
		}
		sort.Strings(sources)

		for _, source := range sources {
			target := cfg.Mappings[source]
			sourcePath := filepath.Join(cfg.Git.RepoPath, source)
			status, statusIcon := checkSymlinkStatus(sourcePath, target)

			fmt.Printf("📄 %s\n", source)
			fmt.Printf("   🔗 %s %s\n", target, statusIcon)
			if status != "linked" {
				fmt.Printf("   ⚠️  Status: %s\n", status)
			}
			fmt.Println()
		}
	} else {
		fmt.Printf("Files (%d):\n", len(cfg.Mappings))
		// Sort target paths for deterministic output
		targets := make([]string, 0, len(cfg.Mappings))
		for _, target := range cfg.Mappings {
			targets = append(targets, target)
		}
		sort.Strings(targets)

		for _, target := range targets {
			fmt.Printf("📄 %s\n", target)
		}
	}

	fmt.Printf("\n📁 Repository: %s\n", cfg.Git.RepoPath)
	fmt.Printf("📊 Total tracked files: %d\n", len(cfg.Mappings))
	return nil
}

// checkSymlinkStatus verifies if a symlink exists at targetPath and correctly points to sourcePath.
// It expands tilde paths, checks symlink validity, and resolves relative symlinks to absolute paths.
// Returns two strings: (status description, status icon).
// Possible status values: "linked" (✓), "not linked" (○), "broken symlink" (✗), "points elsewhere" (✗).
func checkSymlinkStatus(sourcePath, targetPath string) (string, string) {
	// Expand target path
	expandedPath, err := util.ExpandPath(targetPath)
	if err != nil {
		return fmt.Sprintf("error expanding path %s", targetPath), "✗"
	}
	targetPath = expandedPath

	// Check if target exists
	targetInfo, err := os.Lstat(targetPath)
	if os.IsNotExist(err) {
		return "not linked", "○"
	}
	if err != nil {
		return "error checking", "✗"
	}

	// Check if target is a symlink
	if targetInfo.Mode()&os.ModeSymlink == 0 {
		// Target exists but is not a symlink (could be original file)
		return "not a symlink", "○"
	}

	// Read the symlink target
	linkTarget, err := os.Readlink(targetPath)
	if err != nil {
		return "broken symlink", "✗"
	}

	// If linkTarget is not absolute, resolve it relative to the symlink's directory
	if !filepath.IsAbs(linkTarget) {
		linkTarget = filepath.Join(filepath.Dir(targetPath), linkTarget)
	}

	// Now resolve to absolute paths for comparison
	absLinkTarget, err := filepath.Abs(linkTarget)
	if err != nil {
		return "error resolving link", "✗"
	}
	absSourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return "error resolving source", "✗"
	}

	// Check if symlink points to the correct source
	if absLinkTarget != absSourcePath {
		return "points elsewhere", "✗"
	}

	// Verify source file exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return "source missing", "✗"
	}

	return "linked", "✓"
}
