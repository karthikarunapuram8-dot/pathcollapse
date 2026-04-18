// Package graph provides the identity graph engine: adjacency indexes,
// add/remove operations, and analysis helpers.
package graph

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
)

// Sentinel errors returned by graph mutation operations.
// Callers can inspect errors with errors.Is.
var (
	ErrNilNode     = errors.New("graph: nil node")
	ErrEmptyNodeID = errors.New("graph: empty node ID")
	ErrNilEdge     = errors.New("graph: nil edge")
	ErrEmptyEdgeID = errors.New("graph: empty edge ID")
	// ErrSourceMissing is wrapped with the missing node ID.
	ErrSourceMissing = errors.New("graph: source node not found")
	// ErrTargetMissing is wrapped with the missing node ID.
	ErrTargetMissing = errors.New("graph: target node not found")
)

// Graph is a concurrency-safe directed multi-graph of identity nodes and edges.
// Forward and reverse adjacency indexes are maintained for O(1) neighbor lookup.
type Graph struct {
	mu       sync.RWMutex
	nodes    map[string]*model.Node
	edges    map[string]*model.Edge
	forward  map[string][]string // nodeID → []edgeID
	reverse  map[string][]string // nodeID → []edgeID (incoming)
}

// New returns an empty Graph.
func New() *Graph {
	return &Graph{
		nodes:   make(map[string]*model.Node),
		edges:   make(map[string]*model.Edge),
		forward: make(map[string][]string),
		reverse: make(map[string][]string),
	}
}

// AddNode inserts or replaces a node.
func (g *Graph) AddNode(n *model.Node) error {
	if n == nil {
		return ErrNilNode
	}
	if n.ID == "" {
		return ErrEmptyNodeID
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[n.ID] = n
	return nil
}

// AddEdge inserts or replaces an edge. Both endpoint nodes must exist.
func (g *Graph) AddEdge(e *model.Edge) error {
	if e == nil {
		return ErrNilEdge
	}
	if e.ID == "" {
		return ErrEmptyEdgeID
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.nodes[e.Source]; !ok {
		return fmt.Errorf("%w: %q", ErrSourceMissing, e.Source)
	}
	if _, ok := g.nodes[e.Target]; !ok {
		return fmt.Errorf("%w: %q", ErrTargetMissing, e.Target)
	}
	g.edges[e.ID] = e
	g.forward[e.Source] = appendUnique(g.forward[e.Source], e.ID)
	g.reverse[e.Target] = appendUnique(g.reverse[e.Target], e.ID)
	return nil
}

// RemoveNode removes a node and all edges incident to it.
func (g *Graph) RemoveNode(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	// Remove all edges that touch this node.
	for eid, e := range g.edges {
		if e.Source == id || e.Target == id {
			g.removeEdgeLocked(eid, e)
		}
	}
	delete(g.nodes, id)
}

// RemoveEdge removes a single edge by ID.
func (g *Graph) RemoveEdge(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if e, ok := g.edges[id]; ok {
		g.removeEdgeLocked(id, e)
	}
}

func (g *Graph) removeEdgeLocked(eid string, e *model.Edge) {
	g.forward[e.Source] = removeStr(g.forward[e.Source], eid)
	g.reverse[e.Target] = removeStr(g.reverse[e.Target], eid)
	delete(g.edges, eid)
}

// GetNode returns a node by ID, or nil.
func (g *Graph) GetNode(id string) *model.Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nodes[id]
}

// GetEdge returns an edge by ID, or nil.
func (g *Graph) GetEdge(id string) *model.Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.edges[id]
}

// Nodes returns a snapshot of all nodes sorted by ID.
func (g *Graph) Nodes() []*model.Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]*model.Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Edges returns a snapshot of all edges sorted by ID.
func (g *Graph) Edges() []*model.Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]*model.Edge, 0, len(g.edges))
	for _, e := range g.edges {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// NodeCount returns the number of nodes in the graph.
func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// EdgeCount returns the number of edges in the graph.
func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.edges)
}

// Neighbors returns edges leaving nodeID.
func (g *Graph) Neighbors(nodeID string) []*model.Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.edgesByIDs(g.forward[nodeID])
}

// ReverseNeighbors returns edges arriving at nodeID.
func (g *Graph) ReverseNeighbors(nodeID string) []*model.Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.edgesByIDs(g.reverse[nodeID])
}

func (g *Graph) edgesByIDs(ids []string) []*model.Edge {
	out := make([]*model.Edge, 0, len(ids))
	for _, id := range ids {
		if e, ok := g.edges[id]; ok {
			out = append(out, e)
		}
	}
	return out
}

// PrivilegeConcentration returns the nodes with the most inbound privileged edges,
// sorted descending. These are high-value targets.
func (g *Graph) PrivilegeConcentration() []ConcentrationEntry {
	privileged := map[model.EdgeType]bool{
		model.EdgeAdminTo:            true,
		model.EdgeLocalAdminTo:       true,
		model.EdgePrivilegedOver:     true,
		model.EdgeCanResetPasswordOf: true,
		model.EdgeCanWriteACLOf:      true,
	}

	g.mu.RLock()
	counts := make(map[string]int, len(g.nodes))
	for _, e := range g.edges {
		if privileged[e.Type] {
			counts[e.Target]++
		}
	}
	g.mu.RUnlock()

	entries := make([]ConcentrationEntry, 0, len(counts))
	for nodeID, count := range counts {
		if n := g.GetNode(nodeID); n != nil {
			entries = append(entries, ConcentrationEntry{Node: n, InboundPrivileged: count})
		}
	}
	sortByCount(entries)
	return entries
}

// ConcentrationEntry pairs a node with its inbound privileged edge count.
type ConcentrationEntry struct {
	Node              *model.Node
	InboundPrivileged int
}

// helpers

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

func removeStr(s []string, v string) []string {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func sortByCount(entries []ConcentrationEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].InboundPrivileged != entries[j].InboundPrivileged {
			return entries[i].InboundPrivileged > entries[j].InboundPrivileged
		}
		return entries[i].Node.ID < entries[j].Node.ID
	})
}
