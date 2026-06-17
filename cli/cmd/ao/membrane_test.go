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

	var buf bytes.Buffer
	membraneDeriveCmd.SetOut(&buf)
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
		var buf bytes.Buffer
		membraneDeriveCmd.SetOut(&buf)
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

	var buf bytes.Buffer
	membraneDeriveCmd.SetOut(&buf)
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
		var buf bytes.Buffer
		membraneDeriveCmd.SetOut(&buf)
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
		var buf bytes.Buffer
		membraneDeriveCmd.SetOut(&buf)
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
