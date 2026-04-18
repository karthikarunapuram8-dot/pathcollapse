// Package detection maps high-risk paths to detection artefacts:
// required log sources, Sigma rules, KQL queries, SPL queries, and ATT&CK techniques.
package detection

import (
	"fmt"
	"strings"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
)

// ATTACKTechnique is an ATT&CK technique ID and name.
type ATTACKTechnique struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DetectionArtefact bundles all detection content for one path.
type DetectionArtefact struct {
	PathDescription string            `json:"path_description"`
	LogSources      []string          `json:"log_sources"`
	MissingTelemetry []string         `json:"missing_telemetry"`
	SigmaRule       string            `json:"sigma_rule"`
	KQLQuery        string            `json:"kql_query"`
	SPLQuery        string            `json:"spl_query"`
	ATTACKTechniques []ATTACKTechnique `json:"attack_techniques"`
}

// MapPath generates detection artefacts for a single path.
// Edge types are processed in path order so output is deterministic.
func MapPath(p graph.Path) DetectionArtefact {
	art := DetectionArtefact{
		PathDescription: describePath(p),
	}

	seenTypes := map[model.EdgeType]bool{}
	for _, e := range p.Edges {
		if seenTypes[e.Type] {
			continue
		}
		seenTypes[e.Type] = true
		art.LogSources = append(art.LogSources, logSourcesForEdge(e.Type)...)
		art.ATTACKTechniques = append(art.ATTACKTechniques, attackTechniquesForEdge(e.Type)...)
		art.MissingTelemetry = append(art.MissingTelemetry, missingTelemetryForEdge(e.Type)...)
	}

	art.LogSources = dedup(art.LogSources)
	art.MissingTelemetry = dedup(art.MissingTelemetry)
	art.ATTACKTechniques = dedupTechniques(art.ATTACKTechniques)

	art.SigmaRule = buildSigmaRule(p, art.ATTACKTechniques)
	art.KQLQuery = buildKQL(p)
	art.SPLQuery = buildSPL(p)

	return art
}

func describePath(p graph.Path) string {
	if len(p.Nodes) < 2 {
		return "unknown path"
	}
	src := p.Source()
	tgt := p.Target()
	return fmt.Sprintf("%s → %s (%d hops)", src.Name, tgt.Name, len(p.Edges))
}

func logSourcesForEdge(et model.EdgeType) []string {
	switch et {
	case model.EdgeMemberOf:
		return []string{"Windows Security EventID 4728", "Windows Security EventID 4756"}
	case model.EdgeAdminTo, model.EdgeLocalAdminTo:
		return []string{"Windows Security EventID 4672", "Windows Security EventID 4648"}
	case model.EdgeHasSessionOn:
		return []string{"Windows Security EventID 4624", "Windows Security EventID 4648"}
	case model.EdgeCanDelegateTo:
		return []string{"Windows Security EventID 4769", "Kerberos TGS requests"}
	case model.EdgeCanSyncTo:
		return []string{"Windows Security EventID 4662", "Directory Service Access logs"}
	case model.EdgeCanEnrollIn:
		return []string{"Certificate Services EventID 4886", "Certificate Services EventID 4887"}
	default:
		return []string{"Windows Security Event Log"}
	}
}

func attackTechniquesForEdge(et model.EdgeType) []ATTACKTechnique {
	switch et {
	case model.EdgeMemberOf:
		return []ATTACKTechnique{{ID: "T1078", Name: "Valid Accounts"}}
	case model.EdgeAdminTo, model.EdgeLocalAdminTo:
		return []ATTACKTechnique{{ID: "T1078.002", Name: "Domain Accounts"}, {ID: "T1021", Name: "Remote Services"}}
	case model.EdgeCanDelegateTo:
		return []ATTACKTechnique{{ID: "T1558.003", Name: "Kerberoasting"}, {ID: "T1550.003", Name: "Pass the Ticket"}}
	case model.EdgeCanSyncTo:
		return []ATTACKTechnique{{ID: "T1003.006", Name: "DCSync"}}
	case model.EdgeCanEnrollIn:
		return []ATTACKTechnique{{ID: "T1649", Name: "Steal or Forge Authentication Certificates"}}
	default:
		return nil
	}
}

func missingTelemetryForEdge(et model.EdgeType) []string {
	switch et {
	case model.EdgeCanSyncTo:
		return []string{"Directory replication monitoring not enabled"}
	case model.EdgeCanDelegateTo:
		return []string{"Kerberos delegation anomaly detection not deployed"}
	case model.EdgeCanEnrollIn:
		return []string{"Certificate template change monitoring not configured"}
	default:
		return nil
	}
}

func buildSigmaRule(p graph.Path, techniques []ATTACKTechnique) string {
	if len(p.Edges) == 0 {
		return ""
	}
	techIDs := make([]string, 0, len(techniques))
	for _, t := range techniques {
		techIDs = append(techIDs, t.ID)
	}
	return fmt.Sprintf(`title: PathCollapse - %s
status: experimental
description: Detects lateral movement path %s
tags:
  - attack.lateral_movement
  - %s
detection:
  selection:
    EventID:
      - 4624
      - 4648
      - 4672
  condition: selection
falsepositives:
  - Legitimate administrative activity
level: high
`, describePath(p), describePath(p), strings.Join(techIDs, "\n  - "))
}

func buildKQL(p graph.Path) string {
	if len(p.Nodes) == 0 {
		return ""
	}
	return fmt.Sprintf(
		`SecurityEvent
| where EventID in (4624, 4648, 4672)
| where TargetUserName has_any (%s)
| project TimeGenerated, EventID, TargetUserName, IpAddress, WorkstationName
| order by TimeGenerated desc`,
		nodeNameList(p.Nodes))
}

func buildSPL(p graph.Path) string {
	if len(p.Nodes) == 0 {
		return ""
	}
	return fmt.Sprintf(
		`index=wineventlog EventCode IN (4624,4648,4672)
(TargetUserName=%s)
| table _time, EventCode, TargetUserName, src_ip, dest_host
| sort -_time`,
		strings.Join(quoteNodes(p.Nodes), " OR TargetUserName="))
}

func nodeNameList(nodes []*model.Node) string {
	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		names = append(names, fmt.Sprintf("%q", n.Name))
	}
	return strings.Join(names, ", ")
}

func quoteNodes(nodes []*model.Node) []string {
	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		names = append(names, fmt.Sprintf("%q", n.Name))
	}
	return names
}

func dedup(s []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func dedupTechniques(ts []ATTACKTechnique) []ATTACKTechnique {
	seen := map[string]bool{}
	var out []ATTACKTechnique
	for _, t := range ts {
		if !seen[t.ID] {
			seen[t.ID] = true
			out = append(out, t)
		}
	}
	return out
}
