# PathCollapse

[![CI](https://github.com/karunapuram/pathcollapse/actions/workflows/ci.yml/badge.svg)](https://github.com/karunapuram/pathcollapse/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/karunapuram/pathcollapse)](https://goreportcard.com/report/github.com/karunapuram/pathcollapse)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**Graph-based identity exposure analysis that finds the smallest fixes with the biggest security impact.**

PathCollapse ingests identity and policy metadata from enterprise environments, constructs a typed privilege/exposure graph, reasons over realistic escalation and blast-radius paths, then computes the **minimal control changes** that collapse the highest-risk paths.

> **Purely defensive.** PathCollapse is an analytics and reporting tool. It performs no network scanning, credential access, or attack execution.

---

## Features

- **Typed identity graph** — 15 node types, 17 edge types covering AD/Entra ID relationships
- **Multi-mode path reasoning** — Reachability, Plausibility (condition-filtered), and full Defensive analysis
- **Greedy breakpoint optimizer** — set-cover algorithm finds the fewest changes with the most impact
- **Detection mapper** — generates Sigma rules, KQL queries, SPL queries, and ATT&CK mappings per path
- **Drift detection** — snapshot diffing highlights new privileged memberships, delegation changes, cert template drift
- **SQLite snapshot persistence** — save, list, diff, and prune graph snapshots with `pathcollapse snapshot`
- **HTML reports** — single-file CISO-ready reports with executive summary, risk paths, breakpoints, and drift
- **Analyst DSL** — human-readable query language: `FIND PATHS FROM user:alice TO privilege:tier0 WHERE confidence > 0.7`
- **Multiple ingestion formats** — Generic JSON, CSV (users/groups/admins/GPOs), BloodHound JSON exports, YAML facts

---

## Installation

### Pre-built binaries (recommended)

Download the latest release for your platform from [GitHub Releases](https://github.com/karunapuram/pathcollapse/releases):

```bash
# macOS / Linux — extract and move to PATH
tar xzf pathcollapse_*_darwin_arm64.tar.gz
mv pathcollapse /usr/local/bin/
```

### Build from source

```bash
git clone https://github.com/karunapuram/pathcollapse
cd pathcollapse
go build ./cmd/pathcollapse
```

Or install all binaries:

```bash
go install ./cmd/...
```

**Requirements**: Go 1.22+

### Performance

See [BENCHMARKS.md](BENCHMARKS.md) for latency and memory numbers.

---

## Quick Start

### Ingest from JSON

```bash
pathcollapse ingest --input identity-data.json --type json --output snapshot.json
```

### Analyse paths

```bash
# Find all paths from alice to any tier-0 asset (uses ingested snapshot)
pathcollapse analyze --graph snapshot.json \
  --query "FIND PATHS FROM user:alice TO privilege:tier0 LIMIT 10"

# Find the minimal breakpoints to collapse top paths
pathcollapse analyze --graph snapshot.json \
  --query "FIND BREAKPOINTS FOR top_paths LIMIT 5"

# Show drift relative to a prior snapshot
pathcollapse analyze --graph snapshot-feb.json --baseline snapshot-jan.json \
  --query "SHOW DRIFT"
```

### Generate a report

```bash
pathcollapse report --graph snapshot.json --format markdown --top 20
pathcollapse report --graph snapshot.json --format json --output report.json

# HTML report for CISO review (with optional drift section)
pathcollapse report --graph snapshot.json --format html --output report.html
pathcollapse report --graph snapshot.json --baseline baseline.json --format html --output drift-report.html
```

### Diff two snapshots

```bash
# File-based diff
pathcollapse diff snapshot-jan.json snapshot-feb.json
```

### Snapshot persistence (SQLite)

```bash
# Save a named snapshot to ~/.pathcollapse/snapshots.db
pathcollapse snapshot save --name weekly-jan --graph snapshot.json

# List all stored snapshots
pathcollapse snapshot list

# Compare two stored snapshots by ID (uses drift engine)
pathcollapse snapshot diff 1 3

# Prune old snapshots, keeping at least 5 recent ones
pathcollapse snapshot prune --older-than 90 --keep-min 5
```

### Compute breakpoints directly

```bash
pathcollapse breakpoints --graph snapshot.json --top 10
```

---

## Architecture Overview

```
Ingestion (pkg/ingest)
    │  JSON / CSV / BloodHound / YAML
    ▼
Normalization (pkg/normalize)
    │  dedup, canonicalize
    ▼
Graph Engine (pkg/graph)
    │  adjacency list, 100K nodes / 1M edges target
    ├──► Scoring (pkg/scoring)
    │        RiskScore formula, configurable weights
    ├──► Reasoning (pkg/reasoning)
    │        Reachability / Plausibility / Defensive modes
    ├──► Controls (pkg/controls)
    │        Greedy set-cover breakpoint optimizer  ← KILLER FEATURE
    ├──► Detection (pkg/detection)
    │        Sigma / KQL / SPL generation, ATT&CK mapping
    ├──► Drift (pkg/drift)
    │        Snapshot comparison, blast-radius delta
    └──► Reporting (pkg/reporting)
             Executive + engineer reports, Markdown / JSON
```

---

## Supported Ingestion Formats

| Adapter | Flag | Description |
|---------|------|-------------|
| Generic JSON | `--type json` | PathCollapse native format |
| CSV Users | `--type csv_users` | `id,name,type,tags` |
| CSV Groups | `--type csv_groups` | `member_id,group_id` |
| CSV Local Admin | `--type csv_local_admin` | `user_id,computer_id,confidence` |
| CSV GPO | `--type csv_gpo` | `gpo_id,ou_id,gpo_name` |
| BloodHound JSON | `--type bloodhound` | Read-only parser for BH collector exports |
| YAML Facts | `--type yaml` | Analyst-provided manual relationships |

---

## DSL Query Language

```
# Find lateral movement paths
FIND PATHS FROM user:alice TO privilege:tier0 WHERE confidence > 0.7 ORDER BY risk DESC LIMIT 10

# Find optimal control breakpoints
FIND BREAKPOINTS FOR top_paths LIMIT 5

# Show drift since last snapshot
SHOW DRIFT SINCE last_snapshot

# Find high-risk service accounts
FIND HIGH_RISK_SERVICE_ACCOUNTS
```

See [docs/query-language.md](docs/query-language.md) for the full reference.

---

## Risk Scoring Formula

```
RiskScore = (TargetCriticality × 0.30)
          + (Confidence × 0.20)
          + (Exploitability × 0.20)
          + ((1 - Detectability) × 0.15)
          + (BlastRadius × 0.15)
```

All weights are tunable via `ScoringConfig`. See [docs/scoring.md](docs/scoring.md).

---

## Package Layout

| Package | Role |
|---------|------|
| `pkg/model` | Core domain types: Node, Edge, enums |
| `pkg/graph` | Graph engine: adjacency, traversal, path search |
| `pkg/scoring` | Risk scoring functions |
| `pkg/ingest` | Ingestion adapters |
| `pkg/normalize` | Data canonicalization |
| `pkg/reasoning` | Three analysis modes |
| `pkg/controls` | Breakpoint optimizer |
| `pkg/detection` | Sigma/KQL/SPL generation |
| `pkg/drift` | Snapshot diffing |
| `pkg/query` | DSL lexer + parser + executor |
| `pkg/reporting` | Markdown, JSON, and HTML report rendering |
| `pkg/snapshot` | SQLite-backed graph snapshot persistence |
| `pkg/policy` | Policy rule evaluation |
| `pkg/telemetry` | Telemetry requirement mapping |
| `pkg/evidence` | Evidence reference management |
| `pkg/providers` | LLM provider abstraction |

---

## Current Status

### What works today

| Capability | Status |
|-----------|--------|
| Graph engine (nodes/edges/traversal) | ✅ Fully implemented |
| JSON / CSV / BloodHound / YAML ingest | ✅ Implemented with tests |
| Risk scoring (configurable weights) | ✅ Implemented with tests |
| Path finding (iterative DFS, depth-limited) | ✅ Implemented with tests |
| Breakpoint optimizer (greedy set-cover) | ✅ Implemented with tests |
| DSL lexer + parser | ✅ Implemented with tests |
| `FIND PATHS` query execution | ✅ Returns ranked paths |
| `FIND BREAKPOINTS` query execution | ✅ Returns control recommendations |
| `SHOW DRIFT` query execution | ✅ Requires `--baseline` flag; calls `drift.CompareSnapshots` |
| Markdown + JSON reporting | ✅ Implemented with tests |
| Drift detection (CompareSnapshots) | ✅ Detects 4 drift categories |
| `ingest` CLI subcommand | ✅ Reads files, writes snapshots |
| `analyze` CLI subcommand | ✅ DSL queries against snapshot or fixture |
| `breakpoints` CLI subcommand | ✅ Greedy optimizer over snapshot or fixture |
| `report` CLI subcommand | ✅ Markdown/JSON/HTML report over snapshot or fixture |
| `diff` CLI subcommand | ✅ Drift report between two snapshots |
| `snapshot` CLI subcommand | ✅ save/list/diff/prune via SQLite at `~/.pathcollapse/snapshots.db` |
| HTML report generation | ✅ Single-file self-contained report for CISO review |
| GitHub Actions CI | ✅ Build, vet, race detector, goreleaser check |
| Integration test (full pipeline) | ✅ `test/integration/pipeline_test.go` |

### Stubbed / thin implementations

| Package | Status |
|---------|--------|
| `pkg/normalize` | Stub — dedup and canonicalization not yet implemented |
| `pkg/policy` | Stub — policy rule evaluation not yet implemented |
| `pkg/providers` | Stub — LLM provider abstraction not yet wired |
| `pkg/evidence` | Types and helpers only; no ingestion pipeline feeds it |
| `pkg/telemetry` | Requirement mapper only; no live telemetry collection |
| Detection (Sigma/KQL/SPL) | Generic templates; not path-specific yet |

### Verified demo workflow

```bash
# Ingest the built-in fixture dataset
./pathcollapse ingest --input internal/testdata/enterprise_ad.json \
  --type json --output /tmp/snapshot.json

# Find attack paths from alice to tier-0
./pathcollapse analyze --graph /tmp/snapshot.json \
  --query "FIND PATHS FROM user:alice TO privilege:tier0 LIMIT 5"

# Compute minimal breakpoints
./pathcollapse breakpoints --graph /tmp/snapshot.json --top 5

# Generate a full markdown report
./pathcollapse report --graph /tmp/snapshot.json --format markdown --top 5
```

---

## Contributing

PathCollapse is open-source and welcomes contributions. Please open an issue before submitting large changes.

```bash
go test ./...
go vet ./...
```

### CI gate

Every pull request must pass the race detector before merging:

```bash
go test -race ./...
```

### Profiling

A `Makefile` is provided for common profiling tasks:

```bash
make bench        # run all benchmarks with allocation counts
make cpu-profile  # capture CPU profile for FindPaths (writes cpu.prof)
make mem-profile  # capture heap profile for FindPaths (writes mem.prof)
make race         # run full test suite under the race detector
```

---

## License

Apache 2.0
