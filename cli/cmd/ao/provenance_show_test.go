// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// setProvShowJSON sets the --json cobra global and restores it on cleanup
// (test-isolation rule: package-global flag vars must not leak across tests).
func setProvShowJSON(t *testing.T, v bool) {
	t.Helper()
	prev := provShowJSON
	provShowJSON = v
	t.Cleanup(func() { provShowJSON = prev })
}

// Fixture shas: full 40-char commit OIDs with distinct 7-char prefixes.
const (
	showSHAOneVerdict   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	showSHAMultiVerdict = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	showSHAUnreviewed   = "cccccccccccccccccccccccccccccccccccccccc"
)

// seedShowLedger builds the fixture ledger THROUGH the production writer:
// edges come from the production constructors (buildBeadCommitEdge /
// buildVerdictCommitEdge — the same code emit-landed and emit-verdict run)
// and are sealed+appended by provenancegraph.Store.Append, then read back by
// Store.Read inside the command under test (fixture-fidelity rule).
//
// Shape (6 records):
//  1. ag-show.1 --wasGeneratedBy--> sha-a
//  2. verdict ag-show.1@aaaaaaa --wasDerivedFrom--> sha-a (CONFIRMED)
//  3. ag-show.2 --wasGeneratedBy--> sha-b
//  4. verdict ag-show.2@bbbbbbb --wasDerivedFrom--> sha-b (REFUTED)
//  5. verdict ag-show.2@bbbbbbb --wasDerivedFrom--> sha-b (CONFIRMED)
//  6. ag-show.3 --wasGeneratedBy--> sha-c (landed, never reviewed)
func seedShowLedger(t *testing.T) {
	t.Helper()
	store := provenancegraph.NewStore(resolveLedgerPath())

	edges := []provenancegraph.Edge{
		buildBeadCommitEdge("ag-show.1", showSHAOneVerdict),
		buildVerdictCommitEdge(pawlVerdict{
			BeadID: "ag-show.1", HeadSHA: showSHAOneVerdict, Disposition: "CONFIRMED"}),
		buildBeadCommitEdge("ag-show.2", showSHAMultiVerdict),
		buildVerdictCommitEdge(pawlVerdict{
			BeadID: "ag-show.2", HeadSHA: showSHAMultiVerdict, Disposition: "REFUTED"}),
		buildVerdictCommitEdge(pawlVerdict{
			BeadID: "ag-show.2", HeadSHA: showSHAMultiVerdict, Disposition: "CONFIRMED"}),
		buildBeadCommitEdge("ag-show.3", showSHAUnreviewed),
	}
	for i, e := range edges {
		e.TS = "2026-06-30T00:00:0" + string(rune('0'+i)) + "Z"
		res, err := store.Append(e)
		if err != nil {
			t.Fatalf("seed edge %d: %v", i, err)
		}
		if res.Skipped {
			t.Fatalf("seed edge %d unexpectedly deduped", i)
		}
	}
}

// TestProvenanceShow_HumanLineage covers the human rendering across the
// resolution modes: full sha, >=7-char prefix, bead-id, multi-verdict, and
// the landed-but-unreviewed honest line.
func TestProvenanceShow_HumanLineage(t *testing.T) {
	cases := []struct {
		name        string
		query       string
		wantSubstrs []string
		notSubstrs  []string
	}{
		{
			name:  "full sha with one verdict",
			query: showSHAOneVerdict,
			wantSubstrs: []string{
				"commit " + showSHAOneVerdict,
				"bead    ag-show.1  [inferred]",
				"verdict ag-show.1@aaaaaaa  disposition=CONFIRMED",
				"(record 1/6)",
				"(record 2/6)",
				"evidence: pawl-verdict ag-show.1 disposition=CONFIRMED",
			},
			notSubstrs: []string{"ag-show.2", "no verdict recorded"},
		},
		{
			name:  "seven-char prefix resolves the same commit",
			query: showSHAOneVerdict[:7],
			wantSubstrs: []string{
				"commit " + showSHAOneVerdict,
				"bead    ag-show.1",
			},
			notSubstrs: []string{"ag-show.2", "ag-show.3"},
		},
		{
			name:  "sha with multiple verdicts renders all of them",
			query: showSHAMultiVerdict,
			wantSubstrs: []string{
				"commit " + showSHAMultiVerdict,
				"bead    ag-show.2",
				"disposition=REFUTED",
				"disposition=CONFIRMED",
				"(record 4/6)",
				"(record 5/6)",
			},
			notSubstrs: []string{"no verdict recorded"},
		},
		{
			name:  "bead id resolves to its landed commit",
			query: "ag-show.2",
			wantSubstrs: []string{
				"commit " + showSHAMultiVerdict,
				"disposition=REFUTED",
				"disposition=CONFIRMED",
			},
			notSubstrs: []string{"commit " + showSHAOneVerdict},
		},
		{
			name:  "landed but unreviewed renders honestly",
			query: showSHAUnreviewed,
			wantSubstrs: []string{
				"commit " + showSHAUnreviewed,
				"bead    ag-show.3",
				"no verdict recorded — landed but unreviewed",
				"(record 6/6)",
			},
			notSubstrs: []string{"disposition="},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chdirRepoFixture(t)
			seedShowLedger(t)
			setProvShowJSON(t, false)

			c, out := provTestCmd()
			if err := runProvenanceShow(c, []string{tc.query}); err != nil {
				t.Fatalf("show %q: %v", tc.query, err)
			}
			got := out.String()
			for _, want := range tc.wantSubstrs {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q:\n%s", want, got)
				}
			}
			for _, not := range tc.notSubstrs {
				if strings.Contains(got, not) {
					t.Errorf("output must not contain %q:\n%s", not, got)
				}
			}
		})
	}
}

// TestProvenanceShow_UnknownIDErrors: an id with no ledger match must return
// a non-nil (non-zero exit) corrective error naming the search commands.
func TestProvenanceShow_UnknownIDErrors(t *testing.T) {
	chdirRepoFixture(t)
	seedShowLedger(t)
	setProvShowJSON(t, false)

	for _, query := range []string{"ag-nope", "deadbeefdeadbee"} {
		c, _ := provTestCmd()
		err := runProvenanceShow(c, []string{query})
		if err == nil {
			t.Fatalf("show %q: want error, got nil", query)
		}
		msg := err.Error()
		if !strings.Contains(msg, query) {
			t.Errorf("error %q does not name the query %q", msg, query)
		}
		if !strings.Contains(msg, "ao provenance list") || !strings.Contains(msg, "ao provenance position") {
			t.Errorf("error %q does not name how to search (list/position)", msg)
		}
	}
}

// TestProvenanceShow_ShortHexPrefixErrors: a hex prefix under 7 chars must
// not silently miss — the corrective error names the minimum length.
func TestProvenanceShow_ShortHexPrefixErrors(t *testing.T) {
	chdirRepoFixture(t)
	seedShowLedger(t)
	setProvShowJSON(t, false)

	c, _ := provTestCmd()
	err := runProvenanceShow(c, []string{showSHAOneVerdict[:6]})
	if err == nil {
		t.Fatal("show 6-char prefix: want error, got nil")
	}
	if !strings.Contains(err.Error(), "at least 7") {
		t.Errorf("error %q does not name the 7-char minimum", err.Error())
	}
}

// TestProvenanceShow_JSONShape: --json emits the exact showReport contract
// (stdout-as-data), verdict-absent included as an empty array, never null.
func TestProvenanceShow_JSONShape(t *testing.T) {
	chdirRepoFixture(t)
	seedShowLedger(t)
	setProvShowJSON(t, true)

	c, out := provTestCmd()
	if err := runProvenanceShow(c, []string{showSHAMultiVerdict}); err != nil {
		t.Fatalf("show --json: %v", err)
	}

	var r showReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("--json output not a showReport: %v\n%s", err, out.String())
	}
	if r.Query != showSHAMultiVerdict {
		t.Fatalf("query = %q, want %q", r.Query, showSHAMultiVerdict)
	}
	if r.TotalRecords != 6 {
		t.Fatalf("total_records = %d, want 6", r.TotalRecords)
	}
	if len(r.Commits) != 1 {
		t.Fatalf("commits len = %d, want 1", len(r.Commits))
	}
	lin := r.Commits[0]
	if lin.CommitSHA != showSHAMultiVerdict {
		t.Fatalf("commit_sha = %q, want %q", lin.CommitSHA, showSHAMultiVerdict)
	}
	if len(lin.Beads) != 1 {
		t.Fatalf("beads len = %d, want 1", len(lin.Beads))
	}
	bead := lin.Beads[0]
	if bead.BeadID != "ag-show.2" || bead.TrustTier != "inferred" || bead.Record != 3 {
		t.Fatalf("bead entry = %+v, want ag-show.2/inferred/record 3", bead)
	}
	if bead.Timestamp != "2026-06-30T00:00:02Z" {
		t.Fatalf("bead ts = %q, want 2026-06-30T00:00:02Z", bead.Timestamp)
	}
	if len(lin.Verdicts) != 2 {
		t.Fatalf("verdicts len = %d, want 2", len(lin.Verdicts))
	}
	wantVerdicts := []showVerdictEntry{
		{
			VerdictID:   "ag-show.2@bbbbbbb",
			Disposition: "REFUTED",
			EvidenceRef: "pawl-verdict ag-show.2 disposition=REFUTED",
			TrustTier:   "inferred",
			Timestamp:   "2026-06-30T00:00:03Z",
			Record:      4,
		},
		{
			VerdictID:   "ag-show.2@bbbbbbb",
			Disposition: "CONFIRMED",
			EvidenceRef: "pawl-verdict ag-show.2 disposition=CONFIRMED",
			TrustTier:   "inferred",
			Timestamp:   "2026-06-30T00:00:04Z",
			Record:      5,
		},
	}
	for i, want := range wantVerdicts {
		if lin.Verdicts[i] != want {
			t.Errorf("verdict[%d] = %+v, want %+v", i, lin.Verdicts[i], want)
		}
	}

	// Verdict-absent commit: verdicts is a concrete empty array, never null.
	setProvShowJSON(t, true)
	c2, out2 := provTestCmd()
	if err := runProvenanceShow(c2, []string{showSHAUnreviewed}); err != nil {
		t.Fatalf("show --json unreviewed: %v", err)
	}
	if !strings.Contains(out2.String(), `"verdicts": []`) {
		t.Fatalf("unreviewed --json must carry \"verdicts\": [] (never null):\n%s", out2.String())
	}
	var r2 showReport
	if err := json.Unmarshal(out2.Bytes(), &r2); err != nil {
		t.Fatalf("unreviewed --json parse: %v", err)
	}
	if len(r2.Commits) != 1 || len(r2.Commits[0].Verdicts) != 0 || len(r2.Commits[0].Beads) != 1 {
		t.Fatalf("unreviewed lineage = %+v, want 1 bead, 0 verdicts", r2.Commits)
	}
}

// TestBuildShowReport_BeadLandedTwice: a bead cited by two landing commits
// resolves to BOTH commits, each carrying its own lineage.
func TestBuildShowReport_BeadLandedTwice(t *testing.T) {
	shaA := strings.Repeat("d", 40)
	shaB := strings.Repeat("e", 40)
	e1 := buildBeadCommitEdge("ag-twice", shaA)
	e1.TS = "2026-06-30T01:00:00Z"
	e2 := buildBeadCommitEdge("ag-twice", shaB)
	e2.TS = "2026-06-30T02:00:00Z"

	r, err := buildShowReport([]provenancegraph.Edge{e1, e2}, "ag-twice")
	if err != nil {
		t.Fatalf("buildShowReport: %v", err)
	}
	if len(r.Commits) != 2 {
		t.Fatalf("commits len = %d, want 2", len(r.Commits))
	}
	if r.Commits[0].CommitSHA != shaA || r.Commits[1].CommitSHA != shaB {
		t.Fatalf("commit order = %q, %q; want ledger order %q, %q",
			r.Commits[0].CommitSHA, r.Commits[1].CommitSHA, shaA, shaB)
	}
	for i, c := range r.Commits {
		if len(c.Beads) != 1 || c.Beads[0].BeadID != "ag-twice" {
			t.Fatalf("commit %d beads = %+v, want single ag-twice", i, c.Beads)
		}
	}
}

// TestParseDisposition: the evidence_ref convention parser is exact.
func TestParseDisposition(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"pawl-verdict ag-x disposition=CONFIRMED", "CONFIRMED"},
		{"pawl-verdict ag-x disposition=REFUTED", "REFUTED"},
		{"commit abc123", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := parseDisposition(tc.in); got != tc.want {
			t.Errorf("parseDisposition(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
