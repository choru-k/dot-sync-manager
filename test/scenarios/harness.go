package scenarios

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ScenarioHarness provides simplified E2E test operations
type ScenarioHarness struct {
	T          *testing.T
	SourceDir  string
	TargetDir  string
	ConfigPath string
	Cleanup    func()
}

// HarnessOption configures ScenarioHarness behavior
type HarnessOption func(*ScenarioHarness)

// NewScenarioHarness creates a test environment with DSM configuration
func NewScenarioHarness(t *testing.T, template string, opts ...HarnessOption) *ScenarioHarness {
	t.Helper()

	testID := RequireTestID(t)
	sourceDir, targetDir := CreateTestEnvironment(t, testID)

	h := &ScenarioHarness{
		T:         t,
		SourceDir: sourceDir,
		TargetDir: targetDir,
	}

	// Apply options
	for _, opt := range opts {
		opt(h)
	}

	// Create config
	h.ConfigPath = writeConfigFromTemplate(t, template, map[string]interface{}{
		"SourceDir": h.SourceDir,
		"TargetDir": h.TargetDir,
	})

	// Setup cleanup
	h.Cleanup = func() {
		os.RemoveAll(h.SourceDir)
		os.RemoveAll(h.TargetDir)
	}

	return h
}

// RunDSM executes DSM commands with the harness configuration
func (h *ScenarioHarness) RunDSM(args ...string) (string, error) {
	h.T.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	allArgs := []string{"--config", h.ConfigPath}
	allArgs = append(allArgs, args...)

	cmd := execCommandContext(ctx, "dsm", allArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// MustAdd files to DSM using the harness configuration
func (h *ScenarioHarness) MustAdd(files ...string) {
	h.T.Helper()
	for _, file := range files {
		addFileToDSMWithConfig(h.T, h.ConfigPath, file)
	}
}

// Sync changes using the harness configuration
func (h *ScenarioHarness) Sync() {
	h.T.Helper()
	syncChangesWithConfig(h.T, h.ConfigPath)
}

// MakeSourceFile creates a test file in the source directory
func (h *ScenarioHarness) MakeSourceFile(name, content string) string {
	h.T.Helper()
	filePath := filepath.Join(h.SourceDir, name)
	require.NoError(h.T, os.WriteFile(filePath, []byte(content), 0644))
	return filePath
}

// RequireEventuallySynced waits for a file to be synced with validation
func (h *ScenarioHarness) RequireEventuallySynced(filename string, validate func(string) bool) {
	h.T.Helper()
	targetFile := filepath.Join(h.TargetDir, filename)

	require.Eventually(h.T, func() bool {
		content, err := os.ReadFile(targetFile)
		if err != nil {
			return false
		}
		return validate(string(content))
	}, 10*time.Second, 200*time.Millisecond, "File should be synced and pass validation")
}

// RequireFileExists checks that a file exists in the target directory
func (h *ScenarioHarness) RequireFileExists(filename string) {
	h.T.Helper()
	targetFile := filepath.Join(h.TargetDir, filename)
	require.FileExists(h.T, targetFile, "Target file should exist: %s", filename)
}

// RequireFileNotExists checks that a file does not exist in the target directory
func (h *ScenarioHarness) RequireFileNotExists(filename string) {
	h.T.Helper()
	targetFile := filepath.Join(h.TargetDir, filename)
	require.NoFileExists(h.T, targetFile, "Target file should not exist: %s", filename)
}

// ReadTargetFile reads the content of a file in the target directory
func (h *ScenarioHarness) ReadTargetFile(filename string) string {
	h.T.Helper()
	targetFile := filepath.Join(h.TargetDir, filename)
	content, err := os.ReadFile(targetFile)
	require.NoError(h.T, err, "Should be able to read target file: %s", filename)
	return string(content)
}
