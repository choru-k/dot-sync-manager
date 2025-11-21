# Dotfile Sync Manager – Unified Style Guide

This is the single source of truth for coding, testing, and review standards. It merges the former `.gemini/styleguide.md`, `CODING_RULES.md`, and `TEST_ARCHITECTURE_BOOK.md`.

## Core Principles
- Prefer simplicity; avoid over-engineering. Make it work, then refine.
- Separation of concerns: normalize → validate → act. Validation is pure (no mutation).
- Context everywhere: long-running functions accept `context.Context` and respect cancellation.
- Paths are absolute and expanded early (tilde, relative); never ignore expansion errors.
- Cross‑platform: keep Windows/macOS/Linux in mind (paths, permissions, symlinks, execs).

## Architecture & Code Style
- Use Go idioms; code must be `gofmt`/`golangci-lint` clean.
- Keep functions focused; eliminate duplication; name for intent.
- Favor helpers over repeated blocks; use maps for O(1) lookups instead of linear scans.
- Error handling: wrap with context (`fmt.Errorf("op failed: %w", err)`); messages use “must”, not “should”.
- Path handling: resolve symlinks relative to their dir; expand `~` via `util.ExpandPath`; keep repo paths absolute.
- Avoid shelling out when libraries exist (use go-git, fsnotify, etc.).
- Comments explain “why”; Godoc on exported items; raw string literals for multi-line text.
- Security: never overwrite user files without confirmation; keep secrets out of logs; check `known_hosts` for SSH.

## Test Strategy (3 Layers)
- **Unit (`*_test.go`)**: pure logic, no real FS/network/git; fast (<100 ms). Run: `make test-unit`.
- **Integration (`*_integration_test.go`)**: real FS/git, component interactions; medium (1–5 s). Run: `make test-integration`.
- **E2E (`test/scenarios/*.go`)**: full CLI workflows with real editors/SSH; slower. Run: `make test-all` or `./test/scripts/run-e2e.sh -s all`.
- Use table-driven tests; `t.Cleanup` for teardown; check every error return.
- Coverage focus: git ops, process management, CLI commands. Keep debounce/backoff behavior covered.

## Debounce/Backoff Essentials
- Debounce default 30s; advanced debouncer supports backoff, churn window, decay reset, manual sync timeout.
- Backoff params must be validated: max delay ≥ debounce; multiplier > 1.0; thresholds/window/decay > 0; manual sync timeout ≥ 0.

## Git & Sync Practices
- Use go-git; stash-on-pull to preserve dirty state; report conflicts via `.dsm/conflicts/<timestamp>/`.
- Auto-commit message includes timestamp and changed files list.
- Ignore handling via `.syncignore`; exclude `.git` from watchers; watch recursively.

## Commit & PR Guidelines
- Keep commits small, single-purpose; state whether change is structural or behavioral.
- Reference issues (`Fixes #123`); branch naming `phase-x/<slug>` when relevant.
- Before pushing: `make lint`, `make test-unit`; run broader suites when touching integrations.
- PRs: summary, rationale, linked issues, test commands/results; add screenshots only for UI changes.

## Quick Checklists
- [ ] Paths expanded and absolute; no ignored errors.
- [ ] Validation is pure; no hidden mutations.
- [ ] Magic numbers/constants named (incl. file perms).
- [ ] Errors wrapped with context; optional fields guarded.
- [ ] Tests table-driven; use `t.Cleanup`; no unhandled errors.
- [ ] No sensitive data in logs; destructive ops require confirmation.
