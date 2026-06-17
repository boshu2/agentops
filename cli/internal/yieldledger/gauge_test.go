package yieldledger

import (
	"fmt"
	"math"
	"testing"
	"time"
)

const dogfoodRun = "r-2026-06-14-dynamo"

func loadFixture(t *testing.T) *Ledger {
	t.Helper()
	l, err := LoadPath(fixturePath(t, "dogfood-chain.jsonl"))
	if err != nil {
		t.Fatalf("LoadPath: %v", err)
	}
	return l
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestComputeGauges_Fixture exercises the whole vector over the tracked
// dogfood-chain fixture: ag-grcz3 (difficulty 3, attempt-1 CONFIRMED + accept +
// productive usage) and ag-qzinh (difficulty 2, attempt-1 REFUTED + rework
// usage, never accepted). It pins A, Q numerator/denominator over the mixed
// accepted+rejected set, A/R, and the L read-time join.
func TestComputeGauges_Fixture(t *testing.T) {
	l := loadFixture(t)
	g := ComputeGauges(l, dogfoodRun, 0, false)

	if g.A != 1 {
		t.Errorf("A = %d, want 1 (only ag-grcz3 accepted)", g.A)
	}
	// R = tokens_out: 18000 (grcz3) + 6000 (qzinh) = 24000.
	if g.R != 24000 {
		t.Errorf("R = %d, want 24000", g.R)
	}

	// Q: numerator = 3 (ag-grcz3 clean-first-pass AND accepted);
	//    denominator = 3 + 2 = 5 (both beads attempted, weighted by difficulty);
	//    Q = 0.6. ag-qzinh is in the DENOMINATOR (attempted) but not the
	//    numerator (REFUTED, never accepted) — the instrument honestly counts the
	//    rejected attempt against first-pass yield.
	if g.QAttemptBeads != 2 {
		t.Errorf("QAttemptBeads = %d, want 2", g.QAttemptBeads)
	}
	if g.QCleanBeads != 1 {
		t.Errorf("QCleanBeads = %d, want 1", g.QCleanBeads)
	}
	if !approx(g.QNumerator, 3) {
		t.Errorf("QNumerator = %v, want 3", g.QNumerator)
	}
	if !approx(g.QDenominator, 5) {
		t.Errorf("QDenominator = %v, want 5", g.QDenominator)
	}
	if !g.QDefined || !approx(g.Q, 0.6) {
		t.Errorf("Q = %v (defined=%v), want 0.6", g.Q, g.QDefined)
	}

	// A/R = 1 / 24000.
	if !g.AOverRDefined || !approx(g.AOverR, 1.0/24000.0) {
		t.Errorf("A/R = %v, want %v", g.AOverR, 1.0/24000.0)
	}

	// L read-time join: ag-grcz3 usage is productive (accepted, phase=implement);
	// ag-qzinh usage is rework loss (phase=rework). L spend = 6000; L = 6000/24000.
	if g.LCategory.Productive != 18000 {
		t.Errorf("L productive = %d, want 18000", g.LCategory.Productive)
	}
	if g.LCategory.Rework != 6000 {
		t.Errorf("L rework = %d, want 6000", g.LCategory.Rework)
	}
	if g.LCategory.Rejected != 0 {
		t.Errorf("L rejected = %d, want 0 (qzinh's spend is phase=rework, classified rework first)", g.LCategory.Rejected)
	}
	if g.LSpend != 6000 {
		t.Errorf("L spend = %d, want 6000", g.LSpend)
	}
	if !g.LDefined || !approx(g.L, 6000.0/24000.0) {
		t.Errorf("L = %v, want %v", g.L, 6000.0/24000.0)
	}

	// C pending path: no published delta passed.
	if !g.CPendingFlag || g.CValue != CPending {
		t.Errorf("C should be pending sentinel, got pending=%v value=%q", g.CPendingFlag, g.CValue)
	}
}

// TestComputeGauges_RejectedSpendIsLoss verifies the read-time join classifies
// spend on a never-accepted bead as a rejected loss even when the usage row's
// phase is NOT rework/coordination (i.e. it was emitted "productive" but the bead
// was later refuted). The append-only category_hint must NOT override the join.
func TestComputeGauges_RejectedSpendIsLoss(t *testing.T) {
	root := t.TempDir()
	ref := PawlVerdictRef{BeadID: "ag-lost", HeadSHA: "abc1234"}
	w := Writer{}

	// A refuted bead, never accepted, whose usage was emitted in the IMPLEMENT
	// phase with an optimistic productive hint.
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-lost", RunID: "r1", Difficulty: 4, PawlVerdictRef: ref,
		Disposition: DispositionRefuted, HeadSHA: "abc1234", Attempt: 1,
		AuthorContextID: "ctx-1", RefuterFamilies: []string{"gpt"}, AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendUsage(root, UsageInput{
		BeadID: "ag-lost", RunID: "r1", TokensIn: 1000, TokensOut: 500,
		CostUSD: 0.1, WallClockS: 30, Model: "m", Phase: PhaseImplement,
		CategoryHint: CategoryProductive, // optimistic hint — must be overridden
	}); err != nil {
		t.Fatal(err)
	}

	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	g := ComputeGauges(l, "r1", 0, false)

	if g.A != 0 {
		t.Errorf("A = %d, want 0 (no accept)", g.A)
	}
	if g.LCategory.Rejected != 500 {
		t.Errorf("rejected loss = %d, want 500 (read-time join overrides productive hint)", g.LCategory.Rejected)
	}
	if g.LCategory.Productive != 0 {
		t.Errorf("productive = %d, want 0", g.LCategory.Productive)
	}
	if !g.LDefined || !approx(g.L, 1.0) {
		t.Errorf("L = %v, want 1.0 (all spend is loss)", g.L)
	}
	// Q: attempted (denom weight 4) but not clean — numerator 0, Q = 0.
	if !g.QDefined || !approx(g.Q, 0) {
		t.Errorf("Q = %v, want 0", g.Q)
	}
}

// TestComputeGauges_Escalation verifies E counts ESCALATE and HOLD verdicts over
// accepts, and is undefined (not a misleading 0) when there are no accepts.
func TestComputeGauges_Escalation(t *testing.T) {
	root := t.TempDir()
	ref := PawlVerdictRef{BeadID: "ag-e", HeadSHA: "abc1234"}
	w := Writer{}

	// One accepted bead (clean) + an ESCALATE verdict + a HOLD verdict on two
	// other beads — E must count BOTH ESCALATE and HOLD.
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-ok", RunID: "r1", Difficulty: 1, PawlVerdictRef: PawlVerdictRef{BeadID: "ag-ok", HeadSHA: "abc1234"},
		Disposition: DispositionConfirmed, HeadSHA: "abc1234", Attempt: 1,
		AuthorContextID: "ctx-1", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendAccept(root, AcceptInput{
		BeadID: "ag-ok", RunID: "r1", MergeSHA: "def5678", MergedBy: "orch",
		GateVerdictRef: PawlVerdictRef{BeadID: "ag-ok", HeadSHA: "abc1234"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-e", RunID: "r1", Difficulty: 1, PawlVerdictRef: ref,
		Disposition: DispositionEscalate, HeadSHA: "abc1234", Attempt: 1,
		AuthorContextID: "ctx-1", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-h", RunID: "r1", Difficulty: 1, PawlVerdictRef: PawlVerdictRef{BeadID: "ag-h", HeadSHA: "abc1234"},
		Disposition: DispositionHold, HeadSHA: "abc1234", Attempt: 1,
		AuthorContextID: "ctx-1", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}

	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	g := ComputeGauges(l, "r1", 0, false)

	if g.A != 1 {
		t.Fatalf("A = %d, want 1", g.A)
	}
	// Both the ESCALATE and the HOLD verdict count toward E.
	if g.EEscalateHolds != 2 {
		t.Errorf("EEscalateHolds = %d, want 2 (1 ESCALATE + 1 HOLD)", g.EEscalateHolds)
	}
	if !g.EDefined || !approx(g.E, 2.0) {
		t.Errorf("E = %v, want 2.0 ((1 escalate + 1 hold) / 1 accept)", g.E)
	}

	// No-accept run: E undefined.
	g2 := ComputeGauges(l, "no-such-run", 0, false)
	if g2.EDefined {
		t.Errorf("E should be undefined for a run with 0 accepts, got %v", g2.E)
	}
}

// TestComputeGauges_LateConfirmNotCleanFirstPass is the exact case codex's
// check-7 flagged: a bead REFUTED on attempt 1, then CONFIRMED on attempt 4, with
// an accept authorized by attempt 4's head_sha. It must NOT count toward the Q
// numerator (it is NOT a clean first pass) but MUST appear in the denominator.
func TestComputeGauges_LateConfirmNotCleanFirstPass(t *testing.T) {
	root := t.TempDir()
	w := Writer{}

	// Attempt 1: REFUTED at head_sha sha-att1.
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-late", RunID: "r1", Difficulty: 5,
		PawlVerdictRef: PawlVerdictRef{BeadID: "ag-late", HeadSHA: "sha-att1"},
		Disposition:    DispositionRefuted, HeadSHA: "sha-att1", Attempt: 1,
		AuthorContextID: "ctx-1", RefuterFamilies: []string{"gpt"}, AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	// Attempt 4: CONFIRMED at a DIFFERENT head_sha sha-att4.
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-late", RunID: "r1", Difficulty: 5,
		PawlVerdictRef: PawlVerdictRef{BeadID: "ag-late", HeadSHA: "sha-att4"},
		Disposition:    DispositionConfirmed, HeadSHA: "sha-att4", Attempt: 4,
		AuthorContextID: "ctx-1", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	// Accept authorized by attempt 4's head_sha — NOT attempt 1's.
	if _, err := w.AppendAccept(root, AcceptInput{
		BeadID: "ag-late", RunID: "r1", MergeSHA: "merge-1", MergedBy: "orch",
		GateVerdictRef: PawlVerdictRef{BeadID: "ag-late", HeadSHA: "sha-att4"},
	}); err != nil {
		t.Fatal(err)
	}

	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	g := ComputeGauges(l, "r1", 0, false)

	// Accepted at bead level, but NOT a clean first pass: the accept is bound to
	// the attempt-4 verdict, not the attempt-1 (which was REFUTED).
	if g.A != 1 {
		t.Fatalf("A = %d, want 1 (the bead was accepted)", g.A)
	}
	if g.QCleanBeads != 0 {
		t.Errorf("QCleanBeads = %d, want 0 (attempt-1 was REFUTED; accept bound to attempt 4)", g.QCleanBeads)
	}
	if !approx(g.QNumerator, 0) {
		t.Errorf("QNumerator = %v, want 0 (late confirm must NOT inflate Q)", g.QNumerator)
	}
	// But it IS in the denominator (one distinct attempted bead, weight 5).
	if g.QAttemptBeads != 1 {
		t.Errorf("QAttemptBeads = %d, want 1 (the bead was attempted)", g.QAttemptBeads)
	}
	if !approx(g.QDenominator, 5) {
		t.Errorf("QDenominator = %v, want 5 (difficulty-weighted attempted bead)", g.QDenominator)
	}
	if !g.QDefined || !approx(g.Q, 0) {
		t.Errorf("Q = %v, want 0 (0/5)", g.Q)
	}
}

func TestComputeGauges_AcceptedLateAttemptCountsPriorAttemptsAsRework(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	base := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	if _, err := w.AppendUsage(root, UsageInput{
		BeadID: "ag-late", RunID: "r1", TS: base.Add(1 * time.Minute),
		TokensOut: 100, Model: "m", Phase: PhaseImplement,
		CategoryHint: CategoryProductive,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-late", RunID: "r1", TS: base.Add(2 * time.Minute), Difficulty: 5,
		PawlVerdictRef: PawlVerdictRef{BeadID: "ag-late", HeadSHA: "shaatt01"},
		Disposition:    DispositionRefuted, HeadSHA: "shaatt01", Attempt: 1,
		AuthorContextID: "ctx-1", RefuterFamilies: []string{"gpt"}, AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendUsage(root, UsageInput{
		BeadID: "ag-late", RunID: "r1", TS: base.Add(3 * time.Minute),
		TokensOut: 200, Model: "m", Phase: PhaseImplement,
		CategoryHint: CategoryProductive,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-late", RunID: "r1", TS: base.Add(4 * time.Minute), Difficulty: 5,
		PawlVerdictRef: PawlVerdictRef{BeadID: "ag-late", HeadSHA: "shaatt02"},
		Disposition:    DispositionRefuted, HeadSHA: "shaatt02", Attempt: 2,
		AuthorContextID: "ctx-1", RefuterFamilies: []string{"gpt"}, AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendUsage(root, UsageInput{
		BeadID: "ag-late", RunID: "r1", TS: base.Add(5 * time.Minute),
		TokensOut: 300, Model: "m", Phase: PhaseImplement,
		CategoryHint: CategoryProductive,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-late", RunID: "r1", TS: base.Add(6 * time.Minute), Difficulty: 5,
		PawlVerdictRef: PawlVerdictRef{BeadID: "ag-late", HeadSHA: "shaatt03"},
		Disposition:    DispositionConfirmed, HeadSHA: "shaatt03", Attempt: 3,
		AuthorContextID: "ctx-1", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendAccept(root, AcceptInput{
		BeadID: "ag-late", RunID: "r1", TS: base.Add(7 * time.Minute),
		MergeSHA: "merge01", MergedBy: "orch",
		GateVerdictRef: PawlVerdictRef{BeadID: "ag-late", HeadSHA: "shaatt03"},
	}); err != nil {
		t.Fatal(err)
	}

	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	g := ComputeGauges(l, "r1", 0, false)

	if g.R != 600 {
		t.Fatalf("R = %d, want 600", g.R)
	}
	if g.LCategory.Rework != 300 {
		t.Errorf("rework = %d, want 300 (attempts 1 and 2 before accepting attempt 3)", g.LCategory.Rework)
	}
	if g.LCategory.Productive != 300 {
		t.Errorf("productive = %d, want 300 (accepting attempt 3)", g.LCategory.Productive)
	}
	if g.LSpend != 300 {
		t.Errorf("LSpend = %d, want 300", g.LSpend)
	}
	if !g.LDefined || !approx(g.L, 0.5) {
		t.Errorf("L = %v, want 0.5", g.L)
	}
}

func TestComputeGauges_PostConfirmProductiveSpendStaysProductive(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	ts := time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		sha         string
		attempt     int
		disposition string
	}{
		{"shaatt01", 1, DispositionRefuted},
		{"shaatt02", 2, DispositionConfirmed},
	} {
		if _, err := w.AppendGateVerdict(root, GateVerdictInput{
			BeadID: "ag-aggregate", RunID: "r1", TS: ts, Difficulty: 3,
			PawlVerdictRef: PawlVerdictRef{BeadID: "ag-aggregate", HeadSHA: tc.sha},
			Disposition:    tc.disposition, HeadSHA: tc.sha, Attempt: tc.attempt,
			AuthorContextID: "ctx-1", RefuterFamilies: []string{"gpt"}, AuthorFamily: "claude",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.AppendUsage(root, UsageInput{
		BeadID: "ag-aggregate", RunID: "r1", TS: ts,
		TokensOut: 100, Model: "m", Phase: PhaseReview,
		CategoryHint: CategoryProductive,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendAccept(root, AcceptInput{
		BeadID: "ag-aggregate", RunID: "r1", TS: ts,
		MergeSHA: "merge01", MergedBy: "orch",
		GateVerdictRef: PawlVerdictRef{BeadID: "ag-aggregate", HeadSHA: "shaatt02"},
	}); err != nil {
		t.Fatal(err)
	}

	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	g := ComputeGauges(l, "r1", 0, false)

	if g.LCategory.Rework != 0 {
		t.Errorf("rework = %d, want 0 (post-confirm productive spend must not be prorated into rework)", g.LCategory.Rework)
	}
	if g.LCategory.Productive != 100 {
		t.Errorf("productive = %d, want 100 (usage emitted after confirming verdict is productive)", g.LCategory.Productive)
	}
	if !g.LDefined || !approx(g.L, 0) {
		t.Errorf("L = %v, want 0", g.L)
	}
}

// TestComputeGauges_CleanFirstPassPositive is the positive case: attempt-1
// CONFIRMED with an accept referencing attempt-1's head_sha — DOES contribute to
// the Q numerator.
func TestComputeGauges_CleanFirstPassPositive(t *testing.T) {
	root := t.TempDir()
	w := Writer{}

	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-clean", RunID: "r1", Difficulty: 3,
		PawlVerdictRef: PawlVerdictRef{BeadID: "ag-clean", HeadSHA: "sha-001"},
		Disposition:    DispositionConfirmed, HeadSHA: "sha-001", Attempt: 1,
		AuthorContextID: "ctx-1", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	// Accept references attempt-1's head_sha.
	if _, err := w.AppendAccept(root, AcceptInput{
		BeadID: "ag-clean", RunID: "r1", MergeSHA: "merge-1", MergedBy: "orch",
		GateVerdictRef: PawlVerdictRef{BeadID: "ag-clean", HeadSHA: "sha-001"},
	}); err != nil {
		t.Fatal(err)
	}

	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	g := ComputeGauges(l, "r1", 0, false)

	if g.QCleanBeads != 1 {
		t.Errorf("QCleanBeads = %d, want 1 (attempt-1 CONFIRMED, accept bound to attempt-1)", g.QCleanBeads)
	}
	if !approx(g.QNumerator, 3) {
		t.Errorf("QNumerator = %v, want 3", g.QNumerator)
	}
	if !approx(g.QDenominator, 3) {
		t.Errorf("QDenominator = %v, want 3", g.QDenominator)
	}
	if !g.QDefined || !approx(g.Q, 1.0) {
		t.Errorf("Q = %v, want 1.0 (clean first pass)", g.Q)
	}
}

// TestComputeGauges_LooseMatchGuard is the regression guard for the codex
// check-7 BLOCKING bug: attempt-1 is CONFIRMED, but the accept references a
// DIFFERENT head_sha than attempt-1's verdict. The bead must NOT count in the Q
// numerator — the accept must be authorized by THAT specific attempt-1 verdict.
// This test fails against the old loose code (which only checked attempt-1
// CONFIRMED + bead-level accepted) and passes against the head_sha-match fix.
func TestComputeGauges_LooseMatchGuard(t *testing.T) {
	root := t.TempDir()
	w := Writer{}

	// Attempt-1 CONFIRMED at head_sha sha-real.
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-loose", RunID: "r1", Difficulty: 2,
		PawlVerdictRef: PawlVerdictRef{BeadID: "ag-loose", HeadSHA: "sha-real"},
		Disposition:    DispositionConfirmed, HeadSHA: "sha-real", Attempt: 1,
		AuthorContextID: "ctx-1", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	// Accept references a DIFFERENT head_sha — not the attempt-1 verdict's.
	if _, err := w.AppendAccept(root, AcceptInput{
		BeadID: "ag-loose", RunID: "r1", MergeSHA: "merge-1", MergedBy: "orch",
		GateVerdictRef: PawlVerdictRef{BeadID: "ag-loose", HeadSHA: "sha-other"},
	}); err != nil {
		t.Fatal(err)
	}

	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	g := ComputeGauges(l, "r1", 0, false)

	// E-G admission (ag-9gg4j): the accept references head_sha "sha-other", which
	// has NO gate-verdict — it claims authorization from a verdict that does not
	// exist. It is an UNADMITTED deposit: A must NOT count it (that would let the
	// mesh self-excite on unjudged work); it is surfaced as Unadmitted instead.
	if g.A != 0 {
		t.Fatalf("A = %d, want 0 (accept references a head_sha with no CONFIRMED verdict → unadmitted)", g.A)
	}
	if g.Unadmitted != 1 {
		t.Fatalf("Unadmitted = %d, want 1 (the unbacked accept is surfaced)", g.Unadmitted)
	}
	// The accept does not reference the attempt-1 verdict's head_sha, so the bead
	// is NOT a clean first pass — numerator 0.
	if g.QCleanBeads != 0 {
		t.Errorf("QCleanBeads = %d, want 0 (accept references a different head_sha than attempt-1)", g.QCleanBeads)
	}
	if !approx(g.QNumerator, 0) {
		t.Errorf("QNumerator = %v, want 0 (loose match must NOT count)", g.QNumerator)
	}
	// Still in the denominator (attempted).
	if g.QAttemptBeads != 1 || !approx(g.QDenominator, 2) {
		t.Errorf("denominator: QAttemptBeads=%d QDenominator=%v, want 1 and 2", g.QAttemptBeads, g.QDenominator)
	}
}

// E-G admission gate (ag-9gg4j): A counts only gate-CONFIRMED-backed accepts; an
// accept backed by a REFUTED verdict is an unadmitted deposit. Without this the
// mesh self-excites on unjudged work.
func TestComputeGauges_EGAdmissionGate(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	// ag-good: CONFIRMED verdict at sha-good + accept bound to it → ADMITTED.
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-good", RunID: "r1", Difficulty: 1,
		PawlVerdictRef: PawlVerdictRef{BeadID: "ag-good", HeadSHA: "sha-good"},
		Disposition:    DispositionConfirmed, HeadSHA: "sha-good", Attempt: 1,
		AuthorContextID: "ctx-1", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendAccept(root, AcceptInput{
		BeadID: "ag-good", RunID: "r1", MergeSHA: "merge-good", MergedBy: "orch",
		GateVerdictRef: PawlVerdictRef{BeadID: "ag-good", HeadSHA: "sha-good"},
	}); err != nil {
		t.Fatal(err)
	}
	// ag-bad: REFUTED verdict at sha-bad + accept bound to it → UNADMITTED
	// (an accept backed by a REFUTED verdict must NOT count as accepted work).
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-bad", RunID: "r1", Difficulty: 1,
		PawlVerdictRef: PawlVerdictRef{BeadID: "ag-bad", HeadSHA: "sha-bad"},
		Disposition:    DispositionRefuted, HeadSHA: "sha-bad", Attempt: 1,
		AuthorContextID: "ctx-1", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendAccept(root, AcceptInput{
		BeadID: "ag-bad", RunID: "r1", MergeSHA: "merge-badd", MergedBy: "orch",
		GateVerdictRef: PawlVerdictRef{BeadID: "ag-bad", HeadSHA: "sha-bad"},
	}); err != nil {
		t.Fatal(err)
	}
	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	g := ComputeGauges(l, "r1", 0, false)
	if g.A != 1 {
		t.Errorf("A = %d, want 1 (only the CONFIRMED-backed accept is admitted)", g.A)
	}
	if g.Unadmitted != 1 {
		t.Errorf("Unadmitted = %d, want 1 (the REFUTED-backed accept is rejected from A)", g.Unadmitted)
	}
}

// TestComputeGauges_CConsumed verifies C is consumed (not recomputed) when a
// published delta is supplied, and is the pending sentinel otherwise.
func TestComputeGauges_CConsumed(t *testing.T) {
	l := loadFixture(t)

	pending := ComputeGauges(l, dogfoodRun, 0, false)
	if !pending.CPendingFlag || pending.CValue != CPending {
		t.Errorf("C without delta should be pending, got pending=%v value=%q", pending.CPendingFlag, pending.CValue)
	}

	published := ComputeGauges(l, dogfoodRun, 0.42, true)
	if published.CPendingFlag {
		t.Error("C with delta should not be pending")
	}
	if !approx(published.CDelta, 0.42) {
		t.Errorf("CDelta = %v, want 0.42", published.CDelta)
	}
}

// TestActuationHypotheses_AreShadow verifies the pre-registered hypotheses ship
// verbatim and are all shadow/watch (never an executable/auto mode).
func TestActuationHypotheses_AreShadow(t *testing.T) {
	hs := ActuationHypotheses()
	if len(hs) != 5 {
		t.Fatalf("len(hypotheses) = %d, want 5 (pre-registration §3)", len(hs))
	}
	for _, h := range hs {
		if h.Mode != "shadow" && h.Mode != "watch" {
			t.Errorf("hypothesis %q has mode %q, want shadow|watch (never auto-steered)", h.Trigger, h.Mode)
		}
	}
	// The A/R rule must be watch-only (Goodhart guard).
	var foundAR bool
	for _, h := range hs {
		if h.Trigger == "A/R moves" {
			foundAR = true
			if h.Mode != "watch" {
				t.Errorf("A/R hypothesis mode = %q, want watch", h.Mode)
			}
		}
	}
	if !foundAR {
		t.Error("missing the A/R WATCH-ONLY hypothesis")
	}
}

// TestComputeGauges_EGCrossBeadRefNotAdmitted closes the codex-gate false-admit:
// an accept on bead-X carrying a gate_verdict_ref.bead_id of bead-Y must NOT be
// admitted via bead-X's own CONFIRMED verdict at the same head_sha — the ref must
// bind to the accept's bead.
func TestComputeGauges_EGCrossBeadRefNotAdmitted(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-x", RunID: "r1", Difficulty: 1,
		PawlVerdictRef: PawlVerdictRef{BeadID: "ag-x", HeadSHA: "sha-shared"},
		Disposition:    DispositionConfirmed, HeadSHA: "sha-shared", Attempt: 1,
		AuthorContextID: "ctx-1", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	// accept on bead-X, but its ref claims bead-Y (mismatched) at the same sha.
	if _, err := w.AppendAccept(root, AcceptInput{
		BeadID: "ag-x", RunID: "r1", MergeSHA: "merge-xref", MergedBy: "orch",
		GateVerdictRef: PawlVerdictRef{BeadID: "ag-y", HeadSHA: "sha-shared"},
	}); err != nil {
		t.Fatal(err)
	}
	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	g := ComputeGauges(l, "r1", 0, false)
	if g.A != 0 {
		t.Errorf("A = %d, want 0 (cross-bead ref.bead_id must not admit via the wrong bead's verdict)", g.A)
	}
	if g.Unadmitted != 1 {
		t.Errorf("Unadmitted = %d, want 1 (mismatched-ref accept surfaced)", g.Unadmitted)
	}
}

// TestComputeGauges_EGUnadmittedSpendIsLoss closes the L-side of E-G (codex gate):
// a bead whose only accept is UNADMITTED (backed by a REFUTED verdict) must be
// treated as never-accepted by the L join — its spend is rejected-loss, NOT
// productive. Otherwise the mesh under-counts loss on unjudged deposits.
func TestComputeGauges_EGUnadmittedSpendIsLoss(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-uad", RunID: "r1", Difficulty: 1,
		PawlVerdictRef: PawlVerdictRef{BeadID: "ag-uad", HeadSHA: "sha-refd"},
		Disposition:    DispositionRefuted, HeadSHA: "sha-refd", Attempt: 1,
		AuthorContextID: "ctx-1", AuthorFamily: "claude",
	}); err != nil {
		t.Fatal(err)
	}
	// accept backed by the REFUTED verdict → unadmitted.
	if _, err := w.AppendAccept(root, AcceptInput{
		BeadID: "ag-uad", RunID: "r1", MergeSHA: "merge-uad", MergedBy: "orch",
		GateVerdictRef: PawlVerdictRef{BeadID: "ag-uad", HeadSHA: "sha-refd"},
	}); err != nil {
		t.Fatal(err)
	}
	// productive-phase usage with an optimistic productive hint.
	if _, err := w.AppendUsage(root, UsageInput{
		BeadID: "ag-uad", RunID: "r1", TokensIn: 0, TokensOut: 800,
		CostUSD: 0, WallClockS: 5, Model: "m", Phase: PhaseImplement,
		CategoryHint: CategoryProductive,
	}); err != nil {
		t.Fatal(err)
	}
	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	g := ComputeGauges(l, "r1", 0, false)
	if g.A != 0 || g.Unadmitted != 1 {
		t.Errorf("A=%d Unadmitted=%d, want 0/1 (unadmitted accept)", g.A, g.Unadmitted)
	}
	// the unadmitted-accept bead is never-accepted → its spend is rejected-loss.
	if g.LCategory.Productive != 0 {
		t.Errorf("productive = %d, want 0 (unadmitted bead spend must not be productive)", g.LCategory.Productive)
	}
	if g.LCategory.Rejected != 800 {
		t.Errorf("rejected loss = %d, want 800 (unadmitted bead spend is loss)", g.LCategory.Rejected)
	}
}

// emitVerdict appends one gate-verdict through the PRODUCTION writer so the
// catch-rate fixtures are built by round-tripping the real persisted shape, not
// a hand-built in-memory Ledger (go.md Fixture Fidelity).
func emitVerdict(t *testing.T, w Writer, root, bead, disp string, crossFamily bool) {
	t.Helper()
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: bead, RunID: "r1", Difficulty: 1,
		PawlVerdictRef:  PawlVerdictRef{BeadID: bead, HeadSHA: "sha-" + bead},
		Disposition:     disp,
		HeadSHA:         "sha-" + bead,
		Attempt:         1,
		AuthorContextID: "ctx-1",
		AuthorFamily:    "claude",
		RefuterFamilies: []string{"gpt"},
		CrossFamily:     crossFamily,
	}); err != nil {
		t.Fatal(err)
	}
}

// TestGauge_CatchRate verifies catch_rate = REFUTED / (REFUTED + CONFIRMED) over
// a round-tripped real fixture of 6 REFUTED + 3 CONFIRMED → 0.667 (the real
// ledger's actual composition this session).
func TestGauge_CatchRate(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	for i := 0; i < 6; i++ {
		emitVerdict(t, w, root, fmt.Sprintf("ag-ref-%d", i), DispositionRefuted, true)
	}
	for i := 0; i < 3; i++ {
		emitVerdict(t, w, root, fmt.Sprintf("ag-con-%d", i), DispositionConfirmed, true)
	}

	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	g := ComputeGauges(l, "r1", 0, false)

	if g.Refuted != 6 {
		t.Errorf("Refuted = %d, want 6", g.Refuted)
	}
	if g.Confirmed != 3 {
		t.Errorf("Confirmed = %d, want 3", g.Confirmed)
	}
	if g.CatchRate == nil {
		t.Fatalf("CatchRate = nil, want ~0.667")
	}
	if !approx(*g.CatchRate, 6.0/9.0) {
		t.Errorf("CatchRate = %v, want %v (6/9)", *g.CatchRate, 6.0/9.0)
	}
	if g.CatchRateNote != "" {
		t.Errorf("CatchRateNote = %q, want empty (denom > 0)", g.CatchRateNote)
	}
}

// TestGauge_CatchRate_DivGuard verifies the 0/0 guard: zero adjudicated verdicts
// (only ESCALATE/HOLD) → CatchRate nil + note, never a divide-by-zero.
func TestGauge_CatchRate_DivGuard(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	emitVerdict(t, w, root, "ag-esc", DispositionEscalate, false)
	emitVerdict(t, w, root, "ag-hold", DispositionHold, false)

	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	g := ComputeGauges(l, "r1", 0, false)

	if g.Refuted != 0 || g.Confirmed != 0 {
		t.Errorf("Refuted/Confirmed = %d/%d, want 0/0", g.Refuted, g.Confirmed)
	}
	if g.CatchRate != nil {
		t.Errorf("CatchRate = %v, want nil (0 adjudicated)", *g.CatchRate)
	}
	if g.CatchRateNote != "no confirmed+refuted gate-verdicts" {
		t.Errorf("CatchRateNote = %q, want the divide-guard note", g.CatchRateNote)
	}
	if g.CatchRateCrossFamily != nil {
		t.Errorf("CatchRateCrossFamily = %v, want nil", *g.CatchRateCrossFamily)
	}
}

// TestGauge_CatchRate_CrossFamilySplit verifies the cross-family-restricted rate
// differs correctly from the overall rate. Overall: 4 REFUTED + 2 CONFIRMED =
// 0.667. Cross-family subset: 2 REFUTED + 2 CONFIRMED = 0.500.
func TestGauge_CatchRate_CrossFamilySplit(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	// 2 cross-family REFUTED, 2 single-family REFUTED.
	emitVerdict(t, w, root, "ag-rxf-0", DispositionRefuted, true)
	emitVerdict(t, w, root, "ag-rxf-1", DispositionRefuted, true)
	emitVerdict(t, w, root, "ag-rsf-0", DispositionRefuted, false)
	emitVerdict(t, w, root, "ag-rsf-1", DispositionRefuted, false)
	// 2 cross-family CONFIRMED.
	emitVerdict(t, w, root, "ag-cxf-0", DispositionConfirmed, true)
	emitVerdict(t, w, root, "ag-cxf-1", DispositionConfirmed, true)

	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	g := ComputeGauges(l, "r1", 0, false)

	if g.Refuted != 4 || g.Confirmed != 2 {
		t.Fatalf("Refuted/Confirmed = %d/%d, want 4/2", g.Refuted, g.Confirmed)
	}
	if g.CatchRate == nil || !approx(*g.CatchRate, 4.0/6.0) {
		t.Errorf("CatchRate = %v, want %v (4/6)", g.CatchRate, 4.0/6.0)
	}
	if g.CatchRateCrossFamily == nil {
		t.Fatalf("CatchRateCrossFamily = nil, want 0.5")
	}
	if !approx(*g.CatchRateCrossFamily, 0.5) {
		t.Errorf("CatchRateCrossFamily = %v, want 0.5 (2 refuted / 4 adjudicated cross-family)", *g.CatchRateCrossFamily)
	}
	// The two rates MUST differ — this is the whole point of the split.
	if approx(*g.CatchRate, *g.CatchRateCrossFamily) {
		t.Errorf("overall (%v) and cross-family (%v) catch-rate should differ", *g.CatchRate, *g.CatchRateCrossFamily)
	}
}
