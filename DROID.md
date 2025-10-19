# DROID.md

## Purpose and Scope

This document serves as the system prompt for automation droids working on Dotfile Sync Manager (DSM). DSM is a Go application that automatically syncs dotfiles to a git repository using file system watching, enabling seamless multi-machine dotfile management.

## Operating Constraints

- Activate the Serena project context before doing anything else
- Adhere strictly to CODING_RULES.md and .gemini/styleguide.md standards
- Always plan before acting - no direct code execution without detailed analysis
- Act only via approved tools (apply_patch, go commands, git operations)
- Never bypass established workflows or coding standards
- Use absolute paths for all file operations
- Validate all error returns and handle them appropriately

## Task Workflow

1. **Understand Issue Context**
   - Read issue description and linked PRD sections
   - Analyze current codebase state and relevant components
   - Identify dependencies and potential impact areas

2. **Build Detailed Plan**
   - Create comprehensive implementation strategy
   - Define specific, testable components
   - Identify edge cases and error handling requirements
   - Plan testing approach

3. **Decompose Work**
   - Break into small, manageable droid exec commands
   - Each command should complete one logical unit
   - Ensure each change is individually testable

4. **Update Project Tracking**
   - Update GitHub issue status with progress
   - Move items on DSM Roadmap project board
   - Reference issue numbers in commit messages

## Development Standards

### Path Handling
- Expand all user paths via `expandPaths()` method
- Use `path[2:]` for `~/` prefix removal
- Always validate path expansion returns

### Validation Rules
- Never mutate state in `Validate()` methods
- Follow pattern: normalize → validate → use
- Return specific, actionable error messages

### Error Messaging
- Use clear, user-friendly language
- Include context and suggested resolution
- Wrap errors with proper context using `fmt.Errorf`

### Magic Numbers
- Extract all magic numbers to named constants
- Use descriptive constant names
- Group related constants logically

### Testing Requirements
- Run `go test ./...` for all tests
- Execute `golangci-lint run` for linting
- Use `t.Cleanup` not `defer` in tests
- Achieve adequate test coverage for new code
- Use table-driven tests for multiple scenarios

### Code Review Process
- Request `/gemini review` for all PRs
- Address all review feedback before merge
- Ensure all CI checks pass

## Delivery Checklist

### Self-Review
- [ ] Code adheres to all styleguide rules
- [ ] All error paths are handled
- [ ] Tests cover main functionality and edge cases
- [ ] Documentation is updated if needed

### Change Summary
- Provide concise 1-4 sentence summary of changes
- Focus on what was accomplished, not implementation details
- Highlight any breaking changes or new features

### Next Steps
- Suggest follow-up work if applicable
- Identify areas needing further development
- Recommend validation steps for post-merge

### Post-Merge Validation
- Verify application builds and runs correctly
- Confirm new features work as expected
- Check for performance regressions

## Escalation Guidance

### Scope Changes
- Immediately flag requirements that exceed original scope
- Create new issues for additional work
- Update project timeline and board accordingly

### Blocked Approvals
- Detail specific blockers in issue comments
- Propose alternative approaches if needed
- Escalate to project maintainers for resolution

### Critical Issues
- Use `critical` label for blocking problems
- Provide clear reproduction steps
- Suggest workaround if available
