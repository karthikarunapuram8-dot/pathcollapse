package reporting

import (
	"fmt"
	"io"
	"strings"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/controls"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
)

// renderHTML writes a single-file self-contained HTML report with inline CSS.
// No external JS or CSS dependencies — suitable for offline CISO review.
func (r *Reporter) renderHTML(w io.Writer, rep *Report) error {
	sb := &strings.Builder{}

	topScore := 0.0
	if len(rep.TopPaths) > 0 {
		topScore = rep.TopPaths[0].Score
	}

	riskClass, riskLabel := htmlRiskBadge(topScore)

	// WriteString avoids misinterpreting literal % chars in CSS.
	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>PathCollapse Security Report</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:system-ui,-apple-system,sans-serif;background:#f0f2f5;color:#1a202c;line-height:1.6}
header{background:#1a1f36;color:#fff;padding:24px 40px;display:flex;justify-content:space-between;align-items:center}
header h1{font-size:1.5rem;font-weight:700;letter-spacing:-.5px}
header .meta{font-size:.85rem;opacity:.7;text-align:right}
main{max-width:1100px;margin:32px auto;padding:0 24px}
.card{background:#fff;border-radius:10px;box-shadow:0 1px 4px rgba(0,0,0,.08);margin-bottom:24px;overflow:hidden}
.card-header{padding:16px 24px;border-bottom:1px solid #e2e8f0;display:flex;align-items:center;gap:12px}
.card-header h2{font-size:1.1rem;font-weight:600;color:#2d3748}
.card-body{padding:24px}
.kpi-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:16px;margin-bottom:24px}
.kpi{background:#f7fafc;border-radius:8px;padding:16px 20px;border-left:4px solid #4299e1}
.kpi.danger{border-left-color:#e53e3e}
.kpi .label{font-size:.8rem;text-transform:uppercase;letter-spacing:.05em;color:#718096;margin-bottom:4px}
.kpi .value{font-size:1.8rem;font-weight:700;color:#1a202c}
.kpi .sub{font-size:.8rem;color:#718096}
.risk-badge{display:inline-block;padding:4px 12px;border-radius:20px;font-size:.8rem;font-weight:600;text-transform:uppercase;letter-spacing:.05em}
.risk-critical{background:#fff5f5;color:#c53030;border:1px solid #fed7d7}
.risk-high{background:#fffaf0;color:#c05621;border:1px solid #feebc8}
.risk-medium{background:#fffff0;color:#975a16;border:1px solid #fefcbf}
.risk-low{background:#f0fff4;color:#276749;border:1px solid #c6f6d5}
table{width:100%;border-collapse:collapse;font-size:.9rem}
th{background:#f7fafc;padding:10px 14px;text-align:left;font-weight:600;font-size:.8rem;text-transform:uppercase;letter-spacing:.05em;color:#4a5568;border-bottom:2px solid #e2e8f0}
td{padding:10px 14px;border-bottom:1px solid #f0f2f5;vertical-align:middle}
tr:last-child td{border-bottom:none}
tr:hover td{background:#fafafa}
.score-bar{position:relative;height:8px;background:#e2e8f0;border-radius:4px;width:100px;display:inline-block;vertical-align:middle}
.score-fill{height:100%;border-radius:4px;background:linear-gradient(90deg,#48bb78,#ed8936,#e53e3e)}
.pill{display:inline-block;padding:2px 8px;border-radius:10px;font-size:.78rem;font-weight:600}
.pill-high{background:#fff5f5;color:#c53030}
.pill-medium{background:#fffaf0;color:#c05621}
.pill-low{background:#f0fff4;color:#276749}
.pill-info{background:#ebf8ff;color:#2b6cb0}
.section-empty{padding:20px 0;color:#a0aec0;font-style:italic;text-align:center}
.exec-text{font-size:1rem;color:#4a5568;margin-bottom:16px}
.exec-callout{background:#ebf8ff;border-left:4px solid #4299e1;border-radius:0 8px 8px 0;padding:12px 16px;margin-top:16px;font-size:.9rem;color:#2c5282}
.stack{display:grid;gap:18px}
.path-card{border:1px solid #e2e8f0;border-radius:10px;padding:18px;background:#f8fafc}
.path-card h3{font-size:1rem;margin-bottom:8px;color:#1a202c}
.meta-row{font-size:.88rem;color:#4a5568;margin-bottom:10px}
.detail-list{margin:0 0 12px 18px;color:#2d3748}
.detail-list li{margin-bottom:6px}
pre{background:#0f172a;color:#e2e8f0;padding:14px;border-radius:8px;overflow:auto;font-size:.8rem;line-height:1.5;margin-top:10px}
.drift-stat{display:inline-block;margin-right:24px;font-size:.9rem}
.drift-stat strong{font-weight:700}
footer{text-align:center;font-size:.8rem;color:#a0aec0;margin:32px 0;padding-bottom:24px}
</style>
</head>
<body>
<header>
  <div>
    <h1>PathCollapse &mdash; Security Report</h1>
    <div style="font-size:.9rem;opacity:.8;margin-top:4px">Identity Exposure Analysis</div>
  </div>
  <div class="meta">Generated<br>`)
	sb.WriteString(rep.GeneratedAt.Format("2006-01-02 15:04 UTC"))
	sb.WriteString("</div>\n</header>\n<main>\n")

	// ── Executive Summary ────────────────────────────────────────────────────
	fmt.Fprintf(sb, "<div class=\"card\">\n  <div class=\"card-header\"><h2>Executive Summary</h2><span class=\"risk-badge %s\">%s Risk</span></div>\n  <div class=\"card-body\">\n    <div class=\"kpi-grid\">\n      <div class=\"kpi\"><div class=\"label\">Nodes Analysed</div><div class=\"value\">%d</div><div class=\"sub\">identity graph vertices</div></div>\n      <div class=\"kpi\"><div class=\"label\">Relationships</div><div class=\"value\">%d</div><div class=\"sub\">typed privilege edges</div></div>\n      <div class=\"kpi danger\"><div class=\"label\">High-Risk Paths</div><div class=\"value\">%d</div><div class=\"sub\">attack chains identified</div></div>\n      <div class=\"kpi danger\"><div class=\"label\">Top Risk Score</div><div class=\"value\">%.2f</div><div class=\"sub\">max 1.00</div></div>\n    </div>\n",
		riskClass, riskLabel, rep.NodeCount, rep.EdgeCount, len(rep.TopPaths), topScore)

	if len(rep.TopPaths) > 0 {
		sp := rep.TopPaths[0]
		src := htmlEsc(nodeName(sp.Path.Source()))
		tgt := htmlEsc(nodeName(sp.Path.Target()))
		fmt.Fprintf(sb, "    <p class=\"exec-text\">PathCollapse identified <strong>%d lateral-movement attack chains</strong> across %d identity graph nodes. The highest-risk path (%s &rarr; %s) carries a composite risk score of <strong>%.2f / 1.00</strong>.</p>\n",
			len(rep.TopPaths), rep.NodeCount, src, tgt, topScore)
	} else {
		sb.WriteString("    <p class=\"exec-text\">No high-risk paths were detected in the analysed environment.</p>\n")
	}

	if len(rep.Recommendations) > 0 {
		rec := rep.Recommendations[0]
		fmt.Fprintf(sb, "    <div class=\"exec-callout\"><strong>Top recommended control:</strong> %s<br>This single change collapses <strong>%d attack paths</strong> and reduces aggregate risk by <strong>%.2f</strong>.</div>\n",
			htmlEsc(rec.Change.Description), rec.PathsRemoved, rec.RiskReduction)
	}
	sb.WriteString("  </div>\n</div>\n\n")

	// ── Confidence Summary ───────────────────────────────────────────────────
	if rep.Confidence.HasConfidence {
		c := rep.Confidence
		sb.WriteString("<div class=\"card\">\n  <div class=\"card-header\"><h2>Recommendation Confidence</h2></div>\n  <div class=\"card-body\">\n    <div class=\"kpi-grid\">\n")
		fmt.Fprintf(sb,
			"      <div class=\"kpi\"><div class=\"label\">Scored</div><div class=\"value\">%d</div><div class=\"sub\">recommendations</div></div>\n"+
				"      <div class=\"kpi\"><div class=\"label\">Average</div><div class=\"value\">%.0f%%</div><div class=\"sub\">across scored recs</div></div>\n"+
				"      <div class=\"kpi\"><div class=\"label\">Highest</div><div class=\"value\">%.0f%%</div><div class=\"sub\">most confident rec</div></div>\n"+
				"      <div class=\"kpi\"><div class=\"label\">Lowest</div><div class=\"value\">%.0f%%</div><div class=\"sub\">least confident rec</div></div>\n",
			c.Count, c.Average*100, c.Highest*100, c.Lowest*100)
		sb.WriteString("    </div>\n")
		fmt.Fprintf(sb,
			"    <p style=\"margin-top:12px;font-size:.88rem;color:#4a5568\">Calibration regime: "+
				"<strong>%d</strong> cold-start &middot; <strong>%d</strong> partial &middot; <strong>%d</strong> calibrated. "+
				"See <code>docs/confidence.md</code> for the scoring algorithm.</p>\n",
			c.ColdStart, c.Partial, c.Calibrated)
		sb.WriteString("  </div>\n</div>\n\n")
	}

	// ── Top Risk Paths ───────────────────────────────────────────────────────
	sb.WriteString("<div class=\"card\">\n  <div class=\"card-header\"><h2>Top Risk Paths</h2></div>\n  <div class=\"card-body\">\n")
	if len(rep.TopPaths) == 0 {
		sb.WriteString("    <p class=\"section-empty\">No paths to display.</p>\n")
	} else {
		sb.WriteString("    <table>\n      <thead><tr><th>#</th><th>Risk Score</th><th>Length</th><th>Source &rarr; Target</th></tr></thead>\n      <tbody>\n")
		for i, sp := range rep.TopPaths {
			src := htmlEsc(nodeName(sp.Path.Source()))
			tgt := htmlEsc(nodeName(sp.Path.Target()))
			fillPct := int(sp.Score * 100)
			fmt.Fprintf(sb, "        <tr><td>%d</td><td><div class=\"score-bar\"><div class=\"score-fill\" style=\"width:%d%%\"></div></div> <strong>%.3f</strong></td><td>%d</td><td>%s &rarr; %s</td></tr>\n",
				i+1, fillPct, sp.Score, sp.Path.Len(), src, tgt)
		}
		sb.WriteString("      </tbody>\n    </table>\n")
	}
	sb.WriteString("  </div>\n</div>\n\n")

	// ── Control Recommendations / Breakpoints ────────────────────────────────
	sb.WriteString("<div class=\"card\">\n  <div class=\"card-header\"><h2>Recommended Controls</h2></div>\n  <div class=\"card-body\">\n")
	if len(rep.Recommendations) == 0 {
		sb.WriteString("    <p class=\"section-empty\">No control recommendations generated.</p>\n")
	} else {
		sb.WriteString("    <table>\n      <thead><tr><th>#</th><th>Action</th><th>Type</th><th>Paths Collapsed</th><th>Risk Reduction</th><th>Difficulty</th><th>Confidence</th><th>Why</th></tr></thead>\n      <tbody>\n")
		for i, rec := range rep.Recommendations {
			diffClass := "pill-low"
			switch rec.Difficulty {
			case controls.DifficultyMedium:
				diffClass = "pill-medium"
			case controls.DifficultyHigh:
				diffClass = "pill-high"
			}
			why := htmlEsc(joinDrivers(TopDrivers(rec.Breakdown)))
			if why == "" {
				why = "<span style=\"color:#a0aec0\">&mdash;</span>"
			}
			fmt.Fprintf(sb, "        <tr><td>%d</td><td>%s</td><td><code>%s</code></td><td>%d</td><td>%.3f</td><td><span class=\"pill %s\">%s</span></td><td>%.0f%%</td><td style=\"font-size:.82rem;color:#4a5568\">%s</td></tr>\n",
				i+1, htmlEsc(rec.Change.Description), rec.Change.Type,
				rec.PathsRemoved, rec.RiskReduction,
				diffClass, rec.Difficulty, rec.Confidence*100, why)
		}
		sb.WriteString("      </tbody>\n    </table>\n")
	}
	sb.WriteString("  </div>\n</div>\n\n")

	// ── Detection & Telemetry ────────────────────────────────────────────────
	sb.WriteString("<div class=\"card\">\n  <div class=\"card-header\"><h2>Detection &amp; Telemetry</h2></div>\n  <div class=\"card-body\">\n")
	if len(rep.PathDetails) == 0 {
		sb.WriteString("    <p class=\"section-empty\">No path detections to display.</p>\n")
	} else {
		sb.WriteString("    <div class=\"stack\">\n")
		for _, detail := range rep.PathDetails {
			fmt.Fprintf(sb, "      <div class=\"path-card\">\n        <h3>Path %d: %s &rarr; %s</h3>\n        <div class=\"meta-row\">Risk %.3f &middot; %d hops</div>\n",
				detail.Rank, htmlEsc(detail.Source), htmlEsc(detail.Target), detail.Score, detail.HopCount)
			sb.WriteString("        <ul class=\"detail-list\">\n")
			fmt.Fprintf(sb, "          <li><strong>ATT&amp;CK:</strong> %s</li>\n", htmlEsc(joinTechniques(detail.Detection.ATTACKTechniques)))
			fmt.Fprintf(sb, "          <li><strong>Log sources:</strong> %s</li>\n", htmlEsc(strings.Join(detail.Detection.LogSources, ", ")))
			fmt.Fprintf(sb, "          <li><strong>Telemetry requirements:</strong> %s</li>\n", htmlEsc(joinTelemetry(detail.Telemetry)))
			if len(detail.Detection.MissingTelemetry) > 0 {
				fmt.Fprintf(sb, "          <li><strong>Gaps:</strong> %s</li>\n", htmlEsc(strings.Join(detail.Detection.MissingTelemetry, ", ")))
			}
			sb.WriteString("        </ul>\n")
			fmt.Fprintf(sb, "        <pre>%s</pre>\n", htmlEsc(detail.Detection.SigmaRule))
			fmt.Fprintf(sb, "        <pre>%s</pre>\n", htmlEsc(detail.Detection.KQLQuery))
			fmt.Fprintf(sb, "        <pre>%s</pre>\n", htmlEsc(detail.Detection.SPLQuery))
			sb.WriteString("      </div>\n")
		}
		sb.WriteString("    </div>\n")
	}
	sb.WriteString("  </div>\n</div>\n\n")

	// ── Drift Analysis ───────────────────────────────────────────────────────
	sb.WriteString("<div class=\"card\">\n  <div class=\"card-header\"><h2>Drift Analysis</h2></div>\n  <div class=\"card-body\">\n")
	if rep.Drift == nil {
		sb.WriteString("    <p class=\"section-empty\">No baseline snapshot provided &mdash; run with <code>--baseline</code> to enable drift analysis.</p>\n")
	} else {
		d := rep.Drift
		fmt.Fprintf(sb, "    <p style=\"margin-bottom:16px\">\n      <span class=\"drift-stat\">Nodes added: <strong>%d</strong></span>\n      <span class=\"drift-stat\">Nodes removed: <strong>%d</strong></span>\n      <span class=\"drift-stat\">Edges added: <strong>%d</strong></span>\n      <span class=\"drift-stat\">Edges removed: <strong>%d</strong></span>\n    </p>\n",
			d.NodesAdded, d.NodesRemoved, d.EdgesAdded, d.EdgesRemoved)
		if len(d.Items) == 0 {
			sb.WriteString("    <p class=\"section-empty\">No security-relevant drift detected.</p>\n")
		} else {
			sb.WriteString("    <table>\n      <thead><tr><th>Severity</th><th>Category</th><th>Description</th></tr></thead>\n      <tbody>\n")
			for _, item := range d.Items {
				sevClass := "pill-info"
				switch item.Severity {
				case "high":
					sevClass = "pill-high"
				case "medium":
					sevClass = "pill-medium"
				}
				fmt.Fprintf(sb, "        <tr><td><span class=\"pill %s\">%s</span></td><td><code>%s</code></td><td>%s</td></tr>\n",
					sevClass, item.Severity, item.Category, htmlEsc(item.Description))
			}
			sb.WriteString("      </tbody>\n    </table>\n")
		}
	}
	sb.WriteString("  </div>\n</div>\n\n")

	fmt.Fprintf(sb, "<footer>Generated by <strong>PathCollapse</strong> &mdash; graph-based identity exposure analysis &mdash; %s</footer>\n</main>\n</body>\n</html>\n",
		rep.GeneratedAt.Format("2006-01-02"))

	_, err := io.WriteString(w, sb.String())
	return err
}

func htmlRiskBadge(score float64) (cls, label string) {
	switch {
	case score >= 0.7:
		return "risk-critical", "Critical"
	case score >= 0.5:
		return "risk-high", "High"
	case score >= 0.3:
		return "risk-medium", "Medium"
	default:
		return "risk-low", "Low"
	}
}

func htmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&#34;")
	return s
}

func nodeName(n *model.Node) string {
	if n == nil {
		return ""
	}
	return n.Name
}
