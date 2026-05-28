package eval

import (
	"encoding/json"
	"testing"
)

// Round-trip JSON snapshot tests for structs that gained new fields in
// the LID-primitives epic. Pattern enforces f-2026-04-26-001:
// any struct that gains a field must have a paired round-trip test that
// catches missing JSON tags / propagation gaps.

func TestSuiteEnvironmentRoundTripPreservesDisableHooks(t *testing.T) {
	cases := []struct {
		name     string
		env      SuiteEnvironment
		wantJSON string
	}{
		{
			name:     "default omits disable_hooks",
			env:      SuiteEnvironment{},
			wantJSON: `{}`,
		},
		{
			name:     "explicit false omits disable_hooks (omitempty)",
			env:      SuiteEnvironment{DisableHooks: false},
			wantJSON: `{}`,
		},
		{
			name:     "true emits disable_hooks",
			env:      SuiteEnvironment{DisableHooks: true},
			wantJSON: `{"disable_hooks":true}`,
		},
		{
			name: "preserves siblings",
			env: SuiteEnvironment{
				MaxAttempts:  3,
				DisableHooks: true,
			},
			wantJSON: `{"max_attempts":3,"disable_hooks":true}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.env)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if got := string(data); got != tc.wantJSON {
				t.Fatalf("marshal: got %s, want %s", got, tc.wantJSON)
			}
			var decoded SuiteEnvironment
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if decoded.DisableHooks != tc.env.DisableHooks {
				t.Fatalf("DisableHooks round-trip mismatch: got %v, want %v", decoded.DisableHooks, tc.env.DisableHooks)
			}
			if decoded.MaxAttempts != tc.env.MaxAttempts {
				t.Fatalf("MaxAttempts round-trip mismatch: got %d, want %d", decoded.MaxAttempts, tc.env.MaxAttempts)
			}
		})
	}
}

func TestEnvironmentRecordRoundTripPreservesHooksDisabled(t *testing.T) {
	cases := []struct {
		name                    string
		record                  EnvironmentRecord
		wantHooksDisabledInJSON bool
	}{
		{
			name:                    "false omits via omitempty",
			record:                  EnvironmentRecord{ScrubbedEnvPrefixes: []string{}, NetworkAccess: NetworkDisabled},
			wantHooksDisabledInJSON: false,
		},
		{
			name:                    "true is preserved through round-trip",
			record:                  EnvironmentRecord{ScrubbedEnvPrefixes: []string{}, NetworkAccess: NetworkDisabled, HooksDisabled: true},
			wantHooksDisabledInJSON: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.record)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if has := containsKey(data, "hooks_disabled"); has != tc.wantHooksDisabledInJSON {
				t.Fatalf("hooks_disabled in JSON: got %v, want %v (json=%s)", has, tc.wantHooksDisabledInJSON, string(data))
			}
			var decoded EnvironmentRecord
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if decoded.HooksDisabled != tc.record.HooksDisabled {
				t.Fatalf("HooksDisabled round-trip mismatch: got %v, want %v", decoded.HooksDisabled, tc.record.HooksDisabled)
			}
		})
	}
}

func TestTierConstants(t *testing.T) {
	tiers := []Tier{TierDeterministic, TierHeadless, TierLive, TierRelease}
	seen := make(map[Tier]bool)
	for _, tier := range tiers {
		if tier == "" {
			t.Error("empty tier constant")
		}
		if seen[tier] {
			t.Errorf("duplicate tier: %q", tier)
		}
		seen[tier] = true
	}
}

func TestStatusConstants(t *testing.T) {
	statuses := []Status{StatusPass, StatusFail, StatusError, StatusSkipped, StatusInconclusive}
	seen := make(map[Status]bool)
	for _, s := range statuses {
		if s == "" {
			t.Error("empty status constant")
		}
		if seen[s] {
			t.Errorf("duplicate status: %q", s)
		}
		seen[s] = true
	}
}

func TestVerdictConstants(t *testing.T) {
	verdicts := []Verdict{VerdictPass, VerdictFail, VerdictImprovement, VerdictRegression, VerdictAdvisory, VerdictInconclusive}
	seen := make(map[Verdict]bool)
	for _, v := range verdicts {
		if v == "" {
			t.Error("empty verdict constant")
		}
		if seen[v] {
			t.Errorf("duplicate verdict: %q", v)
		}
		seen[v] = true
	}
}

func TestCaseResultRoundTrip(t *testing.T) {
	original := CaseResult{
		ID:     "case-1",
		Status: StatusPass,
		Score:  0.95,
		DimensionScores: map[Dimension]float64{
			DimensionCorrectness: 1.0,
			DimensionSafety:      0.9,
		},
		DurationMS:     1500,
		Critical:       true,
		FailureMessage: "",
		Diagnostics:    []string{"note1"},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded CaseResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ID != original.ID {
		t.Errorf("ID: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status: got %q, want %q", decoded.Status, original.Status)
	}
	if decoded.Score != original.Score {
		t.Errorf("Score: got %f, want %f", decoded.Score, original.Score)
	}
	if decoded.DimensionScores[DimensionCorrectness] != 1.0 {
		t.Errorf("DimensionScores[correctness]: got %f", decoded.DimensionScores[DimensionCorrectness])
	}
	if decoded.DurationMS != 1500 {
		t.Errorf("DurationMS: got %d", decoded.DurationMS)
	}
	if !decoded.Critical {
		t.Error("Critical should be true")
	}
}

func TestBaselineComparisonRoundTrip(t *testing.T) {
	original := BaselineComparison{
		Verdict:        VerdictRegression,
		BaselineRunID:  "run-001",
		BaselineScore:  0.85,
		AggregateDelta: -0.05,
		DimensionDelta: map[Dimension]float64{
			DimensionCorrectness: -0.1,
		},
		Regressions: []ComparisonItem{
			{CaseID: "case-1", Dimension: DimensionCorrectness, Delta: -0.1, Reason: "score dropped"},
		},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded BaselineComparison
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Verdict != VerdictRegression {
		t.Errorf("Verdict: got %q", decoded.Verdict)
	}
	if decoded.AggregateDelta != -0.05 {
		t.Errorf("AggregateDelta: got %f", decoded.AggregateDelta)
	}
	if len(decoded.Regressions) != 1 {
		t.Fatalf("Regressions count: got %d", len(decoded.Regressions))
	}
	if decoded.Regressions[0].CaseID != "case-1" {
		t.Errorf("Regression CaseID: got %q", decoded.Regressions[0].CaseID)
	}
}

func containsKey(data []byte, key string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}
