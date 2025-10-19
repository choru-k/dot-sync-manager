# Useful Commands
- Build: `go build -v -o bin/dotfile-sync-manager .`
- Run CLI: `./bin/dotfile-sync-manager -config ~/.dotfile-sync.json [-verbose]`
- Execute Cobra subcommands (e.g.): `go run . add ~/.zshrc`
- Tests: `go test ./...` (coverage: `go test -v -coverprofile=coverage.out ./...`)
- Static checks: `go vet ./...` (CI runs this); run `golangci-lint run` if the linter is installed.
- Module maintenance: `go mod tidy`, `go mod verify`, `go mod download`.
- Respond to GitHub reviews: `bin/review_report.sh https://github.com/choru-k/dot-sync-manager/pull/<n>`.