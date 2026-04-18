package graph

import "github.com/karunapuram/pathcollapse/pkg/model"

// Path is an ordered sequence of alternating nodes and edges through the graph.
type Path struct {
	Nodes []*model.Node
	Edges []*model.Edge
}

// Len returns the number of edges in the path.
func (p Path) Len() int { return len(p.Edges) }

// Source returns the first node of the path.
func (p Path) Source() *model.Node {
	if len(p.Nodes) == 0 {
		return nil
	}
	return p.Nodes[0]
}

// Target returns the last node of the path.
func (p Path) Target() *model.Node {
	if len(p.Nodes) == 0 {
		return nil
	}
	return p.Nodes[len(p.Nodes)-1]
}

// PathOptions controls FindPaths behaviour.
type PathOptions struct {
	MaxDepth      int
	MinConfidence float64
	EdgeFilter    func(*model.Edge) bool // nil = accept all
}

// DefaultPathOptions returns sensible defaults.
func DefaultPathOptions() PathOptions {
	return PathOptions{
		MaxDepth:      8,
		MinConfidence: 0.0,
	}
}

// pathSearch holds the shared mutable state for one FindPaths call.
// Backtracking DFS appends and truncates nodeBuf/edgeBuf in place so that
// no per-step copies are needed; the caller's read lock is held throughout.
type pathSearch struct {
	g        *Graph
	toID     string
	maxDepth int
	minConf  float64
	filter   func(*model.Edge) bool
	results  []Path
	nodeBuf  []*model.Node
	edgeBuf  []*model.Edge
	visited  map[string]bool
}

// dfs extends the current path from nodeID, recording complete paths.
// The caller must hold g.mu.RLock for the duration.
func (s *pathSearch) dfs(nodeID string) {
	if nodeID == s.toID && len(s.edgeBuf) > 0 {
		ns := make([]*model.Node, len(s.nodeBuf))
		es := make([]*model.Edge, len(s.edgeBuf))
		copy(ns, s.nodeBuf)
		copy(es, s.edgeBuf)
		s.results = append(s.results, Path{Nodes: ns, Edges: es})
		return
	}
	if len(s.edgeBuf) >= s.maxDepth {
		return
	}
	for _, eid := range s.g.forward[nodeID] {
		e := s.g.edges[eid]
		if e == nil {
			continue
		}
		if e.Confidence < s.minConf {
			continue
		}
		if s.filter != nil && !s.filter(e) {
			continue
		}
		if s.visited[e.Target] {
			continue
		}
		tgt := s.g.nodes[e.Target]
		if tgt == nil {
			continue
		}
		s.visited[e.Target] = true
		s.nodeBuf = append(s.nodeBuf, tgt)
		s.edgeBuf = append(s.edgeBuf, e)
		s.dfs(e.Target)
		s.nodeBuf = s.nodeBuf[:len(s.nodeBuf)-1]
		s.edgeBuf = s.edgeBuf[:len(s.edgeBuf)-1]
		delete(s.visited, e.Target)
	}
}

// FindPaths returns all simple paths from fromID to toID in the graph.
// A single read lock covers the entire traversal. Backtracking DFS avoids
// per-step copies of path state and the visited set.
func (g *Graph) FindPaths(fromID, toID string, opts PathOptions) []Path {
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 8
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	start := g.nodes[fromID]
	if start == nil {
		return nil
	}

	s := &pathSearch{
		g:        g,
		toID:     toID,
		maxDepth: opts.MaxDepth,
		minConf:  opts.MinConfidence,
		filter:   opts.EdgeFilter,
		nodeBuf:  make([]*model.Node, 0, opts.MaxDepth+1),
		edgeBuf:  make([]*model.Edge, 0, opts.MaxDepth),
		visited:  make(map[string]bool, opts.MaxDepth+1),
	}
	s.nodeBuf = append(s.nodeBuf, start)
	s.visited[fromID] = true
	s.dfs(fromID)
	return s.results
}
