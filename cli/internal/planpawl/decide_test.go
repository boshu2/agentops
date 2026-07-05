package planpawl

import (
	"strings"
	"testing"
)

// These tests are the executable form of plan-pawl.feature. They define the
// deterministic quorum/round/breaker contract: PASS on no-FAIL with quorum,
// auto-redo on FAIL, BLOCKED when a circuit breaker trips.

func v(family string, d Disposition) JudgeVerdict {
	return JudgeVerdict{Family: family, Disposition: d}
}

func TestDecide_QuorumPass(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), v("gpt", PASS)},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionPass {
		t.Fatalf("want PASS, got %s (%s)", out.Decision, out.Reason)
	}
}

func TestDecide_JudgmentWarnDoesNotBlockPass(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), {Family: "gpt", Disposition: WARN, WarnClass: Judgment}},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionPass {
		t.Fatalf("want PASS, got %s", out.Decision)
	}
	if len(out.SurfacedWarns) != 1 {
		t.Fatalf("want 1 surfaced judgment WARN, got %d", len(out.SurfacedWarns))
	}
}

func TestDecide_AutoRedoOnFail(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), v("gpt", FAIL)},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionRedo {
		t.Fatalf("want REDO, got %s", out.Decision)
	}
	if out.BreakerTripped != "" {
		t.Fatalf("breaker should not trip on a plain FAIL with rounds left, got %q", out.BreakerTripped)
	}
}

func TestDecide_MechanicalWarnAutoApplied(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), {Family: "gpt", Disposition: WARN, WarnClass: Mechanical}},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionRedo {
		t.Fatalf("want REDO (re-judge after auto-apply), got %s", out.Decision)
	}
	if len(out.AutoApplied) != 1 {
		t.Fatalf("want 1 auto-applied mechanical WARN, got %d", len(out.AutoApplied))
	}
}

func TestDecide_BlockedOnRoundOverMax(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), v("gpt", FAIL)},
		Round:     4,
		MaxRounds: 3,
	})
	if out.Decision != DecisionBlocked {
		t.Fatalf("want BLOCKED, got %s", out.Decision)
	}
	if out.BreakerTripped != "max-attempts" {
		t.Fatalf("want breaker max-attempts, got %q", out.BreakerTripped)
	}
}

func TestDecide_BlockedOnJudgmentFlag(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), {Family: "gpt", Disposition: PASS, JudgmentFlag: true}},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionBlocked {
		t.Fatalf("want BLOCKED, got %s", out.Decision)
	}
	if out.BreakerTripped != "judgment-flag" {
		t.Fatalf("want breaker judgment-flag, got %q", out.BreakerTripped)
	}
}

func TestDecide_BlockedOnOscillation(t *testing.T) {
	out := Decide(Input{
		Verdicts:    []JudgeVerdict{v("claude", PASS), v("gpt", FAIL)},
		Round:       2,
		MaxRounds:   3,
		Oscillation: true,
	})
	if out.Decision != DecisionBlocked {
		t.Fatalf("want BLOCKED, got %s", out.Decision)
	}
	if out.BreakerTripped != "oscillation" {
		t.Fatalf("want breaker oscillation, got %q", out.BreakerTripped)
	}
}

// Fail-closed (codex refuter round 5): a malformed round budget (round < 1 or
// max-rounds < 1) must BLOCK, never reach a silent PASS.
func TestDecide_BlockedOnInvalidRound(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), v("gpt", PASS)},
		Round:     0,
		MaxRounds: 3,
	})
	if out.Decision != DecisionBlocked || out.BreakerTripped != "invalid-input" {
		t.Fatalf("round 0 must BLOCK (invalid-input), got %s/%q", out.Decision, out.BreakerTripped)
	}
}

func TestDecide_BlockedOnInvalidMaxRounds(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), v("gpt", PASS)},
		Round:     1,
		MaxRounds: 0,
	})
	if out.Decision != DecisionBlocked || out.BreakerTripped != "invalid-input" {
		t.Fatalf("max-rounds 0 must BLOCK (invalid-input), got %s/%q", out.Decision, out.BreakerTripped)
	}
}

func TestDecide_QuorumNotMetSingleFamily(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS)},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionRedo {
		t.Fatalf("want REDO (quorum not met), got %s", out.Decision)
	}
	if out.Reason == "" {
		t.Fatalf("want a reason mentioning quorum")
	}
}

// Two panes of the SAME family do not satisfy the multi-model quorum floor.
func TestDecide_QuorumNotMetSameFamilyTwice(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), v("fable", PASS)}, // both normalize to claude
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionRedo {
		t.Fatalf("want REDO (one distinct family != quorum), got %s", out.Decision)
	}
}

// An unknown/off-roster family is rejected and cannot help meet quorum.
func TestDecide_OffRosterFamilyRejected(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), v("llama", PASS)},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionRedo {
		t.Fatalf("want REDO (off-roster family doesn't count), got %s", out.Decision)
	}
}

// Fail-closed (codex refuter round 4): an off-roster pane alongside a satisfied
// quorum must NOT slip through as a redundant PASS — it fail-closes to REDO so a
// junk/misconfigured pane can never pad a quorum.
func TestDecide_OffRosterPaneCannotPadQuorum(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), v("gpt", PASS), v("llama", PASS)},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionRedo {
		t.Fatalf("an off-roster pane must fail-closed to REDO even with a satisfied quorum, got %s (%s)", out.Decision, out.Reason)
	}
}

// Fail-closed (codex refuter round 2): a missing/empty disposition must NOT be
// treated as a clean PASS — it counts as a FAIL so two bad files can't exit PASS.
func TestDecide_FailClosedOnMissingDisposition(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{{Family: "claude"}, {Family: "gpt"}}, // no disposition set
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision == DecisionPass {
		t.Fatalf("missing disposition must NOT PASS (fail-closed), got %s", out.Decision)
	}
}

func TestDecide_FailClosedOnGarbageDisposition(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), {Family: "gpt", Disposition: Disposition("approved")}},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionRedo {
		t.Fatalf("garbage disposition must fail-closed to REDO, got %s", out.Decision)
	}
}

// Valid dispositions are matched case-insensitively (a judge JSON may write "pass").
func TestDecide_DispositionCaseInsensitive(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{{Family: "claude", Disposition: Disposition("pass")}, {Family: "gpt", Disposition: Disposition("Pass")}},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionPass {
		t.Fatalf("lowercase/mixed-case PASS should PASS, got %s (%s)", out.Decision, out.Reason)
	}
}

// Fail-closed (codex refuter round 3): a WARN must EXPLICITLY declare its class to
// be non-blocking. A WARN with a missing warn_class must not silently get the lenient
// judgment-and-PASS path — it fail-closes to REDO.
func TestDecide_FailClosedOnWarnMissingClass(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), {Family: "gpt", Disposition: WARN}}, // no warn_class
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionRedo {
		t.Fatalf("WARN with no warn_class must fail-closed to REDO, got %s (%s)", out.Decision, out.Reason)
	}
}

func TestDecide_FailClosedOnWarnUnknownClass(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), {Family: "gpt", Disposition: WARN, WarnClass: WarnClass("cosmetic")}},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionRedo {
		t.Fatalf("WARN with unknown warn_class must fail-closed to REDO, got %s", out.Decision)
	}
}

// An explicitly-declared judgment WARN is still non-blocking and matched case-insensitively.
func TestDecide_JudgmentWarnClassCaseInsensitive(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", PASS), {Family: "gpt", Disposition: WARN, WarnClass: WarnClass("Judgment")}},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionPass {
		t.Fatalf("explicit judgment WARN should still PASS, got %s", out.Decision)
	}
}

// Breaker precedence: an explicit hard breaker (judgment flag) wins even when a
// FAIL is also present.
func TestDecide_JudgmentFlagBeatsFail(t *testing.T) {
	out := Decide(Input{
		Verdicts:  []JudgeVerdict{v("claude", FAIL), {Family: "gpt", Disposition: FAIL, JudgmentFlag: true}},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionBlocked || out.BreakerTripped != "judgment-flag" {
		t.Fatalf("want BLOCKED/judgment-flag, got %s/%q", out.Decision, out.BreakerTripped)
	}
}

// --- age-gascity-port-slate-irye.2: degradation-aware Decide ------------------
// A judge lane that infrastructure-failed (provider timeout / rate limit / no
// verdict) is a retryable outage, NOT a refutation. These tests pin the none /
// transient / hard classification and the DEGRADED decision.

// TestClassifyFailure_Table is the token-table contract for ClassifyFailure.
func TestClassifyFailure_Table(t *testing.T) {
	cases := []struct {
		name       string
		class      FailureClass
		reason     string
		wantClass  FailureClass
		wantReason string
	}{
		// The transient reason token table (empty class -> classify by reason).
		{"reason_rate_limited", "", "rate_limited", FailureTransient, "rate_limited"},
		{"reason_provider_rate_limited", "", "provider_rate_limited", FailureTransient, "provider_rate_limited"},
		{"reason_provider_unavailable", "", "provider_unavailable", FailureTransient, "provider_unavailable"},
		{"reason_provider_timeout", "", "provider_timeout", FailureTransient, "provider_timeout"},
		{"reason_temporary_unavailable", "", "temporary_unavailable", FailureTransient, "temporary_unavailable"},
		{"reason_transport_interrupted", "", "transport_interrupted", FailureTransient, "transport_interrupted"},
		{"reason_timeout", "", "timeout", FailureTransient, "timeout"},
		{"reason_no_verdict", "", "no_verdict", FailureTransient, "no_verdict"},
		// Case-insensitive normalization of the reason token.
		{"reason_case_insensitive", "", "Provider_Timeout", FailureTransient, "provider_timeout"},
		// Explicit class wins over the reason table.
		{"explicit_transient", "transient", "whatever_reason", FailureTransient, "whatever_reason"},
		{"explicit_hard", "hard", "disk_full", FailureHard, "disk_full"},
		{"explicit_transient_case", "Transient", "x", FailureTransient, "x"},
		// Unrecognized class token WITH a reason -> transient (fail-safe: never a
		// false refute).
		{"unknown_class_with_reason", "flaky", "provider_timeout", FailureTransient, "provider_timeout"},
		{"unknown_class_with_arbitrary_reason", "weird", "something_odd", FailureTransient, "something_odd"},
		// Empty class + off-table reason -> hard.
		{"empty_class_off_table_reason", "", "disk_full", FailureHard, "disk_full"},
		// No class and no reason -> none (a clean lane).
		{"empty_empty_none", "", "", FailureNone, ""},
		// Explicit none stays none.
		{"explicit_none", "none", "", FailureNone, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotClass, gotReason := ClassifyFailure(tc.class, tc.reason)
			if gotClass != tc.wantClass {
				t.Fatalf("ClassifyFailure(%q,%q) class = %q, want %q", tc.class, tc.reason, gotClass, tc.wantClass)
			}
			if gotReason != tc.wantReason {
				t.Fatalf("ClassifyFailure(%q,%q) reason = %q, want %q", tc.class, tc.reason, gotReason, tc.wantReason)
			}
		})
	}
}

// The age-5olx incident replay: a warm panel where codex+agy timed out was
// recorded REFUTED at 1/3 coverage. With the fix, the two timed-out lanes are
// transient (not refutations), coverage drops below quorum, and the decision is
// DEGRADED (retryable — re-run the panel) — never REDO/REFUTED.
func TestDecide_Age5olxIncidentReplay(t *testing.T) {
	out := Decide(Input{
		Verdicts: []JudgeVerdict{
			v("claude", PASS),
			v("codex", Disposition("<timeout>")),
			v("agy", Disposition("<timeout>")),
		},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionDegraded {
		t.Fatalf("want DEGRADED (transient lane loss below quorum), got %s (%s)", out.Decision, out.Reason)
	}
	if !out.Degraded {
		t.Fatalf("want Degraded=true, got false")
	}
	if !containsAll(out.DegradedFamilies, "gpt", "gemini") {
		t.Fatalf("want degraded families [gpt gemini], got %v", out.DegradedFamilies)
	}
	if !strings.Contains(out.Reason, "degraded") || !strings.Contains(out.Reason, "retryable") {
		t.Fatalf("reason must name degraded coverage + retryable, got %q", out.Reason)
	}
}

// A transient lane alongside two SURVIVING distinct families that both PASS still
// PASSes — the floor is met by the survivors — but records Degraded=true so the
// coverage loss is honest.
func TestDecide_TransientLaneSurvivingQuorumPasses(t *testing.T) {
	out := Decide(Input{
		Verdicts: []JudgeVerdict{
			v("claude", PASS),
			v("gpt", PASS),
			v("agy", Disposition("<timeout>")),
		},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionPass {
		t.Fatalf("want PASS (floor met by survivors), got %s (%s)", out.Decision, out.Reason)
	}
	if !out.Degraded {
		t.Fatalf("want Degraded=true on a transient lane loss, got false")
	}
	if len(out.DegradedFamilies) != 1 || out.DegradedFamilies[0] != "gemini" {
		t.Fatalf("want degraded families [gemini], got %v", out.DegradedFamilies)
	}
}

// Every pawl-review.sh transport sentinel is recognized as a transient
// disposition, not a refutation. Each token, alongside a surviving quorum, PASSes
// with Degraded=true.
func TestDecide_TransientDispositionTokens(t *testing.T) {
	for _, tok := range []string{"<timeout>", "timeout", "no verdict", "NO-VERDICT", "no_verdict"} {
		t.Run(tok, func(t *testing.T) {
			out := Decide(Input{
				Verdicts: []JudgeVerdict{
					v("claude", PASS),
					v("gpt", PASS),
					v("agy", Disposition(tok)),
				},
				Round:     1,
				MaxRounds: 3,
			})
			if out.Decision != DecisionPass {
				t.Fatalf("token %q: want PASS, got %s (%s)", tok, out.Decision, out.Reason)
			}
			if !out.Degraded || len(out.DegradedFamilies) != 1 || out.DegradedFamilies[0] != "gemini" {
				t.Fatalf("token %q: want Degraded=true families=[gemini], got %v / %v", tok, out.Degraded, out.DegradedFamilies)
			}
		})
	}
}

// A transient failure declared via the failure fields (not the disposition
// sentinel) is treated identically: excluded from fails and from coverage.
func TestDecide_TransientFailureClassFields(t *testing.T) {
	out := Decide(Input{
		Verdicts: []JudgeVerdict{
			v("claude", PASS),
			{Family: "gpt", Disposition: PASS, FailureClass: FailureTransient, FailureReason: "rate_limited"},
			{Family: "agy", Disposition: PASS, FailureReason: "provider_timeout"}, // empty class, transient reason
		},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionDegraded {
		t.Fatalf("want DEGRADED (only claude survives), got %s (%s)", out.Decision, out.Reason)
	}
	if !containsAll(out.DegradedFamilies, "gpt", "gemini") {
		t.Fatalf("want degraded [gpt gemini], got %v", out.DegradedFamilies)
	}
}

// A verdict whose class token is UNRECOGNIZED but carries a reason is transient
// (fail-safe), so a surviving quorum still PASSes with Degraded=true.
func TestDecide_UnknownClassTokenWithReasonIsTransient(t *testing.T) {
	out := Decide(Input{
		Verdicts: []JudgeVerdict{
			v("claude", PASS),
			v("gpt", PASS),
			{Family: "agy", Disposition: PASS, FailureClass: FailureClass("flaky"), FailureReason: "provider_timeout"},
		},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionPass || !out.Degraded {
		t.Fatalf("want PASS+Degraded (unknown class w/ reason = transient), got %s Degraded=%v", out.Decision, out.Degraded)
	}
}

// An explicit HARD failure class is a refutation, fail-closed to REDO (unchanged).
func TestDecide_ExplicitHardFailureRedoes(t *testing.T) {
	out := Decide(Input{
		Verdicts: []JudgeVerdict{
			v("claude", PASS),
			{Family: "gpt", Disposition: PASS, FailureClass: FailureHard, FailureReason: "auth_error"},
		},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionRedo {
		t.Fatalf("want REDO (hard failure = refutation), got %s (%s)", out.Decision, out.Reason)
	}
	if out.Degraded {
		t.Fatalf("a hard failure is not a degraded/transient lane; want Degraded=false")
	}
}

// Empty class + an off-table reason classifies HARD -> REDO (fail-closed).
func TestDecide_EmptyClassOffTableReasonHardRedoes(t *testing.T) {
	out := Decide(Input{
		Verdicts: []JudgeVerdict{
			v("claude", PASS),
			{Family: "gpt", Disposition: PASS, FailureReason: "disk_full"},
		},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionRedo {
		t.Fatalf("want REDO (off-table reason = hard), got %s (%s)", out.Decision, out.Reason)
	}
}

// Regression pin: a garbage disposition with NO failure fields and NOT a transient
// token stays fail-closed (REDO) — degradation handling must not rescue it.
func TestDecide_GarbageDispositionNoFailureFieldsStillRedoes(t *testing.T) {
	out := Decide(Input{
		Verdicts: []JudgeVerdict{
			v("claude", PASS),
			{Family: "gpt", Disposition: Disposition("approved")},
		},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionRedo {
		t.Fatalf("garbage disposition must fail-closed to REDO, got %s (%s)", out.Decision, out.Reason)
	}
	if out.Degraded || len(out.DegradedFamilies) != 0 {
		t.Fatalf("garbage disposition is not degraded; want Degraded=false families=[], got %v / %v", out.Degraded, out.DegradedFamilies)
	}
}

// Precedence: a GENUINE FAIL outranks transient degradation — REDO, not DEGRADED.
func TestDecide_GenuineFailBeatsDegradation(t *testing.T) {
	out := Decide(Input{
		Verdicts: []JudgeVerdict{
			v("claude", FAIL),
			v("gpt", Disposition("<timeout>")),
			v("agy", Disposition("<timeout>")),
		},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionRedo {
		t.Fatalf("a genuine FAIL must REDO over degradation, got %s (%s)", out.Decision, out.Reason)
	}
}

// Precedence: a hard breaker (judgment flag) outranks transient degradation.
func TestDecide_BreakerBeatsDegradation(t *testing.T) {
	out := Decide(Input{
		Verdicts: []JudgeVerdict{
			{Family: "claude", Disposition: PASS, JudgmentFlag: true},
			v("gpt", Disposition("<timeout>")),
		},
		Round:     1,
		MaxRounds: 3,
	})
	if out.Decision != DecisionBlocked || out.BreakerTripped != "judgment-flag" {
		t.Fatalf("a breaker must beat degradation, got %s/%q", out.Decision, out.BreakerTripped)
	}
}

// containsAll reports whether every want is present in got (order-independent).
func containsAll(got []string, want ...string) bool {
	set := map[string]bool{}
	for _, g := range got {
		set[g] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}
