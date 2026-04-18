// Package normalize canonicalizes node/edge data after ingestion.
package normalize

import (
	"sort"
	"strings"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/ingest"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
)

// Result holds normalized nodes and edges after deduplication.
type Result struct {
	Nodes []*model.Node
	Edges []*model.Edge
	Warns []string
}

// Normalize deduplicates and canonicalizes an ingest result.
// Nodes with the same ID are merged (last-write wins for attributes).
// Edges with the same source+type+target are deduplicated.
func Normalize(raw *ingest.Result) *Result {
	res := &Result{}

	// Deduplicate nodes by ID.
	nodeMap := make(map[string]*model.Node, len(raw.Nodes))
	for _, n := range raw.Nodes {
		existing, ok := nodeMap[n.ID]
		if !ok {
			nodeMap[n.ID] = n
			continue
		}
		// Merge attributes from duplicate.
		for k, v := range n.Attributes {
			existing.Attributes[k] = v
		}
		existing.Tags = mergeStrings(existing.Tags, n.Tags)
		if n.LastSeen.After(existing.LastSeen) {
			existing.LastSeen = n.LastSeen
		}
	}
	for _, n := range nodeMap {
		canonicalizeNode(n)
		res.Nodes = append(res.Nodes, n)
	}
	sort.Slice(res.Nodes, func(i, j int) bool { return res.Nodes[i].ID < res.Nodes[j].ID })

	// Deduplicate edges by source+type+target.
	type edgeKey struct {
		src string
		typ model.EdgeType
		tgt string
	}
	edgeMap := make(map[edgeKey]*model.Edge, len(raw.Edges))
	for _, e := range raw.Edges {
		key := edgeKey{src: e.Source, typ: e.Type, tgt: e.Target}
		existing, ok := edgeMap[key]
		if !ok {
			edgeMap[key] = e
			continue
		}
		// Keep higher confidence.
		if e.Confidence > existing.Confidence {
			edgeMap[key] = e
		}
	}
	for _, e := range edgeMap {
		res.Edges = append(res.Edges, e)
	}
	sort.Slice(res.Edges, func(i, j int) bool { return res.Edges[i].ID < res.Edges[j].ID })

	res.Warns = raw.Warns
	return res
}

func canonicalizeNode(n *model.Node) {
	n.Name = strings.TrimSpace(n.Name)
	// Lowercase email-style names for consistent matching.
	if strings.Contains(n.Name, "@") {
		n.Name = strings.ToLower(n.Name)
	}
}

func mergeStrings(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	for _, s := range a {
		seen[s] = true
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			a = append(a, s)
		}
	}
	return a
}
