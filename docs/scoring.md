# PathCollapse Risk Scoring

## Formula

```
RiskScore = (TargetCriticality × w_tc)
          + (AvgConfidence × w_conf)
          + (AvgExploitability × w_exploit)
          + ((1 - AvgDetectability) × w_detect)
          + (AvgBlastRadius × w_blast)
```

All values are in [0, 1]. The final score is clamped to [0, 1].

## Default Weights

| Factor | Default Weight | Meaning |
|--------|---------------|---------|
| TargetCriticality | 0.30 | How sensitive is the destination node |
| Confidence | 0.20 | How reliable is the path evidence |
| Exploitability | 0.20 | How easy is each hop to execute |
| (1 − Detectability) | 0.15 | How unlikely is the path to be detected |
| BlastRadius | 0.15 | How much damage if the path is exercised |

Weights sum to 1.0. Adjust via `ScoringConfig`:

```go
cfg := scoring.ScoringConfig{
    TargetCriticalityWeight: 0.40,
    ConfidenceWeight:        0.20,
    ExploitabilityWeight:    0.20,
    DetectabilityWeight:     0.10,
    BlastRadiusWeight:       0.10,
}
scored := scoring.RankPaths(paths, g, cfg)
```

## TargetCriticality Derivation

| Node Tag / Type | Criticality |
|-----------------|-------------|
| `tier0` | 1.0 |
| `tier1` | 0.7 |
| `tier2` | 0.4 |
| `certificate_authority`, `service_account` | 0.8 |
| `group` | 0.6 |
| `computer` | 0.5 |
| Other | 0.3 |

Tag-based criticality takes priority over type-based.

## Edge Attribute Guidelines

When ingesting or enriching edges, use these ranges:

| Attribute | Low (0.0–0.3) | Medium (0.4–0.6) | High (0.7–1.0) |
|-----------|--------------|-----------------|---------------|
| Confidence | Inferred/low evidence | Single source | Multiple corroborating sources |
| Exploitability | Requires complex prerequisites | Moderate skill | Trivial to exploit |
| Detectability | No logging | Partial coverage | Full SIEM coverage |
| BlastRadius | Affects one system | Affects a team | Affects the domain |

## Path Aggregation

For multi-hop paths, per-factor values are averaged across all edges. This means:
- A long path with one highly-exploitable edge averages down
- Short direct paths often score highest for Exploitability

To prioritize shorter paths, increase `ExploitabilityWeight` or reduce `MaxDepth` in `PathOptions`.
