package cmd

import (
	"bufio"
	"fmt"
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
	// Default remote name for git operations
	defaultRemoteName = "origin"
)

// Default .syncignore content
const defaultSyncIgnoreContent = `# Sensitive authentication files
.ssh/id_rsa
.ssh/id_rsa.pub
.ssh/id_ed25519
.ssh/id_ed25519.pub
*.pem
*.key

# Cloud credentials
.aws/credentials
.aws/config
.gcp/credentials
.azure/credentials

# GPG keys
.gnupg/private-keys-v1.d/
.gnupg/*.key

# Other sensitive
.env
.env.local
secrets/

# Temporary files
*.tmp
*.log
*.swp
*~
.DS_Store
Thumbs.db

# Cache directories
.cache/
cache/
node_modules/
`

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
	var err error
	expandedRepoPath, err := util.ExpandPath(repoPath)
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
		
		// Confirm before removing directory
		fmt.Printf("⚠️  Warning: --force will delete the entire directory: %s\n", repoPath)
		confirmation, err := promptForInput("Type 'yes' to confirm deletion: ", "")
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		if confirmation != "yes" {
			return fmt.Errorf("operation cancelled")
		}
		
		if err := os.RemoveAll(repoPath); err != nil {
			return fmt.Errorf("failed to remove existing directory: %w", err)
		}
		fmt.Println("✅ Directory removed")
	}

	// Get machine name
	machineName := getMachineName()

	// Get Git author info if not provided
	if authorName == "" {
		authorName, err = promptForNonEmpty("Enter your name: ", "author name")
		if err != nil {
			return err
		}
	}
	if authorEmail == "" {
		authorEmail, err = promptForNonEmpty("Enter your email: ", "author email")
		if err != nil {
			return err
		}
	}

	ctx := cmd.Context()

	// Create repository
	gmCfg := gitmanager.Config{
		RepoPath:    repoPath,
		RemoteURL:   gitURL,
		RemoteName:  defaultRemoteName,
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
	configPath := filepath.Join(repoPath, ".sync-config.json")
	ignorePath := filepath.Join(repoPath, ".syncignore")
	configExists := false
	ignoreExists := false
	
	if _, err := os.Stat(configPath); err == nil {
		configExists = true
	}
	if _, err := os.Stat(ignorePath); err == nil {
		ignoreExists = true
	}

	// Only create config if it doesn't exist (e.g., new repo or clone without config)
	if !configExists {
		// Create configuration using defaults, then override user-provided values
		cfg := config.DefaultConfig()
		
		// Override with user-provided values
		cfg.Machine.Name = machineName
		cfg.Git.RepoPath = repoPath
		cfg.Git.RemoteURL = gitURL
		cfg.Git.RemoteName = defaultRemoteName
		cfg.Git.AuthorName = authorName
		cfg.Git.AuthorEmail = authorEmail
		cfg.Git.AuthType = gitmanager.AuthStrategyNone // Override default SSH for init
		
		// Adjust paths relative to the new repo
		cfg.ConflictResolution.BackupDir = filepath.Join(repoPath, ".backup")
		cfg.ConfigPath = configPath

		if err := cfg.SaveToFile(configPath); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}
		fmt.Printf("✅ Configuration created: %s\n", configPath)
	} else {
		fmt.Printf("✅ Using existing configuration: %s\n", configPath)
	}

	// Only create .syncignore if it doesn't exist
	if !ignoreExists {
		if err := os.WriteFile(ignorePath, []byte(defaultSyncIgnoreContent), 0644); err != nil {
			return fmt.Errorf("failed to create .syncignore file: %w", err)
		}
		fmt.Printf("✅ Ignore file created: %s\n", ignorePath)
	} else {
		fmt.Printf("✅ Using existing ignore file: %s\n", ignorePath)
	}

	fmt.Printf("\n✅ Dotfiles repository initialized successfully!\n")
	fmt.Printf("📁 Repository: %s\n", repoPath)

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

func getMachineName() string {
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}
	return "unknown-machine"
}

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
		displayName := strings.ToUpper(fieldName[:1]) + fieldName[1:]
		fmt.Printf("%s cannot be empty. Please try again.\n", displayName)
	}
}

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
