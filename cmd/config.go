package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config [subcommand]",
	Short: "Manage configuration",
	Long: `Manage the dotfile sync configuration. Allows editing the configuration
file, setting individual values, and viewing current settings.

Subcommands:
  config         Edit configuration file in default editor
  config set    Set a configuration value
  config get    Get a configuration value
  config edit    Edit configuration file in default editor

Examples:
  dsm config
  dsm config set sync.pull_interval_seconds 600
  dsm config get sync.pull_interval_seconds
  dsm config edit`,
	RunE: runConfig,
}


func init() {
	rootCmd.AddCommand(configCmd)

	// config get subcommand
	configGetCmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get configuration value",
		Long: `Get a configuration value from the sync configuration file.

Examples:
  dsm config get sync.pull_interval_seconds
  dsm config get machine.name`,
		Args: cobra.ExactArgs(1),
		RunE: runConfigGet,
	}
	configCmd.AddCommand(configGetCmd)

	// config set subcommand
	configSetCmd := &cobra.Command{
		Use:   "set <key> <value>",
	Short: "Set configuration value",
		Long: `Set a configuration value in the sync configuration file.
This will update the configuration file with the new value.

Examples:
  dsm config set sync.pull_interval_seconds 600
  dsm config set machine.name "My Laptop"`,
		Args: cobra.ExactArgs(2),
		RunE: runConfigSet,
	}
	configCmd.AddCommand(configSetCmd)

	// config edit subcommand
	configEditCmd := &cobra.Command{
		Use:   "edit",
	Short: "Edit configuration file",
		Long: `Edit the sync configuration file in the default editor.
This opens the configuration file for manual editing.

Examples:
  dsm config edit`,
		RunE: runConfigEdit,
	}
	configCmd.AddCommand(configEditCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	configPath := cfg.GetConfigPath()

	// Open config file in default editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Fallback editors by platform
		switch {
		case strings.Contains(configPath, ".json"):
			editor = "code" // Try VS Code for JSON files
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

	fmt.Printf("📝 Opening configuration file: %s\n", configPath)
	fmt.Printf("Using editor: %s\n", editor)

	execCmd := exec.Command(editor, configPath)
	if output, err := execCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to open editor: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	key := args[0]
	value := getNestedValue(cfg, key)

	if value == nil {
		return fmt.Errorf("configuration key not found: %s", key)
	}

	fmt.Printf("%s: %v\n", key, value)
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	key := args[0]
	value := args[1]

	// Try to parse value as appropriate type
	parsedValue := parseConfigValue(value)

	if !setNestedValue(cfg, key, parsedValue) {
		return fmt.Errorf("failed to set configuration key: %s", key)
	}

	// Save updated configuration
	configPath := cfg.GetConfigPath()
	if err := cfg.SaveToFile(configPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("✅ Set %s = %v\n", key, parsedValue)
	fmt.Printf("💡 Configuration saved to: %s\n", configPath)
	return nil
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	return runConfig(cmd, []string{})
}

// Helper functions for nested configuration access
func getNestedValue(obj interface{}, key string) interface{} {
	parts := strings.Split(key, ".")
	current := obj

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
			if current == nil {
				return nil
			}
		default:
			return nil
		}
	}

	return current
}

func setNestedValue(obj interface{}, key string, value interface{}) bool {
	parts := strings.Split(key, ".")
	return setNestedValueRecursive(obj, parts, value)
}

func setNestedValueRecursive(obj interface{}, parts []string, value interface{}) bool {
	if len(parts) == 0 {
		return false // Can't set root object
	}

	part := parts[0]
	remaining := parts[1:]

	switch v := obj.(type) {
	case map[string]interface{}:
		if len(remaining) == 0 {
			// Set the final value
			v[part] = value
			return true
		} else {
			// Continue nesting
			if next, exists := v[part]; exists {
				return setNestedValueRecursive(next, remaining, value)
			} else {
				// Create nested map
				v[part] = make(map[string]interface{})
				return setNestedValueRecursive(v[part], remaining, value)
			}
		}
	default:
		return false
	}
}

func parseConfigValue(value string) interface{} {
	// Try to parse as JSON for complex values
	if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
		var parsed interface{}
		if _, err := fmt.Sscanf(value, "%v", &parsed); err == nil {
			return parsed
		}
	}

	// Try to parse as integer
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal
	}

	// Try to parse as float
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal
	}

	// Try to parse as boolean
	if strings.ToLower(value) == "true" {
		return true
	}
	if strings.ToLower(value) == "false" {
		return false
	}

	// Return as string
	return value
}