package provenancegraph

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// OrphanCode is the single defect code the orphan audit emits: an engineered
// artifact node with no inbound authored/inferred provenance edge. It
// generalizes the chain-gap detection in cmd/ao/goals_trace_orphans.go (which
// flags trace-chain nodes that no edge yields to) onto the provenance graph,
// where the orphan condition is "an artifact node that nothing produces".
const OrphanCode = "no_inbound_authored_edge"

// graphArtifactType is the artifact node type the orphan audit treats as an
// engineered bit that MUST have a provenance ancestor. The seeded fixtures
// (tests/fixtures/provenance/) model phantom gates, retired scripts, and stale
// doctrine claims all as artifact nodes.
const graphArtifactType = "artifact"

// GraphRecord is one line of a provenance trace-graph JSONL file. The format is
// the goalstrace Node/Edge JSON contract (cli/internal/goalstrace/graph.go)
// plus a leading "record" discriminator, so the orphan audit reads the same
// fixtures the goals-trace walker emits.
type GraphRecord struct {
	Record   string `json:"record"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Label    string `json:"label,omitempty"`
	Path     string `json:"path,omitempty"`
	EdgeType string `json:"edge_type,omitempty"`
	FromID   string `json:"from_id,omitempty"`
	ToID     string `json:"to_id,omitempty"`
}

// OrphanFinding is one orphan-class defect: an artifact node with no inbound
// edge. The fields mirror tests/fixtures/provenance/expected-orphans.json so a
// fixture's expected_finding compares field-for-field.
type OrphanFinding struct {
	Severity         string `json:"severity"`
	Code             string `json:"code"`
	OrphanArtifactID string `json:"orphan_artifact_id"`
	Path             string `json:"path,omitempty"`
	Message          string `json:"message"`
}

// ReadGraphRecords parses a provenance trace-graph JSONL file into its records.
// Blank lines are skipped; a malformed line or an unknown record discriminator
// is a hard error so the audit never silently drops a record. A missing file
// is an error: the audit is explicitly handed a graph to inspect.
func ReadGraphRecords(path string) ([]GraphRecord, error) {
	// #nosec G304 -- path is an operator/CI-supplied graph file by design: the
	// audit's whole purpose is to read a trace-graph the caller names (a fixture
	// or the committed ledger projection). No untrusted network input.
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open graph: %w", err)
	}
	defer func() { _ = f.Close() }()

	var recs []GraphRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Bytes()
		if len(trimSpace(raw)) == 0 {
			continue
		}
		var rec GraphRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			return nil, fmt.Errorf("graph line %d: invalid JSON: %w", line, err)
		}
		if rec.Record != "node" && rec.Record != "edge" {
			return nil, fmt.Errorf("graph line %d: record must be \"node\" or \"edge\", got %q", line, rec.Record)
		}
		recs = append(recs, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan graph: %w", err)
	}
	return recs, nil
}

// FindOrphans returns one OrphanFinding for every artifact node that has no
// inbound edge (no edge's to_id targets it). Findings are sorted by orphan
// artifact id for deterministic output. An artifact gains an inbound edge — and
// so stops being an orphan — the moment any edge record points at it, which is
// exactly how the seeded fixtures flip green once wired.
func FindOrphans(recs []GraphRecord) []OrphanFinding {
	inbound := map[string]bool{}
	for _, r := range recs {
		if r.Record == "edge" && r.ToID != "" {
			inbound[r.ToID] = true
		}
	}

	var findings []OrphanFinding
	for _, r := range recs {
		if r.Record != "node" || r.Type != graphArtifactType {
			continue
		}
		if inbound[r.ID] {
			continue
		}
		findings = append(findings, OrphanFinding{
			Severity:         "error",
			Code:             OrphanCode,
			OrphanArtifactID: r.ID,
			Path:             r.Path,
			Message:          fmt.Sprintf("artifact %s has no inbound authored/inferred provenance edge", r.ID),
		})
	}
	sort.SliceStable(findings, func(i, j int) bool {
		return findings[i].OrphanArtifactID < findings[j].OrphanArtifactID
	})
	return findings
}
