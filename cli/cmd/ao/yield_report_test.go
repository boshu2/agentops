// Tests for `ao yield report` — the on-the-loop governance surface (age-mv67).
//
// Executed-red TDD: seeded ledger (via the production yieldledger.Writer, per the
// fixture-fidelity rule in .claude/rules/go.md) + a stubbed beads seam
// (yieldReportListBeadsByStatus) → assert section counts, andon rows, honest
// empty-states, and the --json shape.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/trackerexec"
	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// reportTestNow is the frozen clock every `ao yield report` test runs under so
// window math (default --since 24h) and age strings are deterministic.
var reportTestNow = time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)

// setYieldReportState points the report command at root, freezes its clock,
// stubs the beads seam to empty, and restores ALL shared package globals —
// including the shared cobra command's out/err writers — via t.Cleanup
// (.claude/rules/go.md test-isolation rule; mirrors setDigestProjectDir).
func setYieldReportState(t *testing.T, root string) {
	t.Helper()
	origProjectDir := testProjectDir
	testProjectDir = root
	origSince, origJSON := yieldReportSince, yieldReportJSON
	yieldReportSince, yieldReportJSON = "", false
	origList := yieldReportListBeadsByStatus
	yieldReportListBeadsByStatus = func(_ context.Context, cwd, status string) ([]reportBead, error) {
		return nil, nil
	}
	origNow := yieldReportNow
	yieldReportNow = func() time.Time { return reportTestNow }
	t.Cleanup(func() {
		testProjectDir = origProjectDir
		yieldReportSince, yieldReportJSON = origSince, origJSON
		yieldReportListBeadsByStatus = origList
		yieldReportNow = origNow
		yieldReportCmd.SetOut(nil)
		yieldReportCmd.SetErr(nil)
	})
}

// stubReportBeads points the beads seam at a canned per-status map (the canonical
// br {issues:[...]} elements, already decoded). Restore is registered by
// setYieldReportState.
func stubReportBeads(t *testing.T, byStatus map[string][]reportBead) {
	t.Helper()
	yieldReportListBeadsByStatus = func(_ context.Context, cwd, status string) ([]reportBead, error) {
		return byStatus[status], nil
	}
}

// seedReportVerdict appends one gate-verdict through the production writer so
// the fixture is the real persisted shape.
func seedReportVerdict(t *testing.T, root, bead, disposition, domain, reason string, ts time.Time) {
	t.Helper()
	w := yieldledger.Writer{}
	if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
		BeadID: bead, RunID: "run-report-test", TS: ts,
		Difficulty:     1,
		PawlVerdictRef: yieldledger.PawlVerdictRef{BeadID: bead, HeadSHA: "abcdef0"},
		Disposition:    disposition, HeadSHA: "abcdef0", Attempt: 1,
		AuthorContextID: "ctx-report-test", AuthorFamily: "claude",
		Domain: domain, Reason: reason,
	}); err != nil {
		t.Fatalf("seedReportVerdict(%s %s): %v", bead, disposition, err)
	}
}

// runReport executes runYieldReport on the shared command and returns stdout.
func runReport(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	yieldReportCmd.SetOut(&buf)
	t.Cleanup(func() { yieldReportCmd.SetOut(nil) }) // age-ztf8: shared cobra command — reset at the set-site
	if err := runYieldReport(yieldReportCmd, nil); err != nil {
		t.Fatalf("runYieldReport: %v", err)
	}
	return buf.String()
}

// decodeReport executes the command with --json and unmarshals the full struct.
func decodeReport(t *testing.T) yieldReportDoc {
	t.Helper()
	yieldReportJSON = true
	out := runReport(t)
	var doc yieldReportDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("unmarshal report JSON: %v\noutput:\n%s", err, out)
	}
	return doc
}

func TestYieldReportUsesCallerContextAndResolvedTrackerCommand(t *testing.T) {
	root := makeGitRepoForTracker(t)
	ledger := filepath.Join(root, "ledger")
	if err := os.Mkdir(ledger, 0o755); err != nil {
		t.Fatal(err)
	}
	tracker := filepath.Join(root, "tracker")
	trackerBody := `#!/bin/sh
printf 'cwd=%s beads=%s argc=%s arg1=<%s> arg2=<%s> arg3=<%s> arg4=<%s>\n' \
  "$PWD" "${BEADS_DIR-unset}" "$#" "$1" "$2" "$3" "$4" >> "$YIELD_TRACKER_LOG"
printf 'tracker-stdout\n'
printf 'tracker-stderr\n' >&2
exit 23
`
	if err := os.WriteFile(tracker, []byte(trackerBody), 0o755); err != nil {
		t.Fatal(err)
	}
	originalLookPath := trackerLookPath
	trackerLookPath = func(name string) (string, error) {
		if name == trackerBR || name == trackerBD {
			return tracker, nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { trackerLookPath = originalLookPath })
	t.Setenv("HOME", t.TempDir())

	for _, testCase := range []struct {
		name      string
		tracker   string
		wantBeads string
	}{
		{name: "br", tracker: trackerBR, wantBeads: ledger},
		{name: "bd", tracker: trackerBD, wantBeads: "unset"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			logPath := filepath.Join(root, testCase.name+".log")
			t.Setenv("AGENTOPS_TRACKER", testCase.tracker)
			t.Setenv("BEADS_DIR", ledger)
			t.Setenv("YIELD_TRACKER_LOG", logPath)
			_, queryErr := listReportBeadsByStatus(root, "open")
			var sharedExit *trackerexec.ExitError
			if !errors.As(queryErr, &sharedExit) || sharedExit.ExitCode() != 23 {
				t.Fatalf("tracker query error = %T %v, want shared *trackerexec.ExitError(23)", queryErr, queryErr)
			}
			if !strings.Contains(queryErr.Error(), "tracker-stderr") {
				t.Fatalf("tracker query error = %q, want child stderr", queryErr)
			}
			logBody, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			wantLog := "cwd=" + root + " beads=" + testCase.wantBeads +
				" argc=4 arg1=<list> arg2=<--json> arg3=<--status> arg4=<open>\n"
			if got := string(logBody); got != wantLog {
				t.Fatalf("resolved tracker log = %q, want %q", got, wantLog)
			}
		})
	}

	cancelLog := filepath.Join(root, "canceled.log")
	t.Setenv("AGENTOPS_TRACKER", trackerBR)
	t.Setenv("BEADS_DIR", ledger)
	t.Setenv("YIELD_TRACKER_LOG", cancelLog)
	originalProjectDir := testProjectDir
	originalSince, originalJSON := yieldReportSince, yieldReportJSON
	originalNow := yieldReportNow
	testProjectDir = root
	yieldReportSince, yieldReportJSON = "", false
	yieldReportNow = func() time.Time { return reportTestNow }
	t.Cleanup(func() {
		testProjectDir = originalProjectDir
		yieldReportSince, yieldReportJSON = originalSince, originalJSON
		yieldReportNow = originalNow
		yieldReportCmd.SetContext(context.Background())
		yieldReportCmd.SetOut(nil)
		yieldReportCmd.SetErr(nil)
	})
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	yieldReportCmd.SetContext(canceled)
	var output bytes.Buffer
	yieldReportCmd.SetOut(&output)
	if err := runYieldReport(yieldReportCmd, nil); err != nil {
		t.Fatalf("runYieldReport with canceled context: %v", err)
	}
	if _, err := os.Stat(cancelLog); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled caller context launched tracker child: stat error %v", err)
	}
}

// TestRunYieldReport_VerdictCountsSinceCutoff seeds verdicts inside and outside
// the default 24h window and asserts only the in-window ones are counted, per
// disposition.
func TestRunYieldReport_VerdictCountsSinceCutoff(t *testing.T) {
	root := t.TempDir()
	setYieldReportState(t, root)

	in := reportTestNow.Add(-2 * time.Hour)
	old := reportTestNow.Add(-30 * time.Hour) // before the 24h cutoff
	seedReportVerdict(t, root, "age-a", yieldledger.DispositionConfirmed, "", "", in)
	seedReportVerdict(t, root, "age-b", yieldledger.DispositionConfirmed, "", "", in)
	seedReportVerdict(t, root, "age-c", yieldledger.DispositionRefuted, "go", "missing cleanup restore of shared global", in)
	seedReportVerdict(t, root, "age-d", yieldledger.DispositionEscalate, "", "", in)
	seedReportVerdict(t, root, "age-old", yieldledger.DispositionConfirmed, "", "", old)
	seedReportVerdict(t, root, "age-old2", yieldledger.DispositionHold, "", "", old)

	doc := decodeReport(t)
	v := doc.Yield.Verdicts
	if v.Confirmed != 2 || v.Refuted != 1 || v.Escalate != 1 || v.Hold != 0 {
		t.Errorf("verdict counts = C%d R%d E%d H%d, want C2 R1 E1 H0", v.Confirmed, v.Refuted, v.Escalate, v.Hold)
	}
	if doc.Since != reportTestNow.Add(-24*time.Hour).Format(time.RFC3339) {
		t.Errorf("default since = %q, want now-24h", doc.Since)
	}
}

// TestRunYieldReport_CatchesSinceCutoff asserts catches are detected from
// in-window classifiable REFUTEs only, with class fields carried through.
func TestRunYieldReport_CatchesSinceCutoff(t *testing.T) {
	root := t.TempDir()
	setYieldReportState(t, root)

	in := reportTestNow.Add(-3 * time.Hour)
	old := reportTestNow.Add(-72 * time.Hour)
	seedReportVerdict(t, root, "age-1", yieldledger.DispositionRefuted, "shell", "unguarded cmdsub aborts under set -e", in)
	seedReportVerdict(t, root, "age-2", yieldledger.DispositionRefuted, "shell", "unguarded cmdsub aborts under set -e", in)
	seedReportVerdict(t, root, "age-3", yieldledger.DispositionRefuted, "docs", "stale retired surface in shipped docs", old)

	doc := decodeReport(t)
	if len(doc.Yield.Catches) != 1 {
		t.Fatalf("catches = %d, want exactly 1 (the out-of-window class must be excluded); got %+v",
			len(doc.Yield.Catches), doc.Yield.Catches)
	}
	c := doc.Yield.Catches[0]
	if c.Domain != "shell" || c.Hits != 2 {
		t.Errorf("catch = domain %q hits %d, want shell/2", c.Domain, c.Hits)
	}
	if len(c.Beads) != 2 {
		t.Errorf("catch beads = %v, want the 2 distinct beads", c.Beads)
	}
	if c.Reason != "unguarded cmdsub aborts under set -e" {
		t.Errorf("catch reason = %q", c.Reason)
	}
}

// TestRunYieldReport_ClosedBeadsWindow asserts the closed-beads section filters
// by closed_at against the cutoff and falls back to updated_at when closed_at is
// absent (tracker-agnostic: bd may omit it).
func TestRunYieldReport_ClosedBeadsWindow(t *testing.T) {
	root := t.TempDir()
	setYieldReportState(t, root)

	inWin := reportTestNow.Add(-1 * time.Hour).Format(time.RFC3339)
	outWin := reportTestNow.Add(-50 * time.Hour).Format(time.RFC3339)
	stubReportBeads(t, map[string][]reportBead{
		"closed": {
			{ID: "age-new", Title: "landed increment", Status: "closed", ClosedAt: inWin},
			{ID: "age-ancient", Title: "old work", Status: "closed", ClosedAt: outWin},
			{ID: "age-fallback", Title: "closed_at-less tracker row", Status: "closed", UpdatedAt: inWin},
		},
	})

	doc := decodeReport(t)
	got := map[string]bool{}
	for _, b := range doc.Yield.ClosedBeads {
		got[b.ID] = true
	}
	if !got["age-new"] || !got["age-fallback"] || got["age-ancient"] {
		t.Errorf("closed beads = %+v, want age-new + age-fallback only", doc.Yield.ClosedBeads)
	}
	if len(doc.Yield.ClosedBeads) != 2 {
		t.Errorf("closed beads count = %d, want 2", len(doc.Yield.ClosedBeads))
	}
}

// TestRunYieldReport_AndonRows asserts the three andon sources land as rows —
// blocked beads, ESCALATE/HOLD verdicts, and a REFUTED verdict whose bead is
// still open (a stalled slice) — that a REFUTED bead already closed is NOT
// parked, and that a bead is never listed twice (blocked wins over stalled).
func TestRunYieldReport_AndonRows(t *testing.T) {
	root := t.TempDir()
	setYieldReportState(t, root)

	in := reportTestNow.Add(-5 * time.Hour)
	seedReportVerdict(t, root, "age-esc", yieldledger.DispositionEscalate, "", "", in)
	seedReportVerdict(t, root, "age-hold", yieldledger.DispositionHold, "", "", in)
	seedReportVerdict(t, root, "age-stall", yieldledger.DispositionRefuted, "go", "acceptance not proven", in)
	seedReportVerdict(t, root, "age-done", yieldledger.DispositionRefuted, "go", "fixed then landed", in)
	seedReportVerdict(t, root, "age-both", yieldledger.DispositionRefuted, "go", "refuted then blocked", in)

	parkedAt := reportTestNow.Add(-26 * time.Hour).Format(time.RFC3339)
	stubReportBeads(t, map[string][]reportBead{
		"blocked": {
			{ID: "age-blk", Title: "needs a credential decision", Status: "blocked", UpdatedAt: parkedAt},
			{ID: "age-both", Title: "refuted then blocked", Status: "blocked", UpdatedAt: parkedAt},
		},
		"open": {
			{ID: "age-stall", Title: "stalled slice", Status: "open", UpdatedAt: parkedAt},
		},
		"closed": {
			{ID: "age-done", Title: "closed after fix", Status: "closed", ClosedAt: parkedAt},
		},
	})

	doc := decodeReport(t)
	kinds := map[string]string{}
	for _, row := range doc.AndonQueue {
		if prev, dup := kinds[row.ID]; dup {
			t.Errorf("bead %s parked twice (%s and %s) — andon rows must dedup", row.ID, prev, row.Kind)
		}
		kinds[row.ID] = row.Kind
	}
	want := map[string]string{
		"age-blk":   "blocked",
		"age-both":  "blocked", // blocked wins over stalled
		"age-esc":   "escalate",
		"age-hold":  "hold",
		"age-stall": "stalled",
	}
	for id, kind := range want {
		if kinds[id] != kind {
			t.Errorf("andon[%s] kind = %q, want %q (rows: %+v)", id, kinds[id], kind, doc.AndonQueue)
		}
	}
	if _, parked := kinds["age-done"]; parked {
		t.Errorf("age-done is closed — a REFUTED verdict on a closed bead must NOT be parked")
	}
	if len(doc.AndonQueue) != len(want) {
		t.Errorf("andon rows = %d, want %d: %+v", len(doc.AndonQueue), len(want), doc.AndonQueue)
	}
	// Each row carries why + age.
	for _, row := range doc.AndonQueue {
		if strings.TrimSpace(row.Why) == "" || strings.TrimSpace(row.Age) == "" {
			t.Errorf("andon row %+v missing why/age", row)
		}
	}
}

// TestRunYieldReport_TextSections asserts the plain-text rendering carries both
// section headers, the verdict counts line, and the andon table rows.
func TestRunYieldReport_TextSections(t *testing.T) {
	root := t.TempDir()
	setYieldReportState(t, root)

	in := reportTestNow.Add(-2 * time.Hour)
	seedReportVerdict(t, root, "age-a", yieldledger.DispositionConfirmed, "", "", in)
	seedReportVerdict(t, root, "age-b", yieldledger.DispositionRefuted, "go", "assertion missing", in)
	stubReportBeads(t, map[string][]reportBead{
		"closed": {{ID: "age-c", Title: "shipped thing", Status: "closed",
			ClosedAt: reportTestNow.Add(-1 * time.Hour).Format(time.RFC3339)}},
		"blocked": {{ID: "age-p", Title: "parked on human", Status: "blocked",
			UpdatedAt: reportTestNow.Add(-3 * time.Hour).Format(time.RFC3339)}},
		"open": {{ID: "age-b", Title: "still open", Status: "open"}},
	})

	out := runReport(t)
	for _, want := range []string{
		"YIELD",
		"ANDON QUEUE",
		"1 CONFIRMED",
		"1 REFUTED",
		"beads closed: 1",
		"age-c",
		"age-p",
		"blocked",
		"age-b",
		"stalled",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text report missing %q; output:\n%s", want, out)
		}
	}
}

// TestRunYieldReport_EmptyStates asserts honest empty-states: an empty ledger and
// empty tracker produce explicit "none"/"empty" wording, never blank sections.
func TestRunYieldReport_EmptyStates(t *testing.T) {
	root := t.TempDir()
	setYieldReportState(t, root)

	out := runReport(t)
	for _, want := range []string{
		"verdicts: none in window",
		"catches: none",
		"beads closed: none",
		"andon queue: empty — nothing parked",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("empty-state %q missing; output:\n%s", want, out)
		}
	}

	// JSON shape stays structured on empty: zero counts, empty (non-null) arrays.
	doc := decodeReport(t)
	if doc.Yield.Verdicts.Confirmed != 0 || doc.Yield.Verdicts.Refuted != 0 {
		t.Errorf("empty report verdicts = %+v, want zeros", doc.Yield.Verdicts)
	}
	if doc.Yield.Catches == nil || doc.Yield.ClosedBeads == nil || doc.AndonQueue == nil {
		t.Errorf("empty report arrays must be [] not null: %+v", doc)
	}
}

// TestRunYieldReport_SinceFlag asserts --since accepts a duration and an RFC3339
// instant, and rejects junk.
func TestRunYieldReport_SinceFlag(t *testing.T) {
	root := t.TempDir()
	setYieldReportState(t, root)
	seedReportVerdict(t, root, "age-a", yieldledger.DispositionConfirmed, "", "",
		reportTestNow.Add(-30*time.Hour))

	// 48h window catches the 30h-old verdict the default 24h window would drop.
	yieldReportSince = "48h"
	doc := decodeReport(t)
	if doc.Yield.Verdicts.Confirmed != 1 {
		t.Errorf("--since 48h confirmed = %d, want 1", doc.Yield.Verdicts.Confirmed)
	}

	yieldReportSince = reportTestNow.Add(-31 * time.Hour).Format(time.RFC3339)
	doc = decodeReport(t)
	if doc.Yield.Verdicts.Confirmed != 1 {
		t.Errorf("--since <RFC3339> confirmed = %d, want 1", doc.Yield.Verdicts.Confirmed)
	}

	yieldReportSince = "not-a-window"
	yieldReportCmd.SetOut(&bytes.Buffer{})
	t.Cleanup(func() { yieldReportCmd.SetOut(nil) }) // age-ztf8: shared cobra command — reset at the set-site
	if err := runYieldReport(yieldReportCmd, nil); err == nil {
		t.Errorf("junk --since must error")
	}
}

// TestParseReportSince is the table-driven L1 for the window parser.
func TestParseReportSince(t *testing.T) {
	now := reportTestNow
	cases := []struct {
		name    string
		raw     string
		want    time.Time
		wantErr bool
	}{
		{name: "default 24h", raw: "", want: now.Add(-24 * time.Hour)},
		{name: "duration", raw: "8h", want: now.Add(-8 * time.Hour)},
		{name: "minutes", raw: "90m", want: now.Add(-90 * time.Minute)},
		{name: "rfc3339", raw: "2026-07-08T00:00:00Z", want: time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)},
		{name: "junk", raw: "yesterdayish", wantErr: true},
		{name: "negative duration", raw: "-4h", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseReportSince(tc.raw, now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseReportSince(%q) = %v, want error", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseReportSince(%q): %v", tc.raw, err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("parseReportSince(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestFmtReportAge pins the compact age rendering the andon table uses.
func TestFmtReportAge(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "0m"},
		{45 * time.Minute, "45m"},
		{5 * time.Hour, "5h"},
		{26 * time.Hour, "26h"},
		{72 * time.Hour, "3d"},
		{-time.Hour, "0m"}, // clock skew never renders negative
	}
	for _, tc := range cases {
		if got := fmtReportAge(tc.d); got != tc.want {
			t.Errorf("fmtReportAge(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}
