package normalize

import (
	"testing"
	"time"

	"github.com/karunapuram/pathcollapse/pkg/ingest"
	"github.com/karunapuram/pathcollapse/pkg/model"
)

func TestNormalize_DeduplicatesNodesByID(t *testing.T) {
	now := time.Now().UTC()
	raw := &ingest.Result{
		Nodes: []*model.Node{
			{ID: "u1", Type: model.NodeUser, Name: "Alice", Attributes: map[string]any{"dept": "eng"}, FirstSeen: now, LastSeen: now},
			{ID: "u1", Type: model.NodeUser, Name: "Alice", Attributes: map[string]any{"dept": "security"}, FirstSeen: now, LastSeen: now.Add(time.Hour)},
		},
	}
	res := Normalize(raw)
	if len(res.Nodes) != 1 {
		t.Fatalf("expected 1 node after dedup, got %d", len(res.Nodes))
	}
	// Later attribute wins (last-write).
	if res.Nodes[0].Attributes["dept"] != "security" {
		t.Errorf("expected merged dept=security, got %v", res.Nodes[0].Attributes["dept"])
	}
	// LastSeen should be the later timestamp.
	if !res.Nodes[0].LastSeen.After(now) {
		t.Errorf("LastSeen should be updated to later time")
	}
}

func TestNormalize_MergesTagsOnDuplicate(t *testing.T) {
	now := time.Now().UTC()
	raw := &ingest.Result{
		Nodes: []*model.Node{
			{ID: "u1", Type: model.NodeUser, Name: "Bob", Attributes: make(map[string]any), Tags: []string{"tier1"}, FirstSeen: now, LastSeen: now},
			{ID: "u1", Type: model.NodeUser, Name: "Bob", Attributes: make(map[string]any), Tags: []string{"tier1", "vip"}, FirstSeen: now, LastSeen: now},
		},
	}
	res := Normalize(raw)
	if len(res.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(res.Nodes))
	}
	tagSet := make(map[string]bool)
	for _, tg := range res.Nodes[0].Tags {
		tagSet[tg] = true
	}
	if !tagSet["tier1"] || !tagSet["vip"] {
		t.Errorf("expected both tags, got %v", res.Nodes[0].Tags)
	}
}

func TestNormalize_DeduplicatesEdgesBySourceTypeTarget(t *testing.T) {
	now := time.Now().UTC()
	raw := &ingest.Result{
		Edges: []*model.Edge{
			{ID: "e1", Type: model.EdgeMemberOf, Source: "u1", Target: "g1", Confidence: 0.7, FirstSeen: now, LastSeen: now},
			{ID: "e2", Type: model.EdgeMemberOf, Source: "u1", Target: "g1", Confidence: 0.9, FirstSeen: now, LastSeen: now},
		},
	}
	res := Normalize(raw)
	if len(res.Edges) != 1 {
		t.Fatalf("expected 1 edge after dedup, got %d", len(res.Edges))
	}
	// Higher confidence wins.
	if res.Edges[0].Confidence != 0.9 {
		t.Errorf("expected confidence 0.9, got %f", res.Edges[0].Confidence)
	}
}

func TestNormalize_DistinctEdgesPreserved(t *testing.T) {
	now := time.Now().UTC()
	raw := &ingest.Result{
		Edges: []*model.Edge{
			{ID: "e1", Type: model.EdgeMemberOf, Source: "u1", Target: "g1", Confidence: 1.0, FirstSeen: now, LastSeen: now},
			{ID: "e2", Type: model.EdgeAdminTo, Source: "u1", Target: "dc1", Confidence: 1.0, FirstSeen: now, LastSeen: now},
		},
	}
	res := Normalize(raw)
	if len(res.Edges) != 2 {
		t.Fatalf("expected 2 distinct edges, got %d", len(res.Edges))
	}
}

func TestNormalize_CanonicalizeEmailName(t *testing.T) {
	now := time.Now().UTC()
	raw := &ingest.Result{
		Nodes: []*model.Node{
			{ID: "u1", Type: model.NodeUser, Name: "Alice@CORP.LOCAL", Attributes: make(map[string]any), FirstSeen: now, LastSeen: now},
		},
	}
	res := Normalize(raw)
	if res.Nodes[0].Name != "alice@corp.local" {
		t.Errorf("email name should be lowercased, got %q", res.Nodes[0].Name)
	}
}

func TestNormalize_OutputIsSortedByID(t *testing.T) {
	now := time.Now().UTC()
	raw := &ingest.Result{
		Nodes: []*model.Node{
			{ID: "zzz", Type: model.NodeUser, Name: "Z", Attributes: make(map[string]any), FirstSeen: now, LastSeen: now},
			{ID: "aaa", Type: model.NodeUser, Name: "A", Attributes: make(map[string]any), FirstSeen: now, LastSeen: now},
			{ID: "mmm", Type: model.NodeUser, Name: "M", Attributes: make(map[string]any), FirstSeen: now, LastSeen: now},
		},
	}
	res := Normalize(raw)
	if len(res.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(res.Nodes))
	}
	if res.Nodes[0].ID != "aaa" || res.Nodes[1].ID != "mmm" || res.Nodes[2].ID != "zzz" {
		t.Errorf("nodes not sorted by ID: got %s, %s, %s", res.Nodes[0].ID, res.Nodes[1].ID, res.Nodes[2].ID)
	}
}

func TestNormalize_PropagatesWarns(t *testing.T) {
	raw := &ingest.Result{Warns: []string{"skipped empty node"}}
	res := Normalize(raw)
	if len(res.Warns) != 1 || res.Warns[0] != "skipped empty node" {
		t.Errorf("expected warn propagated, got %v", res.Warns)
	}
}
