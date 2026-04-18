package reporting

import (
	"fmt"
	"sort"
	"strings"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/confidence"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/controls"
)

// ConfidenceSummary aggregates confidence statistics across the
// recommendations in a report. It is only populated when at least one
// recommendation carries a Breakdown; otherwise HasConfidence is false
// and the summary is hidden from the rendered report.
type ConfidenceSummary struct {
	HasConfidence bool `json:"has_confidence"`

	// Count of recommendations that actually carry a Breakdown — may be
	// less than len(Recommendations) if some entries fall back to legacy
	// scoring mid-batch (e.g. edge-not-found).
	Count int `json:"count"`

	Average float64 `json:"average"`
	Highest float64 `json:"highest"`
	Lowest  float64 `json:"lowest"`

	ColdStart  int `json:"cold_start"`
	Partial    int `json:"partial"`
	Calibrated int `json:"calibrated"`
}

// BuildConfidenceSummary scans recs and returns an aggregate summary.
// Returns a zero ConfidenceSummary (HasConfidence=false) when no rec
// carries a Breakdown, so callers can unconditionally include it in the
// Report and let the renderers skip empty summaries.
func BuildConfidenceSummary(recs []controls.ControlRecommendation) ConfidenceSummary {
	var s ConfidenceSummary
	s.Lowest = 1.0

	for _, r := range recs {
		if r.Breakdown == nil {
			continue
		}
		s.Count++
		s.Average += r.Confidence
		if r.Confidence > s.Highest {
			s.Highest = r.Confidence
		}
		if r.Confidence < s.Lowest {
			s.Lowest = r.Confidence
		}
		switch r.Regime {
		case confidence.RegimeCalibrated:
			s.Calibrated++
		case confidence.RegimePartial:
			s.Partial++
		default:
			s.ColdStart++
		}
	}

	if s.Count == 0 {
		return ConfidenceSummary{} // HasConfidence zero-value
	}
	s.HasConfidence = true
	s.Average /= float64(s.Count)
	return s
}

// TopDrivers returns the two highest-valued factor names from a Breakdown,
// formatted for terse display in a "Why?" column. Returns a single-entry
// slice when the breakdown is nil.
//
// Example output: ["robustness 0.92", "coverage 0.88"].
func TopDrivers(b *confidence.Breakdown) []string {
	if b == nil {
		return nil
	}
	type factor struct {
		name  string
		value float64
	}
	all := []factor{
		{"evidence", b.Evidence},
		{"robustness", b.Robustness},
		{"safety", b.Safety},
		{"temporal", b.TemporalStability},
		{"coverage", b.CoverageConcentration},
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].value > all[j].value })

	n := 2
	if len(all) < n {
		n = len(all)
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("%s %.2f", all[i].name, all[i].value))
	}
	return out
}

// LowestDriver returns the single weakest factor from a Breakdown as a
// short phrase, or "" if no Breakdown is present. Useful for flagging
// *why* a recommendation's confidence is not higher, which is often more
// actionable than seeing the top drivers.
func LowestDriver(b *confidence.Breakdown) string {
	if b == nil {
		return ""
	}
	type factor struct {
		name  string
		value float64
	}
	all := []factor{
		{"evidence", b.Evidence},
		{"robustness", b.Robustness},
		{"safety", b.Safety},
		{"temporal", b.TemporalStability},
		{"coverage", b.CoverageConcentration},
	}
	low := all[0]
	for _, f := range all[1:] {
		if f.value < low.value {
			low = f
		}
	}
	return fmt.Sprintf("%s %.2f", low.name, low.value)
}

// joinDrivers renders a slice from TopDrivers as a comma-separated string.
func joinDrivers(ds []string) string {
	return strings.Join(ds, ", ")
}
