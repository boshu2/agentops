package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// seedEscapeLedger writes a yield ledger under root containing one escape
// (age-flaky CONFIRMED@1 then REFUTED@2) plus a clean confirmed bead, using the
// production writer (round-trip fidelity).
func seedEscapeLedger(t *testing.T, root, run string) {
	t.Helper()
	w := yieldledger.Writer{}
	appendGV := func(bead, disp, sha string, attempt int, families []string) {
		if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
			BeadID:          bead,
			RunID:           run,
			TS:              time.Date(2026, 6, 17, 12, attempt, 0, 0, time.UTC),
			Difficulty:      1,
			PawlVerdictRef:  yieldledger.PawlVerdictRef{BeadID: bead, HeadSHA: sha},
			Disposition:     disp,
			HeadSHA:         sha,
			Attempt:         attempt,
			AuthorContextID: "ctx-" + bead,
			AuthorFamily:    "claude",
			RefuterFamilies: families,
		}); err != nil {
			t.Fatalf("append %s %s a%d: %v", disp, bead, attempt, err)
		}
	}
	appendGV("age-flaky", yieldledger.DispositionConfirmed, "abc1234def", 1, nil)
	appendGV("age-flaky", yieldledger.DispositionRefuted, "999feed888", 2, []string{"gemini"})
	appendGV("age-solid", yieldledger.DispositionConfirmed, "5550000111", 1, nil)
}

// captureMembraneDerive points the shared membraneDeriveCmd's writer at a fresh
// buffer and registers a t.Cleanup writer-reset (age-ztf8) so an unrestored writer
// can't leak into later -shuffle tests via cmd.OutOrStdout().
func captureMembraneDerive(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	membraneDeriveCmd.SetOut(&buf)
	t.Cleanup(func() { membraneDeriveCmd.SetOut(nil); membraneDeriveCmd.SetErr(nil) })
	return &buf
}

func TestMembraneDeriveChecks_WritesFindingAndCheck(t *testing.T) {
	root := t.TempDir()
	const run = "r-membrane-test"
	seedEscapeLedger(t, root, run)

	orig := testProjectDir
	testProjectDir = root
	defer func() { testProjectDir = orig }()

	membraneDeriveRun = run
	membraneDeriveDryRun = false
	membraneDeriveForce = false
	defer func() { membraneDeriveRun, membraneDeriveDryRun, membraneDeriveForce = "", false, false }()

	buf := captureMembraneDerive(t)
	if err := runMembraneDeriveChecks(membraneDeriveCmd, nil); err != nil {
		t.Fatalf("runMembraneDeriveChecks: %v", err)
	}

	// Exactly one escape → one finding, one pre-mortem-check; the clean bead is absent.
	// ID is keyed on run + bead + confirmed + refuted sha (collision-safe).
	findingID := "escape-age-flaky-" + run + "-abc1234def-999feed888"
	findingPath := filepath.Join(root, ".agents", "findings", findingID+".md")
	checkPath := filepath.Join(root, ".agents", "pre-mortem-checks", findingID+".md")

	findingBytes, err := os.ReadFile(findingPath)
	if err != nil {
		t.Fatalf("finding not written: %v", err)
	}
	finding := string(findingBytes)
	if !strings.Contains(finding, "source: \"escape\"") {
		t.Errorf("finding missing source:escape frontmatter:\n%s", finding)
	}
	if !strings.Contains(finding, "age-flaky") || !strings.Contains(finding, "fresh-context refuter") {
		t.Errorf("finding body missing escape detail / detection question:\n%s", finding)
	}

	checkBytes, err := os.ReadFile(checkPath)
	if err != nil {
		t.Fatalf("pre-mortem-check not written: %v", err)
	}
	check := string(checkBytes)
	if !strings.Contains(check, "Pre-Mortem Check") {
		t.Errorf("compiled check missing pre-mortem heading:\n%s", check)
	}
	if !strings.Contains(check, "fresh-context refuter") {
		t.Errorf("compiled check missing the derived detection question:\n%s", check)
	}

	// The clean bead must NOT have produced an artifact.
	if _, err := os.Stat(filepath.Join(root, ".agents", "findings", "escape-age-solid-5550000111.md")); !os.IsNotExist(err) {
		t.Errorf("clean confirmed bead must not yield an escape artifact")
	}

	out := buf.String()
	if !strings.Contains(out, "Derived 1 membrane check") {
		t.Errorf("report = %q, want it to announce 1 derived check", out)
	}
}

func TestMembraneDeriveChecks_Idempotent(t *testing.T) {
	root := t.TempDir()
	const run = "r-idem"
	seedEscapeLedger(t, root, run)

	orig := testProjectDir
	testProjectDir = root
	defer func() { testProjectDir = orig }()
	membraneDeriveRun = run
	defer func() { membraneDeriveRun, membraneDeriveDryRun, membraneDeriveForce = "", false, false }()

	run1 := func() string {
		buf := captureMembraneDerive(t)
		if err := runMembraneDeriveChecks(membraneDeriveCmd, nil); err != nil {
			t.Fatalf("run: %v", err)
		}
		return buf.String()
	}

	first := run1()
	if !strings.Contains(first, "[written]") {
		t.Errorf("first run should write: %q", first)
	}
	second := run1()
	if !strings.Contains(second, "exists, skipped") {
		t.Errorf("second run should skip existing (idempotent): %q", second)
	}
}

func TestMembraneDeriveChecks_DryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	const run = "r-dry"
	seedEscapeLedger(t, root, run)

	orig := testProjectDir
	testProjectDir = root
	defer func() { testProjectDir = orig }()
	membraneDeriveRun = run
	membraneDeriveDryRun = true
	defer func() { membraneDeriveRun, membraneDeriveDryRun, membraneDeriveForce = "", false, false }()

	buf := captureMembraneDerive(t)
	if err := runMembraneDeriveChecks(membraneDeriveCmd, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(buf.String(), "Would derive") {
		t.Errorf("dry-run report = %q, want 'Would derive'", buf.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "findings")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not create .agents/findings")
	}
}

func TestMembraneDeriveChecks_RequiresRun(t *testing.T) {
	membraneDeriveRun = ""
	defer func() { membraneDeriveRun = "" }()
	if err := runMembraneDeriveChecks(membraneDeriveCmd, nil); err == nil {
		t.Fatal("expected error when --run is empty")
	}
}

// TestMembraneDeriveChecks_RepairsMissingCheck guards the partial-artifact
// fail-quiet hole: if the finding exists but its compiled check was deleted, a
// re-run must REPAIR the missing check, not report it already-done.
func TestMembraneDeriveChecks_RepairsMissingCheck(t *testing.T) {
	root := t.TempDir()
	const run = "r-repair"
	seedEscapeLedger(t, root, run)

	orig := testProjectDir
	testProjectDir = root
	defer func() { testProjectDir = orig }()
	membraneDeriveRun = run
	defer func() { membraneDeriveRun, membraneDeriveDryRun, membraneDeriveForce = "", false, false }()

	run1 := func() string {
		buf := captureMembraneDerive(t)
		if err := runMembraneDeriveChecks(membraneDeriveCmd, nil); err != nil {
			t.Fatalf("run: %v", err)
		}
		return buf.String()
	}

	run1() // first derive writes finding + check
	findingID := "escape-age-flaky-" + run + "-abc1234def-999feed888"
	checkPath := filepath.Join(root, ".agents", "pre-mortem-checks", findingID+".md")

	// Delete the load-bearing compiled check; the finding stays.
	if err := os.Remove(checkPath); err != nil {
		t.Fatalf("remove check: %v", err)
	}
	out := run1() // re-run must repair, not skip
	if _, err := os.Stat(checkPath); err != nil {
		t.Fatalf("missing compiled check was NOT repaired on re-run: %v", err)
	}
	if !strings.Contains(out, "[written]") {
		t.Errorf("repair re-run should report a write, got: %q", out)
	}
}

// TestMembraneDeriveChecks_CrossRunNoCollision guards the ID-collision hole: the
// same bead confirmed at the same head sha in two distinct runs is two distinct
// escapes and must produce two distinct artifacts (the escape corpus must not
// under-count).
func TestMembraneDeriveChecks_CrossRunNoCollision(t *testing.T) {
	root := t.TempDir()
	w := yieldledger.Writer{}
	appendGV := func(run, bead, disp, sha string, attempt int, fams []string) {
		if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
			BeadID: bead, RunID: run, TS: time.Date(2026, 6, 17, 12, attempt, 0, 0, time.UTC),
			Difficulty: 1, PawlVerdictRef: yieldledger.PawlVerdictRef{BeadID: bead, HeadSHA: sha},
			Disposition: disp, HeadSHA: sha, Attempt: attempt,
			AuthorContextID: "ctx", AuthorFamily: "claude", RefuterFamilies: fams,
		}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	// Same bead, same CONFIRMED head sha, two runs — but refuted by different
	// reviewers at different heads. Two genuinely distinct escapes.
	appendGV("run-a", "age-dup", yieldledger.DispositionConfirmed, "samehead11", 1, nil)
	appendGV("run-a", "age-dup", yieldledger.DispositionRefuted, "refa000001", 2, []string{"gemini"})
	appendGV("run-b", "age-dup", yieldledger.DispositionConfirmed, "samehead11", 1, nil)
	appendGV("run-b", "age-dup", yieldledger.DispositionRefuted, "refb000002", 2, []string{"codex"})

	orig := testProjectDir
	testProjectDir = root
	defer func() { testProjectDir = orig }()
	defer func() { membraneDeriveRun, membraneDeriveDryRun, membraneDeriveForce = "", false, false }()

	for _, run := range []string{"run-a", "run-b"} {
		membraneDeriveRun = run
		captureMembraneDerive(t) // redirect (and cleanup) the shared writer; output unread here
		if err := runMembraneDeriveChecks(membraneDeriveCmd, nil); err != nil {
			t.Fatalf("run %s: %v", run, err)
		}
	}

	findingsDir := filepath.Join(root, ".agents", "findings")
	entries, err := os.ReadDir(findingsDir)
	if err != nil {
		t.Fatalf("read findings dir: %v", err)
	}
	if len(entries) != 2 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("cross-run escapes collided: got %d findings %v, want 2 distinct", len(entries), names)
	}
}

// Slice 3 (age-membrane-memory-j9c6.3): the derived finding (gold) carries the
// escape's domain + what-was-missed, in frontmatter and body, so it's a
// domain-tagged "look out for this here" check.
func TestDeriveFindingFromEscape_CarriesDomainAndMissed(t *testing.T) {
	a := deriveFindingFromEscape(yieldledger.Escape{
		BeadID: "age-racy", RunID: "r1",
		ConfirmedHeadSHA: "aaaaaaa1", ConfirmedAttempt: 1,
		RefutedHeadSHA: "bbbbbbb2", RefutedAttempt: 2,
		RefuterFamilies: []string{"codex"},
		Domain:          "concurrency",
		Missed:          "data race on a shared counter",
	})
	if a.Frontmatter["escape_domain"] != "concurrency" {
		t.Errorf("escape_domain = %q, want concurrency", a.Frontmatter["escape_domain"])
	}
	if a.Frontmatter["escape_missed"] != "data race on a shared counter" {
		t.Errorf("escape_missed = %q, want the missed reason", a.Frontmatter["escape_missed"])
	}
	if !strings.Contains(a.Body, "concurrency") || !strings.Contains(a.Body, "data race on a shared counter") {
		t.Errorf("finding body must surface domain + what-was-missed; got:\n%s", a.Body)
	}
	// domain and missed are INDEPENDENT optionals (a refuted reason is useful
	// even when the emitter didn't tag a domain). A truly-legacy escape (neither)
	// adds neither key; a missed-only escape adds only escape_missed.
	legacy := deriveFindingFromEscape(yieldledger.Escape{BeadID: "age-x", RunID: "r1", ConfirmedHeadSHA: "c1", RefutedHeadSHA: "r2"})
	if _, ok := legacy.Frontmatter["escape_domain"]; ok {
		t.Error("legacy escape (no domain) must not set escape_domain")
	}
	if _, ok := legacy.Frontmatter["escape_missed"]; ok {
		t.Error("legacy escape (no missed) must not set escape_missed")
	}
	missedOnly := deriveFindingFromEscape(yieldledger.Escape{BeadID: "age-m", RunID: "r1", ConfirmedHeadSHA: "c1", RefutedHeadSHA: "r2", Missed: "nil deref"})
	if _, ok := missedOnly.Frontmatter["escape_domain"]; ok {
		t.Error("missed-only escape must not set escape_domain")
	}
	if missedOnly.Frontmatter["escape_missed"] != "nil deref" {
		t.Error("missed-only escape should still set escape_missed (independent optional)")
	}
}

// Slice 4 (age-membrane-memory-j9c6.4): recall the membrane's memory for one
// domain — the consumption side ("look out for this here").
func TestRecallByDomain(t *testing.T) {
	root := t.TempDir()
	w := yieldledger.Writer{}
	emit := func(run, bead, disp, sha, domain string, attempt int) {
		t.Helper()
		if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
			BeadID: bead, RunID: run,
			TS:         time.Date(2026, 6, 19, 15, attempt, 0, 0, time.UTC),
			Difficulty: 1, PawlVerdictRef: yieldledger.PawlVerdictRef{BeadID: bead, HeadSHA: sha},
			Disposition: disp, HeadSHA: sha, Attempt: attempt,
			AuthorContextID: "ctx", AuthorFamily: "claude", Domain: domain,
		}); err != nil {
			t.Fatalf("emit: %v", err)
		}
	}
	emit("r1", "age-c", yieldledger.DispositionConfirmed, "c0nc1111", "concurrency", 1)
	emit("r1", "age-c", yieldledger.DispositionRefuted, "c0nc2222", "concurrency", 2)
	emit("r1", "age-d", yieldledger.DispositionConfirmed, "d0cs1111", "docs", 1)
	emit("r1", "age-d", yieldledger.DispositionRefuted, "d0cs2222", "docs", 2)

	got, err := recallByDomain(root, "concurrency")
	if err != nil {
		t.Fatalf("recallByDomain: %v", err)
	}
	if len(got) != 1 || got[0].BeadID != "age-c" || got[0].Domain != "concurrency" {
		t.Fatalf("recallByDomain(concurrency) = %+v, want [age-c@concurrency]", got)
	}
	if none, _ := recallByDomain(root, "nonexistent"); len(none) != 0 {
		t.Errorf("recallByDomain(nonexistent) = %+v, want empty", none)
	}
}

func TestRunMembraneRecall_TrimsDomain(t *testing.T) {
	root := t.TempDir()
	w := yieldledger.Writer{}
	emit := func(disp, sha string, attempt int) {
		t.Helper()
		if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
			BeadID: "age-c", RunID: "r1",
			TS:         time.Date(2026, 6, 19, 16, attempt, 0, 0, time.UTC),
			Difficulty: 1, PawlVerdictRef: yieldledger.PawlVerdictRef{BeadID: "age-c", HeadSHA: sha},
			Disposition: disp, HeadSHA: sha, Attempt: attempt,
			AuthorContextID: "ctx", AuthorFamily: "claude", Domain: "concurrency",
		}); err != nil {
			t.Fatalf("emit: %v", err)
		}
	}
	emit(yieldledger.DispositionConfirmed, "c0nc1111", 1)
	emit(yieldledger.DispositionRefuted, "c0nc2222", 2)

	origProjectDir := testProjectDir
	testProjectDir = root
	t.Cleanup(func() { testProjectDir = origProjectDir })

	membraneRecallDomain = " concurrency "
	membraneRecallJSON = false
	t.Cleanup(func() { membraneRecallDomain, membraneRecallJSON = "", false })

	var buf bytes.Buffer
	membraneRecallCmd.SetOut(&buf)
	t.Cleanup(func() { membraneRecallCmd.SetOut(nil) })
	if err := runMembraneRecall(membraneRecallCmd, nil); err != nil {
		t.Fatalf("runMembraneRecall: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "1 past escape") || !strings.Contains(out, `domain "concurrency"`) {
		t.Fatalf("recall output = %q, want trimmed concurrency hit", out)
	}
	if strings.Contains(out, `" concurrency "`) {
		t.Fatalf("recall output used untrimmed domain: %q", out)
	}
}
