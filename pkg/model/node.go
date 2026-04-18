package model

import "time"

// Node is a vertex in the identity graph.
type Node struct {
	ID         string         `json:"id"`
	Type       NodeType       `json:"type"`
	Name       string         `json:"name"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	Evidence   []EvidenceRef  `json:"evidence,omitempty"`
	FirstSeen  time.Time      `json:"first_seen"`
	LastSeen   time.Time      `json:"last_seen"`
}

// NewNode returns a Node with sensible defaults.
func NewNode(id string, t NodeType, name string) *Node {
	now := time.Now().UTC()
	return &Node{
		ID:         id,
		Type:       t,
		Name:       name,
		Attributes: make(map[string]any),
		FirstSeen:  now,
		LastSeen:   now,
	}
}

// HasTag reports whether the node carries the given tag.
func (n *Node) HasTag(tag string) bool {
	for _, t := range n.Tags {
		if t == tag {
			return true
		}
	}
	return false
}
