package scenarios

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"text/template"
)

// ConfigTemplateData represents the data available to configuration templates
type ConfigTemplateData struct {
	// Test data directories
	TestDataDir string
	TestID      string

	// Fixture directories
	FixturesDir string

	// Repository and target directories
	RepoRoot  string
	TargetDir string
	SourceDir string

	// SSH and authentication
	SSHKeyPath string

	// Machine identification
	MachineName string
	MachineID   string

	// Platform information
	Platform string
	Arch     string
}

// writeConfigFromTemplate renders a configuration template and writes it to a temporary file
// Returns the path to the generated config file
func writeConfigFromTemplate(t *testing.T, templateName string, overrides map[string]interface{}) string {
	t.Helper()

	// Initialize path resolution
	initPathResolution()

	// Default template data
	data := ConfigTemplateData{
		TestDataDir: GetTestDataDir(),
		TestID:      RequireTestID(t),
		FixturesDir: GetFixturesDir(),
		RepoRoot:    GetRepoRoot(),
		SSHKeyPath:  GetSSHKeyPath(),
		MachineName: fmt.Sprintf("test-machine-%s", t.Name()),
		MachineID:   RequireTestID(t),
		Platform:    "test",
		Arch:        "test",
	}

	// Apply template-specific overrides
	switch templateName {
	case "basic":
		data.TargetDir = filepath.Join(data.TestDataDir, "dotfiles-test")
		data.SourceDir = filepath.Join(data.TestDataDir, "source_dotfiles")
	case "watching":
		data.TargetDir = filepath.Join(data.TestDataDir, "dotfiles-test-watch")
		data.SourceDir = filepath.Join(data.TestDataDir, "source_dotfiles")
		data.MachineName = "test-machine-watching"
	case "cross_platform":
		data.TargetDir = filepath.Join(data.TestDataDir, "dotfiles-test-crossplatform")
		data.SourceDir = filepath.Join(data.TestDataDir, "source_dotfiles")
		data.MachineName = "test-machine-crossplatform"
	case "conflict":
		data.TargetDir = filepath.Join(data.TestDataDir, "dotfiles-test-conflict")
		data.SourceDir = filepath.Join(data.TestDataDir, "source_dotfiles")
		data.MachineName = "test-machine-conflict"
	}

	// Apply user overrides
	for key, value := range overrides {
		switch key {
		case "TargetDir":
			data.TargetDir = value.(string)
		case "SourceDir":
			data.SourceDir = value.(string)
		case "MachineName":
			data.MachineName = value.(string)
		case "MachineID":
			data.MachineID = value.(string)
		case "TestDataDir":
			data.TestDataDir = value.(string)
		}
	}

	// Read the template file
	templatePath := filepath.Join(GetFixturesDir(), "test_configs", templateName+"_config.json.template")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		// Try without .template extension
		templatePath = filepath.Join(GetFixturesDir(), "test_configs", templateName+"_config.json")
		templateContent, err = os.ReadFile(templatePath)
		if err != nil {
			t.Fatalf("Failed to read config template %s: %v", templateName, err)
		}
	}

	// Parse and execute template
	tmpl, err := template.New("config").Parse(string(templateContent))
	if err != nil {
		t.Fatalf("Failed to parse config template %s: %v", templateName, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Failed to execute config template %s: %v", templateName, err)
	}

	// Write to temporary file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, templateName+"_config.json")
	if err := os.WriteFile(configPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to write generated config: %v", err)
	}

	t.Logf("Generated config from template %s: %s", templateName, configPath)
	return configPath
}

// getBasicConfigPath returns a path to a basic configuration, creating it if necessary
func getBasicConfigPath(t *testing.T) string {
	t.Helper()
	return writeConfigFromTemplate(t, "basic", nil)
}

// getWatchingConfigPath returns a path to a watching configuration, creating it if necessary
func getWatchingConfigPath(t *testing.T) string {
	t.Helper()
	return writeConfigFromTemplate(t, "watching", nil)
}

// getCrossPlatformConfigPath returns a path to a cross-platform configuration, creating it if necessary
func getCrossPlatformConfigPath(t *testing.T) string {
	t.Helper()
	return writeConfigFromTemplate(t, "cross_platform", nil)
}

// getConflictConfigPath returns a path to a conflict resolution configuration, creating it if necessary
func getConflictConfigPath(t *testing.T) string {
	t.Helper()
	return writeConfigFromTemplate(t, "conflict", nil)
}

// ensureTestConfigTemplates ensures all required config templates exist
// This can be called in TestMain to set up the testing environment
func ensureTestConfigTemplates() error {
	initPathResolution()

	configDir := filepath.Join(GetFixturesDir(), "test_configs")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// List of required config templates
	requiredConfigs := []string{
		"basic_config.json",
		"watching_config.json",
		"cross_platform_config.json",
		"conflict_config.json",
	}

	for _, configName := range requiredConfigs {
		configPath := filepath.Join(configDir, configName)
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			// Create a minimal config file
			minimalConfig := `{
  "version": "1.0",
  "machine": "test-machine",
  "git": {
    "repo_path": "{{ .TargetDir }}",
    "remote_url": "",
    "branch": "main",
    "auth_type": "none"
  },
  "sync": {
    "auto_sync_enabled": true,
    "debounce_seconds": 1,
    "auto_commit": true,
    "auto_push": true,
    "auto_pull": false
  },
  "mappings": {}
}`

			if err := os.WriteFile(configPath, []byte(minimalConfig), 0644); err != nil {
				return fmt.Errorf("failed to create config file %s: %w", configName, err)
			}
		}
	}

	return nil
}
