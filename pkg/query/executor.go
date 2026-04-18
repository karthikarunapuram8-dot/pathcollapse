package query

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/controls"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/drift"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/scoring"
)

// Result is the output of executing a DSL statement.
type Result struct {
	Paths           []graph.Path
	ScoredPaths     []scoring.ScoredPath
	Recommendations []controls.ControlRecommendation
	Message         string
}

// Executor runs parsed statements against a graph.
type Executor struct {
	g        *graph.Graph
	cfg      scoring.ScoringConfig
	baseline *graph.Graph // used by SHOW DRIFT
}

// NewExecutor returns an Executor bound to g with the given scoring config.
func NewExecutor(g *graph.Graph, cfg scoring.ScoringConfig) *Executor {
	return &Executor{g: g, cfg: cfg}
}

// SetBaseline provides a previous-snapshot graph used when SHOW DRIFT is executed.
func (ex *Executor) SetBaseline(g *graph.Graph) {
	ex.baseline = g
}

// Execute runs stmt and returns the result.
func (ex *Executor) Execute(stmt *Statement) (*Result, error) {
	switch stmt.Type {
	case StmtFindPaths:
		return ex.execFindPaths(stmt)
	case StmtFindBreakpoints:
		return ex.execFindBreakpoints(stmt)
	case StmtShowDrift:
		return ex.execShowDrift(stmt)
	case StmtFindHighRisk:
		return ex.execFindHighRisk(stmt)
	default:
		return nil, fmt.Errorf("executor: unknown statement type %d", stmt.Type)
	}
}

func (ex *Executor) execFindPaths(stmt *Statement) (*Result, error) {
	fromIDs := ex.resolveRef(stmt.From)
	toIDs := ex.resolveRef(stmt.To)

	if len(fromIDs) == 0 {
		return nil, fmt.Errorf("executor: FROM ref %v resolved to no nodes", stmt.From)
	}
	if len(toIDs) == 0 {
		return nil, fmt.Errorf("executor: TO ref %v resolved to no nodes", stmt.To)
	}

	opts := graph.DefaultPathOptions()
	if len(stmt.Where) > 0 {
		opts.EdgeFilter = buildEdgeFilter(stmt.Where)
		for _, pred := range stmt.Where {
			if pred.Field == "confidence" {
				if v, err := strconv.ParseFloat(pred.Value, 64); err == nil && (pred.Operator == ">" || pred.Operator == ">=") {
					opts.MinConfidence = v
				}
			}
		}
	}

	var allPaths []graph.Path
	for _, fromID := range fromIDs {
		for _, toID := range toIDs {
			paths := ex.g.FindPaths(fromID, toID, opts)
			allPaths = append(allPaths, paths...)
		}
	}

	scored := scoring.RankPaths(allPaths, ex.g, ex.cfg)
	if err := applyOrderBy(scored, stmt.OrderBy, stmt.OrderDesc); err != nil {
		return nil, err
	}
	if stmt.Limit > 0 && len(scored) > stmt.Limit {
		scored = scored[:stmt.Limit]
	}

	paths := make([]graph.Path, len(scored))
	for i, sp := range scored {
		paths[i] = sp.Path
	}
	return &Result{Paths: paths, ScoredPaths: scored}, nil
}

// execFindBreakpoints gathers all paths to tier-0 targets, scores them,
// and runs the greedy set-cover optimizer to produce ControlRecommendations.
func (ex *Executor) execFindBreakpoints(stmt *Statement) (*Result, error) {
	opts := graph.DefaultPathOptions()
	var allPaths []graph.Path
	for _, tgt := range ex.g.Nodes() {
		if !tgt.HasTag(model.TagTier0) {
			continue
		}
		for _, src := range ex.g.Nodes() {
			if src.ID == tgt.ID {
				continue
			}
			paths := ex.g.FindPaths(src.ID, tgt.ID, opts)
			allPaths = append(allPaths, paths...)
		}
	}

	scored := scoring.RankPaths(allPaths, ex.g, ex.cfg)

	optCfg := controls.DefaultOptimizerConfig()
	if stmt.Limit > 0 {
		optCfg.MaxRecommendations = stmt.Limit
	}
	recs := controls.Optimize(scored, ex.g, optCfg)

	return &Result{
		ScoredPaths:     scored,
		Recommendations: recs,
		Message:         fmt.Sprintf("Breakpoint analysis: %d paths scored, %d control recommendations", len(scored), len(recs)),
	}, nil
}

// execShowDrift compares ex.g against the baseline set via SetBaseline.
// If no baseline is set, it returns an instructional message.
func (ex *Executor) execShowDrift(stmt *Statement) (*Result, error) {
	if ex.baseline == nil {
		return &Result{
			Message: fmt.Sprintf(
				"SHOW DRIFT requires a baseline snapshot.\n"+
					"  Use: pathcollapse analyze --baseline <old-snapshot.json> --query %q\n"+
					"  Or:  pathcollapse diff <old.json> <new.json>",
				`SHOW DRIFT SINCE `+stmt.DriftSince,
			),
		}, nil
	}

	rep := drift.CompareSnapshots(ex.baseline, ex.g, time.Time{}, time.Time{})

	var sb strings.Builder
	fmt.Fprintf(&sb, "Drift report (baseline → current):\n\n")
	fmt.Fprintf(&sb, "  Nodes  added: %d  removed: %d\n", rep.NodesAdded, rep.NodesRemoved)
	fmt.Fprintf(&sb, "  Edges  added: %d  removed: %d\n\n", rep.EdgesAdded, rep.EdgesRemoved)

	if len(rep.Items) == 0 {
		fmt.Fprintf(&sb, "  No security-relevant drift detected.\n")
	} else {
		fmt.Fprintf(&sb, "  Security-relevant changes (%d):\n\n", len(rep.Items))
		for i, item := range rep.Items {
			fmt.Fprintf(&sb, "  %d. [%s] %s — %s\n", i+1, item.Severity, item.Category, item.Description)
		}
	}
	return &Result{Message: sb.String()}, nil
}

func (ex *Executor) execFindHighRisk(stmt *Statement) (*Result, error) {
	var matches []graph.Path
	target := stmt.HighRiskTarget

	for _, n := range ex.g.Nodes() {
		if !matchesHighRiskTarget(n, target) {
			continue
		}
		for _, tier0 := range ex.g.Nodes() {
			if !tier0.HasTag(model.TagTier0) {
				continue
			}
			paths := ex.g.FindPaths(n.ID, tier0.ID, graph.DefaultPathOptions())
			matches = append(matches, paths...)
		}
	}

	scored := scoring.RankPaths(matches, ex.g, ex.cfg)
	return &Result{
		ScoredPaths: scored,
		Message:     fmt.Sprintf("High-risk %s paths: %d found", target, len(scored)),
	}, nil
}

func (ex *Executor) resolveRef(ref *Ref) []string {
	if ref == nil {
		return nil
	}
	var ids []string
	for _, n := range ex.g.Nodes() {
		if matchesRef(n, ref) {
			ids = append(ids, n.ID)
		}
	}
	return ids
}

func matchesRef(n *model.Node, ref *Ref) bool {
	if ref.Kind == "" {
		return strings.EqualFold(n.Name, ref.Value) || n.ID == ref.Value
	}
	switch strings.ToLower(ref.Kind) {
	case "user":
		return n.Type == model.NodeUser && (strings.EqualFold(n.Name, ref.Value) || n.ID == ref.Value)
	case "group":
		return n.Type == model.NodeGroup && (strings.EqualFold(n.Name, ref.Value) || n.ID == ref.Value)
	case "computer":
		return n.Type == model.NodeComputer && (strings.EqualFold(n.Name, ref.Value) || n.ID == ref.Value)
	case "service_account":
		return n.Type == model.NodeServiceAccount && (strings.EqualFold(n.Name, ref.Value) || n.ID == ref.Value)
	case "privilege", "tier":
		return n.HasTag(ref.Value)
	default:
		return strings.EqualFold(n.Name, ref.Value) || n.ID == ref.Value
	}
}

func matchesHighRiskTarget(n *model.Node, target string) bool {
	switch target {
	case "high_risk_service_accounts", "service_accounts":
		return n.Type == model.NodeServiceAccount
	case "users":
		return n.Type == model.NodeUser
	case "groups":
		return n.Type == model.NodeGroup
	default:
		return false
	}
}

func buildEdgeFilter(predicates []Predicate) func(*model.Edge) bool {
	return func(e *model.Edge) bool {
		for _, pred := range predicates {
			if !evalPredicate(e, pred) {
				return false
			}
		}
		return true
	}
}

func evalPredicate(e *model.Edge, pred Predicate) bool {
	var fieldVal float64
	switch pred.Field {
	case "confidence":
		fieldVal = e.Confidence
	case "exploitability":
		fieldVal = e.Exploitability
	case "detectability":
		fieldVal = e.Detectability
	case "blast_radius":
		fieldVal = e.BlastRadius
	default:
		return true
	}

	threshold, err := strconv.ParseFloat(pred.Value, 64)
	if err != nil {
		return true
	}

	switch pred.Operator {
	case ">":
		return fieldVal > threshold
	case "<":
		return fieldVal < threshold
	case ">=":
		return fieldVal >= threshold
	case "<=":
		return fieldVal <= threshold
	case "=":
		return fieldVal == threshold
	case "!=":
		return fieldVal != threshold
	default:
		return true
	}
}

func applyOrderBy(scored []scoring.ScoredPath, field string, desc bool) error {
	if field == "" {
		return nil
	}
	if len(scored) == 0 {
		return nil
	}

	valueFor := func(sp scoring.ScoredPath) (float64, error) {
		switch strings.ToLower(field) {
		case "risk":
			return sp.Score, nil
		case "confidence":
			return sp.Breakdown.Confidence, nil
		case "exploitability":
			return sp.Breakdown.Exploitability, nil
		case "detectability":
			return sp.Breakdown.Detectability, nil
		case "blast_radius":
			return sp.Breakdown.BlastRadius, nil
		default:
			return 0, fmt.Errorf("executor: unsupported ORDER BY field %q", field)
		}
	}

	if _, err := valueFor(scored[0]); err != nil {
		return err
	}

	sort.SliceStable(scored, func(i, j int) bool {
		vi, _ := valueFor(scored[i])
		vj, _ := valueFor(scored[j])
		if vi == vj {
			return scored[i].Path.Len() < scored[j].Path.Len()
		}
		if desc {
			return vi > vj
		}
		return vi < vj
	})

	return nil
}
