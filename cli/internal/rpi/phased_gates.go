package rpi

import (
	"fmt"
	"strings"

	"github.com/boshu2/agentops/cli/internal/types"
)

// MaxGateRetryDepth is the hard ceiling for gate retry attempts.
// Attempts 1 through MaxGateRetryDepth proceed normally; attempt
// MaxGateRetryDepth+1 forces escalation regardless of MemRL policy.
const MaxGateRetryDepth = 5

// ShouldForceEscalation returns true when the attempt count exceeds the hard ceiling.
func ShouldForceEscalation(attempt int) bool {
	return attempt > MaxGateRetryDepth
}

// GateFailError signals a gate check failure that may be retried.
type GateFailError struct {
	Phase    int
	Verdict  string
	Findings []Finding
	Report   string
}

func (e *GateFailError) Error() string {
	return fmt.Sprintf("gate FAIL at phase %d: %s (report: %s)", e.Phase, e.Verdict, e.Report)
}

// LegacyGateAction returns the pre-MemRL retry/escalate decision based on attempt count.
func LegacyGateAction(attempt, maxRetries int) types.MemRLAction {
	if attempt >= maxRetries {
		return types.MemRLActionEscalate
	}
	return types.MemRLActionRetry
}

// ClassifyGateFailureClass maps a phase number and gate error to a MemRL failure class.
func ClassifyGateFailureClass(phaseNum int, verdict string) types.MemRLFailureClass {
	verdict = strings.ToUpper(strings.TrimSpace(verdict))
	if fc := ClassifyByPhase(phaseNum, verdict); fc != "" {
		return fc
	}
	return ClassifyByVerdict(verdict)
}

// ClassifyByPhase returns a failure class based on phase-specific verdict rules.
func ClassifyByPhase(phaseNum int, verdict string) types.MemRLFailureClass {
	switch phaseNum {
	case 1:
		if verdict == "FAIL" {
			return types.MemRLFailureClassPreMortemFail
		}
	case 2:
		switch verdict {
		case "BLOCKED":
			return types.MemRLFailureClassCrankBlocked
		case "PARTIAL":
			return types.MemRLFailureClassCrankPartial
		}
	case 3:
		if verdict == "FAIL" {
			return types.MemRLFailureClassVibeFail
		}
	}
	return ""
}

// ClassifyByVerdict returns a failure class based on generic verdict strings.
// The failReason constants ("timeout", "stall", "exit_error") are lowercase,
// but the verdict passed here is already uppercased by ClassifyGateFailureClass.
// This means the case branches only match when the caller passes lowercase values.
func ClassifyByVerdict(verdict string) types.MemRLFailureClass {
	switch verdict {
	case "timeout":
		return types.MemRLFailureClassPhaseTimeout
	case "stall":
		return types.MemRLFailureClassPhaseStall
	case "exit_error":
		return types.MemRLFailureClassPhaseExitError
	default:
		return types.MemRLFailureClass(strings.ToLower(verdict))
	}
}
