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

// TestJoinKeys_AreNonPayload_HashUnaffected is the load-bearing invariant for
// ag-5qltf: the additive (bead_id, merge_sha) mesh join keys MUST NOT enter the
// payload hash. If they did, every edge committed before the fields existed
// would fail VerifyChain. Proof: an edge sealed WITH the join keys has the
// identical payload_hash/hash as the same edge sealed WITHOUT them, and a chain
// mixing both verifies.
func TestJoinKeys_AreNonPayload_HashUnaffected(t *testing.T) {
	bare := validEdge()
	withKeys := validEdge()
	withKeys.BeadID = "ag-62jrm"
	withKeys.MergeSHA = "abc1234def5678"

	sealedBare, err := Seal(bare, "")
	if err != nil {
		t.Fatalf("seal bare: %v", err)
	}
	sealedKeys, err := Seal(withKeys, "")
	if err != nil {
		t.Fatalf("seal withKeys: %v", err)
	}

	if sealedBare.PayloadHash != sealedKeys.PayloadHash {
		t.Fatalf("payload_hash changed by join keys: bare=%s withKeys=%s — join keys leaked into the payload, which would break every pre-existing committed edge",
			sealedBare.PayloadHash, sealedKeys.PayloadHash)
	}
	if sealedBare.Hash != sealedKeys.Hash {
		t.Fatalf("hash changed by join keys: bare=%s withKeys=%s", sealedBare.Hash, sealedKeys.Hash)
	}
	// The keys still round-trip on the record (just not into the hash).
	if sealedKeys.BeadID != "ag-62jrm" || sealedKeys.MergeSHA != "abc1234def5678" {
		t.Fatalf("join keys lost after seal: bead_id=%q merge_sha=%q", sealedKeys.BeadID, sealedKeys.MergeSHA)
	}
	// A chain mixing a legacy (no-keys) edge and a join-key edge verifies intact.
	second, err := Seal(withKeys, sealedBare.Hash)
	if err != nil {
		t.Fatalf("seal second: %v", err)
	}
	if idx, err := VerifyChain([]Edge{sealedBare, second}); err != nil || idx != 0 {
		t.Fatalf("mixed legacy+join-key chain: idx=%d err=%v, want 0/nil", idx, err)
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
	// v1.1 enrichment fields are omitempty: all absent on a bare edge.
	for _, key := range []string{"reviewer_family", "degraded", "rounds", "duration_s", "evidence_path"} {
		if _, ok := m[key]; ok {
			t.Errorf("v1.1 field %q should be omitted when empty (additivity)", key)
		}
	}
}

// legacyEdgePayload mirrors the PRE-v1.1 edgePayload shape (before age-rk3r.3):
// the exact 9 payload fields an ao binary predating the enrichment marshals. It
// is the fixture for the documented compatibility boundary — a reader with this
// struct DROPS the v1.1 fields and therefore recomputes a different payload_hash
// on any record that sets them.
type legacyEdgePayload struct {
	SchemaVersion string `json:"schema_version"`
	FromID        string `json:"from_id"`
	FromType      string `json:"from_type"`
	ToID          string `json:"to_id"`
	ToType        string `json:"to_type"`
	Relation      string `json:"relation"`
	EvidenceRef   string `json:"evidence_ref,omitempty"`
	TrustTier     string `json:"trust_tier"`
	TS            string `json:"ts"`
}

// legacyPayloadHash recomputes payload_hash the way a pre-v1.1 ao binary would:
// marshal ONLY the legacy 9 fields (dropping any v1.1 enrichment), then sha256.
func legacyPayloadHash(e Edge) string {
	b, _ := json.Marshal(legacyEdgePayload{
		SchemaVersion: e.SchemaVersion,
		FromID:        e.FromID,
		FromType:      e.FromType,
		ToID:          e.ToID,
		ToType:        e.ToType,
		Relation:      e.Relation,
		EvidenceRef:   e.EvidenceRef,
		TrustTier:     e.TrustTier,
		TS:            e.TS,
	})
	return hashHex(b)
}

// TestEnrichmentFields_AdditiveHashProtection is the core OPTION-C invariant
// (age-rk3r.3): the v1.1 enrichment fields are both (a) ADDITIVE — an edge that
// leaves them empty seals to the identical payload_hash a pre-v1.1 binary would
// compute, so every pre-existing committed edge keeps its hash and VerifyChain
// stays intact — and (b) HASH-PROTECTED — an edge that sets any field changes the
// payload_hash (unlike the bead_id/merge_sha join keys, which do not).
func TestEnrichmentFields_AdditiveHashProtection(t *testing.T) {
	bare := validEdge()
	sealedBare, err := Seal(bare, "")
	if err != nil {
		t.Fatalf("seal bare: %v", err)
	}
	// (a) ADDITIVE: an empty-enrichment edge hashes identically to the legacy
	// 9-field payload — proof that omitempty keeps pre-v1.1 records unchanged.
	if got := legacyPayloadHash(sealedBare); got != sealedBare.PayloadHash {
		t.Fatalf("empty-enrichment payload_hash %s != legacy recompute %s — the fields are NOT additive; every pre-v1.1 committed edge would break",
			sealedBare.PayloadHash, got)
	}

	// (b) HASH-PROTECTED: setting the enrichment fields changes payload_hash.
	enriched := validEdge()
	enriched.ReviewerFamily = "claude+gpt"
	enriched.Rounds = 3
	enriched.DurationS = 12.5
	enriched.Degraded = true
	enriched.EvidencePath = ".agents/pawl-verdicts/ag-x.transcript"
	sealedEnriched, err := Seal(enriched, "")
	if err != nil {
		t.Fatalf("seal enriched: %v", err)
	}
	if sealedEnriched.PayloadHash == sealedBare.PayloadHash {
		t.Fatal("enrichment fields did not change payload_hash — they are not hash-protected")
	}
	// The legacy reader's recompute of the enriched edge DIFFERS from the stored
	// hash (it dropped the fields): the documented old-reader mismatch.
	if legacyPayloadHash(sealedEnriched) == sealedEnriched.PayloadHash {
		t.Fatal("legacy recompute matched the enriched payload_hash — expected the boundary mismatch")
	}
	// Fields round-trip on the record.
	if sealedEnriched.ReviewerFamily != "claude+gpt" || sealedEnriched.Rounds != 3 ||
		sealedEnriched.DurationS != 12.5 || !sealedEnriched.Degraded ||
		sealedEnriched.EvidencePath != ".agents/pawl-verdicts/ag-x.transcript" {
		t.Fatalf("enrichment fields lost after seal: %+v", sealedEnriched)
	}
}

// TestEnrichment_MixedChain_NewReaderPassesOldReaderFalseBreaks is the versioned
// hash-reader boundary test (age-rk3r.3 acceptance i): a chain mixing a v1-shaped
// edge and a v1.1-shaped edge verifies clean under the CURRENT reader
// (VerifyChain), and reproduces the OLD-reader false-break — a reader predating
// the fields recomputes a different payload_hash on the v1.1 record (a false
// "broken chain") while still matching every v1 record.
func TestEnrichment_MixedChain_NewReaderPassesOldReaderFalseBreaks(t *testing.T) {
	v1, err := Seal(validEdge(), "")
	if err != nil {
		t.Fatalf("seal v1: %v", err)
	}
	v11edge := validEdge()
	v11edge.FromType = "verdict"
	v11edge.ToType = "commit"
	v11edge.Relation = "wasDerivedFrom"
	v11edge.TrustTier = "inferred"
	v11edge.ReviewerFamily = "claude"
	v11, err := Seal(v11edge, v1.Hash)
	if err != nil {
		t.Fatalf("seal v11: %v", err)
	}
	chain := []Edge{v1, v11}

	// CURRENT reader: the mixed chain is intact.
	if idx, err := VerifyChain(chain); err != nil || idx != 0 {
		t.Fatalf("current reader on mixed chain: idx=%d err=%v, want 0/nil", idx, err)
	}

	// OLD reader simulation: recompute each record's payload_hash with the legacy
	// 9-field payload. The v1 record still matches; the v1.1 record does NOT.
	if legacyPayloadHash(v1) != v1.PayloadHash {
		t.Fatal("old reader should still verify the pre-v1.1 record, but its payload_hash recompute differs")
	}
	if legacyPayloadHash(v11) == v11.PayloadHash {
		t.Fatal("old reader should FALSE-BREAK on the v1.1 record (dropped fields), but its recompute matched")
	}
}
