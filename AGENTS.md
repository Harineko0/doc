# Repository Guidelines

## Project Structure & Module Organization

This repository contains the Go 1.25 CLI `doc`, which validates and updates links in Git-hosted Markdown documentation.

- `cmd/doc/main.go`: command-line entry point, argument parsing, output, and exit codes.
- `internal/app/`: orchestration for `doc lint` and `doc mv`; its tests exercise repository-level behavior.
- `internal/mdparse/`: Markdown parsing, link rewriting, and GitHub-style heading anchors.
- `internal/gitstore/`: tracked, untracked, and staged-file access through Git.
- `cli_integration_test.go`: builds the binary and verifies commands end to end.
- `README.md`: user-facing behavior and command reference. Keep it synchronized with CLI changes.

## Build, Test, and Development Commands

- `go build -o doc ./cmd/doc` builds a local executable.
- `go run ./cmd/doc lint` runs the CLI against this checkout without keeping a binary.
- `go test ./...` runs unit and CLI integration tests.
- `go test -race ./...` checks the suite for data races.
- `go vet ./...` reports suspicious Go constructs.
- `gofmt -w <files>` formats changed Go files before review.

The tool invokes Git internally, so integration tests require `git` on `PATH`.

## Coding Style & Naming Conventions

Follow standard Go style and let `gofmt` define indentation and layout. Use short, lowercase package names and descriptive mixed-cap identifiers. Export names only when another package needs them, and document non-obvious invariants near the relevant code. Keep command concerns in `cmd/doc`, workflow logic in `internal/app`, parsing in `internal/mdparse`, and Git-specific behavior in `internal/gitstore`.

## Testing Guidelines

Use Go's `testing` package. Name deterministic cases `TestBehavior`, helpers with lowercase verbs, and fuzz targets `FuzzBehavior`. Prefer temporary repositories via `t.TempDir()` and assert both diagnostics and filesystem state. Add regression coverage for malformed Markdown, staged/index behavior, path casing, rollback, and exit-code changes. Run `go test ./...` before every pull request.

## Commit & Pull Request Guidelines

History currently contains only `first commit`, so no established commit convention exists. Use concise imperative subjects such as `Reject links that escape the repository`, and keep each commit focused. Pull requests should explain the user-visible behavior, list validation commands, link relevant issues, and include representative CLI input/output when diagnostics or exit codes change. Update `README.md` whenever commands, guarantees, or requirements change.
