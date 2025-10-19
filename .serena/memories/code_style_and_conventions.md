# Code Style & Conventions
- Follow Go best practices plus project-specific rules captured in PRD/CLAUDE guidance; keep implementations context-aware (pass `context.Context` to long-running flows) and maintain debounced, event-driven patterns.
- Configuration loaders must expand user-supplied paths via the shared `expandPath` helpers, return explicit errors on failure, and enforce absolute repo paths; error messages should use "must" for requirements.
- Avoid magic numbers: promote shared constants; prefer helper functions to cut duplication; validation routines should never mutate state.
- Tests: replace `defer` cleanups with `t.Cleanup`, assert every error, and cover advanced debouncer/backoff logic plus git workflows with table-driven cases where helpful.
- Path handling: tilde expansion should trim `~/` properly (see util helpers) and avoid silent fallbacks; ensure tilde-only (`~`) resolves to the home directory.
- Sensitive operations (git auth, file moves) should log or warn per CLAUDE.md guidance and maintain backup/rollback paths as in existing CLI commands.