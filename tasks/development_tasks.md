# Development Tasks

## Phase 1: Core Sync (Weeks 1-2)
- [x] [Build go-git based commit/push/pull workflow per PRD Feature 1, including auth handling and repo bootstrapping.](https://github.com/choru-k/dot-sync-manager/issues/1)
- [x] [Implement fsnotify watchers for dotfiles repository with debounce logic tuned to PRD latency goals, including the inactivity guard that only triggers auto-commit when the repo has been quiet for the configured period.](https://github.com/choru-k/dot-sync-manager/issues/2) - **Completed** in PR #27 with full fsnotify integration, thread-safe debouncer, and gitignore-style pattern matching.
- [ ] [Deliver configurable debounce/backoff engine covering rapid file churn and manual sync triggers.](https://github.com/choru-k/dot-sync-manager/issues/3)
- [ ] [Create initial `.sync-config.json` loader with validation and defaults described in PRD §5.](https://github.com/choru-k/dot-sync-manager/issues/4)
- [x] [Implement CLI commands (`init`, `add`, `start`, `stop`) matching behaviors in PRD §6.](https://github.com/choru-k/dot-sync-manager/issues/5) - **Completed** in [PR #57](https://github.com/choru-k/dot-sync-manager/pull/57) with full PRD §6 compliance including all required commands: setup (init), file management (add, remove, list), sync control (start, stop, restart, sync, push, pull), conflict management (check, conflicts, resolve), configuration (config, ignore), and utilities (log, open, version).

## Phase 2: Symlink Management (Week 3)
- [ ] [Create symlink manager that provisions and verifies mappings defined in `.sync-config.json`.](https://github.com/choru-k/dot-sync-manager/issues/6)
- [ ] [Build mapping editor to add/update/remove entries with validation and conflict checks.](https://github.com/choru-k/dot-sync-manager/issues/7)
- [ ] [Implement backup workflow for pre-existing files prior to symlink replacement.](https://github.com/choru-k/dot-sync-manager/issues/8)
- [ ] [Extend CLI with add/remove link operations surfaced in PRD §6.](https://github.com/choru-k/dot-sync-manager/issues/9)

## Phase 3: Conflict Handling (Week 4)
- [ ] [Detect merge conflicts using go-git status diffing and rules from PRD Feature 3.](https://github.com/choru-k/dot-sync-manager/issues/10)
- [ ] [Implement automatic backup snapshots for conflicting files before resolution.](https://github.com/choru-k/dot-sync-manager/issues/11)
- [ ] [Send conflict notifications via Wails runtime according to PRD Feature 6.](https://github.com/choru-k/dot-sync-manager/issues/12)
- [ ] [Deliver manual resolution flow integrating conflict UI hooks and CLI fallbacks.](https://github.com/choru-k/dot-sync-manager/issues/13)

## Phase 4: GUI Development (Weeks 5-6)
- [ ] [Implement cross-platform system tray icon and contextual menu with live status.](https://github.com/choru-k/dot-sync-manager/issues/14)
- [ ] [Build status window in Vanilla JS/HTML/CSS delivering sync overview and activity log.](https://github.com/choru-k/dot-sync-manager/issues/15)
- [ ] [Build settings window supporting repository paths, ignore rules, and scheduling toggles.](https://github.com/choru-k/dot-sync-manager/issues/16)
- [ ] [Implement conflict resolution UI matching PRD wireframes and state transitions.](https://github.com/choru-k/dot-sync-manager/issues/17)
- [ ] [Expose Wails bindings between Go services and frontend views (status, settings, conflicts).](https://github.com/choru-k/dot-sync-manager/issues/18)

## Phase 5: Polish & Package (Week 7)
- [ ] [Ship interactive setup wizard covering first-run flow, auth, and initial sync tests.](https://github.com/choru-k/dot-sync-manager/issues/19)
- [ ] [Execute cross-platform QA matrix (macOS, Linux, Windows) documenting issues per PRD §10.](https://github.com/choru-k/dot-sync-manager/issues/20)
- [ ] [Produce installers for macOS (DMG), Linux (AppImage), and Windows (.exe/MSIX) with signing.](https://github.com/choru-k/dot-sync-manager/issues/21)
- [ ] [Document user guide, developer setup, and troubleshooting as specified in PRD §18.](https://github.com/choru-k/dot-sync-manager/issues/22)
- [ ] [Finalize icon assets and tray states aligned with branding guidance.](https://github.com/choru-k/dot-sync-manager/issues/23)

## Phase 6: Beta Testing (Week 8)
- [ ] [Coordinate internal beta program, capture telemetry-lite feedback, and triage issues.](https://github.com/choru-k/dot-sync-manager/issues/24)
- [ ] [Resolve high/medium bugs, stabilize performance, and prep release candidate build.](https://github.com/choru-k/dot-sync-manager/issues/25)
