package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/spf13/cobra"
)

var (
	gitURL      string
	repoPath    string
	authorName  string
	authorEmail string
	force       bool
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
	repoPath = expandPath(repoPath)

	// Check if directory already exists
	if _, err := os.Stat(repoPath); err == nil && !force {
		return fmt.Errorf("directory %s already exists. Use --force to overwrite", repoPath)
	}

	// Get machine name
	machineName, err := getMachineName()
	if err != nil {
		return fmt.Errorf("failed to get machine name: %w", err)
	}

	// Get Git author info if not provided
	if authorName == "" {
		authorName = promptForInput("Enter your name: ", "")
	}
	if authorEmail == "" {
		authorEmail = promptForInput("Enter your email: ", "")
	}

	// Create repository
	if gitURL != "" {
		// Clone existing repository
		fmt.Printf("Cloning repository from %s...\n", gitURL)

		// TODO: Integrate with gitmanager to clone repository
		// This should call gitmanager.Clone(ctx, gitURL, repoPath, auth)
		fmt.Printf("⚠️  Git cloning not yet implemented - please clone manually:\n")
		fmt.Printf("   git clone %s %s\n", gitURL, repoPath)
	} else {
		// Create new repository
		fmt.Printf("Creating new repository in %s...\n", repoPath)

		// Create directory
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// TODO: Integrate with gitmanager to initialize repository
		// This should call gitmanager.Init(ctx, repoPath)
		fmt.Printf("⚠️  Git initialization not yet implemented - please initialize manually:\n")
		fmt.Printf("   cd %s && git init\n", repoPath)
	}

	// Create configuration
	cfg := &config.SyncConfig{
		Version: "1.0",
		Machine: config.MachineConfig{
			Name: machineName,
		},
		Git: config.GitConfig{
			RepoPath:    repoPath,
			RemoteURL:   gitURL,
			RemoteName:  "origin",
			Branch:      "main",
			AuthorName:  authorName,
			AuthorEmail: authorEmail,
			AuthType:    gitmanager.AuthStrategySSH,
		},
		Sync: config.SyncSettings{
			AutoSyncEnabled:     true,
			PullIntervalSeconds: 300, // 5 minutes
			DebounceSeconds:     30,  // 30 seconds
			AutoCommit:          true,
			AutoPush:            true,
			AutoPull:            true,
		},
		Notifications: config.NotificationConfig{
			Enabled:             true,
			ShowSuccess:         false,
			ShowPulls:           true,
			PlaySoundOnConflict: false,
		},
		ConflictResolution: config.ConflictConfig{
			Strategy:        "manual",
			BackupDir:       filepath.Join(repoPath, ".backup"),
			KeepBackupsDays: 7,
		},
		Mappings: make(map[string]string),
		UI: config.UIConfig{
			StartAtBoot:    false,
			MinimizeToTray: true,
			Theme:          "auto",
		},
		Advanced: config.AdvancedConfig{
			DebugLogging: false,
			LogFile:      filepath.Join(os.Getenv("HOME"), ".dotfile-sync.log"),
			MaxLogSizeMB: 10,
		},
	}

	// Save configuration
	configPath := filepath.Join(repoPath, ".sync-config.json")
	if err := cfg.SaveToFile(configPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Create default .syncignore file
	ignorePath := filepath.Join(repoPath, ".syncignore")
	ignoreContent := `# Sensitive authentication files
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
	if err := os.WriteFile(ignorePath, []byte(ignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to create .syncignore file: %w", err)
	}

	fmt.Printf("✅ Dotfiles repository initialized successfully!\n")
	fmt.Printf("📁 Repository: %s\n", repoPath)
	fmt.Printf("⚙️  Configuration: %s\n", configPath)
	fmt.Printf("📄 Ignore file: %s\n", ignorePath)

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


func getMachineName() (string, error) {
	if hostname, err := os.Hostname(); err == nil {
		return hostname, nil
	}
	return "unknown-machine", nil
}

func promptForInput(prompt, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)

	if defaultValue != "" {
		fmt.Printf("(%s) ", defaultValue)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue
	}
	return input
}