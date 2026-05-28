// practices: [dora-metrics, sre]
package main

import (
	"encoding/json"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

func TestNudgeResult_JSONMarshaling(t *testing.T) {
	result := NudgeResult{
		Status:         "COMPOUNDING",
		Velocity:       0.15,
		EscapeVelocity: true,
		SessionsCount:  42,
		LearningsCount: 18,
		RPIState: RPIState{
			LastStep: "research",
			NextStep: "plan",
			Artifact: ".agents/research/foo.md",
			Skill:    "/plan",
		},
		PoolPending:     5,
		PoolApproaching: 2,
		Suggestion:      "Resume /plan — research complete",
	}

	// Marshal to JSON
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Unmarshal back
	var decoded NudgeResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	// Verify round-trip
	if decoded.Status != result.Status {
		t.Errorf("status mismatch: got %s, want %s", decoded.Status, result.Status)
	}
	if decoded.Velocity != result.Velocity {
		t.Errorf("velocity mismatch: got %.2f, want %.2f", decoded.Velocity, result.Velocity)
	}
	if decoded.EscapeVelocity != result.EscapeVelocity {
		t.Errorf("escape_velocity mismatch: got %v, want %v", decoded.EscapeVelocity, result.EscapeVelocity)
	}
	if decoded.RPIState.LastStep != result.RPIState.LastStep {
		t.Errorf("rpi_state.last_step mismatch: got %s, want %s", decoded.RPIState.LastStep, result.RPIState.LastStep)
	}
	if decoded.Suggestion != result.Suggestion {
		t.Errorf("suggestion mismatch: got %s, want %s", decoded.Suggestion, result.Suggestion)
	}
}

func TestDetermineNextStep(t *testing.T) {
	tests := []struct {
		name     string
		lastStep string
		want     string
	}{
		{"empty start", "", "research"},
		{"after research", "research", "pre-mortem"},
		{"after pre-mortem", "pre-mortem", "plan"},
		{"after plan", "plan", "implement"},
		{"after implement", "implement", "vibe"},
		{"after crank", "crank", "vibe"},
		{"after vibe", "vibe", "post-mortem"},
		{"after post-mortem", "post-mortem", "research"},
		{"unknown step", "unknown", "research"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineNextStep(tt.lastStep)
			if got != tt.want {
				t.Errorf("determineNextStep(%q) = %q, want %q", tt.lastStep, got, tt.want)
			}
		})
	}
}

func TestStepToSkill(t *testing.T) {
	tests := []struct {
		name string
		step string
		want string
	}{
		{"research", "research", "/research"},
		{"pre-mortem", "pre-mortem", "/pre-mortem"},
		{"plan", "plan", "/plan"},
		{"implement", "implement", "/implement"},
		{"crank", "crank", "/crank"},
		{"vibe", "vibe", "/vibe"},
		{"post-mortem", "post-mortem", "/post-mortem"},
		{"unknown", "unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stepToSkill(tt.step)
			if got != tt.want {
				t.Errorf("stepToSkill(%q) = %q, want %q", tt.step, got, tt.want)
			}
		})
	}
}

func TestBuildSuggestion(t *testing.T) {
	tests := []struct {
		name            string
		metrics         *types.FlywheelMetrics
		rpiState        RPIState
		poolPending     int
		poolApproaching int
		wantContains    string
	}{
		{
			name:            "pool approaching threshold",
			metrics:         &types.FlywheelMetrics{AboveEscapeVelocity: true},
			rpiState:        RPIState{},
			poolPending:     3,
			poolApproaching: 2,
			wantContains:    "approaching auto-promote",
		},
		{
			name:            "many pending candidates",
			metrics:         &types.FlywheelMetrics{AboveEscapeVelocity: true},
			rpiState:        RPIState{},
			poolPending:     10,
			poolApproaching: 0,
			wantContains:    "pending pool candidates",
		},
		{
			name:            "resume workflow",
			metrics:         &types.FlywheelMetrics{AboveEscapeVelocity: true},
			rpiState:        RPIState{LastStep: "research", NextStep: "plan", Skill: "/plan"},
			poolPending:     2,
			poolApproaching: 0,
			wantContains:    "Resume /plan",
		},
		{
			name: "low sigma",
			metrics: &types.FlywheelMetrics{
				AboveEscapeVelocity: false,
				Sigma:               0.2,
				Rho:                 0.6,
			},
			rpiState:        RPIState{},
			poolPending:     0,
			poolApproaching: 0,
			wantContains:    "ao lookup",
		},
		{
			name: "low rho",
			metrics: &types.FlywheelMetrics{
				AboveEscapeVelocity: false,
				Sigma:               0.5,
				Rho:                 0.3,
			},
			rpiState:        RPIState{},
			poolPending:     0,
			poolApproaching: 0,
			wantContains:    "Cite more learnings",
		},
		{
			name: "healthy flywheel",
			metrics: &types.FlywheelMetrics{
				AboveEscapeVelocity: true,
			},
			rpiState:        RPIState{},
			poolPending:     0,
			poolApproaching: 0,
			wantContains:    "Knowledge compounding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSuggestion(tt.metrics, tt.rpiState, tt.poolPending, tt.poolApproaching)
			if got == "" {
				t.Errorf("buildSuggestion returned empty string")
			}
			// Check if suggestion contains expected text
			if tt.wantContains != "" {
				if !contains(got, tt.wantContains) {
					t.Errorf("buildSuggestion() = %q, want to contain %q", got, tt.wantContains)
				}
			}
		})
	}
}

func TestBuildRPIState_EmptyChain(t *testing.T) {
	chain := &ratchet.Chain{}
	state := buildRPIState(chain)

	if state.LastStep != "" {
		t.Errorf("expected empty LastStep, got %q", state.LastStep)
	}
	if state.NextStep != "research" {
		t.Errorf("expected NextStep=research, got %q", state.NextStep)
	}
	if state.Skill != "/research" {
		t.Errorf("expected Skill=/research, got %q", state.Skill)
	}
}

func TestBuildRPIState_NilChain(t *testing.T) {
	state := buildRPIState(nil)

	if state.LastStep != "" {
		t.Errorf("expected empty LastStep, got %q", state.LastStep)
	}
	if state.NextStep != "research" {
		t.Errorf("expected NextStep=research, got %q", state.NextStep)
	}
	if state.Skill != "/research" {
		t.Errorf("expected Skill=/research, got %q", state.Skill)
	}
}

// contains checks if s contains substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

// findSubstring is a simple substring search.
func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
