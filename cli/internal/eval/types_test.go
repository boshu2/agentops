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

func containsKey(data []byte, key string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}
