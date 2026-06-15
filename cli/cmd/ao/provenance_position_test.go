package main

import (
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// TestProvenancePositionCommand_Registered asserts the `ao provenance position`
// command is wired into the cobra tree under provenance with the --json flag.
func TestProvenancePositionCommand_Registered(t *testing.T) {
	if provenancePositionCmd.Use != "position" {
		t.Fatalf("Use = %q, want position", provenancePositionCmd.Use)
	}
	var found bool
	for _, c := range provenanceCmd.Commands() {
		if c.Use == "position" {
			found = true
		}
	}
	if !found {
		t.Error("position is not registered under `ao provenance`")
	}
	if provenancePositionCmd.Flags().Lookup("json") == nil {
		t.Error("missing flag --json on `ao provenance position`")
	}
}

// TestExtractLandedBeads_FiltersCorrectly exercises the core filter logic with
// edges round-tripped through the real store (fixture fidelity: production
// writer, production reader).
func TestExtractLandedBeads_FiltersCorrectly(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	store := provenancegraph.NewStore(ledger)

	beadEdge := provenancegraph.Edge{
		FromID: "ag-test1", FromType: "bead",
		ToID: "abc123def456", ToType: "commit",
		Relation: "wasGeneratedBy", TrustTier: "inferred",
		TS: "2026-06-15T00:00:00Z", EvidenceRef: "commit abc123def456",
	}
	if _, err := store.Append(beadEdge); err != nil {
		t.Fatalf("append bead edge: %v", err)
	}

	nonBeadEdge := provenancegraph.Edge{
		FromID: "soc-x1", FromType: "decision",
		ToID: "ag-test1", ToType: "bead",
		Relation: "wasInfluencedBy", TrustTier: "authored",
		TS: "2026-06-15T00:01:00Z", EvidenceRef: "council verdict",
	}
	if _, err := store.Append(nonBeadEdge); err != nil {
		t.Fatalf("append non-bead edge: %v", err)
	}

	edges, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}

	landed := extractLandedBeads(edges)
	if len(landed) != 1 {
		t.Fatalf("expected 1 landed bead, got %d", len(landed))
	}
	if landed[0].BeadID != "ag-test1" {
		t.Errorf("BeadID = %q, want ag-test1", landed[0].BeadID)
	}
	if landed[0].CommitRef != "abc123def456" {
		t.Errorf("CommitRef = %q, want abc123def456", landed[0].CommitRef)
	}
	if landed[0].TrustTier != "inferred" {
		t.Errorf("TrustTier = %q, want inferred", landed[0].TrustTier)
	}
}

// TestBuildPositionReport_EmptyLedger verifies graceful degradation on a
// genesis-only (empty) ledger — the second acceptance scenario.
func TestBuildPositionReport_EmptyLedger(t *testing.T) {
	report := buildPositionReport(nil)
	if len(report.LandedBeads) != 0 {
		t.Errorf("expected 0 landed beads on nil edges, got %d", len(report.LandedBeads))
	}
	if report.LandedBeads == nil {
		t.Error("LandedBeads should be empty slice (not nil) for clean JSON marshaling")
	}
	if report.TotalEdges != 0 {
		t.Errorf("TotalEdges = %d, want 0", report.TotalEdges)
	}
}

// TestBuildPositionReport_WithLandedBeads exercises the full report shape with
// edges from a real store round-trip.
func TestBuildPositionReport_WithLandedBeads(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	store := provenancegraph.NewStore(ledger)

	for _, id := range []string{"ag-aaa", "ag-bbb"} {
		edge := provenancegraph.Edge{
			FromID: id, FromType: "bead",
			ToID: "commit" + id, ToType: "commit",
			Relation: "wasGeneratedBy", TrustTier: "inferred",
			TS: "2026-06-15T00:00:00Z", EvidenceRef: "commit commit" + id,
		}
		if _, err := store.Append(edge); err != nil {
			t.Fatalf("append %s: %v", id, err)
		}
	}

	edges, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	report := buildPositionReport(edges)
	if len(report.LandedBeads) != 2 {
		t.Fatalf("expected 2 landed beads, got %d", len(report.LandedBeads))
	}
	if report.TotalEdges != 2 {
		t.Fatalf("TotalEdges = %d, want 2", report.TotalEdges)
	}
	if report.LandedBeads[0].BeadID != "ag-aaa" {
		t.Errorf("first bead = %q, want ag-aaa", report.LandedBeads[0].BeadID)
	}
	if report.LandedBeads[1].BeadID != "ag-bbb" {
		t.Errorf("second bead = %q, want ag-bbb", report.LandedBeads[1].BeadID)
	}
}
