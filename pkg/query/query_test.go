package query

import (
	"testing"

	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/model"
	"github.com/karunapuram/pathcollapse/pkg/scoring"
)

// ---- Lexer tests ----

func TestLexer_FindPaths(t *testing.T) {
	q := `FIND PATHS FROM user:alice TO privilege:tier0 WHERE confidence > 0.7 ORDER BY risk DESC LIMIT 10`
	l := NewLexer(q)
	tokens, err := l.Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	expects := []TokenType{
		tokFIND, tokPATHS, tokFROM, tokIdent, tokColon, tokIdent,
		tokTO, tokIdent, tokColon, tokIdent,
		tokWHERE, tokIdent, tokGT, tokNumber,
		tokORDER, tokBY, tokIdent, tokDESC,
		tokLIMIT, tokNumber, tokEOF,
	}
	if len(tokens) != len(expects) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expects), len(tokens), tokens)
	}
	for i, tt := range expects {
		if tokens[i].Type != tt {
			t.Errorf("token[%d]: expected type %d, got %d (%q)", i, tt, tokens[i].Type, tokens[i].Literal)
		}
	}
}

func TestLexer_Operators(t *testing.T) {
	tests := []struct {
		input string
		want  TokenType
	}{
		{">", tokGT}, {">=", tokGTE}, {"<", tokLT}, {"<=", tokLTE},
		{"=", tokEQ}, {"!=", tokNEQ},
	}
	for _, tc := range tests {
		l := NewLexer(tc.input)
		toks, err := l.Tokenize()
		if err != nil {
			t.Fatalf("input=%q: %v", tc.input, err)
		}
		if toks[0].Type != tc.want {
			t.Errorf("input=%q: expected %d, got %d", tc.input, tc.want, toks[0].Type)
		}
	}
}

// ---- Parser tests ----

func TestParser_FindPaths(t *testing.T) {
	stmt, err := ParseQuery(`FIND PATHS FROM user:alice TO privilege:tier0 WHERE confidence > 0.7 ORDER BY risk DESC LIMIT 5`)
	if err != nil {
		t.Fatal(err)
	}
	if stmt.Type != StmtFindPaths {
		t.Fatalf("expected StmtFindPaths, got %d", stmt.Type)
	}
	if stmt.From.Kind != "user" || stmt.From.Value != "alice" {
		t.Fatalf("FROM ref mismatch: %+v", stmt.From)
	}
	if stmt.To.Kind != "privilege" || stmt.To.Value != "tier0" {
		t.Fatalf("TO ref mismatch: %+v", stmt.To)
	}
	if len(stmt.Where) != 1 {
		t.Fatalf("expected 1 WHERE predicate, got %d", len(stmt.Where))
	}
	if stmt.Where[0].Field != "confidence" || stmt.Where[0].Operator != ">" || stmt.Where[0].Value != "0.7" {
		t.Fatalf("WHERE predicate mismatch: %+v", stmt.Where[0])
	}
	if !stmt.OrderDesc {
		t.Fatal("expected DESC order")
	}
	if stmt.Limit != 5 {
		t.Fatalf("expected limit 5, got %d", stmt.Limit)
	}
}

func TestParser_FindBreakpoints(t *testing.T) {
	stmt, err := ParseQuery(`FIND BREAKPOINTS FOR top_paths LIMIT 5`)
	if err != nil {
		t.Fatal(err)
	}
	if stmt.Type != StmtFindBreakpoints {
		t.Fatalf("expected StmtFindBreakpoints")
	}
	if stmt.BreakpointsFor != "top_paths" {
		t.Fatalf("expected top_paths, got %q", stmt.BreakpointsFor)
	}
	if stmt.Limit != 5 {
		t.Fatalf("expected limit 5, got %d", stmt.Limit)
	}
}

func TestParser_ShowDrift(t *testing.T) {
	stmt, err := ParseQuery(`SHOW DRIFT SINCE last_snapshot`)
	if err != nil {
		t.Fatal(err)
	}
	if stmt.Type != StmtShowDrift {
		t.Fatalf("expected StmtShowDrift")
	}
	if stmt.DriftSince != "last_snapshot" {
		t.Fatalf("expected last_snapshot, got %q", stmt.DriftSince)
	}
}

func TestParser_EmptyQuery(t *testing.T) {
	_, err := ParseQuery(``)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

// ---- Executor tests ----

func buildTestGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g := graph.New()
	alice := model.NewNode("alice", model.NodeUser, "alice")
	admins := model.NewNode("domain-admins", model.NodeGroup, "Domain Admins")
	dc := model.NewNode("dc01", model.NodeComputer, "DC01")
	dc.Tags = []string{model.TagTier0}

	for _, n := range []*model.Node{alice, admins, dc} {
		if err := g.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}
	e1 := model.NewEdge("e1", model.EdgeMemberOf, "alice", "domain-admins")
	e2 := model.NewEdge("e2", model.EdgeAdminTo, "domain-admins", "dc01")
	for _, e := range []*model.Edge{e1, e2} {
		if err := g.AddEdge(e); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

func buildOrderByGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g := graph.New()

	nodes := []*model.Node{
		model.NewNode("alice", model.NodeUser, "alice"),
		model.NewNode("high-path", model.NodeGroup, "high-path"),
		model.NewNode("low-path", model.NodeGroup, "low-path"),
		model.NewNode("dc01", model.NodeComputer, "DC01"),
	}
	nodes[len(nodes)-1].Tags = []string{model.TagTier0}

	for _, n := range nodes {
		if err := g.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}

	high1 := model.NewEdge("high-1", model.EdgeMemberOf, "alice", "high-path")
	high1.Confidence = 0.9
	high1.Exploitability = 0.9
	high2 := model.NewEdge("high-2", model.EdgeAdminTo, "high-path", "dc01")
	high2.Confidence = 0.9
	high2.Exploitability = 0.9

	low1 := model.NewEdge("low-1", model.EdgeMemberOf, "alice", "low-path")
	low1.Confidence = 0.2
	low1.Exploitability = 0.2
	low2 := model.NewEdge("low-2", model.EdgeAdminTo, "low-path", "dc01")
	low2.Confidence = 0.2
	low2.Exploitability = 0.2

	for _, e := range []*model.Edge{high1, high2, low1, low2} {
		if err := g.AddEdge(e); err != nil {
			t.Fatal(err)
		}
	}

	return g
}

func TestExecutor_FindPaths(t *testing.T) {
	g := buildTestGraph(t)
	ex := NewExecutor(g, scoring.DefaultConfig())

	stmt, err := ParseQuery(`FIND PATHS FROM user:alice TO privilege:tier0 LIMIT 10`)
	if err != nil {
		t.Fatal(err)
	}
	res, err := ex.Execute(stmt)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) == 0 {
		t.Fatal("expected at least one path from alice to tier0")
	}
}

func TestExecutor_ShowDrift(t *testing.T) {
	g := graph.New()
	ex := NewExecutor(g, scoring.DefaultConfig())
	stmt := &Statement{Type: StmtShowDrift, DriftSince: "last_snapshot"}
	res, err := ex.Execute(stmt)
	if err != nil {
		t.Fatal(err)
	}
	if res.Message == "" {
		t.Fatal("expected non-empty message for SHOW DRIFT")
	}
}

func TestExecutor_FindPaths_OrderByConfidenceAsc(t *testing.T) {
	g := buildOrderByGraph(t)
	ex := NewExecutor(g, scoring.DefaultConfig())

	stmt, err := ParseQuery(`FIND PATHS FROM user:alice TO privilege:tier0 ORDER BY confidence ASC`)
	if err != nil {
		t.Fatal(err)
	}

	res, err := ex.Execute(stmt)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ScoredPaths) != 2 {
		t.Fatalf("expected 2 scored paths, got %d", len(res.ScoredPaths))
	}
	if got := res.ScoredPaths[0].Breakdown.Confidence; got != 0.2 {
		t.Fatalf("expected lowest-confidence path first, got %.2f", got)
	}
	if got := res.ScoredPaths[1].Breakdown.Confidence; got != 0.9 {
		t.Fatalf("expected highest-confidence path second, got %.2f", got)
	}
}
