# Contributing to OniWorks

Thank you for your interest in contributing! Here's how to get started.

## Development Setup

```bash
git clone https://github.com/onipixel/oniworks
cd oniworks
go mod download
go build ./...
go test ./...
```

## Project Structure

- `framework/` — Core framework packages
- `cmd/oni/` — CLI tool
- `stubs/` — Code generation templates
- `examples/` — Example applications
- `docs/` — Documentation

## Making Changes

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Make your changes and add tests
4. Ensure `go test ./...` passes
5. Ensure `go vet ./...` is clean
6. Submit a pull request

## Commit Messages

Use conventional commits:

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `refactor:` Code refactor (no feature/fix)
- `test:` Adding or updating tests
- `chore:` Maintenance (deps, CI, etc.)

## Reporting Issues

Use the GitHub issue tracker. For bugs, include:
- Go version (`go version`)
- OniWorks version
- Minimal reproduction case

## Code Style

- Run `gofmt` before committing
- Follow standard Go conventions
- Document all exported types and functions
