// practices: [in-toto-provenance, dora-metrics]

package drrebuild

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildChainedLedger takes payload-only events (hash fields empty) and returns
// a fully hash-chained ledger plus its JSONL serialization, using the same
// discipline the rebuild verifies. This is the "original export" side of the
// proof: it stands in for what the production ledger writer commits.
func buildChainedLedger(t *testing.T, payloads []LedgerEvent) ([]LedgerEvent, []byte) {
	t.Helper()
	prev := ""
	var buf bytes.Buffer
	chained := make([]LedgerEvent, 0, len(payloads))
	for i := range payloads {
		ev := payloads[i]
		ev.PrevHash = prev
		ph, h, err := computeHashes(ev)
		if err != nil {
			t.Fatalf("computeHashes[%d]: %v", i, err)
		}
		ev.PayloadHash = ph
		ev.Hash = h
		prev = h
		chained = append(chained, ev)
		b, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("marshal[%d]: %v", i, err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return chained, buf.Bytes()
}

// decisionBlob and artifactBlob are the content-addressed evidence bodies. The
// ledger references their git blob OIDs as evidence_ref.
const (
	decisionBlob = "Decision: invert source of truth — append-only event log is canonical; Dolt is a rebuildable projection.\n"
	artifactBlob = "schemas/agentops-sdlc-provenance.v1.schema.json\n"
)

// fixturePayloads returns the canonical proof scenario as payload-only events:
// a small but representative SDLC provenance graph (a decision that produces an
// artifact, a bead that scopes the decision, a commit that implements it, with
// content-addressed evidence on the content-bearing edges).
func fixturePayloads() ([]LedgerEvent, MapBlobStore) {
	blobs := MapBlobStore{
		GitBlobOID([]byte(decisionBlob)): []byte(decisionBlob),
		GitBlobOID([]byte(artifactBlob)): []byte(artifactBlob),
	}
	events := []LedgerEvent{
		{
			SchemaVersion: "agentops-sdlc-provenance.v1",
			FromID:        "ag-lmdx", FromType: "bead",
			ToID: "ag-lmdx.1", ToType: "decision",
			Relation: "wasInfluencedBy", TrustTier: "authored",
			TS:          "2026-05-30T00:00:00Z",
			EvidenceRef: "ag-lmdx",
		},
		{
			SchemaVersion: "agentops-sdlc-provenance.v1",
			FromID:        "ag-lmdx.1", FromType: "decision",
			ToID: "schemas/agentops-sdlc-provenance.v1.schema.json", ToType: "artifact",
			Relation: "wasGeneratedBy", TrustTier: "authored",
			TS:          "2026-05-30T01:00:00Z",
			EvidenceRef: GitBlobOID([]byte(artifactBlob)),
		},
		{
			SchemaVersion: "agentops-sdlc-provenance.v1",
			FromID:        "commit:004fcbc2", FromType: "commit",
			ToID: "ag-lmdx.1", ToType: "decision",
			Relation: "wasRevisionOf", TrustTier: "inferred",
			TS:          "2026-05-30T02:00:00Z",
			EvidenceRef: GitBlobOID([]byte(decisionBlob)),
		},
	}
	return events, blobs
}

// TestRebuild_MatchesOriginal is the bead's first scenario: "Rebuild matches
// original". Given a populated context graph with an exported hash-chained
// JSONL and its git blobs, when Dolt is wiped and rebuild runs from JSONL +
// blobs, the reconstructed graph hash-matches the original.
func TestRebuild_MatchesOriginal(t *testing.T) {
	payloads, blobs := fixturePayloads()
	chained, jsonl := buildChainedLedger(t, payloads)

	// "Original" projection: built directly from the in-memory chained events
	// (stands in for the live Dolt graph at export time).
	original, err := Rebuild(chained, blobs)
	if err != nil {
		t.Fatalf("building original projection: %v", err)
	}

	// DR path: parse the committed JSONL bytes, verify the chain, replay.
	rebuilt, err := RebuildFromLedger(bytes.NewReader(jsonl), blobs)
	if err != nil {
		t.Fatalf("RebuildFromLedger: %v", err)
	}

	if rebuilt.Hash() != original.Hash() {
		t.Fatalf("rebuilt graph hash != original:\n  original=%s\n  rebuilt =%s\n  origCanon=%s\n  rebuiltCanon=%s",
			original.Hash(), rebuilt.Hash(), original.Canonicalize(), rebuilt.Canonicalize())
	}

	// Exact structural assertions (not just hash-equal): 4 distinct nodes, 3 edges.
	if got, want := len(rebuilt.Nodes), 4; got != want {
		t.Fatalf("node count = %d, want %d (nodes=%+v)", got, want, rebuilt.Nodes)
	}
	if got, want := len(rebuilt.Edges), 3; got != want {
		t.Fatalf("edge count = %d, want %d", got, want)
	}

	// The artifact node must be believed at the authored tier.
	canon := rebuilt.Canonicalize()
	wantNode := `{"id":"schemas/agentops-sdlc-provenance.v1.schema.json","type":"artifact","trust_tier":"authored"}`
	if !strings.Contains(string(canon), wantNode) {
		t.Fatalf("canonical graph missing expected artifact node %s\ngot: %s", wantNode, canon)
	}

	// Content-addressed evidence must be resolved verbatim into the projection.
	var found bool
	for _, e := range rebuilt.Edges {
		if e.Relation == "wasGeneratedBy" {
			found = true
			if e.Evidence != artifactBlob {
				t.Fatalf("resolved evidence = %q, want %q", e.Evidence, artifactBlob)
			}
		}
	}
	if !found {
		t.Fatal("wasGeneratedBy edge not present in rebuilt graph")
	}
}

// TestRebuild_FromCommittedFixture proves the rebuild is reproducible from a
// STATIC committed JSONL fixture file + a static blob set on disk — i.e. a
// fresh clone with no live Dolt can rebuild and hit a frozen expected hash.
func TestRebuild_FromCommittedFixture(t *testing.T) {
	dir := filepath.Join("testdata", "fixture-graph")

	jsonl, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatalf("reading fixture ledger: %v", err)
	}
	blobs := loadBlobDir(t, filepath.Join(dir, "blobs"))

	g, err := RebuildFromLedger(bytes.NewReader(jsonl), blobs)
	if err != nil {
		t.Fatalf("RebuildFromLedger(fixture): %v", err)
	}

	wantHash, err := os.ReadFile(filepath.Join(dir, "expected-graph-hash.txt"))
	if err != nil {
		t.Fatalf("reading expected hash: %v", err)
	}
	want := strings.TrimSpace(string(wantHash))
	if got := g.Hash(); got != want {
		t.Fatalf("fixture rebuild hash mismatch:\n  got  =%s\n  want =%s\n  canon=%s", got, want, g.Canonicalize())
	}
}

// loadBlobDir loads every file in dir as a blob keyed by its git OID, asserting
// the on-disk filename (the committed OID) matches the recomputed OID.
func loadBlobDir(t *testing.T, dir string) MapBlobStore {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading blob dir: %v", err)
	}
	store := MapBlobStore{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading blob %s: %v", e.Name(), err)
		}
		oid := GitBlobOID(body)
		if oid != e.Name() {
			t.Fatalf("blob filename %q != recomputed git OID %q (fixture corrupt)", e.Name(), oid)
		}
		store[oid] = body
	}
	return store
}

// TestRebuild_MissingBlobDetectedNotDropped is the bead's second scenario:
// "Missing blob is detected not silently dropped". An exported JSONL
// referencing a content_hash whose git blob is absent must fail loudly, naming
// the dangling content_ref, and must not present a partial graph as complete.
func TestRebuild_MissingBlobDetectedNotDropped(t *testing.T) {
	payloads, blobs := fixturePayloads()
	chained, jsonl := buildChainedLedger(t, payloads)

	// Simulate git GC pruning the artifact's blob.
	missingOID := GitBlobOID([]byte(artifactBlob))
	delete(blobs, missingOID)

	g, err := RebuildFromLedger(bytes.NewReader(jsonl), blobs)
	if g != nil {
		t.Fatalf("expected nil graph on dangling ref, got non-nil graph (partial graph presented as complete): %+v", g)
	}
	if err == nil {
		t.Fatal("expected DanglingRefError, got nil (missing blob silently dropped — the do-not-ship failure)")
	}
	var dre *DanglingRefError
	if !errors.As(err, &dre) {
		t.Fatalf("expected *DanglingRefError, got %T: %v", err, err)
	}
	if dre.ContentHash != missingOID {
		t.Fatalf("DanglingRefError names %q, want missing OID %q", dre.ContentHash, missingOID)
	}
	if !strings.Contains(err.Error(), missingOID) {
		t.Fatalf("error message does not name the dangling content_ref %q: %s", missingOID, err.Error())
	}
	// Sanity: the chained ledger itself was valid (failure is the blob, not the chain).
	if verr := VerifyChain(chained); verr != nil {
		t.Fatalf("precondition: chain should verify, got: %v", verr)
	}
}

// TestVerifyChain_DetectsTamper proves the witness property: mutating any
// hash-covered field after export breaks chain verification, so a rebuild from
// a tampered ledger is refused rather than trusted.
func TestVerifyChain_DetectsTamper(t *testing.T) {
	payloads, _ := fixturePayloads()
	chained, _ := buildChainedLedger(t, payloads)

	tampered := append([]LedgerEvent(nil), chained...)
	tampered[1].TrustTier = "authored" // was authored already; change to a real downgrade
	tampered[1].Relation = "wasAttributedTo"

	if err := VerifyChain(tampered); err == nil {
		t.Fatal("expected chain verification to detect the tampered relation, got nil")
	}
}

// TestVerifyChain_DetectsReorder proves reordering records (without re-chaining)
// breaks the prev_hash linkage.
func TestVerifyChain_DetectsReorder(t *testing.T) {
	payloads, _ := fixturePayloads()
	chained, _ := buildChainedLedger(t, payloads)
	if len(chained) < 2 {
		t.Fatal("fixture must have >=2 events")
	}
	reordered := []LedgerEvent{chained[1], chained[0], chained[2]}
	if err := VerifyChain(reordered); err == nil {
		t.Fatal("expected chain verification to detect reordering, got nil")
	}
}

// TestComputeEventHashes_AgreesWithChain proves the exported writer helper
// produces the same payload_hash/hash that VerifyChain accepts — so a ledger
// written with ComputeEventHashes is rebuildable by this DR path.
func TestComputeEventHashes_AgreesWithChain(t *testing.T) {
	payloads, _ := fixturePayloads()
	prev := ""
	var chained []LedgerEvent
	for _, p := range payloads {
		p.PrevHash = prev
		ph, h := ComputeEventHashes(p)
		p.PayloadHash, p.Hash = ph, h
		prev = h
		chained = append(chained, p)
	}
	if err := VerifyChain(chained); err != nil {
		t.Fatalf("chain built via ComputeEventHashes failed VerifyChain: %v", err)
	}
}

// TestGitBlobOID_MatchesGit pins the content-addressing to git's exact scheme.
// `printf 'hello' | git hash-object --stdin` == this OID.
func TestGitBlobOID_MatchesGit(t *testing.T) {
	got := GitBlobOID([]byte("hello"))
	const want = "b6fc4c620b67d95f953a5c1c1230aaab5db5a1b0"
	if got != want {
		t.Fatalf("GitBlobOID(\"hello\") = %s, want git's %s", got, want)
	}
}

// TestIsContentRef distinguishes content-addressed evidence refs (40-hex git
// OIDs) from other evidence pointers (bead ids, paths, URLs) so non-content
// refs are not spuriously resolved against the blob store.
func TestIsContentRef(t *testing.T) {
	cases := []struct {
		ref  string
		want bool
	}{
		{"b6fc4c620b67d95f953a5c1c1230aaab5db5a1b0", true},
		{"ag-lmdx", false},
		{"docs/provenance/ledger.jsonl", false},
		{"https://ci.example/run/42", false},
		{"B6FC4C620B67D95F953A5C1C1230AAAB5DB5A1B0", false}, // uppercase: not git's lowercase OID form
		{"", false},
	}
	for _, c := range cases {
		if got := isContentRef(c.ref); got != c.want {
			t.Fatalf("isContentRef(%q) = %v, want %v", c.ref, got, c.want)
		}
	}
}

// TestVerifyChain_V11EnrichmentAdditive is the drrebuild half of the age-rk3r.3
// additivity guarantee: this parallel implementation of the ledger hashing must
// stay faithful when a record carries the v1.1 enrichment fields (reviewer_family,
// degraded, rounds, duration_s, evidence_path), which ARE part of payload_hash.
// A chain mixing a v1-shaped event and a v1.1-shaped event verifies clean, and
// the fields survive the JSONL round-trip — proving the DR rebuild neither
// falsely reports v1.1 records as tampered nor silently drops the enrichment.
func TestVerifyChain_V11EnrichmentAdditive(t *testing.T) {
	chained, jsonl := buildChainedLedger(t, []LedgerEvent{
		{SchemaVersion: "agentops-sdlc-provenance.v1", FromID: "ag-old", FromType: "bead",
			ToID: "abc123", ToType: "commit", Relation: "wasGeneratedBy",
			TrustTier: "authored", TS: "2026-07-01T00:00:00Z"},
		{SchemaVersion: "agentops-sdlc-provenance.v1", FromID: "ag-new@abc1234", FromType: "verdict",
			ToID: "abc1234def", ToType: "commit", Relation: "wasDerivedFrom",
			TrustTier: "inferred", TS: "2026-07-01T01:00:00Z",
			ReviewerFamily: "claude+gpt", Degraded: true, Rounds: 2, DurationS: 7.5,
			EvidencePath: ".agents/x.md"},
	})

	// The mixed chain verifies through the (now v1.1-aware) parallel implementation.
	if err := VerifyChain(chained); err != nil {
		t.Fatalf("mixed v1/v1.1 chain should verify, got: %v", err)
	}

	// The enrichment survives the JSONL round-trip (parse then re-verify).
	parsed, err := ParseLedger(bytes.NewReader(jsonl))
	if err != nil {
		t.Fatalf("ParseLedger: %v", err)
	}
	if err := VerifyChain(parsed); err != nil {
		t.Fatalf("parsed mixed chain should verify, got: %v", err)
	}
	v11 := parsed[1]
	if v11.ReviewerFamily != "claude+gpt" || !v11.Degraded || v11.Rounds != 2 ||
		v11.DurationS != 7.5 || v11.EvidencePath != ".agents/x.md" {
		t.Fatalf("v1.1 enrichment lost through the ledger round-trip: %+v", v11)
	}
	// The v1 event stays field-empty (additive; no data invented).
	if parsed[0].ReviewerFamily != "" || parsed[0].Degraded || parsed[0].Rounds != 0 ||
		parsed[0].DurationS != 0 || parsed[0].EvidencePath != "" {
		t.Errorf("v1 event unexpectedly gained enrichment fields: %+v", parsed[0])
	}
}
