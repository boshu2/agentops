// practices: [agile-manifesto, dora-metrics]
package main

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/types"
)

// --- classifyByPhase ---

func TestClassifyByPhase_UnknownPhase(t *testing.T) {
	got := classifyByPhase(99, "FAIL")
	if got != "" {
		t.Errorf("classifyByPhase(99, FAIL) = %q, want empty", got)
	}
}

func TestClassifyByPhase_Phase2NonBlockedOrPartial(t *testing.T) {
	got := classifyByPhase(2, "FAIL")
	if got != "" {
		t.Errorf("classifyByPhase(2, FAIL) = %q, want empty (only BLOCKED/PARTIAL handled for phase 2)", got)
	}
}

// --- classifyByVerdict ---

func TestClassifyByVerdict_Timeout(t *testing.T) {
	got := classifyByVerdict(string(failReasonTimeout))
	if got != types.MemRLFailureClassPhaseTimeout {
		t.Errorf("got %q, want %q", got, types.MemRLFailureClassPhaseTimeout)
	}
}

func TestClassifyByVerdict_Stall(t *testing.T) {
	got := classifyByVerdict(string(failReasonStall))
	if got != types.MemRLFailureClassPhaseStall {
		t.Errorf("got %q, want %q", got, types.MemRLFailureClassPhaseStall)
	}
}

func TestClassifyByVerdict_ExitError(t *testing.T) {
	got := classifyByVerdict(string(failReasonExit))
	if got != types.MemRLFailureClassPhaseExitError {
		t.Errorf("got %q, want %q", got, types.MemRLFailureClassPhaseExitError)
	}
}

func TestClassifyByVerdict_UnknownVerdict(t *testing.T) {
	got := classifyByVerdict("SOMETHING_ELSE")
	if got != types.MemRLFailureClass("something_else") {
		t.Errorf("got %q, want lowercase version", got)
	}
}
