package subcmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/model"
)

// gatherTopPaths collects paths from all nodes to all tier-0 targets, up to limit.
func gatherTopPaths(g *graph.Graph, limit int) []graph.Path {
	opts := graph.DefaultPathOptions()
	var paths []graph.Path
	for _, tgt := range g.Nodes() {
		if !tgt.HasTag(model.TagTier0) {
			continue
		}
		for _, src := range g.Nodes() {
			if src.ID == tgt.ID {
				continue
			}
			found := g.FindPaths(src.ID, tgt.ID, opts)
			paths = append(paths, found...)
			if len(paths) >= limit {
				return paths
			}
		}
	}
	return paths
}

// LoadGraphFromFile reads a PathCollapse graph snapshot (written by ingest --output)
// and returns a populated Graph.
func LoadGraphFromFile(path string) (*graph.Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()

	var snap struct {
		Nodes []model.Node `json:"nodes"`
		Edges []model.Edge `json:"edges"`
	}
	if err := json.NewDecoder(f).Decode(&snap); err != nil {
		return nil, fmt.Errorf("decode snapshot %q: %w", path, err)
	}

	g := graph.New()
	for i := range snap.Nodes {
		if err := g.AddNode(&snap.Nodes[i]); err != nil {
			return nil, fmt.Errorf("load node %q: %w", snap.Nodes[i].ID, err)
		}
	}
	for i := range snap.Edges {
		// Silently skip edges whose endpoints weren't in the snapshot.
		_ = g.AddEdge(&snap.Edges[i])
	}
	return g, nil
}
