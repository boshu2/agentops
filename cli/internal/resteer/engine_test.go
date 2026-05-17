// practices: [tdd, bdd, design-by-contract]
package resteer

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/boshu2/agentops/cli/internal/verdictledger"
)

// repoRoot climbs from this test file to the agentops repo root so tests can
// read tracked fixtures without embedding duplicates.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file = .../cli/internal/resteer/engine_test.go
	// climb: resteer/ -> internal/ -> cli/ -> repo root
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// loadFixture loads a tracked verdict-ledger fixture by file name.
func loadFixture(t *testing.T, name string) *verdictledger.Ledger {
	t.Helper()
	path := filepath.Join(repoRoot(t), "tests", "fixtures", "verdict-ledger", name)
	ledger, err := verdictledger.LoadPath(path)
	if err != nil {
		t.Fatalf("LoadPath(%s): %v", name, err)
	}
	return ledger
}

// flipPolicy returns a policy with steer_flip dual opt-in satisfied: both
// allow_steer_flip:true AND "steer_flip" present in allowed_mutation_types.
// priority_bump is intentionally removed so a steer_flip is actually chosen.
func flipPolicy() Policy {
	p := DefaultPolicy()
	p.AllowSteerFlip = true
	p.AllowedMutationTypes = []string{MutationSteerFlip}
	return p
}

// TestDefaultPolicy_SafeDefaults pins ADR-0006 §Default Policy.
func TestDefaultPolicy_SafeDefaults(t *testing.T) {
	p := DefaultPolicy()
	if p.MinimumEvidenceCount != 5 {
		t.Errorf("MinimumEvidenceCount = %d, want 5", p.MinimumEvidenceCount)
	}
	if p.FailureStreakLength != 3 {
		t.Errorf("FailureStreakLength = %d, want 3", p.FailureStreakLength)
	}
	if p.CooldownIterations != 5 {
		t.Errorf("CooldownIterations = %d, want 5", p.CooldownIterations)
	}
	if p.MaxPriorityBump != 3 {
		t.Errorf("MaxPriorityBump = %d, want 3", p.MaxPriorityBump)
	}
	if p.AutoApply {
		t.Error("AutoApply = true, want false (ADR-0006 I-2)")
	}
	if p.AllowSteerFlip {
		t.Error("AllowSteerFlip = true, want false (ADR-0006 I-3)")
	}
	if p.SteerFlipPermitted() {
		t.Error("SteerFlipPermitted() = true under default policy, want false")
	}
}

// TestSteerFlipPermitted_DualOptIn pins ADR-0006 I-3: a steer flip needs BOTH
// allow_steer_flip:true AND "steer_flip" in allowed_mutation_types. Either flag
// alone must not permit it.
func TestSteerFlipPermitted_DualOptIn(t *testing.T) {
	cases := []struct {
		name        string
		allowFlip   bool
		mutations   []string
		wantAllowed bool
	}{
		{"neither", false, []string{MutationPriorityBump}, false},
		{"flag only", true, []string{MutationPriorityBump}, false},
		{"list only", false, []string{MutationPriorityBump, MutationSteerFlip}, false},
		{"both", true, []string{MutationPriorityBump, MutationSteerFlip}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := DefaultPolicy()
			p.AllowSteerFlip = tc.allowFlip
			p.AllowedMutationTypes = tc.mutations
			if got := p.SteerFlipPermitted(); got != tc.wantAllowed {
				t.Errorf("SteerFlipPermitted() = %v, want %v", got, tc.wantAllowed)
			}
		})
	}
}

// TestEvaluate_FixtureScenarios is the core table: each case seeds the engine
// with a tracked verdict-ledger fixture and asserts the exact recommendation /
// skip outcome.
func TestEvaluate_FixtureScenarios(t *testing.T) {
	cases := []struct {
		name        string
		fixture     string
		policy      Policy
		directives  []string
		wantRecs    int
		wantRecID   string
		wantRecType string
		wantSkipID  string
		wantSkip    SkipReason
	}{
		{
			name:        "seeded failure streak yields priority-bump recommendation",
			fixture:     "resteer-eligible.json",
			policy:      DefaultPolicy(),
			directives:  []string{"d-reduce-flaky-tests"},
			wantRecs:    1,
			wantRecID:   "d-reduce-flaky-tests",
			wantRecType: MutationPriorityBump,
		},
		{
			name:       "healthy directive yields no recommendation",
			fixture:    "resteer-eligible.json",
			policy:     DefaultPolicy(),
			directives: []string{"d-improve-coverage"},
			wantRecs:   0,
			wantSkipID: "d-improve-coverage",
			wantSkip:   SkipHealthy,
		},
		{
			name:       "cooldown active suppresses recommendation despite streak",
			fixture:    "resteer-cooldown.json",
			policy:     DefaultPolicy(),
			directives: []string{"d-cut-build-time"},
			wantRecs:   0,
			wantSkipID: "d-cut-build-time",
			wantSkip:   SkipCooldown,
		},
		{
			name:       "below minimum evidence yields no recommendation",
			fixture:    "failure-streak.json",
			policy:     DefaultPolicy(),
			directives: []string{"d-reduce-flaky-tests"},
			wantRecs:   0,
			wantSkipID: "d-reduce-flaky-tests",
			wantSkip:   SkipInsufficientEvidence,
		},
		{
			name:        "streak meets a lowered min-evidence yields recommendation",
			fixture:     "failure-streak.json",
			policy:      policyWithMinEvidence(3),
			directives:  []string{"d-reduce-flaky-tests"},
			wantRecs:    1,
			wantRecID:   "d-reduce-flaky-tests",
			wantRecType: MutationPriorityBump,
		},
		{
			name:        "steer-flip permitted under dual opt-in",
			fixture:     "resteer-eligible.json",
			policy:      flipPolicy(),
			directives:  []string{"d-reduce-flaky-tests"},
			wantRecs:    1,
			wantRecID:   "d-reduce-flaky-tests",
			wantRecType: MutationSteerFlip,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ledger := loadFixture(t, tc.fixture)
			res := Evaluate(ledger, tc.policy, tc.directives)

			if len(res.Recommendations) != tc.wantRecs {
				t.Fatalf("Recommendations = %d, want %d (%+v)", len(res.Recommendations), tc.wantRecs, res.Recommendations)
			}
			if tc.wantRecs == 1 {
				rec := res.Recommendations[0]
				if rec.DirectiveID != tc.wantRecID {
					t.Errorf("rec.DirectiveID = %q, want %q", rec.DirectiveID, tc.wantRecID)
				}
				if rec.MutationType != tc.wantRecType {
					t.Errorf("rec.MutationType = %q, want %q", rec.MutationType, tc.wantRecType)
				}
				if rec.FailureStreak < tc.policy.FailureStreakLength {
					t.Errorf("rec.FailureStreak = %d, want >= %d", rec.FailureStreak, tc.policy.FailureStreakLength)
				}
				if rec.Rationale == "" {
					t.Error("rec.Rationale is empty")
				}
			}
			if tc.wantRecs == 0 {
				if len(res.Skipped) != 1 {
					t.Fatalf("Skipped = %d, want 1 (%+v)", len(res.Skipped), res.Skipped)
				}
				skip := res.Skipped[0]
				if skip.DirectiveID != tc.wantSkipID {
					t.Errorf("skip.DirectiveID = %q, want %q", skip.DirectiveID, tc.wantSkipID)
				}
				if skip.Reason != tc.wantSkip {
					t.Errorf("skip.Reason = %q, want %q", skip.Reason, tc.wantSkip)
				}
			}
		})
	}
}

// policyWithMinEvidence returns the default policy with a lowered
// minimum_evidence_count, used to exercise the failure-streak fixture (which
// has only 3 iterations).
func policyWithMinEvidence(n int) Policy {
	p := DefaultPolicy()
	p.MinimumEvidenceCount = n
	return p
}

// TestEvaluate_SteerFlipSuppressedUnderDefaultPolicy proves that even an
// eligible failure streak never produces a steer_flip under the default
// policy (ADR-0006 I-3). The recommendation must instead be a priority bump.
func TestEvaluate_SteerFlipSuppressedUnderDefaultPolicy(t *testing.T) {
	ledger := loadFixture(t, "resteer-eligible.json")
	res := Evaluate(ledger, DefaultPolicy(), []string{"d-reduce-flaky-tests"})
	if len(res.Recommendations) != 1 {
		t.Fatalf("Recommendations = %d, want 1", len(res.Recommendations))
	}
	if got := res.Recommendations[0].MutationType; got != MutationPriorityBump {
		t.Fatalf("MutationType = %q, want %q (steer_flip must be suppressed)", got, MutationPriorityBump)
	}
}

// TestEvaluate_PriorityBumpBounded proves the proposed bump never exceeds
// max_priority_bump even when the streak is long.
func TestEvaluate_PriorityBumpBounded(t *testing.T) {
	ledger := loadFixture(t, "resteer-eligible.json")
	policy := DefaultPolicy()
	policy.MaxPriorityBump = 2
	res := Evaluate(ledger, policy, []string{"d-reduce-flaky-tests"})
	if len(res.Recommendations) != 1 {
		t.Fatalf("Recommendations = %d, want 1", len(res.Recommendations))
	}
	bump := res.Recommendations[0].PriorityBump
	if bump < 1 || bump > 2 {
		t.Errorf("PriorityBump = %d, want in [1,2]", bump)
	}
}

// TestEvaluate_MultipleDirectivesSorted feeds a mix of eligible and healthy
// directives and asserts both buckets are populated and sorted.
func TestEvaluate_MultipleDirectivesSorted(t *testing.T) {
	ledger := loadFixture(t, "resteer-eligible.json")
	res := Evaluate(ledger, DefaultPolicy(), []string{"d-improve-coverage", "d-reduce-flaky-tests"})
	if len(res.Recommendations) != 1 {
		t.Fatalf("Recommendations = %d, want 1", len(res.Recommendations))
	}
	if res.Recommendations[0].DirectiveID != "d-reduce-flaky-tests" {
		t.Errorf("rec directive = %q, want d-reduce-flaky-tests", res.Recommendations[0].DirectiveID)
	}
	if len(res.Skipped) != 1 {
		t.Fatalf("Skipped = %d, want 1", len(res.Skipped))
	}
	if res.Skipped[0].DirectiveID != "d-improve-coverage" {
		t.Errorf("skip directive = %q, want d-improve-coverage", res.Skipped[0].DirectiveID)
	}
}
