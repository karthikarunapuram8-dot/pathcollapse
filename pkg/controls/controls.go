// Package controls implements the breakpoint optimizer: given a ranked list of
// high-risk paths, it uses a greedy set-cover algorithm to select the minimal
// set of control changes that collapse the most paths.
package controls

import (
	"fmt"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/confidence"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/scoring"
)

// ControlChange describes a single defensive action.
type ControlChange struct {
	Type        ChangeType `json:"type"`
	Description string     `json:"description"`
	// Edge or node targeted by the change.
	EdgeID string `json:"edge_id,omitempty"`
	NodeID string `json:"node_id,omitempty"`
}

// ChangeType classifies the kind of defensive change.
type ChangeType string

const (
	ChangeRemoveEdge   ChangeType = "remove_edge"
	ChangeModifyNode   ChangeType = "modify_node"
	ChangeRemoveMember ChangeType = "remove_member"
	ChangeDisableAcct  ChangeType = "disable_account"
)

// ControlRecommendation is a single recommended control change with its impact.
type ControlRecommendation struct {
	Change        ControlChange `json:"change"`
	PathsRemoved  int           `json:"paths_removed"`
	RiskReduction float64       `json:"risk_reduction"`
	Difficulty    Difficulty    `json:"difficulty"`
	Confidence    float64       `json:"confidence"`

	// Breakdown is the per-factor decomposition from pkg/confidence.
	// Nil when OptimizerConfig.Confidence is unset (legacy 0.85 mode).
	Breakdown *confidence.Breakdown `json:"confidence_breakdown,omitempty"`

	// Regime reports how much labeled data the active calibrator was fit
	// against. Empty string when Breakdown is nil.
	Regime confidence.Regime `json:"confidence_regime,omitempty"`

	AffectedPaths []graph.Path `json:"-"`
}

// Difficulty rates the operational effort to apply a control.
type Difficulty string

const (
	DifficultyLow    Difficulty = "low"
	DifficultyMedium Difficulty = "medium"
	DifficultyHigh   Difficulty = "high"
)

// OptimizerConfig tunes the breakpoint optimizer behaviour.
type OptimizerConfig struct {
	// MaxRecommendations caps the output list.
	MaxRecommendations int
	// MinPathsToQualify is the minimum paths a control must remove to appear.
	MinPathsToQualify int

	// Confidence, when non-nil, enables the five-factor calibrated
	// confidence algorithm (see pkg/confidence and docs/confidence.md).
	// When nil, recommendations fall back to the legacy static 0.85 value
	// for backward compatibility.
	Confidence *ConfidenceOptions
}

// ConfidenceOptions configures confidence scoring inside the optimizer.
// All fields are optional; zero values yield cold-start defaults.
type ConfidenceOptions struct {
	// ScoringCfg is reused for residual-path risk scoring in R(e, G).
	// Zero value uses scoring.DefaultConfig().
	ScoringCfg scoring.ScoringConfig

	// Snapshots supplies edge-presence data for T(e). Nil is permitted
	// and yields the cold-start 0.5 presence prior.
	Snapshots confidence.SnapshotProvider

	// Calibrator maps raw scores to calibrated probabilities. Nil yields
	// the identity map (cold-start regime).
	Calibrator confidence.Calibrator

	// Config holds aggregation weights and factor parameters. Zero value
	// uses confidence.DefaultConfig().
	Config confidence.Config
}

// DefaultOptimizerConfig returns sensible defaults. Confidence remains
// disabled — callers that want calibrated scores must set it explicitly.
func DefaultOptimizerConfig() OptimizerConfig {
	return OptimizerConfig{
		MaxRecommendations: 20,
		MinPathsToQualify:  1,
	}
}

// LegacyConfidence is the hardcoded value used when OptimizerConfig.Confidence
// is nil. Retained as a named constant so tests and reports can reference it.
const LegacyConfidence = 0.85

// Optimize runs the greedy set-cover algorithm over scored paths and returns
// a ranked list of control recommendations.
//
// Performance notes (vs. prior map[int]bool implementation):
//   - pathIdxs uses []int instead of map[int]bool — tighter memory, no per-entry allocs.
//   - counts[edgeID] is precomputed and updated incrementally after each pick;
//     the inner loop never re-scans all paths for a candidate.
//   - pathEdges[i] caches the edge IDs in path i for O(edges-per-path) count updates.
func Optimize(scored []scoring.ScoredPath, g *graph.Graph, cfg OptimizerConfig) []ControlRecommendation {
	if len(scored) == 0 {
		return nil
	}

	type candidate struct {
		change   ControlChange
		pathIdxs []int // indices into scored; simple paths guarantee no duplicates
	}

	candidates := make(map[string]*candidate)
	// pathEdges[i] holds the edge IDs contained in scored[i].Path.
	// Precomputed so the count-update step is O(edges-per-path) not O(all-paths).
	pathEdges := make([][]string, len(scored))

	for i, sp := range scored {
		for _, e := range sp.Path.Edges {
			c, ok := candidates[e.ID]
			if !ok {
				c = &candidate{change: changeForEdge(e, g)}
				candidates[e.ID] = c
			}
			c.pathIdxs = append(c.pathIdxs, i)
			pathEdges[i] = append(pathEdges[i], e.ID)
		}
	}

	// Build a CandidateIndex for K(c) if confidence is enabled. Each
	// candidate's pathIdxs is naturally sorted ascending because i advances
	// monotonically in the scan above.
	var candIdx *confidence.CandidateIndex
	if cfg.Confidence != nil {
		candIdx = confidence.NewCandidateIndex()
		for edgeID, c := range candidates {
			candIdx.Register(edgeID, c.pathIdxs)
		}
	}

	// counts[edgeID] = number of currently uncovered paths that contain this edge.
	// Initialised to len(pathIdxs); decremented incrementally after each pick.
	counts := make(map[string]int, len(candidates))
	for eid, c := range candidates {
		counts[eid] = len(c.pathIdxs)
	}

	covered := make([]bool, len(scored))
	var recommendations []ControlRecommendation

	for len(recommendations) < cfg.MaxRecommendations {
		// O(E) scan — but each lookup is now O(1).
		bestKey := ""
		bestCount := 0
		for key, cnt := range counts {
			if cnt > bestCount {
				bestCount = cnt
				bestKey = key
			}
		}
		if bestKey == "" || bestCount < cfg.MinPathsToQualify {
			break
		}

		best := candidates[bestKey]
		var affectedPaths []graph.Path
		var riskReduced float64

		for _, idx := range best.pathIdxs {
			if covered[idx] {
				continue
			}
			covered[idx] = true
			affectedPaths = append(affectedPaths, scored[idx].Path)
			riskReduced += scored[idx].Score
			// Decrement counts for sibling edges in this newly-covered path.
			for _, eid := range pathEdges[idx] {
				if eid != bestKey {
					counts[eid]--
				}
			}
		}

		recommendations = append(recommendations, ControlRecommendation{
			Change:        best.change,
			PathsRemoved:  bestCount,
			RiskReduction: riskReduced,
			Difficulty:    difficultyForChange(best.change.Type),
			Confidence:    LegacyConfidence, // overwritten below if confidence is enabled
			AffectedPaths: affectedPaths,
		})

		delete(candidates, bestKey)
		delete(counts, bestKey)
	}

	if cfg.Confidence != nil {
		scoreConfidence(recommendations, g, candIdx, cfg.Confidence)
	}

	return recommendations
}

// scoreConfidence annotates each recommendation with its calibrated
// confidence and per-factor breakdown. Edge-not-found or scoring errors
// leave the legacy LegacyConfidence value intact so the optimizer output
// stays well-formed even if one rec fails.
func scoreConfidence(
	recs []ControlRecommendation,
	g *graph.Graph,
	candIdx *confidence.CandidateIndex,
	opts *ConfidenceOptions,
) {
	scoringCfg := opts.ScoringCfg
	if scoringCfg == (scoring.ScoringConfig{}) {
		scoringCfg = scoring.DefaultConfig()
	}
	confCfg := opts.Config
	if confCfg == (confidence.Config{}) {
		confCfg = confidence.DefaultConfig()
	}
	deps := confidence.Deps{
		Graph:      g,
		ScoringCfg: scoringCfg,
		Snapshots:  opts.Snapshots,
		Calibrator: opts.Calibrator,
	}

	for i := range recs {
		edgeID := recs[i].Change.EdgeID
		if edgeID == "" {
			continue
		}
		edge := g.GetEdge(edgeID)
		if edge == nil {
			continue
		}
		final, b, regime, err := confidence.ScoreEdge(
			confidence.ScoreEdgeInput{
				Edge:           edge,
				AffectedPaths:  recs[i].AffectedPaths,
				CandidateIndex: candIdx,
			},
			deps,
			confCfg,
		)
		if err != nil {
			continue
		}
		recs[i].Confidence = final
		bCopy := b
		recs[i].Breakdown = &bCopy
		recs[i].Regime = regime
	}
}

func changeForEdge(e *model.Edge, g *graph.Graph) ControlChange {
	srcNode := g.GetNode(e.Source)
	tgtNode := g.GetNode(e.Target)

	srcName := e.Source
	if srcNode != nil {
		srcName = srcNode.Name
	}
	tgtName := e.Target
	if tgtNode != nil {
		tgtName = tgtNode.Name
	}

	switch e.Type {
	case model.EdgeMemberOf:
		return ControlChange{
			Type:        ChangeRemoveMember,
			Description: fmt.Sprintf("Remove %s from group %s", srcName, tgtName),
			EdgeID:      e.ID,
		}
	case model.EdgeAdminTo, model.EdgeLocalAdminTo:
		return ControlChange{
			Type:        ChangeRemoveEdge,
			Description: fmt.Sprintf("Revoke admin rights of %s over %s", srcName, tgtName),
			EdgeID:      e.ID,
		}
	case model.EdgeCanDelegateTo, model.EdgeCanSyncTo:
		return ControlChange{
			Type:        ChangeModifyNode,
			Description: fmt.Sprintf("Disable delegation/sync from %s to %s", srcName, tgtName),
			EdgeID:      e.ID,
			NodeID:      e.Source,
		}
	case model.EdgeCanEnrollIn:
		return ControlChange{
			Type:        ChangeModifyNode,
			Description: fmt.Sprintf("Restrict enrollment ACL on certificate template %s", tgtName),
			EdgeID:      e.ID,
			NodeID:      e.Target,
		}
	default:
		return ControlChange{
			Type:        ChangeRemoveEdge,
			Description: fmt.Sprintf("Remove %s relationship from %s to %s", e.Type, srcName, tgtName),
			EdgeID:      e.ID,
		}
	}
}

func difficultyForChange(ct ChangeType) Difficulty {
	switch ct {
	case ChangeRemoveMember:
		return DifficultyLow
	case ChangeDisableAcct:
		return DifficultyLow
	case ChangeRemoveEdge:
		return DifficultyMedium
	default:
		return DifficultyHigh
	}
}
