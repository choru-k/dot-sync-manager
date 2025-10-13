package gitmanager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestNewGitManager_CloneBootstrap(t *testing.T) {
	ctx := context.Background()
	remotePath, seedCommit := setupRemote(t)

	repoPath := filepath.Join(t.TempDir(), "dotfiles")

	cfg := Config{
		RepoPath:    repoPath,
		RemoteURL:   remotePath,
		RemoteName:  "origin",
		AuthorName:  "Dotfile Bot",
		AuthorEmail: "bot@example.com",
		AuthType:    AuthStrategyNone,
	}

	manager, err := NewGitManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewGitManager error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		t.Fatalf("expected .git directory: %v", err)
	}

	head, err := manager.repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}

	if head.Hash() != seedCommit {
		t.Fatalf("unexpected head hash: got %s want %s", head.Hash(), seedCommit)
	}
}

func TestStageCommitAndPush(t *testing.T) {
	ctx := context.Background()
	remotePath, _ := setupRemote(t)

	repoPath := filepath.Join(t.TempDir(), "dotfiles")
	manager := mustManager(t, ctx, Config{
		RepoPath:    repoPath,
		RemoteURL:   remotePath,
		AuthorName:  "Dotfile Bot",
		AuthorEmail: "bot@example.com",
		AuthType:    AuthStrategyNone,
	})

	filePath := filepath.Join(repoPath, "bashrc")
	if err := os.WriteFile(filePath, []byte("export PATH=$PATH:/custom\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	changed, err := manager.StageCommitAndPush(ctx, time.Unix(1_735_000_000, 0))
	if err != nil {
		t.Fatalf("StageCommitAndPush: %v", err)
	}

	if len(changed) != 1 || changed[0] != "bashrc" {
		t.Fatalf("unexpected changed files: %v", changed)
	}

	remoteRepo := mustOpenRepo(t, remotePath)
	ref, err := remoteRepo.Head()
	if err != nil {
		t.Fatalf("remote head: %v", err)
	}

	commit, err := remoteRepo.CommitObject(ref.Hash())
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	if got, want := commit.Author.Name, "Dotfile Bot"; got != want {
		t.Fatalf("commit author mismatch: got %s, want %s", got, want)
	}

	if got := commit.Message; got == "" || got[:9] != "Auto-sync" {
		t.Fatalf("unexpected commit message: %q", got)
	}
}

func TestPullWithStash(t *testing.T) {
	ctx := context.Background()
	remotePath, _ := setupRemote(t)

	tmp := t.TempDir()

	managerA := mustManager(t, ctx, Config{
		RepoPath:    filepath.Join(tmp, "repoA"),
		RemoteURL:   remotePath,
		AuthorName:  "Bot A",
		AuthorEmail: "a@example.com",
		AuthType:    AuthStrategyNone,
	})

	managerB := mustManager(t, ctx, Config{
		RepoPath:    filepath.Join(tmp, "repoB"),
		RemoteURL:   remotePath,
		AuthorName:  "Bot B",
		AuthorEmail: "b@example.com",
		AuthType:    AuthStrategyNone,
	})

	// Manager A adds file1 and pushes.
	fileA := filepath.Join(managerA.cfg.RepoPath, "fileA.txt")
	if err := os.WriteFile(fileA, []byte("A1\n"), 0o644); err != nil {
		t.Fatalf("write fileA: %v", err)
	}
	if _, err := managerA.StageCommitAndPush(ctx, time.Now()); err != nil {
		t.Fatalf("managerA commit: %v", err)
	}

	// Manager B pulls to get fileA.
	if err := managerB.PullWithStash(ctx); err != nil {
		t.Fatalf("managerB initial pull: %v", err)
	}

	// Manager B modifies fileB locally without committing.
	fileB := filepath.Join(managerB.cfg.RepoPath, "fileB.txt")
	if err := os.WriteFile(fileB, []byte("B local change\n"), 0o644); err != nil {
		t.Fatalf("write fileB: %v", err)
	}

	// Manager A modifies fileC and pushes.
	fileC := filepath.Join(managerA.cfg.RepoPath, "fileC.txt")
	if err := os.WriteFile(fileC, []byte("C remote change\n"), 0o644); err != nil {
		t.Fatalf("write fileC: %v", err)
	}
	if _, err := managerA.StageCommitAndPush(ctx, time.Now()); err != nil {
		t.Fatalf("managerA second commit: %v", err)
	}

	// Manager B pulls with stash; expect local fileB content preserved and fileC present.
	if err := managerB.PullWithStash(ctx); err != nil {
		t.Fatalf("managerB pull with stash: %v", err)
	}

	bContent, err := os.ReadFile(fileB)
	if err != nil {
		t.Fatalf("read fileB: %v", err)
	}
	if string(bContent) != "B local change\n" {
		t.Fatalf("stash not restored, got: %q", string(bContent))
	}

	if _, err := os.Stat(fileC); err != nil {
		t.Fatalf("expected fileC from remote: %v", err)
	}
}

func TestPullWithStashConflict(t *testing.T) {
	ctx := context.Background()
	remotePath, _ := setupRemote(t)

	tmp := t.TempDir()

	managerA := mustManager(t, ctx, Config{
		RepoPath:    filepath.Join(tmp, "repoA"),
		RemoteURL:   remotePath,
		AuthorName:  "Bot A",
		AuthorEmail: "a@example.com",
		AuthType:    AuthStrategyNone,
	})

	managerB := mustManager(t, ctx, Config{
		RepoPath:    filepath.Join(tmp, "repoB"),
		RemoteURL:   remotePath,
		AuthorName:  "Bot B",
		AuthorEmail: "b@example.com",
		AuthType:    AuthStrategyNone,
	})

	fileA := filepath.Join(managerA.cfg.RepoPath, "README.md")

	// B pulls latest and modifies fileA locally (uncommitted).
	if err := managerB.PullWithStash(ctx); err != nil {
		t.Fatalf("managerB initial pull: %v", err)
	}

	if err := os.WriteFile(filepath.Join(managerB.cfg.RepoPath, "README.md"), []byte("local edit\n"), 0o644); err != nil {
		t.Fatalf("managerB local edit: %v", err)
	}

	// A modifies the same file and pushes.
	if err := os.WriteFile(fileA, []byte("remote edit\n"), 0o644); err != nil {
		t.Fatalf("managerA remote edit: %v", err)
	}
	if _, err := managerA.StageCommitAndPush(ctx, time.Now()); err != nil {
		t.Fatalf("managerA remote commit: %v", err)
	}

	err := managerB.PullWithStash(ctx)
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected conflict error, got: %v", err)
	}

	if len(conflictErr.Files) != 1 || conflictErr.Files[0] != "README.md" {
		t.Fatalf("unexpected conflict files: %+v", conflictErr.Files)
	}
	if conflictErr.ConflictDir == "" {
		t.Fatalf("expected conflict dir to be populated")
	}

	remoteContent, err := os.ReadFile(filepath.Join(managerB.cfg.RepoPath, "README.md"))
	if err != nil {
		t.Fatalf("read remote file: %v", err)
	}
	if string(remoteContent) != "remote edit\n" {
		t.Fatalf("working tree should retain remote version, got: %q", remoteContent)
	}

	localConflictPath := filepath.Join(conflictErr.ConflictDir, "README.md.local")
	data, err := os.ReadFile(localConflictPath)
	if err != nil {
		t.Fatalf("read local conflict artifact: %v", err)
	}
	if string(data) != "local edit\n" {
		t.Fatalf("expected local conflict artifact to contain local edit, got: %q", data)
	}
}

func setupRemote(t *testing.T) (string, plumbing.Hash) {
	t.Helper()
	remotePath := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remotePath, 0o755); err != nil {
		t.Fatalf("mkdir remote: %v", err)
	}

	remoteRepo, err := git.PlainInit(remotePath, true)
	if err != nil {
		t.Fatalf("plain init remote: %v", err)
	}

	workingDir := t.TempDir()
	repo, err := git.PlainInit(workingDir, false)
	if err != nil {
		t.Fatalf("plain init working: %v", err)
	}

	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{remotePath},
	}); err != nil {
		t.Fatalf("create remote: %v", err)
	}

	filePath := filepath.Join(workingDir, "README.md")
	if err := os.WriteFile(filePath, []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	if _, err := worktree.Add("README.md"); err != nil {
		t.Fatalf("add: %v", err)
	}

	commit, err := worktree.Commit("seed commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Seeder",
			Email: "seed@example.com",
			When:  time.Unix(1_600_000_000, 0),
		},
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	_ = commit

	if err := repo.Push(&git.PushOptions{RemoteName: "origin"}); err != nil {
		t.Fatalf("push: %v", err)
	}

	if runtime.GOOS == "windows" {
		remotePath = filepath.ToSlash(remotePath)
	}

	// Ensure bare repo has HEAD pointing to master/main
	headRef, err := remoteRepo.Head()
	if err != nil {
		t.Fatalf("remote head: %v", err)
	}

	return remotePath, headRef.Hash()
}

func mustManager(t *testing.T, ctx context.Context, cfg Config) *GitManager {
	t.Helper()
	manager, err := NewGitManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewGitManager: %v", err)
	}
	return manager
}

func mustOpenRepo(t *testing.T, path string) *git.Repository {
	t.Helper()
	repo, err := git.PlainOpen(path)
	if err != nil {
		t.Fatalf("PlainOpen: %v", err)
	}
	return repo
}
