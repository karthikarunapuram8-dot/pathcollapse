# Calibrated Breakpoint Confidence — Short Abstract

*Companion to [confidence.md](confidence.md). ~250 words; suitable for a blog post lede, conference submission, or README section.*

---

## The problem

Identity-graph tools that recommend control changes — BloodHound-style
minimal-breakpoint optimizers, PathCollapse, ACL-hardening engines — routinely
report a static confidence value on every recommendation. PathCollapse
historically emitted `0.85` on all outputs. The constant carries no signal:
an analyst cannot distinguish a robust recommendation from one that merely
satisfies the optimizer's cover set without actually reducing exposure.

## The algorithm

We define a recommendation's confidence as the predicted probability that
applying it will (a) collapse every targeted attack path below a risk
threshold, and (b) not regress any legitimate authentication flow.

The score decomposes into five independent, graph-local factors:

- **Evidence quality** — ingest confidence, source corroboration, precondition health
- **Structural robustness** — residual-path search on `G \ e` to verify the exposure actually collapses
- **Operational safety** — tier alignment, edge-type prior, inverse blast radius
- **Temporal stability** — snapshot-presence rate and age
- **Coverage concentration** — uniqueness of the recommendation vs interchangeable candidates

Factors combine additively in log-odds space with weights β that are initially
informed priors, then refit via **isotonic regression** (Pool-Adjacent-Violators)
against labeled post-change outcomes. Three **regimes** — cold-start, partial,
calibrated — gate how prominently the score is surfaced as training data
accumulates.

## What you get

Output is a probability interpretable as a frequentist success rate, plus an
explainable per-factor breakdown that drives a *Why?* column in the report.
Computation fits inline in the existing greedy set-cover optimizer at an added
~7 ms per run on 10k-node enterprise graphs — opt-in, bounded, no change to
asymptotic complexity. Implementation: [pkg/confidence](../pkg/confidence).
