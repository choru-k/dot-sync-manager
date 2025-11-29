package gitmanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// GitManager manages local repository interactions (commit/push/pull) using go-git.
type GitManager struct {
	cfg  Config
	auth transport.AuthMethod
	repo *git.Repository
}

// NewGitManager boots the repository (clone or open) and prepares authentication.
func NewGitManager(ctx context.Context, cfg Config) (*GitManager, error) {
	// Normalize config (apply defaults) before validation
	cfg.normalize()

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	auth, err := cfg.authMethod()
	if err != nil {
		return nil, err
	}

	manager := &GitManager{
		cfg:  cfg,
		auth: auth,
	}

	if err := manager.bootstrapRepo(ctx); err != nil {
		return nil, err
	}

	return manager, nil
}

// bootstrapRepo ensures the working tree exists and is connected to the configured remote.
func (gm *GitManager) bootstrapRepo(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, err := os.Stat(gm.cfg.RepoPath); errors.Is(err, os.ErrNotExist) {
		if err := util.CreateDirectorySecurely(gm.cfg.RepoPath, 0o755); err != nil {
			return fmt.Errorf("gitmanager: create repo directory: %w", err)
		}

		// Clone from remote if URL provided, otherwise init empty repo
		if gm.cfg.RemoteURL != "" {
			repo, err := git.PlainCloneContext(ctx, gm.cfg.RepoPath, false, &git.CloneOptions{
				URL:               gm.cfg.RemoteURL,
				RemoteName:        gm.cfg.RemoteName,
				Auth:              gm.auth,
				RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
			})
			if err != nil {
				return fmt.Errorf("gitmanager: clone: %w", err)
			}
			gm.repo = repo
		} else {
			// Local-only repo: initialize without remote
			repo, err := git.PlainInit(gm.cfg.RepoPath, false)
			if err != nil {
				return fmt.Errorf("gitmanager: init: %w", err)
			}
			gm.repo = repo
		}
		return nil
	}

	repo, err := git.PlainOpen(gm.cfg.RepoPath)
	if err != nil {
		// repository might be missing; initialize fresh
		if errors.Is(err, git.ErrRepositoryNotExists) {
			repo, initErr := git.PlainInit(gm.cfg.RepoPath, false)
			if initErr != nil {
				return fmt.Errorf("gitmanager: init repo: %w", initErr)
			}
			// Only create remote if URL provided
			if gm.cfg.RemoteURL != "" {
				if _, err := repo.CreateRemote(&config.RemoteConfig{
					Name: gm.cfg.RemoteName,
					URLs: []string{gm.cfg.RemoteURL},
				}); err != nil {
					return fmt.Errorf("gitmanager: configure remote: %w", err)
				}
			}
			gm.repo = repo
			return nil
		}
		return fmt.Errorf("gitmanager: open repo: %w", err)
	}

	gm.repo = repo
	// Only ensure remote if RemoteURL is configured
	if gm.cfg.RemoteURL != "" {
		return gm.ensureRemote()
	}
	return nil
}

// ensureRemote keeps remote configuration aligned with Config.
func (gm *GitManager) ensureRemote() error {
	remote, err := gm.repo.Remote(gm.cfg.RemoteName)
	if err != nil {
		if errors.Is(err, git.ErrRemoteNotFound) {
			if _, err = gm.repo.CreateRemote(&config.RemoteConfig{
				Name: gm.cfg.RemoteName,
				URLs: []string{gm.cfg.RemoteURL},
			}); err != nil {
				return fmt.Errorf("gitmanager: create remote: %w", err)
			}
			return nil
		}
		return fmt.Errorf("gitmanager: fetch remote: %w", err)
	}

	urls := remote.Config().URLs
	if len(urls) == 0 || urls[0] != gm.cfg.RemoteURL {
		if err := gm.repo.DeleteRemote(gm.cfg.RemoteName); err != nil {
			return fmt.Errorf("gitmanager: delete remote: %w", err)
		}
		if _, err := gm.repo.CreateRemote(&config.RemoteConfig{
			Name: gm.cfg.RemoteName,
			URLs: []string{gm.cfg.RemoteURL},
		}); err != nil {
			return fmt.Errorf("gitmanager: recreate remote: %w", err)
		}
	}
	return nil
}

// Repo returns the underlying repository pointer for advanced workflows.
func (gm *GitManager) Repo() *git.Repository {
	return gm.repo
}

// StageCommitAndPush stages all changes, creates an auto-sync commit, and pushes to remote.
// It returns the list of file paths that were included in the commit.
func (gm *GitManager) StageCommitAndPush(ctx context.Context, when time.Time) ([]string, error) {
	changed, err := gm.StageAndCommit(ctx, when)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return nil, nil
	}
	if err := gm.Push(ctx); err != nil {
		return nil, err
	}
	return changed, nil
}

// StageAndCommit stages all changes and creates an auto-sync commit without pushing.
// It returns the list of file paths that were included in the commit.
func (gm *GitManager) StageAndCommit(ctx context.Context, when time.Time) ([]string, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	worktree, err := gm.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("gitmanager: worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("gitmanager: status: %w", err)
	}
	if status.IsClean() {
		return nil, nil
	}

	changedFiles := collectChangedFiles(status)

	if err := worktree.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return nil, fmt.Errorf("gitmanager: add --all: %w", err)
	}

	message := BuildAutoCommitMessage(when, changedFiles)
	_, err = worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  gm.cfg.AuthorName,
			Email: gm.cfg.AuthorEmail,
			When:  when,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("gitmanager: commit: %w", err)
	}

	return changedFiles, nil
}

// Push pushes the current HEAD to the configured remote.
func (gm *GitManager) Push(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	err := gm.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: gm.cfg.RemoteName,
		Auth:       gm.auth,
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("gitmanager: push: %w", err)
	}
	return nil
}

// PullWithStash stashes local modifications, pulls remote changes, then reapplies the stash.
func (gm *GitManager) PullWithStash(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	worktree, err := gm.repo.Worktree()
	if err != nil {
		return fmt.Errorf("gitmanager: worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return fmt.Errorf("gitmanager: status: %w", err)
	}

	var stash *stashSnapshot
	if !status.IsClean() {
		stash, err = gm.createStash(status)
		if err != nil {
			return err
		}
		if err := worktree.Reset(&git.ResetOptions{
			Mode: git.HardReset,
		}); err != nil {
			return fmt.Errorf("gitmanager: reset before pull: %w", err)
		}
	}

	err = worktree.PullContext(ctx, &git.PullOptions{
		RemoteName:   gm.cfg.RemoteName,
		Auth:         gm.auth,
		SingleBranch: true,
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("gitmanager: pull: %w", err)
	}

	if stash != nil {
		if err := gm.applyStash(stash); err != nil {
			return err
		}
	}

	return nil
}

// BuildAutoCommitMessage formats the auto-sync commit message.
func BuildAutoCommitMessage(when time.Time, files []string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Auto-sync: %s\n\nChanged files:\n", when.Format("2006-01-02 15:04:05")))
	for _, file := range files {
		builder.WriteString(fmt.Sprintf("- %s\n", file))
	}
	return builder.String()
}

func collectChangedFiles(status git.Status) []string {
	files := make([]string, 0, len(status))
	for path, fileStatus := range status {
		if fileStatus.Worktree != git.Unmodified || fileStatus.Staging != git.Unmodified {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return files
}
