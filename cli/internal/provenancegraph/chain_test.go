package provenancegraph

import (
	"encoding/json"
	"testing"
)

// edgeAt builds a field-valid (unsealed) edge with a given ts/from/to so tests
// can control canonical order independently of insertion order.
func edgeAt(ts, fromID, toID string) Edge {
	return Edge{
		SchemaVersion: SchemaVersion,
		FromID:        fromID,
		FromType:      "decision",
		ToID:          toID,
		ToType:        "artifact",
		Relation:      "decision_produces_artifact",
		TrustTier:     "authored",
		TS:            ts,
	}
}

func TestCanonicalSort_OrdersByTupleAndIsStable(t *testing.T) {
	in := []Edge{
		edgeAt("2026-05-31T03:00:00Z", "b", "z"),
		edgeAt("2026-05-31T01:00:00Z", "a", "y"),
		edgeAt("2026-05-31T01:00:00Z", "a", "x"), // same ts+from, earlier to_id
		edgeAt("2026-05-31T02:00:00Z", "a", "y"),
	}
	got := CanonicalSort(in)

	wantTuples := [][3]string{
		{"2026-05-31T01:00:00Z", "a", "x"},
		{"2026-05-31T01:00:00Z", "a", "y"},
		{"2026-05-31T02:00:00Z", "a", "y"},
		{"2026-05-31T03:00:00Z", "b", "z"},
	}
	if len(got) != len(wantTuples) {
		t.Fatalf("got %d edges, want %d", len(got), len(wantTuples))
	}
	for i, w := range wantTuples {
		if got[i].TS != w[0] || got[i].FromID != w[1] || got[i].ToID != w[2] {
			t.Fatalf("position %d: got (%s,%s,%s), want %v",
				i, got[i].TS, got[i].FromID, got[i].ToID, w)
		}
	}
	// Input slice must be untouched.
	if in[0].FromID != "b" {
		t.Fatalf("CanonicalSort mutated input: in[0].FromID=%q", in[0].FromID)
	}
}

func TestReChain_VerifiesAndIsOrderIndependent(t *testing.T) {
	a := edgeAt("2026-05-31T01:00:00Z", "ag-1", "a.go")
	b := edgeAt("2026-05-31T02:00:00Z", "ag-2", "b.go")
	c := edgeAt("2026-05-31T03:00:00Z", "ag-3", "c.go")

	chain1, err := ReChain([]Edge{a, b, c})
	if err != nil {
		t.Fatalf("rechain 1: %v", err)
	}
	// Same set, shuffled order, must produce an identical sealed chain.
	chain2, err := ReChain([]Edge{c, a, b})
	if err != nil {
		t.Fatalf("rechain 2: %v", err)
	}

	if len(chain1) != 3 {
		t.Fatalf("chain length = %d, want 3", len(chain1))
	}
	if idx, err := VerifyChain(chain1); err != nil || idx != 0 {
		t.Fatalf("rechained chain does not verify: idx=%d err=%v", idx, err)
	}
	for i := range chain1 {
		if chain1[i].Hash != chain2[i].Hash {
			t.Fatalf("order-dependent hash at %d: %q vs %q", i, chain1[i].Hash, chain2[i].Hash)
		}
		if chain1[i].PrevHash != chain2[i].PrevHash {
			t.Fatalf("order-dependent prev_hash at %d", i)
		}
	}
	// Genesis prev_hash empty; subsequent links chain.
	if chain1[0].PrevHash != "" {
		t.Fatalf("genesis prev_hash = %q, want empty", chain1[0].PrevHash)
	}
	if chain1[1].PrevHash != chain1[0].Hash {
		t.Fatalf("link 1 prev_hash = %q, want %q", chain1[1].PrevHash, chain1[0].Hash)
	}
}

func TestReChain_ByteIdenticalSerialization(t *testing.T) {
	edges := []Edge{
		edgeAt("2026-05-31T02:00:00Z", "ag-2", "b.go"),
		edgeAt("2026-05-31T01:00:00Z", "ag-1", "a.go"),
	}
	render := func(es []Edge) []byte {
		chain, err := ReChain(es)
		if err != nil {
			t.Fatalf("rechain: %v", err)
		}
		var buf []byte
		for _, e := range chain {
			line, err := json.Marshal(e)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			buf = append(buf, line...)
			buf = append(buf, '\n')
		}
		return buf
	}
	first := render(edges)
	// Re-run with reversed input order: bytes must be identical.
	reversed := []Edge{edges[1], edges[0]}
	second := render(reversed)
	if string(first) != string(second) {
		t.Fatalf("export not byte-identical across runs:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestReChain_EmptyLedger(t *testing.T) {
	chain, err := ReChain(nil)
	if err != nil {
		t.Fatalf("rechain empty: %v", err)
	}
	if len(chain) != 0 {
		t.Fatalf("empty ledger rechain = %d edges, want 0", len(chain))
	}
	if idx, err := VerifyChain(chain); err != nil || idx != 0 {
		t.Fatalf("empty chain verify: idx=%d err=%v", idx, err)
	}
}

func TestReChain_RejectsInvalidEdge(t *testing.T) {
	bad := edgeAt("2026-05-31T01:00:00Z", "ag-1", "a.go")
	bad.Relation = "not_a_relation"
	if _, err := ReChain([]Edge{bad}); err == nil {
		t.Fatal("expected ReChain to reject an edge with an invalid relation")
	}
}

// TestReChain_TamperBreaksVerify confirms that mutating a sealed exported edge
// (a tamper) makes VerifyChain flag the exact tampered record.
func TestReChain_TamperBreaksVerify(t *testing.T) {
	chain, err := ReChain([]Edge{
		edgeAt("2026-05-31T01:00:00Z", "ag-1", "a.go"),
		edgeAt("2026-05-31T02:00:00Z", "ag-2", "b.go"),
	})
	if err != nil {
		t.Fatalf("rechain: %v", err)
	}
	// Tamper with the second record's payload without re-sealing.
	chain[1].ToID = "tampered.go"
	idx, err := VerifyChain(chain)
	if err == nil {
		t.Fatal("expected tamper to break VerifyChain")
	}
	if idx != 2 {
		t.Fatalf("tamper detected at record %d, want 2", idx)
	}
}
