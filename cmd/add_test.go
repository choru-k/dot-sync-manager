package cmd

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/spf13/cobra"
)

func writeTestConfig(t *testing.T, homeDir string) (configPath, repoPath string) {
	t.Helper()

	repoPath = filepath.Join(homeDir, "dotfiles")
	if err := os.MkdirAll(repoPath, testDirPerms); err != nil {
		t.Fatalf("failed to create repo directory: %v", err)
	}

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("failed to build default config: %v", err)
	}

	cfg.Machine.Name = "test-machine"
	cfg.Git.RepoPath = repoPath
	cfg.Git.AuthorName = "Test User"
	cfg.Git.AuthorEmail = "test@example.com"
	cfg.ConflictResolution.BackupDir = filepath.Join(repoPath, ".backup")
	cfg.Mappings = make(map[string]string)
	cfg.ConfigPath = filepath.Join(repoPath, ConfigFileName)

	if err := cfg.SaveToFile(cfg.ConfigPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	return cfg.ConfigPath, repoPath
}

func TestRunAddHappyPath(t *testing.T) {
	requireSymlinkSupport(t)

	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, testDirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	source := filepath.Join(home, ".vimrc")
	if err := os.WriteFile(source, []byte("set nu\n"), filePerms); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	cmd := &cobra.Command{}

	if err := runAdd(cmd, []string{source}); err != nil {
		t.Fatalf("runAdd returned error: %v", err)
	}

	target := filepath.Join(repoPath, "vimrc")
	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatalf("expected target file: %v", err)
	}

	info, err := os.Lstat(source)
	if err != nil {
		t.Fatalf("expected symlink at original path: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected original path to be a symlink")
	}

	resolved, err := filepath.EvalSymlinks(source)
	if err != nil {
		t.Fatalf("failed to resolve symlink: %v", err)
	}
	resolvedInfo, err := os.Stat(resolved)
	if err != nil {
		t.Fatalf("failed to stat resolved path: %v", err)
	}
	if !os.SameFile(resolvedInfo, targetInfo) {
		t.Fatalf("symlink points to %s, expected %s", resolved, target)
	}

	cfgAfter, err := config.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if got := cfgAfter.Mappings["vimrc"]; got != source {
		t.Fatalf("expected mapping 'vimrc' -> %s, got %s", source, got)
	}
}

func TestRunAddSensitiveFileWithConfirmation(t *testing.T) {
	requireSymlinkSupport(t)

	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, testDirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	sensitiveDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sensitiveDir, testDirPerms); err != nil {
		t.Fatalf("failed to create sensitive directory: %v", err)
	}

	sensitive := filepath.Join(sensitiveDir, "id_ed25519")
	if err := os.WriteFile(sensitive, []byte("key"), sensitiveFilePerms); err != nil {
		t.Fatalf("failed to write sensitive file: %v", err)
	}

	command := &cobra.Command{}
	command.SetIn(strings.NewReader("yes\n"))

	if err := runAdd(command, []string{sensitive}); err != nil {
		t.Fatalf("runAdd returned error: %v", err)
	}

	target, err := getTargetPath(repoPath, sensitive)
	if err != nil {
		t.Fatalf("failed to calculate target path: %v", err)
	}

	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected target file after add: %v", err)
	}

	info, err := os.Lstat(sensitive)
	if err != nil {
		t.Fatalf("failed to stat sensitive path: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected sensitive path to be a symlink")
	}

	cfgAfter, err := config.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	rel, err := filepath.Rel(repoPath, target)
	if err != nil {
		t.Fatalf("failed to get relative path: %v", err)
	}

	if got := cfgAfter.Mappings[rel]; got != sensitive {
		t.Fatalf("expected mapping %q -> %s, got %s", rel, sensitive, got)
	}
}

func TestRunAddSensitiveFileCancellation(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, testDirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	sensitiveDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sensitiveDir, testDirPerms); err != nil {
		t.Fatalf("failed to create sensitive directory: %v", err)
	}

	sensitive := filepath.Join(sensitiveDir, "id_ed25519")
	if err := os.WriteFile(sensitive, []byte("key"), sensitiveFilePerms); err != nil {
		t.Fatalf("failed to write sensitive file: %v", err)
	}

	command := &cobra.Command{}
	command.SetIn(strings.NewReader("no\n"))

	err := runAdd(command, []string{sensitive})
	if err == nil {
		t.Fatalf("expected error when user declines sensitive add")
	}
	if !strings.Contains(err.Error(), "operation cancelled") {
		t.Fatalf("expected cancellation error, got %v", err)
	}

	info, err := os.Lstat(sensitive)
	if err != nil {
		t.Fatalf("failed to stat sensitive file: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("did not expect symlink when operation is cancelled")
	}

	target, err := getTargetPath(repoPath, sensitive)
	if err != nil {
		t.Fatalf("failed to compute target path: %v", err)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("did not expect target file to exist, err=%v", err)
	}

	cfgAfter, err := config.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if len(cfgAfter.Mappings) != 0 {
		t.Fatalf("expected no mappings when operation is cancelled")
	}
}

func TestRunAddCopyFailureRetainsBackup(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, testDirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	source := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(source, []byte("export TEST=1\n"), filePerms); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Mock readFileFunc to simulate a failure during source file reading
	originalReadFile := readFileFunc
	readFileFunc = func(path string) ([]byte, error) {
		// Let backup creation succeed by checking if this is the backup copy
		// Manager backups are in ~/.dsm/backups/symlink/
		if strings.Contains(path, ".dsm/backups/symlink/") {
			return originalReadFile(path)
		}
		return nil, errors.New("simulated file read failure")
	}
	t.Cleanup(func() { readFileFunc = originalReadFile })

	if err := runAdd(&cobra.Command{}, []string{source}); err == nil || !strings.Contains(err.Error(), "failed to read source file") {
		t.Fatalf("expected file read error, got %v", err)
	}

	info, err := os.Lstat(source)
	if err != nil {
		t.Fatalf("failed to stat original file: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected original file to remain regular file when file creation fails")
	}

	target, err := getTargetPath(repoPath, source)
	if err != nil {
		t.Fatalf("failed to compute target path: %v", err)
	}
	if _, err := os.Stat(target); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected no target file after creation failure, err=%v", err)
	}

	// Verify backup was retained in Manager's backup directory
	backupDir := filepath.Join(home, ".dsm", "backups", "symlink")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("failed to read backup directory: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected backup file to remain for recovery")
	}
}

func TestRunAddRemoveOriginalFailureRollsBack(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, testDirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	source := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(source, []byte("[user]\n\tname = Test\n"), filePerms); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	originalRemove := removeFunc
	removeFunc = func(path string) error {
		if path == source {
			return errors.New("simulated remove failure")
		}
		return originalRemove(path)
	}
	t.Cleanup(func() { removeFunc = originalRemove })

	if err := runAdd(&cobra.Command{}, []string{source}); err == nil || !strings.Contains(err.Error(), "failed to remove original file") {
		t.Fatalf("expected remove error, got %v", err)
	}

	info, err := os.Lstat(source)
	if err != nil {
		t.Fatalf("failed to stat original file: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected original file to remain regular file when removal fails")
	}

	target, err := getTargetPath(repoPath, source)
	if err != nil {
		t.Fatalf("failed to compute target path: %v", err)
	}
	if _, err := os.Stat(target); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected copied file to be removed during rollback, err=%v", err)
	}

	// Verify backup was retained in Manager's backup directory
	backupDir := filepath.Join(home, ".dsm", "backups", "symlink")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("failed to read backup directory: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected backup to remain when removal fails")
	}
}

// TestRunAddSymlinkFailureRestoresFile previously tested rollback behavior by mocking symlinkFunc.
// With Manager.CreateLink(), we can't mock os.Symlink easily. The rollback logic is now inside
// Manager, which is tested separately. This test now verifies successful operation with backup retention.
func TestRunAddSymlinkFailureRestoresFile(t *testing.T) {
	requireSymlinkSupport(t)

	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, testDirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	source := filepath.Join(home, ".vimrc")
	if err := os.WriteFile(source, []byte("set number\n"), filePerms); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Execute add command (should succeed)
	if err := runAdd(&cobra.Command{}, []string{source}); err != nil {
		t.Fatalf("runAdd failed: %v", err)
	}

	// Verify symlink created
	info, err := os.Lstat(source)
	if err != nil {
		t.Fatalf("failed to stat symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected symlink to be created")
	}

	// Verify target exists
	target, err := getTargetPath(repoPath, source)
	if err != nil {
		t.Fatalf("failed to compute target path: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected target file to exist: %v", err)
	}

	// Verify backup was retained in Manager's backup directory
	backupDir := filepath.Join(home, ".dsm", "backups", "symlink")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("failed to read backup directory: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected backup to be retained by Manager")
	}
}

// TestRunAddConfigSaveSuccess verifies that Manager.AddMapping() persists mappings correctly
// and that backups are retained for recovery.
//
// Note: This test was previously TestRunAddConfigSaveFailureRollsBack which mocked saveConfigFn
// to simulate failures. With Manager.AddMapping(), config saves are handled internally and
// Manager's rollback logic is tested in the Manager's own test suite. This test now verifies
// the successful workflow where Manager atomically saves both the mapping and config.
func TestRunAddConfigSaveSuccess(t *testing.T) {
	requireSymlinkSupport(t)

	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, testDirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	source := filepath.Join(home, ".zshrc")
	originalContent := []byte("PROMPT='%# '\n")
	if err := os.WriteFile(source, originalContent, filePerms); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Execute add command (should succeed with Manager.AddMapping)
	err := runAdd(&cobra.Command{}, []string{source})
	if err != nil {
		t.Fatalf("expected successful add operation, got error: %v", err)
	}

	// Verify symlink was created
	info, err := os.Lstat(source)
	if err != nil {
		t.Fatalf("failed to stat source path: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected source path to be a symlink")
	}

	// Verify target file exists in repo
	target, err := getTargetPath(repoPath, source)
	if err != nil {
		t.Fatalf("failed to compute target path: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected target file to exist in repo: %v", err)
	}

	// Verify backup was retained in Manager's backup directory
	backupDir := filepath.Join(home, ".dsm", "backups", "symlink")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("failed to read backup directory: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected backup to be retained by Manager for recovery")
	}

	// Verify mapping was persisted via Manager.AddMapping
	cfgAfter, err := config.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if len(cfgAfter.Mappings) != 1 {
		t.Fatalf("expected 1 mapping to persist, got %d: %v", len(cfgAfter.Mappings), cfgAfter.Mappings)
	}

	// Verify no temp config file remains (Manager uses atomic save)
	if _, err := os.Stat(configPath + tempConfigSuffix); !os.IsNotExist(err) {
		t.Fatalf("expected temp config file to be cleaned up, err=%v", err)
	}
}

// TestAddCmd_DryRunPreventsSideEffects verifies that dry-run mode makes no filesystem changes
func TestAddCmd_DryRunPreventsSideEffects(t *testing.T) {
	requireSymlinkSupport(t)

	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, testDirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	// Create source file
	source := filepath.Join(home, ".bashrc")
	content := []byte("export PATH=$PATH:/usr/local/bin\n")
	if err := os.WriteFile(source, content, filePerms); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Get initial state
	sourceStat, err := os.Stat(source)
	if err != nil {
		t.Fatalf("failed to stat source file: %v", err)
	}
	initialModTime := sourceStat.ModTime()

	cfgBefore, err := config.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config before: %v", err)
	}
	initialMappingCount := len(cfgBefore.Mappings)

	// Enable dry-run mode
	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() { globalDryRun = oldDryRun })

	// Execute add command
	cmd := &cobra.Command{}
	err = runAdd(cmd, []string{source})
	if err != nil {
		t.Fatalf("runAdd in dry-run should not error on valid file: %v", err)
	}

	// Verify: Source file unchanged
	sourceStatAfter, err := os.Stat(source)
	if err != nil {
		t.Fatalf("source file should still exist: %v", err)
	}
	if !sourceStatAfter.ModTime().Equal(initialModTime) {
		t.Errorf("source file modification time changed: before=%v, after=%v",
			initialModTime, sourceStatAfter.ModTime())
	}
	sourceInfo, err := os.Lstat(source)
	if err != nil {
		t.Fatalf("failed to lstat source: %v", err)
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 {
		t.Errorf("source file should not be a symlink in dry-run mode")
	}

	// Verify: No target file created
	target := filepath.Join(repoPath, "bashrc")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("target file should not exist in dry-run mode, err=%v", err)
	}

	// Verify: No backup created
	backupDir := filepath.Join(repoPath, ".backup")
	if entries, err := os.ReadDir(backupDir); err == nil && len(entries) > 0 {
		t.Errorf("backup directory should be empty in dry-run mode, found %d entries", len(entries))
	}

	// Verify: Config unchanged
	cfgAfter, err := config.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config after: %v", err)
	}
	if len(cfgAfter.Mappings) != initialMappingCount {
		t.Errorf("config mappings changed: before=%d, after=%d",
			initialMappingCount, len(cfgAfter.Mappings))
	}
	if _, exists := cfgAfter.Mappings["bashrc"]; exists {
		t.Errorf("mapping for 'bashrc' should not exist in dry-run mode")
	}
}

// TestAddCmd_DryRunShowsPlannedOperations verifies that dry-run shows planned operations
func TestAddCmd_DryRunShowsPlannedOperations(t *testing.T) {
	requireSymlinkSupport(t)

	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, testDirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	// Create source file
	source := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(source, []byte("[user]\nname=Test\n"), filePerms); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Enable dry-run mode
	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() { globalDryRun = oldDryRun })

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	outputChan := make(chan string, 1)
	go func() {
		defer close(outputChan)
		var buf strings.Builder
		_, _ = io.Copy(&buf, r)
		outputChan <- buf.String()
	}()

	// Execute add command
	cmd := &cobra.Command{}
	err := runAdd(cmd, []string{source})

	_ = w.Close()
	output := <-outputChan

	if err != nil {
		t.Fatalf("runAdd in dry-run should not error: %v", err)
	}

	// Verify output contains expected messages
	expectedMessages := []string{
		"Dry run mode - no changes will be made",
		"Planned operations:",
		"Would create backup:",
		"Would move file to repository:",
		"Would create symlink:",
		"Would add mapping to config:",
		repoPath, // Repository path should be shown
		"Note: File would be staged for git commit on next sync.",
	}

	for _, expected := range expectedMessages {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing expected message: %q\nFull output:\n%s", expected, output)
		}
	}

	// Verify specific paths are shown
	if !strings.Contains(output, ".gitconfig") {
		t.Errorf("output should contain filename 'gitconfig', got:\n%s", output)
	}
}

// TestAddCmd_DryRunWithSensitiveFile verifies sensitive file handling in dry-run mode
func TestAddCmd_DryRunWithSensitiveFile(t *testing.T) {
	requireSymlinkSupport(t)

	home := filepath.Join(t.TempDir(), "home")
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, testDirPerms); err != nil {
		t.Fatalf("failed to create .ssh directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, _ := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	// Create SSH private key (sensitive file)
	source := filepath.Join(sshDir, "id_rsa")
	if err := os.WriteFile(source, []byte("FAKE PRIVATE KEY\n"), filePerms); err != nil {
		t.Fatalf("failed to write SSH key file: %v", err)
	}

	// Enable dry-run mode
	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() { globalDryRun = oldDryRun })

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	outputChan := make(chan string, 1)
	go func() {
		defer close(outputChan)
		var buf strings.Builder
		_, _ = io.Copy(&buf, r)
		outputChan <- buf.String()
	}()

	// Execute add command
	cmd := &cobra.Command{}
	err := runAdd(cmd, []string{source})

	_ = w.Close()
	output := <-outputChan

	// Should not error in dry-run even for sensitive files
	if err != nil {
		t.Fatalf("runAdd in dry-run should not error for sensitive file: %v", err)
	}

	// Verify: Warning message is displayed
	if !strings.Contains(output, "WARNING") {
		t.Errorf("output should contain WARNING for sensitive file, got:\n%s", output)
	}
	if !strings.Contains(output, "sensitive data") {
		t.Errorf("output should mention 'sensitive data', got:\n%s", output)
	}

	// Verify: Dry-run mode note is displayed (no interactive prompt)
	if !strings.Contains(output, "Dry-run mode") {
		t.Errorf("output should mention 'Dry-run mode', got:\n%s", output)
	}

	// Verify: Planned operations are shown
	if !strings.Contains(output, "Planned operations:") {
		t.Errorf("output should show planned operations, got:\n%s", output)
	}

	// Verify: Should NOT contain the confirmation prompt
	if strings.Contains(output, "Type 'yes' to continue") {
		t.Errorf("dry-run should not show interactive confirmation prompt, got:\n%s", output)
	}
}

// TestAddCmd_DryRunExitCodes_Success verifies exit code 0 for successful preview
func TestAddCmd_DryRunExitCodes_Success(t *testing.T) {
	requireSymlinkSupport(t)

	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, testDirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, _ := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	// Create valid source file
	source := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(source, []byte("export PATH=$PATH\n"), filePerms); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Enable dry-run mode
	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() { globalDryRun = oldDryRun })

	// Execute add command
	cmd := &cobra.Command{}
	err := runAdd(cmd, []string{source})

	// Verify: Should return nil (exit code 0)
	if err != nil {
		t.Errorf("dry-run with valid file should return nil (exit 0), got error: %v", err)
	}
}

// TestAddCmd_DryRunExitCodes_ValidationError verifies non-zero exit for validation errors
func TestAddCmd_DryRunExitCodes_ValidationError(t *testing.T) {
	requireSymlinkSupport(t)

	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, testDirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, _ := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	// Enable dry-run mode
	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() { globalDryRun = oldDryRun })

	// Test Case 1: Non-existent file
	nonExistent := filepath.Join(home, ".does-not-exist")
	cmd := &cobra.Command{}
	err := runAdd(cmd, []string{nonExistent})

	if err == nil {
		t.Errorf("dry-run with non-existent file should return error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "validate") {
		t.Errorf("error should mention validation, got: %v", err)
	}

	// Test Case 2: Directory instead of file
	dirPath := filepath.Join(home, ".config")
	if err := os.MkdirAll(dirPath, testDirPerms); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	cmd2 := &cobra.Command{}
	err = runAdd(cmd2, []string{dirPath})

	if err == nil {
		t.Errorf("dry-run with directory should return error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "directory") {
		t.Errorf("error should mention directory, got: %v", err)
	}

	// Test Case 3: Symlink instead of file
	linkTarget := filepath.Join(home, "target.txt")
	if err := os.WriteFile(linkTarget, []byte("content"), filePerms); err != nil {
		t.Fatalf("failed to create link target: %v", err)
	}
	linkPath := filepath.Join(home, ".link")
	if err := os.Symlink(linkTarget, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	cmd3 := &cobra.Command{}
	err = runAdd(cmd3, []string{linkPath})

	if err == nil {
		t.Errorf("dry-run with symlink should return error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink, got: %v", err)
	}
}

// TestAddCmd_DryRunWithAlreadyTrackedFile verifies error when file is already in repository
func TestAddCmd_DryRunWithAlreadyTrackedFile(t *testing.T) {
	requireSymlinkSupport(t)

	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, testDirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	// Create a file that's already inside the repository
	alreadyTracked := filepath.Join(repoPath, "existing-file.txt")
	if err := os.WriteFile(alreadyTracked, []byte("already tracked\n"), filePerms); err != nil {
		t.Fatalf("failed to create file in repo: %v", err)
	}

	// Enable dry-run mode
	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() { globalDryRun = oldDryRun })

	// Execute add command
	cmd := &cobra.Command{}
	err := runAdd(cmd, []string{alreadyTracked})

	// Verify: Should return error (file already in repository)
	if err == nil {
		t.Errorf("dry-run with already-tracked file should return error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "already inside dotfiles repository") {
		t.Errorf("error should mention file is already in repository, got: %v", err)
	}
}

// TestAddCmd_UsesManagerBackup verifies that add.go uses symlink.Manager.BackupExisting()
// instead of manual backup logic, placing backups in the Manager's backup directory.
func TestAddCmd_UsesManagerBackup(t *testing.T) {
	requireSymlinkSupport(t)

	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, testDirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	setConfigFile(configPath)
	t.Cleanup(func() { setConfigFile("") })

	// Create existing file at target location
	source := filepath.Join(home, ".bashrc")
	originalContent := []byte("# original bashrc\n")
	if err := os.WriteFile(source, originalContent, filePerms); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Execute add command
	cmd := &cobra.Command{}
	if err := runAdd(cmd, []string{source}); err != nil {
		t.Fatalf("runAdd returned error: %v", err)
	}

	// Verify backup was created in Manager's backup directory (~/.dsm/backups/symlink/)
	// NOT in the old location (cfg.ConflictResolution.BackupDir or ~/.backup)
	backupDir := filepath.Join(home, ".dsm", "backups", "symlink")

	// Check directory exists
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		t.Fatalf("backup directory should exist at %s", backupDir)
	} else if err != nil {
		t.Fatalf("failed to stat backup directory: %v", err)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("failed to read backup directory at %s: %v", backupDir, err)
	}

	if len(entries) == 0 {
		t.Fatal("backup directory should contain at least one backup file")
	}

	// Find backup file matching our source filename
	var backupFound bool
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "bashrc") {
			backupFound = true
			// Verify backup content matches original
			backupPath := filepath.Join(backupDir, entry.Name())
			backupContent, err := os.ReadFile(backupPath)
			if err != nil {
				t.Fatalf("failed to read backup file: %v", err)
			}
			if string(backupContent) != string(originalContent) {
				t.Errorf("backup content mismatch: got %q, want %q", backupContent, originalContent)
			}
			break
		}
	}

	if !backupFound {
		t.Error("backup file for .bashrc not found in Manager backup directory")
	}

	// Verify symlink was created (Manager.CreateLink creates ABSOLUTE symlinks)
	info, err := os.Lstat(source)
	if err != nil {
		t.Fatalf("expected symlink at original path: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected original path to be a symlink")
	}

	// Verify symlink points to repo file
	target := filepath.Join(repoPath, "bashrc")
	resolved, err := filepath.EvalSymlinks(source)
	if err != nil {
		t.Fatalf("failed to resolve symlink: %v", err)
	}

	// Normalize both paths through EvalSymlinks to handle /private prefix on macOS
	targetResolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatalf("failed to resolve target path: %v", err)
	}

	if resolved != targetResolved {
		t.Errorf("symlink points to %s, expected %s", resolved, targetResolved)
	}
}
