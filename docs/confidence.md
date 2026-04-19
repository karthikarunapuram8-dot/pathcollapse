# Calibrated Breakpoint Confidence for Identity Exposure Graphs

*Working paper — PathCollapse v0.x*
*Last updated: 2026-04-18*

> **Implementation status:** the algorithm described here is implemented
> in [pkg/confidence][pc-conf-pkg] and wired into the optimizer behind an
> opt-in `OptimizerConfig.Confidence` field. When enabled, the static
> `0.85` value is replaced per-recommendation with a calibrated score plus
> a `Breakdown` exposing the five factors. When disabled, the legacy
> constant is preserved for backward compatibility. See §8 for the
> integration shape and [controls.go][pc-controls] for the call site.
> Revision history of note: §5.1 `ε` moved from `10⁻³` to `0.05` after a
> structural-saturation regression surfaced in implementation testing.

---

## Abstract

PathCollapse historically emitted a static confidence of `0.85` on every
`ControlRecommendation` returned by the greedy set-cover optimizer
([pkg/controls/controls.go][pc-controls]). This single constant carried no
information about whether the recommended control will actually collapse the
flagged exposure, whether the underlying graph evidence is trustworthy, or
whether the change is safe to apply in production. It is also uncalibrated:
given a hundred `0.85`-confidence recommendations, the empirical rate at which
they succeed in collapsing their target paths is unknown.

We propose a decomposable, calibrated confidence score
`C(c) ∈ [0, 1]` for a recommended control change `c`. The score combines five
independent factors — evidence quality, structural robustness, operational
safety, temporal stability, and coverage concentration — via log-odds
aggregation, then passes through an isotonic regression step trained on a
labeled validation set of historical change outcomes. The resulting score is
interpretable as a probability: out of 100 recommendations with
`C(c) = 0.72`, we expect ~72 to collapse their target paths without operational
regression.

The algorithm runs in `O(|P_e| · d̄ + |E| · ρ)` per recommendation, where
`P_e` is the paths covered by the edge, `d̄` is average path length, and
`ρ` is the average residual-path search cost — cheap enough to compute inline
inside the existing greedy loop without changing its `O(|E|²)` worst case.

This document specifies the factor definitions, the aggregation, the
calibration methodology, the evaluation protocol (Brier score and reliability
diagrams), and a concrete integration plan for the PathCollapse Go codebase.

---

## 1. Introduction

### 1.1 The 0.85 problem

The current implementation is:

```go
// pkg/controls/controls.go:146-153
recommendations = append(recommendations, ControlRecommendation{
    Change:        best.change,
    PathsRemoved:  bestCount,
    RiskReduction: riskReduced,
    Difficulty:    difficultyForChange(best.change.Type),
    Confidence:    0.85, // ← hardcoded, uncalibrated, uninformative (pre-implementation)
    AffectedPaths: affectedPaths,
})
```

This is equivalent to shipping a risk score that always returns `0.85`. It
defeats the purpose of ranking. In operational terms, three pathologies follow:

1. **Analyst desensitization.** When every recommendation reports `0.85`, the
   field is ignored. Confidence becomes decorative.
2. **No triage signal.** A change that removes a unique membership with a single
   BloodHound-sourced piece of evidence scores identically to a change that
   revokes a long-standing, multiply-corroborated delegation. The former is far
   riskier to apply; the latter is far more certain to be real.
3. **No feedback loop.** Without a calibrated score, there is no meaningful
   ground-truth to compare against after the change is applied. Post-hoc
   analysis (the change worked / did not work) cannot retrain anything.

### 1.2 Desiderata

A strong confidence algorithm for PathCollapse must satisfy:

- **D1 — Decomposability.** The score must be explainable as a product (or
  sum-in-log-odds) of named, locally-measurable factors. An analyst must be able
  to read "`C = 0.41` because safety is low (0.22) despite high evidence (0.95)"
  directly from the output.
- **D2 — Calibration.** `C(c) = p` should predict that the recommendation
  succeeds with empirical frequency `p ± ε` on a held-out set.
- **D3 — Graph-local.** The score must be computable from the already-loaded
  `*graph.Graph` and recently-stored snapshots. No external enrichment.
- **D4 — Cheap.** Must fit inside the greedy loop without changing its
  asymptotic complexity on enterprise-scale graphs (10k–100k nodes).
- **D5 — Cold-start safe.** On an unlabeled deployment, the score must degrade
  gracefully to a principled prior, not to a constant.

The algorithm below satisfies all five.

---

## 2. Problem formulation

Let:

- `G = (V, E)` be the directed identity graph.
- `P = {p₁, p₂, …}` be the set of scored attack paths surfaced by
  `scoring.RankPaths`.
- A recommendation `c` consists of a `ControlChange` targeting a graph element
  — currently always an edge `e ∈ E` in the implementation — together with a
  set `P_e = {p ∈ P : e ∈ p}` of paths the change covers.
- Let `G \ e` denote the graph with edge `e` deleted.

The **collapse event** `Y(c) ∈ {0, 1}` is the ground-truth indicator that,
after applying `c`:

1. Every path in `P_e` becomes either non-existent or scores below a
   pre-specified risk threshold `τ` in `G \ e` (the change **collapsed** the
   exposure), **and**
2. No previously-legitimate service experienced authentication failure within
   a bounded observation window (the change did not **regress**).

`Y(c) = 1` iff both hold. We define **confidence** as the predicted
probability of the collapse event:

```
C(c) := P[Y(c) = 1 | G, snapshots, c]
```

This is the quantity the current `0.85` nominally claims to estimate.
Below we construct it properly.

---

## 3. Confidence decomposition

We decompose `C(c)` into five factors, each mapped into `[0, 1]`:

| Symbol  | Factor                    | Question it answers                                               |
|--------|---------------------------|-------------------------------------------------------------------|
| `E(e)` | Evidence quality          | Is the edge itself real?                                          |
| `R(e, G)` | Structural robustness | Does removing `e` actually collapse the exposure, or is there an equivalent bypass? |
| `S(e, G)` | Operational safety    | Is the edge used only by attackers, or also by legitimate operations? |
| `T(e)` | Temporal stability        | Has the edge existed long enough that removing it is considered, not reactive? |
| `K(c)` | Coverage concentration    | Does this single change cleanly own its covered paths, or is it one of many near-duplicates? |

The factors are chosen to be *approximately independent*: a high-evidence edge
can be structurally non-robust (`R` low), operationally critical (`S` low),
etc. This supports the log-odds aggregation in §5.1.

Notation: for any edge-level factor `F(e) ∈ (0, 1)` we write the logit
`ℓ(F) := log(F / (1 − F))`.

---

## 4. Per-factor computation

### 4.1 Evidence quality `E(e)`

Three sub-signals, all already present in `model.Edge`:

1. **Ingest-reported confidence.** `e.Confidence` directly (range
   `[0, 1]`).
2. **Source corroboration.** If `|e.Evidence| ≥ 2` and the sources are
   distinct (BloodHound + CSV + YAML), the edge has been observed by more than
   one collector. Let `n_src = |unique(e.Evidence[*].Source)|`. Define
   `s_src = 1 − exp(−λ · (n_src − 1))` with `λ = 0.7` so `n_src = 1 ⇒
   s_src = 0`, `n_src = 2 ⇒ s_src ≈ 0.50`, `n_src = 3 ⇒ s_src ≈ 0.75`.
3. **Precondition health.** If the edge type declares
   `Preconditions` (e.g. "user must hold a valid TGT"), let
   `s_pre = |satisfied| / |preconditions|`. If no preconditions are
   declared, `s_pre = 1` (neutral).

Aggregate as a weighted geometric mean to respect the "chain of necessary
conditions" intuition — if any sub-signal is near 0, the whole factor should
be near 0:

```
E(e) = e.Confidence ^ 0.5  ·  max(s_src, ε) ^ 0.3  ·  s_pre ^ 0.2
```

with `ε = 0.05` to prevent single-source edges from zeroing out the product.
The exponent weights sum to 1, so `E(e) ∈ [0, 1]`.

### 4.2 Structural robustness `R(e, G)`

This is the most important factor and directly addresses D1 / the collapse
event definition in §2.

For each path `p ∈ P_e`, compute whether the source can still reach the
target in `G \ e` with score ≥ `τ`:

```
residual(p, e) = max{ScorePath(p', G, cfg) : p' ∈ FindPaths(p.source, p.target, G\e, MaxDepth=5)}
r(p, e)        = 1 − min(residual(p, e) / τ, 1)        // higher = edge truly kills the path
R(e, G)        = (1 / |P_e|) · Σ_{p ∈ P_e} r(p, e)
```

Interpretation:

- `R ≈ 1` ⇒ removing `e` leaves every covered path without a viable
  high-risk alternative. The recommendation is structurally clean.
- `R ≈ 0` ⇒ for most covered paths, an alternative of comparable risk
  still exists after removing `e`. The greedy optimizer "covered" `P_e` in
  a bookkeeping sense but did not actually reduce exposure.

**Cost.** Each residual computation is one `FindPaths` call on `G \ e`.
Rather than mutate `G`, we pass a visited-edge set to a path-search variant
(`FindPathsMasked`) that treats `e` as absent. Average cost per path is
comparable to the path-finding already done during ranking.

Total for one recommendation: `O(|P_e| · ρ)` where `ρ` is per-query path
search cost. For top-10 breakpoints with ~50 paths each on the 10k-node
benchmark, this adds an estimated ~2–5 ms to `Optimize`, well under the current
440 µs budget × |recs| overhead. Profiling is required before committing to the
inline design; an opt-in
`OptimizerConfig.ConfidenceEnabled bool` flag lets us ship cheaply and
measure.

### 4.3 Operational safety `S(e, G)`

"Does this edge also carry legitimate load?" There is no ground truth inside
the graph, but three proxies are available:

1. **Tier alignment.** Let `src_tier, tgt_tier ∈ {tier0, tier1, tier2, none}`
   from node tags. If `src_tier ≥ tgt_tier` (e.g. a tier-1 admin group with
   admin_to on a tier-1 server), the edge is plausibly legitimate:
   `s_tier = 0.3`. If `src_tier < tgt_tier`, the edge is suspicious (a
   tier-2 user reaching tier-0), `s_tier = 0.9`. Default `s_tier = 0.6`.

2. **Edge-type prior.** Some edge types are almost always abuse paths
   (`EdgeCanSyncTo` / DCSync from a non-DC principal, `EdgeCanEnrollIn`
   on a vulnerable template), while others are routinely legitimate
   (`EdgeMemberOf` for ordinary groups). Maintain a static table
   `s_type[EdgeType]` calibrated from the Microsoft Security
   advisories and ATT&CK technique families already wired in
   [pkg/detection/detection.go:87][pc-detection].

3. **Blast-radius inversion.** Large `e.BlastRadius` means "if this edge
   breaks, a lot breaks," which is a safety penalty, not a reward.
   `s_blast = 1 − e.BlastRadius`.

Aggregate:

```
S(e, G) = 0.4 · s_tier + 0.4 · s_type + 0.2 · s_blast
```

`S` close to 1 means "safe to sever" — the edge is almost certainly
malicious or orphaned privilege. `S` close to 0 means "severing this will
break legitimate auth flows."

### 4.4 Temporal stability `T(e)`

Using the existing `pkg/snapshot` SQLite store, fetch the `N` most recent
snapshots (default `N = 8`). Let `n_present` be the number of snapshots in
which `e` (keyed by `(Source, Target, Type)`, not by `e.ID`, since IDs are
regenerated on re-ingest) appears.

```
presence       = n_present / N
age_days       = days_since(e.FirstSeen)
age_factor     = 1 − exp(−age_days / 30)               // half-life ≈ 21 days
T(e)           = 0.6 · presence + 0.4 · age_factor
```

**Intent.** A recommendation to revoke a freshly-observed edge (possible
drift from an attacker creating the membership) gets lower `T`, which *raises
the urgency* — but urgency is already handled by risk scoring. Temporal
stability is about **change safety**: removing a stable, long-observed edge
is less likely to break a legitimate workflow that was relying on it
transiently. Attacker-introduced edges are caught by the drift engine on the
risk-score side of the ledger.

If fewer than 2 snapshots exist, `T(e) = 0.5` (non-informative prior) — this
is the cold-start case of D5.

### 4.5 Coverage concentration `K(c)`

The greedy optimizer's first pick usually covers many paths with one edge,
but a *tie* between candidates (two edges each covering the same 10 paths
the same way) tells us the recommendation is a proxy — either edge works.
Ambiguity in the selection is a form of uncertainty that belongs in
confidence.

Let `A(e) = {e' ∈ candidates : pathIdxs(e') = pathIdxs(e)}` be the set of
candidates covering exactly the same paths. Define:

```
K(c) = 1 / |A(e)|
```

`K = 1` ⇒ `e` uniquely covers its path set. `K = 1/3` ⇒ three interchangeable
candidates; the analyst should know they can pick any of them and the
"specific edge" identity in the recommendation is arbitrary.

A stricter variant uses Jaccard overlap over `pathIdxs` with a threshold;
set-equality is the cheap default.

---

## 5. Aggregation and calibration

### 5.1 Log-odds aggregation

Factors combine additively in log-odds space:

```
z(c) = β₀ + β_E · ℓ(E)  + β_R · ℓ(R)  + β_S · ℓ(S)
            + β_T · ℓ(T)  + β_K · ℓ(K)
C_raw(c) = σ(z(c))      // σ = logistic
```

Starting weights (before any calibration data is available):

```
β₀ = 0.0, β_E = 1.2, β_R = 1.6, β_S = 1.1, β_T = 0.6, β_K = 0.8
```

The priors reflect the observation that structural robustness (`R`) is the
strongest predictor of "did the change actually collapse the path," and
temporal stability is a weaker, tie-breaking signal.

Clip each factor to `[ε, 1 − ε]` with `ε = 0.05` before taking `ℓ(·)` to
bound each factor's logit contribution to `±log(0.95/0.05) ≈ ±2.94`.

**Why `ε = 0.05`, not the more common `10⁻³`.** An earlier draft of this
spec proposed `ε = 10⁻³`, which bounds logits to `±6.9`. Implementation
testing surfaced a concrete failure mode: factors that routinely saturate
(K(c) = 1.0 whenever a recommendation is the sole candidate covering its
path set, E(e) = 1.0 whenever ingest reports perfect confidence) then push
a single factor's weighted contribution to `±βᵢ·6.9`, easily overwhelming
the other four factors and driving `C_raw` toward 0 or 1 regardless of
evidence on the other axes.

This is not a calibration problem — isotonic regression (§5.2) cannot
recover from structural saturation because the *ordering* is wrong, not
just the scale. The fix belongs upstream, in the aggregation: cap each
factor's contribution before it enters the sum.

At `ε = 0.05`, a single saturating factor contributes at most `|βᵢ|·2.94`,
which for our default weights (max `β_R = 1.6`) is `~4.7` in log-odds
space — enough to dominate when justified, but not enough to nullify the
other four factors.

The regression that caught this is preserved as
`TestScoreEdgeEndToEndDiamondGraph` in [pkg/confidence/confidence_test.go][pc-conf-test].
Moving `ε` from `10⁻³` to `0.05` changed that test's observed Final from
`0.99997` (single-factor saturation) to `0.88` (influenced by all factors),
without changing the underlying graph or any other input.

### 5.2 Isotonic calibration

`C_raw` is *monotone* in each factor by construction but not
*probability-calibrated*. We fit a one-dimensional monotone increasing
function `g: [0, 1] → [0, 1]` (isotonic regression) from historical pairs
`(C_raw(cᵢ), Yᵢ)` collected during operations:

```
C(c) = g(C_raw(c))
```

Implementation: the Pool-Adjacent-Violators algorithm on a sorted
validation set. Isotonic regression is preferred over Platt scaling here
because our `C_raw` is already sigmoidal and the mis-calibration is expected
to be non-sigmoidal (e.g. systematic under-confidence at the high end due to
unmodeled post-change churn).

Retrain `g` weekly from the rolling window of applied changes.

### 5.3 Cold-start priors

For deployments with zero history, `g` is the identity and the aggregation
weights are the priors from §5.1. This guarantees D5: a brand-new
deployment still emits meaningful, factor-driven variation — not `0.85`.

We also recommend an explicit `confidence_regime` field in the output with
values `{cold_start, partial, calibrated}` reflecting the count of
labeled examples `g` was trained on.

---

## 6. Algorithm

### 6.1 Pseudocode

```
Inputs:
  scored  : []ScoredPath                 // already computed
  g       : *graph.Graph
  recs    : []ControlRecommendation       // output of greedy set-cover
  snaps   : snapshot.Store                // for T(e)
  cal     : calibration.Model             // for g(·); identity if cold start

for each rec in recs:
    e := edgeOf(rec.Change)
    P_e := rec.AffectedPaths

    // §4.1
    E := edgeEvidence(e)

    // §4.2 — residual-path check
    R := structuralRobustness(e, P_e, g, cfg)

    // §4.3
    S := operationalSafety(e, g)

    // §4.4
    T := temporalStability(e, snaps)

    // §4.5
    K := coverageConcentration(e, candidateIndex)

    // §5.1
    z := β0 + β_E·logit(clip(E)) + β_R·logit(clip(R)) +
              β_S·logit(clip(S)) + β_T·logit(clip(T)) +
              β_K·logit(clip(K))
    C_raw := σ(z)

    // §5.2
    rec.Confidence = cal.Apply(C_raw)
    rec.ConfidenceBreakdown = {E, R, S, T, K, raw: C_raw}
```

The greedy selection loop is unchanged; confidence runs as a post-pass.

### 6.2 Complexity

Per recommendation, dominated by §4.2 residual path-finding:

```
O(|P_e| · ρ + d̄ + snapshots_scanned + |A(e)|)
  ≈ O(|P_e| · ρ)
```

For the top-10 recommendations on the 10k-node benchmark (`Optimizer_10k`:
440 µs, 10 breakpoints/op), measured projection based on average
`|P_e| ≈ 50` and residual-path cost ≈ `FindPaths_10k` time (14.2 µs):

```
added ≈ 10 · 50 · 14.2 µs ≈ 7.1 ms
```

This is substantial (15× the current optimizer wall-clock) but still well
under a 100 ms per-analysis budget. The additional cost is confined to the
confidence pass, not the path-ranking hot loop, and is opt-in.

### 6.3 Optimizations

1. **Memoize `FindPaths`** keyed by `(src, dst, e)`. Multiple breakpoints
   touching the same source/target pair share residual computations.
2. **Early termination.** If the first residual path found has score < τ,
   `r(p, e) = 1` immediately — stop searching.
3. **Pre-pass candidate index.** Build `A(e)` via an inverted index on
   `pathIdxs` hashes, populated during the existing candidate scan in
   [pkg/controls/controls.go:92–101][pc-controls].

---

## 7. Evaluation protocol

### 7.1 Labeled dataset

For each applied change `cᵢ`, we require a post-change label `Yᵢ` from two
sources:

- **Collapse check.** Re-ingest 24 h after the change, re-run the same
  `FIND PATHS` query, score the result. `collapsed = 1` iff no path in the
  original `P_e` scores ≥ τ.
- **Regression check.** Parse authentication-failure logs from the affected
  source/target for the 24 h window. `no_regression = 1` iff failure rate
  is within the ±2σ band of the pre-change baseline.

`Yᵢ = collapsed · no_regression`.

The regression check depends on telemetry that `pkg/telemetry` does not yet
ingest; until then, `Yᵢ` uses `collapsed` only and the model is trained
against a proxy label. This is a known limitation (see §9).

### 7.2 Metrics

Three standard probability-calibration metrics:

1. **Brier score.** `BS = (1/N) · Σ (C(cᵢ) − Yᵢ)²`. Lower is better.
   Baseline (static 0.85): `BS₀ = (0.85 − Ȳ)² + Var(Y)`. Publish the ratio
   `BS / BS₀` — a value < 1 beats the baseline.
2. **Reliability diagram.** Bucket `C(c)` into deciles, plot mean predicted
   vs. mean empirical. A calibrated model lies on `y = x`.
3. **Expected calibration error (ECE).** Mean over deciles of
   `|mean_predicted − mean_observed|`, weighted by bucket size. Target
   `ECE < 0.05` after a full retraining cycle (≥ 500 labeled examples).

Additionally, **discrimination** (AUC-ROC) should exceed 0.80 to justify the
algorithm's operational cost over the static baseline.

### 7.3 Baselines

Compare against:

- `C(c) = 0.85` (current).
- `C(c) = E(e)` (evidence-only; the obvious naive improvement).
- `C(c) = R(e, G)` (structural-only).
- The full five-factor model.

We expect the full model to dominate on both Brier and ECE; single-factor
models will show characteristic reliability-diagram failures (e.g.
evidence-only is systematically over-confident on structurally non-robust
edges).

---

## 8. Integration with PathCollapse

### 8.1 Data model changes

Extend `ControlRecommendation` in `pkg/controls/controls.go`:

```go
type ControlRecommendation struct {
    Change        ControlChange          `json:"change"`
    PathsRemoved  int                    `json:"paths_removed"`
    RiskReduction float64                `json:"risk_reduction"`
    Difficulty    Difficulty             `json:"difficulty"`
    Confidence    float64                `json:"confidence"`
    Breakdown     *ConfidenceBreakdown   `json:"confidence_breakdown,omitempty"`
    Regime        ConfidenceRegime       `json:"confidence_regime"`
    AffectedPaths []graph.Path           `json:"-"`
}

type ConfidenceBreakdown struct {
    Evidence         float64 `json:"evidence"`           // E(e)
    Robustness       float64 `json:"robustness"`         // R(e, G)
    Safety           float64 `json:"safety"`             // S(e, G)
    TemporalStability float64 `json:"temporal_stability"` // T(e)
    CoverageConcentration float64 `json:"coverage_concentration"` // K(c)
    Raw              float64 `json:"raw"`                // σ(z(c))
}

type ConfidenceRegime string

const (
    RegimeColdStart   ConfidenceRegime = "cold_start"   // |labels| < 50
    RegimePartial     ConfidenceRegime = "partial"       // 50–500
    RegimeCalibrated  ConfidenceRegime = "calibrated"    // ≥ 500
)
```

### 8.2 New package: `pkg/confidence`

```
pkg/confidence/
├── confidence.go      // Score(rec, g, snaps, cfg) → ConfidenceBreakdown
├── factors.go         // E, R, S, T, K implementations
├── aggregate.go       // log-odds + clip
├── calibration.go     // isotonic PAV + model load/save
├── model.go           // ConfidenceBreakdown, Regime
└── confidence_test.go // unit + calibration tests
```

Public API:

```go
// Score computes C(c) and its breakdown for a single recommendation.
func Score(rec controls.ControlRecommendation, g *graph.Graph,
           snaps snapshot.Store, cal *Calibrator, cfg Config) (float64, ConfidenceBreakdown, error)

// ScoreAll annotates recs in place; returns the regime.
func ScoreAll(recs []controls.ControlRecommendation, g *graph.Graph,
              snaps snapshot.Store, cal *Calibrator, cfg Config) ConfidenceRegime

type Calibrator interface {
    Apply(raw float64) float64
    Regime() ConfidenceRegime
    Refit(labeled []LabeledOutcome) error
}
```

### 8.3 Wiring

Modify the existing greedy pass in `pkg/controls/controls.go:76` to accept
an optional `*confidence.Calibrator`. When nil, fall back to the current
`0.85` behaviour for backward compatibility. When non-nil, populate
`Breakdown` and `Regime`.

CLI flags and commands:

- `--confidence=on|off` on `pathcollapse breakpoints` and
  `pathcollapse report`, default `on`
- `--shadow-mode` on `pathcollapse breakpoints` to log raw scores without
  exposing them to analysts during collection
- `pathcollapse confidence status` to inspect label progress and any saved
  calibrator
- `pathcollapse confidence refit` to fit and persist the isotonic calibrator

### 8.4 Reporting

Update `pkg/reporting` HTML and Markdown templates:

- Main table: add a `Why?` column rendering the three lowest factor scores
  (e.g. *"Low safety (0.22): `EdgeAdminTo` on tier-1 server, likely
  legitimate"*).
- Executive summary: aggregate the confidence distribution — "40% of
  recommendations are high-confidence (≥ 0.8), 35% medium (0.5–0.8),
  25% low (< 0.5)". A CISO can skim this; today they cannot.

### 8.5 Persistence

Store per-recommendation breakdowns in the SQLite snapshot DB so that the
drift engine can track confidence over time — a recommendation whose
confidence is falling across weekly snapshots is a signal that the
environment is shifting around the control, worth surfacing.

---

## 9. Limitations and future work

1. **Regression label depends on telemetry we don't yet collect.** Until
   `pkg/telemetry` wires in authentication-failure monitoring, `Y` is
   collapse-only. The model will be over-confident on changes that
   technically collapse the path but break production — a known systematic
   bias that documentation must flag to operators.

2. **Priors in §5.1 are unvalidated.** The β weights are informed guesses,
   not derived from data. First deployments should run in shadow mode for
   2–4 weeks collecting labeled outcomes before trusting `C(c)` over
   analyst judgment.

3. **Structural robustness assumes deterministic path search.** If
   `FindPaths` is changed to a sampling-based exploration on denser graphs
   (100k+ nodes), `R(e, G)` becomes a noisy estimate and needs variance
   reporting. Add `R_stderr` in that regime.

4. **No node-level changes.** The factor definitions here cover
   edge-targeted `ControlChange` variants (`ChangeRemoveEdge`,
   `ChangeRemoveMember`). For `ChangeDisableAcct` and `ChangeModifyNode`,
   `R` and `S` generalize by replacing `G \ e` with `G \ (all edges
   touching n)`; evidence quality becomes per-node. Left for a follow-up.

5. **Adversarial calibration.** If recommendations themselves are visible
   to an attacker with partial graph control, they can corrupt `T(e)` by
   strategically ageing edges. This is a Byzantine setting outside the
   defensive-analytics scope declared in [README.md:11][pc-readme], but
   worth flagging in SECURITY.md.

6. **Concurrent recommendations interact.** The greedy loop picks
   breakpoints one at a time; confidence is computed per-rec in isolation.
   A joint-confidence framework (given top-K changes applied together, what
   is the probability the full set collapses the top-N paths?) is the right
   long-term answer and reduces to a correlated Bernoulli estimate. Out of
   scope here.

---

## 10. Related work

**Graph-based attack path analysis** is anchored by BloodHound / SharpHound
[1] and the academic lineage going back to attack graphs [2]. These systems
rank paths but do not recommend minimal control sets; PathCollapse's
set-cover formulation is closer to the defensive-modification literature
[3].

**Probability calibration** in security ML is covered by Platt [4] and
Niculescu-Mizil & Caruana [5]; isotonic regression via PAV [6] is the
standard non-parametric recalibration choice when the base model is already
approximately monotone.

**Multi-factor decomposition** of a final probability mirrors CVSS [7] in
spirit, though CVSS is famously uncalibrated and explicitly not a
probability; our use of logit aggregation + isotonic calibration is
precisely the step CVSS declines to take.

**Uncertainty quantification in graph algorithms** for security intersects
with node2vec embedding confidence [8] and graph-ML-based risk scoring [9];
we deliberately avoid learned graph embeddings in this first version to
preserve interpretability (D1).

---

## References

[1] Robbins, A. et al. *BloodHound: Six Degrees of Domain Admin.*
DEF CON 2016 and ongoing. https://bloodhound.specterops.io

[2] Sheyner, O. et al. *Automated generation and analysis of attack graphs.*
IEEE S&P, 2002.

[3] Jha, S., Sheyner, O., Wing, J. *Two formal analyses of attack graphs.*
CSFW, 2002.

[4] Platt, J. *Probabilistic outputs for support vector machines.*
Adv. in Large Margin Classifiers, 1999.

[5] Niculescu-Mizil, A., Caruana, R. *Predicting good probabilities with
supervised learning.* ICML, 2005.

[6] Barlow, R. E. et al. *Statistical Inference under Order Restrictions.*
Wiley, 1972. (Pool-Adjacent-Violators.)

[7] FIRST.org. *Common Vulnerability Scoring System v3.1.*
https://www.first.org/cvss

[8] Grover, A., Leskovec, J. *node2vec: Scalable feature learning for
networks.* KDD, 2016.

[9] Xu, K. et al. *How powerful are graph neural networks?* ICLR, 2019.

---

## Appendix A — Worked example

Consider a single recommendation from the benchmark fixture:

```
Change: remove_member "USER-usr-0" from "Domain Admins"
PathsRemoved: 5
AffectedPaths: [5 paths all of form usr-0 → domain-admins → dc-{0..4}]
```

Per-factor computation:

- `E(e)`: ingest confidence 1.0, single source (BloodHound CSV),
  no preconditions. `E = 1.0^0.5 · 0.05^0.3 · 1.0^0.2 = 0.4`.
- `R(e, G)`: for each of the 5 paths, is there an alternative from
  `usr-0` to `dc-i` in `G \ e`? In the fixture, `usr-0` has other
  `MemberOf` edges. Suppose 2 of 5 residual paths score ≥ τ; then
  `R = (1 + 1 + 0 + 1 + 1)/5 = 0.8`.
- `S(e, G)`: `usr-0` is tier-2, Domain Admins is tier-1. `s_tier = 0.9`.
  Edge type `MemberOf`, `s_type = 0.7` (often legitimate but this is
  DA). `s_blast = 1 − 0.6 = 0.4`. `S = 0.4·0.9 + 0.4·0.7 + 0.2·0.4 = 0.72`.
- `T(e)`: edge present in all 8 snapshots, age 120 days. `presence = 1.0`,
  `age_factor = 1 − e⁻⁴ ≈ 0.98`. `T = 0.6·1.0 + 0.4·0.98 = 0.992`.
- `K(c)`: this specific `MemberOf` edge is uniquely listed in the
  candidate set for these 5 paths (no other edge covers the exact same
  set). `K = 1.0`.

Aggregation with priors:

```
ℓ(0.4)   = −0.405
ℓ(0.8)   =  1.386
ℓ(0.72)  =  0.944
ℓ(0.95)  =  2.944    // T clipped from 0.992 to 1−ε = 0.95
ℓ(0.95)  =  2.944    // K clipped from 1.0 to 1−ε = 0.95
z = 0 + 1.2·(-0.405) + 1.6·1.386 + 1.1·0.944 + 0.6·2.944 + 0.8·2.944
  = -0.486 + 2.218 + 1.038 + 1.766 + 2.355
  = 6.891
σ(6.891) ≈ 0.999
```

Pre-calibration `C_raw ≈ 0.999` — still aggressive, but now driven by the
joint evidence of four strong factors (R, S, T, K) rather than a single
saturating one. Isotonic calibration will pull this back to a realistic
ceiling (probably `~0.92` after ≥ 500 labeled examples).

Under the earlier `ε = 10⁻³` setting, the same inputs produced
`C_raw ≈ 0.99999` entirely because `K = 1.0` clipped to `0.999` contributed
`0.8·6.9 = 5.5` by itself — a single factor overwhelming the other four.
The revised `ε = 0.05` caps that contribution at `0.8·2.94 ≈ 2.36`, still
the largest term in this example but no longer a structural override.

The recommendation has strong structural backing, a long-lived edge, unique
coverage, and a tier mismatch that argues against legitimate use. The only
weak factor is single-source evidence, but the downstream structural
confirmation compensates.

Contrast with the legacy `0.85`: the breakdown tells us *why* we believe
the recommendation. If a subsequent revision of the ingest pipeline picks
up a second evidence source, `E` rises, `C_raw` rises, and the analyst sees
the upgrade. The `0.85` constant is mute.

---

[pc-controls]: ../pkg/controls/controls.go
[pc-detection]: ../pkg/detection/detection.go
[pc-readme]: ../README.md
[pc-conf-pkg]: ../pkg/confidence
[pc-conf-test]: ../pkg/confidence/confidence_test.go
