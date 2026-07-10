// practices: [dora-metrics, andon-cord]
//
// `ao yield report` — the ON-THE-LOOP governance surface (age-mv67, flywheel
// epic age-36be). A returning operator reads ONE command after hours of
// autonomy — no transcript required — showing:
//
//  1. YIELD — what the loop banked: gate-verdict counts since the cutoff,
//     the catches the membrane recorded (classed REFUTEs, via
//     yieldledger.DetectCatches on the in-window events), and the beads the
//     tracker closed in the window.
//  2. VERIFIED FRONTIER — the last-known-good origin/main sha (every walked
//     ancestor RESOLVED: CONFIRMED verdict edge or #trivial provenance-only
//     waiver) plus the pending window of landed commits awaiting verdicts
//     (age-fdae; computation in yield_frontier.go).
//  3. ANDON QUEUE — what the loop parked for a human: blocked beads,
//     ESCALATE/HOLD pawl verdicts, any REFUTED verdict whose bead is
//     still open (a stalled slice), and stale goal-design packets — a
//     draft/validated packet under .agents/goal-design/ whose candidate work
//     demonstrably shipped (evidence-bound-goal-closeout S3). Each row: id,
//     why parked, age.
//
// Doctrine: docs/architecture/the-flywheel.md — "The human moves from in the
// loop to on it": review asynchronously the yield and the andon queue, the two
// things the loop accumulates.
//
// Data sources: the yield ledger (.agents/yield/yield-ledger.jsonl, via
// cli/internal/yieldledger) and the resolved beads tracker (tracker-agnostic —
// the same resolveTracker/canonicalizeBDReadJSON internals `ao beads exec`
// uses, so bd and br both work; the canonical row shape is br's
// {issues:[...]}). Beads access degrades gracefully: a tracker failure is
// REPORTED (beads_error), never fabricated and never fatal — the operator
// still sees the ledger half of the surface.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/wiki"
	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// yieldReportDefaultSince is the default lookback window: "read it the next
// morning" after an overnight autonomous run.
const yieldReportDefaultSince = 24 * time.Hour

var (
	yieldReportSince string
	yieldReportJSON  bool
)

// yieldReportNow is the report's clock, a seam so tests freeze window math and
// age rendering. Production never overrides it.
var yieldReportNow = time.Now

// yieldReportListBeadsByStatus is the beads seam: list the tracker's issues
// with one status, decoded from the canonical (br-shaped) {issues:[...]} JSON.
// A package-level var so tests stub the tracker without spawning processes.
var yieldReportListBeadsByStatus = listReportBeadsByStatus

var yieldReportCmd = &cobra.Command{
	Use:   "report [--since <RFC3339|duration>] [--json]",
	Short: "The on-the-loop governance surface: the YIELD and the ANDON QUEUE since a cutoff",
	Long: `Print what an autonomous loop did — and what it parked for you — without
reading any transcript (the on-the-loop review surface;
docs/architecture/the-flywheel.md).

Sections:
  YIELD        gate-verdict counts (CONFIRMED/REFUTED/ESCALATE/HOLD) since the
               cutoff, the catches recorded (classed membrane REFUTEs), and the
               beads closed in the window.
  VERIFIED FRONTIER
               the last-known-good origin/main sha — the highest commit whose
               walked ancestors ALL resolve (a CONFIRMED verdict edge in
               docs/provenance/ledger.jsonl, or the #trivial provenance-only
               waiver; a REFUTED verdict beats the waiver) — plus the pending
               window: each landed commit above the frontier still awaiting
               its verdict (sha, bead, age). Empty window ⇒ the frontier IS
               origin/main. Walk bounded at 200 commits.
  ANDON QUEUE  what needs a human: blocked beads, ESCALATE/HOLD pawl verdicts,
               any REFUTED verdict whose bead is still open (a stalled
               slice), and stale goal-design packets (still draft/validated
               under .agents/goal-design/ while a CONFIRMED provenance verdict
               or a closed candidate bead proves the work shipped). Each row:
               id, why parked, age.

Data sources: the yield ledger (.agents/yield/yield-ledger.jsonl) and the
resolved beads tracker (bd or br — the same tracker-agnostic resolution
'ao beads exec' uses). A tracker failure is reported, never fatal: the ledger
half still prints.

--since accepts an RFC3339 instant or a Go duration (e.g. 8h, 90m); default 24h.

  ao yield report
  ao yield report --since 8h
  ao yield report --since 2026-07-08T00:00:00Z --json`,
	Args: cobra.NoArgs,
	RunE: runYieldReport,
}

func init() {
	yieldReportCmd.Flags().StringVar(&yieldReportSince, "since", "",
		"cutoff: an RFC3339 instant or a duration lookback like 8h (default 24h)")
	yieldReportCmd.Flags().BoolVar(&yieldReportJSON, "json", false,
		"emit the full report struct as JSON")
	yieldCmd.AddCommand(yieldReportCmd)
}

// reportBead is the subset of a canonical tracker issue row the report reads
// (br's {issues:[...]} element; bd rows are reshaped to it upstream).
type reportBead struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ClosedAt  string `json:"closed_at"`
}

// yieldReportVerdicts are the gate-verdict counts in the window, by disposition.
type yieldReportVerdicts struct {
	Confirmed int `json:"confirmed"`
	Refuted   int `json:"refuted"`
	Escalate  int `json:"escalate"`
	Hold      int `json:"hold"`
}

// yieldReportCatch is one recorded catch class (a membrane REFUTED with real
// domain+reason) detected in the window.
type yieldReportCatch struct {
	ClassKey string   `json:"class_key"`
	Domain   string   `json:"domain"`
	Reason   string   `json:"reason"`
	Hits     int      `json:"hits"`
	Beads    []string `json:"beads"`
}

// yieldReportClosedBead is one bead the tracker closed in the window.
type yieldReportClosedBead struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	ClosedAt string `json:"closed_at"`
}

// yieldReportYield is the YIELD section: what the loop banked.
type yieldReportYield struct {
	Verdicts    yieldReportVerdicts     `json:"verdicts"`
	Catches     []yieldReportCatch      `json:"catches"`
	ClosedBeads []yieldReportClosedBead `json:"closed_beads"`
}

// Andon row kinds — why a row is parked.
const (
	andonKindBlocked     = "blocked"
	andonKindEscalate    = "escalate"
	andonKindHold        = "hold"
	andonKindStalled     = "stalled"
	andonKindStalePacket = "stale-packet"
)

// yieldReportAndonRow is one parked item awaiting a human: the bead, why it is
// parked, and how long it has been waiting.
type yieldReportAndonRow struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"` // blocked | escalate | hold | stalled | stale-packet
	Why   string `json:"why"`
	Age   string `json:"age"`
	Since string `json:"since,omitempty"` // RFC3339 of when it parked, when known
	Title string `json:"title,omitempty"`
}

// yieldReportDoc is the full --json struct.
type yieldReportDoc struct {
	Since       string                `json:"since"`
	GeneratedAt string                `json:"generated_at"`
	Yield       yieldReportYield      `json:"yield"`
	// FrontierSHA is the VERIFIED FRONTIER: the highest origin/main commit
	// whose walked ancestors ALL satisfy RESOLVED (age-fdae; "" when no
	// walked commit qualifies or the frontier is unavailable).
	FrontierSHA string `json:"frontier_sha"`
	// Pending is the pending window: every origin/main commit above the
	// frontier, newest first — landed, awaiting its verdict.
	Pending []yieldReportPendingCommit `json:"pending"`
	// FrontierError reports a degraded frontier (no git repo, no
	// origin/main) — reported, never fatal, like BeadsError.
	FrontierError string                `json:"frontier_error,omitempty"`
	AndonQueue    []yieldReportAndonRow `json:"andon_queue"`
	BeadsError    string                `json:"beads_error,omitempty"`
}

// runYieldReport wires the cobra invocation to the report core.
func runYieldReport(cmd *cobra.Command, _ []string) error {
	// repoRootOrCwd, not resolveProjectDir: every data source below is a
	// repo-rooted artifact, and a raw cwd from a subdirectory (e.g. cli/)
	// silently empties the report (verification-surface-honesty S1).
	root, err := repoRootOrCwd()
	if err != nil {
		return err
	}
	now := yieldReportNow().UTC()
	since, err := parseReportSince(yieldReportSince, now)
	if err != nil {
		return err
	}
	ledger, err := yieldledger.Load(root)
	if err != nil {
		return err
	}
	doc := buildYieldReport(ledger, root, since, now)
	if yieldReportJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	}
	return writeYieldReportText(cmd.OutOrStdout(), doc, now)
}

// parseReportSince resolves the --since value against now: empty means the
// default 24h lookback, a Go duration (8h, 90m) means now-dur, and an RFC3339
// instant is used as-is. Anything else — including a non-positive duration —
// is an error, never a silent default.
func parseReportSince(raw string, now time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return now.Add(-yieldReportDefaultSince), nil
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts.UTC(), nil
	}
	if d, err := time.ParseDuration(raw); err == nil {
		if d <= 0 {
			return time.Time{}, fmt.Errorf("--since duration must be positive, got %q", raw)
		}
		return now.Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("invalid --since %q (want RFC3339 like 2026-07-08T00:00:00Z or a duration like 8h)", raw)
}

// buildYieldReport assembles the full report document from the ledger and the
// tracker. Tracker failures degrade to BeadsError; the ledger sections always
// compute.
func buildYieldReport(ledger *yieldledger.Ledger, root string, since, now time.Time) yieldReportDoc {
	doc := yieldReportDoc{
		Since:       since.Format(time.RFC3339),
		GeneratedAt: now.Format(time.RFC3339),
	}

	window := windowedLedger(ledger, since)
	doc.Yield.Verdicts = countReportVerdicts(window)
	doc.Yield.Catches = reportCatches(window)

	frontierSHA, pending, frontierErr := buildFrontierSection(root, now)
	if frontierErr != nil {
		doc.FrontierError = frontierErr.Error()
	}
	doc.FrontierSHA = frontierSHA
	doc.Pending = pending

	beads, beadsErr := fetchReportBeads(root)
	if beadsErr != nil {
		doc.BeadsError = beadsErr.Error()
	}
	doc.Yield.ClosedBeads = reportClosedBeads(beads["closed"], since)
	doc.AndonQueue = buildAndonQueue(root, window, beads, since, now)
	return doc
}

// windowedLedger returns a sub-ledger holding only the events at or after the
// cutoff, so every ledger-derived section (counts, catches) shares one filter.
func windowedLedger(l *yieldledger.Ledger, since time.Time) *yieldledger.Ledger {
	out := &yieldledger.Ledger{SchemaVersion: yieldledger.SchemaVersion}
	if l == nil {
		return out
	}
	out.GeneratedAt = l.GeneratedAt
	for _, ev := range l.Events {
		ts, err := time.Parse(time.RFC3339, ev.TS)
		if err != nil || ts.Before(since) {
			continue
		}
		out.Events = append(out.Events, ev)
	}
	return out
}

// countReportVerdicts tallies the windowed gate-verdicts by disposition.
func countReportVerdicts(window *yieldledger.Ledger) yieldReportVerdicts {
	var v yieldReportVerdicts
	for _, ev := range window.Events {
		if ev.Event != yieldledger.EventGateVerdict || ev.GateVerdict == nil {
			continue
		}
		switch ev.GateVerdict.Disposition {
		case yieldledger.DispositionConfirmed:
			v.Confirmed++
		case yieldledger.DispositionRefuted:
			v.Refuted++
		case yieldledger.DispositionEscalate:
			v.Escalate++
		case yieldledger.DispositionHold:
			v.Hold++
		}
	}
	return v
}

// reportCatches projects DetectCatches over the windowed events into the
// report's catch rows, most-hit classes first (stable tie-break by class key).
func reportCatches(window *yieldledger.Ledger) []yieldReportCatch {
	catches := yieldledger.DetectCatches(window)
	out := make([]yieldReportCatch, 0, len(catches))
	for _, c := range catches {
		out = append(out, yieldReportCatch{
			ClassKey: c.ClassKey,
			Domain:   c.Domain,
			Reason:   c.Reason,
			Hits:     c.HitCount,
			Beads:    append([]string{}, c.Beads...),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Hits != out[j].Hits {
			return out[i].Hits > out[j].Hits
		}
		return out[i].ClassKey < out[j].ClassKey
	})
	return out
}

// reportBeadStatuses are the tracker statuses the report queries: closed feeds
// the yield section; blocked feeds the andon queue; open + in_progress resolve
// whether a REFUTED bead is still open (stalled).
var reportBeadStatuses = []string{"closed", "blocked", "open", "in_progress"}

// fetchReportBeads queries the tracker once per status of interest. The first
// failure aborts the fetch and is returned for honest reporting (no partial
// silent results); whatever was fetched before the failure is still returned.
func fetchReportBeads(root string) (map[string][]reportBead, error) {
	out := make(map[string][]reportBead, len(reportBeadStatuses))
	for _, status := range reportBeadStatuses {
		rows, err := yieldReportListBeadsByStatus(root, status)
		if err != nil {
			return out, fmt.Errorf("beads list --status %s: %w", status, err)
		}
		out[status] = rows
	}
	return out, nil
}

// reportClosedBeads filters the closed rows to those closed at/after the
// cutoff (closed_at, falling back to updated_at for trackers that omit it),
// most recent first.
func reportClosedBeads(closed []reportBead, since time.Time) []yieldReportClosedBead {
	out := []yieldReportClosedBead{}
	for _, b := range closed {
		closedAt := firstParseableTime(b.ClosedAt, b.UpdatedAt)
		if closedAt.IsZero() || closedAt.Before(since) {
			continue
		}
		out = append(out, yieldReportClosedBead{
			ID:       b.ID,
			Title:    b.Title,
			ClosedAt: closedAt.UTC().Format(time.RFC3339),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ClosedAt > out[j].ClosedAt })
	return out
}

// buildAndonQueue assembles the parked-for-a-human rows from the four
// sources, deduped per id (blocked wins over a verdict row, ESCALATE/HOLD
// wins over stalled, bead rows win over a same-id stale-packet row),
// oldest-parked first.
func buildAndonQueue(root string, window *yieldledger.Ledger, beads map[string][]reportBead, since, now time.Time) []yieldReportAndonRow {
	rows := []yieldReportAndonRow{}
	seen := map[string]bool{}
	add := func(row yieldReportAndonRow) {
		if row.ID == "" || seen[row.ID] {
			return
		}
		seen[row.ID] = true
		rows = append(rows, row)
	}

	// 1) Blocked beads — parked regardless of when (a queue, not a window).
	for _, b := range beads["blocked"] {
		parkedAt := firstParseableTime(b.UpdatedAt, b.CreatedAt)
		add(yieldReportAndonRow{
			ID:    b.ID,
			Kind:  andonKindBlocked,
			Why:   "bead blocked — needs unblocking",
			Age:   fmtReportAge(now.Sub(parkedAt)),
			Since: rfc3339OrEmpty(parkedAt),
			Title: b.Title,
		})
	}

	// 2) ESCALATE / HOLD verdicts in the window — the loop asked for a human.
	// 3) REFUTED verdicts whose bead is still open — a stalled slice.
	titles := beadTitleIndex(beads)
	stillOpen := beadStatusSet(beads, "open", "in_progress")
	for _, v := range latestVerdictPerBead(window) {
		ts, _ := time.Parse(time.RFC3339, v.ts)
		switch v.disposition {
		case yieldledger.DispositionEscalate:
			add(yieldReportAndonRow{
				ID: v.bead, Kind: andonKindEscalate,
				Why: "pawl ESCALATE — awaiting human decision",
				Age: fmtReportAge(now.Sub(ts)), Since: rfc3339OrEmpty(ts), Title: titles[v.bead],
			})
		case yieldledger.DispositionHold:
			add(yieldReportAndonRow{
				ID: v.bead, Kind: andonKindHold,
				Why: "pawl HOLD — held for a human",
				Age: fmtReportAge(now.Sub(ts)), Since: rfc3339OrEmpty(ts), Title: titles[v.bead],
			})
		case yieldledger.DispositionRefuted:
			if !stillOpen[v.bead] {
				continue
			}
			add(yieldReportAndonRow{
				ID: v.bead, Kind: andonKindStalled,
				Why: "REFUTED, bead still open — stalled slice",
				Age: fmtReportAge(now.Sub(ts)), Since: rfc3339OrEmpty(ts), Title: titles[v.bead],
			})
		}
	}

	// 4) Stale goal-design packets — still draft/validated while their
	// candidate work demonstrably shipped (evidence-bound-goal-closeout S3).
	// Pull-style at report time, never a daemon (ADR-0009).
	for _, row := range sweepStalePackets(root, beads, now) {
		add(row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		// Oldest parked first: an empty Since (unknown) sorts last.
		si, sj := rows[i].Since, rows[j].Since
		if (si == "") != (sj == "") {
			return sj == ""
		}
		if si != sj {
			return si < sj
		}
		return rows[i].ID < rows[j].ID
	})
	return rows
}

// beadVerdict is the latest-verdict projection buildAndonQueue reads.
type beadVerdict struct {
	bead        string
	disposition string
	ts          string
}

// latestVerdictPerBead collapses the windowed gate-verdicts to the LATEST one
// per bead (append order — review rounds supersede), preserving first-seen bead
// order for determinism.
func latestVerdictPerBead(window *yieldledger.Ledger) []beadVerdict {
	byBead := map[string]*beadVerdict{}
	order := []string{}
	for _, ev := range window.Events {
		if ev.Event != yieldledger.EventGateVerdict || ev.GateVerdict == nil {
			continue
		}
		v, ok := byBead[ev.BeadID]
		if !ok {
			v = &beadVerdict{bead: ev.BeadID}
			byBead[ev.BeadID] = v
			order = append(order, ev.BeadID)
		}
		v.disposition = ev.GateVerdict.Disposition
		v.ts = ev.TS
	}
	out := make([]beadVerdict, 0, len(order))
	for _, id := range order {
		out = append(out, *byBead[id])
	}
	return out
}

// beadTitleIndex maps bead id -> title across every fetched status list.
func beadTitleIndex(beads map[string][]reportBead) map[string]string {
	out := map[string]string{}
	for _, rows := range beads {
		for _, b := range rows {
			if b.Title != "" {
				out[b.ID] = b.Title
			}
		}
	}
	return out
}

// beadStatusSet returns the set of bead ids present in any of the named status
// lists.
func beadStatusSet(beads map[string][]reportBead, statuses ...string) map[string]bool {
	out := map[string]bool{}
	for _, s := range statuses {
		for _, b := range beads[s] {
			out[b.ID] = true
		}
	}
	return out
}

// goalDesignRelDir is the repo-rooted directory the stale-packet sweep globs.
const goalDesignRelDir = ".agents/goal-design"

// sweepStalePackets is andon source 4: the stale goal-design-packet sweep
// (evidence-bound-goal-closeout S3). It globs <root>/.agents/goal-design/*/
// and flags every packet still in draft or validated whose candidate work
// demonstrably shipped. Pull-style at report time from files already on disk
// — no daemon, no background anything (ADR-0009). Tolerant by design: an
// unreadable dir, artifact, or ledger line is skipped, never fatal.
func sweepStalePackets(root string, beads map[string][]reportBead, now time.Time) []yieldReportAndonRow {
	dirs, err := filepath.Glob(filepath.Join(root, goalDesignRelDir, "*"))
	if err != nil || len(dirs) == 0 {
		return nil
	}
	rows := []yieldReportAndonRow{}
	for _, dir := range dirs { // Glob output is sorted — deterministic sweep order.
		if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
			continue
		}
		if row, ok := stalePacketRow(root, dir, beads, now); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// stalePacketRow evaluates ONE packet dir and, when it is stale, returns its
// andon row: id = the packet slug, why = the packet path + the closing
// evidence found. A packet is stale only when its status is draft/validated
// in every artifact that declares one (closed or superseded anywhere is
// terminal — never flagged) AND at least one mechanical evidence arm holds:
// a CONFIRMED verdict edge in the provenance ledger naming the slug, a
// driver candidate bead resolving to a CLOSED tracker bead, or a landed
// trunk commit whose subject cites the slug together with a candidate id.
// A packet with no evidence is never flagged; bare git-log slug mentions are
// deliberately not an arm (the packet-creation commit always names the slug —
// the landed-commit arm requires a candidate id precisely so it can never
// self-trigger on that commit).
func stalePacketRow(root, dir string, beads map[string][]reportBead, now time.Time) (yieldReportAndonRow, bool) {
	intentText := readPacketArtifact(filepath.Join(dir, "intent.md"))
	driverText := readPacketArtifact(filepath.Join(dir, "driver.md"))
	if intentText == "" && driverText == "" {
		return yieldReportAndonRow{}, false
	}
	codec := wiki.FrontmatterCodec{}
	intentFM := codec.ExtractStringFields(strings.Split(intentText, "\n"), "status", "slug")
	driverFM := codec.ExtractStringFields(strings.Split(driverText, "\n"), "status", "slug")
	status, eligible := stalePacketStatus(intentFM["status"], driverFM["status"])
	if !eligible {
		return yieldReportAndonRow{}, false
	}
	slug := firstNonEmpty(driverFM["slug"], intentFM["slug"], filepath.Base(dir))
	candidateIDs := packetCandidateBeadIDs(driverText)

	evidence := []string{}
	parkedAt := time.Time{}
	if why, ts := stalePacketLedgerEvidence(root, slug); why != "" {
		evidence = append(evidence, why)
		parkedAt = ts
	}
	if why, ts := stalePacketClosedBeadEvidence(candidateIDs, beads["closed"]); why != "" {
		evidence = append(evidence, why)
		if parkedAt.IsZero() || (!ts.IsZero() && ts.Before(parkedAt)) {
			parkedAt = ts
		}
	}
	if why, ts := stalePacketLandedCommitEvidence(root, slug, candidateIDs); why != "" {
		evidence = append(evidence, why)
		if parkedAt.IsZero() || (!ts.IsZero() && ts.Before(parkedAt)) {
			parkedAt = ts
		}
	}
	if len(evidence) == 0 {
		return yieldReportAndonRow{}, false
	}

	age := "0m"
	if !parkedAt.IsZero() {
		age = fmtReportAge(now.Sub(parkedAt))
	}
	return yieldReportAndonRow{
		ID:   slug,
		Kind: andonKindStalePacket,
		Why: fmt.Sprintf("stale goal-design packet %s/%s (status %s) — %s",
			goalDesignRelDir, filepath.Base(dir), status, strings.Join(evidence, "; ")),
		Age:   age,
		Since: rfc3339OrEmpty(parkedAt),
	}, true
}

// stalePacketStatus folds the per-artifact statuses into (display status,
// sweep-eligible). closed/superseded in ANY artifact is terminal; eligibility
// requires draft or validated in at least one. Unknown or missing statuses
// alone never flag a packet (tolerant: fail toward silence, not false alarms).
func stalePacketStatus(statuses ...string) (string, bool) {
	display, eligible := "", false
	for _, s := range statuses {
		switch s {
		case "closed", "superseded":
			return s, false
		case "draft", "validated":
			display, eligible = s, true
		}
	}
	return display, eligible
}

// readPacketArtifact reads one packet artifact, mapping any error (missing
// file, unreadable) to "" — the sweep is advisory and tolerant.
func readPacketArtifact(path string) string {
	data, err := os.ReadFile(path) // #nosec G304 -- path is <root>/.agents/goal-design/<dir>/{intent,driver}.md under the resolved project dir.
	if err != nil {
		return ""
	}
	return string(data)
}

// packetCandidateBeadIDs extracts candidate_beads[].id values from the driver
// frontmatter with a tolerant line scan: inside the frontmatter block, collect
// "- id:" entries between the top-level "candidate_beads:" key and the next
// top-level key. The top-level artifact "id:" field is never collected.
func packetCandidateBeadIDs(driverText string) []string {
	ids := []string{}
	inFrontmatter, inCandidates := false, false
	dashes := 0
	for _, line := range strings.Split(driverText, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			dashes++
			if dashes >= 2 {
				break
			}
			inFrontmatter = true
			continue
		}
		if !inFrontmatter {
			continue
		}
		if strings.HasPrefix(line, "candidate_beads:") {
			inCandidates = true
			continue
		}
		if inCandidates && line != "" && line[0] != ' ' && line[0] != '-' {
			inCandidates = false // next top-level key ends the block
		}
		if !inCandidates {
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "- id:"); ok {
			if id := strings.Trim(strings.TrimSpace(rest), `"'`); id != "" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// stalePacketLedgerEvidence is evidence arm (a): scan the worktree provenance
// ledger for a CONFIRMED verdict edge naming the packet slug in bead_id or
// evidence_ref. Line-tolerant on purpose — the sweep only needs the edge to
// parse, not the hash chain to verify (the frontier owns strict reads); a
// missing ledger or an undecodable line is silently skipped. Returns the
// evidence sentence and the latest matching edge ts.
func stalePacketLedgerEvidence(root, slug string) (string, time.Time) {
	data, err := os.ReadFile(filepath.Join(root, provenanceLedgerRelPath)) // #nosec G304 -- fixed repo-relative path under the resolved project dir.
	if err != nil {
		return "", time.Time{}
	}
	why := ""
	latest := time.Time{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var edge struct {
			FromType    string `json:"from_type"`
			BeadID      string `json:"bead_id"`
			EvidenceRef string `json:"evidence_ref"`
			TS          string `json:"ts"`
		}
		if json.Unmarshal([]byte(line), &edge) != nil {
			continue // tolerant sweep: skip undecodable lines
		}
		if edge.FromType != "verdict" || !strings.Contains(edge.EvidenceRef, "disposition=CONFIRMED") {
			continue
		}
		if !strings.Contains(edge.BeadID, slug) && !strings.Contains(edge.EvidenceRef, slug) {
			continue
		}
		why = fmt.Sprintf("CONFIRMED verdict edge naming %s in %s", slug, provenanceLedgerRelPath)
		if ts, terr := time.Parse(time.RFC3339, edge.TS); terr == nil && ts.After(latest) {
			latest = ts
		}
	}
	return why, latest
}

// stalePacketClosedBeadEvidence is evidence arm (b): the first driver
// candidate bead id that resolves to a CLOSED bead in the already-fetched
// tracker rows. When no tracker rows were fetchable (unresolvable tracker,
// degraded fetch) the closed list is empty and the arm skips silently.
func stalePacketClosedBeadEvidence(candidateIDs []string, closed []reportBead) (string, time.Time) {
	if len(candidateIDs) == 0 || len(closed) == 0 {
		return "", time.Time{}
	}
	closedByID := make(map[string]reportBead, len(closed))
	for _, b := range closed {
		closedByID[b.ID] = b
	}
	for _, id := range candidateIDs {
		if b, ok := closedByID[id]; ok {
			return fmt.Sprintf("candidate bead %s closed in tracker", id),
				firstParseableTime(b.ClosedAt, b.UpdatedAt)
		}
	}
	return "", time.Time{}
}

// stalePacketLandedCommitEvidence is evidence arm (c) (verification-surface-
// honesty S2): a landed trunk commit whose SUBJECT cites the packet slug
// together with one of the driver's candidate bead ids. Requiring the
// candidate id is what keeps the arm non-self-triggering: the packet-creation
// commit names the slug alone, so bare slug matching (the rejected design)
// would flag every packet the moment it is committed. `git log --grep` is
// only a pre-filter; the subject predicate is enforced here. Tolerant like
// the other arms: no git repo at root, or no matching commit, skips silently.
// The OLDEST matching commit wins so Since reflects when the work first
// landed.
func stalePacketLandedCommitEvidence(root, slug string, candidateIDs []string) (string, time.Time) {
	if slug == "" || len(candidateIDs) == 0 {
		return "", time.Time{}
	}
	cmd := exec.Command("git", "-C", root, "log", "--fixed-strings", "--grep", slug, "--format=%H%x09%cI%x09%s")
	cmd.Env = gitDiscoveryEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", time.Time{}
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- { // git log is newest-first; walk oldest-first
		parts := strings.SplitN(lines[i], "\t", 3)
		if len(parts) != 3 {
			continue
		}
		sha, ciso, subject := parts[0], parts[1], parts[2]
		if len(sha) < 12 || !strings.Contains(subject, slug) {
			continue
		}
		for _, cid := range candidateIDs {
			if !containsWord(subject, cid) {
				continue
			}
			why := fmt.Sprintf("landed commit %s cites candidate %s (%q)", sha[:12], cid, subject)
			ts, terr := time.Parse(time.RFC3339, ciso)
			if terr != nil {
				return why, time.Time{}
			}
			return why, ts.UTC()
		}
	}
	return "", time.Time{}
}

// containsWord reports whether word occurs in s bounded by non-alphanumerics
// (so candidate "B1" never matches inside "B12" or "AB1x").
func containsWord(s, word string) bool {
	if word == "" {
		return false
	}
	for start := 0; ; {
		i := strings.Index(s[start:], word)
		if i < 0 {
			return false
		}
		i += start
		before, after := i-1, i+len(word)
		leftOK := before < 0 || !isWordChar(s[before])
		rightOK := after >= len(s) || !isWordChar(s[after])
		if leftOK && rightOK {
			return true
		}
		start = i + 1
	}
}

// isWordChar reports whether b is an ASCII letter or digit (the word-boundary
// alphabet for candidate-id matching).
func isWordChar(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// firstParseableTime returns the first candidate that parses as RFC3339, or the
// zero time.
func firstParseableTime(candidates ...string) time.Time {
	for _, c := range candidates {
		if strings.TrimSpace(c) == "" {
			continue
		}
		if ts, err := time.Parse(time.RFC3339, c); err == nil {
			return ts
		}
	}
	return time.Time{}
}

// rfc3339OrEmpty formats a time, rendering the zero time (unknown) as "".
func rfc3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// fmtReportAge renders a parked-duration compactly: minutes under an hour,
// hours under two days, whole days beyond. Negative (clock skew) and unknown
// clamp to "0m".
func fmtReportAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// writeYieldReportText renders the two-section human report with honest
// empty-states — a blank section is never printed as silence.
func writeYieldReportText(out io.Writer, doc yieldReportDoc, now time.Time) error {
	fmt.Fprintf(out, "Yield report — since %s (generated %s)\n\n", doc.Since, doc.GeneratedAt)

	fmt.Fprintln(out, "YIELD — what the loop banked")
	v := doc.Yield.Verdicts
	if v.Confirmed+v.Refuted+v.Escalate+v.Hold == 0 {
		fmt.Fprintln(out, "  verdicts: none in window")
	} else {
		fmt.Fprintf(out, "  verdicts: %d CONFIRMED · %d REFUTED · %d ESCALATE · %d HOLD\n",
			v.Confirmed, v.Refuted, v.Escalate, v.Hold)
	}

	if len(doc.Yield.Catches) == 0 {
		fmt.Fprintln(out, "  catches: none")
	} else {
		fmt.Fprintf(out, "  catches (classed membrane REFUTEs): %d\n", len(doc.Yield.Catches))
		tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "    HITS\tDOMAIN\tREASON\tBEADS")
		for _, c := range doc.Yield.Catches {
			fmt.Fprintf(tw, "    %d\t%s\t%s\t%s\n",
				c.Hits, c.Domain, truncateReportText(c.Reason, 60), strings.Join(c.Beads, ","))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	if len(doc.Yield.ClosedBeads) == 0 {
		fmt.Fprintln(out, "  beads closed: none")
	} else {
		fmt.Fprintf(out, "  beads closed: %d\n", len(doc.Yield.ClosedBeads))
		tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "    ID\tCLOSED\tTITLE")
		for _, b := range doc.Yield.ClosedBeads {
			fmt.Fprintf(tw, "    %s\t%s\t%s\n", b.ID, b.ClosedAt, truncateReportText(b.Title, 60))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	if err := renderFrontierText(out, doc); err != nil {
		return err
	}

	fmt.Fprintln(out, "\nANDON QUEUE — what the loop parked for you")
	if doc.BeadsError != "" {
		fmt.Fprintf(out, "  ⚠ beads unavailable: %s (queue may be incomplete)\n", doc.BeadsError)
	}
	if len(doc.AndonQueue) == 0 {
		fmt.Fprintln(out, "  andon queue: empty — nothing parked")
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "  ID\tWHY\tAGE\tTITLE")
	for _, r := range doc.AndonQueue {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", r.ID, r.Why, r.Age, truncateReportText(r.Title, 48))
	}
	return tw.Flush()
}

// truncateReportText caps s at max runes with an ellipsis so table rows stay
// scannable.
func truncateReportText(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// listReportBeadsByStatus is the production beads seam: it resolves the
// tracker exactly as `ao beads exec` does, runs `<tracker> list --json
// --status <status>`, reshapes a bd payload to the canonical br
// {issues:[...]} envelope, and decodes the rows.
func listReportBeadsByStatus(cwd, status string) ([]reportBead, error) {
	res, err := resolveTracker(cwd, os.Environ())
	if err != nil {
		return nil, err
	}
	args := []string{"list", "--json", "--status", status}
	c := exec.Command(res.Binary, args...) // #nosec G204 -- res.Binary is resolved by resolveTracker (bd|br); args are a fixed read-only list query.
	c.Env = beadsExecChildEnv(res, cwd)
	c.Dir = beadsExecChildDir(res, cwd)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		var exitErr *exec.ExitError
		if msg := strings.TrimSpace(stderr.String()); msg != "" && errors.As(err, &exitErr) {
			return nil, fmt.Errorf("%s list --json --status %s: %s", res.Tracker, status, msg)
		}
		return nil, fmt.Errorf("%s list --json --status %s: %w", res.Tracker, status, err)
	}
	raw := stdout.Bytes()
	if res.Tracker == trackerBD {
		canonical, cerr := canonicalizeBDReadJSON("list", raw)
		if cerr != nil {
			return nil, fmt.Errorf("reshape bd list --json: %w", cerr)
		}
		raw = canonical
	}
	var envelope struct {
		Issues []reportBead `json:"issues"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(raw), &envelope); err != nil {
		return nil, fmt.Errorf("parse %s list --json: %w", res.Tracker, err)
	}
	return envelope.Issues, nil
}
