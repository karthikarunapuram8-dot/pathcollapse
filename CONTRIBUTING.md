# Contributing to PathCollapse

Thank you for helping improve PathCollapse.

## Development Setup

```bash
git clone https://github.com/karthikarunapuram8-dot/pathcollapse
cd pathcollapse
go mod download
go build ./...
go test ./...
```

**Requirements**: Go 1.22+

## Workflow

1. Fork and create a feature branch from `main`.
2. Write tests first (TDD) — see [testing guidelines](#testing).
3. Implement the change; keep functions ≤ 50 lines and files ≤ 800 lines.
4. Ensure `go vet ./...` and `go test -race ./...` pass clean.
5. Open a pull request with a clear title and description.

## Testing

All packages must maintain ≥ 80% test coverage.

```bash
# Run all tests
go test ./...

# Race detector
go test -race ./...

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add YAML ingestion for AWS IAM roles
fix: handle nil node in path traversal
docs: add query language examples
test: cover edge cases in drift detector
```

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf`, `ci`

## Pull Request Checklist

- [ ] `go vet ./...` passes
- [ ] `go test -race ./...` passes
- [ ] New functionality has tests
- [ ] No hardcoded secrets or credentials
- [ ] Functions are focused (≤ 50 lines)

## Reporting Bugs

Open a GitHub Issue with:
- Go version (`go version`)
- OS and architecture
- Minimal reproducer
- Expected vs. actual behaviour

## Security Issues

**Do not open a public issue for security vulnerabilities.**
See [SECURITY.md](SECURITY.md) for responsible disclosure instructions.

## Code of Conduct

Be respectful and constructive. Harassment of any kind is not tolerated.
