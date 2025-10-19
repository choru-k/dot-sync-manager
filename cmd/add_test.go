package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/spf13/cobra"
)

func requireSymlinkSupport(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src")
	dst := filepath.Join(tempDir, "dst")

	if err := os.WriteFile(src, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := os.Symlink(src, dst); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation not supported: %v", err)
		}
		t.Fatalf("failed to create symlink: %v", err)
	}

	if err := os.Remove(dst); err != nil {
		t.Fatalf("failed to cleanup symlink: %v", err)
	}
}

func writeTestConfig(t *testing.T, homeDir string) (configPath, repoPath string) {
	t.Helper()

	repoPath = filepath.Join(homeDir, "dotfiles")
	if err := os.MkdirAll(repoPath, dirPerms); err != nil {
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
	if err := os.MkdirAll(home, dirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	configFile = configPath
	t.Cleanup(func() { configFile = "" })

	source := filepath.Join(home, ".vimrc")
	if err := os.WriteFile(source, []byte("set nu\n"), 0644); err != nil {
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
	if err := os.MkdirAll(home, dirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	configFile = configPath
	t.Cleanup(func() { configFile = "" })

	sensitiveDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sensitiveDir, dirPerms); err != nil {
		t.Fatalf("failed to create sensitive directory: %v", err)
	}

	sensitive := filepath.Join(sensitiveDir, "id_ed25519")
	if err := os.WriteFile(sensitive, []byte("key"), 0600); err != nil {
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
	if err := os.MkdirAll(home, dirPerms); err != nil {
		t.Fatalf("failed to create home directory: %v", err)
	}
	t.Setenv("HOME", home)

	configPath, repoPath := writeTestConfig(t, home)
	configFile = configPath
	t.Cleanup(func() { configFile = "" })

	sensitiveDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sensitiveDir, dirPerms); err != nil {
		t.Fatalf("failed to create sensitive directory: %v", err)
	}

	sensitive := filepath.Join(sensitiveDir, "id_ed25519")
	if err := os.WriteFile(sensitive, []byte("key"), 0600); err != nil {
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
