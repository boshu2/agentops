package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// TestProvenanceEmitVerdictCommand_Registered asserts the `ao provenance
// emit-verdict` command is wired into the cobra tree under provenance with the
// required flags. This is the command-surface coverage for the new leaf.
func TestProvenanceEmitVerdictCommand_Registered(t *testing.T) {
	if provenanceEmitVerdictCmd.Use != "emit-verdict" {
		t.Fatalf("Use = %q, want emit-verdict", provenanceEmitVerdictCmd.Use)
	}
	var found bool
	for _, c := range provenanceCmd.Commands() {
		if c.Use == "emit-verdict" {
			found = true
		}
	}
	if !found {
		t.Error("emit-verdict is not registered under `ao provenance`")
	}
	for _, f := range []string{"file", "dry-run", "json"} {
		if provenanceEmitVerdictCmd.Flags().Lookup(f) == nil {
			t.Errorf("missing flag --%s on `ao provenance emit-verdict`", f)
		}
	}
}

// TestExtractVerdict covers the pure parser that reads bead_id, head_sha, and
// disposition from a pawl-verdict JSON file. Uses a real on-disk fixture
// (fixture fidelity: production writer shape).
func TestExtractVerdict(t *testing.T) {
	cases := []struct {
		name    string
		data    map[string]any
		wantErr string
	}{
		{
			name: "valid full verdict",
			data: map[string]any{
				"schema_version":    "pawl-verdict.v1",
				"bead_id":           "ag-abc",
				"pr":                42,
				"head_sha":          "0123456789abcdef",
				"disposition":       "CONFIRMED",
				"generated_at":      "2026-06-15T00:00:00Z",
				"author_context_id": "ctx-1",
				"refuters":          []any{map[string]any{"family": "claude", "verdict": "CONFIRMED", "context_id": "ctx-2"}},
			},
		},
		{
			name:    "missing bead_id",
			data:    map[string]any{"head_sha": "abcdefg", "disposition": "CONFIRMED"},
			wantErr: "bead_id",
		},
		{
			name:    "missing head_sha",
			data:    map[string]any{"bead_id": "ag-x", "disposition": "CONFIRMED"},
			wantErr: "head_sha",
		},
		{
			name:    "head_sha too short",
			data:    map[string]any{"bead_id": "ag-x", "head_sha": "abc", "disposition": "CONFIRMED"},
			wantErr: "head_sha",
		},
		{
			name:    "missing disposition",
			data:    map[string]any{"bead_id": "ag-x", "head_sha": "abcdefg1234"},
			wantErr: "disposition",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, _ := json.Marshal(c.data)
			f := filepath.Join(t.TempDir(), "verdict.json")
			if err := os.WriteFile(f, b, 0644); err != nil {
				t.Fatal(err)
			}
			v, err := extractVerdict(f)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("error %q does not contain %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.BeadID != "ag-abc" {
				t.Errorf("BeadID = %q, want ag-abc", v.BeadID)
			}
			if v.HeadSHA != "0123456789abcdef" {
				t.Errorf("HeadSHA = %q, want 0123456789abcdef", v.HeadSHA)
			}
			if v.Disposition != "CONFIRMED" {
				t.Errorf("Disposition = %q, want CONFIRMED", v.Disposition)
			}
		})
	}
}

// TestBuildVerdictCommitEdge enforces the edge shape: verdict --wasDerivedFrom-->
// commit, trust_tier inferred, node id = bead@sha7.
func TestBuildVerdictCommitEdge(t *testing.T) {
	v := pawlVerdict{
		BeadID:      "ag-abc",
		HeadSHA:     "0123456789abcdef0123456789abcdef01234567",
		Disposition: "CONFIRMED",
	}
	e := buildVerdictCommitEdge(v)

	if e.FromID != "ag-abc@0123456" {
		t.Errorf("FromID = %q, want ag-abc@0123456", e.FromID)
	}
	if e.FromType != "verdict" {
		t.Errorf("FromType = %q, want verdict", e.FromType)
	}
	if e.ToID != v.HeadSHA {
		t.Errorf("ToID = %q, want %s", e.ToID, v.HeadSHA)
	}
	if e.ToType != "commit" {
		t.Errorf("ToType = %q, want commit", e.ToType)
	}
	if e.Relation != "wasDerivedFrom" {
		t.Errorf("Relation = %q, want wasDerivedFrom", e.Relation)
	}
	if e.TrustTier != "inferred" {
		t.Errorf("TrustTier = %q, want inferred", e.TrustTier)
	}
	if !strings.Contains(e.EvidenceRef, "ag-abc") {
		t.Errorf("EvidenceRef should reference bead id, got %q", e.EvidenceRef)
	}
	if !strings.Contains(e.EvidenceRef, "CONFIRMED") {
		t.Errorf("EvidenceRef should reference disposition, got %q", e.EvidenceRef)
	}
}

// TestEmitVerdict_LedgerGrowsAndStaysIntact is the L2 scenario proof for
// ag-cm8nd: emitting verdict→commit edges appends schema-valid, hash-chained
// rows to the ledger, the chain verifies, and re-emitting is idempotent.
// Exercises the REAL store on a temp ledger (fixture-fidelity).
func TestEmitVerdict_LedgerGrowsAndStaysIntact(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	store := provenancegraph.NewStore(ledger)

	v := pawlVerdict{
		BeadID:      "ag-cm8nd",
		HeadSHA:     "fedcba9876543210fedcba9876543210fedcba98",
		Disposition: "CONFIRMED",
	}

	edge := buildVerdictCommitEdge(v)
	edge.TS = "2026-06-15T00:00:00Z"

	res, err := store.Append(edge)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if res.Skipped {
		t.Error("first emit should not be skipped")
	}

	vr, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !vr.Pass {
		t.Fatalf("chain not intact: %s (line %d)", vr.Message, vr.FirstBrokenLine)
	}
	if vr.RecordCount != 1 {
		t.Errorf("RecordCount = %d, want 1", vr.RecordCount)
	}

	// Re-emission is idempotent.
	edge2 := buildVerdictCommitEdge(v)
	edge2.TS = "2026-06-15T00:00:00Z"
	res2, err := store.Append(edge2)
	if err != nil {
		t.Fatalf("re-append: %v", err)
	}
	if !res2.Skipped {
		t.Error("re-emit should be an idempotent no-op")
	}
	vr2, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("verify after re-emit: %v", err)
	}
	if vr2.RecordCount != 1 {
		t.Errorf("idempotent re-emit changed RecordCount to %d, want 1", vr2.RecordCount)
	}
}
