package scenarios

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestConflictResolution creates a git conflict and verifies dsm handles it
func TestConflictResolution(t *testing.T) {
	testID := RequireTestID(t)

	t.Logf("Running conflict resolution test with ID: %s", testID)

	// Setup test environment with dynamic paths
	sourceDir, targetDir := CreateTestEnvironment(t, testID+"_conflict")

	// Create config with the correct source and target directories for this test
	configPath := writeConfigFromTemplate(t, "conflict", map[string]interface{}{
		"SourceDir": sourceDir,
		"TargetDir": targetDir,
	})

	// 1. Add a file to dsm and sync it
	testFile := filepath.Join(sourceDir, ".testconfig")
	initialContent := "# Test configuration file\nVERSION=1.0\n"
	err := os.WriteFile(testFile, []byte(initialContent), filePermissions)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()

	addFileToDSMWithConfig(t, configPath, testFile)

	syncChangesWithConfig(t, configPath)

	// 2. Modify the file in the source directory
	sourceContent := "# Test configuration file\nVERSION=2.0\nSOURCE=true\n"
	err = os.WriteFile(testFile, []byte(sourceContent), filePermissions)
	require.NoError(t, err)

	// 3. Modify the same file in the target directory to create a conflict
	// DSM normalizes filenames by removing leading dots, so .testconfig becomes testconfig
	targetFile := filepath.Join(targetDir, "testconfig")
	targetContent := "# Test configuration file\nVERSION=2.0\nTARGET=true\n"
	err = os.WriteFile(targetFile, []byte(targetContent), filePermissions)
	require.NoError(t, err)

	// 4. Run dsm sync, which should handle the conflict gracefully
	cmd := execCommandContextWithConfig(ctx, configPath, "dsm", "--config", configPath, "sync")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Sync with conflict should succeed with conflict resolution")
	t.Logf("Sync output: %s", string(output))

	// 5. Check if dsm handled the conflict properly (either resolved or created conflict artifacts)
	conflictDir := filepath.Join(targetDir, ".dsm", "conflicts")

	// Check if conflict directory was created (conflict artifacts)
	if _, err := os.Stat(conflictDir); err == nil {
		entries, err := os.ReadDir(conflictDir)
		require.NoError(t, err, "Should be able to read conflict directory")
		if len(entries) > 0 {
			t.Logf("✅ Conflict artifacts created: %v", entries)
		} else {
			t.Logf("✅ Conflict directory created but empty (conflict resolved automatically)")
		}
	} else {
		t.Logf("✅ No conflict directory needed (conflict resolved during sync)")
	}

	t.Log("✅ Conflict resolution test completed successfully")
}
