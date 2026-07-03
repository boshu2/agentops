package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// fixedNow is the deterministic clock the fixture tests anchor the trailing
// window to. All fixture verdict timestamps fall in [now-30d, now].
var fixedNow = time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)

// Distinct 40-char hex commit shas with distinct 7-char prefixes, so from_id
// (<bead>@<sha7>) and distinct-commit counts never collide by accident.
const (
	fxSHA1 = "1111111111111111111111111111111111111111"
	fxSHA2 = "2222222222222222222222222222222222222222"
	fxSHA3 = "3333333333333333333333333333333333333333"
	fxSHA4 = "4444444444444444444444444444444444444444"
	fxSHA5 = "5555555555555555555555555555555555555555"
	fxSHA6 = "6666666666666666666666666666666666666666"
)

// mkVerdictEdge builds a verdict→commit edge the way production does
// (buildVerdictCommitEdge): from_id=<bead>@<sha7>, evidence_ref carrying the
// disposition. When v11 is set it also fills the v1.1 enrichment fields. This is
// a real, schema-valid Edge fed through the production writer (Store.Append), so
// the fixture round-trips the exact on-disk shape (fixture-fidelity rule).
func mkVerdictEdge(bead, sha, disp, ts string, v11 bool, fam string, degraded bool, rounds int, dur float64) provenancegraph.Edge {
	e := provenancegraph.Edge{
		FromID:      bead + "@" + sha[:7],
		FromType:    "verdict",
		ToID:        sha,
		ToType:      "commit",
		Relation:    "wasDerivedFrom",
		TrustTier:   "inferred",
		EvidenceRef: "pawl-verdict " + bead + " disposition=" + disp,
		TS:          ts,
	}
	if v11 {
		e.BeadID = bead
		e.ReviewerFamily = fam
		e.Degraded = degraded
		e.Rounds = rounds
		e.DurationS = dur
	}
	return e
}

// writeStatsFixture appends every edge through the PRODUCTION writer
// (provenancegraph.Store.Append) to a temp ledger, then reads it back with the
// production reader. Returns the round-tripped edges — the exact on-disk shape a
// real ledger has. Never hand-builds JSON.
func writeStatsFixture(t *testing.T, edges []provenancegraph.Edge) ([]provenancegraph.Edge, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	store := provenancegraph.NewStore(path)
	for i, e := range edges {
		res, err := store.Append(e)
		if err != nil {
			t.Fatalf("append fixture edge %d: %v", i, err)
		}
		if res.Skipped {
			t.Fatalf("fixture edge %d was unexpectedly a dedupe no-op: %+v", i, e)
		}
	}
	got, err := store.Read()
	if err != nil {
		t.Fatalf("read back fixture ledger: %v", err)
	}
	return got, path
}

// mixedFixture is the canonical fixture: 4 v1.1 verdict edges + 2 v1 verdict
// edges + 1 non-verdict edge (7 records, 6 verdict events). It exercises every
// metric, the v1/v1.1 split, and the re-review proxy.
func mixedFixture(t *testing.T) ([]provenancegraph.Edge, string) {
	t.Helper()
	edges := []provenancegraph.Edge{
		mkVerdictEdge("age-aaa", fxSHA1, "CONFIRMED", "2026-06-20T00:00:00Z", true, "claude+gpt", false, 1, 10),
		mkVerdictEdge("age-bbb", fxSHA2, "CONFIRMED", "2026-06-25T00:00:00Z", true, "claude+gpt", true, 2, 20),
		mkVerdictEdge("age-ccc", fxSHA3, "REFUTED", "2026-06-25T00:00:00Z", true, "gpt", false, 3, 30),
		// second CONFIRMED for age-aaa (different commit) => a re-review.
		mkVerdictEdge("age-aaa", fxSHA4, "CONFIRMED", "2026-07-01T00:00:00Z", true, "gpt", false, 1, 40),
		// v1-shaped verdicts (no enrichment fields): age-ddd re-reviewed twice.
		mkVerdictEdge("age-ddd", fxSHA5, "CONFIRMED", "2026-06-18T00:00:00Z", false, "", false, 0, 0),
		mkVerdictEdge("age-ddd", fxSHA6, "CONFIRMED", "2026-06-19T00:00:00Z", false, "", false, 0, 0),
		// non-verdict edge — counted in ledger_records, ignored by every verdict metric.
		{
			FromID: "age-eee", FromType: "bead", ToID: fxSHA1, ToType: "commit",
			Relation: "wasGeneratedBy", TrustTier: "authored", BeadID: "age-eee",
			TS: "2026-06-20T00:00:00Z",
		},
	}
	return writeStatsFixture(t, edges)
}

// TestComputeVerifyStats_MixedLedger asserts every derived number against a
// fixture whose values are independently hand-computable. This is the
// correctness proof (the golden test below only pins the exact text format).
func TestComputeVerifyStats_MixedLedger(t *testing.T) {
	edges, _ := mixedFixture(t)
	r := computeVerifyStats(edges, 30, fixedNow)

	if r.LedgerRecords != 7 {
		t.Errorf("LedgerRecords = %d, want 7", r.LedgerRecords)
	}
	if r.VerdictEvents != 6 {
		t.Errorf("VerdictEvents = %d, want 6", r.VerdictEvents)
	}
	if r.DistinctBeadsReviewed != 4 {
		t.Errorf("DistinctBeadsReviewed = %d, want 4 (aaa,bbb,ccc,ddd)", r.DistinctBeadsReviewed)
	}
	if r.Dispositions["CONFIRMED"] != 5 || r.Dispositions["REFUTED"] != 1 {
		t.Errorf("Dispositions = %v, want CONFIRMED=5 REFUTED=1", r.Dispositions)
	}
	if math.Abs(r.RefuteRate-1.0/6.0) > 1e-9 {
		t.Errorf("RefuteRate = %v, want %v", r.RefuteRate, 1.0/6.0)
	}

	// Rounds distribution (v1.1): {1:2, 2:1, 3:1}, 4 records carry rounds.
	if !r.Rounds.Available {
		t.Fatal("Rounds.Available = false, want true")
	}
	if r.Rounds.RecordsWithRounds != 4 {
		t.Errorf("RecordsWithRounds = %d, want 4", r.Rounds.RecordsWithRounds)
	}
	wantRounds := map[int]int{1: 2, 2: 1, 3: 1}
	if len(r.Rounds.Distribution) != len(wantRounds) {
		t.Errorf("rounds distribution = %+v, want %v", r.Rounds.Distribution, wantRounds)
	}
	for _, b := range r.Rounds.Distribution {
		if wantRounds[b.Rounds] != b.Count {
			t.Errorf("rounds[%d] = %d, want %d", b.Rounds, b.Count, wantRounds[b.Rounds])
		}
	}
	// Distribution must be sorted ascending by rounds.
	for i := 1; i < len(r.Rounds.Distribution); i++ {
		if r.Rounds.Distribution[i-1].Rounds >= r.Rounds.Distribution[i].Rounds {
			t.Errorf("rounds distribution not sorted ascending: %+v", r.Rounds.Distribution)
		}
	}

	// Duration (v1.1): [10,20,30,40] => median 25, p90 40 (nearest-rank), n=4.
	if !r.Duration.Available {
		t.Fatal("Duration.Available = false, want true")
	}
	if r.Duration.Count != 4 {
		t.Errorf("Duration.Count = %d, want 4", r.Duration.Count)
	}
	if r.Duration.MedianS != 25 {
		t.Errorf("Duration.MedianS = %v, want 25", r.Duration.MedianS)
	}
	if r.Duration.P90S != 40 {
		t.Errorf("Duration.P90S = %v, want 40", r.Duration.P90S)
	}

	// Degraded by family: claude+gpt = 1/2 (0.5), gpt = 0/2 (0.0).
	if !r.Degraded.Available {
		t.Fatal("Degraded.Available = false, want true")
	}
	cg := r.Degraded.ByReviewerFamily["claude+gpt"]
	if cg.Total != 2 || cg.Degraded != 1 || cg.Share != 0.5 {
		t.Errorf("claude+gpt = %+v, want total=2 degraded=1 share=0.5", cg)
	}
	gpt := r.Degraded.ByReviewerFamily["gpt"]
	if gpt.Total != 2 || gpt.Degraded != 0 || gpt.Share != 0 {
		t.Errorf("gpt = %+v, want total=2 degraded=0 share=0", gpt)
	}

	// Re-review proxy: age-aaa (2 CONFIRMED / 2 commits) + age-ddd (2/2).
	if r.ReReview.RereviewedBeads != 2 {
		t.Errorf("RereviewedBeads = %d, want 2", r.ReReview.RereviewedBeads)
	}
	if r.ReReview.RedundantConfirmations != 2 {
		t.Errorf("RedundantConfirmations = %d, want 2", r.ReReview.RedundantConfirmations)
	}
	if len(r.ReReview.Beads) != 2 || r.ReReview.Beads[0].Bead != "age-aaa" || r.ReReview.Beads[1].Bead != "age-ddd" {
		t.Errorf("re-review beads = %+v, want [age-aaa, age-ddd] sorted", r.ReReview.Beads)
	}
	for _, b := range r.ReReview.Beads {
		if b.Confirmed != 2 || b.DistinctCommits != 2 {
			t.Errorf("re-review bead %s = %+v, want confirmed=2 distinct_commits=2", b.Bead, b)
		}
	}

	// Trend: 5 active days in window, with 06-25 carrying the REFUTE.
	if r.Trend.Days != 30 {
		t.Errorf("Trend.Days = %d, want 30", r.Trend.Days)
	}
	if len(r.Trend.ByDay) != 5 {
		t.Fatalf("Trend.ByDay has %d days, want 5: %+v", len(r.Trend.ByDay), r.Trend.ByDay)
	}
	byDate := map[string]trendDay{}
	for _, d := range r.Trend.ByDay {
		byDate[d.Date] = d
	}
	if d := byDate["2026-06-25"]; d.Verdicts != 2 || d.Confirmed != 1 || d.Refuted != 1 {
		t.Errorf("2026-06-25 = %+v, want 2 verdicts / 1 confirmed / 1 refuted", d)
	}
	// Days must be sorted ascending.
	for i := 1; i < len(r.Trend.ByDay); i++ {
		if r.Trend.ByDay[i-1].Date >= r.Trend.ByDay[i].Date {
			t.Errorf("trend days not sorted ascending: %+v", r.Trend.ByDay)
		}
	}
}

// TestComputeVerifyStats_V1OnlyDegradesGracefully proves the acceptance
// criterion: a pre-v1.1 ledger reports the enrichment metrics as UNAVAILABLE
// with a reason (never a misleading zero), while the v1-derivable metrics
// (dispositions, re-review) still work.
func TestComputeVerifyStats_V1OnlyDegradesGracefully(t *testing.T) {
	edges, _ := writeStatsFixture(t, []provenancegraph.Edge{
		mkVerdictEdge("age-x", fxSHA1, "CONFIRMED", "2026-06-18T00:00:00Z", false, "", false, 0, 0),
		mkVerdictEdge("age-y", fxSHA2, "REFUTED", "2026-06-19T00:00:00Z", false, "", false, 0, 0),
	})
	r := computeVerifyStats(edges, 30, fixedNow)

	if r.VerdictEvents != 2 {
		t.Errorf("VerdictEvents = %d, want 2", r.VerdictEvents)
	}
	if r.Dispositions["CONFIRMED"] != 1 || r.Dispositions["REFUTED"] != 1 {
		t.Errorf("Dispositions = %v, want CONFIRMED=1 REFUTED=1", r.Dispositions)
	}

	// The three v1.1 metrics degrade gracefully.
	if r.Rounds.Available {
		t.Error("Rounds.Available = true on a v1-only ledger, want false")
	}
	if !strings.Contains(r.Rounds.Reason, "rounds") {
		t.Errorf("Rounds.Reason = %q, want it to name the rounds field", r.Rounds.Reason)
	}
	if r.Duration.Available || !strings.Contains(r.Duration.Reason, "duration_s") {
		t.Errorf("Duration should be unavailable with a duration_s reason, got %+v", r.Duration)
	}
	if r.Degraded.Available || !strings.Contains(r.Degraded.Reason, "reviewer_family") {
		t.Errorf("Degraded should be unavailable with a reviewer_family reason, got %+v", r.Degraded)
	}

	// The unavailable sub-objects must NOT leak zero-valued detail fields into
	// JSON (omitempty) — no misleading zeros.
	if r.Duration.Count != 0 || r.Duration.MedianS != 0 || r.Duration.P90S != 0 {
		t.Errorf("unavailable duration must carry no numbers, got %+v", r.Duration)
	}
	if r.Rounds.RecordsWithRounds != 0 || r.Rounds.Distribution != nil {
		t.Errorf("unavailable rounds must carry no distribution, got %+v", r.Rounds)
	}
}

// TestRenderVerifyStatsText_Golden pins the exact human text output for the
// mixed fixture, via the package captureStdout convention. Values are proven
// independently in TestComputeVerifyStats_MixedLedger; this locks the format.
func TestRenderVerifyStatsText_Golden(t *testing.T) {
	edges, _ := mixedFixture(t)
	r := computeVerifyStats(edges, 30, fixedNow)

	out, err := captureStdout(t, func() error {
		renderVerifyStatsText(os.Stdout, r)
		return nil
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	const golden = `Verified-done cost — provenance ledger stats (as of 2026-07-02T00:00:00Z)
ledger: docs/provenance/ledger.jsonl
records: 7   verdict events: 6   distinct beads reviewed: 4

Dispositions:
  CONFIRMED    5
  REFUTED      1
  refute rate: 16.7%

Rounds per verdict:
  1 round(s): 2
  2 round(s): 1
  3 round(s): 1
  (4 verdict records carry rounds)

Duration per verdict (seconds):
  median: 25.0   p90: 40.0   (n=4)

Degraded share by reviewer family:
  claude+gpt       1/2 degraded (50.0%)
  gpt              0/2 degraded (0.0%)

Re-review (rebase-waste proxy — beads with >1 CONFIRMED verdict):
  re-reviewed beads: 2   redundant confirmations: 2
  age-aaa                    2 CONFIRMED across 2 commit(s)
  age-ddd                    2 CONFIRMED across 2 commit(s)

Trend (verdicts in the trailing 30 days, since 2026-06-02T00:00:00Z):
  2026-06-18  1 verdicts (1 confirmed, 0 refuted)
  2026-06-19  1 verdicts (1 confirmed, 0 refuted)
  2026-06-20  1 verdicts (1 confirmed, 0 refuted)
  2026-06-25  2 verdicts (1 confirmed, 1 refuted)
  2026-07-01  1 verdicts (1 confirmed, 0 refuted)
`
	if out != golden {
		t.Errorf("text output mismatch\n--- got ---\n%s\n--- want ---\n%s", out, golden)
	}
}

// TestVerifyStatsCommand_RegisteredAndRoutable proves the subcommand is wired
// under `ao verify` AND that cobra routes `verify stats` to it despite the
// parent verifyCmd having DisableFlagParsing=true (the real risk of nesting a
// subcommand under a flag-forwarding leaf).
func TestVerifyStatsCommand_RegisteredAndRoutable(t *testing.T) {
	var found bool
	for _, c := range verifyCmd.Commands() {
		if c.Name() == "stats" {
			found = true
		}
	}
	if !found {
		t.Fatal("`stats` is not registered under `ao verify`")
	}

	cmd, _, err := rootCmd.Find([]string{"verify", "stats"})
	if err != nil {
		t.Fatalf("rootCmd.Find([verify stats]): %v", err)
	}
	if cmd != verifyStatsCmd {
		t.Fatalf("`ao verify stats` routed to %q, not the stats subcommand", cmd.Name())
	}

	for _, f := range []string{"json", "days", "ledger"} {
		if verifyStatsCmd.Flags().Lookup(f) == nil {
			t.Errorf("missing flag --%s on `ao verify stats`", f)
		}
	}
}

// TestRunVerifyStats_ThroughCommand exercises the full RunE path against a
// fixture ledger via --ledger, with the clock seam pinned (restored via
// t.Cleanup per the test-isolation rule), and confirms the report renders.
func TestRunVerifyStats_ThroughCommand(t *testing.T) {
	_, path := mixedFixture(t)

	prevNow := verifyStatsNow
	verifyStatsNow = func() time.Time { return fixedNow }
	t.Cleanup(func() { verifyStatsNow = prevNow })

	prevLedger, prevJSON, prevDays := verifyStatsLedger, verifyStatsJSON, verifyStatsDays
	verifyStatsLedger, verifyStatsJSON, verifyStatsDays = path, false, 30
	t.Cleanup(func() {
		verifyStatsLedger, verifyStatsJSON, verifyStatsDays = prevLedger, prevJSON, prevDays
	})

	out, err := captureStdout(t, func() error {
		return runVerifyStats(verifyStatsCmd, nil)
	})
	if err != nil {
		t.Fatalf("runVerifyStats: %v", err)
	}
	// The override path relabels the ledger to the actual file read.
	if !strings.Contains(out, "ledger: "+path) {
		t.Errorf("output should name the overridden ledger path %q; got:\n%s", path, out)
	}
	for _, want := range []string{"verdict events: 6", "CONFIRMED    5", "redundant confirmations: 2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}
}

// --- Builder unit tests (the computeVerifyStats gocyclo split, age-hl9q) ---
// Each builder owns one report section; these pin the exact section shapes the
// split must preserve: availability flags, unavailability reasons, sort orders,
// and the arithmetic (median/p90, degraded share, redundant confirmations).

func TestBuildRoundsStats_SortedDistributionAndEmptyReason(t *testing.T) {
	s := buildRoundsStats(map[int]int{3: 1, 1: 2}, 3)
	if !s.Available {
		t.Fatalf("Available = false, want true")
	}
	if s.RecordsWithRounds != 3 {
		t.Fatalf("RecordsWithRounds = %d, want 3", s.RecordsWithRounds)
	}
	want := []roundBucket{{Rounds: 1, Count: 2}, {Rounds: 3, Count: 1}}
	if len(s.Distribution) != len(want) {
		t.Fatalf("Distribution len = %d, want %d", len(s.Distribution), len(want))
	}
	for i, b := range want {
		if s.Distribution[i] != b {
			t.Fatalf("Distribution[%d] = %+v, want %+v", i, s.Distribution[i], b)
		}
	}

	empty := buildRoundsStats(map[int]int{}, 0)
	if empty.Available {
		t.Fatalf("empty: Available = true, want false")
	}
	if !strings.Contains(empty.Reason, "rounds") {
		t.Fatalf("empty: Reason = %q, want mention of rounds", empty.Reason)
	}
}

func TestBuildDurationStats_ExactMedianAndNearestRankP90(t *testing.T) {
	s := buildDurationStats([]float64{5, 1, 3, 2, 4})
	if !s.Available || s.Count != 5 {
		t.Fatalf("Available/Count = %v/%d, want true/5", s.Available, s.Count)
	}
	if s.MedianS != 3 {
		t.Fatalf("MedianS = %v, want 3", s.MedianS)
	}
	// nearest-rank p90 over n=5: rank = ceil(0.9*5) = 5 -> sorted[4] = 5.
	if s.P90S != 5 {
		t.Fatalf("P90S = %v, want 5", s.P90S)
	}

	empty := buildDurationStats(nil)
	if empty.Available {
		t.Fatalf("empty: Available = true, want false")
	}
	if !strings.Contains(empty.Reason, "duration_s") {
		t.Fatalf("empty: Reason = %q, want mention of duration_s", empty.Reason)
	}
}

func TestBuildDegradedStats_ExactShare(t *testing.T) {
	s := buildDegradedStats(true, map[string]int{"codex": 4}, map[string]int{"codex": 1})
	if !s.Available {
		t.Fatalf("Available = false, want true")
	}
	got, ok := s.ByReviewerFamily["codex"]
	if !ok {
		t.Fatalf("ByReviewerFamily missing codex: %+v", s.ByReviewerFamily)
	}
	if got.Total != 4 || got.Degraded != 1 || math.Abs(got.Share-0.25) > 1e-9 {
		t.Fatalf("codex = %+v, want {Total:4 Degraded:1 Share:0.25}", got)
	}

	unseen := buildDegradedStats(false, nil, nil)
	if unseen.Available {
		t.Fatalf("unseen: Available = true, want false")
	}
	if !strings.Contains(unseen.Reason, "reviewer_family") {
		t.Fatalf("unseen: Reason = %q, want mention of reviewer_family", unseen.Reason)
	}
}

func TestBuildReReviewStats_RedundantConfirmationsSorted(t *testing.T) {
	confirmed := map[string]int{"age-a": 3, "age-b": 1, "age-c": 2}
	commits := map[string]map[string]bool{
		"age-a": {fxSHA1: true, fxSHA2: true},
		"age-c": {fxSHA3: true},
	}
	s := buildReReviewStats(confirmed, commits)
	if s.RereviewedBeads != 2 {
		t.Fatalf("RereviewedBeads = %d, want 2 (age-b has a single CONFIRMED)", s.RereviewedBeads)
	}
	if s.RedundantConfirmations != 3 {
		t.Fatalf("RedundantConfirmations = %d, want 3 ((3-1)+(2-1))", s.RedundantConfirmations)
	}
	want := []reReviewedBead{
		{Bead: "age-a", Confirmed: 3, DistinctCommits: 2},
		{Bead: "age-c", Confirmed: 2, DistinctCommits: 1},
	}
	if len(s.Beads) != len(want) {
		t.Fatalf("Beads len = %d, want %d", len(s.Beads), len(want))
	}
	for i, b := range want {
		if s.Beads[i] != b {
			t.Fatalf("Beads[%d] = %+v, want %+v (sorted by bead id)", i, s.Beads[i], b)
		}
	}

	none := buildReReviewStats(map[string]int{"age-x": 1}, nil)
	if none.Beads == nil || len(none.Beads) != 0 {
		t.Fatalf("no re-reviews: Beads = %#v, want non-nil empty slice (JSON [])", none.Beads)
	}
}

func TestBuildTrendStats_SortedByDateAndWindowStamp(t *testing.T) {
	since := fixedNow.AddDate(0, 0, -7)
	buckets := map[string]*trendDay{
		"2026-07-01": {Date: "2026-07-01", Verdicts: 2, Confirmed: 1, Refuted: 1},
		"2026-06-29": {Date: "2026-06-29", Verdicts: 1, Confirmed: 1},
	}
	s := buildTrendStats(7, since, buckets)
	if s.Days != 7 {
		t.Fatalf("Days = %d, want 7", s.Days)
	}
	if s.Since != since.UTC().Format(time.RFC3339) {
		t.Fatalf("Since = %q, want %q", s.Since, since.UTC().Format(time.RFC3339))
	}
	if len(s.ByDay) != 2 || s.ByDay[0].Date != "2026-06-29" || s.ByDay[1].Date != "2026-07-01" {
		t.Fatalf("ByDay = %+v, want ascending dates [2026-06-29 2026-07-01]", s.ByDay)
	}

	allTime := buildTrendStats(0, time.Time{}, map[string]*trendDay{})
	if allTime.Since != "" {
		t.Fatalf("all-time Since = %q, want empty (days<=0 leaves the window unstamped)", allTime.Since)
	}
	if allTime.ByDay == nil || len(allTime.ByDay) != 0 {
		t.Fatalf("all-time ByDay = %#v, want non-nil empty slice (JSON [])", allTime.ByDay)
	}
}
