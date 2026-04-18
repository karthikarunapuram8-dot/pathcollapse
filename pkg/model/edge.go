package model

import "time"

// Edge is a directed relationship between two Nodes.
type Edge struct {
	ID             string         `json:"id"`
	Type           EdgeType       `json:"type"`
	Source         string         `json:"source"`
	Target         string         `json:"target"`
	Confidence     float64        `json:"confidence"`
	Exploitability float64        `json:"exploitability"`
	Detectability  float64        `json:"detectability"`
	BlastRadius    float64        `json:"blast_radius"`
	Conditions     []Condition    `json:"conditions,omitempty"`
	Preconditions  []Precondition `json:"preconditions,omitempty"`
	Evidence       []EvidenceRef  `json:"evidence,omitempty"`
	FirstSeen      time.Time      `json:"first_seen"`
	LastSeen       time.Time      `json:"last_seen"`
}

// Condition is a key/operator/value constraint on an edge.
type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// Precondition is an attacker prerequisite for traversing the edge.
type Precondition struct {
	Description string        `json:"description"`
	Satisfied   bool          `json:"satisfied"`
	Evidence    []EvidenceRef `json:"evidence,omitempty"`
}

// EvidenceRef links a fact back to its source artifact.
type EvidenceRef struct {
	Source      string    `json:"source"`
	Type        string    `json:"type"`
	Reference   string    `json:"reference"`
	CollectedAt time.Time `json:"collected_at"`
}

// NewEdge returns an Edge with neutral defaults (confidence 1, exploitability/detectability/blast 0.5).
func NewEdge(id string, t EdgeType, source, target string) *Edge {
	now := time.Now().UTC()
	return &Edge{
		ID:             id,
		Type:           t,
		Source:         source,
		Target:         target,
		Confidence:     1.0,
		Exploitability: 0.5,
		Detectability:  0.5,
		BlastRadius:    0.5,
		FirstSeen:      now,
		LastSeen:       now,
	}
}

// AllPreconditionsMet returns true when every precondition is satisfied.
func (e *Edge) AllPreconditionsMet() bool {
	for _, p := range e.Preconditions {
		if !p.Satisfied {
			return false
		}
	}
	return true
}
