// practices: [in-toto-provenance]

package drwitness

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/drrebuild"
)

// fixtureRows is a small, fixed Dolt projection: three provenance edges. Order
// is deliberately NOT canonical (ts out of order) to prove ReDeriveWitness
// canonicalizes before sealing.
func fixtureRows() []DoltRow {
	return []DoltRow{
		{
			SchemaVersion: "agentops-sdlc-provenance.v1",
			FromID:        "commit:abc123", FromType: "commit",
			ToID: "ag-lmdx.3", ToType: "bead",
			Relation: "wasRevisionOf", EvidenceRef: "ci://run/9",
			TrustTier: "inferred", TS: "2026-05-30T02:00:00Z",
		},
		{
			SchemaVersion: "agentops-sdlc-provenance.v1",
			FromID:        "ag-lmdx", FromType: "bead",
			ToID: "ag-lmdx.3", ToType: "decision",
			Relation: "wasInfluencedBy", EvidenceRef: "ag-lmdx",
			TrustTier: "authored", TS: "2026-05-30T00:00:00Z",
		},
		{
			SchemaVersion: "agentops-sdlc-provenance.v1",
			FromID:        "ag-lmdx.3", FromType: "decision",
			ToID: "scripts/witness-dolt-jsonl-crosscheck.sh", ToType: "artifact",
			Relation: "wasGeneratedBy", EvidenceRef: "GOALS.md",
			TrustTier: "authored", TS: "2026-05-30T01:00:00Z",
		},
	}
}

// committedFromRows is the "git-committed witness" side: the faithful witness a
// previous export would have committed. It is produced by re-deriving from the
// same faithful rows — that is exactly what the cross-check expects to match.
func committedFromRows(t *testing.T, rows []DoltRow) []byte {
	t.Helper()
	b, err := SerializeWitness(ReDeriveWitness(rows))
	if err != nil {
		t.Fatalf("SerializeWitness: %v", err)
	}
	return b
}

// Scenario: Clean state passes — Dolt and the JSONL witness are consistent
// renderings of the same event log; the cross-check passes and the committed
// chain verifies.
func TestCrossCheck_CleanStatePasses(t *testing.T) {
	rows := fixtureRows()
	committed := committedFromRows(t, rows)

	res, err := CrossCheck(rows, committed)
	if err != nil {
		t.Fatalf("CrossCheck: %v", err)
	}
	if !res.Match {
		t.Fatalf("clean state should match; got mismatch: %s", res.Detail)
	}
	if !res.BytesMatch {
		t.Errorf("clean state: BytesMatch should be true")
	}
	if !res.CommittedChainOK {
		t.Errorf("clean state: committed chain should verify")
	}
	if res.RederivedHead != res.CommittedHead {
		t.Errorf("clean state: heads must match: rederived=%s committed=%s", res.RederivedHead, res.CommittedHead)
	}
	if res.RederivedHead == "" {
		t.Errorf("clean state: chain head must be non-empty")
	}
}

// Determinism: the same Dolt rows in a different input order re-derive to a
// byte-identical witness (the property that makes the hash-compare meaningful).
func TestReDeriveWitness_OrderIndependent(t *testing.T) {
	rows := fixtureRows()
	a, err := SerializeWitness(ReDeriveWitness(rows))
	if err != nil {
		t.Fatalf("SerializeWitness a: %v", err)
	}
	// Reverse the input order.
	rev := make([]DoltRow, len(rows))
	for i := range rows {
		rev[len(rows)-1-i] = rows[i]
	}
	b, err := SerializeWitness(ReDeriveWitness(rev))
	if err != nil {
		t.Fatalf("SerializeWitness b: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("re-derive not order-independent:\n a=%q\n b=%q", a, b)
	}
}

// Scenario: Tampered Dolt fails the cross-check — a Dolt row is rewritten; the
// re-derived chain head no longer matches the committed witness head.
func TestCrossCheck_TamperedDoltFails(t *testing.T) {
	committed := committedFromRows(t, fixtureRows())

	tampered := fixtureRows()
	// Rewrite a row's trust_tier — a silent Dolt edit a force-update could make.
	tampered[1].TrustTier = "mined"

	res, err := CrossCheck(tampered, committed)
	if err != nil {
		t.Fatalf("CrossCheck: %v", err)
	}
	if res.Match {
		t.Fatalf("tampered Dolt must NOT match the committed witness")
	}
	if res.RederivedHead == res.CommittedHead {
		t.Errorf("tampered Dolt: re-derived head should differ from committed head")
	}
	if !strings.Contains(res.Detail, "DIVERGED") {
		t.Errorf("tampered Dolt: detail should report divergence, got %q", res.Detail)
	}
}

// Tampering with the COMMITTED witness (not Dolt) is also caught: a broken chain
// in the committed file means it is not tamper-evidence at all.
func TestCrossCheck_TamperedCommittedWitnessFails(t *testing.T) {
	rows := fixtureRows()
	committed := committedFromRows(t, rows)

	// Corrupt the committed witness: flip a hex digit in the first record's hash.
	events, err := drrebuild.ParseLedger(bytes.NewReader(committed))
	if err != nil {
		t.Fatalf("ParseLedger: %v", err)
	}
	events[0].Hash = strings.Repeat("0", len(events[0].Hash))
	broken, err := SerializeWitness(events)
	if err != nil {
		t.Fatalf("SerializeWitness: %v", err)
	}

	res, err := CrossCheck(rows, broken)
	if err != nil {
		t.Fatalf("CrossCheck: %v", err)
	}
	if res.Match {
		t.Fatalf("broken committed witness must not match")
	}
	if res.CommittedChainOK {
		t.Errorf("broken committed witness: CommittedChainOK should be false")
	}
	if !strings.Contains(res.Detail, "committed witness chain is broken") {
		t.Errorf("broken committed witness: detail should name the broken chain, got %q", res.Detail)
	}
}

// A row added to Dolt that the committed witness never saw must fail (heads
// match on a subset only by accident; here record count diverges → head diff).
func TestCrossCheck_ExtraDoltRowFails(t *testing.T) {
	rows := fixtureRows()
	committed := committedFromRows(t, rows)

	extra := append(fixtureRows(), DoltRow{
		SchemaVersion: "agentops-sdlc-provenance.v1",
		FromID:        "ag-lmdx.3", FromType: "decision",
		ToID: "verdict:green", ToType: "verdict",
		Relation: "wasGeneratedBy", EvidenceRef: "ci://run/10",
		TrustTier: "inferred", TS: "2026-05-30T03:00:00Z",
	})

	res, err := CrossCheck(extra, committed)
	if err != nil {
		t.Fatalf("CrossCheck: %v", err)
	}
	if res.Match {
		t.Fatalf("extra Dolt row must NOT match the committed witness")
	}
}

func TestParseDoltRows_RejectsMissingEndpoint(t *testing.T) {
	bad := `{"schema_version":"agentops-sdlc-provenance.v1","from_id":"","to_id":"x","relation":"r","trust_tier":"authored","ts":"2026-05-30T00:00:00Z"}`
	if _, err := ParseDoltRows(strings.NewReader(bad)); err == nil {
		t.Fatalf("expected error for row missing from_id")
	}
}

func TestParseDoltRows_SkipsBlankLines(t *testing.T) {
	in := "\n" + `{"schema_version":"v","from_id":"a","to_id":"b","relation":"r","trust_tier":"authored","ts":"t"}` + "\n\n"
	rows, err := ParseDoltRows(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseDoltRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}
