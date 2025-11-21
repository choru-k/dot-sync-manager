# Repository Guidelines

## Project Structure & Module Organization
- CLI entrypoint in `main.go`; subcommands live in `cmd/` (one command per file).
- Core packages under `internal/` (config, gitmanager, sync, process, debouncer, ignore, util); keep cross-package imports minimal.
- Tests: unit/integration alongside code (`*_test.go`, `*_integration_test.go`); E2E scenarios in `test/scenarios/` with helper scripts in `test/scripts/`. Fixtures in `test/fixtures/` and sample data in `test-data/`.
- Design docs and rules: `docs/STYLE_GUIDE.md` (canonical, via `.gemini/styleguide.md`), `docs/git-manager.md` are the key references—read before major changes.

## Build, Test, and Development Commands
- `make test-unit` – Go unit tests (fast, default).  
- `make test-integration` – Unit + integration tests (medium).  
- `make test-all` or `./test/scripts/run-e2e.sh -s all` – Full E2E suite.  
- `make test-quick` – Targeted fast loop for `internal/...`.  
- `make lint` – `golangci-lint run ./...`.  
- `make build` – Build CLI binary to `bin/dsm`.  
- `make verify` – lint + unit + integration + build.  
- `make deps` / `make setup-dev` – install Go deps and tooling (golangci-lint, script perms).

## Coding Style & Naming Conventions
- Go code must be `gofmt` + `golangci-lint` clean; prefer tabs (Go default).
- Path handling: expand user/tilde paths early, return errors on expansion, keep validation pure (no mutation), require absolute paths.
- Error messages use “must …” and wrap with context (`fmt.Errorf("op failed: %w", err)`); use raw string literals for multi-line text.
- Avoid duplicating config defaults—start from `config.DefaultConfig()`. Keep file permission constants named (0600/0644).
- Exported items need Godoc; comments explain “why”. Prefer table-driven tests and helpers over duplication.

## Testing Guidelines
- Framework: `go test`. Layers: unit (`*_test.go`), integration (`*_integration_test.go`), E2E (`test/scenarios/*.go`).
- Boundaries: unit tests avoid real FS/network/git; integration can touch real FS/git but not full workflows; E2E exercises CLI flows with real editors/SSH as needed.
- Targets (see STYLE_GUIDE): unit 85% coverage goal, integration 70%, E2E scenario coverage of critical flows. Keep unit tests <100ms; use `t.Cleanup` for teardown.
- Naming: `TestPackage_Feature_WhenCondition`. Use maps over loops for validation in tests to match production patterns.

## Commit & Pull Request Guidelines
- Commit style: imperative summaries with clear scope; recent history uses “Phase X: <title> (#NN)”—stay consistent and include issue/PR refs (`Fixes #123` when applicable).
- Branches: `phase-x/<short-slug>` (per style guide).
- Before pushing: run `make lint` and at least `make test-unit`; note what you ran in the PR description.
- PRs should include: concise summary, rationale, linked issues, test evidence (commands + results), and mention of user-facing changes or CLI flags. Add screenshots only when UI differ; otherwise short output snippets are enough.

## Workflow: Plan-First TDD
- Always check `plan.md` first; follow the next unchecked item in order. If missing, create it with a minimal checklist before coding.
- Cycle: Red → Green → Refactor. One test at a time: write failing test, implement minimal code, run `go test ./...` or the narrowest `go test ./internal/<pkg>`; refactor only with green tests.
- Separate structural vs behavioral changes (“Tidy First”): do renames/extractions with no behavior change, then new behavior/tests; avoid mixing.
- Keep tests fast: prefer unit scope; skip long E2E unless required. Document skipped long runs in PR notes.
- After each checklist item: update `plan.md` (mark done), rerun relevant tests, then proceed to the next item.
- For the full TDD/Tidy discipline playbook, see `TDD_WORKFLOW.md`.

## Security & Configuration Tips
- Never overwrite user config or dotfiles without confirmation; check existence before writing.
- Respect `--config` / discovered paths (`cfg.GetConfigPath()`), keep operations absolute, and avoid shelling out for git (use internal gitmanager).
- Keep logs free of secrets (SSH paths, tokens); prefer contextual errors over raw outputs.

## Guiding Principles
- Make it first, enhance later.
- Simple is always best.
- Do not over-engineer: when solutions are equivalent in behavior, choose the simpler one.
