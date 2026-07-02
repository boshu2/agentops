// practices: [design-by-contract, code-complete]
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// verifyStatsDefaultDays is the sensible default trailing window for the trend
// section: one month of recent verified-done activity.
const verifyStatsDefaultDays = 30

var (
	// verifyStatsJSON emits the report as machine-readable JSON (stdout-as-data)
	// instead of the human text table.
	verifyStatsJSON bool
	// verifyStatsDays bounds the trailing-window trend (<=0 = all time).
	verifyStatsDays int
	// verifyStatsLedger optionally overrides the ledger path (default: the repo's
	// docs/provenance/ledger.jsonl via resolveLedgerPath). Exposed so the command
	// is hermetically testable against a fixture and can inspect any ledger copy.
	verifyStatsLedger string
)

// verifyStatsNow is the clock seam for the trailing-window trend. Production
// uses the wall clock; tests override it (restoring via t.Cleanup) so the trend
// section is deterministic. computeVerifyStats itself is PURE — it takes `now`
// as a parameter — so only the RunE path reads this var.
var verifyStatsNow = func() time.Time { return time.Now().UTC() }

var verifyStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Cost-of-verified-done instrument, derived entirely from the provenance ledger",
	Long: `Report the COST of verified-done from the committed provenance ledger
(docs/provenance/ledger.jsonl): how many rounds, how much wall time, what
refute rate, what degraded share, and how much re-review waste it took to reach
"done". North star age-hew1 drives the cost of verified-done toward the cost of
unverified-done; this is the ruler. Zero new state — pure ledger derivation.

Every metric is computed over verdict edges (from_type == "verdict"). The v1.1
enrichment metrics (rounds, duration_s, degraded/reviewer_family) DEGRADE
GRACEFULLY: on a pre-v1.1 ledger they report "unavailable" with the reason,
never a misleading zero.

Metrics:
  * verdict count by disposition (CONFIRMED/REFUTED/UNRECORDED/...)
  * round distribution                (v1.1 rounds field, when present)
  * median / p90 duration_s           (v1.1 duration_s field, when present)
  * degraded share by reviewer_family (v1.1 fields, when present)
  * re-review count: beads with >1 CONFIRMED verdict edge — the rebase-waste
    proxy (works on v1 too, via the same bead identity gen-membrane-receipts.sh
    uses: structured bead_id, else the evidence_ref pattern, else from_id@sha)
  * trend: verdicts per day over the trailing --days window

Reproduce every number with jq over the same file (L=docs/provenance/ledger.jsonl):
  ledger_records:   jq -s 'length' "$L"
  verdict_events:   jq -s '[.[]|select(.from_type=="verdict")]|length' "$L"
  dispositions:     jq -s '[.[]|select(.from_type=="verdict")
                       |((.evidence_ref//"")|capture("disposition=(?<d>[A-Za-z]+)")?.d)//"UNRECORDED"]
                       |group_by(.)|map({(.[0]):length})|add' "$L"
  rounds dist:      jq -s '[.[]|select(.from_type=="verdict")|select((.rounds//0)>0)|.rounds]
                       |group_by(.)|map({(.[0]|tostring):length})|add' "$L"
  duration median:  jq -s '[.[]|select(.from_type=="verdict")|select((.duration_s//0)>0)|.duration_s]|sort
                       |if length==0 then null
                        elif length%2==1 then .[length/2|floor]
                        else (.[length/2-1]+.[length/2])/2 end' "$L"
  duration p90:     jq -s '[.[]|select(.from_type=="verdict")|select((.duration_s//0)>0)|.duration_s]|sort
                       as $s|($s|length) as $n|if $n==0 then null
                       else $s[(($n*0.9|ceil)-1)|if .<0 then 0 else . end] end' "$L"
  degraded/family:  jq -s '[.[]|select(.from_type=="verdict")|select((.reviewer_family//"")!="")]
                       |group_by(.reviewer_family)
                       |map({family:.[0].reviewer_family,total:length,
                             degraded:(map(select(.degraded==true))|length)})' "$L"
  redundant CONF.:  jq -s '[.[]|select(.from_type=="verdict")
                       |select(((.evidence_ref//"")|test("disposition=CONFIRMED")))
                       |(.bead_id
                         //((.evidence_ref//"")|capture("^pawl-verdict (?<b>.+) disposition=[A-Za-z]+$")?.b)
                         //(.from_id|tostring|split("@")[0]))]
                       |group_by(.)|map(length-1)|add // 0' "$L"

Agent-ergonomic (Directive 13): --json emits stdout-as-data; diagnostics stay
on stderr.

Examples:
  ao verify stats                 # text report over the repo ledger
  ao verify stats --json          # machine-readable, jq-reproducible
  ao verify stats --days 7        # trend over the trailing week`,
	Args: cobra.NoArgs,
	RunE: runVerifyStats,
}

func init() {
	verifyCmd.AddCommand(verifyStatsCmd)
	verifyStatsCmd.Flags().BoolVar(&verifyStatsJSON, "json", false, "Emit machine-readable JSON (stdout-as-data)")
	verifyStatsCmd.Flags().IntVar(&verifyStatsDays, "days", verifyStatsDefaultDays, "Trailing window in days for the trend section (<=0 = all time)")
	verifyStatsCmd.Flags().StringVar(&verifyStatsLedger, "ledger", "", "Ledger path override (default: repo docs/provenance/ledger.jsonl)")
}

// verifyStatsReport is the structured, JSON-serializable cost-of-verified-done
// report. Field order mirrors the text render for readability.
type verifyStatsReport struct {
	// GeneratedAt is the report clock (the trailing-window anchor), RFC3339 UTC.
	GeneratedAt string `json:"generated_at"`
	// Ledger is the ledger the numbers came from (canonical relative path).
	Ledger string `json:"ledger"`
	// LedgerRecords is the total record count (every edge, not just verdicts).
	LedgerRecords int `json:"ledger_records"`
	// VerdictEvents is the count of verdict edges (from_type == "verdict").
	VerdictEvents int `json:"verdict_events"`
	// DistinctBeadsReviewed is the number of distinct beads any verdict reviewed.
	DistinctBeadsReviewed int `json:"distinct_beads_reviewed"`
	// Dispositions maps each verdict disposition to its count.
	Dispositions map[string]int `json:"dispositions"`
	// RefuteRate is REFUTED / VerdictEvents (0 when no verdicts).
	RefuteRate float64 `json:"refute_rate"`
	// Rounds is the v1.1 rounds-per-verdict distribution (degrades gracefully).
	Rounds roundsStats `json:"rounds"`
	// Duration is the v1.1 wall-time-per-verdict summary (degrades gracefully).
	Duration durationStats `json:"duration_s"`
	// Degraded is the v1.1 degraded-share-by-family summary (degrades gracefully).
	Degraded degradedStats `json:"degraded"`
	// ReReview is the rebase-waste proxy: beads with >1 CONFIRMED verdict.
	ReReview reReviewStats `json:"re_review"`
	// Trend is the per-day verdict trend over the trailing window.
	Trend trendStats `json:"trend"`
}

// roundsStats is the rounds-per-verdict distribution, or an unavailability
// reason when no verdict record carries the v1.1 rounds field.
type roundsStats struct {
	Available         bool          `json:"available"`
	Reason            string        `json:"reason,omitempty"`
	RecordsWithRounds int           `json:"records_with_rounds,omitempty"`
	Distribution      []roundBucket `json:"distribution,omitempty"`
}

// roundBucket is one (rounds, count) pair of the rounds distribution.
type roundBucket struct {
	Rounds int `json:"rounds"`
	Count  int `json:"count"`
}

// durationStats is the median/p90 wall-time summary, or an unavailability
// reason when no verdict record carries the v1.1 duration_s field.
type durationStats struct {
	Available bool    `json:"available"`
	Reason    string  `json:"reason,omitempty"`
	Count     int     `json:"count,omitempty"`
	MedianS   float64 `json:"median_s,omitempty"`
	P90S      float64 `json:"p90_s,omitempty"`
}

// degradedStats is the degraded share grouped by reviewer_family, or an
// unavailability reason when no verdict record carries the v1.1 fields.
type degradedStats struct {
	Available        bool                      `json:"available"`
	Reason           string                    `json:"reason,omitempty"`
	ByReviewerFamily map[string]familyDegraded `json:"by_reviewer_family,omitempty"`
}

// familyDegraded is the degraded share for one reviewer family.
type familyDegraded struct {
	Total    int     `json:"total"`
	Degraded int     `json:"degraded"`
	Share    float64 `json:"share"`
}

// reReviewStats is the rebase-waste proxy: how many beads were re-CONFIRMED and
// how many of those confirmations were redundant (count - 1 per bead).
type reReviewStats struct {
	RereviewedBeads        int              `json:"rereviewed_beads"`
	RedundantConfirmations int              `json:"redundant_confirmations"`
	Beads                  []reReviewedBead `json:"beads"`
}

// reReviewedBead is one bead that earned more than one CONFIRMED verdict.
type reReviewedBead struct {
	Bead            string `json:"bead"`
	Confirmed       int    `json:"confirmed"`
	DistinctCommits int    `json:"distinct_commits"`
}

// trendStats is the per-day verdict trend over the trailing window.
type trendStats struct {
	Days  int        `json:"days"`
	Since string     `json:"since,omitempty"`
	ByDay []trendDay `json:"by_day"`
}

// trendDay is one day's verdict activity.
type trendDay struct {
	Date      string `json:"date"`
	Verdicts  int    `json:"verdicts"`
	Confirmed int    `json:"confirmed"`
	Refuted   int    `json:"refuted"`
}

// reVerdictBead mirrors bead_of's regex in scripts/gen-membrane-receipts.sh:
// the greedy capture between "pawl-verdict " and " disposition=<alpha>" at the
// end of the evidence_ref. Go RE2 is greedy by default, matching jq's capture.
var reVerdictBead = regexp.MustCompile(`^pawl-verdict (.+) disposition=[A-Za-z]+$`)

// verdictBeadID resolves the bead a verdict edge reviews, mirroring bead_of in
// scripts/gen-membrane-receipts.sh so ao and the receipt generator agree:
// prefer the structured v1.1 bead_id, else parse the free-text evidence_ref,
// else the from_id prefix before "@". This is the re-review proxy's identity,
// and it works on v1 ledgers (no bead_id field) via the fallbacks.
func verdictBeadID(e provenancegraph.Edge) string {
	if strings.TrimSpace(e.BeadID) != "" {
		return e.BeadID
	}
	if m := reVerdictBead.FindStringSubmatch(e.EvidenceRef); m != nil {
		return m[1]
	}
	if i := strings.Index(e.FromID, "@"); i >= 0 {
		return e.FromID[:i]
	}
	return e.FromID
}

// verdictDisposition resolves a verdict edge's disposition using the SAME parse
// as `ao provenance show` (parseDisposition), falling back to "UNRECORDED" when
// the evidence_ref carries no disposition= token. On real data this equals the
// documented jq regex disposition=([A-Za-z]+) (dispositions are single words).
func verdictDisposition(e provenancegraph.Edge) string {
	if d := parseDisposition(e.EvidenceRef); d != "" {
		return d
	}
	return "UNRECORDED"
}

// noV11Reason is the standard "this v1.1 metric is unavailable" explanation.
func noV11Reason(field string) string {
	return fmt.Sprintf("no verdict record carries the v1.1 %s field (pre-v1.1 ledger, or not yet populated)", field)
}

// median returns the median of an ascending-sorted slice (0 for empty).
func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// percentileNearestRank returns the p-quantile (0<p<=1) of an ascending-sorted
// slice by the nearest-rank method: rank = ceil(p*n), value = sorted[rank-1].
// Matches the documented jq p90 query exactly.
func percentileNearestRank(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := int(math.Ceil(p * float64(n)))
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}

// computeVerifyStats derives the whole report from the ledger edges. PURE: no
// I/O and `now` is a parameter, so the trailing-window trend is deterministic
// under test. v1.1 field PRESENCE is detected as a nonzero typed value, which is
// exactly equivalent to jq's `!= null`: the production writer tags every v1.1
// field `omitempty`, so a zero value is always ABSENT from the JSON (and a
// present value is always nonzero — rounds>=1, duration_s>0, reviewer_family
// non-empty). That equivalence is why the documented jq queries reproduce these
// numbers byte-for-byte over any writer-produced ledger.
func computeVerifyStats(edges []provenancegraph.Edge, days int, now time.Time) verifyStatsReport {
	r := verifyStatsReport{
		GeneratedAt:   now.UTC().Format(time.RFC3339),
		Ledger:        provenancegraph.LedgerRelativePath,
		LedgerRecords: len(edges),
		Dispositions:  map[string]int{},
	}

	var (
		durations            []float64
		roundCounts          = map[int]int{}
		roundsN              int
		famTotals            = map[string]int{}
		famDegraded          = map[string]int{}
		famSeen              bool
		beadsSeen            = map[string]bool{}
		beadConfirmed        = map[string]int{}
		beadConfirmedCommits = map[string]map[string]bool{}
		refuted              int
	)

	// Trailing-window lower bound for the trend. days<=0 => all time.
	var since time.Time
	if days > 0 {
		since = now.AddDate(0, 0, -days)
	}
	dayBuckets := map[string]*trendDay{}

	for _, e := range edges {
		if e.FromType != "verdict" {
			continue
		}
		r.VerdictEvents++

		disp := verdictDisposition(e)
		r.Dispositions[disp]++
		if disp == "REFUTED" {
			refuted++
		}

		bead := verdictBeadID(e)
		beadsSeen[bead] = true
		if disp == "CONFIRMED" {
			beadConfirmed[bead]++
			if beadConfirmedCommits[bead] == nil {
				beadConfirmedCommits[bead] = map[string]bool{}
			}
			beadConfirmedCommits[bead][e.ToID] = true
		}

		// v1.1 enrichment — presence == nonzero (see doc comment above).
		if e.Rounds > 0 {
			roundCounts[e.Rounds]++
			roundsN++
		}
		if e.DurationS > 0 {
			durations = append(durations, e.DurationS)
		}
		if fam := strings.TrimSpace(e.ReviewerFamily); fam != "" {
			famSeen = true
			famTotals[fam]++
			if e.Degraded {
				famDegraded[fam]++
			}
		}

		// Trend bucket: verdicts on/after the window start, keyed by UTC date.
		if t, err := time.Parse(time.RFC3339, e.TS); err == nil {
			tu := t.UTC()
			if days <= 0 || !tu.Before(since) {
				date := tu.Format("2006-01-02")
				td := dayBuckets[date]
				if td == nil {
					td = &trendDay{Date: date}
					dayBuckets[date] = td
				}
				td.Verdicts++
				switch disp {
				case "CONFIRMED":
					td.Confirmed++
				case "REFUTED":
					td.Refuted++
				}
			}
		}
	}

	r.DistinctBeadsReviewed = len(beadsSeen)
	if r.VerdictEvents > 0 {
		r.RefuteRate = float64(refuted) / float64(r.VerdictEvents)
	}

	// Rounds distribution (v1.1).
	if roundsN > 0 {
		r.Rounds.Available = true
		r.Rounds.RecordsWithRounds = roundsN
		for k, v := range roundCounts {
			r.Rounds.Distribution = append(r.Rounds.Distribution, roundBucket{Rounds: k, Count: v})
		}
		sort.Slice(r.Rounds.Distribution, func(i, j int) bool {
			return r.Rounds.Distribution[i].Rounds < r.Rounds.Distribution[j].Rounds
		})
	} else {
		r.Rounds.Reason = noV11Reason("rounds")
	}

	// Duration median/p90 (v1.1).
	if len(durations) > 0 {
		sort.Float64s(durations)
		r.Duration.Available = true
		r.Duration.Count = len(durations)
		r.Duration.MedianS = median(durations)
		r.Duration.P90S = percentileNearestRank(durations, 0.90)
	} else {
		r.Duration.Reason = noV11Reason("duration_s")
	}

	// Degraded share by reviewer_family (v1.1).
	if famSeen {
		r.Degraded.Available = true
		r.Degraded.ByReviewerFamily = map[string]familyDegraded{}
		for fam, total := range famTotals {
			d := famDegraded[fam]
			share := 0.0
			if total > 0 {
				share = float64(d) / float64(total)
			}
			r.Degraded.ByReviewerFamily[fam] = familyDegraded{Total: total, Degraded: d, Share: share}
		}
	} else {
		r.Degraded.Reason = noV11Reason("reviewer_family")
	}

	// Re-review (rebase-waste proxy): beads with >1 CONFIRMED verdict.
	for bead, c := range beadConfirmed {
		if c >= 2 {
			r.ReReview.RereviewedBeads++
			r.ReReview.RedundantConfirmations += c - 1
			r.ReReview.Beads = append(r.ReReview.Beads, reReviewedBead{
				Bead:            bead,
				Confirmed:       c,
				DistinctCommits: len(beadConfirmedCommits[bead]),
			})
		}
	}
	sort.Slice(r.ReReview.Beads, func(i, j int) bool { return r.ReReview.Beads[i].Bead < r.ReReview.Beads[j].Bead })
	if r.ReReview.Beads == nil {
		r.ReReview.Beads = []reReviewedBead{}
	}

	// Trend.
	r.Trend.Days = days
	if days > 0 {
		r.Trend.Since = since.UTC().Format(time.RFC3339)
	}
	for _, td := range dayBuckets {
		r.Trend.ByDay = append(r.Trend.ByDay, *td)
	}
	sort.Slice(r.Trend.ByDay, func(i, j int) bool { return r.Trend.ByDay[i].Date < r.Trend.ByDay[j].Date })
	if r.Trend.ByDay == nil {
		r.Trend.ByDay = []trendDay{}
	}

	return r
}

// renderVerifyStatsText writes the human-readable cost-of-verified-done report.
func renderVerifyStatsText(out io.Writer, r verifyStatsReport) {
	fmt.Fprintf(out, "Verified-done cost — provenance ledger stats (as of %s)\n", r.GeneratedAt)
	fmt.Fprintf(out, "ledger: %s\n", r.Ledger)
	fmt.Fprintf(out, "records: %d   verdict events: %d   distinct beads reviewed: %d\n",
		r.LedgerRecords, r.VerdictEvents, r.DistinctBeadsReviewed)
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Dispositions:")
	if len(r.Dispositions) == 0 {
		fmt.Fprintln(out, "  (no verdict edges in ledger)")
	} else {
		for _, k := range sortedKeys(r.Dispositions) {
			fmt.Fprintf(out, "  %-12s %d\n", k, r.Dispositions[k])
		}
		fmt.Fprintf(out, "  refute rate: %.1f%%\n", r.RefuteRate*100)
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Rounds per verdict:")
	if r.Rounds.Available {
		for _, b := range r.Rounds.Distribution {
			fmt.Fprintf(out, "  %d round(s): %d\n", b.Rounds, b.Count)
		}
		fmt.Fprintf(out, "  (%d verdict records carry rounds)\n", r.Rounds.RecordsWithRounds)
	} else {
		fmt.Fprintf(out, "  unavailable — %s\n", r.Rounds.Reason)
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Duration per verdict (seconds):")
	if r.Duration.Available {
		fmt.Fprintf(out, "  median: %.1f   p90: %.1f   (n=%d)\n", r.Duration.MedianS, r.Duration.P90S, r.Duration.Count)
	} else {
		fmt.Fprintf(out, "  unavailable — %s\n", r.Duration.Reason)
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Degraded share by reviewer family:")
	if r.Degraded.Available {
		fams := make([]string, 0, len(r.Degraded.ByReviewerFamily))
		for f := range r.Degraded.ByReviewerFamily {
			fams = append(fams, f)
		}
		sort.Strings(fams)
		for _, fam := range fams {
			d := r.Degraded.ByReviewerFamily[fam]
			fmt.Fprintf(out, "  %-16s %d/%d degraded (%.1f%%)\n", fam, d.Degraded, d.Total, d.Share*100)
		}
	} else {
		fmt.Fprintf(out, "  unavailable — %s\n", r.Degraded.Reason)
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Re-review (rebase-waste proxy — beads with >1 CONFIRMED verdict):")
	fmt.Fprintf(out, "  re-reviewed beads: %d   redundant confirmations: %d\n",
		r.ReReview.RereviewedBeads, r.ReReview.RedundantConfirmations)
	for _, b := range r.ReReview.Beads {
		fmt.Fprintf(out, "  %-26s %d CONFIRMED across %d commit(s)\n", b.Bead, b.Confirmed, b.DistinctCommits)
	}
	fmt.Fprintln(out)

	if r.Trend.Days > 0 {
		fmt.Fprintf(out, "Trend (verdicts in the trailing %d days, since %s):\n", r.Trend.Days, r.Trend.Since)
	} else {
		fmt.Fprintln(out, "Trend (all recorded verdicts, by day):")
	}
	if len(r.Trend.ByDay) == 0 {
		fmt.Fprintln(out, "  (no verdicts in window)")
	} else {
		for _, d := range r.Trend.ByDay {
			fmt.Fprintf(out, "  %s  %d verdicts (%d confirmed, %d refuted)\n",
				d.Date, d.Verdicts, d.Confirmed, d.Refuted)
		}
	}
}

func runVerifyStats(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	path := strings.TrimSpace(verifyStatsLedger)
	if path == "" {
		path = resolveLedgerPath()
	}
	store := provenancegraph.NewStore(path)
	edges, err := store.Read()
	if err != nil {
		return fmt.Errorf("read provenance ledger: %w", err)
	}

	report := computeVerifyStats(edges, verifyStatsDays, verifyStatsNow())
	// When an explicit ledger override is given, report the path actually read
	// rather than the canonical relative label.
	if strings.TrimSpace(verifyStatsLedger) != "" {
		report.Ledger = path
	}

	out := cmd.OutOrStdout()
	if verifyStatsJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	renderVerifyStatsText(out, report)
	return nil
}
