// Package benchmarks_test provides macrobenchmarks for large enterprise-scale
// identity graphs. Microbenchmarks (Chain10/50/100, FanOut) live in
// pkg/graph/graph_test.go and serve as regression gates.
package benchmarks_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/karunapuram/pathcollapse/pkg/controls"
	"github.com/karunapuram/pathcollapse/pkg/drift"
	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/ingest"
	"github.com/karunapuram/pathcollapse/pkg/model"
	"github.com/karunapuram/pathcollapse/pkg/scoring"
)

// buildEnterpriseGraph constructs a synthetic AD-style identity graph with a
// realistic privilege structure:
//   - users → groups (member_of, avg ~8 per user)
//   - groups → computers/DCs (admin_to, 2 per group)
//   - groups → groups (member_of nesting, ~20% of groups)
//   - service accounts with delegation and group membership
//   - DC trust relationships
//   - certificate template enrollment rights
//   - tier0 tags on DCs and CAs
//
// The RNG is seeded deterministically so graph shape is stable across runs.
func buildEnterpriseGraph(users, groups, computers, serviceAccounts int) *graph.Graph {
	g := graph.New()
	rng := rand.New(rand.NewSource(42))
	seq := 0

	addEdge := func(typ model.EdgeType, src, tgt string, conf, exploit, detect float64) {
		e := model.NewEdge(fmt.Sprintf("e-%d", seq), typ, src, tgt)
		seq++
		e.Confidence = conf
		e.Exploitability = exploit
		e.Detectability = detect
		e.BlastRadius = 0.3 + rng.Float64()*0.5
		_ = g.AddEdge(e)
	}

	numDCs := max(5, computers/100)
	numCAs := max(2, computers/200)
	numCertTemplates := max(10, groups/100)

	// tier0: domain controllers
	dcIDs := make([]string, numDCs)
	for i := range dcIDs {
		id := fmt.Sprintf("dc-%d", i)
		dcIDs[i] = id
		n := model.NewNode(id, model.NodeComputer, "DC-"+id)
		n.Tags = []string{model.TagTier0}
		_ = g.AddNode(n)
	}

	// tier0: certificate authorities
	caIDs := make([]string, numCAs)
	for i := range caIDs {
		id := fmt.Sprintf("ca-%d", i)
		caIDs[i] = id
		n := model.NewNode(id, model.NodeCA, "CA-"+id)
		n.Tags = []string{model.TagTier0}
		_ = g.AddNode(n)
	}

	// certificate templates
	ctIDs := make([]string, numCertTemplates)
	for i := range ctIDs {
		id := fmt.Sprintf("ct-%d", i)
		ctIDs[i] = id
		_ = g.AddNode(model.NewNode(id, model.NodeCertTemplate, "CertTemplate-"+id))
	}

	// regular computers
	compIDs := make([]string, computers)
	for i := range compIDs {
		id := fmt.Sprintf("comp-%d", i)
		compIDs[i] = id
		_ = g.AddNode(model.NewNode(id, model.NodeComputer, "COMP-"+id))
	}

	// all computer targets: DCs first so index 0 = dc-0
	allComps := make([]string, 0, numDCs+computers)
	allComps = append(allComps, dcIDs...)
	allComps = append(allComps, compIDs...)

	groupIDs := make([]string, groups)
	for i := range groupIDs {
		id := fmt.Sprintf("grp-%d", i)
		groupIDs[i] = id
		_ = g.AddNode(model.NewNode(id, model.NodeGroup, "GROUP-"+id))
	}

	saIDs := make([]string, serviceAccounts)
	for i := range saIDs {
		id := fmt.Sprintf("sa-%d", i)
		saIDs[i] = id
		_ = g.AddNode(model.NewNode(id, model.NodeServiceAccount, "SVC-"+id))
	}

	userIDs := make([]string, users)
	for i := range userIDs {
		id := fmt.Sprintf("usr-%d", i)
		userIDs[i] = id
		_ = g.AddNode(model.NewNode(id, model.NodeUser, "USER-"+id))
	}

	// user → group memberships (avg ~8 per user)
	for i, uid := range userIDs {
		n := 4 + rng.Intn(9) // 4–12
		for m := 0; m < n; m++ {
			gid := groupIDs[(i*7+m*13+rng.Intn(groups/4+1))%groups]
			addEdge(model.EdgeMemberOf, uid, gid,
				0.9+rng.Float64()*0.1, 0.4+rng.Float64()*0.4, 0.3+rng.Float64()*0.4)
		}
	}

	// group → group nesting (~20% of groups)
	for i, gid := range groupIDs {
		if rng.Float64() < 0.2 {
			parent := groupIDs[(i+1+rng.Intn(groups/4+1))%groups]
			addEdge(model.EdgeMemberOf, gid, parent, 1.0, 0.5, 0.4)
		}
	}

	// group → computer admin_to (2 per group; index 0 = dc-0 guaranteed for grp-0)
	for i, gid := range groupIDs {
		for j := 0; j < 2; j++ {
			cid := allComps[(i*3+j*7)%len(allComps)]
			addEdge(model.EdgeAdminTo, gid, cid, 0.95, 0.8, 0.2)
		}
	}

	// service accounts: delegation + group membership
	for i, said := range saIDs {
		for j := 0; j < 2; j++ {
			cid := allComps[(i*5+j*11)%len(allComps)]
			addEdge(model.EdgeCanDelegateTo, said, cid, 0.9, 0.85, 0.1)
		}
		addEdge(model.EdgeMemberOf, said, groupIDs[(i*3)%groups], 1.0, 0.5, 0.3)
	}

	// DC trust chain
	for i := 0; i < len(dcIDs)-1; i++ {
		addEdge(model.EdgeTrustedBy, dcIDs[i], dcIDs[i+1], 1.0, 0.9, 0.05)
	}

	// cert template enrollment (groups → templates → CAs)
	for i, ctid := range ctIDs {
		n := 2 + rng.Intn(4)
		for j := 0; j < n; j++ {
			addEdge(model.EdgeCanEnrollIn, groupIDs[(i*7+j*13)%groups], ctid, 0.8, 0.7, 0.3)
		}
		if len(caIDs) > 0 {
			addEdge(model.EdgeAuthenticatesTo, ctid, caIDs[i%len(caIDs)], 0.9, 0.8, 0.2)
		}
	}

	// password-reset rights
	for i := 0; i < groups/10; i++ {
		addEdge(model.EdgeCanResetPasswordOf,
			groupIDs[rng.Intn(groups)], userIDs[rng.Intn(users)], 0.85, 0.75, 0.35)
	}

	// domain-admins: over-privileged group with direct admin to all DCs.
	// ~10% of users are members, creating many 2-hop paths to tier0.
	daID := "domain-admins"
	da := model.NewNode(daID, model.NodeGroup, "Domain Admins")
	da.Tags = []string{model.TagTier1}
	_ = g.AddNode(da)
	for _, dcID := range dcIDs {
		addEdge(model.EdgeAdminTo, daID, dcID, 0.99, 0.95, 0.05)
	}
	for i := 0; i < users/10; i++ {
		addEdge(model.EdgeMemberOf, userIDs[i], daID, 1.0, 0.9, 0.1)
	}

	return g
}

// collectScoredPaths gathers scored paths from numUsers sources to all DCs
// up to numDCTargets. The domain-admins group in the generated graph ensures
// ~10% of users have a 2-hop path to every DC.
func collectScoredPaths(g *graph.Graph, numUsers, numDCTargets int) []scoring.ScoredPath {
	opts := graph.PathOptions{MaxDepth: 5}
	cfg := scoring.DefaultConfig()
	var all []scoring.ScoredPath
	for t := 0; t < numDCTargets; t++ {
		tgt := fmt.Sprintf("dc-%d", t)
		for u := 0; u < numUsers; u++ {
			paths := g.FindPaths(fmt.Sprintf("usr-%d", u), tgt, opts)
			all = append(all, scoring.RankPaths(paths, g, cfg)...)
		}
	}
	return all
}

// addDriftEdges appends fraction*edgeCount new membership edges to g,
// simulating ~10% drift between two snapshots.
func addDriftEdges(g *graph.Graph, fraction float64) {
	rng := rand.New(rand.NewSource(99))
	nodes := g.Nodes()
	nodeIDs := make([]string, len(nodes))
	for i, n := range nodes {
		nodeIDs[i] = n.ID
	}
	total := int(float64(g.EdgeCount()) * fraction)
	base := g.EdgeCount()
	for i := 0; i < total; i++ {
		src := nodeIDs[rng.Intn(len(nodeIDs))]
		tgt := nodeIDs[rng.Intn(len(nodeIDs))]
		if src == tgt {
			tgt = nodeIDs[(rng.Intn(len(nodeIDs)-1)+1)%len(nodeIDs)]
		}
		e := model.NewEdge(fmt.Sprintf("drift-%d", base+i), model.EdgeMemberOf, src, tgt)
		_ = g.AddEdge(e)
	}
}

// generateIngestJSON builds a JSON payload with n nodes and e edges,
// suitable for benchmarking the ingest.JSONAdapter.
func generateIngestJSON(n, e int) []byte {
	type jNode struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Name string `json:"name"`
	}
	type jEdge struct {
		ID             string  `json:"id"`
		Type           string  `json:"type"`
		Source         string  `json:"source"`
		Target         string  `json:"target"`
		Confidence     float64 `json:"confidence"`
		Exploitability float64 `json:"exploitability"`
		Detectability  float64 `json:"detectability"`
		BlastRadius    float64 `json:"blast_radius"`
	}
	type payload struct {
		Nodes []jNode `json:"nodes"`
		Edges []jEdge `json:"edges"`
	}

	rng := rand.New(rand.NewSource(77))
	p := payload{Nodes: make([]jNode, n), Edges: make([]jEdge, e)}
	edgeTypes := []string{"member_of", "admin_to", "can_delegate_to", "local_admin_to"}
	for i := range p.Nodes {
		p.Nodes[i] = jNode{
			ID:   fmt.Sprintf("n-%d", i),
			Type: "user",
			Name: fmt.Sprintf("User-%d", i),
		}
	}
	for i := range p.Edges {
		src := rng.Intn(n)
		tgt := (src + 1 + rng.Intn(n-1)) % n
		p.Edges[i] = jEdge{
			ID:             fmt.Sprintf("e-%d", i),
			Type:           edgeTypes[i%len(edgeTypes)],
			Source:         fmt.Sprintf("n-%d", src),
			Target:         fmt.Sprintf("n-%d", tgt),
			Confidence:     0.7 + rng.Float64()*0.3,
			Exploitability: 0.4 + rng.Float64()*0.5,
			Detectability:  0.2 + rng.Float64()*0.5,
			BlastRadius:    0.3 + rng.Float64()*0.5,
		}
	}
	data, _ := json.Marshal(p)
	return data
}

// ── 10k graph benchmarks ──────────────────────────────────────────────────────

// BenchmarkFindPaths_10k measures the full find→score→rank pipeline on a
// ~10k-node, ~60k-edge enterprise graph. grp-0 has admin_to dc-0 by
// construction (index formula: (0*3+0*7)%len(allComps) == 0), so the seed
// edge ensures at least one 2-hop path exists.
func BenchmarkFindPaths_10k(b *testing.B) {
	g := buildEnterpriseGraph(7000, 1500, 1000, 500)
	// Guarantee usr-0 → grp-0 → dc-0 path exists regardless of RNG outcome.
	_ = g.AddEdge(model.NewEdge("bench-seed", model.EdgeMemberOf, "usr-0", "grp-0"))
	b.ReportMetric(float64(g.NodeCount()), "nodes")
	b.ReportMetric(float64(g.EdgeCount()), "edges")

	opts := graph.PathOptions{MaxDepth: 5}
	cfg := scoring.DefaultConfig()
	b.ResetTimer()
	b.ReportAllocs()

	var totalPaths int
	for i := 0; i < b.N; i++ {
		paths := g.FindPaths("usr-0", "dc-0", opts)
		scored := scoring.RankPaths(paths, g, cfg)
		if len(scored) > 25 {
			scored = scored[:25]
		}
		totalPaths += len(scored)
	}
	b.ReportMetric(float64(totalPaths)/float64(b.N), "paths/op")
}

// BenchmarkOptimizer_10k measures controls.Optimize (greedy set-cover) on a
// ~10k-node graph with realistic scored paths as input.
func BenchmarkOptimizer_10k(b *testing.B) {
	g := buildEnterpriseGraph(7000, 1500, 1000, 500)
	_ = g.AddEdge(model.NewEdge("bench-seed", model.EdgeMemberOf, "usr-0", "grp-0"))
	b.ReportMetric(float64(g.NodeCount()), "nodes")
	b.ReportMetric(float64(g.EdgeCount()), "edges")

	scored := collectScoredPaths(g, 100, 5)
	cfg := controls.OptimizerConfig{MaxRecommendations: 10, MinPathsToQualify: 1}
	b.ReportMetric(float64(len(scored)), "input-paths")

	b.ResetTimer()
	b.ReportAllocs()

	var totalRecs int
	for i := 0; i < b.N; i++ {
		recs := controls.Optimize(scored, g, cfg)
		totalRecs += len(recs)
	}
	b.ReportMetric(float64(totalRecs)/float64(b.N), "breakpoints/op")
}

// BenchmarkDiff_10k measures drift.CompareSnapshots on two ~10k-node graphs
// where the second has ~10% new edges added.
func BenchmarkDiff_10k(b *testing.B) {
	g1 := buildEnterpriseGraph(7000, 1500, 1000, 500)
	g2 := buildEnterpriseGraph(7000, 1500, 1000, 500)
	addDriftEdges(g2, 0.10)
	b.ReportMetric(float64(g1.NodeCount()), "nodes")
	b.ReportMetric(float64(g1.EdgeCount()), "base-edges")

	t1, t2 := time.Time{}, time.Now()
	b.ResetTimer()
	b.ReportAllocs()

	var totalItems int
	for i := 0; i < b.N; i++ {
		rep := drift.CompareSnapshots(g1, g2, t1, t2)
		totalItems += len(rep.Items)
	}
	b.ReportMetric(float64(totalItems)/float64(b.N), "drift-items/op")
}

// ── 50k graph benchmarks ──────────────────────────────────────────────────────

// BenchmarkFindPaths_50k is the same pipeline as _10k on a ~50k-node graph.
func BenchmarkFindPaths_50k(b *testing.B) {
	g := buildEnterpriseGraph(35000, 7500, 5000, 2500)
	_ = g.AddEdge(model.NewEdge("bench-seed", model.EdgeMemberOf, "usr-0", "grp-0"))
	b.ReportMetric(float64(g.NodeCount()), "nodes")
	b.ReportMetric(float64(g.EdgeCount()), "edges")

	opts := graph.PathOptions{MaxDepth: 5}
	cfg := scoring.DefaultConfig()
	b.ResetTimer()
	b.ReportAllocs()

	var totalPaths int
	for i := 0; i < b.N; i++ {
		paths := g.FindPaths("usr-0", "dc-0", opts)
		scored := scoring.RankPaths(paths, g, cfg)
		if len(scored) > 25 {
			scored = scored[:25]
		}
		totalPaths += len(scored)
	}
	b.ReportMetric(float64(totalPaths)/float64(b.N), "paths/op")
}

// BenchmarkOptimizer_50k measures the optimizer on a ~50k-node graph.
func BenchmarkOptimizer_50k(b *testing.B) {
	g := buildEnterpriseGraph(35000, 7500, 5000, 2500)
	_ = g.AddEdge(model.NewEdge("bench-seed", model.EdgeMemberOf, "usr-0", "grp-0"))
	b.ReportMetric(float64(g.NodeCount()), "nodes")
	b.ReportMetric(float64(g.EdgeCount()), "edges")

	scored := collectScoredPaths(g, 100, 5)
	cfg := controls.OptimizerConfig{MaxRecommendations: 10, MinPathsToQualify: 1}
	b.ReportMetric(float64(len(scored)), "input-paths")

	b.ResetTimer()
	b.ReportAllocs()

	var totalRecs int
	for i := 0; i < b.N; i++ {
		recs := controls.Optimize(scored, g, cfg)
		totalRecs += len(recs)
	}
	b.ReportMetric(float64(totalRecs)/float64(b.N), "breakpoints/op")
}

// BenchmarkDiff_50k measures drift.CompareSnapshots on ~50k-node graphs.
func BenchmarkDiff_50k(b *testing.B) {
	g1 := buildEnterpriseGraph(35000, 7500, 5000, 2500)
	g2 := buildEnterpriseGraph(35000, 7500, 5000, 2500)
	addDriftEdges(g2, 0.10)
	b.ReportMetric(float64(g1.NodeCount()), "nodes")
	b.ReportMetric(float64(g1.EdgeCount()), "base-edges")

	t1, t2 := time.Time{}, time.Now()
	b.ResetTimer()
	b.ReportAllocs()

	var totalItems int
	for i := 0; i < b.N; i++ {
		rep := drift.CompareSnapshots(g1, g2, t1, t2)
		totalItems += len(rep.Items)
	}
	b.ReportMetric(float64(totalItems)/float64(b.N), "drift-items/op")
}

// ── Ingest benchmark ─────────────────────────────────────────────────────────

// BenchmarkIngest_100k_edges measures JSON parsing throughput for a 100k-edge
// payload (5k nodes). The payload is generated once in setup.
func BenchmarkIngest_100k_edges(b *testing.B) {
	data := generateIngestJSON(5000, 100000)
	b.ReportMetric(100000, "edges")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(data)
		if _, err := (&ingest.JSONAdapter{}).Ingest(r); err != nil {
			b.Fatal(err)
		}
	}
}
