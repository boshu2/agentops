// Package findingsynth synthesizes review findings across N cross-family
// reviewer lanes: it deduplicates findings that describe the same substance and
// accumulates their cross-lane / cross-family attribution (corroboration). It is
// the layer BENEATH the whole-change decision (planpawl.Decide) — the decision
// stays where it is; this only reconciles the pile of per-lane finding notes into
// one deduplicated, attribution-carrying list.
//
// Typical use: fan a change out to several cross-family reviewer lanes, collect
// each lane's findings into a LaneFindings, then Merge them:
//
//	merged := findingsynth.Merge([]findingsynth.LaneFindings{claudeLane, geminiLane, gptLane})
//	for _, f := range merged { // ordered by corroboration then severity
//	    // f.Families / f.Lanes carry who reported it
//	}
package findingsynth

import (
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Finding is a normalized reviewer finding carrying accumulated attribution.
// Families records which model families reported it; Lanes records the reporting
// lane ids. After Merge both are sorted and unique.
type Finding struct {
	Title     string   `json:"title,omitempty"`
	Body      string   `json:"body,omitempty"`
	File      string   `json:"file,omitempty"`
	StartLine int      `json:"start_line,omitempty"`
	EndLine   int      `json:"end_line,omitempty"`
	Severity  string   `json:"severity,omitempty"`
	Families  []string `json:"families,omitempty"`
	Lanes     []string `json:"lanes,omitempty"`
}

// LaneFindings is one reviewer lane's raw findings, tagged with the lane id and
// the model family that produced them.
type LaneFindings struct {
	LaneID   string    `json:"lane_id"`
	Family   string    `json:"family"`
	Findings []Finding `json:"findings"`
}

// keySep separates the key components; NUL cannot appear in a normalized field.
const keySep = "\x00"

// Key returns the canonical dedup key for a finding: normalized severity, title,
// file, and line span joined by a NUL separator. Normalization lowercases, trims,
// and collapses internal whitespace, so two findings phrased with case or
// whitespace differences but the same substance collide on the same key while a
// differing span keeps them apart.
func Key(f Finding) string {
	return strings.Join([]string{
		normalize(f.Severity),
		normalize(f.Title),
		normalize(f.File),
		strconv.Itoa(f.StartLine),
		strconv.Itoa(f.EndLine),
	}, keySep)
}

// Merge deduplicates findings across lanes by Key. The merged finding accumulates
// the sorted-unique union of Families (each lane's Family plus any families the
// finding already carried) and Lanes across every input that produced the same
// key; the first-seen non-empty Body wins, and the first-seen identity fields
// (title/severity/file/span) are preserved. Lanes are processed in canonical
// lane-id order so "first-seen" is stable regardless of input permutation. Output
// order is deterministic: corroboration count (distinct lanes) descending, then
// severity rank (critical > high > medium > low > unknown), then Key ascending —
// so the same lanes in any input order marshal byte-identically. Empty input
// yields an empty, non-nil slice.
func Merge(lanes []LaneFindings) []Finding {
	sorted := make([]LaneFindings, len(lanes))
	copy(sorted, lanes)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].LaneID < sorted[j].LaneID
	})

	acc := map[string]*Finding{}
	var order []string
	for _, lane := range sorted {
		for _, f := range lane.Findings {
			key := Key(f)
			cur, ok := acc[key]
			if !ok {
				// Preserve first-seen identity; clear attribution so it
				// accumulates uniformly below across all reporting lanes.
				clone := f
				clone.Families = nil
				clone.Lanes = nil
				acc[key] = &clone
				cur = &clone
				order = append(order, key)
			}
			if cur.Body == "" && strings.TrimSpace(f.Body) != "" {
				cur.Body = f.Body
			}
			cur.Families = appendUnique(cur.Families, f.Families...)
			cur.Families = appendUnique(cur.Families, lane.Family)
			cur.Lanes = appendUnique(cur.Lanes, lane.LaneID)
		}
	}

	out := make([]Finding, 0, len(order))
	for _, key := range order {
		f := acc[key]
		sort.Strings(f.Families)
		sort.Strings(f.Lanes)
		out = append(out, *f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return less(out[i], out[j])
	})
	return out
}

// less orders findings by corroboration count descending, then severity rank
// ascending (critical first), then canonical Key ascending. Key is unique per
// merged finding, so this is a total order and the result is permutation-stable.
func less(a, b Finding) bool {
	if la, lb := len(a.Lanes), len(b.Lanes); la != lb {
		return la > lb
	}
	if ra, rb := severityRank(a.Severity), severityRank(b.Severity); ra != rb {
		return ra < rb
	}
	return Key(a) < Key(b)
}

// severityRank maps a severity to its ordering index; unrecognized severities
// rank as unknown (last).
func severityRank(severity string) int {
	switch normalize(severity) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

// normalize lowercases, trims, and collapses internal whitespace to a single
// space. strings.Fields drops leading/trailing/repeated whitespace.
func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// appendUnique appends each trimmed, non-empty addition not already present,
// preserving insertion order (the caller sorts when a canonical order is needed).
func appendUnique(dst []string, additions ...string) []string {
	for _, add := range additions {
		add = strings.TrimSpace(add)
		if add == "" {
			continue
		}
		if !slices.Contains(dst, add) {
			dst = append(dst, add)
		}
	}
	return dst
}
