package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// writeFixtureLedger seeds a ledger under root mirroring the dogfood chain shape:
// one accepted clean bead and one refuted-never-accepted bead.
func writeFixtureLedger(t *testing.T, root, run string) {
	t.Helper()
	w := yieldledger.Writer{}
	okRef := yieldledger.PawlVerdictRef{BeadID: "ag-ok", HeadSHA: "abc1234"}
	if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
		BeadID: "ag-ok", RunID: run, Difficulty: 3, PawlVerdictRef: okRef,
		Disposition: yieldledger.DispositionConfirmed, HeadSHA: "abc1234", Attempt: 1,
		AuthorContextID: "ctx-1", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendUsage(root, yieldledger.UsageInput{
		BeadID: "ag-ok", RunID: run, TokensIn: 100, TokensOut: 18000, CostUSD: 1, WallClockS: 1,
		Model: "m", Phase: yieldledger.PhaseImplement, CategoryHint: yieldledger.CategoryProductive,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendAccept(root, yieldledger.AcceptInput{
		BeadID: "ag-ok", RunID: run, MergeSHA: "def5678", MergedBy: "orch", GateVerdictRef: okRef,
	}); err != nil {
		t.Fatal(err)
	}
	lostRef := yieldledger.PawlVerdictRef{BeadID: "ag-lost", HeadSHA: "9990aaa"}
	if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
		BeadID: "ag-lost", RunID: run, Difficulty: 2, PawlVerdictRef: lostRef,
		Disposition: yieldledger.DispositionRefuted, HeadSHA: "9990aaa", Attempt: 1,
		AuthorContextID: "ctx-2", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendUsage(root, yieldledger.UsageInput{
		BeadID: "ag-lost", RunID: run, TokensIn: 50, TokensOut: 6000, CostUSD: 1, WallClockS: 1,
		Model: "m", Phase: yieldledger.PhaseRework, CategoryHint: yieldledger.CategoryRework,
	}); err != nil {
		t.Fatal(err)
	}
}

// TestRunYieldGauge_Report verifies the human report prints the five gauges, the
// computed values, and the shadow-mode hypotheses table marked not-auto-steered.
func TestRunYieldGauge_Report(t *testing.T) {
	root := t.TempDir()
	const run = "r-test"
	writeFixtureLedger(t, root, run)

	prev := testProjectDir
	testProjectDir = root
	defer func() { testProjectDir = prev }()

	cmd := yieldGaugeCmd
	var out bytes.Buffer
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil); cmd.SetErr(nil) }) // age-ztf8: shared command; don't leak the writer
	if err := cmd.Flags().Set("run", run); err != nil {
		t.Fatal(err)
	}
	if err := runYieldGauge(cmd, nil); err != nil {
		t.Fatalf("runYieldGauge: %v", err)
	}
	got := out.String()

	for _, want := range []string{
		"A (accepted)", "Q (first-pass yield)", "A/R", "WATCH ONLY", "E (escalation rate)", "L (loss)",
		"Shadow-mode actuation hypotheses", "not auto-steered",
		"0.600", // Q = 3 / (3+2)
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report missing %q\n--- report ---\n%s", want, got)
		}
	}
}

// TestRunYieldGauge_UnadmittedWarning verifies the report surfaces the E-G leak
// line when an accept is NOT gate-admitted (its ref resolves to no CONFIRMED
// verdict) — so an operator sees that the mesh saw unjudged deposits.
func TestRunYieldGauge_UnadmittedWarning(t *testing.T) {
	root := t.TempDir()
	const run = "r-unadmitted"
	w := yieldledger.Writer{}
	// REFUTED verdict + an accept backed by it → unadmitted deposit.
	if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
		BeadID: "ag-bad", RunID: run, Difficulty: 1,
		PawlVerdictRef: yieldledger.PawlVerdictRef{BeadID: "ag-bad", HeadSHA: "sha-bad7"},
		Disposition:    yieldledger.DispositionRefuted, HeadSHA: "sha-bad7", Attempt: 1,
		AuthorContextID: "ctx-1", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendAccept(root, yieldledger.AcceptInput{
		BeadID: "ag-bad", RunID: run, MergeSHA: "merge-bad7", MergedBy: "orch",
		GateVerdictRef: yieldledger.PawlVerdictRef{BeadID: "ag-bad", HeadSHA: "sha-bad7"},
	}); err != nil {
		t.Fatal(err)
	}

	prev := testProjectDir
	testProjectDir = root
	defer func() { testProjectDir = prev }()

	cmd := yieldGaugeCmd
	var out bytes.Buffer
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil); cmd.SetErr(nil) }) // age-ztf8: shared command; don't leak the writer
	if err := cmd.Flags().Set("run", run); err != nil {
		t.Fatal(err)
	}
	if err := runYieldGauge(cmd, nil); err != nil {
		t.Fatalf("runYieldGauge: %v", err)
	}
	got := out.String()
	for _, want := range []string{"unadmitted deposits", "E-G LEAK"} {
		if !strings.Contains(got, want) {
			t.Errorf("report missing %q (unadmitted accept should surface the leak)\n--- report ---\n%s", want, got)
		}
	}
}

// TestRunYieldGauge_JSON verifies --json emits the gauges plus the hypotheses,
// and that C is the pending sentinel when no --c-delta is supplied.
func TestRunYieldGauge_JSON(t *testing.T) {
	root := t.TempDir()
	const run = "r-test"
	writeFixtureLedger(t, root, run)

	prev := testProjectDir
	testProjectDir = root
	defer func() { testProjectDir = prev }()

	cmd := yieldGaugeCmd
	var out bytes.Buffer
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil); cmd.SetErr(nil) }) // age-ztf8: shared command; don't leak the writer
	if err := cmd.Flags().Set("run", run); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("json", "true"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Flags().Set("json", "false") }()
	if err := runYieldGauge(cmd, nil); err != nil {
		t.Fatalf("runYieldGauge --json: %v", err)
	}

	var doc gaugeJSON
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal gauge json: %v\n%s", err, out.String())
	}
	if doc.Gauges.A != 1 {
		t.Errorf("json A = %d, want 1", doc.Gauges.A)
	}
	if !doc.Gauges.QDefined || doc.Gauges.Q < 0.59 || doc.Gauges.Q > 0.61 {
		t.Errorf("json Q = %v, want ~0.6", doc.Gauges.Q)
	}
	if !doc.Gauges.CPendingFlag {
		t.Error("json C should be pending without --c-delta")
	}
	if len(doc.Hypotheses) != 5 {
		t.Errorf("json hypotheses = %d, want 5", len(doc.Hypotheses))
	}
}

// TestRunYieldGauge_RequiresRun verifies the run flag is mandatory.
func TestRunYieldGauge_RequiresRun(t *testing.T) {
	root := t.TempDir()
	prev := testProjectDir
	testProjectDir = root
	defer func() { testProjectDir = prev }()

	cmd := yieldGaugeCmd
	var out bytes.Buffer
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil); cmd.SetErr(nil) }) // age-ztf8: shared command; don't leak the writer
	if err := cmd.Flags().Set("run", ""); err != nil {
		t.Fatal(err)
	}
	if err := runYieldGauge(cmd, nil); err == nil {
		t.Error("runYieldGauge without --run should error")
	}
}
