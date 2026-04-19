# PathCollapse Roadmap

## v0.1 — Foundation (shipped)

- [x] Typed identity graph (15 node types, 17 edge types)
- [x] Multi-mode path reasoning (reachability, plausibility, defensive)
- [x] Greedy breakpoint / set-cover optimizer
- [x] Detection mapper (Sigma, KQL, SPL, ATT&CK)
- [x] Analyst DSL query language
- [x] Ingestion: generic JSON, CSV, BloodHound JSON, YAML
- [x] Drift detection across graph snapshots
- [x] Reporting: Markdown, JSON, HTML
- [x] SQLite snapshot persistence (`snapshot save/list/diff/prune`)
- [x] Calibrated recommendation confidence with per-factor breakdowns and snapshot-backed temporal stability
- [x] Shadow-mode data collection (`--shadow-mode`) and isotonic calibrator refit (`confidence refit`)
- [x] GitHub Actions CI + GoReleaser cross-platform binaries

## v0.2 — Live Data & Integrations

- [ ] Azure AD / Entra ID Graph API ingestor
- [ ] AWS IAM ingestor (roles, policies, resource-based policies)
- [ ] Okta ingestor
- [ ] BloodHound CE API ingestor (live, not file-based)
- [ ] SIEM alert correlation (enrich paths with existing detections)

## v0.3 — Scale & Performance

- [ ] Streaming graph updates (incremental diff, not full reload)
- [ ] Graph partitioning for environments with > 500k nodes
- [ ] Parallel path scoring with worker pools
- [ ] Persistent graph store (SQLite or Postgres backend)

## v0.4 — Automation & Remediation

- [ ] Jira / ServiceNow ticket creation for top recommendations
- [ ] Slack / Teams notifications on drift exceeding threshold
- [ ] CI gate mode — fail pipeline if risk score exceeds budget
- [ ] Remediation playbook generation (Ansible, Terraform snippets)

## v1.0 — Production Ready

- [ ] Stable CLI API (no breaking flag changes)
- [ ] Comprehensive integration test suite against real AD lab
- [ ] Learned-β refit (logistic regression over shadow-mode data; extends the isotonic calibrator shipped in v0.1)
- [ ] Docker / OCI image on GHCR
- [ ] Homebrew tap
- [ ] Operator documentation and runbooks
