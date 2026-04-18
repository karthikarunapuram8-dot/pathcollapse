# Changelog

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
- SQLite snapshot persistence (`pathcollapse snapshot save/list/diff/prune`) backed by `modernc.org/sqlite` (pure-Go, no CGO)
- HTML report format: single-file self-contained CISO report with executive summary, top paths, recommended controls, and drift analysis
- `--baseline` flag on `report` subcommand to populate the drift section of HTML reports
- `pkg/snapshot` package with normalized table schema, transaction-safe writes, and a full test suite using the enterprise AD fixture
- GitHub Actions CI workflow: build, vet, unit tests, race detector, golangci-lint, goreleaser check
- GitHub Actions release workflow: cross-platform binaries (linux/darwin/windows × amd64/arm64) via goreleaser on tag push
- `.goreleaser.yaml` with changelog grouping, checksum file, and CISO-friendly release header
- `CONTRIBUTING.md` — development setup, TDD guidelines, commit message format, PR checklist
- `SECURITY.md` — supported versions, scope, responsible disclosure contact
- `ROADMAP.md` — shipped items, near-term and medium-term plans
- `CHANGELOG.md` (this file)
- CI and release badges in README
- Snapshot and HTML report documentation in README quick-start section

### Changed
- `reporting.Report` struct gains optional `Drift *drift.DriftReport` field (JSON: `"drift,omitempty"`)
- `reporting.Format` gains `FormatHTML = "html"` constant
- `report` subcommand `--format` now accepts `html` in addition to `markdown` and `json`
- README: features list, quick-start, package table, and status table updated to reflect new capabilities
