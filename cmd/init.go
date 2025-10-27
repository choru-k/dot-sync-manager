package cmd

import (
	"bufio"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/util"
	"github.com/spf13/cobra"
)

var (
	gitURL      string
	repoPath    string
	authorName  string
	authorEmail string
	force       bool
)

const (
	// forceConfirmationKeyword is the exact string users must type to confirm destructive operations
	forceConfirmationKeyword = "DELETE"

	// restrictiveConfigFilePerms is for sensitive configuration data
	restrictiveConfigFilePerms = 0600 // owner read/write only
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init [git-url]",
	Short: "Initialize dotfiles repository",
	Long: `Initialize a dotfiles repository either by cloning an existing Git repository
or creating a new one.

Examples:
  dsm init                           # Create new local repository
  dsm init https://github.com/user/dotfiles.git  # Clone existing repository`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVar(&repoPath, "path", "~/dotfiles", "Path to dotfiles directory")
	initCmd.Flags().StringVar(&authorName, "name", "", "Git author name")
	initCmd.Flags().StringVar(&authorEmail, "email", "", "Git author email")
	initCmd.Flags().BoolVar(&force, "force", false, "Force initialization even if directory exists")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Get Git URL from argument or flag
	if len(args) > 0 {
		gitURL = args[0]
	}

	// Expand repo path
	var (
		err              error
		expandedRepoPath string
	)
	expandedRepoPath, err = util.ExpandPath(repoPath)
	if err != nil {
		return fmt.Errorf("failed to expand repo path: %w", err)
	}
	repoPath = expandedRepoPath

	// Check if directory already exists
	if _, statErr := os.Stat(repoPath); statErr == nil {
		if !force {
			return fmt.Errorf(`directory %s already exists

Options:
  - Use --force to reinitialize
  - Use a different --path
  - Remove the existing directory first`, repoPath)
		}

		// Confirm before removing directory (require strong confirmation)
		fmt.Printf("⚠️  WARNING: --force will delete the entire directory: %s\n", repoPath)
		fmt.Printf("⚠️  This action CANNOT be undone!\n")
		confirmation, err := promptForInput(fmt.Sprintf("Type '%s' in all caps to confirm: ", forceConfirmationKeyword), "")
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		if confirmation != forceConfirmationKeyword {
			return fmt.Errorf("operation cancelled (you must type '%s' exactly)", forceConfirmationKeyword)
		}

		if err := os.RemoveAll(repoPath); err != nil {
			return fmt.Errorf("failed to remove existing directory: %w", err)
		}
		fmt.Println("✅ Directory removed")
	}

	// Get machine name
	machineName := getMachineNameFromOS()

	// Get Git author info if not provided
	if authorName == "" {
		authorName, err = promptForNonEmpty("Enter your name: ", "author name")
		if err != nil {
			return err
		}
	}

	// Get email from flag or prompt, then validate once
	if authorEmail == "" {
		authorEmail, err = promptForNonEmpty("Enter your email: ", "author email")
		if err != nil {
			return err
		}
	}

	// Validate email format once for both flag and prompt inputs
	if _, err := mail.ParseAddress(authorEmail); err != nil {
		inputSource := "prompt"
		if cmd.Flags().Changed("email") {
			inputSource = "--email flag"
		}
		return fmt.Errorf("invalid email from %s %q: %w\nExample: user@example.com", inputSource, authorEmail, err)
	}

	ctx := cmd.Context()

	// Create repository
	gmCfg := gitmanager.Config{
		RepoPath:    repoPath,
		RemoteURL:   gitURL,
		RemoteName:  gitmanager.DefaultRemoteName,
		AuthorName:  authorName,
		AuthorEmail: authorEmail,
		AuthType:    gitmanager.AuthStrategyNone,
	}

	if gitURL != "" {
		fmt.Printf("Cloning repository from %s...\n", gitURL)
	} else {
		fmt.Printf("Creating new repository in %s...\n", repoPath)
	}

	if _, err := gitmanager.NewGitManager(ctx, gmCfg); err != nil {
		return fmt.Errorf("failed to set up repository: %w", err)
	}

	if gitURL != "" {
		fmt.Println("✅ Repository cloned")
	} else {
		fmt.Println("✅ Repository initialized")
	}

	// Check if cloning an existing repo with config already present
	configPath := filepath.Join(repoPath, ConfigFileName)
	ignorePath := filepath.Join(repoPath, ".syncignore")
	configExists := false
	ignoreExists := false

	if _, err := os.Stat(configPath); err == nil {
		configExists = true
	}
	if _, err := os.Stat(ignorePath); err == nil {
		ignoreExists = true
	}

	// Create or update config for this machine
	var cfg *config.SyncConfig

	if !configExists {
		// Create new configuration using defaults
		cfg, err = config.DefaultConfig()
		if err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}
		fmt.Printf("✅ Configuration created: %s\n", configPath)

	} else {
		// Load existing config and update for this machine
		cfg, err = config.LoadFromFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to load existing config: %w", err)
		}
		fmt.Printf("✅ Loaded existing configuration, updating for this machine\n")
		// Preserve existing AuthType when loading from cloned repo
	}

	// Update machine-specific fields for new location/machine
	cfg.Machine.Name = machineName
	cfg.Git.RepoPath = repoPath

	// Only override remote values when cloning
	if gitURL != "" {
		cfg.Git.RemoteURL = gitURL
		if !configExists || cfg.Git.RemoteName == "" {
			cfg.Git.RemoteName = gitmanager.DefaultRemoteName
		}
	}

	cfg.Git.AuthorName = authorName
	cfg.Git.AuthorEmail = authorEmail

	// Adjust paths relative to the new repo location
	cfg.ConflictResolution.BackupDir = filepath.Join(repoPath, ".backup")
	cfg.ConfigPath = configPath

	if err := cfg.SaveToFile(configPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Set restrictive permissions on config file (may contain sensitive data)
	if err := os.Chmod(configPath, restrictiveConfigFilePerms); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	// Only create .syncignore if it doesn't exist
	if !ignoreExists {
		if err := util.CreateFileSecurely(ignorePath, []byte(defaultSyncIgnoreContent), defaultFilePerms); err != nil {
			return fmt.Errorf("failed to create .syncignore file: %w", err)
		}
		fmt.Printf("✅ Ignore file created: %s\n", ignorePath)
	} else {
		fmt.Printf("✅ Using existing ignore file: %s\n", ignorePath)
	}

	fmt.Printf("\n✅ Dotfiles repository initialized successfully!\n")
	fmt.Printf("📁 Repository: %s\n", repoPath)

	fmt.Printf("\n⚠️  SECURITY WARNING: Configuration file may contain sensitive data.\n")
	fmt.Printf("   Never commit %s to version control repositories.\n", ConfigFileName)

	if gitURL == "" {
		fmt.Printf("\n📝 Next steps:\n")
		fmt.Printf("1. Add dotfiles: dsm add ~/.bashrc\n")
		fmt.Printf("2. Set up remote: cd %s && git remote add origin <your-repo-url>\n", repoPath)
		fmt.Printf("3. Push initial commit: cd %s && git push -u origin main\n", repoPath)
		fmt.Printf("4. Start daemon: dsm start\n")
	} else {
		fmt.Printf("\n📝 Next steps:\n")
		fmt.Printf("1. Add dotfiles: dsm add ~/.bashrc\n")
		fmt.Printf("2. Start daemon: dsm start\n")
	}

	return nil
}


// promptForNonEmpty repeatedly prompts the user until a non-empty value is entered.
// It displays a friendly error message with the field name capitalized.
// fieldName is used for error messages (e.g., "email" becomes "Email").
func promptForNonEmpty(prompt, fieldName string) (string, error) {
	for {
		value, err := promptForInput(prompt, "")
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", fieldName, err)
		}
		if value != "" {
			return value, nil
		}
		// Capitalize first letter for display
		displayName := fieldName
		if len(fieldName) > 0 {
			displayName = strings.ToUpper(fieldName[:1]) + fieldName[1:]
		}
		fmt.Printf("%s cannot be empty. Please try again.\n", displayName)
	}
}

// promptForInput reads a single line from stdin with support for default values.
// If defaultValue is provided, it displays the default in brackets and returns it if user enters nothing.
// Returns the trimmed input value (either user input or default).
func promptForInput(prompt, defaultValue string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)

	if defaultValue != "" {
		fmt.Printf("(%s) ", defaultValue)
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading input: %w", err)
	}
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue, nil
	}
	return input, nil
}
