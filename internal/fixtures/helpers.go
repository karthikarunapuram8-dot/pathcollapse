// Package fixtures provides test helpers for building graphs and asserting paths.
package fixtures

import (
	"fmt"
	"testing"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
)

// BuildLinear creates a graph with n nodes linked in a chain: n0 → n1 → ... → n(n-1).
func BuildLinear(t *testing.T, n int) (*graph.Graph, []string) {
	t.Helper()
	g := graph.New()
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = fmt.Sprintf("node-%d", i)
		if err := g.AddNode(model.NewNode(ids[i], model.NodeUser, ids[i])); err != nil {
			t.Fatalf("AddNode %s: %v", ids[i], err)
		}
	}
	for i := 0; i < n-1; i++ {
		e := model.NewEdge(fmt.Sprintf("edge-%d", i), model.EdgeMemberOf, ids[i], ids[i+1])
		if err := g.AddEdge(e); err != nil {
			t.Fatalf("AddEdge %d: %v", i, err)
		}
	}
	return g, ids
}

// BuildStar creates a graph with a hub node and n spoke nodes, all edges hub → spoke.
func BuildStar(t *testing.T, n int) (*graph.Graph, string, []string) {
	t.Helper()
	g := graph.New()
	hub := "hub"
	g.AddNode(model.NewNode(hub, model.NodeGroup, hub))
	spokes := make([]string, n)
	for i := 0; i < n; i++ {
		spokes[i] = fmt.Sprintf("spoke-%d", i)
		g.AddNode(model.NewNode(spokes[i], model.NodeUser, spokes[i]))
		e := model.NewEdge(fmt.Sprintf("se-%d", i), model.EdgeMemberOf, hub, spokes[i])
		g.AddEdge(e)
	}
	return g, hub, spokes
}

// AssertPathExists fails the test if no path from fromID to toID is found.
func AssertPathExists(t *testing.T, g *graph.Graph, fromID, toID string) {
	t.Helper()
	paths := g.FindPaths(fromID, toID, graph.DefaultPathOptions())
	if len(paths) == 0 {
		t.Fatalf("expected path from %s to %s, found none", fromID, toID)
	}
}

// AssertNoPath fails the test if any path from fromID to toID is found.
func AssertNoPath(t *testing.T, g *graph.Graph, fromID, toID string) {
	t.Helper()
	paths := g.FindPaths(fromID, toID, graph.DefaultPathOptions())
	if len(paths) > 0 {
		t.Fatalf("expected no path from %s to %s, but found %d", fromID, toID, len(paths))
	}
}

// MustAddNode adds a node and fails the test on error.
func MustAddNode(t *testing.T, g *graph.Graph, id string, nt model.NodeType) *model.Node {
	t.Helper()
	n := model.NewNode(id, nt, id)
	if err := g.AddNode(n); err != nil {
		t.Fatalf("MustAddNode %s: %v", id, err)
	}
	return n
}

// MustAddEdge adds an edge and fails the test on error.
func MustAddEdge(t *testing.T, g *graph.Graph, id string, et model.EdgeType, src, tgt string) *model.Edge {
	t.Helper()
	e := model.NewEdge(id, et, src, tgt)
	if err := g.AddEdge(e); err != nil {
		t.Fatalf("MustAddEdge %s: %v", id, err)
	}
	return e
}
