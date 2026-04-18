// Package drift compares two graph snapshots and reports identity exposure changes.
package drift

import (
	"time"

	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/model"
)

// DriftCategory classifies a detected change.
type DriftCategory string

const (
	DriftNewPrivilegedMembership DriftCategory = "new_privileged_membership"
	DriftDangerousDelegation     DriftCategory = "dangerous_delegation"
	DriftTrustExpansion          DriftCategory = "trust_expansion"
	DriftExposedServiceAccount   DriftCategory = "exposed_service_account"
	DriftCertTemplateChange      DriftCategory = "cert_template_change"
	DriftTieringRegression       DriftCategory = "tiering_regression"
	DriftDetectionRegression     DriftCategory = "detection_regression"
)

// DriftItem represents a single detected change between snapshots.
type DriftItem struct {
	Category    DriftCategory `json:"category"`
	Description string        `json:"description"`
	NodeID      string        `json:"node_id,omitempty"`
	EdgeID      string        `json:"edge_id,omitempty"`
	Severity    string        `json:"severity"` // high, medium, low
}

// DriftReport holds all changes detected between two graph snapshots.
type DriftReport struct {
	OldSnapshotAt time.Time   `json:"old_snapshot_at"`
	NewSnapshotAt time.Time   `json:"new_snapshot_at"`
	Items         []DriftItem `json:"items"`
	NodesAdded    int         `json:"nodes_added"`
	NodesRemoved  int         `json:"nodes_removed"`
	EdgesAdded    int         `json:"edges_added"`
	EdgesRemoved  int         `json:"edges_removed"`
}

// CompareSnapshots detects changes between oldG and newG.
// oldAt and newAt should be the collection times of each snapshot; pass
// time.Time{} if unknown — the zero value is at least honest rather than
// setting both timestamps to the same time.Now() call.
func CompareSnapshots(oldG, newG *graph.Graph, oldAt, newAt time.Time) *DriftReport {
	rep := &DriftReport{
		OldSnapshotAt: oldAt,
		NewSnapshotAt: newAt,
	}

	// Build node/edge ID sets for both snapshots.
	oldNodes := nodeSet(oldG)
	newNodes := nodeSet(newG)
	oldEdges := edgeMap(oldG)
	newEdges := edgeMap(newG)

	for id := range newNodes {
		if !oldNodes[id] {
			rep.NodesAdded++
		}
	}
	for id := range oldNodes {
		if !newNodes[id] {
			rep.NodesRemoved++
		}
	}

	for id, e := range newEdges {
		if _, existed := oldEdges[id]; !existed {
			rep.EdgesAdded++
			item := classifyNewEdge(e, newG)
			if item != nil {
				rep.Items = append(rep.Items, *item)
			}
		}
	}
	for id := range oldEdges {
		if _, exists := newEdges[id]; !exists {
			rep.EdgesRemoved++
		}
	}

	return rep
}

func classifyNewEdge(e *model.Edge, g *graph.Graph) *DriftItem {
	switch e.Type {
	case model.EdgeMemberOf:
		tgt := g.GetNode(e.Target)
		if tgt != nil && tgt.HasTag(model.TagTier0) {
			return &DriftItem{
				Category:    DriftNewPrivilegedMembership,
				Description: "New membership in tier-0 group detected",
				EdgeID:      e.ID,
				Severity:    "high",
			}
		}
	case model.EdgeCanDelegateTo:
		return &DriftItem{
			Category:    DriftDangerousDelegation,
			Description: "New unconstrained delegation relationship detected",
			EdgeID:      e.ID,
			Severity:    "high",
		}
	case model.EdgeTrustedBy:
		return &DriftItem{
			Category:    DriftTrustExpansion,
			Description: "New trust relationship detected",
			EdgeID:      e.ID,
			Severity:    "medium",
		}
	case model.EdgeCanEnrollIn:
		return &DriftItem{
			Category:    DriftCertTemplateChange,
			Description: "New certificate template enrollment right detected",
			EdgeID:      e.ID,
			Severity:    "medium",
		}
	}
	return nil
}

func nodeSet(g *graph.Graph) map[string]bool {
	nodes := g.Nodes()
	s := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		s[n.ID] = true
	}
	return s
}

func edgeMap(g *graph.Graph) map[string]*model.Edge {
	edges := g.Edges()
	m := make(map[string]*model.Edge, len(edges))
	for _, e := range edges {
		m[e.ID] = e
	}
	return m
}
