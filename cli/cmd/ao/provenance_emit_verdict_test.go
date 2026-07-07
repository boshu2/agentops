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

// TestBuildVerdictCommitEdge_V11Enrichment covers the additive v1.1 derivation
// (age-rk3r.3): reviewer_family is the sorted, de-duplicated, alias-collapsed
// canonical family set; evidence_path is the first refuter evidence (else the
// council_artifact); bead_id is set as the structured join key; and the fields
// with no source in a v1 verdict file (degraded/rounds/duration_s) stay empty
// until the sibling beads populate them.
func TestBuildVerdictCommitEdge_V11Enrichment(t *testing.T) {
	cases := []struct {
		name         string
		v            pawlVerdict
		wantFamily   string
		wantEvidence string
	}{
		{
			name: "multi-family collapses aliases, sorts, dedups; first refuter evidence wins",
			v: pawlVerdict{
				BeadID: "ag-a", HeadSHA: "0123456789ab", Disposition: "CONFIRMED",
				Refuters: []pawlRefuter{
					{Family: "codex", Evidence: ".agents/r1.md"},
					{Family: "fable"},
					{Family: "claude"},
				},
			},
			wantFamily:   "claude+gpt", // fable->claude (dedup w/ claude), codex->gpt; sorted
			wantEvidence: ".agents/r1.md",
		},
		{
			name: "single family; evidence falls back to council_artifact",
			v: pawlVerdict{
				BeadID: "ag-b", HeadSHA: "abcdef012345", Disposition: "CONFIRMED",
				Refuters:        []pawlRefuter{{Family: "claude"}},
				CouncilArtifact: ".agents/council/x.md",
			},
			wantFamily:   "claude",
			wantEvidence: ".agents/council/x.md",
		},
		{
			name: "off-roster family and no evidence => empty enrichment (not junk)",
			v: pawlVerdict{
				BeadID: "ag-c", HeadSHA: "aaaaaaabbbbb", Disposition: "REFUTED",
				Refuters: []pawlRefuter{{Family: "totally-fake"}},
			},
			wantFamily:   "",
			wantEvidence: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := buildVerdictCommitEdge(tc.v)
			if e.ReviewerFamily != tc.wantFamily {
				t.Errorf("ReviewerFamily = %q, want %q", e.ReviewerFamily, tc.wantFamily)
			}
			if e.EvidencePath != tc.wantEvidence {
				t.Errorf("EvidencePath = %q, want %q", e.EvidencePath, tc.wantEvidence)
			}
			if e.BeadID != tc.v.BeadID {
				t.Errorf("BeadID = %q, want %q (structured join key)", e.BeadID, tc.v.BeadID)
			}
			// These fixtures carry no attempt/degraded/cost — the meter fields
			// (ebec.1) must stay at zero so omitempty drops them (pre-meter shape).
			if e.Degraded || e.Rounds != 0 || e.DurationS != 0 || e.TokensEst != 0 {
				t.Errorf("degraded/rounds/duration_s/tokens_est should be empty, got %v/%d/%g/%d",
					e.Degraded, e.Rounds, e.DurationS, e.TokensEst)
			}
		})
	}
}

// TestBuildVerdictCommitEdge_CostMeter proves the verification-economics meter
// (age-verification-economics-ebec.1): a verdict carrying attempt/degraded and
// the cost object projects into the edge's rounds/degraded/duration_s/tokens_est
// — and a nil cost stays fully absent (additive back-compat).
func TestBuildVerdictCommitEdge_CostMeter(t *testing.T) {
	withCost := pawlVerdict{
		BeadID: "ag-cost", HeadSHA: "0123456789ab", Disposition: "CONFIRMED",
		Refuters: []pawlRefuter{{Family: "gemini", Evidence: ".agents/e.txt"}},
		Attempt:  2,
		Degraded: true,
		Cost:     &pawlCost{WallSeconds: 312.0, TokensEst: 48200, Estimated: true},
	}
	e := buildVerdictCommitEdge(withCost)
	e.SchemaVersion = provenancegraph.SchemaVersion
	e.TS = "2026-07-07T00:00:00Z"
	if e.DurationS != 312.0 {
		t.Errorf("DurationS = %g, want 312.0 (cost.wall_seconds)", e.DurationS)
	}
	if e.TokensEst != 48200 {
		t.Errorf("TokensEst = %d, want 48200 (cost.tokens_est)", e.TokensEst)
	}
	if e.Rounds != 2 {
		t.Errorf("Rounds = %d, want 2 (verdict attempt)", e.Rounds)
	}
	if !e.Degraded {
		t.Errorf("Degraded = false, want true (verdict degraded)")
	}

	// Payload round-trip: the metered edge's payload JSON must carry the fields...
	sealed, err := provenancegraph.Seal(e, "")
	if err != nil {
		t.Fatalf("SealEdge(metered): %v", err)
	}
	b, _ := json.Marshal(sealed)
	for _, want := range []string{`"duration_s":312`, `"tokens_est":48200`, `"rounds":2`, `"degraded":true`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("sealed metered edge missing %s in %s", want, string(b))
		}
	}

	// ...and an unmetered verdict's payload must omit every meter field, so
	// pre-meter records keep byte-identical payload hashes.
	noCost := pawlVerdict{
		BeadID: "ag-nocost", HeadSHA: "abcdef012345", Disposition: "CONFIRMED",
		Refuters: []pawlRefuter{{Family: "gemini"}},
	}
	e2 := buildVerdictCommitEdge(noCost)
	e2.SchemaVersion = provenancegraph.SchemaVersion
	e2.TS = "2026-07-07T00:00:00Z"
	sealed2, err := provenancegraph.Seal(e2, "")
	if err != nil {
		t.Fatalf("SealEdge(unmetered): %v", err)
	}
	b2, _ := json.Marshal(sealed2)
	for _, absent := range []string{"duration_s", "tokens_est", "rounds", "degraded"} {
		if strings.Contains(string(b2), absent) {
			t.Errorf("unmetered edge unexpectedly carries %q: %s", absent, string(b2))
		}
	}
}

// TestEmitVerdict_RealArtifact_DerivesReviewerFamily proves the sensor derives
// the v1.1 reviewer_family from the REAL producer artifact (fixture-fidelity):
// the tracked sample carries one refuter (family claude) and no evidence path,
// so the edge gains reviewer_family=claude + a structured bead_id while
// evidence_path/degraded/rounds/duration_s stay empty.
func TestEmitVerdict_RealArtifact_DerivesReviewerFamily(t *testing.T) {
	fixture := filepath.Join("..", "..", "..", "tests", "fixtures", "provenance", "pawl-verdict-real-sample.json")
	v, err := extractVerdict(fixture)
	if err != nil {
		t.Fatalf("extractVerdict: %v", err)
	}
	e := buildVerdictCommitEdge(v)
	if e.ReviewerFamily != "claude" {
		t.Errorf("ReviewerFamily = %q, want claude", e.ReviewerFamily)
	}
	if e.EvidencePath != "" {
		t.Errorf("EvidencePath = %q, want empty (fixture carries no evidence path)", e.EvidencePath)
	}
	if e.BeadID != v.BeadID {
		t.Errorf("BeadID = %q, want %q", e.BeadID, v.BeadID)
	}
}

// TestEmitVerdict_ConsumesRealProducerArtifact is the producer→sensor seam
// guard (age-d16-self-hosting-route-nkr.1 / M1). The fixture is the REAL on-disk
// artifact emitted by `scripts/pawl-verdict.sh write` (not a hand-built struct),
// per the fixture-fidelity rule in standards/references/test-pyramid.md. It
// locks the contract that the sensor (extractVerdict → buildVerdictCommitEdge →
// ledger append) consumes the producer's actual output shape: the producer
// emits extra fields (pr, generated_at, author_context_id, attempt, refuters)
// the sensor must ignore while still extracting bead_id/head_sha/disposition.
// If the two halves drift apart the verdict feed silently dies (back to 0
// verdict rows) — exactly the M1 starvation this guards against.
func TestEmitVerdict_ConsumesRealProducerArtifact(t *testing.T) {
	fixture := filepath.Join("..", "..", "..", "tests", "fixtures", "provenance", "pawl-verdict-real-sample.json")

	v, err := extractVerdict(fixture)
	if err != nil {
		t.Fatalf("extractVerdict on real producer artifact: %v", err)
	}
	if v.BeadID != "age-d16-self-hosting-route-nkr.1" {
		t.Errorf("BeadID = %q, want age-d16-self-hosting-route-nkr.1", v.BeadID)
	}
	if v.HeadSHA != "611615d9b78717eca0fa1b2d1eb75a54c9dc6970" {
		t.Errorf("HeadSHA = %q, want the fixture's head_sha", v.HeadSHA)
	}
	if v.Disposition != "CONFIRMED" {
		t.Errorf("Disposition = %q, want CONFIRMED", v.Disposition)
	}

	// The extracted verdict appends a valid, hash-chained edge to the ledger,
	// and the verdict→commit join key is the FULL head_sha (the footgun: it
	// must be the SHA that lands on origin/main, not a truncated/local one).
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	store := provenancegraph.NewStore(ledger)
	edge := buildVerdictCommitEdge(v)
	edge.TS = "2026-06-16T00:00:00Z"
	res, err := store.Append(edge)
	if err != nil {
		t.Fatalf("append edge from real artifact: %v", err)
	}
	if res.Skipped {
		t.Error("first emit of the real artifact should not be skipped")
	}
	if res.Edge.ToID != v.HeadSHA {
		t.Errorf("edge to_id = %q, want full head_sha %q (the SHA join key)", res.Edge.ToID, v.HeadSHA)
	}
	if res.Edge.FromType != "verdict" {
		t.Errorf("edge from_type = %q, want verdict", res.Edge.FromType)
	}
	vr, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("verify after appending real-artifact edge: %v", err)
	}
	if !vr.Pass {
		t.Fatalf("chain not intact after real-artifact append: %s", vr.Message)
	}
}

// TestEmitVerdict_ReboundArtifact_RoundTripsChain is the age-rk3r.9 acceptance
// proof: a REBOUND verdict edge round-trips `ao provenance verify` clean (the
// chain stays intact). The fixture is the REAL on-disk shape the shell writer
// `scripts/pawl-verdict.sh rebind-verified` emits (disposition REBOUND + the
// three lineage fields rebound_from_verdict/rebound_from_sha/patch_id_proof),
// per the fixture-fidelity rule — not a hand-built struct. The sensor must emit a
// schema-valid, hash-chained edge for a REBOUND exactly as it does for a
// CONFIRMED (the disposition is carried in evidence_ref), so the provenance chain
// never breaks when a byte-identical rebase is re-bound instead of re-reviewed.
func TestEmitVerdict_ReboundArtifact_RoundTripsChain(t *testing.T) {
	fixture := filepath.Join("..", "..", "..", "tests", "fixtures", "provenance", "pawl-verdict-rebound-sample.json")

	v, err := extractVerdict(fixture)
	if err != nil {
		t.Fatalf("extractVerdict on real REBOUND artifact: %v", err)
	}
	if v.Disposition != "REBOUND" {
		t.Errorf("Disposition = %q, want REBOUND", v.Disposition)
	}
	if v.HeadSHA != "3fb96d7908a9cf81cd51acb9a2b1c1991f8a7449" {
		t.Errorf("HeadSHA = %q, want the fixture's head_sha", v.HeadSHA)
	}

	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	store := provenancegraph.NewStore(ledger)
	edge := buildVerdictCommitEdge(v)
	edge.TS = "2026-07-02T00:00:00Z"

	res, err := store.Append(edge)
	if err != nil {
		t.Fatalf("append REBOUND edge: %v", err)
	}
	if res.Skipped {
		t.Error("first emit of the REBOUND artifact should not be skipped")
	}
	// The disposition rides in evidence_ref so downstream consumers can see the edge
	// is a REBOUND, not a fresh CONFIRMED.
	if !strings.Contains(res.Edge.EvidenceRef, "disposition=REBOUND") {
		t.Errorf("edge evidence_ref = %q, want it to carry disposition=REBOUND", res.Edge.EvidenceRef)
	}
	if res.Edge.ToID != v.HeadSHA {
		t.Errorf("edge to_id = %q, want full head_sha %q", res.Edge.ToID, v.HeadSHA)
	}

	// The chain verifies clean — the REBOUND edge round-trips provenance verify.
	vr, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("verify after appending REBOUND edge: %v", err)
	}
	if !vr.Pass {
		t.Fatalf("chain not intact after REBOUND append: %s (line %d)", vr.Message, vr.FirstBrokenLine)
	}
	if vr.RecordCount != 1 {
		t.Errorf("RecordCount = %d, want 1", vr.RecordCount)
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
