// Package telemetry defines expected telemetry requirements for graph edges and paths.
package telemetry

import (
	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/model"
)

// Requirement describes a telemetry source needed to detect an edge traversal.
type Requirement struct {
	Source      string `json:"source"`
	EventIDs    []int  `json:"event_ids,omitempty"`
	Description string `json:"description"`
	Priority    string `json:"priority"` // critical, high, medium, low
}

// ForEdge returns telemetry requirements for a single edge type.
func ForEdge(et model.EdgeType) []Requirement {
	switch et {
	case model.EdgeAdminTo, model.EdgeLocalAdminTo:
		return []Requirement{
			{Source: "Windows Security", EventIDs: []int{4624, 4648, 4672}, Description: "Privileged logon monitoring", Priority: "critical"},
			{Source: "Sysmon", EventIDs: []int{1}, Description: "Process creation on target host", Priority: "high"},
		}
	case model.EdgeMemberOf:
		return []Requirement{
			{Source: "Windows Security", EventIDs: []int{4728, 4732, 4756}, Description: "Group membership changes", Priority: "high"},
		}
	case model.EdgeCanSyncTo:
		return []Requirement{
			{Source: "Windows Security", EventIDs: []int{4662}, Description: "Directory service access (DCSync)", Priority: "critical"},
		}
	case model.EdgeCanDelegateTo:
		return []Requirement{
			{Source: "Windows Security", EventIDs: []int{4769}, Description: "Kerberos service ticket requests", Priority: "high"},
		}
	case model.EdgeCanEnrollIn:
		return []Requirement{
			{Source: "Certificate Services", EventIDs: []int{4886, 4887}, Description: "Certificate enrollment events", Priority: "medium"},
		}
	case model.EdgeHasSessionOn:
		return []Requirement{
			{Source: "Windows Security", EventIDs: []int{4624}, Description: "Interactive/network logons", Priority: "high"},
		}
	default:
		return []Requirement{
			{Source: "Windows Security", Description: "General event log monitoring", Priority: "low"},
		}
	}
}

// ForPath returns the union of telemetry requirements for all edges in a path,
// deduplicated by source+description.
func ForPath(p graph.Path) []Requirement {
	seen := map[string]bool{}
	var out []Requirement
	for _, e := range p.Edges {
		for _, req := range ForEdge(e.Type) {
			key := req.Source + "|" + req.Description
			if !seen[key] {
				seen[key] = true
				out = append(out, req)
			}
		}
	}
	return out
}
