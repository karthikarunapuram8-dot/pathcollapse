package confidence

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ShadowEntry is a single JSONL record written when `breakpoints --shadow-mode`
// is used. Each recommendation emitted by the optimizer becomes one entry.
//
// The three *bool fields are initially nil and are later set by an operator
// (or an automated post-change check) once the collapse/regression outcome
// is known. Entries with all three nil are "unlabeled" and excluded from
// refitting.
//
// See docs/confidence.md §7 (Evaluation protocol) and docs/release-v0.2.1.md
// for the full shadow-mode workflow.
type ShadowEntry struct {
	Timestamp  time.Time `json:"ts"`
	EdgeID     string    `json:"edge_id"`
	EdgeSource string    `json:"edge_source"`
	EdgeTarget string    `json:"edge_target"`
	EdgeType   string    `json:"edge_type"`

	// Raw is the pre-calibration σ(z) score; Breakdown is the five-factor
	// decomposition that produced it.
	Raw       float64   `json:"raw"`
	Breakdown Breakdown `json:"breakdown"`

	// Post-change labels. Pointer-to-bool so nil cleanly represents
	// "not yet observed." Callers annotate after the collapse/regression
	// check has been performed (re-ingest and auth-log scan respectively).
	RecApplied         *bool `json:"rec_applied,omitempty"`
	ObservedCollapsed  *bool `json:"observed_collapsed,omitempty"`
	ObservedRegression *bool `json:"observed_regression,omitempty"`
}

// IsLabeled reports whether this entry carries enough post-change information
// to be used as a training example. The minimal required signal is
// ObservedCollapsed; ObservedRegression is optional (missing regression data
// is treated as "no regression" by Refit, consistent with the pragmatic
// limitation noted in docs/confidence.md §9.1).
func (e ShadowEntry) IsLabeled() bool {
	return e.ObservedCollapsed != nil
}

// Label returns the binary outcome Y derived from the entry's annotations.
//
//	Y = observed_collapsed AND (observed_regression == nil OR !observed_regression)
//
// A nil ObservedRegression is interpreted as "no regression observed" — the
// optimistic default in the collapse-only regime. IsLabeled must be true
// before calling; otherwise returns (0, false).
func (e ShadowEntry) Label() (y float64, ok bool) {
	if !e.IsLabeled() {
		return 0, false
	}
	collapsed := *e.ObservedCollapsed
	regressed := e.ObservedRegression != nil && *e.ObservedRegression
	if collapsed && !regressed {
		return 1, true
	}
	return 0, true
}

// DefaultShadowLogPath returns ~/.pathcollapse/shadow.jsonl.
// Callers create the parent directory before first write.
func DefaultShadowLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("confidence: locate home dir: %w", err)
	}
	return filepath.Join(home, ".pathcollapse", "shadow.jsonl"), nil
}

// AppendShadowEntry appends one JSONL-encoded entry to path. The parent
// directory is created if missing. Uses O_APPEND so line-oriented writes
// from multiple processes interleave cleanly per-line on POSIX and Windows
// for payloads below the OS's atomic-write threshold (typical entries are
// well under 4 KiB).
func AppendShadowEntry(path string, entry ShadowEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("confidence: create shadow log dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("confidence: open shadow log: %w", err)
	}
	defer f.Close()

	// Encode manually so we can add a trailing newline and avoid
	// json.Encoder's internal buffering quirks around O_APPEND.
	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("confidence: marshal shadow entry: %w", err)
	}
	b = append(b, '\n')
	if _, err := f.Write(b); err != nil {
		return fmt.Errorf("confidence: write shadow entry: %w", err)
	}
	return nil
}

// ReadStats reports the counts observed while reading a shadow log.
type ReadStats struct {
	TotalLines int
	Parsed     int
	Malformed  int
	Labeled    int
}

// ReadShadowLog reads every JSONL line from path and returns the parsed
// entries plus read statistics. Missing file is NOT an error — returns an
// empty result so callers can handle cold-start uniformly.
//
// Malformed lines are skipped and counted in ReadStats.Malformed; a single
// bad write from a crashed process should not disable the whole signal.
func ReadShadowLog(path string) ([]ShadowEntry, ReadStats, error) {
	var stats ReadStats

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, stats, nil
		}
		return nil, stats, fmt.Errorf("confidence: open shadow log: %w", err)
	}
	defer f.Close()

	var entries []ShadowEntry
	sc := bufio.NewScanner(f)
	// Allow long lines — a breakdown plus labels should fit in 4 KiB but
	// future schema additions could push it higher.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		stats.TotalLines++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e ShadowEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			stats.Malformed++
			continue
		}
		stats.Parsed++
		if e.IsLabeled() {
			stats.Labeled++
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return entries, stats, fmt.Errorf("confidence: scan shadow log: %w", err)
	}
	return entries, stats, nil
}

// NewShadowEntry bundles an edge and a breakdown into a ready-to-append
// entry. Timestamp defaults to time.Now().UTC() when zero.
func NewShadowEntry(
	edgeID, edgeSource, edgeTarget, edgeType string,
	raw float64,
	breakdown Breakdown,
	ts time.Time,
) ShadowEntry {
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	return ShadowEntry{
		Timestamp:  ts,
		EdgeID:     edgeID,
		EdgeSource: edgeSource,
		EdgeTarget: edgeTarget,
		EdgeType:   edgeType,
		Raw:        raw,
		Breakdown:  breakdown,
	}
}
