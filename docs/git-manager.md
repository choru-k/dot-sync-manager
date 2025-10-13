# Git Manager Overview

The `internal/gitmanager` package wraps go-git to provide repository bootstrap, commit, push, and pull behaviour required by the DSM sync service.

## Construction

```go
cfg := gitmanager.Config{
    RepoPath:    "/Users/me/dotfiles",
    RemoteURL:   "git@github.com:choru-k/dotfiles.git",
    AuthorName:  "Dotfile Sync",
    AuthorEmail: "dotfile-sync@example.com",
    AuthType:    gitmanager.AuthStrategySSH,
    SSHKeyPath:  "~/.ssh/id_ed25519",
}
manager, err := gitmanager.NewGitManager(ctx, cfg)
```

`NewGitManager` will clone the remote if `RepoPath` is missing, or initialise a new repository and attach the configured remote.

## Auto Commit Pipeline

- `StageCommitAndPush(ctx, when)` gathers the working tree status, stages all changes, generates the auto-sync message (`Auto-sync: YYYY-MM-DD HH:MM:SS` + changed file listing), commits, and pushes.
- The helper returns the list of committed file paths so higher layers can emit notifications.

## Pull Pipeline

- `PullWithStash(ctx)` captures any dirty worktree state (including untracked files) into a temporary stash, performs a pull from the configured remote, and reapplies the stash content. This ensures the PRD requirement of preserving user edits during background pulls.
- Stashed data is kept under the OS temp directory and cleaned after successful application.
- If remote commits touch the same paths as the stashed changes, the manager preserves the remote version, writes conflict artifacts under `.dsm/conflicts/<timestamp>/`, and returns a `ConflictError` for the caller to surface to the user.

## Authentication Notes

- SSH keys: set `AuthType` to `AuthStrategySSH` and supply `SSHKeyPath` (optionally `SSHKeyPassphrase` and `KnownHostsPath`).
- HTTPS/PAT: set `AuthType` to `AuthStrategyHTTPS` with `Username` and `Password` (token).
- Public repositories or local remotes can use `AuthStrategyNone`.

## Next Steps

Integrate the manager into the upcoming sync service by wiring it into the debounce timer and file watcher once those layers land. The tests in `internal/gitmanager/git_manager_test.go` demonstrate the expected behaviour for bootstrap, commit/push, and pull-with-stash flows.
