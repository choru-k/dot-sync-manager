# Finish-Task Checklist
- Update GitHub issue status on the DSM Roadmap board: move to `In Progress` while working and `Done` only after merge and verification; close the issue and tick `tasks/development_tasks.md` items when applicable.
- Ensure feature branches follow `phase-x/<slug>` naming and commits reference issues (e.g., `Fixes #<n>`); capture findings/decisions in the issue for handoffs.
- Before requesting review or merging, run `go test ./...`, `go vet ./...`, and other relevant checks (coverage, optional `golangci-lint run`); confirm CI parity with `.github/workflows/ci.yml`.
- Link PRs back to their issues, trigger `/gemini review`, and respond to feedback with follow-up commits using descriptive messages.
- After merge, perform post-merge validation (tests/build as needed) and file follow-up issues for remaining work rather than reopening closed ones.