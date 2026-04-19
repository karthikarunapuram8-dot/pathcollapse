# Example Workflows

## Workflow 1: Analyse a BloodHound Export

```bash
# 1. Ingest BloodHound users export
pathcollapse ingest --input BloodHound_Users.json --type bloodhound --output users-snap.json

# 2. Ingest groups
pathcollapse ingest --input BloodHound_Groups.json --type bloodhound --output groups-snap.json

# 3. Find paths from all users to tier-0 assets
pathcollapse analyze --query "FIND PATHS FROM user:jsmith TO privilege:tier0 LIMIT 10"

# 4. Get the top breakpoints
pathcollapse breakpoints --top 5
```

## Workflow 2: CSV-Based Enterprise AD Audit

```bash
# Ingest users
pathcollapse ingest --input users.csv --type csv_users

# Ingest group memberships
pathcollapse ingest --input groups.csv --type csv_groups

# Ingest local admin relationships
pathcollapse ingest --input local-admins.csv --type csv_local_admin

# Generate a markdown report with detections and telemetry requirements
pathcollapse report --format markdown --top 20 --output security-report.md

# Generate JSON for downstream tooling and SOAR ingestion
pathcollapse report --format json --output report.json
```

## Workflow 3: Drift Detection

```bash
# Take a baseline snapshot
pathcollapse ingest --input identity-jan.json --type json --output snap-jan.json

# After a change window, take a new snapshot
pathcollapse ingest --input identity-feb.json --type json --output snap-feb.json

# Compare
pathcollapse diff snap-jan.json snap-feb.json
```

Expected output:
```
# Drift Report

Nodes added: 3, removed: 1
Edges added: 12, removed: 2

Security-relevant changes (2):

1. [high] new_privileged_membership — New membership in tier-0 group detected
2. [high] dangerous_delegation — New unconstrained delegation relationship detected
```

## Workflow 4: YAML Facts for Manual Analyst Additions

```yaml
# facts.yaml — analyst-provided relationships
nodes:
  - id: svc-reporting
    type: service_account
    name: svc-reporting
    tags: []

edges:
  - id: manual-001
    type: can_sync_to
    source: svc-reporting
    target: cmp-dc01
    confidence: 0.85
    exploitability: 0.9
    detectability: 0.1
    blast_radius: 1.0
```

```bash
pathcollapse ingest --input facts.yaml --type yaml
pathcollapse analyze --query "FIND PATHS FROM service_account:svc-reporting TO privilege:tier0"
```

## Workflow 5: Detection-Focused Reporting

```bash
# Build a rich HTML report for responders and detection engineers
pathcollapse report --graph snapshot.json --format html --top 10 --output detections.html

# Build JSON for downstream enrichment pipelines
pathcollapse report --graph snapshot.json --format json --top 10 --output detections.json

# Turn off calibrated recommendation confidence for side-by-side comparison
pathcollapse report --graph snapshot.json --format markdown --top 10 --confidence off
```

The resulting report includes:
- ATT&CK techniques per high-risk path
- Sigma, KQL, and SPL content ready for tuning
- Required log sources and event IDs
- Visibility gaps that may block reliable detection
- Optional calibrated recommendation confidence with factor-level drivers

## Workflow 6: Generating Detection Content Programmatically

Use the detection package programmatically:

```go
import "github.com/karthikarunapuram8-dot/pathcollapse/pkg/detection"

art := detection.MapPath(path)
fmt.Println(art.SigmaRule)   // Sigma YAML
fmt.Println(art.KQLQuery)    // Microsoft Sentinel
fmt.Println(art.SPLQuery)    // Splunk
for _, t := range art.ATTACKTechniques {
    fmt.Printf("%s: %s\n", t.ID, t.Name)
}
```

## Workflow 7: Shadow-Mode Confidence Calibration

```bash
# 1. Collect recommendations without exposing the unvalidated score to analysts
pathcollapse breakpoints --graph snapshot.json --top 5 --shadow-mode

# 2. Inspect how many labeled outcomes you have so far
pathcollapse confidence status

# 3. Annotate ~/.pathcollapse/shadow.jsonl once outcomes are known
#    observed_collapsed=true|false
#    observed_regression=true|false

# 4. Fit and persist the calibrator after you have enough labels
pathcollapse confidence refit --require-minimum 50

# 5. Subsequent runs auto-load ~/.pathcollapse/calibrator.json
pathcollapse breakpoints --graph snapshot.json --top 5 --confidence on
```

What `confidence status` tells you:
- How many shadow-log entries were parsed and how many are labeled
- Progress toward `partial` (50 labels) and `calibrated` (500 labels)
- Whether a saved calibrator exists and will auto-load
- Brier / ECE metrics from the last refit
