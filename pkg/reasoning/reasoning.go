// Package reasoning implements three analysis modes for identity graph paths:
// Reachability (path exists?), Plausibility (conditions satisfied?),
// and Defensive (full attacker-realistic + telemetry analysis).
package reasoning

import (
	"sort"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/scoring"
)

// Mode controls the depth of path analysis.
type Mode int

const (
	// ModeReachability checks whether a path exists (no condition evaluation).
	ModeReachability Mode = iota
	// ModePlausibility only traverses edges whose preconditions are satisfied.
	ModePlausibility
	// ModeDefensive performs full realistic analysis including detectability scoring.
	ModeDefensive
)

// PathAnalysis is the result of reasoning over a single path.
type PathAnalysis struct {
	Path               graph.Path
	Score              float64
	Realistic          bool
	AverageConfidence  float64
	AverageDetect      float64
	MissingPreconditions []string
	RemediationHints   []string
}

// Reasoner analyses paths from a graph using a chosen mode.
type Reasoner struct {
	g   *graph.Graph
	cfg scoring.ScoringConfig
}

// New returns a Reasoner bound to the given graph.
func New(g *graph.Graph, cfg scoring.ScoringConfig) *Reasoner {
	return &Reasoner{g: g, cfg: cfg}
}

// AnalysePath runs the given mode over path p.
func (r *Reasoner) AnalysePath(p graph.Path, mode Mode) PathAnalysis {
	switch mode {
	case ModeReachability:
		return r.reachability(p)
	case ModePlausibility:
		return r.plausibility(p)
	default:
		return r.defensive(p)
	}
}

// AnalysePaths returns a ranked list of PathAnalysis for all provided paths.
func (r *Reasoner) AnalysePaths(paths []graph.Path, mode Mode) []PathAnalysis {
	results := make([]PathAnalysis, len(paths))
	for i, p := range paths {
		results[i] = r.AnalysePath(p, mode)
	}
	sortByScore(results)
	return results
}

// FindAndAnalyse finds paths from fromID to toID and analyses them.
func (r *Reasoner) FindAndAnalyse(fromID, toID string, mode Mode, opts graph.PathOptions) []PathAnalysis {
	paths := r.g.FindPaths(fromID, toID, opts)
	return r.AnalysePaths(paths, mode)
}

func (r *Reasoner) reachability(p graph.Path) PathAnalysis {
	return PathAnalysis{
		Path:       p,
		Score:      scoring.ScorePath(p, r.g, r.cfg),
		Realistic:  len(p.Edges) > 0,
	}
}

func (r *Reasoner) plausibility(p graph.Path) PathAnalysis {
	a := r.reachability(p)
	var missing []string
	for _, e := range p.Edges {
		for _, pc := range e.Preconditions {
			if !pc.Satisfied {
				missing = append(missing, pc.Description)
			}
		}
	}
	a.MissingPreconditions = missing
	a.Realistic = len(missing) == 0
	return a
}

func (r *Reasoner) defensive(p graph.Path) PathAnalysis {
	a := r.plausibility(p)
	if len(p.Edges) == 0 {
		return a
	}

	var sumConf, sumDetect float64
	for _, e := range p.Edges {
		sumConf += e.Confidence
		sumDetect += e.Detectability
	}
	n := float64(len(p.Edges))
	a.AverageConfidence = sumConf / n
	a.AverageDetect = sumDetect / n

	// Generate defensive remediation hints per edge type.
	seen := map[model.EdgeType]bool{}
	for _, e := range p.Edges {
		if seen[e.Type] {
			continue
		}
		seen[e.Type] = true
		if hint := remediationHint(e.Type); hint != "" {
			a.RemediationHints = append(a.RemediationHints, hint)
		}
	}

	return a
}

func remediationHint(et model.EdgeType) string {
	hints := map[model.EdgeType]string{
		model.EdgeMemberOf:           "Review group membership and apply least-privilege",
		model.EdgeAdminTo:            "Remove unnecessary domain admin grants",
		model.EdgeLocalAdminTo:       "Audit local admin rights via LAPS or PAW",
		model.EdgeHasSessionOn:       "Implement credential guard and session isolation",
		model.EdgeCanDelegateTo:      "Disable unconstrained delegation; prefer resource-based constrained",
		model.EdgeCanSyncTo:          "Restrict DCSync rights to dedicated accounts",
		model.EdgeCanEnrollIn:        "Restrict certificate template enrollment ACLs",
		model.EdgeCanResetPasswordOf: "Require MFA and approval workflow for password resets",
		model.EdgeCanWriteACLOf:      "Audit and restrict ACL write permissions",
		model.EdgeControlsGPO:        "Limit GPO write access to dedicated change-management accounts",
	}
	return hints[et]
}

func sortByScore(a []PathAnalysis) {
	sort.Slice(a, func(i, j int) bool {
		if a[i].Score != a[j].Score {
			return a[i].Score > a[j].Score
		}
		return a[i].Path.Len() < a[j].Path.Len()
	})
}
