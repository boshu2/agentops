package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/domainsignal"
	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/search"
	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// A mechanical finding's compiled constraint must MERGE into the root
// .agents/constraints/index.json as a warn-only shadow entry rather
// than land as a per-id markdown file — the producer↔gate seam of EM-ENF.
func TestWriteDerivedArtifacts_MergesConstraintIntoIndex(t *testing.T) {
	root := t.TempDir()
	outs, err := newProductionFindingCompiler().Compile(context.Background(), ports.FindingArtifact{
		ID: "f-merge",
		Frontmatter: map[string]string{
			"compiler_targets":      "constraint",
			"detectability":         "mechanical",
			"detector_kind":         "regex",
			"detector_pattern":      "panic\\(",
			"constraint_path_globs": "cli/**",
			"compiled_at":           "2026-06-21T00:00:00Z",
		},
		Body:             "x",
		DetectorEvidence: loadDetectorEvidenceFixtureFromRepo(t, "replay-pass.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	wrote, err := writeDerivedArtifacts(root, ports.FindingArtifact{ID: "f-merge"}, outs, false)
	if err != nil || !wrote {
		t.Fatalf("writeDerivedArtifacts: wrote=%v err=%v", wrote, err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".agents", "constraints", "index.json"))
	if err != nil {
		t.Fatalf("index.json not written: %v", err)
	}
	var idx search.ConstraintIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("index.json malformed: %v", err)
	}
	if len(idx.Constraints) != 1 {
		t.Fatalf("want 1 constraint, got %d", len(idx.Constraints))
	}
	e := idx.Constraints[0]
	if e.ID != "f-merge" || e.Status != "shadow" || e.EnforcementMode != "warn" || e.Detector.Pattern != "panic\\(" {
		t.Fatalf("merged entry = %+v, want f-merge/shadow/warn/panic\\(", e)
	}
	// No per-id markdown constraint file should exist (the old dead artifact).
	if _, err := os.Stat(filepath.Join(root, ".agents", "constraints", "f-merge.md")); err == nil {
		t.Fatalf("dead per-id constraint markdown was written")
	}
}

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

func writeDetectorEvidenceFixture(t *testing.T, root, name string) string {
	t.Helper()
	return writeDetectorEvidence(t, root, name, loadDetectorEvidenceFixtureFromRepo(t, name))
}

func writeDetectorEvidence(t *testing.T, root, name string, evidence *ports.DetectorEvidence) string {
	t.Helper()
	raw, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
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

	// Exactly one escape → one finding, one premortem check; the clean bead is absent.
	// ID is keyed on run + bead + confirmed + refuted sha (collision-safe).
	findingID := "escape-age-flaky-" + run + "-abc1234def-999feed888"
	findingPath := filepath.Join(root, ".agents", "findings", findingID+".md")
	checkPath := filepath.Join(root, ".agents", "premortem-checks", findingID+".md")

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
		t.Fatalf("premortem check not written: %v", err)
	}
	check := string(checkBytes)
	if !strings.Contains(check, "Premortem Check") {
		t.Errorf("compiled check missing premortem heading:\n%s", check)
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

// EM.2.10 ACCEPTANCE: a MECHANICAL escape, run through `ao membrane derive-checks`,
// lands a warn-only shadow ConstraintEntry in .agents/constraints/index.json
// the whole direction was unblocking). This is the product-path proof at the
// command level: the loop's compile half fires end-to-end on a real ledger input.
func TestMembraneDeriveChecks_MechanicalEscape_WritesConstraint(t *testing.T) {
	root := t.TempDir()
	const run = "r-mech-e2e"
	w := yieldledger.Writer{}
	mk := func(disp, sha string, attempt int, in yieldledger.GateVerdictInput) {
		in.BeadID, in.RunID, in.Disposition, in.HeadSHA, in.Attempt = "age-mech", run, disp, sha, attempt
		in.TS = time.Date(2026, 6, 22, 12, attempt, 0, 0, time.UTC)
		in.Difficulty = 1
		in.PawlVerdictRef = yieldledger.PawlVerdictRef{BeadID: "age-mech", HeadSHA: sha}
		in.AuthorContextID = "ctx"
		in.AuthorFamily = "claude"
		if _, err := w.AppendGateVerdict(root, in); err != nil {
			t.Fatalf("append %s: %v", disp, err)
		}
	}
	mk(yieldledger.DispositionConfirmed, "aaa1111aaa", 1, yieldledger.GateVerdictInput{})
	mk(yieldledger.DispositionRefuted, "bbb2222bbb", 2, yieldledger.GateVerdictInput{
		Domain: "validation", Reason: "forbidden eval()",
		DetectorPattern: `\beval\(`, ConstraintPathGlobs: "cli/**", DetectorKind: "regex",
	})

	orig := testProjectDir
	testProjectDir = root
	defer func() { testProjectDir = orig }()
	membraneDeriveRun = run
	membraneDeriveEvidence = writeDetectorEvidence(t, root, "eval-replay.json", &ports.DetectorEvidence{
		PositiveFixtures: []ports.DetectorFixture{{Ref: "positive/eval.go", Content: `func bad() { eval("x") }`}},
		NegativeControls: []ports.DetectorFixture{{Ref: "negative/safe.go", Content: `func safe() { parse("x") }`}},
	})
	defer func() {
		membraneDeriveRun, membraneDeriveEvidence, membraneDeriveDryRun, membraneDeriveForce = "", "", false, false
	}()

	captureMembraneDerive(t)
	if err := runMembraneDeriveChecks(membraneDeriveCmd, nil); err != nil {
		t.Fatalf("derive-checks: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".agents", "constraints", "index.json"))
	if err != nil {
		t.Fatalf("index.json not written — the loop did not fire: %v", err)
	}
	var idx search.ConstraintIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("index.json malformed: %v", err)
	}
	if len(idx.Constraints) != 1 {
		t.Fatalf("mechanical escape must yield exactly 1 shadow constraint, got %d", len(idx.Constraints))
	}
	c := idx.Constraints[0]
	if c.Status != "shadow" || c.EnforcementMode != "warn" || c.Detector.Pattern != `\beval\(` || len(c.AppliesTo.PathGlobs) == 0 {
		t.Fatalf("constraint not compiled from the escape's detector: %+v", c)
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
	checkPath := filepath.Join(root, ".agents", "premortem-checks", findingID+".md")

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
	}, domainsignal.Record{})
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
	legacy := deriveFindingFromEscape(yieldledger.Escape{BeadID: "age-x", RunID: "r1", ConfirmedHeadSHA: "c1", RefutedHeadSHA: "r2"}, domainsignal.Record{})
	if _, ok := legacy.Frontmatter["escape_domain"]; ok {
		t.Error("legacy escape (no domain) must not set escape_domain")
	}
	if _, ok := legacy.Frontmatter["escape_missed"]; ok {
		t.Error("legacy escape (no missed) must not set escape_missed")
	}
	missedOnly := deriveFindingFromEscape(yieldledger.Escape{BeadID: "age-m", RunID: "r1", ConfirmedHeadSHA: "c1", RefutedHeadSHA: "r2", Missed: "nil deref"}, domainsignal.Record{})
	if _, ok := missedOnly.Frontmatter["escape_domain"]; ok {
		t.Error("missed-only escape must not set escape_domain")
	}
	if missedOnly.Frontmatter["escape_missed"] != "nil deref" {
		t.Error("missed-only escape should still set escape_missed (independent optional)")
	}
}

// EM.2.1: the UNCLASSIFIED domain / unspecified reason sentinels are visible DEBT,
// never real signal — the derived finding must flag them for classification, not
// render them as a routable "look out for this here" / "what was missed".
func TestDeriveFindingFromEscape_SentinelsRenderAsDebt(t *testing.T) {
	a := deriveFindingFromEscape(yieldledger.Escape{
		BeadID: "age-unc", RunID: "r1",
		ConfirmedHeadSHA: "aaaaaaa1", ConfirmedAttempt: 1,
		RefutedHeadSHA: "bbbbbbb2", RefutedAttempt: 2,
		Domain: yieldledger.DomainUnclassified,
		Missed: yieldledger.ReasonUnspecified,
	}, domainsignal.Record{})
	// The body must call out the debt, NOT present the placeholders as real signal.
	if !strings.Contains(a.Body, "UNCLASSIFIED") || !strings.Contains(a.Body, "never classified") {
		t.Errorf("UNCLASSIFIED domain must render as classification debt; got:\n%s", a.Body)
	}
	if strings.Contains(a.Body, "look out for this class of miss when working here") {
		t.Errorf("UNCLASSIFIED must NOT render as a routable domain signal; got:\n%s", a.Body)
	}
	if !strings.Contains(a.Body, "unspecified") || !strings.Contains(a.Body, "set --reason") {
		t.Errorf("unspecified reason must render as classification debt; got:\n%s", a.Body)
	}
}

// EM.2.2: the three-signal domain record renders intent + changed-file domains and,
// when they disagree, a DOMAIN MISMATCH note — all queryable via frontmatter.
func TestDeriveFindingFromEscape_ThreeSignalRecord(t *testing.T) {
	rec := domainsignal.Build(
		domainsignal.BC2Validation,                             // intended BC2
		[]string{"cli/cmd/ao/x.go", "cli/internal/swarm/y.go"}, // code in BC5 + BC6
		"concurrency", // escape domain (free text)
	)
	if !rec.Mismatch {
		t.Fatalf("precondition: BC2 intent vs BC5/BC6 changes must mismatch; %+v", rec)
	}
	a := deriveFindingFromEscape(yieldledger.Escape{
		BeadID: "age-xdom", RunID: "r1",
		ConfirmedHeadSHA: "aaaaaaa1", RefutedHeadSHA: "bbbbbbb2",
		Domain: "concurrency",
	}, rec)
	if a.Frontmatter["intent_domain"] != domainsignal.BC2Validation {
		t.Errorf("intent_domain frontmatter = %q", a.Frontmatter["intent_domain"])
	}
	if a.Frontmatter["changed_file_domains"] != "BC5 Runtime, BC6 Orchestration" {
		t.Errorf("changed_file_domains frontmatter = %q", a.Frontmatter["changed_file_domains"])
	}
	if a.Frontmatter["domain_mismatch"] != "true" {
		t.Errorf("domain_mismatch frontmatter = %q, want true", a.Frontmatter["domain_mismatch"])
	}
	if !strings.Contains(a.Body, "DOMAIN MISMATCH") || !strings.Contains(a.Body, "crossed bounded contexts") {
		t.Errorf("body must surface the cross-context mismatch; got:\n%s", a.Body)
	}
	// A no-signal record renders no domain-signals block (graceful degrade).
	b := deriveFindingFromEscape(yieldledger.Escape{BeadID: "age-y", RunID: "r1", ConfirmedHeadSHA: "c", RefutedHeadSHA: "r"}, domainsignal.Record{})
	if strings.Contains(b.Body, "Domain signals:") {
		t.Errorf("empty record must not render a domain-signals block; got:\n%s", b.Body)
	}
}

// EM.2.10 — THE CUT WIRE, reconnected. A MECHANICAL escape (one carrying a
// detector pattern + path globs) must now compile, through the SHARED
// search.BuildConstraintEntry contract, into a warn-only shadow constraint.
func TestDeriveFindingFromEscape_MechanicalEscape_CompilesToConstraint(t *testing.T) {
	a := deriveFindingFromEscape(yieldledger.Escape{
		BeadID: "age-mech", RunID: "r1",
		ConfirmedHeadSHA: "aaaaaaa1", ConfirmedAttempt: 1, ConfirmedTS: "2026-06-22T10:00:00Z",
		RefutedHeadSHA: "bbbbbbb2", RefutedAttempt: 2, RefutedTS: "2026-06-22T11:00:00Z",
		Domain: "validation",
		Missed: "unanchored substring rule misclassified aggregate as a gate",
		// the re-introducible pattern + where it applies:
		DetectorPattern:     `contains:\s*"(gate|pawl)"`,
		ConstraintPathGlobs: "cli/internal/domainsignal/**",
	}, domainsignal.Record{})

	// Frontmatter is upgraded to mechanical with the constraint compile target.
	if a.Frontmatter["detectability"] != "mechanical" {
		t.Fatalf("mechanical escape must set detectability=mechanical, got %q", a.Frontmatter["detectability"])
	}
	if !strings.Contains(a.Frontmatter["compiler_targets"], "constraint") {
		t.Fatalf("compiler_targets must include constraint, got %q", a.Frontmatter["compiler_targets"])
	}
	// THE PROOF: deterministic replay compiles through the shared contract.
	replay := ports.DetectorReplayResult{PositiveRefs: []string{"positive"}, NegativeControlRefs: []string{"negative"}}
	entry, ok := search.BuildConstraintEntry(a.ID, a.Frontmatter, replay)
	if !ok {
		t.Fatalf("mechanical finding must compile to a constraint via BuildConstraintEntry; frontmatter=%v", a.Frontmatter)
	}
	if entry.Status != "shadow" || entry.EnforcementMode != "warn" {
		t.Errorf("derived constraint must be warn-only shadow, got %q/%q", entry.Status, entry.EnforcementMode)
	}
	if entry.Detector.Pattern != `contains:\s*"(gate|pawl)"` {
		t.Errorf("detector pattern not carried through: %q", entry.Detector.Pattern)
	}
	if len(entry.AppliesTo.PathGlobs) == 0 || entry.AppliesTo.PathGlobs[0] != "cli/internal/domainsignal/**" {
		t.Errorf("path globs not carried through: %v", entry.AppliesTo.PathGlobs)
	}
	if entry.CompiledAt != "2026-06-22T11:00:00Z" {
		t.Errorf("compiled_at should be the refuted (catch) TS, got %q", entry.CompiledAt)
	}
}

// A process-gap escape (no detector) stays ADVISORY and compiles to NO constraint —
// the membrane remembers it (pre-mortem note) but cannot mechanically block it.
func TestDeriveFindingFromEscape_ProcessGapEscape_StaysAdvisory(t *testing.T) {
	a := deriveFindingFromEscape(yieldledger.Escape{
		BeadID: "age-proc", RunID: "r1",
		ConfirmedHeadSHA: "c1", RefutedHeadSHA: "r2",
		Domain: "validation", Missed: "missing fresh-context re-verification",
	}, domainsignal.Record{})
	if a.Frontmatter["detectability"] != "advisory" {
		t.Fatalf("process-gap escape must stay advisory, got %q", a.Frontmatter["detectability"])
	}
	if _, ok := search.BuildConstraintEntry(a.ID, a.Frontmatter, ports.DetectorReplayResult{}); ok {
		t.Fatal("an advisory escape must NOT compile to a constraint")
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

// `ao membrane catch` records a judgment-class catch (no detector) that is
// path-recallable and carries a v1 class_key. (epic age-zpj5, S2)
func TestMembraneCatch_RecordsPathRecallableJudgmentClass(t *testing.T) {
	root := t.TempDir()
	in := buildCatchInput("age-x", "pawl", "content-pattern key-injection fail-open",
		[]string{"scripts/pawl.sh"}, "", "", "", "", "abcdef0", "",
		time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC))
	w := yieldledger.Writer{}
	if _, err := w.AppendGateVerdict(root, in); err != nil {
		t.Fatalf("emit: %v", err)
	}
	l, err := yieldledger.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	catches := yieldledger.DetectCatches(l)
	if len(catches) != 1 {
		t.Fatalf("want 1 catch class, got %d", len(catches))
	}
	c := catches[0]
	if c.Domain != "pawl" || c.Reason == "" {
		t.Fatalf("catch missing domain/reason: %+v", c)
	}
	if len(c.AffectedPaths) != 1 || c.AffectedPaths[0] != "scripts/pawl.sh" {
		t.Fatalf("catch must be path-recallable, got %v", c.AffectedPaths)
	}
	if !strings.HasPrefix(c.ClassKey, "v1:") {
		t.Fatalf("catch must carry a v1 class_key, got %q", c.ClassKey)
	}
	if cc := yieldledger.CompileCandidates(catches); len(cc) != 0 {
		t.Fatalf("a judgment-class catch (no detector) must NOT be a compile candidate; got %d", len(cc))
	}
}

// A detector-bearing catch becomes a compile candidate. (epic age-zpj5, S2)
func TestMembraneCatch_DetectorMakesCompileCandidate(t *testing.T) {
	root := t.TempDir()
	in := buildCatchInput("age-y", "shell", "unguarded cmdsub aborts under set -e",
		[]string{"scripts/x.sh"}, "assign-cmdsub-no-guard", "scripts/**", "regex", "deterministic", "abcdef0", "",
		time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC))
	w := yieldledger.Writer{}
	if _, err := w.AppendGateVerdict(root, in); err != nil {
		t.Fatalf("emit: %v", err)
	}
	l, _ := yieldledger.Load(root)
	cc := yieldledger.CompileCandidates(yieldledger.DetectCatches(l))
	if len(cc) != 1 || cc[0].DetectorPattern != "assign-cmdsub-no-guard" {
		t.Fatalf("detector-bearing catch must be a compile candidate; got %+v", cc)
	}
}

// `ao membrane catch --class <slug>` persists the semantic class and keys the catch
// CROSS-BEAD; an invalid slug is rejected before any write. Cobra globals restored via
// t.Cleanup (shared rootCmd/package flag vars — .claude/rules/go.md). (age-jjt8)
func TestMembraneCatch_SemanticClassFlag(t *testing.T) {
	root := t.TempDir()
	origProjectDir := testProjectDir
	testProjectDir = root
	t.Cleanup(func() { testProjectDir = origProjectDir })

	ob, od, orr, ocl, ohd := membraneCatchBead, membraneCatchDomain, membraneCatchReason, membraneCatchClass, membraneCatchHead
	t.Cleanup(func() {
		membraneCatchBead, membraneCatchDomain, membraneCatchReason, membraneCatchClass, membraneCatchHead = ob, od, orr, ocl, ohd
	})
	membraneCatchCmd.SetOut(&bytes.Buffer{})
	t.Cleanup(func() { membraneCatchCmd.SetOut(nil) })

	// Invalid slug -> error BEFORE any write.
	membraneCatchBead, membraneCatchDomain, membraneCatchReason = "age-x", "scripts", "r"
	membraneCatchHead = "abcdef0123"
	membraneCatchClass = "Not A Slug"
	if err := runMembraneCatch(membraneCatchCmd, nil); err == nil {
		t.Fatal("invalid --class slug must be rejected")
	}
	if l, _ := yieldledger.Load(root); l != nil && len(l.Events) != 0 {
		t.Fatalf("a rejected --class must NOT write a catch; got %d events", len(l.Events))
	}

	// Valid slug on two DIFFERENT beads with DIFFERENT reasons -> ONE cross-bead class.
	membraneCatchClass = "stale-retired-surface"
	membraneCatchBead, membraneCatchReason = "age-1", "reason wording one"
	if err := runMembraneCatch(membraneCatchCmd, nil); err != nil {
		t.Fatalf("valid catch #1: %v", err)
	}
	membraneCatchBead, membraneCatchReason = "age-2", "totally different wording two"
	if err := runMembraneCatch(membraneCatchCmd, nil); err != nil {
		t.Fatalf("valid catch #2: %v", err)
	}
	l, err := yieldledger.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	catches := yieldledger.DetectCatches(l)
	if len(catches) != 1 || catches[0].ClassKey != "v1:scripts/stale-retired-surface" || catches[0].HitCount != 2 {
		t.Fatalf("two --class catches on different beads must be ONE cross-bead class; got %#v", catches)
	}
}

// Catch-keyed recall filters by domain and (when paths given) affected_paths overlap. (S3)
func TestRecallCatchesByDomain(t *testing.T) {
	root := t.TempDir()
	w := yieldledger.Writer{}
	mk := func(bead, domain, reason string, paths []string) {
		t.Helper()
		if _, err := w.AppendGateVerdict(root, buildCatchInput(bead, domain, reason, paths, "", "", "", "", "abcdef0", "",
			time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC))); err != nil {
			t.Fatalf("emit: %v", err)
		}
	}
	mk("age-a", "pawl", "key-injection fail-open", []string{"scripts/pawl.sh"})
	mk("age-b", "shell", "unguarded cmdsub", []string{"scripts/x.sh"})

	pawl, err := recallCatchesByDomain(root, "pawl", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pawl) != 1 || pawl[0].Domain != "pawl" {
		t.Fatalf("domain filter: want 1 pawl catch, got %+v", pawl)
	}
	if hit, _ := recallCatchesByDomain(root, "pawl", []string{"scripts/pawl.sh", "other.go"}); len(hit) != 1 {
		t.Fatalf("overlapping path should match, got %d", len(hit))
	}
	if miss, _ := recallCatchesByDomain(root, "pawl", []string{"unrelated.go"}); len(miss) != 0 {
		t.Fatalf("non-overlapping paths should NOT match, got %d", len(miss))
	}
}

// The S3 invariant (advisory negative test): the recall functions must NEVER be
// called from any gate / pass / verdict-deciding path — recall is advisory MEMORY,
// not a gate input. Statically scan the package source: the only legitimate caller
// is runMembraneRecall (the advisory command). (epic age-zpj5, S3)
func TestRecall_IsAdvisoryNeverAGate(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	callRe := regexp.MustCompile(`\brecall(ByDomain|CatchesByDomain)\(`)
	defRe := regexp.MustCompile(`^func\s+recall(ByDomain|CatchesByDomain)\b`)
	fnRe := regexp.MustCompile(`^func\s+(\w+)`)
	callSites := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		cur := ""
		for _, line := range strings.Split(string(src), "\n") {
			if m := fnRe.FindStringSubmatch(line); m != nil {
				cur = m[1]
			}
			if defRe.MatchString(line) {
				continue // the definition line, not a call
			}
			if callRe.MatchString(line) {
				callSites++
				if cur != "runMembraneRecall" {
					t.Errorf("%s: recall is called from %q — recall is ADVISORY memory and must NEVER be a gate/verdict/pass caller (S3 invariant)", f, cur)
				}
			}
		}
	}
	if callSites == 0 {
		t.Fatal("found 0 recall call sites — the scan isn't matching, so this invariant would pass vacuously")
	}
}

// `ao membrane triage` on a below-floor corpus emits the HONEST INSUFFICIENT-DATA
// verdict (the PROVE-FIRST point: do not build the compiler on faith). (epic age-zpj5, S4)
func TestRunMembraneTriage_InsufficientDataBelowFloor(t *testing.T) {
	root := t.TempDir()
	origProjectDir := testProjectDir
	testProjectDir = root
	t.Cleanup(func() { testProjectDir = origProjectDir })

	var buf bytes.Buffer
	membraneTriageCmd.SetOut(&buf)
	t.Cleanup(func() { membraneTriageCmd.SetOut(nil) })
	if err := runMembraneTriage(membraneTriageCmd, nil); err != nil {
		t.Fatalf("runMembraneTriage: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "DECISION: INSUFFICIENT-DATA") {
		t.Fatalf("an empty/below-floor corpus must triage to INSUFFICIENT-DATA, got:\n%s", out)
	}
	if !strings.Contains(out, "do NOT build the compiler on faith") {
		t.Fatalf("INSUFFICIENT-DATA must explain the honest stance, got:\n%s", out)
	}
}

// ── age-ulab: --evidence reason-extraction + domain/paths resolution ─────────
// These port emit_pawl_catch's bash two-tier salvage into Go; the fixtures are
// the exact evidence shapes pawl-review produces (sentinel, routed multi-family,
// prose finding, neither).

// extractRefutedReason applies the two-tier REFUTED reason salvage: tier 1 is
// the last `VERDICT: REFUTED <text>` sentinel line (prefix stripped, 200-char
// cap); tier 2 (when tier 1 yields nothing) is the first substantive
// `REFUTED: <finding>` prose line that is neither a routed `PAWL <nonce>
// REFUTED` sentinel nor a VERDICT line. Empty when neither tier hits.
func TestExtractRefutedReason_TwoTierSalvage(t *testing.T) {
	long := strings.Repeat("x", 250)
	cases := []struct {
		name     string
		evidence string
		want     string
	}{
		{
			name:     "tier1 single-reviewer sentinel with trailing reason",
			evidence: "Reviewed the diff.\nVERDICT: REFUTED — the gate fails open on empty input\n",
			want:     "the gate fails open on empty input",
		},
		{
			name:     "tier1 last sentinel wins over earlier ones",
			evidence: "VERDICT: REFUTED first finding\nsome analysis\nVERDICT: REFUTED second finding\n",
			want:     "second finding",
		},
		{
			name:     "tier1 colon separator and indentation tolerated, case-insensitive",
			evidence: "  verdict: refuted: The Gate Lies\n",
			want:     "The Gate Lies",
		},
		{
			name:     "tier1 caps the reason at 200 chars",
			evidence: "VERDICT: REFUTED " + long + "\n",
			want:     long[:200],
		},
		{
			name: "tier2 multi-family prose finding (routed sentinel carries no reason)",
			evidence: "family codex says:\n" +
				"REFUTED: the audit fails open when the ledger is missing\n" +
				"PAWL r123 REFUTED\n",
			want: "the audit fails open when the ledger is missing",
		},
		{
			name: "tier2 skips routed sentinels and VERDICT lines",
			evidence: "PAWL rdeadbeef REFUTED\n" +
				"the final VERDICT: REFUTED: not this one\n" +
				"REFUTED: silent error swallowing in the retry loop\n",
			want: "silent error swallowing in the retry loop",
		},
		{
			name:     "tier2 strips greedily through the last REFUTED marker (bash sed parity)",
			evidence: "codex REFUTED: the check REFUTED: twice\n",
			want:     "twice",
		},
		{
			name:     "bare sentinel with no trailing text falls through to tier2 prose",
			evidence: "VERDICT: REFUTED\nREFUTED: the embedded copy drifted from source\n",
			want:     "the embedded copy drifted from source",
		},
		{
			name:     "neither tier hits -> empty (caller applies the placeholder)",
			evidence: "Reviewed. All checks pass.\nVERDICT: CONFIRMED\n",
			want:     "",
		},
		{
			name:     "empty evidence -> empty",
			evidence: "",
			want:     "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractRefutedReason(tc.evidence); got != tc.want {
				t.Fatalf("extractRefutedReason() = %q, want %q", got, tc.want)
			}
		})
	}
}

// makeCatchEvidenceRepo builds a temp git repo with committed files under two
// top dirs and returns (root, headSHA). The commit shape drives domain-from-
// top-dir + paths-from-git resolution.
func makeCatchEvidenceRepo(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	root := realPathForTest(t, t.TempDir())
	initHistoryFixtureGitRepo(t, root)
	for name, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runFixtureGit(t, root, nil, "add", "-A")
	runFixtureGit(t, root, nil, "commit", "-m", "fixture")
	head := strings.TrimSpace(runFixtureGit(t, root, nil, "rev-parse", "HEAD"))
	return root, head
}

// setMembraneCatchFlags sets the shared cobra flag globals for one
// runMembraneCatch invocation and restores ALL of them via t.Cleanup
// (.claude/rules/go.md test-isolation rule).
func setMembraneCatchFlags(t *testing.T, set func()) {
	t.Helper()
	ob, od, orr, ocl := membraneCatchBead, membraneCatchDomain, membraneCatchReason, membraneCatchClass
	op, odet, ohd, omo := membraneCatchPaths, membraneCatchDetector, membraneCatchHead, membraneCatchMode
	oev, osc, obase := membraneCatchEvidence, membraneCatchScope, membraneCatchBase
	t.Cleanup(func() {
		membraneCatchBead, membraneCatchDomain, membraneCatchReason, membraneCatchClass = ob, od, orr, ocl
		membraneCatchPaths, membraneCatchDetector, membraneCatchHead, membraneCatchMode = op, odet, ohd, omo
		membraneCatchEvidence, membraneCatchScope, membraneCatchBase = oev, osc, obase
	})
	membraneCatchBead, membraneCatchDomain, membraneCatchReason, membraneCatchClass = "", "", "", ""
	membraneCatchPaths, membraneCatchDetector, membraneCatchHead, membraneCatchMode = nil, "", "", ""
	membraneCatchEvidence, membraneCatchScope, membraneCatchBase = "", "head", ""
	membraneCatchCmd.SetOut(&bytes.Buffer{})
	t.Cleanup(func() { membraneCatchCmd.SetOut(nil) })
	set()
}

// `ao membrane catch --evidence` extracts the reason from the evidence file and
// resolves domain (first path component of the first changed file) + affected
// paths (changed files) from git at --head. (age-ulab)
func TestMembraneCatch_EvidenceExtractsReasonDomainPaths(t *testing.T) {
	root, head := makeCatchEvidenceRepo(t, map[string]string{
		"scripts/a.sh": "echo a\n",
		"scripts/b.sh": "echo b\n",
	})
	evidence := filepath.Join(root, "evidence.txt")
	if err := os.WriteFile(evidence, []byte("Reviewed.\nVERDICT: REFUTED — reads the wrong ledger path\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	origProjectDir := testProjectDir
	testProjectDir = root
	t.Cleanup(func() { testProjectDir = origProjectDir })

	setMembraneCatchFlags(t, func() {
		membraneCatchBead = "age-ulab-t1"
		membraneCatchEvidence = evidence
		membraneCatchHead = head
	})
	if err := runMembraneCatch(membraneCatchCmd, nil); err != nil {
		t.Fatalf("runMembraneCatch: %v", err)
	}
	l, err := yieldledger.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	catches := yieldledger.DetectCatches(l)
	if len(catches) != 1 {
		t.Fatalf("want 1 catch class, got %d", len(catches))
	}
	c := catches[0]
	if c.Reason != "reads the wrong ledger path" {
		t.Fatalf("reason = %q, want the extracted sentinel text", c.Reason)
	}
	if c.Domain != "scripts" {
		t.Fatalf("domain = %q, want first path component of first changed file (scripts)", c.Domain)
	}
	wantPaths := []string{"scripts/a.sh", "scripts/b.sh"}
	if len(c.AffectedPaths) != len(wantPaths) || c.AffectedPaths[0] != wantPaths[0] || c.AffectedPaths[1] != wantPaths[1] {
		t.Fatalf("affected paths = %v, want %v", c.AffectedPaths, wantPaths)
	}
}

// Multi-family evidence: the routed `PAWL <nonce> REFUTED` sentinel carries no
// reason — the substantive `REFUTED: <finding>` prose line is salvaged. (age-9931 parity)
func TestMembraneCatch_EvidenceMultiFamilyProseSalvage(t *testing.T) {
	root, head := makeCatchEvidenceRepo(t, map[string]string{"cli/x.go": "package x\n"})
	evidence := filepath.Join(root, "evidence.txt")
	body := "family gemini:\nREFUTED: the audit fails open when the head sha is unbound\nPAWL r123 REFUTED\n"
	if err := os.WriteFile(evidence, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	origProjectDir := testProjectDir
	testProjectDir = root
	t.Cleanup(func() { testProjectDir = origProjectDir })

	setMembraneCatchFlags(t, func() {
		membraneCatchBead = "age-ulab-t2"
		membraneCatchEvidence = evidence
		membraneCatchHead = head
	})
	if err := runMembraneCatch(membraneCatchCmd, nil); err != nil {
		t.Fatalf("runMembraneCatch: %v", err)
	}
	l, _ := yieldledger.Load(root)
	catches := yieldledger.DetectCatches(l)
	if len(catches) != 1 || catches[0].Reason != "the audit fails open when the head sha is unbound" {
		t.Fatalf("multi-family prose salvage failed; got %+v", catches)
	}
	if catches[0].Domain != "cli" {
		t.Fatalf("domain = %q, want cli", catches[0].Domain)
	}
}

// Evidence with neither tier -> the placeholder reason (never an empty reason,
// which would land in the UNCLASSIFIED floor with no text at all).
func TestMembraneCatch_EvidencePlaceholderWhenNoRefutedText(t *testing.T) {
	root, head := makeCatchEvidenceRepo(t, map[string]string{"docs/y.md": "hi\n"})
	evidence := filepath.Join(root, "evidence.txt")
	if err := os.WriteFile(evidence, []byte("clean review, nothing refuted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	origProjectDir := testProjectDir
	testProjectDir = root
	t.Cleanup(func() { testProjectDir = origProjectDir })

	setMembraneCatchFlags(t, func() {
		membraneCatchBead = "age-ulab-t3"
		membraneCatchEvidence = evidence
		membraneCatchHead = head
	})
	if err := runMembraneCatch(membraneCatchCmd, nil); err != nil {
		t.Fatalf("runMembraneCatch: %v", err)
	}
	l, _ := yieldledger.Load(root)
	catches := yieldledger.DetectCatches(l)
	if len(catches) != 1 || catches[0].Reason != "pawl-review REFUTED (see evidence)" {
		t.Fatalf("want placeholder reason, got %+v", catches)
	}
}

// A caller-supplied --reason (and --domain/--paths) wins over extraction —
// explicit orchestrator intent beats the salvage heuristics.
func TestMembraneCatch_EvidenceCallerFlagsWin(t *testing.T) {
	root, head := makeCatchEvidenceRepo(t, map[string]string{"scripts/z.sh": "echo z\n"})
	evidence := filepath.Join(root, "evidence.txt")
	if err := os.WriteFile(evidence, []byte("VERDICT: REFUTED — extracted text must lose\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	origProjectDir := testProjectDir
	testProjectDir = root
	t.Cleanup(func() { testProjectDir = origProjectDir })

	setMembraneCatchFlags(t, func() {
		membraneCatchBead = "age-ulab-t4"
		membraneCatchEvidence = evidence
		membraneCatchHead = head
		membraneCatchReason = "explicit caller reason"
		membraneCatchDomain = "pawl"
		membraneCatchPaths = []string{"scripts/other.sh"}
	})
	if err := runMembraneCatch(membraneCatchCmd, nil); err != nil {
		t.Fatalf("runMembraneCatch: %v", err)
	}
	l, _ := yieldledger.Load(root)
	catches := yieldledger.DetectCatches(l)
	if len(catches) != 1 {
		t.Fatalf("want 1 catch, got %d", len(catches))
	}
	c := catches[0]
	if c.Reason != "explicit caller reason" || c.Domain != "pawl" {
		t.Fatalf("caller flags must win over extraction; got %+v", c)
	}
	if len(c.AffectedPaths) != 1 || c.AffectedPaths[0] != "scripts/other.sh" {
		t.Fatalf("caller --paths must win, got %v", c.AffectedPaths)
	}
}

// --scope staged resolves domain/paths from the index (git diff --cached), the
// same seam pawl-review uses for scope=staged reviews.
func TestMembraneCatch_EvidenceStagedScope(t *testing.T) {
	root, head := makeCatchEvidenceRepo(t, map[string]string{"docs/base.md": "base\n"})
	// Stage (do not commit) a new file under a DIFFERENT top dir.
	abs := filepath.Join(root, "cli", "staged.go")
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("package staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, nil, "add", "cli/staged.go")
	evidence := filepath.Join(root, "evidence.txt")
	if err := os.WriteFile(evidence, []byte("VERDICT: REFUTED — staged change refuted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	origProjectDir := testProjectDir
	testProjectDir = root
	t.Cleanup(func() { testProjectDir = origProjectDir })

	setMembraneCatchFlags(t, func() {
		membraneCatchBead = "age-ulab-t5"
		membraneCatchEvidence = evidence
		membraneCatchHead = head
		membraneCatchScope = "staged"
	})
	if err := runMembraneCatch(membraneCatchCmd, nil); err != nil {
		t.Fatalf("runMembraneCatch: %v", err)
	}
	l, _ := yieldledger.Load(root)
	catches := yieldledger.DetectCatches(l)
	if len(catches) != 1 || catches[0].Domain != "cli" {
		t.Fatalf("staged scope must resolve domain from the index; got %+v", catches)
	}
	if len(catches[0].AffectedPaths) != 1 || catches[0].AffectedPaths[0] != "cli/staged.go" {
		t.Fatalf("staged paths = %v, want [cli/staged.go]", catches[0].AffectedPaths)
	}
}

func TestMembraneCatch_DocumentsExactUpstreamBase(t *testing.T) {
	if membraneCatchCmd.Flags().Lookup("base") == nil {
		t.Fatal("membrane catch needs --base so an upstream-range REFUTE records the exact reviewed paths")
	}
}

// Without --evidence the original contract is unchanged: --bead, --domain and
// --reason are all required.
func TestMembraneCatch_NoEvidenceStillRequiresDomainReason(t *testing.T) {
	setMembraneCatchFlags(t, func() {
		membraneCatchBead = "age-ulab-t6"
	})
	if err := runMembraneCatch(membraneCatchCmd, nil); err == nil {
		t.Fatal("catch without --evidence and without --domain/--reason must error")
	}
}
