// practices: [agile-manifesto, dora-metrics]
package main

import (
	"fmt"
	"testing"

	"github.com/boshu2/agentops/cli/internal/types"
)

// Tests for pure helper functions in rpi_phased.go

func TestClassifyGateFailureClass(t *testing.T) {
	tests := []struct {
		name     string
		phaseNum int
		gateErr  *gateFailError
		want     types.MemRLFailureClass
	}{
		{
			name:     "nil error returns empty",
			phaseNum: 1,
			gateErr:  nil,
			want:     "",
		},
		{
			name:     "phase 1 FAIL → pre_mortem_fail",
			phaseNum: 1,
			gateErr:  &gateFailError{Phase: 1, Verdict: "FAIL"},
			want:     types.MemRLFailureClassPreMortemFail,
		},
		{
			name:     "phase 1 other verdict → not pre_mortem_fail",
			phaseNum: 1,
			gateErr:  &gateFailError{Phase: 1, Verdict: "BLOCKED"},
			want:     types.MemRLFailureClass("blocked"),
		},
		{
			name:     "phase 2 BLOCKED → crank_blocked",
			phaseNum: 2,
			gateErr:  &gateFailError{Phase: 2, Verdict: "BLOCKED"},
			want:     types.MemRLFailureClassCrankBlocked,
		},
		{
			name:     "phase 2 PARTIAL → crank_partial",
			phaseNum: 2,
			gateErr:  &gateFailError{Phase: 2, Verdict: "PARTIAL"},
			want:     types.MemRLFailureClassCrankPartial,
		},
		{
			name:     "phase 2 other verdict → lowercase",
			phaseNum: 2,
			gateErr:  &gateFailError{Phase: 2, Verdict: "CUSTOM"},
			want:     types.MemRLFailureClass("custom"),
		},
		{
			name:     "phase 3 FAIL → vibe_fail",
			phaseNum: 3,
			gateErr:  &gateFailError{Phase: 3, Verdict: "FAIL"},
			want:     types.MemRLFailureClassVibeFail,
		},
		{
			// Note: verdict is ToUpper'd before switch comparison; failReason constants are lowercase
			// so they hit the default case returning lowercase(verdict)
			name:     "lowercase timeout verdict falls to default lowercase",
			phaseNum: 1,
			gateErr:  &gateFailError{Phase: 1, Verdict: string(failReasonTimeout)},
			want:     types.MemRLFailureClass("timeout"),
		},
		{
			name:     "lowercase stall verdict falls to default lowercase",
			phaseNum: 1,
			gateErr:  &gateFailError{Phase: 1, Verdict: string(failReasonStall)},
			want:     types.MemRLFailureClass("stall"),
		},
		{
			name:     "lowercase exit_error verdict falls to default lowercase",
			phaseNum: 1,
			gateErr:  &gateFailError{Phase: 1, Verdict: string(failReasonExit)},
			want:     types.MemRLFailureClass("exit_error"),
		},
		{
			// Note: failReason constants are lowercase ("timeout", "stall", "exit_error").
			// The switch compares string(failReasonX) (lowercase) against verdict (uppercased).
			// Since they never match, all fall to default returning strings.ToLower(verdict).
			// This is the actual code behavior.
			name:     "TIMEOUT verdict falls to default (lowercase mismatch with constant)",
			phaseNum: 4,
			gateErr:  &gateFailError{Phase: 4, Verdict: "TIMEOUT"},
			want:     types.MemRLFailureClass("timeout"),
		},
		{
			name:     "STALL verdict falls to default",
			phaseNum: 4,
			gateErr:  &gateFailError{Phase: 4, Verdict: "STALL"},
			want:     types.MemRLFailureClass("stall"),
		},
		{
			name:     "EXIT_ERROR verdict falls to default",
			phaseNum: 4,
			gateErr:  &gateFailError{Phase: 4, Verdict: "EXIT_ERROR"},
			want:     types.MemRLFailureClass("exit_error"),
		},
		{
			name:     "verdict with leading/trailing whitespace trimmed",
			phaseNum: 3,
			gateErr:  &gateFailError{Phase: 3, Verdict: "  FAIL  "},
			want:     types.MemRLFailureClassVibeFail,
		},
		{
			name:     "lowercase verdict normalized to lowercase",
			phaseNum: 4,
			gateErr:  &gateFailError{Phase: 4, Verdict: "UNKNOWN_VERDICT"},
			want:     types.MemRLFailureClass("unknown_verdict"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyGateFailureClass(tt.phaseNum, tt.gateErr)
			if got != tt.want {
				t.Errorf("classifyGateFailureClass(%d, %v) = %q, want %q", tt.phaseNum, tt.gateErr, got, tt.want)
			}
		})
	}
}

func TestLegacyGateAction(t *testing.T) {
	tests := []struct {
		attempt    int
		maxRetries int
		want       types.MemRLAction
	}{
		{attempt: 0, maxRetries: 3, want: types.MemRLActionRetry},
		{attempt: 1, maxRetries: 3, want: types.MemRLActionRetry},
		{attempt: 2, maxRetries: 3, want: types.MemRLActionRetry},
		{attempt: 3, maxRetries: 3, want: types.MemRLActionEscalate},
		{attempt: 5, maxRetries: 3, want: types.MemRLActionEscalate},
		{attempt: 0, maxRetries: 0, want: types.MemRLActionEscalate},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt=%d max=%d", tt.attempt, tt.maxRetries), func(t *testing.T) {
			got := legacyGateAction(tt.attempt, tt.maxRetries)
			if got != tt.want {
				t.Errorf("legacyGateAction(%d, %d) = %q, want %q", tt.attempt, tt.maxRetries, got, tt.want)
			}
		})
	}
}

func TestProbeBackendCapabilities(t *testing.T) {
	t.Run("live status and stream mode", func(t *testing.T) {
		caps := probeBackendCapabilities(true, "stream")
		if !caps.LiveStatusEnabled {
			t.Error("expected LiveStatusEnabled=true")
		}
		if caps.RuntimeMode != "stream" {
			t.Errorf("RuntimeMode = %q, want %q", caps.RuntimeMode, "stream")
		}
	})

	t.Run("no live status and direct mode", func(t *testing.T) {
		caps := probeBackendCapabilities(false, "direct")
		if caps.LiveStatusEnabled {
			t.Error("expected LiveStatusEnabled=false")
		}
		if caps.RuntimeMode != "direct" {
			t.Errorf("RuntimeMode = %q, want %q", caps.RuntimeMode, "direct")
		}
	})

	t.Run("auto mode normalized", func(t *testing.T) {
		caps := probeBackendCapabilities(false, "AUTO")
		if caps.RuntimeMode != "auto" {
			t.Errorf("RuntimeMode = %q, want %q", caps.RuntimeMode, "auto")
		}
	})
}

func TestValidateRuntimeMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{"auto is valid", "auto", false},
		{"direct is valid", "direct", false},
		{"stream is valid", "stream", false},
		{"uppercase AUTO normalized", "AUTO", false},
		{"empty defaults to auto", "", false},
		{"invalid mode returns error", "invalid-mode", true},
		{"whitespace-only defaults to auto", "   ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRuntimeMode(tt.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRuntimeMode(%q) error = %v, wantErr %v", tt.mode, err, tt.wantErr)
			}
		})
	}
}

func TestEffectiveTmuxCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{"empty returns default tmux", "", "tmux"},
		{"whitespace-only returns tmux", "   ", "tmux"},
		{"custom command preserved", "my-tmux", "my-tmux"},
		{"command with args preserved", "/usr/local/bin/tmux", "/usr/local/bin/tmux"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveTmuxCommand(tt.command)
			if got != tt.want {
				t.Errorf("effectiveTmuxCommand(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}
