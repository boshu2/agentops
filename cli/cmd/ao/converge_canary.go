package main

// Canary entry gate for `ao converge`. The lesson it encodes: an empty/PASS gate
// result is a lie until proven to bite. Before any judge dispatch, converge runs
// this canary — it feeds the gate a known-BAD fixture (a self-judge verdict) AND
// a known-GOOD fixture (an independent-context verdict) and only proceeds if the
// gate REJECTS the bad one AND ACCEPTS the good one. A gate that cannot reject a
// planted positive (or that rejects everything) fails the canary and aborts the
// run. Precedent: chaos-test / tickSmoke plant fixtures and assert reject codes.

import (
	"fmt"

	verdictparse "github.com/boshu2/agentops/cli/internal/verdict"
)

// convergeCanaryRejectCode is the sentinel reject code a gate returns for a
// planted positive (distinct, non-zero; aligned with the tickExitCouncil family).
const convergeCanaryRejectCode = tickExitCouncil

// convergeCanaryGate is the gate-under-test interface: given a verdict body it
// reports whether the verdict is rejected and, if so, the reject code. It is
// injectable so the canary can also exercise a deliberately-broken gate.
type convergeCanaryGate func(verdictBody string) (rejected bool, code int)

// convergeProductionCanaryGate is the REAL entry gate. It parses the verdict via
// the live tickVerdictIdentity parser and rejects when the verdict has ANY
// identity gap — in particular a self-judge (context_id == author_context_id) or
// a missing context_id. An independent-context verdict with a complete identity
// is accepted.
var convergeProductionCanaryGate convergeCanaryGate = func(verdictBody string) (bool, int) {
	_, gaps := verdictparse.Identity(verdictBody)
	if len(gaps) > 0 {
		return true, convergeCanaryRejectCode
	}
	return false, 0
}

// convergeCanaryResult is the outcome of the canary. Proceed gates the real run:
// converge dispatches no judge unless Proceed is true.
type convergeCanaryResult struct {
	Passed  bool
	Message string
	Proceed bool
}

// convergeRunCanary feeds the gate a known-BAD and a known-GOOD fixture
// (two-sided). It passes iff the gate REJECTS the bad fixture AND ACCEPTS the
// good fixture. If the bad fixture is not rejected, the run aborts (Proceed
// false) with a message stating the gate did not reject a known-bad verdict. If
// the good fixture is rejected (a degenerate all-reject gate), the canary fails —
// an all-reject gate gives false confidence and is not a working gate.
func convergeRunCanary(gate convergeCanaryGate) convergeCanaryResult {
	badRejected, _ := gate(plantedSelfJudgeCanaryVerdict())
	if !badRejected {
		return convergeCanaryResult{
			Passed:  false,
			Proceed: false,
			Message: "canary FAILED: gate did not reject a known-bad (self-judge) verdict — refusing to trust it",
		}
	}
	goodRejected, _ := gate(plantedGoodCanaryVerdict())
	if goodRejected {
		return convergeCanaryResult{
			Passed:  false,
			Proceed: false,
			Message: "canary FAILED: gate rejected a known-good (independent-context) verdict — an all-reject gate gives false confidence",
		}
	}
	return convergeCanaryResult{
		Passed:  true,
		Proceed: true,
		Message: fmt.Sprintf("canary PASSED: gate rejects a planted self-judge (code %d) and accepts an independent-context verdict", convergeCanaryRejectCode),
	}
}

// plantedSelfJudgeCanaryVerdict is the known-BAD fixture: judge context equals the
// author context (self-judge) — a correct gate must reject it.
func plantedSelfJudgeCanaryVerdict() string {
	return "author: codex\nauthor_context_id: ctx-author\n" +
		"judge: codex2\njudge_program: codex-cli\njudge_model_family: openai\n" +
		"context_id: ctx-author\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n"
}

// plantedGoodCanaryVerdict is the known-GOOD fixture: an independent judge context
// with a complete identity — a correct gate must accept it.
func plantedGoodCanaryVerdict() string {
	return "author: codex\nauthor_context_id: ctx-author\n" +
		"judge: athena\njudge_program: claude-code\njudge_model_family: claude\n" +
		"context_id: ctx-judge-1\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n"
}
