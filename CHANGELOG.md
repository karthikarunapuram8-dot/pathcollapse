# Changelog

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
- Calibrated recommendation confidence system (`pkg/confidence`) with five-factor breakdowns, isotonic calibration, and snapshot-backed temporal stability
- `--confidence=on|off` flag on `breakpoints` and `report`
- `pkg/snapshot.Presence` helper to index recent snapshots and satisfy the confidence package's temporal-stability lookup needs
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
- Breakpoint recommendations now emit calibrated confidence by default instead of the legacy static `0.85`; `--confidence off` preserves the old behavior for A/B comparison
- Markdown, JSON, and HTML reports now include recommendation-confidence context, factor breakdowns, and regime information when confidence scoring is enabled
- README quick-start and examples updated to document calibrated confidence, the BloodHound positioning, and the current experimental package boundaries
- `reporting.Report` struct gains optional `Drift *drift.DriftReport` field (JSON: `"drift,omitempty"`)
- `reporting.Format` gains `FormatHTML = "html"` constant
- `report` subcommand `--format` now accepts `html` in addition to `markdown` and `json`
- README: features list, quick-start, package table, and status table updated to reflect new capabilities

### Removed
- `pkg/providers` — the unused LLM provider abstraction stub. Zero callers, no roadmap item required it. Reintroducing the shape later is cheap if AI-assisted analysis is prioritized; keeping dead code on the public surface is not.
