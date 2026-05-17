package feedbackcompiler

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/verdictledger"
)

// repoRoot climbs from the test file to the repo root.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file = .../cli/internal/feedbackcompiler/feedbackcompiler_test.go
	// climb: feedbackcompiler/ -> internal/ -> cli/ -> repo root
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// fixturePath resolves a tracked fixture under tests/fixtures/feedback-compiler.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "tests", "fixtures", "feedback-compiler", name)
}

// ledgerFixturePath resolves a tracked fixture under tests/fixtures/verdict-ledger.
func ledgerFixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "tests", "fixtures", "verdict-ledger", name)
}

// fixedNow is the deterministic clock used in tests.
var fixedNow = time.Date(2026, 5, 17, 20, 0, 0, 0, time.UTC)

// newCompiler returns a Compiler with a fixed clock.
func newCompiler() *Compiler {
	return &Compiler{Now: func() time.Time { return fixedNow }}
}

// loadFixtureLedger loads a verdict-ledger fixture for use in tests that
// bypass the file-system ledger lookup.
func loadFixtureLedger(t *testing.T, name string) *verdictledger.Ledger {
	t.Helper()
	ledger, err := verdictledger.LoadPath(ledgerFixturePath(t, name))
	if err != nil {
		t.Fatalf("LoadPath(%s): %v", name, err)
	}
	return ledger
}

// readDraftFile reads a draft file from the learnings subdir of projectRoot.
func readDraftFile(t *testing.T, projectRoot, filename string) string {
	t.Helper()
	dest := filepath.Join(projectRoot, LearningsRelDir, filename)
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read draft %s: %v", dest, err)
	}
	return string(data)
}

// TestScanTransitions_FailToPass verifies that a fail→pass sequence produces
// exactly one Transition for the directive.
func TestScanTransitions_FailToPass(t *testing.T) {
	ledger := loadFixtureLedger(t, "pass-resets-streak.json")
	// d-improve-coverage: fail, fail, pass, fail — one transition at index 2.
	got := scanTransitions(ledger)
	if len(got) != 1 {
		t.Fatalf("scanTransitions count = %d, want 1", len(got))
	}
	if got[0].DirectiveID != "d-improve-coverage" {
		t.Errorf("DirectiveID = %q, want d-improve-coverage", got[0].DirectiveID)
	}
	if got[0].FailRecord.ScenarioVerdict != verdictledger.VerdictFail {
		t.Errorf("FailRecord.ScenarioVerdict = %q, want fail", got[0].FailRecord.ScenarioVerdict)
	}
	if got[0].PassRecord.ScenarioVerdict != verdictledger.VerdictPass {
		t.Errorf("PassRecord.ScenarioVerdict = %q, want pass", got[0].PassRecord.ScenarioVerdict)
	}
}

// TestScanTransitions_OnlyFail verifies a directive that only ever failed
// produces no transitions.
func TestScanTransitions_OnlyFail(t *testing.T) {
	ledger := loadFixtureLedger(t, "failure-streak.json")
	// d-reduce-flaky-tests: three consecutive fails, no pass.
	got := scanTransitions(ledger)
	for _, tr := range got {
		if tr.DirectiveID == "d-reduce-flaky-tests" {
			t.Errorf("unexpected transition for d-reduce-flaky-tests (only fails)")
		}
	}
}

// TestScanTransitions_OnlyPass verifies that a directive that only ever passed
// (no preceding failure) produces no transitions.
func TestScanTransitions_OnlyPass(t *testing.T) {
	// Build a minimal ledger with a single pass record and no prior fail.
	sat := 0.95
	cnt := 3
	ledger := &verdictledger.Ledger{
		SchemaVersion: verdictledger.SchemaVersion,
		Records: []verdictledger.Record{
			{
				RecordType:           verdictledger.RecordIteration,
				DirectiveID:          "d-always-passing",
				RunTime:              "2026-05-17T10:00:00Z",
				ScenarioVerdict:      verdictledger.VerdictPass,
				ScenarioSatisfaction: &sat,
				ScenarioCount:        &cnt,
				EvaluatedCount:       &cnt,
				RunID:                "run-100",
			},
		},
	}
	got := scanTransitions(ledger)
	for _, tr := range got {
		if tr.DirectiveID == "d-always-passing" {
			t.Errorf("unexpected transition for d-always-passing (no preceding fail)")
		}
	}
}

// TestScanTransitions_MultipleTransitions verifies multiple fail→pass cycles
// within the same directive each produce a separate Transition.
func TestScanTransitions_MultipleTransitions(t *testing.T) {
	sat1, sat2, sat3, sat4 := 0.3, 0.9, 0.4, 0.95
	cnt := 2
	ledger := &verdictledger.Ledger{
		SchemaVersion: verdictledger.SchemaVersion,
		Records: []verdictledger.Record{
			{RecordType: verdictledger.RecordIteration, DirectiveID: "d-multi", RunTime: "2026-05-17T01:00:00Z", ScenarioVerdict: verdictledger.VerdictFail, ScenarioSatisfaction: &sat1, ScenarioCount: &cnt, EvaluatedCount: &cnt, RunID: "r1"},
			{RecordType: verdictledger.RecordIteration, DirectiveID: "d-multi", RunTime: "2026-05-17T02:00:00Z", ScenarioVerdict: verdictledger.VerdictPass, ScenarioSatisfaction: &sat2, ScenarioCount: &cnt, EvaluatedCount: &cnt, RunID: "r2"},
			{RecordType: verdictledger.RecordIteration, DirectiveID: "d-multi", RunTime: "2026-05-17T03:00:00Z", ScenarioVerdict: verdictledger.VerdictFail, ScenarioSatisfaction: &sat3, ScenarioCount: &cnt, EvaluatedCount: &cnt, RunID: "r3"},
			{RecordType: verdictledger.RecordIteration, DirectiveID: "d-multi", RunTime: "2026-05-17T04:00:00Z", ScenarioVerdict: verdictledger.VerdictPass, ScenarioSatisfaction: &sat4, ScenarioCount: &cnt, EvaluatedCount: &cnt, RunID: "r4"},
		},
	}
	got := directiveTransitions("d-multi", ledger.IterationsFor("d-multi"))
	if len(got) != 2 {
		t.Fatalf("directiveTransitions count = %d, want 2", len(got))
	}
	if got[0].FailRecord.RunID != "r1" {
		t.Errorf("first transition FailRecord.RunID = %q, want r1", got[0].FailRecord.RunID)
	}
	if got[0].PassRecord.RunID != "r2" {
		t.Errorf("first transition PassRecord.RunID = %q, want r2", got[0].PassRecord.RunID)
	}
	if got[1].FailRecord.RunID != "r3" {
		t.Errorf("second transition FailRecord.RunID = %q, want r3", got[1].FailRecord.RunID)
	}
	if got[1].PassRecord.RunID != "r4" {
		t.Errorf("second transition PassRecord.RunID = %q, want r4", got[1].PassRecord.RunID)
	}
}

// TestCompileFromLedger_DraftWritten verifies that a fail→pass transition
// produces exactly one learning draft with the required frontmatter fields.
func TestCompileFromLedger_DraftWritten(t *testing.T) {
	ledger := loadFixtureLedger(t, "pass-resets-streak.json")
	root := t.TempDir()
	c := newCompiler()

	result, err := c.CompileFromLedger(root, ledger)
	if err != nil {
		t.Fatalf("CompileFromLedger: %v", err)
	}
	if len(result.Drafts) != 1 {
		t.Fatalf("Drafts count = %d, want 1", len(result.Drafts))
	}
	if result.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", result.Skipped)
	}

	// Verify the draft file exists.
	content := readDraftFile(t, root, filepath.Base(result.Drafts[0]))

	// Frontmatter must carry status: draft.
	if !strings.Contains(content, "status: draft") {
		t.Errorf("draft missing 'status: draft' in frontmatter")
	}
	// Frontmatter must carry directive_id.
	if !strings.Contains(content, "directive_id: d-improve-coverage") {
		t.Errorf("draft missing 'directive_id: d-improve-coverage' in frontmatter")
	}
	// Frontmatter must carry a title.
	if !strings.Contains(content, "title:") {
		t.Errorf("draft missing 'title:' in frontmatter")
	}
	// Body must mention the directive.
	if !strings.Contains(content, "d-improve-coverage") {
		t.Errorf("draft body missing directive ID d-improve-coverage")
	}
	// Body must mention both verdict labels.
	if !strings.Contains(content, "fail") {
		t.Errorf("draft body missing 'fail'")
	}
	if !strings.Contains(content, "pass") {
		t.Errorf("draft body missing 'pass'")
	}
}

// TestCompileFromLedger_OnlyFail verifies that a directive with no pass
// produces no drafts.
func TestCompileFromLedger_OnlyFail(t *testing.T) {
	ledger := loadFixtureLedger(t, "failure-streak.json")
	root := t.TempDir()
	c := newCompiler()

	result, err := c.CompileFromLedger(root, ledger)
	if err != nil {
		t.Fatalf("CompileFromLedger: %v", err)
	}
	if len(result.Drafts) != 0 {
		t.Errorf("Drafts count = %d, want 0 (only-fail directive)", len(result.Drafts))
	}
}

// TestCompileFromLedger_OnlyPass verifies that a directive with only passes
// produces no drafts.
func TestCompileFromLedger_OnlyPass(t *testing.T) {
	sat := 0.95
	cnt := 2
	ledger := &verdictledger.Ledger{
		SchemaVersion: verdictledger.SchemaVersion,
		Records: []verdictledger.Record{
			{RecordType: verdictledger.RecordIteration, DirectiveID: "d-always-passing", RunTime: "2026-05-17T10:00:00Z", ScenarioVerdict: verdictledger.VerdictPass, ScenarioSatisfaction: &sat, ScenarioCount: &cnt, EvaluatedCount: &cnt, RunID: "r1"},
		},
	}
	root := t.TempDir()
	c := newCompiler()

	result, err := c.CompileFromLedger(root, ledger)
	if err != nil {
		t.Fatalf("CompileFromLedger: %v", err)
	}
	if len(result.Drafts) != 0 {
		t.Errorf("Drafts count = %d, want 0 (only-pass directive)", len(result.Drafts))
	}
}

// TestCompileFromLedger_IdempotentSkip verifies that running the compiler
// twice does not overwrite an existing draft; the second run skips it.
func TestCompileFromLedger_IdempotentSkip(t *testing.T) {
	ledger := loadFixtureLedger(t, "pass-resets-streak.json")
	root := t.TempDir()
	c := newCompiler()

	first, err := c.CompileFromLedger(root, ledger)
	if err != nil {
		t.Fatalf("first CompileFromLedger: %v", err)
	}
	if len(first.Drafts) != 1 {
		t.Fatalf("first run Drafts = %d, want 1", len(first.Drafts))
	}

	second, err := c.CompileFromLedger(root, ledger)
	if err != nil {
		t.Fatalf("second CompileFromLedger: %v", err)
	}
	if len(second.Drafts) != 0 {
		t.Errorf("second run Drafts = %d, want 0 (already written)", len(second.Drafts))
	}
	if second.Skipped != 1 {
		t.Errorf("second run Skipped = %d, want 1", second.Skipped)
	}
}

// TestDraftFilename verifies the filename pattern for a draft.
func TestDraftFilename(t *testing.T) {
	passTime := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	got := draftFilename("d-improve-coverage", passTime)
	want := "2026-05-17-auto-improve-coverage-fail-to-pass.md"
	if got != want {
		t.Errorf("draftFilename = %q, want %q", got, want)
	}
}

// TestDraftContent_FrontmatterFields verifies every required frontmatter field
// appears in the rendered draft template with exact expected values.
func TestDraftContent_FrontmatterFields(t *testing.T) {
	sat1, sat2 := 0.40, 0.90
	tr := Transition{
		DirectiveID: "d-my-directive",
		FailRecord: verdictledger.Record{
			RunID:                "run-fail-01",
			RunTime:              "2026-05-17T09:00:00Z",
			ScenarioVerdict:      verdictledger.VerdictFail,
			ScenarioSatisfaction: &sat1,
		},
		PassRecord: verdictledger.Record{
			RunID:                "run-pass-01",
			RunTime:              "2026-05-17T10:00:00Z",
			ScenarioVerdict:      verdictledger.VerdictPass,
			ScenarioSatisfaction: &sat2,
		},
	}
	now := time.Date(2026, 5, 17, 20, 0, 0, 0, time.UTC)
	content, err := draftContent(tr, now)
	if err != nil {
		t.Fatalf("draftContent: %v", err)
	}

	checks := []struct {
		field string
		want  string
	}{
		{"status", "status: draft"},
		{"directive_id", "directive_id: d-my-directive"},
		{"title", "title:"},
		{"date", "date: 2026-05-17"},
		{"fail run_id", "run-fail-01"},
		{"pass run_id", "run-pass-01"},
		{"fail satisfaction", "40%"},
		{"pass satisfaction", "90%"},
	}
	for _, ch := range checks {
		if !strings.Contains(content, ch.want) {
			t.Errorf("draftContent missing %s field (want %q)", ch.field, ch.want)
		}
	}
}

// TestCompileFromLedger_EmptyLedger verifies that an empty ledger produces no
// drafts and no error.
func TestCompileFromLedger_EmptyLedger(t *testing.T) {
	ledger := &verdictledger.Ledger{SchemaVersion: verdictledger.SchemaVersion}
	root := t.TempDir()
	c := newCompiler()

	result, err := c.CompileFromLedger(root, ledger)
	if err != nil {
		t.Fatalf("CompileFromLedger(empty): %v", err)
	}
	if len(result.Drafts) != 0 {
		t.Errorf("Drafts = %d, want 0 for empty ledger", len(result.Drafts))
	}
}

// TestCompileFromLedger_FixtureFile exercises the full Compile path (ledger
// loaded from disk) against the pass-resets-streak fixture.
func TestCompileFromLedger_FixtureFile(t *testing.T) {
	// Set up a fake project root with the ledger at the canonical location.
	root := t.TempDir()
	src := ledgerFixturePath(t, "pass-resets-streak.json")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	ledgerDir := filepath.Join(root, ".agents", "goals")
	if err := os.MkdirAll(ledgerDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ledgerDir, "verdict-ledger.json"), data, 0o600); err != nil {
		t.Fatalf("write fixture ledger: %v", err)
	}

	c := newCompiler()
	result, err := c.Compile(root)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(result.Drafts) != 1 {
		t.Fatalf("Drafts = %d, want 1", len(result.Drafts))
	}
	content := readDraftFile(t, root, filepath.Base(result.Drafts[0]))
	if !strings.Contains(content, "status: draft") {
		t.Errorf("Compile draft missing 'status: draft'")
	}
}
