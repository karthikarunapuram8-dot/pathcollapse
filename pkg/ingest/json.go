package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/karunapuram/pathcollapse/pkg/model"
)

// jsonNode is the wire format for a node in the generic JSON schema.
type jsonNode struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Attributes map[string]any `json:"attributes"`
	Tags       []string       `json:"tags"`
}

// jsonEdge is the wire format for an edge in the generic JSON schema.
type jsonEdge struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Source         string   `json:"source"`
	Target         string   `json:"target"`
	Confidence     *float64 `json:"confidence"`
	Exploitability *float64 `json:"exploitability"`
	Detectability  *float64 `json:"detectability"`
	BlastRadius    *float64 `json:"blast_radius"`
}

type jsonPayload struct {
	Nodes []jsonNode `json:"nodes"`
	Edges []jsonEdge `json:"edges"`
}

// JSONAdapter parses the PathCollapse generic JSON format.
type JSONAdapter struct{}

func (a *JSONAdapter) Name() string { return "generic-json" }

func (a *JSONAdapter) Ingest(r io.Reader) (*Result, error) {
	var payload jsonPayload
	if err := json.NewDecoder(r).Decode(&payload); err != nil {
		return nil, fmt.Errorf("json adapter: decode: %w", err)
	}

	res := &Result{}
	now := time.Now().UTC()

	for _, jn := range payload.Nodes {
		if jn.ID == "" {
			res.Warns = append(res.Warns, "skipping node with empty ID")
			continue
		}
		n := model.NewNode(jn.ID, model.NodeType(jn.Type), jn.Name)
		n.Tags = jn.Tags
		if jn.Attributes != nil {
			n.Attributes = jn.Attributes
		}
		n.FirstSeen = now
		n.LastSeen = now
		res.Nodes = append(res.Nodes, n)
	}

	for _, je := range payload.Edges {
		if je.ID == "" || je.Source == "" || je.Target == "" {
			res.Warns = append(res.Warns, fmt.Sprintf("skipping incomplete edge: %+v", je))
			continue
		}
		e := model.NewEdge(je.ID, model.EdgeType(je.Type), je.Source, je.Target)
		if je.Confidence != nil {
			e.Confidence = *je.Confidence
		}
		if je.Exploitability != nil {
			e.Exploitability = *je.Exploitability
		}
		if je.Detectability != nil {
			e.Detectability = *je.Detectability
		}
		if je.BlastRadius != nil {
			e.BlastRadius = *je.BlastRadius
		}
		res.Edges = append(res.Edges, e)
	}

	return res, nil
}

// BloodHoundAdapter parses the read-only BloodHound JSON export format.
// Supports Users.json, Groups.json, Computers.json from BH collectors.
type BloodHoundAdapter struct{}

func (a *BloodHoundAdapter) Name() string { return "bloodhound-json" }

type bhPayload struct {
	Meta  bhMeta   `json:"meta"`
	Users []bhUser `json:"data"`
}

type bhMeta struct {
	Type    string `json:"type"`
	Count   int    `json:"count"`
	Version int    `json:"version"`
}

type bhUser struct {
	Properties       bhProperties `json:"Properties"`
	ObjectIdentifier string       `json:"ObjectIdentifier"`
	Members          []bhMember   `json:"Members"`
	PrimaryGroupSid  string       `json:"PrimaryGroupSid"`
}

type bhMember struct {
	MemberType       string `json:"MemberType"`
	ObjectIdentifier string `json:"ObjectIdentifier"`
}

type bhProperties struct {
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	Enabled     bool   `json:"enabled"`
	AdminCount  bool   `json:"admincount"`
	ObjectSid   string `json:"objectsid"`
	Description string `json:"description"`
}

func (a *BloodHoundAdapter) Ingest(r io.Reader) (*Result, error) {
	var payload bhPayload
	if err := json.NewDecoder(r).Decode(&payload); err != nil {
		return nil, fmt.Errorf("bloodhound adapter: decode: %w", err)
	}

	res := &Result{}
	now := time.Now().UTC()
	nodeType := bhMetaTypeToNodeType(payload.Meta.Type)
	edgeSeq := 0

	for _, u := range payload.Users {
		id := u.ObjectIdentifier
		if id == "" {
			res.Warns = append(res.Warns, "skipping BH entry with empty ObjectIdentifier")
			continue
		}
		name := u.Properties.Name
		if name == "" {
			name = id
		}
		n := model.NewNode(id, nodeType, name)
		n.Attributes["domain"] = u.Properties.Domain
		n.Attributes["enabled"] = u.Properties.Enabled
		n.Attributes["admin_count"] = u.Properties.AdminCount
		if u.Properties.AdminCount {
			n.Tags = append(n.Tags, "admin_count")
		}
		n.FirstSeen = now
		n.LastSeen = now
		res.Nodes = append(res.Nodes, n)

		for _, m := range u.Members {
			eid := fmt.Sprintf("bh-mem-%d", edgeSeq)
			edgeSeq++
			e := model.NewEdge(eid, model.EdgeMemberOf, m.ObjectIdentifier, id)
			res.Edges = append(res.Edges, e)
		}
	}

	return res, nil
}

func bhMetaTypeToNodeType(t string) model.NodeType {
	switch t {
	case "users":
		return model.NodeUser
	case "groups":
		return model.NodeGroup
	case "computers":
		return model.NodeComputer
	default:
		return model.NodeUser
	}
}
