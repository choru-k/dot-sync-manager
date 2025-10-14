package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	if showMappings && len(cfg.Mappings) > 0 {
		fmt.Printf("File Mappings (%d):\n\n", len(cfg.Mappings))
		for source, target := range cfg.Mappings {
			fmt.Printf("📄 %s\n", source)
			fmt.Printf("   🔗 %s\n", target)
			fmt.Println()
		}
		return nil
	}

	// Scan the dotfiles directory for actual files
	repoPath := cfg.Git.RepoPath
	var files []string
	var dirs []string

	err = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories (except .gitignore, .syncignore, .sync-config.json)
		if strings.HasPrefix(info.Name(), ".") &&
		   info.Name() != ".gitignore" &&
		   info.Name() != ".syncignore" &&
		   info.Name() != ".sync-config.json" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip .git directory
		if path == filepath.Join(repoPath, ".git") {
			return filepath.SkipDir
		}

		// Skip the root directory itself
		if path == repoPath {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			dirs = append(dirs, relPath)
		} else {
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scan repository: %w", err)
	}

	// Show directories first
	if len(dirs) > 0 {
		fmt.Printf("Directories (%d):\n", len(dirs))
		for _, dir := range dirs {
			fmt.Printf("📁 %s/\n", dir)
		}
		fmt.Println()
	}

	// Show files
	if len(files) > 0 {
		fmt.Printf("Files (%d):\n", len(files))
		for _, file := range files {
			fmt.Printf("📄 %s\n", file)
		}
	}

	if len(files) == 0 && len(dirs) == 0 {
		fmt.Println("No files tracked yet.")
		fmt.Println("\n💡 Use 'dsm add <filepath>' to add files to tracking")
	}

	fmt.Printf("\n📁 Repository: %s\n", repoPath)
	return nil
}