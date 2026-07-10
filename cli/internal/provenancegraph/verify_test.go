package provenancegraph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedVerifyLedger appends two distinct, valid edges to a fresh ledger in a
// tmp dir and returns the store + path. The two-edge shape exercises the
// genesis link (prev_hash == "") and a real prev_hash chain link.
func seedVerifyLedger(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	store := NewStore(path)
	edges := []Edge{
		{FromID: "ag-1", FromType: "bead", ToID: "abc123", ToType: "commit",
			Relation: "wasGeneratedBy", TrustTier: "authored", TS: "2026-06-13T01:00:00Z"},
		{FromID: "ag-2", FromType: "decision", ToID: "a.go", ToType: "artifact",
			Relation: "wasDerivedFrom", TrustTier: "inferred", TS: "2026-06-13T02:00:00Z"},
	}
	for i, e := range edges {
		if _, err := store.Append(e); err != nil {
			t.Fatalf("seed edge %d: %v", i, err)
		}
	}
	return store, path
}

// readLines returns the ledger file's lines (sans trailing blank).
func readLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

func writeLines(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("rewrite ledger: %v", err)
	}
}

// TestVerifyFile_IntactChainPasses is the GREEN baseline: two appended edges
// form an intact chain and VerifyFile reports Pass with the right count.
func TestVerifyFile_IntactChainPasses(t *testing.T) {
	store, _ := seedVerifyLedger(t)
	res, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("VerifyFile: %v", err)
	}
	if !res.Pass {
		t.Fatalf("intact chain reported broken: line %d: %s", res.FirstBrokenLine, res.Message)
	}
	if res.RecordCount != 2 {
		t.Fatalf("RecordCount = %d, want 2", res.RecordCount)
	}
	if res.FirstBrokenLine != 0 {
		t.Fatalf("FirstBrokenLine = %d on intact chain, want 0", res.FirstBrokenLine)
	}
}

// TestVerifyFile_MixedV1V11_ThroughProductionWriter is the L2 fixture-fidelity
// proof (age-rk3r.3): a ledger built by appending BOTH a v1-shaped edge and a
// v1.1-shaped (enrichment-carrying) edge THROUGH the production Store.Append
// verifies clean in place, and the enrichment survives the write→read round-trip
// while the v1 edge is unaffected. The v1.1 fields are hash-protected yet
// additive, so the boundary between the two shapes does not break the chain.
func TestVerifyFile_MixedV1V11_ThroughProductionWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	store := NewStore(path)

	// v1-shaped bead→commit edge (no enrichment fields).
	if _, err := store.Append(Edge{
		FromID: "ag-old", FromType: "bead", ToID: "0123456abc", ToType: "commit",
		Relation: "wasGeneratedBy", TrustTier: "authored", TS: "2026-07-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("append v1 edge: %v", err)
	}
	// v1.1-shaped verdict→commit edge carrying every enrichment field.
	if _, err := store.Append(Edge{
		FromID: "ag-new@0123456", FromType: "verdict", ToID: "0123456abc", ToType: "commit",
		Relation: "wasDerivedFrom", TrustTier: "inferred", TS: "2026-07-01T01:00:00Z",
		ReviewerFamily: "claude+gpt", Degraded: true, Rounds: 2, DurationS: 9.5,
		EvidencePath: ".agents/x.md",
	}); err != nil {
		t.Fatalf("append v1.1 edge: %v", err)
	}

	res, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("VerifyFile: %v", err)
	}
	if !res.Pass {
		t.Fatalf("mixed v1/v1.1 chain not intact: line %d: %s", res.FirstBrokenLine, res.Message)
	}
	if res.RecordCount != 2 {
		t.Errorf("RecordCount = %d, want 2", res.RecordCount)
	}

	// The enrichment survives the write→read round-trip through the ledger file;
	// the v1 edge stays field-empty.
	edges, err := store.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if edges[1].ReviewerFamily != "claude+gpt" || !edges[1].Degraded || edges[1].Rounds != 2 ||
		edges[1].DurationS != 9.5 || edges[1].EvidencePath != ".agents/x.md" {
		t.Fatalf("enrichment lost through the ledger round-trip: %+v", edges[1])
	}
	if edges[0].ReviewerFamily != "" || edges[0].Degraded || edges[0].Rounds != 0 ||
		edges[0].DurationS != 0 || edges[0].EvidencePath != "" {
		t.Errorf("v1 edge unexpectedly gained enrichment fields: %+v", edges[0])
	}
}

// TestVerifyFile_GenesisLinks confirms row 1 prev_hash is the empty genesis and
// row 2 prev_hash links to row 1 hash.
func TestVerifyFile_GenesisLinks(t *testing.T) {
	store, _ := seedVerifyLedger(t)
	edges, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if edges[0].PrevHash != "" {
		t.Fatalf("genesis prev_hash = %q, want empty", edges[0].PrevHash)
	}
	if edges[1].PrevHash != edges[0].Hash {
		t.Fatalf("row2 prev_hash = %q, want row1 hash %q", edges[1].PrevHash, edges[0].Hash)
	}
}

// TestVerifyFile_TamperedPayloadFieldCaught is THE windshield-correctness test:
// flip one semantic field in a committed row (without recomputing its hashes)
// and VerifyFile MUST fail, naming that line. A lying ledger is caught.
func TestVerifyFile_TamperedPayloadFieldCaught(t *testing.T) {
	store, path := seedVerifyLedger(t)
	lines := readLines(t, path)

	// Tamper row 2: change to_id but leave the committed payload_hash/hash.
	var e Edge
	if err := json.Unmarshal([]byte(lines[1]), &e); err != nil {
		t.Fatalf("unmarshal row2: %v", err)
	}
	e.ToID = "evil.go" // altered content; hashes now stale
	b, _ := json.Marshal(e)
	lines[1] = string(b)
	writeLines(t, path, lines)

	res, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("VerifyFile: %v", err)
	}
	if res.Pass {
		t.Fatal("TAMPER NOT CAUGHT: a flipped payload field passed verification — the ledger is a lying instrument")
	}
	if res.FirstBrokenLine != 2 {
		t.Fatalf("FirstBrokenLine = %d, want 2", res.FirstBrokenLine)
	}
	if !strings.Contains(res.Message, "payload_hash") {
		t.Fatalf("message = %q, want payload_hash mismatch", res.Message)
	}
}

// TestVerifyFile_PayloadMismatchNamesReaderSkew is verification-surface-honesty
// S4: from THIS reader's seat a payload_hash mismatch is indistinguishable
// between real tampering and a stale reader whose payload fieldset lags the
// writer's (live 2026-07-10: an installed ao false-flagged the real ledger as
// BROKEN at line 423 while a fresh build verified all 441 records). The error
// surface must therefore name reader-version/hashing skew as a possible cause
// and instruct rebuilding ao before the ledger is treated as tampered.
func TestVerifyFile_PayloadMismatchNamesReaderSkew(t *testing.T) {
	store, path := seedVerifyLedger(t)
	lines := readLines(t, path)

	var e Edge
	if err := json.Unmarshal([]byte(lines[1]), &e); err != nil {
		t.Fatalf("unmarshal row2: %v", err)
	}
	e.ToID = "evil.go" // stale hashes — reader-side this IS what skew looks like
	b, _ := json.Marshal(e)
	lines[1] = string(b)
	writeLines(t, path, lines)

	res, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("VerifyFile: %v", err)
	}
	if res.Pass {
		t.Fatal("payload mismatch passed verification")
	}
	if !strings.Contains(res.Message, payloadHashSkewHint) {
		t.Fatalf("message = %q, must carry the shared reader-skew hint %q", res.Message, payloadHashSkewHint)
	}
	for _, phrase := range []string{"reader", "rebuild ao", "tampered"} {
		if !strings.Contains(res.Message, phrase) {
			t.Errorf("message = %q, must name %q", res.Message, phrase)
		}
	}
}

// TestVerifyFile_ForgedHashCaught: flip the chain anchor (hash) of a row → fail.
func TestVerifyFile_ForgedHashCaught(t *testing.T) {
	store, path := seedVerifyLedger(t)
	lines := readLines(t, path)

	var e Edge
	if err := json.Unmarshal([]byte(lines[0]), &e); err != nil {
		t.Fatalf("unmarshal row1: %v", err)
	}
	// Forge the hash to a valid-looking but wrong digest.
	e.Hash = strings.Repeat("a", 64)
	b, _ := json.Marshal(e)
	lines[0] = string(b)
	writeLines(t, path, lines)

	res, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("VerifyFile: %v", err)
	}
	if res.Pass {
		t.Fatal("TAMPER NOT CAUGHT: a forged hash passed verification")
	}
	if res.FirstBrokenLine != 1 {
		t.Fatalf("FirstBrokenLine = %d, want 1", res.FirstBrokenLine)
	}
	if !strings.Contains(res.Message, "hash mismatch") {
		t.Fatalf("message = %q, want hash mismatch", res.Message)
	}
}

// TestVerifyFile_ReorderedRowsCaught: swap two valid rows → the chain links
// break (row at line 1 no longer has the genesis prev_hash).
func TestVerifyFile_ReorderedRowsCaught(t *testing.T) {
	store, path := seedVerifyLedger(t)
	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	lines[0], lines[1] = lines[1], lines[0] // reorder
	writeLines(t, path, lines)

	res, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("VerifyFile: %v", err)
	}
	if res.Pass {
		t.Fatal("TAMPER NOT CAUGHT: reordered rows passed verification")
	}
	if res.FirstBrokenLine != 1 {
		t.Fatalf("FirstBrokenLine = %d, want 1 (genesis link broken at first row)", res.FirstBrokenLine)
	}
	if !strings.Contains(res.Message, "prev_hash mismatch") {
		t.Fatalf("message = %q, want prev_hash mismatch", res.Message)
	}
}

// TestVerifyFile_MissingFileIsEmptyIntact: no file → Pass, count 0 (fresh
// clone with no events must not fail the gate).
func TestVerifyFile_MissingFileIsEmptyIntact(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "nope.jsonl"))
	res, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("VerifyFile: %v", err)
	}
	if !res.Pass || res.RecordCount != 0 {
		t.Fatalf("missing file: Pass=%v count=%d, want Pass=true count=0", res.Pass, res.RecordCount)
	}
}

// TestVerifyFile_AppendCreatesValidGenesis: appending against a missing file
// creates it with a verifiable genesis row.
func TestVerifyFile_AppendCreatesValidGenesis(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh", "ledger.jsonl")
	store := NewStore(path)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("precondition: file should not exist yet")
	}
	_, err := store.Append(Edge{
		FromID: "ag-8jf97", FromType: "bead", ToID: "deadbeef", ToType: "commit",
		Relation: "wasGeneratedBy", TrustTier: "authored", TS: "2026-06-13T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("append to missing file: %v", err)
	}
	res, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("VerifyFile: %v", err)
	}
	if !res.Pass || res.RecordCount != 1 {
		t.Fatalf("fresh genesis not intact: Pass=%v count=%d line %d: %s",
			res.Pass, res.RecordCount, res.FirstBrokenLine, res.Message)
	}
}
