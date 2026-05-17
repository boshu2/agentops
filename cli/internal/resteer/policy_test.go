// practices: [tdd, design-by-contract]
package resteer

import (
	"os"
	"path/filepath"
	"testing"
)

// writePolicy writes a policy JSON file into a temp dir and returns its path.
func writePolicy(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "re-steer-policy.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write policy fixture: %v", err)
	}
	return path
}

// TestLoadPolicy_MissingFileReturnsDefault: a missing policy is not an error;
// the ADR-0006 safe defaults apply.
func TestLoadPolicy_MissingFileReturnsDefault(t *testing.T) {
	p, err := LoadPolicy(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("LoadPolicy(missing) error = %v, want nil", err)
	}
	if !p.Equal(DefaultPolicy()) {
		t.Errorf("LoadPolicy(missing) = %+v, want DefaultPolicy %+v", p, DefaultPolicy())
	}
}

// TestLoadPolicy_TrackedDefaultFileParses verifies the tracked
// docs/re-steer-policy.default.json loads and matches the built-in default.
func TestLoadPolicy_TrackedDefaultFileParses(t *testing.T) {
	path := filepath.Join(repoRoot(t), "docs", "re-steer-policy.default.json")
	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy(tracked default) error = %v", err)
	}
	if !p.Equal(DefaultPolicy()) {
		t.Errorf("tracked default = %+v, want built-in DefaultPolicy %+v", p, DefaultPolicy())
	}
}

// TestLoadPolicy_ValidCustom parses a hand-written valid policy and pins each
// field.
func TestLoadPolicy_ValidCustom(t *testing.T) {
	path := writePolicy(t, `{
		"minimum_evidence_count": 3,
		"failure_streak_length": 2,
		"cooldown_iterations": 4,
		"allowed_mutation_types": ["priority_bump", "steer_flip"],
		"max_priority_bump": 2,
		"auto_apply": true,
		"allow_steer_flip": true
	}`)
	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy error = %v", err)
	}
	if p.MinimumEvidenceCount != 3 {
		t.Errorf("MinimumEvidenceCount = %d, want 3", p.MinimumEvidenceCount)
	}
	if p.FailureStreakLength != 2 {
		t.Errorf("FailureStreakLength = %d, want 2", p.FailureStreakLength)
	}
	if !p.AutoApply {
		t.Error("AutoApply = false, want true")
	}
	if !p.SteerFlipPermitted() {
		t.Error("SteerFlipPermitted() = false, want true (dual opt-in satisfied)")
	}
}

// TestLoadPolicy_RejectsInvalid pins ADR-0006 I-1: a failure_streak_length
// below 2 is schema-invalid and must be rejected, plus other defects.
func TestLoadPolicy_RejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			"failure_streak_length below 2 (ADR-0006 I-1)",
			`{"minimum_evidence_count":5,"failure_streak_length":1,"cooldown_iterations":5,
			  "allowed_mutation_types":["priority_bump"],"max_priority_bump":3,
			  "auto_apply":false,"allow_steer_flip":false}`,
		},
		{
			"minimum_evidence_count below 1",
			`{"minimum_evidence_count":0,"failure_streak_length":3,"cooldown_iterations":5,
			  "allowed_mutation_types":["priority_bump"],"max_priority_bump":3,
			  "auto_apply":false,"allow_steer_flip":false}`,
		},
		{
			"cooldown_iterations below 1",
			`{"minimum_evidence_count":5,"failure_streak_length":3,"cooldown_iterations":0,
			  "allowed_mutation_types":["priority_bump"],"max_priority_bump":3,
			  "auto_apply":false,"allow_steer_flip":false}`,
		},
		{
			"max_priority_bump below 1",
			`{"minimum_evidence_count":5,"failure_streak_length":3,"cooldown_iterations":5,
			  "allowed_mutation_types":["priority_bump"],"max_priority_bump":0,
			  "auto_apply":false,"allow_steer_flip":false}`,
		},
		{
			"unknown mutation type",
			`{"minimum_evidence_count":5,"failure_streak_length":3,"cooldown_iterations":5,
			  "allowed_mutation_types":["bogus_mutation"],"max_priority_bump":3,
			  "auto_apply":false,"allow_steer_flip":false}`,
		},
		{
			"malformed json",
			`{not json`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writePolicy(t, tc.body)
			if _, err := LoadPolicy(path); err == nil {
				t.Errorf("LoadPolicy(%s) error = nil, want non-nil", tc.name)
			}
		})
	}
}
