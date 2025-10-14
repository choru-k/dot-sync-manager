# Agent Workflow Rules

## Purpose
This guide defines how we track work for the Dotfile Sync Manager (DSM) project using GitHub issues, labels, and the `DSM Roadmap` project board.

## Daily Triage
- Check new notifications and confirm any newly created issues are added to the `DSM Roadmap` board.
- Assign the `Phase` field to match the PRD timeline and set `Status` to `Todo` unless work has started.
- Ensure every issue carries the appropriate `phase:` label (e.g., `phase: core sync`).

## Opening New Issues
- Use the naming convention `Phase X: <Short title>` and describe the problem or task clearly.
- Link to PRD sections or other source docs for context.
- Assign the correct phase label and add the issue to the roadmap project, setting `Status: Todo`.
- Update `tasks/development_tasks.md` if the task belongs in the canonical checklist.

## Working an Issue
- Move the item to `Status: In Progress` on the project board when picking it up.
- Create a feature branch named `phase-x/<short-slug>` and reference the issue number in commits (`Fixes #<n>` when appropriate).
- Keep the issue updated with findings, decisions, or blockers so the next agent can continue seamlessly.

## Completing an Issue
- Ensure associated PRs link back to the issue and have passing checks.
- Move the project item to `Status: Done` only after the fix is merged and post-merge validation (tests/build) is complete.
- Close the issue and, if relevant, check off the corresponding entry in `tasks/development_tasks.md`.
- Add any follow-up work as new issues rather than re-opening completed ones.

## Board Hygiene
- Review the board at least twice per week to confirm statuses and assignments are accurate.
- Use the Phase filter to validate progress against the PRD timeline and surface dependencies.
- Keep no more than ten items in `In Progress`; negotiate hand-offs or park items back to `Todo`.

## Code Quality Standards

Before submitting code for review:

### Required Reading
- **CODING_RULES.md** - Quick reference with 18 essential rules
- **.gemini/styleguide.md** - Detailed guide with examples
- **CLAUDE.md** - Project-specific patterns and conventions

### Pre-Commit Checklist
- [ ] All user-provided paths expanded via `expandPaths()` method
- [ ] Path expansion functions return errors (never silently fail)
- [ ] Tilde expansion uses `path[2:]` for `~/` prefix
- [ ] Validation methods only check state (never mutate)
- [ ] Error messages use "must" not "should"
- [ ] Magic numbers extracted to named constants
- [ ] Helper functions reduce code duplication
- [ ] Tests use `t.Cleanup` instead of `defer`
- [ ] All error returns checked in tests

### Review Process
1. Run tests: `go test ./...`
2. Run linter: `golangci-lint run` (if available)
3. Self-review against coding rules checklist
4. Request Gemini review: `/gemini review` in PR comments
5. Address feedback promptly with separate fix commits

### When Receiving Feedback
- Each review issue should be addressed in a separate commit
- Use descriptive commit messages referencing review IDs
- Group related fixes (e.g., all "magic numbers" fixes in one commit)
- Update tests to reflect architectural changes
- Re-run full test suite after all fixes

## Escalations
- Flag scope changes, timeline risk, or missing requirements by creating a new issue labeled `needs clarification` and tagging the product owner.
- Document unresolved questions in the issue and link to the relevant PRD section.
