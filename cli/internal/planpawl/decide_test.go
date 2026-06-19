package planpawl

import "testing"

// These tests are the executable form of plan-pawl.feature. They define the
// deterministic quorum/round/breaker contract: PASS on no-FAIL with quorum,
// auto-redo on FAIL, BLOCKED when a circuit breaker trips.

func v(family string, d Disposition) JudgeVerdict { return JudgeVerdict{Family: family, Disposition: d} }

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
