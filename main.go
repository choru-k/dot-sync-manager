package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/sync"
)

const (
	// Default configuration file path
	defaultConfigFile = "~/.dotfile-sync.json"
)

var (
	configFile = flag.String("config", defaultConfigFile, "Path to configuration file")
	version    = flag.Bool("version", false, "Show version information")
	daemon     = flag.Bool("daemon", false, "Run as daemon (background)")
	verbose    = flag.Bool("verbose", false, "Enable verbose logging")
)

// Application represents the main application
type Application struct {
	config      *config.SyncConfig
	gitManager  *gitmanager.GitManager
	syncService *sync.SyncService
	ctx         context.Context
	cancel      context.CancelFunc
}

func main() {
	flag.Parse()

	if *version {
		fmt.Println("Dotfile Sync Manager v1.0.0")
		os.Exit(0)
	}

	// Expand config file path
	configPath := expandPath(*configFile)

	// Load configuration
	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create application
	app, err := NewApplication(cfg)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the application
	if err := app.Start(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}

	// Wait for shutdown signal
	<-sigChan
	log.Println("Received shutdown signal")

	// Stop the application
	if err := app.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Application stopped")
}

// NewApplication creates a new application instance
func NewApplication(cfg *config.SyncConfig) (*Application, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create git manager
	gitConfig := cfg.ToGitManagerConfig()
	gitManager, err := gitmanager.NewGitManager(ctx, gitConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create git manager: %w", err)
	}

	// Create sync service configuration
	syncServiceConfig := cfg.ToSyncServiceConfig()

	syncConfig := &sync.Config{
		RepoPath:        syncServiceConfig.RepoPath,
		DebounceDelay:   syncServiceConfig.DebounceDelay,
		AutoSyncEnabled: syncServiceConfig.AutoSyncEnabled,
		IgnoreFile:      syncServiceConfig.IgnoreFile,
		Backoff:         syncServiceConfig.Backoff,
	}

	// Create sync service
	syncService, err := sync.New(gitManager, syncConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create sync service: %w", err)
	}

	// Set up event callbacks
	syncService.SetEventCallbacks(
		func() {
			log.Println("Sync started")
		},
		func(files []string, err error) {
			if err != nil {
				log.Printf("Sync failed: %v", err)
			} else if len(files) > 0 {
				log.Printf("Sync completed: %d files synced", len(files))
			} else {
				log.Println("Sync completed: no changes")
			}
		},
		func(err error) {
			log.Printf("Sync error: %v", err)
		},
	)

	return &Application{
		config:      cfg,
		gitManager:  gitManager,
		syncService: syncService,
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// Start starts the application
func (app *Application) Start() error {
	log.Printf("Starting Dotfile Sync Manager")
	log.Printf("Repository: %s", app.config.Git.RepoPath)
	log.Printf("Machine: %s", app.config.Machine.Name)
	log.Printf("Auto-sync: %v", app.config.Sync.AutoSyncEnabled)

	// Start sync service
	if err := app.syncService.Start(); err != nil {
		return fmt.Errorf("failed to start sync service: %w", err)
	}

	// Perform initial sync if enabled
	if app.config.Sync.AutoSyncEnabled {
		log.Println("Performing initial sync check")
		if err := app.syncService.ManualSync(); err != nil {
			log.Printf("Initial sync failed: %v", err)
		}
	}

	log.Println("Application started successfully")
	return nil
}

// Stop stops the application gracefully
func (app *Application) Stop() error {
	log.Println("Stopping application...")

	// Cancel context
	app.cancel()

	// Stop sync service
	app.syncService.Stop()

	log.Println("Application stopped")
	return nil
}

// GetStatus returns the current status of the application
func (app *Application) GetStatus() map[string]interface{} {
	stats := app.syncService.GetStats()

	return map[string]interface{}{
		"machine":     app.config.Machine.Name,
		"config_file": *configFile,
		"sync_stats":  stats,
		"running":     app.syncService.IsRunning(),
	}
}

// Helper functions

// expandPath expands ~ to user home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, path[1:])
		}
	}
	return path
}

// setupLogging configures logging based on verbosity
func setupLogging(verbose bool) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if verbose {
		log.SetOutput(os.Stdout)
	} else {
		log.SetOutput(os.Stderr)
	}
}

// init is called before main
func init() {
	// Set up logging
	setupLogging(*verbose)

	// Log startup
	log.Printf("Dotfile Sync Manager starting (verbose=%v)", *verbose)
}
