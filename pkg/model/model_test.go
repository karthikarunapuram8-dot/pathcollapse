package model

import (
	"testing"
	"time"
)

func TestNewNode(t *testing.T) {
	n := NewNode("u1", NodeUser, "Alice")
	if n.ID != "u1" {
		t.Fatalf("expected ID u1, got %s", n.ID)
	}
	if n.Type != NodeUser {
		t.Fatalf("expected type user, got %s", n.Type)
	}
	if n.Attributes == nil {
		t.Fatal("Attributes map should be initialised")
	}
	if n.FirstSeen.IsZero() || n.LastSeen.IsZero() {
		t.Fatal("timestamps must be set")
	}
}

func TestNodeHasTag(t *testing.T) {
	tests := []struct {
		tags []string
		tag  string
		want bool
	}{
		{[]string{"tier0", "dc"}, "tier0", true},
		{[]string{"tier0", "dc"}, "tier1", false},
		{nil, "tier0", false},
	}
	for _, tc := range tests {
		n := NewNode("x", NodeComputer, "X")
		n.Tags = tc.tags
		if got := n.HasTag(tc.tag); got != tc.want {
			t.Errorf("HasTag(%q) = %v, want %v", tc.tag, got, tc.want)
		}
	}
}

func TestNewEdge(t *testing.T) {
	e := NewEdge("e1", EdgeMemberOf, "u1", "g1")
	if e.Source != "u1" || e.Target != "g1" {
		t.Fatal("source/target mismatch")
	}
	if e.Confidence != 1.0 {
		t.Fatalf("default confidence should be 1.0, got %f", e.Confidence)
	}
	if e.FirstSeen.IsZero() {
		t.Fatal("FirstSeen must be set")
	}
}

func TestEdgeAllPreconditionsMet(t *testing.T) {
	e := NewEdge("e2", EdgeAdminTo, "u1", "c1")

	// No preconditions — always met.
	if !e.AllPreconditionsMet() {
		t.Fatal("empty preconditions should be met")
	}

	e.Preconditions = []Precondition{
		{Description: "requires VPN", Satisfied: true},
		{Description: "requires MFA bypass", Satisfied: false},
	}
	if e.AllPreconditionsMet() {
		t.Fatal("unsatisfied precondition should return false")
	}

	for i := range e.Preconditions {
		e.Preconditions[i].Satisfied = true
	}
	if !e.AllPreconditionsMet() {
		t.Fatal("all satisfied should return true")
	}
}

func TestEvidenceRef(t *testing.T) {
	ref := EvidenceRef{
		Source:      "ldap-export",
		Type:        "group-membership",
		Reference:   "CN=Domain Admins,DC=corp,DC=local",
		CollectedAt: time.Now().UTC(),
	}
	if ref.Source == "" {
		t.Fatal("source must not be empty")
	}
}
