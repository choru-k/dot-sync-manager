package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
)

func main() {
	ctx := context.Background()

	// Example configuration - this would normally come from config files or CLI args
	cfg := gitmanager.Config{
		RepoPath:    "/tmp/dotfile-sync-manager-test",
		RemoteURL:   "https://github.com/example/dotfiles.git",
		RemoteName:  "origin",
		AuthorName:  "Dotfile Sync Manager",
		AuthorEmail: "sync@example.com",
	}

	// Initialize GitManager
	gm, err := gitmanager.NewGitManager(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize GitManager: %v", err)
	}

	// Example usage - this would be replaced with actual application logic
	fmt.Println("Dotfile Sync Manager initialized successfully")
	fmt.Printf("Repository: %s\n", gm.Repo())

	// For now, just print a success message
	fmt.Println("Application started successfully!")
}
