// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// resetProvExportFlags sets the export flags to a known baseline.
func resetProvExportFlags() {
	provExportJSON = false
	provExportVerify = false
}

// seedLedger appends a fixed set of distinct edges (in a deliberately
// non-canonical insertion order) to the ledger at the resolved path.
func seedLedger(t *testing.T) {
	t.Helper()
	store := provenancegraph.NewStore(resolveLedgerPath())
	edges := []provenancegraph.Edge{
		{FromID: "ag-2", FromType: "decision", ToID: "b.go", ToType: "artifact",
			Relation: "wasGeneratedBy", TrustTier: "authored", TS: "2026-05-31T02:00:00Z"},
		{FromID: "ag-1", FromType: "decision", ToID: "a.go", ToType: "artifact",
			Relation: "wasGeneratedBy", TrustTier: "authored", TS: "2026-05-31T01:00:00Z"},
		{FromID: "ag-3", FromType: "decision", ToID: "c.go", ToType: "artifact",
			Relation: "wasGeneratedBy", TrustTier: "authored", TS: "2026-05-31T03:00:00Z"},
	}
	for i, e := range edges {
		if _, err := store.Append(e); err != nil {
			t.Fatalf("seed edge %d: %v", i, err)
		}
	}
}

func TestProvenanceExport_DeterministicBytesAcrossRuns(t *testing.T) {
	chdirRepoFixture(t)
	seedLedger(t)
	resetProvExportFlags()

	c1, out1 := provTestCmd()
	if err := runProvenanceExport(c1, nil); err != nil {
		t.Fatalf("export 1: %v", err)
	}
	c2, out2 := provTestCmd()
	if err := runProvenanceExport(c2, nil); err != nil {
		t.Fatalf("export 2: %v", err)
	}
	if out1.String() != out2.String() {
		t.Fatalf("export not byte-identical across runs:\n--- run1 ---\n%s\n--- run2 ---\n%s", out1.String(), out2.String())
	}

	// JSONL: one edge per non-blank line, in canonical (ts) order.
	lines := strings.Split(strings.TrimRight(out1.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("export emitted %d lines, want 3", len(lines))
	}
	var prevTS string
	for i, ln := range lines {
		var e provenancegraph.Edge
		if err := json.Unmarshal([]byte(ln), &e); err != nil {
			t.Fatalf("line %d not valid Edge JSON: %v\n%s", i, err, ln)
		}
		if e.TS < prevTS {
			t.Fatalf("line %d ts %q out of canonical order (prev %q)", i, e.TS, prevTS)
		}
		prevTS = e.TS
	}
}

func TestProvenanceExport_ChainVerifies(t *testing.T) {
	chdirRepoFixture(t)
	seedLedger(t)
	resetProvExportFlags()
	provExportJSON = true

	c, out := provTestCmd()
	if err := runProvenanceExport(c, nil); err != nil {
		t.Fatalf("export: %v", err)
	}
	var chain []provenancegraph.Edge
	if err := json.Unmarshal(out.Bytes(), &chain); err != nil {
		t.Fatalf("export --json not an array: %v\n%s", err, out.String())
	}
	if len(chain) != 3 {
		t.Fatalf("exported %d edges, want 3", len(chain))
	}
	if idx, err := provenancegraph.VerifyChain(chain); err != nil || idx != 0 {
		t.Fatalf("exported chain does not verify: idx=%d err=%v", idx, err)
	}
	if chain[0].PrevHash != "" {
		t.Fatalf("genesis prev_hash = %q, want empty", chain[0].PrevHash)
	}
	// Canonical order: earliest ts first.
	if chain[0].FromID != "ag-1" || chain[2].FromID != "ag-3" {
		t.Fatalf("canonical order wrong: first=%q last=%q", chain[0].FromID, chain[2].FromID)
	}
}

func TestProvenanceExport_EmptyLedger(t *testing.T) {
	chdirRepoFixture(t)
	resetProvExportFlags()

	// Default JSONL on an empty ledger: no lines, no error.
	c, out := provTestCmd()
	if err := runProvenanceExport(c, nil); err != nil {
		t.Fatalf("export empty: %v", err)
	}
	if out.String() != "" {
		t.Fatalf("empty-ledger JSONL export = %q, want empty", out.String())
	}

	// --json on an empty ledger: a concrete empty array, never null.
	resetProvExportFlags()
	provExportJSON = true
	c2, out2 := provTestCmd()
	if err := runProvenanceExport(c2, nil); err != nil {
		t.Fatalf("export empty json: %v", err)
	}
	if got := strings.TrimSpace(out2.String()); got != "[]" {
		t.Fatalf("empty-ledger --json export = %q, want []", got)
	}
}

func TestProvenanceExport_VerifyFlagSummary(t *testing.T) {
	chdirRepoFixture(t)
	seedLedger(t)
	resetProvExportFlags()
	provExportVerify = true

	c, out := provTestCmd()
	if err := runProvenanceExport(c, nil); err != nil {
		t.Fatalf("export --verify: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "OK:") || !strings.Contains(got, "3 edge") {
		t.Fatalf("--verify summary = %q, want OK with edge count", got)
	}
	// --verify must NOT emit the JSONL/array body.
	if strings.Contains(got, "schema_version") {
		t.Fatalf("--verify leaked edge body: %q", got)
	}
}

func TestProvenanceExport_TamperedLedgerRejected(t *testing.T) {
	chdirRepoFixture(t)
	seedLedger(t)
	resetProvExportFlags()

	// Tamper: rewrite the ledger file with a record whose hash fields are wrong
	// for its payload. Re-chaining recomputes hashes, but ValidateFields still
	// runs; an invalid relation here makes the export fail loudly.
	path := resolveLedgerPath()
	store := provenancegraph.NewStore(path)
	edges, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	edges[1].Relation = "not_a_relation"
	lines := make([]string, 0, len(edges))
	for _, e := range edges {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("rewrite ledger: %v", err)
	}

	c, _ := provTestCmd()
	if err := runProvenanceExport(c, nil); err == nil {
		t.Fatal("expected export to reject a ledger with an invalid edge")
	}
}
