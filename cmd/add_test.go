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
		if strings.Contains(path, ".backup/") {
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

	backupDir := filepath.Join(repoPath, defaultBackupDirName)
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

	backupDir := filepath.Join(repoPath, defaultBackupDirName)
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("failed to read backup directory: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected backup to remain when removal fails")
	}
}

func TestRunAddSymlinkFailureRestoresFile(t *testing.T) {
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

	originalSymlink := symlinkFunc
	symlinkFunc = func(oldname, newname string) error {
		return errors.New("simulated symlink failure")
	}
	t.Cleanup(func() { symlinkFunc = originalSymlink })

	if err := runAdd(&cobra.Command{}, []string{source}); err == nil || !strings.Contains(err.Error(), "failed to create symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}

	info, err := os.Lstat(source)
	if err != nil {
		t.Fatalf("failed to stat original file: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected original file to be restored when symlink fails")
	}

	target, err := getTargetPath(repoPath, source)
	if err != nil {
		t.Fatalf("failed to compute target path: %v", err)
	}
	if _, err := os.Stat(target); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected target to be removed after rollback, err=%v", err)
	}

	backupDir := filepath.Join(repoPath, defaultBackupDirName)
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("failed to read backup directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected backup to be cleaned up after successful restore, entries=%d", len(entries))
	}
}

func TestRunAddConfigSaveFailureRollsBack(t *testing.T) {
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
	if err := os.WriteFile(source, []byte("PROMPT='%# '\n"), filePerms); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	originalSave := saveConfigFn
	saveConfigFn = func(cfg *config.SyncConfig, path string) error {
		return errors.New("simulated config save failure")
	}
	t.Cleanup(func() { saveConfigFn = originalSave })

	err := runAdd(&cobra.Command{}, []string{source})
	if err == nil || !strings.Contains(err.Error(), "failed to prepare configuration update") {
		t.Fatalf("expected config save error, got %v", err)
	}

	info, err := os.Lstat(source)
	if err != nil {
		t.Fatalf("failed to stat original file: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected original file to be restored when config save fails")
	}

	target, err := getTargetPath(repoPath, source)
	if err != nil {
		t.Fatalf("failed to compute target path: %v", err)
	}
	if _, err := os.Stat(target); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected target to be removed after config failure rollback, err=%v", err)
	}

	backupDir := filepath.Join(repoPath, defaultBackupDirName)
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("failed to read backup directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected backup directory to be empty after rollback, entries=%d", len(entries))
	}

	cfgAfter, err := config.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if len(cfgAfter.Mappings) != 0 {
		t.Fatalf("expected no mappings to persist after config save failure, got %v", cfgAfter.Mappings)
	}

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

// TestCalculateBackupPath verifies backup path calculation is consistent
func TestCalculateBackupPath(t *testing.T) {
	cfg := &config.SyncConfig{
		Git:                config.GitConfig{RepoPath: "/home/user/dotfiles"},
		ConflictResolution: config.ConflictConfig{BackupDir: ""},
	}

	path := calculateBackupPath(cfg, "/home/user/.bashrc")

	// Verify format: .backup/.bashrc-TIMESTAMP
	if !strings.Contains(path, ".backup") {
		t.Errorf("backup path should contain .backup, got: %s", path)
	}
	if !strings.Contains(path, "bashrc") {
		t.Errorf("backup path should contain filename, got: %s", path)
	}
	if !strings.Contains(path, "/home/user/dotfiles") {
		t.Errorf("backup path should be under repo path, got: %s", path)
	}

	// Verify custom backup directory is respected
	cfgCustom := &config.SyncConfig{
		Git:                config.GitConfig{RepoPath: "/home/user/dotfiles"},
		ConflictResolution: config.ConflictConfig{BackupDir: "/custom/backup"},
	}

	customPath := calculateBackupPath(cfgCustom, "/home/user/.vimrc")
	if !strings.Contains(customPath, "/custom/backup") {
		t.Errorf("backup path should use custom backup dir, got: %s", customPath)
	}
	if !strings.Contains(customPath, "vimrc") {
		t.Errorf("backup path should contain filename, got: %s", customPath)
	}
}
