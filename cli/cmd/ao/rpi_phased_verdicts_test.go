// practices: [agile-manifesto, dora-metrics]
package main

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/domain"
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
	if got != domain.MemRLFailureClassPhaseTimeout {
		t.Errorf("got %q, want %q", got, domain.MemRLFailureClassPhaseTimeout)
	}
}

func TestClassifyByVerdict_Stall(t *testing.T) {
	got := classifyByVerdict(string(failReasonStall))
	if got != domain.MemRLFailureClassPhaseStall {
		t.Errorf("got %q, want %q", got, domain.MemRLFailureClassPhaseStall)
	}
}

func TestClassifyByVerdict_ExitError(t *testing.T) {
	got := classifyByVerdict(string(failReasonExit))
	if got != domain.MemRLFailureClassPhaseExitError {
		t.Errorf("got %q, want %q", got, domain.MemRLFailureClassPhaseExitError)
	}
}

func TestClassifyByVerdict_UnknownVerdict(t *testing.T) {
	got := classifyByVerdict("SOMETHING_ELSE")
	if got != domain.MemRLFailureClass("something_else") {
		t.Errorf("got %q, want lowercase version", got)
	}
}
