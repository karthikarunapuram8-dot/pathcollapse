package ingest

import (
	"strings"
	"testing"
)

func TestJSONAdapter_Basic(t *testing.T) {
	raw := `{
		"nodes": [
			{"id":"u1","type":"user","name":"Alice","tags":["tier1"]},
			{"id":"g1","type":"group","name":"Domain Admins"}
		],
		"edges": [
			{"id":"e1","type":"member_of","source":"u1","target":"g1","confidence":0.9}
		]
	}`
	a := &JSONAdapter{}
	res, err := a.Ingest(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(res.Nodes))
	}
	if len(res.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(res.Edges))
	}
	if res.Edges[0].Confidence != 0.9 {
		t.Fatalf("expected confidence 0.9, got %f", res.Edges[0].Confidence)
	}
}

func TestJSONAdapter_EmptyNodeID(t *testing.T) {
	raw := `{"nodes":[{"id":"","type":"user","name":"Bad"}],"edges":[]}`
	a := &JSONAdapter{}
	res, err := a.Ingest(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Nodes) != 0 {
		t.Fatal("node with empty ID should be skipped")
	}
	if len(res.Warns) == 0 {
		t.Fatal("expected warning for skipped node")
	}
}

func TestJSONAdapter_PreservesExplicitZeroMetrics(t *testing.T) {
	raw := `{
		"nodes": [
			{"id":"u1","type":"user","name":"Alice"},
			{"id":"c1","type":"computer","name":"DC01"}
		],
		"edges": [
			{
				"id":"e1",
				"type":"admin_to",
				"source":"u1",
				"target":"c1",
				"confidence":0,
				"exploitability":0,
				"detectability":0,
				"blast_radius":0
			}
		]
	}`

	res, err := (&JSONAdapter{}).Ingest(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(res.Edges))
	}
	e := res.Edges[0]
	if e.Confidence != 0 || e.Exploitability != 0 || e.Detectability != 0 || e.BlastRadius != 0 {
		t.Fatalf("expected explicit zero metrics to be preserved, got %+v", *e)
	}
}

func TestCSVUserAdapter(t *testing.T) {
	raw := "id,name,type,tags\nu1,Alice,user,tier1;vip\nu2,Bob,service_account,"
	a := &CSVUserAdapter{}
	res, err := a.Ingest(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(res.Nodes))
	}
	if res.Nodes[0].Name != "Alice" {
		t.Fatalf("expected Alice, got %s", res.Nodes[0].Name)
	}
	if len(res.Nodes[0].Tags) != 2 {
		t.Fatalf("expected 2 tags, got %v", res.Nodes[0].Tags)
	}
}

func TestCSVGroupAdapter(t *testing.T) {
	raw := "member_id,group_id\nu1,g1\nu2,g1"
	a := &CSVGroupAdapter{}
	res, err := a.Ingest(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(res.Edges))
	}
}

func TestCSVLocalAdminAdapter(t *testing.T) {
	raw := "user_id,computer_id,confidence\nu1,c1,0.8\nu2,c2,"
	a := &CSVLocalAdminAdapter{}
	res, err := a.Ingest(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(res.Edges))
	}
	if res.Edges[0].Confidence != 0.8 {
		t.Fatalf("expected 0.8 confidence, got %f", res.Edges[0].Confidence)
	}
}

func TestYAMLAdapter(t *testing.T) {
	raw := `
nodes:
  - id: u1
    type: user
    name: Alice
    tags: [tier0]
edges:
  - id: e1
    type: admin_to
    source: u1
    target: dc1
    confidence: 0.95
`
	a := &YAMLAdapter{}
	res, err := a.Ingest(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Nodes) != 1 || res.Nodes[0].Name != "Alice" {
		t.Fatalf("expected 1 node Alice, got %v", res.Nodes)
	}
	if len(res.Edges) != 1 || res.Edges[0].Confidence != 0.95 {
		t.Fatalf("expected 1 edge with conf 0.95, got %v", res.Edges)
	}
}

func TestYAMLAdapter_PreservesExplicitZeroMetrics(t *testing.T) {
	raw := `
nodes:
  - id: u1
    type: user
    name: Alice
  - id: c1
    type: computer
    name: DC01
edges:
  - id: e1
    type: admin_to
    source: u1
    target: c1
    confidence: 0
    exploitability: 0
    detectability: 0
    blast_radius: 0
`
	res, err := (&YAMLAdapter{}).Ingest(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(res.Edges))
	}
	e := res.Edges[0]
	if e.Confidence != 0 || e.Exploitability != 0 || e.Detectability != 0 || e.BlastRadius != 0 {
		t.Fatalf("expected explicit zero metrics to be preserved, got %+v", *e)
	}
}

func TestGetUnknownAdapter(t *testing.T) {
	_, err := Get("unknown_type")
	if err == nil {
		t.Fatal("expected error for unknown adapter type")
	}
}

func TestGetKnownAdapters(t *testing.T) {
	types := []AdapterType{
		AdapterGenericJSON, AdapterCSVUsers, AdapterCSVGroups,
		AdapterCSVLocalAdmin, AdapterCSVGPO, AdapterBloodHound, AdapterYAMLFacts,
	}
	for _, at := range types {
		if _, err := Get(at); err != nil {
			t.Errorf("Get(%q) returned error: %v", at, err)
		}
	}
}
