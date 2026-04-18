# PathCollapse Architecture

## Data Flow

```
External Data Sources
  ├── AD LDAP Export (JSON)
  ├── BloodHound Collector Output
  ├── CSV Exports (users, groups, local admins, GPOs)
  └── Analyst YAML Facts
         │
         ▼
  pkg/ingest  ────────────────────────────────────────────
  │  Adapter interface: Name() + Ingest(io.Reader) Result
  │  Adapters: JSONAdapter, CSVUserAdapter, CSVGroupAdapter,
  │            CSVLocalAdminAdapter, CSVGPOAdapter,
  │            BloodHoundAdapter, YAMLAdapter
  │
  ▼
  pkg/normalize
  │  Deduplicates nodes (same ID → merge attrs)
  │  Deduplicates edges (same src+type+tgt → keep higher confidence)
  │  Canonicalizes names
  │
  ▼
  pkg/graph  ──────────────────────────────────────────────
  │  Graph{nodes, edges, forward, reverse} — RWMutex-safe
  │  AddNode, AddEdge, RemoveNode, RemoveEdge
  │  GetNode, GetEdge, Neighbors, ReverseNeighbors
  │  FindPaths(from, to, PathOptions) []Path
  │  FilteredTraversal, WeightedTraversal
  │  ConnectedComponents, PrivilegeConcentration
  │
  ├─────────────────────────────────────────────────────────
  │
  ├──► pkg/scoring
  │      ScoringConfig (configurable weights)
  │      ScorePath(path, graph, config) float64
  │      RankPaths(paths, graph, config) []ScoredPath
  │
  ├──► pkg/reasoning
  │      Reasoner.FindAndAnalyse(from, to, mode, opts) []PathAnalysis
  │      Modes: ModeReachability | ModePlausibility | ModeDefensive
  │      Defensive: avg confidence, detectability, remediation hints
  │
  ├──► pkg/controls  ← PRIMARY VALUE PROPOSITION
  │      Optimize(scored, graph, config) []ControlRecommendation
  │      Algorithm: greedy set-cover over edge breakpoints
  │      Output: ordered list of changes → paths-removed, risk-reduction, difficulty
  │
  ├──► pkg/detection
  │      MapPath(path) DetectionArtefact
  │      Outputs: log sources, Sigma YAML, KQL, SPL, ATT&CK techniques
  │
  ├──► pkg/drift
  │      CompareSnapshots(old, new *Graph) DriftReport
  │      Detects: new tier-0 memberships, delegation changes, trust expansion,
  │               cert template drift, DCSync rights
  │
  ├──► pkg/query
  │      ParseQuery(string) *Statement
  │      Executor.Execute(*Statement) *Result
  │      DSL: FIND PATHS / FIND BREAKPOINTS / SHOW DRIFT / FIND HIGH_RISK_*
  │
  └──► pkg/reporting
         Reporter.Render(w, Report)
         Formats: Markdown (executive + engineer), JSON
```

## Graph Model

### Node Types
- `user` — AD/Entra ID user accounts
- `group` — Security groups
- `computer` — Workstations, servers, domain controllers
- `service_account` — Service and managed accounts
- `ou` — Organizational units
- `gpo` — Group Policy Objects
- `certificate_authority` — AD CS CA servers
- `cert_template` — Certificate templates
- `trust` — Domain/forest trusts
- `application` — Enterprise applications
- `session` — Interactive sessions
- `role` — Roles (Azure RBAC / on-prem)
- `tenant` — Azure/Entra tenants
- `control` — Control nodes (abstract)
- `secret_store` — Password managers, vaults

### Edge Types
- `member_of` — Group membership
- `admin_to` — Full domain admin rights
- `local_admin_to` — Local administrator on a host
- `has_session_on` — Active credential session on a host
- `can_delegate_to` — Kerberos delegation (constrained or unconstrained)
- `can_sync_to` — DCSync or directory synchronization rights
- `can_enroll_in` — Certificate template enrollment rights
- `trusted_by` — Domain trust
- `inherits_policy_from` — GPO inheritance
- `controls_gpo` — GPO write/link rights
- `can_reset_password_of` — Password reset ACL
- `can_write_acl_of` — ACL modification rights
- `authenticates_to` — Authentication target
- `synced_to` — Azure AD Sync / hybrid identity
- `monitored_by` — SIEM/XDR coverage
- `protected_by` — MFA or PAM protection
- `privileged_over` — Generic privilege relationship

## Scoring Formula

See [scoring.md](scoring.md).

## Breakpoint Optimizer

The `pkg/controls` greedy set-cover algorithm:

1. Build a universe of scored paths (typically top 50–200 by risk score)
2. For every edge in every path, create a candidate control change
3. Score each candidate by: number of uncovered paths it would collapse
4. Greedily pick the best candidate until all paths are covered or budget is exhausted
5. Return recommendations ordered by paths-collapsed descending

Time complexity: O(P × E × R) where P = paths, E = edges/path, R = recommendations.
For typical inputs (200 paths, 5 edges/path, 20 recommendations) this is fast (<1ms).
