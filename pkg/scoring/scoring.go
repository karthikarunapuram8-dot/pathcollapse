// Package scoring computes risk scores for paths in the identity graph.
// Formula: RiskScore = (TargetCriticality*0.30) + (Confidence*0.20) +
// (Exploitability*0.20) + ((1-Detectability)*0.15) + (BlastRadius*0.15)
package scoring

import (
	"sort"

	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/model"
)

// ScoringConfig holds tunable weights that must sum to 1.0.
type ScoringConfig struct {
	TargetCriticalityWeight float64
	ConfidenceWeight        float64
	ExploitabilityWeight    float64
	DetectabilityWeight     float64 // applied as (1 - detectability)
	BlastRadiusWeight       float64
}

// DefaultConfig returns the canonical weight configuration.
func DefaultConfig() ScoringConfig {
	return ScoringConfig{
		TargetCriticalityWeight: 0.30,
		ConfidenceWeight:        0.20,
		ExploitabilityWeight:    0.20,
		DetectabilityWeight:     0.15,
		BlastRadiusWeight:       0.15,
	}
}

// ScoredPath bundles a path with its computed risk score.
type ScoredPath struct {
	Path      graph.Path
	Score     float64
	Breakdown ScoreBreakdown
}

// ScoreBreakdown exposes the weighted contribution of each factor.
type ScoreBreakdown struct {
	TargetCriticality float64
	Confidence        float64
	Exploitability    float64
	Detectability     float64
	BlastRadius       float64
}

// ScorePath scores a single path.
// TargetCriticality is derived from tags on the terminal node.
func ScorePath(p graph.Path, g interface{ GetNode(string) *model.Node }, cfg ScoringConfig) float64 {
	if len(p.Edges) == 0 {
		return 0
	}

	// Aggregate edge attributes as averages across the path.
	var sumConf, sumExploit, sumDetect, sumBlast float64
	for _, e := range p.Edges {
		sumConf += e.Confidence
		sumExploit += e.Exploitability
		sumDetect += e.Detectability
		sumBlast += e.BlastRadius
	}
	n := float64(len(p.Edges))

	avgConf := sumConf / n
	avgExploit := sumExploit / n
	avgDetect := sumDetect / n
	avgBlast := sumBlast / n

	targetCriticality := targetCriticalityFromNode(p.Target())

	score := targetCriticality*cfg.TargetCriticalityWeight +
		avgConf*cfg.ConfidenceWeight +
		avgExploit*cfg.ExploitabilityWeight +
		(1-avgDetect)*cfg.DetectabilityWeight +
		avgBlast*cfg.BlastRadiusWeight

	return clamp(score)
}

// ScorePathFull returns a ScoredPath including the breakdown.
func ScorePathFull(p graph.Path, g interface{ GetNode(string) *model.Node }, cfg ScoringConfig) ScoredPath {
	if len(p.Edges) == 0 {
		return ScoredPath{Path: p}
	}

	var sumConf, sumExploit, sumDetect, sumBlast float64
	for _, e := range p.Edges {
		sumConf += e.Confidence
		sumExploit += e.Exploitability
		sumDetect += e.Detectability
		sumBlast += e.BlastRadius
	}
	n := float64(len(p.Edges))

	bd := ScoreBreakdown{
		TargetCriticality: targetCriticalityFromNode(p.Target()),
		Confidence:        sumConf / n,
		Exploitability:    sumExploit / n,
		Detectability:     sumDetect / n,
		BlastRadius:       sumBlast / n,
	}

	score := bd.TargetCriticality*cfg.TargetCriticalityWeight +
		bd.Confidence*cfg.ConfidenceWeight +
		bd.Exploitability*cfg.ExploitabilityWeight +
		(1-bd.Detectability)*cfg.DetectabilityWeight +
		bd.BlastRadius*cfg.BlastRadiusWeight

	return ScoredPath{Path: p, Score: clamp(score), Breakdown: bd}
}

// RankPaths scores all paths and returns them sorted descending by risk.
func RankPaths(paths []graph.Path, g interface{ GetNode(string) *model.Node }, cfg ScoringConfig) []ScoredPath {
	scored := make([]ScoredPath, len(paths))
	for i, p := range paths {
		scored[i] = ScorePathFull(p, g, cfg)
	}
	sortScoredPaths(scored)
	return scored
}

func targetCriticalityFromNode(n *model.Node) float64 {
	if n == nil {
		return 0
	}
	if n.HasTag(model.TagTier0) {
		return 1.0
	}
	if n.HasTag(model.TagTier1) {
		return 0.7
	}
	if n.HasTag(model.TagTier2) {
		return 0.4
	}
	// Derive from node type.
	switch n.Type {
	case model.NodeCA, model.NodeServiceAccount:
		return 0.8
	case model.NodeComputer:
		return 0.5
	case model.NodeGroup:
		return 0.6
	default:
		return 0.3
	}
}

func sortScoredPaths(paths []ScoredPath) {
	sort.Slice(paths, func(i, j int) bool {
		if paths[i].Score != paths[j].Score {
			return paths[i].Score > paths[j].Score
		}
		// Stable tie-break: shorter paths rank higher.
		return paths[i].Path.Len() < paths[j].Path.Len()
	})
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
