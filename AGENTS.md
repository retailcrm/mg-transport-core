# Repository Guidelines

## Project Structure & Module Organization

Go library (`github.com/retailcrm/mg-transport-core/v2`, Go 1.27) that provides shared building blocks for Message Gateway transports: error reporting, logging, localization, queues, caches, and more.

- `core/` — main package: engine, config, localizer, validator, templates, job manager.
- `core/queue/` — typed queue with `memory`, `beanstalk`, and `nats` backends.
- `core/cache/` — typed cache with in-memory and NATS JetStream KV backends.
- `core/logger/`, `core/middleware/`, `core/db/`, `core/nats/`, `core/healthcheck/`, `core/util/`, `core/stacktrace/` — supporting packages.
- `cmd/transport-core-tool/` — CLI helper that generates transport migrations.

Tests live next to the code as `*_test.go` (e.g. `core/engine_test.go`).

## Build, Test, and Development Commands

```sh
go build ./...          # compile all packages
go test ./...           # run tests
go test -race ./core/... # run tests with race detector
go vet ./...            # static analysis (used in CI)
golangci-lint run       # full lint (config in .golangci.yml)
go install ./cmd/transport-core-tool  # build the migration generator
```

CI (`.github/workflows/`) runs `go vet` on PRs and `gotestsum` with `-race -cover` on Go 1.27 and stable.

## Coding Style & Naming Conventions

- Standard Go formatting (`gofmt`); tabs for indentation.
- Linting via golangci-lint v2 with strict linters: `funlen` (65 lines), `gocyclo`/`gocognit` (25), `lll`, `gosec`, `revive`, `testifylint`, `godot` (end comments with a period).
- Exported identifiers use PascalCase; error messages and comments in English.

## Testing Guidelines

- Framework: `stretchr/testify` (use `require`/`assert`); HTTP mocks via `gock`, SQL via `go-sqlmock`.
- Place tests in the same package as the code under test; name them `TestXxx` in `xxx_test.go`.
- Use table-driven tests where practical; keep race-safe patterns since CI runs with `-race`.
- NATS-backed tests embed `nats-server/v2` for in-process servers.

## Commit & Pull Request Guidelines

History uses Conventional Commits-style prefixes: `feat:`, `fix:`, `chore:` (e.g. `feat: nats queues`, `chore: bump deps & Go version`). Keep subjects short and imperative.

Pull requests must pass CI (vet + tests on two Go versions), include a clear description of the change, and reference related issues when applicable.
