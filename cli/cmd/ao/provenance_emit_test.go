package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// TestEmitLanded_LedgerGrowsAndStaysIntact is the L2 scenario proof for
// ag-62jrm: emitting bead→commit edges for a landed commit appends schema-valid,
// hash-chained rows to the ledger, the chain verifies, and re-emitting is an
// idempotent no-op. Exercises the REAL store on a temp ledger (fixture-fidelity:
// production writer + production verifier, no hand-built shape).
func TestEmitLanded_LedgerGrowsAndStaysIntact(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	store := provenancegraph.NewStore(ledger)

	commitSHA := "0123456789abcdef0123456789abcdef01234567"
	msg := "feat(x): two arcs (ag-aaa #s)\n\nCloses-scenario: ag-bbb#t\n"

	ids := extractLandedBeadIDs(msg)
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %v", ids)
	}

	// First emission: every edge is appended (not skipped).
	for _, id := range ids {
		edge := buildBeadCommitEdge(id, commitSHA)
		edge.TS = "2026-06-15T00:00:00Z"
		res, err := store.Append(edge)
		if err != nil {
			t.Fatalf("append %s: %v", id, err)
		}
		if res.Skipped {
			t.Errorf("first emit of %s should not be skipped", id)
		}
	}

	// Ledger grew beyond empty, and the chain verifies.
	vr, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !vr.Pass {
		t.Fatalf("chain not intact: %s (line %d)", vr.Message, vr.FirstBrokenLine)
	}
	if vr.RecordCount != 2 {
		t.Errorf("RecordCount = %d, want 2", vr.RecordCount)
	}

	// Re-emission is idempotent: same edges, no new rows, chain still intact.
	for _, id := range ids {
		edge := buildBeadCommitEdge(id, commitSHA)
		edge.TS = "2026-06-15T00:00:00Z"
		res, err := store.Append(edge)
		if err != nil {
			t.Fatalf("re-append %s: %v", id, err)
		}
		if !res.Skipped {
			t.Errorf("re-emit of %s should be an idempotent no-op", id)
		}
	}
	vr2, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("verify after re-emit: %v", err)
	}
	if vr2.RecordCount != 2 {
		t.Errorf("idempotent re-emit changed RecordCount to %d, want 2", vr2.RecordCount)
	}
}

// TestProvenanceEmitLandedCommand_Registered asserts the `ao provenance
// emit-landed` command is wired into the cobra tree under provenance with the
// dry-run/commit/range flags. This is the command-surface coverage for the new
// leaf (the logic is exercised by the unit + L2 tests below).
func TestProvenanceEmitLandedCommand_Registered(t *testing.T) {
	if provenanceEmitLandedCmd.Use != "emit-landed" {
		t.Fatalf("Use = %q, want emit-landed", provenanceEmitLandedCmd.Use)
	}
	var found bool
	for _, c := range provenanceCmd.Commands() {
		if c.Use == "emit-landed" {
			found = true
		}
	}
	if !found {
		t.Error("emit-landed is not registered under `ao provenance`")
	}
	for _, f := range []string{"dry-run", "commit", "range", "trunk-ref"} {
		if provenanceEmitLandedCmd.Flags().Lookup(f) == nil {
			t.Errorf("missing flag --%s on `ao provenance emit-landed`", f)
		}
	}
}

// TestFilterCommitsOnTrunk_EmptyRefPassthrough (age-0tn): blank trunk-ref is a no-op.
func TestFilterCommitsOnTrunk_EmptyRefPassthrough(t *testing.T) {
	in := []landedCommit{{sha: "abc", msg: "feat: x (ag-test)"}}
	got, err := filterCommitsOnTrunk(in, "")
	if err != nil {
		t.Fatalf("filterCommitsOnTrunk: %v", err)
	}
	if len(got) != 1 || got[0].sha != "abc" {
		t.Fatalf("got %v, want unchanged input", got)
	}
}

// TestExtractLandedBeadIDs covers the real commit-message conventions the
// emitter reads to know which beads a commit closes (ag-62jrm, milestone 1).
// The mapping is honest: in the push-to-main workflow a bead "closes" when its
// arc lands on main, and the landing commit cites it via a trailer or the PR
// title convention.
func TestExtractLandedBeadIDs(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want []string
	}{
		{
			name: "Closes-scenario trailer",
			msg:  "feat(x): do thing\n\nCloses-scenario: ag-62jrm#emit-edge\nBounded-context: BC1-corpus\n",
			want: []string{"ag-62jrm"},
		},
		{
			name: "PR-title parens convention",
			msg:  "feat(spawn): born-into-coordination gateway (ag-tixgy #reserve)",
			want: []string{"ag-tixgy"},
		},
		{
			name: "bare parens id in title",
			msg:  "fix(ci): green the trunk (ag-ws4cl)",
			want: []string{"ag-ws4cl"},
		},
		{
			name: "Closes: trailer plain with dotted child",
			msg:  "chore: tidy\n\nCloses: ag-y8b.5\n",
			want: []string{"ag-y8b.5"},
		},
		{
			name: "multiple distinct ids dedup + order",
			msg:  "feat: two arcs (ag-aaa #s)\n\nCloses-scenario: ag-bbb#t\nCloses-scenario: ag-aaa#u\n",
			want: []string{"ag-aaa", "ag-bbb"},
		},
		{
			name: "no bead reference",
			msg:  "docs: typo fix in README",
			want: nil,
		},
		{
			// Regression (smoke-caught): the conventional-commit scope
			// `type(scope):` must NOT be read as a bead id. Only the trailing
			// `(id)` ref counts.
			name: "conventional-commit scope is not a bead id",
			msg:  "feat(cc-hooks): coordination guard (ag-real)",
			want: []string{"ag-real"},
		},
		{
			// ag-62jrm regression: subject comma-list yields BOTH ids (the real
			// fix target — 1b3771563 should have emitted ag-7hdg0 + ag-56cru).
			name: "subject comma-list yields both ids",
			msg:  "feat: dual arc (ag-7hdg0, ag-56cru #slug)\n\nbody text\n",
			want: []string{"ag-7hdg0", "ag-56cru"},
		},
		{
			// ag-62jrm regression: BODY prose parens must NEVER become an edge —
			// this is the exact false-edge bug (bead_id="council-ratified").
			name: "body-prose parens ignored (the false-edge bug)",
			msg:  "feat: real work (ag-real)\n\nthis was a council-ratified (council-ratified) change; see (pre-mortem cleanup)\n",
			want: []string{"ag-real"},
		},
		{
			// Foreign tracker prefixes are not AgentOps landings — the agentops
			// sensor emits ag- only (deliberate namespace, not ledger-derived).
			name: "foreign prefix not emitted",
			msg:  "chore: cross-repo note\n\nCloses: soc-y8b.5\n",
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractLandedBeadIDs(c.msg)
			if len(got) != len(c.want) {
				t.Fatalf("extractLandedBeadIDs() = %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("[%d] = %q, want %q (full: %v)", i, got[i], c.want[i], got)
				}
			}
		})
	}
}

// TestExtractLandedBeadIDs_NoFalsePositives guards against the emitter
// inventing edges from prose that merely resembles an id. A real id is
// prefix-dash-token; ordinary hyphenated words must not match.
func TestExtractLandedBeadIDs_NoFalsePositives(t *testing.T) {
	for _, msg := range []string{
		"refactor: rename well-known cross-context helper; see context-map and follow-up",
		// Body parens of hyphenated prose — the exact false-edge shape.
		"docs: notes\n\nthis is a council-ratified (council-ratified) decision\nand a (pre-mortem cleanup) follow-up\n",
		// A non-ag bead id mentioned in body prose parens is still not an edge.
		"chore: mention (soc-y8b.5) in passing in the body\n",
	} {
		if got := extractLandedBeadIDs(msg); len(got) != 0 {
			t.Errorf("expected no ids from prose %q, got %v", msg, got)
		}
	}
}

// TestBuildBeadCommitEdge enforces the slice-1 edge shape: bead --wasGeneratedBy-->
// commit, trust_tier inferred (deterministically observed, never self-graded).
func TestBuildBeadCommitEdge(t *testing.T) {
	e := buildBeadCommitEdge("ag-62jrm", "abc1234def")
	if e.FromID != "ag-62jrm" || e.FromType != "bead" {
		t.Errorf("from = %s/%s, want ag-62jrm/bead", e.FromID, e.FromType)
	}
	if e.ToID != "abc1234def" || e.ToType != "commit" {
		t.Errorf("to = %s/%s, want abc1234def/commit", e.ToID, e.ToType)
	}
	if e.Relation != "wasGeneratedBy" {
		t.Errorf("relation = %s, want wasGeneratedBy", e.Relation)
	}
	if e.TrustTier != "inferred" {
		t.Errorf("trust_tier = %s, want inferred (deterministic observation, not a self-grade)", e.TrustTier)
	}
	if !strings.Contains(e.EvidenceRef, "abc1234def") {
		t.Errorf("evidence should reference the commit, got %q", e.EvidenceRef)
	}
	// Mesh join keys (ag-5qltf): bead_id + merge_sha are stamped first-class,
	// denormalizing from_id/to_id so the yield↔provenance join resolves on
	// (bead_id, merge_sha) without decoding edge endpoints.
	if e.BeadID != "ag-62jrm" {
		t.Errorf("bead_id = %q, want ag-62jrm (universal mesh join key)", e.BeadID)
	}
	if e.MergeSHA != "abc1234def" {
		t.Errorf("merge_sha = %q, want abc1234def (bead→commit anchor)", e.MergeSHA)
	}
	if e.BeadID != e.FromID || e.MergeSHA != e.ToID {
		t.Errorf("join keys must denormalize from_id/to_id: bead_id=%q from_id=%q, merge_sha=%q to_id=%q", e.BeadID, e.FromID, e.MergeSHA, e.ToID)
	}
}
