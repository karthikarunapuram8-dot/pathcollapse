# PathCollapse — Launch Materials

Ready-to-paste copy for each surface. Keep tone sober — no "AI-powered",
"next-gen", "fully calibrated", "production-ready". The repo is
strongest when it reads as a shipping, honest OSS project.

Canonical repo URL: https://github.com/karthikarunapuram8-dot/pathcollapse

---

## 1. LinkedIn

```
Built and shipped an open-source Go project for identity exposure analysis.

Most tools show attack paths.
PathCollapse shows what to fix first.

It models identity and privilege relationships as a graph, ranks risky paths, and computes the smallest control changes that collapse the most risk. It also ships with drift detection, HTML executive reports, and a calibrated recommendation-confidence system with shadow-mode collection and refit support.

Recent releases include:
• breakpoint optimizer
• snapshot persistence + diffing
• HTML/Markdown/JSON reporting
• calibrated recommendation confidence
• shadow-mode confidence collection + refit loop
• cross-platform binaries and working `go install`

Always interested in feedback from defenders, IAM engineers, security architects, and Go folks.

#opensource #cybersecurity #golang #activedirectory #identitysecurity
```

**Attach:** `docs/assets/demo.gif` (or a still screenshot from it).
**CTA:** link to the repo in the first comment to avoid LinkedIn downranking external URLs.

---

## 2. X / Twitter

**Primary (under 280 chars):**

```
Shipped PathCollapse.

Most tools show attack paths. PathCollapse shows what to fix first.

Open-source Go: breakpoint optimizer, drift detection, HTML reports, calibrated confidence, shadow-mode refit loop.

github.com/karthikarunapuram8-dot/pathcollapse
```

**Alt (more structured):**

```
Launched PathCollapse: identity graph analytics that tells defenders what to fix first.

Go OSS:
• breakpoint optimizer
• attack-path analysis
• drift detection
• HTML reports
• calibrated confidence

github.com/karthikarunapuram8-dot/pathcollapse
```

**Media:** attach `docs/assets/demo.gif`. Twitter autoplay makes the GIF the
conversion lever, not the text.

---

## 3. Hacker News — Show HN

**Title (primary):**
```
Show HN: PathCollapse – Identity graph analytics that tells defenders what to fix first
```

**Title (alternate):**
```
Show HN: PathCollapse – Find the smallest security changes with the biggest impact
```

**URL field:** `https://github.com/karthikarunapuram8-dot/pathcollapse`

**First comment** (HN practice is to drop context as a self-comment right after posting):

```
Author here. Short version: BloodHound and similar tools show attack paths. PathCollapse is aimed at the next question — which few control changes collapse the most risk, and how confident are we in each recommendation?

Core pieces:
- Typed identity graph (15 node types, 17 edge types covering AD/Entra ID)
- Greedy set-cover breakpoint optimizer
- Calibrated per-recommendation confidence with a five-factor decomposable score (evidence, structural robustness, safety, temporal stability, coverage concentration)
- Shadow-mode JSONL logger + isotonic calibrator refit command so the confidence system actually learns from post-change outcomes rather than living on informed priors forever
- Snapshot persistence + drift detection
- HTML / Markdown / JSON reports with per-rec drivers

Honest limitations:
- The calibrator ships in `cold_start` regime. You need ≥50 labeled outcomes to hit `partial`, ≥500 for `calibrated`. The paper (docs/confidence.md) is explicit about this.
- No live ingestors yet — currently works off file-based BloodHound exports, CSVs, and YAML. Entra ID / AWS IAM / Okta live ingestion is the v0.3 target.
- Pure Go, no CGO, cross-platform binaries on GitHub Releases.

Docs: docs/confidence.md has the full paper for the confidence algorithm including factor definitions, calibration methodology, and evaluation protocol (Brier + ECE + reliability diagrams).

Feedback from IAM engineers, detection engineers, and Go folks especially welcome.
```

---

## 4. Reddit

### r/golang

**Title:**
```
Built a graph-based identity risk prioritization engine in Go with reporting, drift detection, and calibrated confidence
```

**Body:**

```
Shipped PathCollapse, an open-source Go tool for identity exposure analysis. It models privilege relationships in AD / Entra ID as a graph and computes the smallest set of control changes that collapse the most risk.

A few bits that might interest this sub specifically:

**Engineering choices**
- Pure Go, no CGO. SQLite via `modernc.org/sqlite` for snapshot persistence — avoids the cgo toolchain headache on Windows.
- Greedy set-cover optimizer with an incremental count-update design. Took a 1.17 ms per-run implementation down to 435 µs by precomputing edge→path indexing.
- Log-odds aggregation for a five-factor decomposable confidence score, followed by isotonic regression (Pool-Adjacent-Violators) for post-hoc probability calibration.

**Repo:** github.com/karthikarunapuram8-dot/pathcollapse

The confidence algorithm has a full working paper at `docs/confidence.md` — factor definitions, log-odds aggregation, isotonic calibration, Brier/ECE evaluation protocol. Would welcome review from folks who've done probability calibration in Go before.

Happy to answer questions about the set-cover optimization, the graph engine choices, or the shadow-mode JSONL → isotonic refit loop.
```

### r/netsec

**Title:**
```
Open-source tool for prioritizing identity remediation in AD and enterprise access graphs
```

**Body:** *r/netsec is strict; keep it factual, no marketing.*

```
PathCollapse is an open-source Go tool that ingests identity/policy metadata (BloodHound JSON, CSV exports, YAML facts), constructs a typed privilege graph, and computes the smallest set of control changes that collapse the highest-risk attack paths.

Defensive-only: no network scanning, credential access, or attack execution.

Core capabilities:
- 15 node types, 17 edge types covering common AD/Entra ID relationships (member_of, admin_to, can_delegate_to, can_sync_to, can_enroll_in, etc.)
- Greedy set-cover breakpoint optimizer over ranked attack paths
- Drift detection across graph snapshots (new privileged memberships, delegation changes, cert template drift)
- HTML / Markdown / JSON reports with path-specific ATT&CK mapping, log-source guidance, and templated Sigma/KQL/SPL content
- Per-recommendation confidence with a five-factor decomposable score and shadow-mode data collection → isotonic calibration refit loop

Current limitations (called out honestly):
- File-based ingestion only. Live ingestors (Entra ID Graph API, AWS IAM, Okta, BloodHound CE API) are v0.3 roadmap, not shipped.
- Confidence calibrator starts in cold_start regime — operators run shadow mode for 2–4 weeks to hit partial, longer for calibrated. Informed priors until then.

Repo: github.com/karthikarunapuram8-dot/pathcollapse
Algorithm paper: docs/confidence.md
```

### r/sysadmin

**Title:**
```
Go tool that helps identify which AD permission changes reduce the most risk first
```

**Body:**

```
Built an open-source tool for the "we have too many findings, what do we fix first" problem in AD and enterprise identity environments.

Input: a BloodHound JSON export, a CSV dump, or a YAML file of facts.
Output: a ranked list of the *fewest* control changes (remove-member, revoke-admin, disable-delegation) that collapse the *most* high-risk attack paths.

Plus: drift detection between snapshots, HTML executive reports for CISO review, and a calibrated confidence score per recommendation so you can triage which changes to push hardest.

Cross-platform binaries on GitHub Releases, or `go install github.com/karthikarunapuram8-dot/pathcollapse/cmd/pathcollapse@latest`.

Repo: github.com/karthikarunapuram8-dot/pathcollapse

Would love feedback from anyone running BloodHound periodically and looking for something that answers "ok but what do we *do* about it" rather than "here are 400 paths."
```

---

## 5. Recruiter / Hiring Manager Framing

Short, action-and-measurement-oriented:

```
I built and shipped PathCollapse, an open-source Go platform for identity exposure analysis. It models enterprise privilege relationships as a graph, ranks risky paths, and computes the minimal control changes that reduce the most risk. I also built reporting, drift detection, cross-platform release automation, and a calibrated confidence pipeline with shadow-mode collection and isotonic refit.
```

---

## 6. Resume Bullet

**Primary:**

```
Built and launched PathCollapse, an open-source Go identity exposure analysis platform that prioritizes high-impact remediation using graph optimization, snapshot drift detection, executive-grade reporting, and calibrated confidence scoring (isotonic regression with shadow-mode outcome collection).
```

**Shorter:**

```
Created an open-source Go security analytics platform for identity risk prioritization, combining graph reasoning, set-cover optimization, reporting, and calibration workflows.
```

---

## 7. Positioning Line

Primary tagline:

> Others show attack paths. PathCollapse shows priorities.

Backup:

> Find the few access changes that matter most.

---

## 8. Language to avoid

Hard-ban in all copy:
- "AI-powered"
- "next-gen"
- "revolutionary"
- "military-grade"
- "world-class"
- "fully calibrated" (until there's real outcome data on a labeled set)
- "production-ready for every environment"

Soft-avoid (overclaims the current detection implementation):
- Any phrasing implying the Sigma/KQL/SPL output is deeply per-edge-sequence specialized. Today it's templated with path context — the ATT&CK mapping is path-aware, the rule bodies are generic.

---

## 9. Launch cadence

Recommended order (per the positioning analysis):

1. Tighten README + visuals (done — GIF + HTML screenshot in place)
2. Post LinkedIn + X simultaneously. Attach the GIF on both.
3. Post to r/golang (engineering angle) 24 hours later — gives the LinkedIn post time to get impressions before a different audience sees a different framing.
4. Show HN on a weekday morning US time (~9am ET is the peak window).
5. r/netsec + r/sysadmin only after HN/r/golang have surfaced any obvious critique — those subs are strict and a single bad reception there is harder to recover from than on LinkedIn.

---

## 10. Links needed in first comments / replies

- GitHub: https://github.com/karthikarunapuram8-dot/pathcollapse
- v0.2.1 release: https://github.com/karthikarunapuram8-dot/pathcollapse/releases/tag/v0.2.1
- Confidence algorithm paper: https://github.com/karthikarunapuram8-dot/pathcollapse/blob/main/docs/confidence.md
- Demo GIF: https://github.com/karthikarunapuram8-dot/pathcollapse/blob/main/docs/assets/demo.gif
