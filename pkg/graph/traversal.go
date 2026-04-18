package graph

import (
	"sort"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
)

// FilteredTraversal performs a BFS from startID, returning all paths reachable
// through edges accepted by filter (nil = accept all). Cycles are not followed.
func (g *Graph) FilteredTraversal(startID string, filter func(*model.Edge) bool) []Path {
	startNode := g.GetNode(startID)
	if startNode == nil {
		return nil
	}

	type state struct {
		path    Path
		visited map[string]bool
	}

	var results []Path
	queue := []state{{
		path:    Path{Nodes: []*model.Node{startNode}},
		visited: map[string]bool{startID: true},
	}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		curID := cur.path.Nodes[len(cur.path.Nodes)-1].ID
		for _, e := range g.Neighbors(curID) {
			if filter != nil && !filter(e) {
				continue
			}
			if cur.visited[e.Target] {
				continue
			}
			targetNode := g.GetNode(e.Target)
			if targetNode == nil {
				continue
			}

			newVisited := make(map[string]bool, len(cur.visited)+1)
			for k, v := range cur.visited {
				newVisited[k] = v
			}
			newVisited[e.Target] = true

			newNodes := make([]*model.Node, len(cur.path.Nodes)+1)
			copy(newNodes, cur.path.Nodes)
			newNodes[len(cur.path.Nodes)] = targetNode

			newEdges := make([]*model.Edge, len(cur.path.Edges)+1)
			copy(newEdges, cur.path.Edges)
			newEdges[len(cur.path.Edges)] = e

			next := state{
				path:    Path{Nodes: newNodes, Edges: newEdges},
				visited: newVisited,
			}
			results = append(results, next.path)
			queue = append(queue, next)
		}
	}

	return results
}

// WeightedTraversal returns paths from startID ordered by descending cumulative
// edge weight (product of Confidence × Exploitability per edge).
func (g *Graph) WeightedTraversal(startID string, maxDepth int) []Path {
	if maxDepth <= 0 {
		maxDepth = 8
	}
	startNode := g.GetNode(startID)
	if startNode == nil {
		return nil
	}

	type item struct {
		path   Path
		weight float64
		seen   map[string]bool
	}

	var results []Path
	queue := []item{{
		path:   Path{Nodes: []*model.Node{startNode}},
		weight: 1.0,
		seen:   map[string]bool{startID: true},
	}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if len(cur.path.Edges) >= maxDepth {
			results = append(results, cur.path)
			continue
		}

		curID := cur.path.Nodes[len(cur.path.Nodes)-1].ID
		expanded := false
		for _, e := range g.Neighbors(curID) {
			if cur.seen[e.Target] {
				continue
			}
			targetNode := g.GetNode(e.Target)
			if targetNode == nil {
				continue
			}

			expanded = true
			newWeight := cur.weight * e.Confidence * e.Exploitability

			newSeen := make(map[string]bool, len(cur.seen)+1)
			for k, v := range cur.seen {
				newSeen[k] = v
			}
			newSeen[e.Target] = true

			newNodes := make([]*model.Node, len(cur.path.Nodes)+1)
			copy(newNodes, cur.path.Nodes)
			newNodes[len(cur.path.Nodes)] = targetNode

			newEdges := make([]*model.Edge, len(cur.path.Edges)+1)
			copy(newEdges, cur.path.Edges)
			newEdges[len(cur.path.Edges)] = e

			queue = append(queue, item{
				path:   Path{Nodes: newNodes, Edges: newEdges},
				weight: newWeight,
				seen:   newSeen,
			})
		}
		if !expanded && len(cur.path.Edges) > 0 {
			results = append(results, cur.path)
		}
	}

	sortPathsByWeight(results, g)
	return results
}

func sortPathsByWeight(paths []Path, g *Graph) {
	weight := func(p Path) float64 {
		w := 1.0
		for _, e := range p.Edges {
			w *= e.Confidence * e.Exploitability
		}
		return w
	}
	sort.Slice(paths, func(i, j int) bool {
		wi, wj := weight(paths[i]), weight(paths[j])
		if wi != wj {
			return wi > wj
		}
		return paths[i].Len() < paths[j].Len()
	})
}

// ConnectedComponents returns groups of node IDs that are connected (undirected).
// Both the component list and each component's node IDs are sorted for determinism.
func (g *Graph) ConnectedComponents() [][]string {
	g.mu.RLock()
	allNodeIDs := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		allNodeIDs = append(allNodeIDs, id)
	}
	g.mu.RUnlock()

	sort.Strings(allNodeIDs)

	visited := make(map[string]bool)
	var components [][]string

	for _, id := range allNodeIDs {
		if visited[id] {
			continue
		}
		comp := g.bfsUndirected(id, visited)
		sort.Strings(comp)
		components = append(components, comp)
	}
	return components
}

func (g *Graph) bfsUndirected(startID string, visited map[string]bool) []string {
	var comp []string
	queue := []string{startID}
	visited[startID] = true

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		comp = append(comp, cur)

		for _, e := range g.Neighbors(cur) {
			if !visited[e.Target] {
				visited[e.Target] = true
				queue = append(queue, e.Target)
			}
		}
		for _, e := range g.ReverseNeighbors(cur) {
			if !visited[e.Source] {
				visited[e.Source] = true
				queue = append(queue, e.Source)
			}
		}
	}
	return comp
}
