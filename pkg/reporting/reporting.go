// Package reporting generates executive and engineer reports from graph analysis results.
package reporting

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/karunapuram/pathcollapse/pkg/controls"
	"github.com/karunapuram/pathcollapse/pkg/detection"
	"github.com/karunapuram/pathcollapse/pkg/drift"
	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/scoring"
	"github.com/karunapuram/pathcollapse/pkg/telemetry"
)

// Format selects the output format.
type Format string

const (
	FormatMarkdown Format = "markdown"
	FormatJSON     Format = "json"
	FormatHTML     Format = "html"
)

// Report bundles all analysis outputs for rendering.
type Report struct {
	GeneratedAt     time.Time                        `json:"generated_at"`
	NodeCount       int                              `json:"node_count"`
	EdgeCount       int                              `json:"edge_count"`
	TopPaths        []scoring.ScoredPath             `json:"top_paths"`
	PathDetails     []PathDetail                     `json:"path_details,omitempty"`
	Recommendations []controls.ControlRecommendation `json:"recommendations"`
	Confidence      ConfidenceSummary                `json:"confidence_summary,omitempty"`
	Drift           *drift.DriftReport               `json:"drift,omitempty"`
}

// PathDetail enriches a scored path with detection and telemetry guidance.
type PathDetail struct {
	Rank      int                         `json:"rank"`
	Score     float64                     `json:"score"`
	Source    string                      `json:"source"`
	Target    string                      `json:"target"`
	HopCount  int                         `json:"hop_count"`
	Detection detection.DetectionArtefact `json:"detection"`
	Telemetry []telemetry.Requirement     `json:"telemetry"`
}

// Reporter renders reports to an io.Writer in the chosen format.
type Reporter struct {
	format Format
}

// New returns a Reporter for the given format.
func New(format Format) *Reporter {
	return &Reporter{format: format}
}

// Render writes the report to w.
func (r *Reporter) Render(w io.Writer, rep *Report) error {
	switch r.format {
	case FormatJSON:
		return r.renderJSON(w, rep)
	case FormatHTML:
		return r.renderHTML(w, rep)
	default:
		return r.renderMarkdown(w, rep)
	}
}

func (r *Reporter) renderJSON(w io.Writer, rep *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

func (r *Reporter) renderMarkdown(w io.Writer, rep *Report) error {
	sb := &strings.Builder{}

	fmt.Fprintf(sb, "# PathCollapse Analysis Report\n\n")
	fmt.Fprintf(sb, "_Generated: %s_\n\n", rep.GeneratedAt.Format(time.RFC1123))
	fmt.Fprintf(sb, "## Graph Summary\n\n")
	fmt.Fprintf(sb, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(sb, "| Nodes | %d |\n", rep.NodeCount)
	fmt.Fprintf(sb, "| Edges | %d |\n", rep.EdgeCount)
	fmt.Fprintf(sb, "| Paths analysed | %d |\n", len(rep.TopPaths))
	fmt.Fprintf(sb, "| Control recommendations | %d |\n\n", len(rep.Recommendations))

	if len(rep.TopPaths) > 0 {
		fmt.Fprintf(sb, "## Top Risk Paths\n\n")
		fmt.Fprintf(sb, "| # | Risk Score | Length | Source → Target |\n")
		fmt.Fprintf(sb, "|---|------------|--------|------------------|\n")
		for i, sp := range rep.TopPaths {
			src := ""
			tgt := ""
			if sp.Path.Source() != nil {
				src = sp.Path.Source().Name
			}
			if sp.Path.Target() != nil {
				tgt = sp.Path.Target().Name
			}
			fmt.Fprintf(sb, "| %d | %.3f | %d | %s → %s |\n",
				i+1, sp.Score, sp.Path.Len(), src, tgt)
		}
		sb.WriteString("\n")
	}

	if rep.Confidence.HasConfidence {
		c := rep.Confidence
		fmt.Fprintf(sb, "## Confidence Summary\n\n")
		fmt.Fprintf(sb, "| Metric | Value |\n|--------|-------|\n")
		fmt.Fprintf(sb, "| Recommendations scored | %d |\n", c.Count)
		fmt.Fprintf(sb, "| Average confidence | %.0f%% |\n", c.Average*100)
		fmt.Fprintf(sb, "| Highest | %.0f%% |\n", c.Highest*100)
		fmt.Fprintf(sb, "| Lowest | %.0f%% |\n", c.Lowest*100)
		fmt.Fprintf(sb, "| Regime | %d cold-start · %d partial · %d calibrated |\n\n",
			c.ColdStart, c.Partial, c.Calibrated)
	}

	if len(rep.Recommendations) > 0 {
		fmt.Fprintf(sb, "## Control Recommendations\n\n")
		fmt.Fprintf(sb, "Ordered by paths collapsed (most impact first).\n\n")
		for i, rec := range rep.Recommendations {
			fmt.Fprintf(sb, "### %d. %s\n\n", i+1, rec.Change.Description)
			fmt.Fprintf(sb, "- **Type**: `%s`\n", rec.Change.Type)
			fmt.Fprintf(sb, "- **Paths collapsed**: %d\n", rec.PathsRemoved)
			fmt.Fprintf(sb, "- **Risk reduction**: %.3f\n", rec.RiskReduction)
			fmt.Fprintf(sb, "- **Difficulty**: %s\n", rec.Difficulty)
			fmt.Fprintf(sb, "- **Confidence**: %.0f%%\n", rec.Confidence*100)
			if drivers := TopDrivers(rec.Breakdown); len(drivers) > 0 {
				fmt.Fprintf(sb, "- **Drivers**: %s\n", joinDrivers(drivers))
				if low := LowestDriver(rec.Breakdown); low != "" {
					fmt.Fprintf(sb, "- **Weakest factor**: %s\n", low)
				}
			}
			sb.WriteString("\n")
		}
	}

	if len(rep.PathDetails) > 0 {
		fmt.Fprintf(sb, "## Detection And Telemetry\n\n")
		fmt.Fprintf(sb, "Detection engineering guidance for the highest-risk paths.\n\n")
		for _, detail := range rep.PathDetails {
			fmt.Fprintf(sb, "### Path %d: %s → %s\n\n", detail.Rank, detail.Source, detail.Target)
			fmt.Fprintf(sb, "- **Risk score**: %.3f\n", detail.Score)
			fmt.Fprintf(sb, "- **Hop count**: %d\n", detail.HopCount)
			fmt.Fprintf(sb, "- **ATT&CK techniques**: %s\n", joinTechniques(detail.Detection.ATTACKTechniques))
			fmt.Fprintf(sb, "- **Log sources**: %s\n", strings.Join(detail.Detection.LogSources, ", "))
			fmt.Fprintf(sb, "- **Telemetry requirements**: %s\n", joinTelemetry(detail.Telemetry))
			if len(detail.Detection.MissingTelemetry) > 0 {
				fmt.Fprintf(sb, "- **Known telemetry gaps**: %s\n", strings.Join(detail.Detection.MissingTelemetry, ", "))
			}
			sb.WriteString("\n```yaml\n")
			sb.WriteString(detail.Detection.SigmaRule)
			sb.WriteString("```\n\n")
			fmt.Fprintf(sb, "```kusto\n%s\n```\n\n", detail.Detection.KQLQuery)
			fmt.Fprintf(sb, "```spl\n%s\n```\n\n", detail.Detection.SPLQuery)
		}
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

// ExecutiveSummary writes a brief executive-level summary.
func ExecutiveSummary(w io.Writer, rep *Report) error {
	if len(rep.TopPaths) == 0 {
		fmt.Fprintf(w, "No high-risk paths detected in the analysed environment.\n")
		return nil
	}

	topScore := rep.TopPaths[0].Score
	fmt.Fprintf(w, "## Executive Summary\n\n")
	fmt.Fprintf(w, "PathCollapse identified **%d high-risk lateral movement paths** across %d nodes.\n\n",
		len(rep.TopPaths), rep.NodeCount)
	fmt.Fprintf(w, "The highest-risk path carries a score of **%.2f/1.00**.\n\n", topScore)

	if len(rep.Recommendations) > 0 {
		fmt.Fprintf(w, "**Top recommended control**: %s\n", rep.Recommendations[0].Change.Description)
		fmt.Fprintf(w, "This single change would collapse **%d paths** and reduce aggregate risk by **%.2f**.\n\n",
			rep.Recommendations[0].PathsRemoved, rep.Recommendations[0].RiskReduction)
	}

	return nil
}

// BuildReport constructs a Report from graph + analysis outputs.
func BuildReport(g *graph.Graph, topPaths []scoring.ScoredPath, recs []controls.ControlRecommendation) *Report {
	pathDetails := make([]PathDetail, 0, len(topPaths))
	for i, sp := range topPaths {
		pathDetails = append(pathDetails, PathDetail{
			Rank:      i + 1,
			Score:     sp.Score,
			Source:    nodeName(sp.Path.Source()),
			Target:    nodeName(sp.Path.Target()),
			HopCount:  sp.Path.Len(),
			Detection: detection.MapPath(sp.Path),
			Telemetry: telemetry.ForPath(sp.Path),
		})
	}

	return &Report{
		GeneratedAt:     time.Now().UTC(),
		NodeCount:       g.NodeCount(),
		EdgeCount:       g.EdgeCount(),
		TopPaths:        topPaths,
		PathDetails:     pathDetails,
		Recommendations: recs,
		Confidence:      BuildConfidenceSummary(recs),
	}
}

func joinTechniques(ts []detection.ATTACKTechnique) string {
	if len(ts) == 0 {
		return "None mapped"
	}
	parts := make([]string, 0, len(ts))
	for _, t := range ts {
		parts = append(parts, fmt.Sprintf("%s (%s)", t.ID, t.Name))
	}
	return strings.Join(parts, ", ")
}

func joinTelemetry(reqs []telemetry.Requirement) string {
	if len(reqs) == 0 {
		return "None specified"
	}
	parts := make([]string, 0, len(reqs))
	for _, req := range reqs {
		entry := fmt.Sprintf("%s: %s", req.Source, req.Description)
		if len(req.EventIDs) > 0 {
			entry = fmt.Sprintf("%s (Event IDs %s)", entry, joinEventIDs(req.EventIDs))
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, "; ")
}

func joinEventIDs(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return strings.Join(parts, ", ")
}
