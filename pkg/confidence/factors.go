package confidence

import (
	"math"
	"time"

	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/model"
	"github.com/karunapuram/pathcollapse/pkg/scoring"
)

// ── §4.1 Evidence quality E(e) ─────────────────────────────────────────────

// evidenceQuality computes E(e) = ingest_conf^0.5 · src_corroboration^0.3 · precondition_health^0.2
//
// See docs/confidence.md §4.1.
func evidenceQuality(e *model.Edge, cfg Config) float64 {
	const (
		wConf    = 0.5
		wSrc     = 0.3
		wPre     = 0.2
		srcFloor = 0.05 // keeps single-source edges from zeroing out
	)

	ingest := clampUnit(e.Confidence)

	uniqueSources := map[string]struct{}{}
	for _, ev := range e.Evidence {
		if ev.Source != "" {
			uniqueSources[ev.Source] = struct{}{}
		}
	}
	n := float64(len(uniqueSources))
	sSrc := srcFloor
	if n > 1 {
		sSrc = 1 - math.Exp(-cfg.EvidenceSourceLambda*(n-1))
	}

	sPre := 1.0
	if len(e.Preconditions) > 0 {
		satisfied := 0
		for _, pc := range e.Preconditions {
			if pc.Satisfied {
				satisfied++
			}
		}
		sPre = float64(satisfied) / float64(len(e.Preconditions))
	}

	return math.Pow(ingest, wConf) * math.Pow(sSrc, wSrc) * math.Pow(sPre, wPre)
}

// ── §4.2 Structural robustness R(e, G) ─────────────────────────────────────

// structuralRobustness computes R(e, G): for each covered path, does a
// similar-risk alternative exist after removing e?
//
// Uses graph.PathOptions.EdgeFilter to simulate G \ e without mutating g.
// See docs/confidence.md §4.2.
func structuralRobustness(
	e *model.Edge,
	covered []graph.Path,
	g *graph.Graph,
	scoringCfg scoring.ScoringConfig,
	cfg Config,
) float64 {
	if len(covered) == 0 {
		return 0.5 // non-informative prior
	}
	τ := cfg.RobustnessThreshold
	if τ <= 0 {
		τ = 0.6
	}

	filter := func(x *model.Edge) bool { return x.ID != e.ID }

	var total float64
	for _, p := range covered {
		src := p.Source()
		tgt := p.Target()
		if src == nil || tgt == nil {
			// Degenerate input; treat as unrecoverable exposure removed.
			total += 1.0
			continue
		}

		alts := g.FindPaths(src.ID, tgt.ID, graph.PathOptions{
			MaxDepth:   cfg.MaxResidualDepth,
			EdgeFilter: filter,
		})

		// Highest residual risk among surviving paths.
		var maxRisk float64
		for _, alt := range alts {
			s := scoring.ScorePath(alt, g, scoringCfg)
			if s > maxRisk {
				maxRisk = s
			}
		}

		// r(p, e) = 1 − min(maxRisk / τ, 1). High r = edge truly kills path.
		r := 1 - math.Min(maxRisk/τ, 1)
		total += r
	}
	return total / float64(len(covered))
}

// ── §4.3 Operational safety S(e, G) ────────────────────────────────────────

// operationalSafety computes S(e, G) = 0.4·s_tier + 0.4·s_type + 0.2·s_blast.
// See docs/confidence.md §4.3.
func operationalSafety(e *model.Edge, g *graph.Graph) float64 {
	srcTier := nodeTier(g.GetNode(e.Source))
	tgtTier := nodeTier(g.GetNode(e.Target))

	sTier := 0.6 // default when either side is untagged
	switch {
	case srcTier >= 0 && tgtTier >= 0 && srcTier <= tgtTier:
		// Source outranks target (lower tier number = more privileged) → plausibly legit.
		sTier = 0.3
	case srcTier >= 0 && tgtTier >= 0 && srcTier > tgtTier:
		// Source is less privileged than target → suspicious.
		sTier = 0.9
	}

	sType := edgeTypeSafetyPrior[e.Type]
	if sType == 0 {
		sType = 0.5
	}

	sBlast := 1 - clampUnit(e.BlastRadius)

	return 0.4*sTier + 0.4*sType + 0.2*sBlast
}

// edgeTypeSafetyPrior maps EdgeType → safety (higher = safer to sever).
// Calibrated against Microsoft Security advisories and ATT&CK families
// already wired into pkg/detection. Re-fit from data when available.
var edgeTypeSafetyPrior = map[model.EdgeType]float64{
	model.EdgeMemberOf:           0.55, // often legit; depends on group
	model.EdgeAdminTo:            0.75,
	model.EdgeLocalAdminTo:       0.70,
	model.EdgeHasSessionOn:       0.50,
	model.EdgeCanDelegateTo:      0.85, // unconstrained delegation is rarely legit
	model.EdgeCanSyncTo:          0.95, // DCSync from non-DC is almost always abuse
	model.EdgeCanEnrollIn:        0.80,
	model.EdgeCanResetPasswordOf: 0.70,
	model.EdgeCanWriteACLOf:      0.85,
	model.EdgeControlsGPO:        0.80,
	model.EdgeTrustedBy:          0.30, // removing a trust is structurally risky
	model.EdgeAuthenticatesTo:    0.40,
}

// nodeTier returns 0/1/2 for tier0/tier1/tier2, or -1 if no tier tag.
// Lower = more privileged. Matches scoring.targetCriticalityFromNode ordering.
func nodeTier(n *model.Node) int {
	if n == nil {
		return -1
	}
	switch {
	case n.HasTag(model.TagTier0):
		return 0
	case n.HasTag(model.TagTier1):
		return 1
	case n.HasTag(model.TagTier2):
		return 2
	default:
		return -1
	}
}

// ── §4.4 Temporal stability T(e) ───────────────────────────────────────────

// SnapshotProvider is the narrow interface confidence needs from the
// snapshot store — deliberately decoupled from pkg/snapshot to keep tests
// fast and avoid pulling SQLite into this package.
type SnapshotProvider interface {
	// EdgePresence returns the fraction of the most-recent `window` snapshots
	// in which an edge matching (source, target, type) appears.
	// Returns (0.5, false) when fewer than 2 snapshots exist (cold start).
	EdgePresence(source, target string, etype model.EdgeType, window int) (frac float64, ok bool)
}

// temporalStability computes T(e) = 0.6·presence + 0.4·age_factor.
// See docs/confidence.md §4.4.
func temporalStability(e *model.Edge, snaps SnapshotProvider, cfg Config, now time.Time) float64 {
	presence := 0.5
	if snaps != nil {
		if p, ok := snaps.EdgePresence(e.Source, e.Target, e.Type, cfg.TemporalSnapshotWindow); ok {
			presence = p
		}
	}

	var ageDays float64
	if !e.FirstSeen.IsZero() {
		ageDays = now.Sub(e.FirstSeen).Hours() / 24
		if ageDays < 0 {
			ageDays = 0
		}
	}
	halflife := cfg.TemporalHalflifeDays
	if halflife <= 0 {
		halflife = 30
	}
	ageFactor := 1 - math.Exp(-ageDays/halflife)

	return 0.6*presence + 0.4*ageFactor
}

// ── §4.5 Coverage concentration K(c) ───────────────────────────────────────

// CandidateIndex groups candidate edges by the exact set of path indices
// they cover. Build once per optimizer run and reuse across all recs.
//
// See docs/confidence.md §4.5.
type CandidateIndex struct {
	// groups maps an equivalence-class key → list of edge IDs that share it.
	groups map[string][]string
	// edgeToKey lets us look up an edge's class in O(1).
	edgeToKey map[string]string
}

// NewCandidateIndex returns an empty index.
func NewCandidateIndex() *CandidateIndex {
	return &CandidateIndex{
		groups:    map[string][]string{},
		edgeToKey: map[string]string{},
	}
}

// Register records that edgeID covers exactly the given sorted-ascending
// slice of path indices. Callers must sort before calling (cheap, done once
// per edge in the existing candidate scan).
func (ci *CandidateIndex) Register(edgeID string, sortedPathIdxs []int) {
	key := pathIdxKey(sortedPathIdxs)
	ci.groups[key] = append(ci.groups[key], edgeID)
	ci.edgeToKey[edgeID] = key
}

// Equivalents returns the number of edges covering exactly the same path
// set as edgeID, including edgeID itself. Always ≥ 1 for registered edges.
func (ci *CandidateIndex) Equivalents(edgeID string) int {
	key, ok := ci.edgeToKey[edgeID]
	if !ok {
		return 1
	}
	return len(ci.groups[key])
}

// coverageConcentration computes K(c) = 1 / |A(e)|.
func coverageConcentration(edgeID string, ci *CandidateIndex) float64 {
	if ci == nil {
		return 1.0 // no index supplied = assume unique
	}
	n := ci.Equivalents(edgeID)
	if n < 1 {
		return 1.0
	}
	return 1.0 / float64(n)
}

// pathIdxKey produces a compact, deterministic key from sorted path indices.
// Collisions across different index sets are acceptable for approximate
// grouping; callers needing cryptographic-strength hashing can swap in sha256.
func pathIdxKey(sorted []int) string {
	// Fowler–Noll–Vo 64-bit, inlined so we don't depend on hash/fnv at API
	// surface.
	const (
		offset = 1469598103934665603
		prime  = 1099511628211
	)
	h := uint64(offset)
	for _, i := range sorted {
		v := uint64(i)
		for b := 0; b < 8; b++ {
			h ^= v & 0xff
			h *= prime
			v >>= 8
		}
	}
	// Hex-ish via a tiny lookup; strconv would pull fmt for no reason.
	const hex = "0123456789abcdef"
	out := make([]byte, 16)
	for i := 0; i < 16; i++ {
		out[15-i] = hex[h&0xf]
		h >>= 4
	}
	return string(out)
}

// clampUnit bounds x to [0, 1].
func clampUnit(x float64) float64 {
	switch {
	case x < 0:
		return 0
	case x > 1:
		return 1
	default:
		return x
	}
}
