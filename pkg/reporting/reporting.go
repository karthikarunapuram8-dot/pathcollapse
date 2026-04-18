// Package reporting generates executive and engineer reports from graph analysis results.
package reporting

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/karunapuram/pathcollapse/pkg/controls"
	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/scoring"
)

// Format selects the output format.
type Format string

const (
	FormatMarkdown Format = "markdown"
	FormatJSON     Format = "json"
)

// Report bundles all analysis outputs for rendering.
type Report struct {
	GeneratedAt     time.Time                        `json:"generated_at"`
	NodeCount       int                              `json:"node_count"`
	EdgeCount       int                              `json:"edge_count"`
	TopPaths        []scoring.ScoredPath             `json:"top_paths"`
	Recommendations []controls.ControlRecommendation `json:"recommendations"`
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

	if len(rep.Recommendations) > 0 {
		fmt.Fprintf(sb, "## Control Recommendations\n\n")
		fmt.Fprintf(sb, "Ordered by paths collapsed (most impact first).\n\n")
		for i, rec := range rep.Recommendations {
			fmt.Fprintf(sb, "### %d. %s\n\n", i+1, rec.Change.Description)
			fmt.Fprintf(sb, "- **Type**: `%s`\n", rec.Change.Type)
			fmt.Fprintf(sb, "- **Paths collapsed**: %d\n", rec.PathsRemoved)
			fmt.Fprintf(sb, "- **Risk reduction**: %.3f\n", rec.RiskReduction)
			fmt.Fprintf(sb, "- **Difficulty**: %s\n", rec.Difficulty)
			fmt.Fprintf(sb, "- **Confidence**: %.0f%%\n\n", rec.Confidence*100)
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
	return &Report{
		GeneratedAt:     time.Now().UTC(),
		NodeCount:       g.NodeCount(),
		EdgeCount:       g.EdgeCount(),
		TopPaths:        topPaths,
		Recommendations: recs,
	}
}
