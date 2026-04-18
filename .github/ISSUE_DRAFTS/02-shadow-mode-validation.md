# Shadow-mode data collection and calibrator refit command

<!--
Draft issue body. Open with:
    gh issue create --repo karunapuram/pathcollapse \
      --title "Shadow-mode data collection and calibrator refit command" \
      --body-file .github/ISSUE_DRAFTS/02-shadow-mode-validation.md
-->

## Problem

The calibrated confidence algorithm ([docs/confidence.md](../../docs/confidence.md))
ships with informed-prior β weights and an identity calibrator (cold-start
regime). Without labeled post-change outcomes, the score cannot be
validated against Brier / ECE / reliability diagrams (§7), and the isotonic
calibrator cannot be fit.

The paper's §9.2 explicitly flags this: "priors in §5.1 are unvalidated.
First deployments should run in shadow mode for 2–4 weeks collecting
labeled outcomes before trusting `C(c)` over analyst judgment."

This issue tracks the shadow-mode harness.

## Proposal

### Phase 1 — data collection

Add a `--shadow-mode` flag to `breakpoints` that:

1. Computes confidence exactly as today.
2. Does NOT display the calibrated score to the user (shows `LegacyConfidence` or hides the field), so analyst decisions aren't biased by an unvalidated score.
3. Appends a JSONL record to `~/.pathcollapse/shadow.jsonl` per recommendation:

```json
{"ts":"2026-04-18T18:00:00Z","edge_id":"e1","raw":0.87,"breakdown":{"E":0.41,"R":1.0,"S":0.48,"T":0.3,"K":1.0},"rec_applied":null,"observed_collapsed":null,"observed_regression":null}
```

Operators later annotate `rec_applied`, `observed_collapsed`, and
`observed_regression` (collapse check via re-ingest; regression check from
auth logs per §7.1) to produce labeled rows.

### Phase 2 — refit command

Add `pathcollapse confidence refit`:

1. Reads `~/.pathcollapse/shadow.jsonl`.
2. Discards rows where both label fields are null.
3. Computes `Y = observed_collapsed AND observed_regression == false`.
4. Fits `confidence.IsotonicCalibrator` on `(raw, Y)` pairs.
5. Writes the fitted calibrator to `~/.pathcollapse/calibrator.json`.
6. Prints Brier, ECE, reliability bucket summary, and the chosen regime.

Subsequent `breakpoints`/`report` runs load `calibrator.json` when
`--confidence on` and use it instead of `IdentityCalibrator`.

### Phase 3 — learned-weight refit (optional, later)

After ≥500 labeled rows, fit the β aggregation weights themselves via
logistic regression, replacing the informed priors in §5.1. Ship as
`pathcollapse confidence refit --refit-weights`. Gated behind an
explicit flag so the default stays reproducible.

## Scope / non-scope

**In scope:** JSONL logging, refit command, calibrator persistence.

**Out of scope for this issue:**
- Regression-check auth-log ingestion (depends on `pkg/telemetry` growing
  a collector — separate issue).
- Cross-organization calibration sharing.
- Any UI / dashboard for reviewing shadow-mode rows.

## Success criteria

- A fresh deployment can run shadow mode for ≥2 weeks, produce ≥50
  labeled rows, refit, and move from `cold_start` to `partial` regime.
- `ECE < 0.05` on the held-out 20% of rows after refit at ≥500 examples.

## References

- [docs/confidence.md](../../docs/confidence.md) §7 (Evaluation protocol)
- [docs/confidence.md](../../docs/confidence.md) §9.1–9.2 (known limitations)
- [pkg/confidence/calibrator.go](../../pkg/confidence/calibrator.go) — `IsotonicCalibrator.Fit` already implemented; this issue wires a command around it.
