# Changelog

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
- `pathcollapse confidence status` command to show shadow-log counts, progress toward `partial` / `calibrated`, and fit-time metadata for any saved calibrator
- **Shadow-mode calibration harness** (closes [#9](https://github.com/karthikarunapuram8-dot/pathcollapse/issues/9))
  - `--shadow-mode` flag on `breakpoints` appends one JSONL line per recommendation to `~/.pathcollapse/shadow.jsonl`, capturing the full five-factor breakdown and raw aggregated score. Display is hidden behind the legacy `0.85` so analyst decisions aren't biased by unvalidated scores during the collection period.
  - `pathcollapse confidence refit` command reads the shadow log, extracts entries where `observed_collapsed` has been annotated, fits an isotonic-regression calibrator via Pool-Adjacent-Violators, and persists it to `~/.pathcollapse/calibrator.json`. Prints Brier score, Brier baseline (vs constant `0.85`), improvement percentage, Expected Calibration Error (ECE), regime, and per-decile reliability buckets.
  - Subsequent `pathcollapse breakpoints --confidence on` runs auto-load the saved calibrator via `LoadCalibrator`, moving from the identity calibrator (cold-start) to real calibrated final scores.
- `--quiet` flag on `breakpoints` and `report` to suppress informational stderr notes such as built-in-fixture and cold-start confidence messages
- Calibrated recommendation confidence system (`pkg/confidence`) with five-factor breakdowns, isotonic calibration, and snapshot-backed temporal stability
- `--confidence=on|off` flag on `breakpoints` and `report`
- `pkg/snapshot.Presence` helper to index recent snapshots and satisfy the confidence package's temporal-stability lookup needs
- SQLite snapshot persistence (`pathcollapse snapshot save/list/diff/prune`) backed by `modernc.org/sqlite` (pure-Go, no CGO)
- HTML report format: single-file self-contained report with executive summary, top paths, recommended controls, and drift analysis
- `--baseline` flag on `report` subcommand to populate the drift section of HTML reports
- GitHub Actions CI workflow: build, vet, unit tests, race detector, golangci-lint, goreleaser check
- GitHub Actions release workflow: cross-platform binaries (linux/darwin/windows × amd64/arm64) via goreleaser on tag push

### Changed
- Module path now matches the public repository URL (`github.com/karthikarunapuram8-dot/pathcollapse`); `go install ...@main` works immediately, and the first corrected semver tag will be `v0.2.1`
- Breakpoint recommendations now emit calibrated confidence by default instead of the legacy static `0.85`; `--confidence off` preserves the old behavior for A/B comparison
- Markdown, JSON, and HTML reports now include recommendation-confidence context, factor breakdowns, and regime information when confidence scoring is enabled
- README and examples updated to document calibrated confidence, shadow mode, and the current install path
