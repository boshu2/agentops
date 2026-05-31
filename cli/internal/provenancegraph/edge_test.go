package provenancegraph

import (
	"encoding/json"
	"strings"
	"testing"
)

// validEdge returns a baseline field-valid (unsealed) edge for tests.
func validEdge() Edge {
	return Edge{
		SchemaVersion: SchemaVersion,
		FromID:        "ag-x31t.4",
		FromType:      "decision",
		ToID:          "cli/cmd/ao/provenance_add.go",
		ToType:        "artifact",
		Relation:      "wasGeneratedBy",
		TrustTier:     "authored",
		TS:            "2026-05-31T00:00:00Z",
	}
}

func TestValidateFields_RejectsBadEnumsAndRequired(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Edge)
		wantSub string
	}{
		{"ok", func(_ *Edge) {}, ""},
		{"bad schema", func(e *Edge) { e.SchemaVersion = "v2" }, "schema_version must be"},
		{"empty from_id", func(e *Edge) { e.FromID = "" }, "from_id is required"},
		{"empty to_id", func(e *Edge) { e.ToID = "  " }, "to_id is required"},
		{"bad from_type", func(e *Edge) { e.FromType = "thing" }, "from_type"},
		{"bad to_type", func(e *Edge) { e.ToType = "blob" }, "to_type"},
		{"bad relation", func(e *Edge) { e.Relation = "frobs" }, "relation"},
		{"bad trust_tier", func(e *Edge) { e.TrustTier = "trusted" }, "trust_tier"},
		{"empty ts", func(e *Edge) { e.TS = "" }, "ts is required"},
		{"non-utc ts", func(e *Edge) { e.TS = "2026-05-31T00:00:00+02:00" }, "ts must be UTC"},
		{"unparseable ts", func(e *Edge) { e.TS = "yesterday" }, "invalid ts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := validEdge()
			tt.mutate(&e)
			err := ValidateFields(e)
			if tt.wantSub == "" {
				if err != nil {
					t.Fatalf("expected valid, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}

func TestSeal_ProducesDeterministicHashChain(t *testing.T) {
	e := validEdge()
	sealed, err := Seal(e, "")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if sealed.PrevHash != "" {
		t.Fatalf("genesis prev_hash = %q, want empty", sealed.PrevHash)
	}
	// Deterministic: sealing the same edge again yields identical hashes.
	sealed2, err := Seal(validEdge(), "")
	if err != nil {
		t.Fatalf("seal2: %v", err)
	}
	if sealed.PayloadHash != sealed2.PayloadHash {
		t.Fatalf("payload_hash not deterministic: %q vs %q", sealed.PayloadHash, sealed2.PayloadHash)
	}
	if sealed.Hash != sealed2.Hash {
		t.Fatalf("hash not deterministic: %q vs %q", sealed.Hash, sealed2.Hash)
	}
	// hash = sha256(payload_hash + "\n" + prev_hash)
	wantHash := hashHex([]byte(sealed.PayloadHash + "\n" + ""))
	if sealed.Hash != wantHash {
		t.Fatalf("hash = %q, want %q", sealed.Hash, wantHash)
	}
	// Hash fields match the schema's 64-char lowercase-hex shape.
	if len(sealed.PayloadHash) != 64 || len(sealed.Hash) != 64 {
		t.Fatalf("hash lengths: payload=%d hash=%d, want 64", len(sealed.PayloadHash), len(sealed.Hash))
	}
}

func TestSeal_ForcesSchemaVersion(t *testing.T) {
	e := validEdge()
	e.SchemaVersion = "wrong"
	sealed, err := Seal(e, "")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if sealed.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %q, want %q", sealed.SchemaVersion, SchemaVersion)
	}
}

func TestVerifyChain_DetectsTamper(t *testing.T) {
	a, err := Seal(validEdge(), "")
	if err != nil {
		t.Fatalf("seal a: %v", err)
	}
	b := validEdge()
	b.ToID = "docs/provenance/ledger.jsonl"
	bSealed, err := Seal(b, a.Hash)
	if err != nil {
		t.Fatalf("seal b: %v", err)
	}

	// Intact two-record chain verifies.
	if idx, err := VerifyChain([]Edge{a, bSealed}); err != nil || idx != 0 {
		t.Fatalf("intact chain: idx=%d err=%v, want 0/nil", idx, err)
	}

	// Tamper with the payload of the second record without re-sealing.
	tampered := bSealed
	tampered.ToID = "evil/path"
	idx, err := VerifyChain([]Edge{a, tampered})
	if err == nil {
		t.Fatal("expected tamper to break the chain")
	}
	if idx != 2 {
		t.Fatalf("first broken index = %d, want 2", idx)
	}

	// Break the chain link by mismatching prev_hash.
	broken := bSealed
	broken.PrevHash = "0000000000000000000000000000000000000000000000000000000000000000"
	idx2, err2 := VerifyChain([]Edge{a, broken})
	if err2 == nil || idx2 != 2 {
		t.Fatalf("broken prev_hash: idx=%d err=%v, want 2/non-nil", idx2, err2)
	}
}

func TestEdgeIdentity_IgnoresTimestampAndHashes(t *testing.T) {
	e1 := validEdge()
	e1.TS = "2026-01-01T00:00:00Z"
	sealed1, _ := Seal(e1, "")
	e2 := validEdge()
	e2.TS = "2026-12-31T23:59:59Z"
	sealed2, _ := Seal(e2, sealed1.Hash)

	if EdgeIdentity(sealed1) != EdgeIdentity(sealed2) {
		t.Fatal("identity changed across ts/hash differences; should be stable")
	}

	// A differing endpoint changes identity.
	e3 := validEdge()
	e3.ToID = "other.go"
	if EdgeIdentity(e1) == EdgeIdentity(e3) {
		t.Fatal("identity should differ when to_id differs")
	}
}

// TestEdge_JSONFieldNamesMatchSchema asserts the marshaled edge uses exactly
// the field names the v1 schema requires (regression guard against tag drift).
func TestEdge_JSONFieldNamesMatchSchema(t *testing.T) {
	sealed, err := Seal(validEdge(), "")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	b, err := json.Marshal(sealed)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{
		"schema_version", "from_id", "from_type", "to_id", "to_type",
		"relation", "trust_tier", "ts", "prev_hash", "payload_hash", "hash",
	} {
		if _, ok := m[key]; !ok {
			t.Errorf("marshaled edge missing required field %q", key)
		}
	}
	// evidence_ref is omitempty: absent when empty.
	if _, ok := m["evidence_ref"]; ok {
		t.Error("evidence_ref should be omitted when empty")
	}
}
