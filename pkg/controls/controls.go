// Package controls implements the breakpoint optimizer: given a ranked list of
// high-risk paths, it uses a greedy set-cover algorithm to select the minimal
// set of control changes that collapse the most paths.
package controls

import (
	"fmt"

	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/model"
	"github.com/karunapuram/pathcollapse/pkg/scoring"
)

// ControlChange describes a single defensive action.
type ControlChange struct {
	Type        ChangeType `json:"type"`
	Description string     `json:"description"`
	// Edge or node targeted by the change.
	EdgeID  string `json:"edge_id,omitempty"`
	NodeID  string `json:"node_id,omitempty"`
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
	Change        ControlChange    `json:"change"`
	PathsRemoved  int              `json:"paths_removed"`
	RiskReduction float64          `json:"risk_reduction"`
	Difficulty    Difficulty       `json:"difficulty"`
	Confidence    float64          `json:"confidence"`
	AffectedPaths []graph.Path     `json:"-"`
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
}

// DefaultOptimizerConfig returns sensible defaults.
func DefaultOptimizerConfig() OptimizerConfig {
	return OptimizerConfig{
		MaxRecommendations: 20,
		MinPathsToQualify:  1,
	}
}

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
			Confidence:    0.85,
			AffectedPaths: affectedPaths,
		})

		delete(candidates, bestKey)
		delete(counts, bestKey)
	}

	return recommendations
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
