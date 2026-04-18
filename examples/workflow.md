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

# Generate a markdown report
pathcollapse report --format markdown --top 20 --output security-report.md

# Generate JSON for downstream tooling
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

## Workflow 5: Generating Detection Content

Use the detection package programmatically:

```go
import "github.com/karunapuram/pathcollapse/pkg/detection"

art := detection.MapPath(path)
fmt.Println(art.SigmaRule)   // Sigma YAML
fmt.Println(art.KQLQuery)    // Microsoft Sentinel
fmt.Println(art.SPLQuery)    // Splunk
for _, t := range art.ATTACKTechniques {
    fmt.Printf("%s: %s\n", t.ID, t.Name)
}
```
