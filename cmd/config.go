package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/config"
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

	// Add flags
	configCmd.PersistentFlags().String("editor", "", "Editor to use for editing configuration")

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
		RunE: runConfig,
	}
	configCmd.AddCommand(configEditCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	// Only require subcommand if this is the main config command (not edit subcommand)
	if (cmd.Use == "config [subcommand]" || cmd.Use == "config") && len(args) == 0 {
		return fmt.Errorf("config command requires a subcommand (get, set, edit)")
	}

	// If this is the main config command and a single subcommand name is provided
	// without the required arguments for that subcommand, error out
	if (cmd.Use == "config [subcommand]" || cmd.Use == "config") && len(args) == 1 {
		switch args[0] {
		case "get", "set":
			// These subcommands require additional arguments
			return fmt.Errorf("config subcommand %s requires additional arguments", args[0])
		case "edit":
			// edit subcommand with no arguments is valid and will open the editor
			break
		default:
			// Unknown subcommand
			return fmt.Errorf("unknown config subcommand: %s", args[0])
		}
	}

	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	configPath := cfg.GetConfigPath()

	// Open config file in editor
	var editor string

	// Check for editor flag first
	if flagEditor, err := cmd.Flags().GetString("editor"); err == nil && flagEditor != "" {
		editor, err = validateEditorCommand(flagEditor)
		if err != nil {
			return fmt.Errorf("invalid editor flag: %w", err)
		}
	} else if envEditor := os.Getenv("EDITOR"); envEditor != "" {
		// Check environment variable second
		editor, err = validateEditorCommand(envEditor)
		if err != nil {
			return fmt.Errorf("invalid EDITOR environment variable: %w", err)
		}
	} else {
		// Use centralized editor selection logic
		editor, err = getDefaultEditorForFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to get default editor: %w", err)
		}
	}

	fmt.Printf("📝 Opening configuration file: %s\n", configPath)
	fmt.Printf("Using editor: %s\n", editor)

	// Parse editor command to handle spaces properly
	editorCmd, editorArgs := parseCommand(editor)
	editorArgs = append(editorArgs, configPath)

	execCmd := exec.Command(editorCmd, editorArgs...)
	if output, err := execCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to open editor: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]

	// Validate configuration key for security and format
	if err := validateConfigKey(key); err != nil {
		return fmt.Errorf("invalid configuration key: %w", err)
	}

	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Convert struct to map for nested access
	cfgMap, err := structToMap(cfg)
	if err != nil {
		return fmt.Errorf("failed to convert configuration: %w", err)
	}

	value := getNestedValue(cfgMap, key)

	if value == nil {
		return fmt.Errorf("configuration key not found: %s", key)
	}

	fmt.Printf("%s: %v\n", key, value)
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]

	// Validate configuration key for security and format
	if err := validateConfigKey(key); err != nil {
		return fmt.Errorf("invalid configuration key: %w", err)
	}

	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Convert struct to map for nested access
	cfgMap, err := structToMap(cfg)
	if err != nil {
		return fmt.Errorf("failed to convert configuration: %w", err)
	}

	value := args[1]

	// Try to parse value as appropriate type
	parsedValue := parseConfigValue(value)

	if !setNestedValueInMap(cfgMap, key, parsedValue) {
		return fmt.Errorf("failed to set configuration key: %s", key)
	}

	// Convert back to struct and save
	updatedCfg, err := mapToStruct(cfgMap)
	if err != nil {
		return fmt.Errorf("failed to convert back to configuration: %w", err)
	}

	// Save updated configuration
	configPath := cfg.GetConfigPath()
	// Ensure the config path is preserved in the updated config
	updatedCfg.ConfigPath = configPath
	if err := updatedCfg.SaveToFile(configPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("✅ Set %s = %v\n", key, parsedValue)
	fmt.Printf("💡 Configuration saved to: %s\n", configPath)
	return nil
}


// Helper functions for nested configuration access
func getNestedValue(obj interface{}, key string) interface{} {
	parts := strings.Split(key, ".")

	// Convert struct to map using JSON marshaling/unmarshaling to respect custom JSON tags
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	var mapObj map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &mapObj); err != nil {
		return nil
	}

	current := interface{}(mapObj)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
			if current == nil {
				return nil
			}
		case string:
			// Special handling for machine.name access when machine is a string (PRD format)
			if len(parts) == 2 && parts[0] == "machine" && parts[1] == "name" {
				return v // Return the machine name string directly
			}
			return nil
		default:
			return nil
		}
	}

	return current
}

func setNestedValue(obj interface{}, key string, value interface{}) bool {
	// Validate key
	if key == "" {
		return false
	}
	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return false
	}

	// Validate that the section is valid (using the same validation as validateConfigKey)
	section := parts[0]
	validSections := map[string]bool{
		"machine":            true,
		"git":                true,
		"sync":               true,
		"notifications":      true,
		"conflict_resolution": true,
		"ui":                 true,
		"advanced":           true,
	}

	if !validSections[section] {
		return false
	}

	// Handle pointer to struct by dereferencing first
	var actualObj interface{}
	switch v := obj.(type) {
	case *config.SyncConfig:
		actualObj = *v
	default:
		actualObj = obj
	}

	// Convert struct to map using JSON marshaling/unmarshaling to respect custom JSON tags
	jsonBytes, err := json.Marshal(actualObj)
	if err != nil {
		return false
	}
	mapObj, err := jsonToMap(jsonBytes)
	if err != nil {
		return false
	}

	// Set the nested value in the map
	if !setNestedValueRecursive(mapObj, parts, value) {
		return false
	}

	// Convert back to struct
	updatedCfg, err := mapToStruct(mapObj)
	if err != nil {
		return false
	}

	// Try to update the original object if it's a pointer to our config struct
	switch v := obj.(type) {
	case *config.SyncConfig:
		*v = *updatedCfg
		return true
	default:
		// For other cases, we can't modify the original
		// but the conversion worked, so we consider it successful
		return true
	}
}

func setNestedValueRecursive(obj interface{}, parts []string, value interface{}) bool {
	if len(parts) == 0 {
		return false // Can't set root object
	}

	part := parts[0]
	remaining := parts[1:]

	switch v := obj.(type) {
	case map[string]interface{}:
		// Special handling for machine.name when machine is currently a string (PRD format)
		if part == "machine" && len(remaining) == 1 && remaining[0] == "name" {
			if _, isString := v["machine"].(string); isString {
				// In PRD format, machine is a string, so just set machine directly
				if valueStr, ok := value.(string); ok {
					v["machine"] = valueStr
					return true
				}
			}
		}

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

// jsonToMap converts JSON bytes to a map[string]interface{}
func jsonToMap(jsonBytes []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}
	return result, nil
}

// setNestedValueInMap sets a nested value directly in a map[string]interface{}
func setNestedValueInMap(cfgMap map[string]interface{}, key string, value interface{}) bool {
	parts := strings.Split(key, ".")
	return setNestedValueRecursive(cfgMap, parts, value)
}

func parseConfigValue(value string) interface{} {
	// Try to parse as int
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	// Try to parse as float
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	// Try to parse as bool
	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}
	// Default to string
	return value
}

// structToMap converts a struct to a map[string]interface{} using JSON marshaling
func structToMap(obj interface{}) (map[string]interface{}, error) {
	// Marshal to JSON
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct: %w", err)
	}

	// Unmarshal to map
	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	return result, nil
}

// mapToStruct converts a map[string]interface{} to the config struct type
func mapToStruct(data map[string]interface{}) (*config.SyncConfig, error) {
	// Marshal to JSON
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal map: %w", err)
	}

	// Unmarshal to struct
	var result config.SyncConfig
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to struct: %w", err)
	}

	return &result, nil
}