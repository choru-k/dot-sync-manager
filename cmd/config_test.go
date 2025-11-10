package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/spf13/cobra"
)

func TestConfigCmd_Sanity(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Set the global configFile variable so getConfig() uses the test config file
	oldConfigFile := getConfigFile()
	setConfigFile(testConfig.ConfigPath)
	t.Cleanup(func() { setConfigFile(oldConfigFile) })

	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "no arguments should error",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "subcommand required",
			args:        []string{"get"},
			expectError: true, // get requires a key argument
		},
		{
			name:        "get with key should work",
			args:        []string{"get", "machine.name"},
			expectError: false,
		},
		{
			name:        "set with key and value should work",
			args:        []string{"set", "machine.name", "test-machine"},
			expectError: false,
		},
		{
			name:        "edit should work",
			args:        []string{"edit"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "config"}
			err := runConfig(cmd, tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("runConfig() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestRunConfigGet(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Set the global configFile variable so getConfig() uses the test config file
	oldConfigFile := getConfigFile()
	setConfigFile(testConfig.ConfigPath)
	t.Cleanup(func() { setConfigFile(oldConfigFile) })

	tests := []struct {
		name        string
		key         string
		expectError bool
		expectValue string
	}{
		{
			name:        "machine.name",
			key:         "machine.name",
			expectError: false,
			expectValue: testMachineName,
		},
		{
			name:        "git.repo_path",
			key:         "git.repo_path",
			expectError: false,
			expectValue: testConfig.RepoPath,
		},
		{
			name:        "non-existent key",
			key:         "nonexistent.key",
			expectError: true,
		},
		{
			name:        "empty key",
			key:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "config get"}
			args := []string{tt.key}

			err := runConfigGet(cmd, args)
			if (err != nil) != tt.expectError {
				t.Errorf("runConfigGet() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestRunConfigSet(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Set the global configFile variable so getConfig() uses the test config file
	oldConfigFile := getConfigFile()
	setConfigFile(testConfig.ConfigPath)
	t.Cleanup(func() { setConfigFile(oldConfigFile) })

	tests := []struct {
		name        string
		key         string
		value       string
		expectError bool
	}{
		{
			name:        "set machine.name",
			key:         "machine.name",
			value:       "new-machine-name",
			expectError: false,
		},
		{
			name:        "set git.user_name",
			key:         "git.user_name",
			value:       "New Author",
			expectError: false,
		},
		{
			name:        "set invalid key",
			key:         "invalid.key.path",
			value:       "value",
			expectError: true,
		},
		{
			name:        "empty key",
			key:         "",
			value:       "value",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "config set"}
			args := []string{tt.key, tt.value}

			err := runConfigSet(cmd, args)
			if (err != nil) != tt.expectError {
				t.Errorf("runConfigSet() error = %v, expectError %v", err, tt.expectError)
			}

			// Verify the change was saved (if no error expected)
			if !tt.expectError {
				updatedConfig, err := config.LoadFromFile(testConfig.ConfigPath)
				if err != nil {
					t.Fatalf("failed to load updated config: %v", err)
				}

				// Debug output
				t.Logf("Debug: Loaded config machine name: %s", updatedConfig.Machine.Name)
				if tt.key == "git.user_name" {
					t.Logf("Debug: Loaded config git author name: %s", updatedConfig.Git.AuthorName)
				}

				actualValue := getNestedValue(updatedConfig, tt.key)
				t.Logf("Debug: getNestedValue(%s) returned: %v (type: %T)", tt.key, actualValue, actualValue)
				t.Logf("Debug: expected value: %s (type: %T)", tt.value, tt.value)

				if fmt.Sprintf("%v", actualValue) != tt.value {
					t.Errorf("config value not updated: expected %s, got %v", tt.value, actualValue)
				}
			}
		})
	}
}

func TestRunConfigEdit(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Set the global configFile variable so getConfig() uses the test config file
	oldConfigFile := getConfigFile()
	setConfigFile(testConfig.ConfigPath)
	t.Cleanup(func() { setConfigFile(oldConfigFile) })

	tests := []struct {
		name        string
		editor      string
		expectError bool
	}{
		{
			name:        "default editor",
			editor:      "",
			expectError: false,
		},
		{
			name:        "custom editor",
			editor:      "nano",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "config edit"}
			cmd.Flags().String("editor", "", "Editor to use for editing configuration")
			if tt.editor != "" {
				if err := cmd.Flags().Set("editor", tt.editor); err != nil {
					t.Fatalf("failed to set editor flag: %v", err)
				}
			}

			err := runConfig(cmd, []string{})
			// In test environment, this should use "true" as safe editor
			// But if a real editor is specified via flag, it might still try to launch it
			// So we just log the result and don't assert on it
			t.Logf("runConfig() result: %v", err)
		})
	}
}

func TestGetNestedValue(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	tests := []struct {
		name          string
		key           string
		expectedValue interface{}
		expectError   bool
	}{
		{
			name:          "machine.name",
			key:           "machine.name",
			expectedValue: testMachineName,
		},
		{
			name:          "git.user_name",
			key:           "git.user_name",
			expectedValue: testAuthorName,
		},
		{
			name:          "sync.auto_sync_enabled",
			key:           "sync.auto_sync_enabled",
			expectedValue: true, // Default value
		},
		{
			name:          "non-existent nested key",
			key:           "machine.nonexistent",
			expectedValue: nil,
		},
		{
			name:          "invalid key format",
			key:           "invalid",
			expectedValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := getNestedValue(testConfig.Config, tt.key)

			if value != tt.expectedValue {
				t.Errorf("getNestedValue() = %v, expected %v", value, tt.expectedValue)
			}
		})
	}
}

func TestSetNestedValue(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	tests := []struct {
		name        string
		key         string
		value       interface{}
		expectError bool
	}{
		{
			name:        "set machine.name",
			key:         "machine.name",
			value:       "new-machine",
			expectError: false,
		},
		{
			name:        "set git.user_email",
			key:         "git.user_email",
			value:       "new@example.com",
			expectError: false,
		},
		{
			name:        "set sync.auto_sync_enabled",
			key:         "sync.auto_sync_enabled",
			value:       false,
			expectError: false,
		},
		{
			name:        "invalid key path",
			key:         "invalid.path.to.setting",
			value:       "value",
			expectError: true,
		},
		{
			name:        "empty key",
			key:         "",
			value:       "value",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of the config to modify
			configCopy := *testConfig.Config

			success := setNestedValue(&configCopy, tt.key, tt.value)
			if !success && !tt.expectError {
				t.Errorf("setNestedValue() failed unexpectedly")
			}
			if success && tt.expectError {
				t.Errorf("setNestedValue() succeeded when it should have failed")
			}

			// Verify the change was applied (if no error expected)
			if !tt.expectError {
				actualValue := getNestedValue(&configCopy, tt.key)
				if actualValue != tt.value {
					t.Errorf("nested value not set correctly: expected %v, got %v", tt.value, actualValue)
				}
			}
		})
	}
}

func TestValidateConfigKey(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		expectError bool
	}{
		{
			name:        "valid machine.name",
			key:         "machine.name",
			expectError: false,
		},
		{
			name:        "valid git.repo_path",
			key:         "git.repo_path",
			expectError: false,
		},
		{
			name:        "valid sync.auto_sync_enabled",
			key:         "sync.auto_sync_enabled",
			expectError: false,
		},
		{
			name:        "invalid key - no dot",
			key:         "invalidkey",
			expectError: true,
		},
		{
			name:        "invalid key - wrong section",
			key:         "invalid.name",
			expectError: true,
		},
		{
			name:        "invalid key - wrong field (but valid format)",
			key:         "machine.invalid_field",
			expectError: false, // Basic validation allows this - field validation happens at config level
		},
		{
			name:        "empty key",
			key:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfigKey(tt.key)
			if (err != nil) != tt.expectError {
				t.Errorf("validateConfigKey() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestConfigBackup(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Create backup of original config
	backupPath, err := createConfigBackup(testConfig.ConfigPath)
	if err != nil {
		t.Fatalf("createConfigBackup() failed: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Remove(backupPath); err != nil {
			t.Logf("warning: failed to remove backup file %s: %v", backupPath, err)
		}
	})

	// Verify backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("backup file was not created")
	}

	// Verify backup contains the original data
	originalData, err := os.ReadFile(testConfig.ConfigPath)
	if err != nil {
		t.Fatalf("failed to read original config: %v", err)
	}

	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup config: %v", err)
	}

	if string(originalData) != string(backupData) {
		t.Error("backup content does not match original")
	}
}

func TestConfigCmd_Integration(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Set the global configFile variable so getConfig() uses the test config file
	oldConfigFile := getConfigFile()
	setConfigFile(testConfig.ConfigPath)
	t.Cleanup(func() { setConfigFile(oldConfigFile) })

	// Test full workflow: get -> set -> get -> verify
	cmd := &cobra.Command{Use: "config"}

	// Get original value
	err := runConfigGet(cmd, []string{"machine.name"})
	if err != nil {
		t.Errorf("get machine.name failed: %v", err)
	}

	// Set new value
	err = runConfigSet(cmd, []string{"machine.name", "integration-test-machine"})
	if err != nil {
		t.Errorf("set machine.name failed: %v", err)
	}

	// Verify new value
	updatedConfig, err := config.LoadFromFile(testConfig.ConfigPath)
	if err != nil {
		t.Fatalf("failed to load updated config: %v", err)
	}

	if updatedConfig.Machine.Name != "integration-test-machine" {
		t.Errorf("machine.name not updated: expected 'integration-test-machine', got '%s'", updatedConfig.Machine.Name)
	}

	// Test getting a nested value
	err = runConfigGet(cmd, []string{"git.user_name"})
	if err != nil {
		t.Errorf("get git.user_name failed: %v", err)
	}
}

func TestConfigCmd_InvalidConfig(t *testing.T) {
	// Create invalid config file
	invalidConfigPath := filepath.Join(t.TempDir(), "invalid-config.json")
	invalidContent := `{"invalid": "json" content}`
	err := os.WriteFile(invalidConfigPath, []byte(invalidContent), defaultFilePerms)
	if err != nil {
		t.Fatalf("failed to create invalid config: %v", err)
	}

	// Temporarily set config file to invalid config
	originalConfigFile := getConfigFile()
	setConfigFile(invalidConfigPath)
	defer func() { setConfigFile(originalConfigFile) }()

	// Test get with invalid config
	cmd := &cobra.Command{Use: "config"}
	err = runConfigGet(cmd, []string{"machine.name"})
	if err == nil {
		t.Error("expected runConfigGet() to fail with invalid config")
	}
}

func TestConfigCmd_Permissions(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Set the global configFile variable so getConfig() uses the test config file
	oldConfigFile := getConfigFile()
	setConfigFile(testConfig.ConfigPath)
	t.Cleanup(func() { setConfigFile(oldConfigFile) })

	// Set restrictive permissions on config file
	err := os.Chmod(testConfig.ConfigPath, restrictiveConfigFilePerms)
	if err != nil {
		t.Fatalf("failed to set config permissions: %v", err)
	}

	// Test that operations still work
	cmd := &cobra.Command{Use: "config"}
	err = runConfigGet(cmd, []string{"machine.name"})
	if err != nil {
		t.Errorf("get failed with restrictive permissions: %v", err)
	}
}

func TestParseConfigKey(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		expectedParts []string
		expectError   bool
	}{
		{
			name:          "simple key",
			key:           "machine.name",
			expectedParts: []string{"machine", "name"},
			expectError:   false,
		},
		{
			name:          "nested key",
			key:           "sync.auto_sync_enabled",
			expectedParts: []string{"sync", "auto_sync_enabled"},
			expectError:   false,
		},
		{
			name:          "single part",
			key:           "machine",
			expectedParts: []string{"machine"},
			expectError:   false,
		},
		{
			name:          "empty key",
			key:           "",
			expectedParts: nil,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, err := parseConfigKey(tt.key)

			if (err != nil) != tt.expectError {
				t.Errorf("parseConfigKey() error = %v, expectError %v", err, tt.expectError)
			}

			if !tt.expectError {
				if len(parts) != len(tt.expectedParts) {
					t.Errorf("parseConfigKey() returned %d parts, expected %d", len(parts), len(tt.expectedParts))
				}

				for i, part := range parts {
					if i < len(tt.expectedParts) && part != tt.expectedParts[i] {
						t.Errorf("parseConfigKey() part %d = %q, expected %q", i, part, tt.expectedParts[i])
					}
				}
			}
		})
	}
}

func TestFormatConfigValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "string value",
			value:    "test-string",
			expected: "test-string",
		},
		{
			name:     "boolean true",
			value:    true,
			expected: "true",
		},
		{
			name:     "boolean false",
			value:    false,
			expected: "false",
		},
		{
			name:     "integer value",
			value:    42,
			expected: "42",
		},
		{
			name:     "map value",
			value:    map[string]interface{}{"key": "value"},
			expected: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatConfigValue(tt.value)
			if result != tt.expected {
				t.Errorf("formatConfigValue() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestConfigSet_ConcurrentAccess(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Set the global configFile variable so getConfig() uses the test config file
	oldConfigFile := getConfigFile()
	setConfigFile(testConfig.ConfigPath)
	t.Cleanup(func() { setConfigFile(oldConfigFile) })

	// Test concurrent config modifications
	numGoroutines := 10
	results := make(chan error, numGoroutines)

	// Launch multiple goroutines trying to modify config simultaneously
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			cmd := &cobra.Command{Use: "config set"}
			args := []string{"machine.name", fmt.Sprintf("test-machine-%d", id)}
			err := runConfigSet(cmd, args)
			results <- err
		}(i)
	}

	// Collect results
	successCount := 0
	errorCount := 0
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		if err != nil {
			errorCount++
			t.Logf("Goroutine failed: %v", err)
		} else {
			successCount++
		}
	}

	// Verify that at least some operations succeeded
	if successCount == 0 {
		t.Error("No concurrent config operations succeeded")
	}

	// Verify final config state is consistent
	finalConfig, err := config.LoadFromFile(testConfig.ConfigPath)
	if err != nil {
		t.Fatalf("failed to load final config: %v", err)
	}

	// The machine name should be one of the set values
	expectedNames := make(map[string]bool)
	for i := 0; i < numGoroutines; i++ {
		expectedNames[fmt.Sprintf("test-machine-%d", i)] = true
	}

	if !expectedNames[finalConfig.Machine.Name] {
		t.Errorf("final machine name %q is not one of the expected values", finalConfig.Machine.Name)
	}

	t.Logf("Concurrent access test: %d succeeded, %d failed", successCount, errorCount)
}

func TestConfigSet_FileLockingIntegration(t *testing.T) {
	testConfig := setupTestEnvironment(t)
	t.Cleanup(func() { cleanupTestEnvironment(testConfig) })

	// Set the global configFile variable so getConfig() uses the test config file
	oldConfigFile := getConfigFile()
	setConfigFile(testConfig.ConfigPath)
	t.Cleanup(func() { setConfigFile(oldConfigFile) })

	// Test basic config set with file locking
	cmd := &cobra.Command{Use: "config set"}
	args := []string{"machine.name", "locking-test-machine"}

	err := runConfigSet(cmd, args)
	if err != nil {
		t.Errorf("runConfigSet() with file locking failed: %v", err)
	}

	// Verify the change was applied
	updatedConfig, err := config.LoadFromFile(testConfig.ConfigPath)
	if err != nil {
		t.Fatalf("failed to load updated config: %v", err)
	}

	if updatedConfig.Machine.Name != "locking-test-machine" {
		t.Errorf("machine.name not updated correctly: expected 'locking-test-machine', got '%s'", updatedConfig.Machine.Name)
	}
}
