// Package resteer — F5.T1 gap-fill regression tests.
//
// Fills genuine coverage gaps not addressed by engine_test.go, policy_test.go,
// and apply_test.go:
//   - Policy.Equal method correctness
//   - SteerFlipPermitted with empty AllowedMutationTypes
//   - Evaluate with empty directive list
//   - Apply with a setpoint mutation type (not priority_bump or steer_flip)
//   - clampBump at exact boundary (bump == maxPriorityBump)
//   - Apply preserves non-target Steer lines on a steer_flip
package resteer

import (
	"strings"
	"testing"
)

// TestPolicyEqual_Symmetry pins that Equal is reflexive and catches single-field
// diffs, with exact expected values for each comparison.
func TestPolicyEqual_Symmetry(t *testing.T) {
	base := DefaultPolicy()

	// Reflexive: a policy equals itself.
	if !base.Equal(base) {
		t.Error("DefaultPolicy().Equal(DefaultPolicy()) = false, want true")
	}

	// Each field mutation breaks equality.
	cases := []struct {
		name  string
		other Policy
	}{
		{"MinimumEvidenceCount differs", func() Policy { p := DefaultPolicy(); p.MinimumEvidenceCount = 99; return p }()},
		{"FailureStreakLength differs", func() Policy { p := DefaultPolicy(); p.FailureStreakLength = 99; return p }()},
		{"CooldownIterations differs", func() Policy { p := DefaultPolicy(); p.CooldownIterations = 99; return p }()},
		{"MaxPriorityBump differs", func() Policy { p := DefaultPolicy(); p.MaxPriorityBump = 99; return p }()},
		{"AutoApply differs", func() Policy { p := DefaultPolicy(); p.AutoApply = true; return p }()},
		{"AllowSteerFlip differs", func() Policy { p := DefaultPolicy(); p.AllowSteerFlip = true; return p }()},
		{"AllowedMutationTypes length differs", func() Policy { p := DefaultPolicy(); p.AllowedMutationTypes = nil; return p }()},
		{"AllowedMutationTypes element differs", func() Policy {
			p := DefaultPolicy()
			p.AllowedMutationTypes = append([]string{}, p.AllowedMutationTypes...)
			p.AllowedMutationTypes[0] = MutationSteerFlip
			return p
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if base.Equal(tc.other) {
				t.Errorf("Equal = true after %s mutation, want false", tc.name)
			}
		})
	}
}

// TestSteerFlipPermitted_EmptyMutationList verifies that an empty
// AllowedMutationTypes list never permits a steer_flip, even when
// AllowSteerFlip is true.
func TestSteerFlipPermitted_EmptyMutationList(t *testing.T) {
	p := DefaultPolicy()
	p.AllowSteerFlip = true
	p.AllowedMutationTypes = []string{}
	if p.SteerFlipPermitted() {
		t.Error("SteerFlipPermitted() = true with empty AllowedMutationTypes, want false")
	}
}

// TestEvaluate_EmptyDirectiveList verifies that an empty directive list
// produces zero recommendations and zero skipped entries without error.
func TestEvaluate_EmptyDirectiveList(t *testing.T) {
	ledger := loadFixture(t, "resteer-eligible.json")
	res := Evaluate(ledger, DefaultPolicy(), []string{})
	if len(res.Recommendations) != 0 {
		t.Errorf("Recommendations = %d, want 0 for empty directive list", len(res.Recommendations))
	}
	if len(res.Skipped) != 0 {
		t.Errorf("Skipped = %d, want 0 for empty directive list", len(res.Skipped))
	}
}

// TestApply_SetpointMutationReturnsError pins ADR-0006: setpoint mutations
// (setpoint_tighten / setpoint_loosen) are valid policy values but are not yet
// implemented in the Apply mutator. Apply must return a non-nil error for these
// types and must not mutate the content.
func TestApply_SetpointMutationReturnsError(t *testing.T) {
	const fixture = `# Goals

## Directives

### 1. Ship fast

Deploy.

**Directive ID:** d-ship-fast
**Steer:** increase
`
	for _, mutType := range []string{MutationSetpointTighten, MutationSetpointLoosen} {
		t.Run(mutType, func(t *testing.T) {
			policy := DefaultPolicy()
			rec := Recommendation{
				DirectiveID:  "d-ship-fast",
				MutationType: mutType,
			}
			_, _, err := Apply([]byte(fixture), policy, rec)
			if err == nil {
				t.Errorf("Apply(%s) error = nil, want non-nil (not yet implemented)", mutType)
			}
		})
	}
}

// TestApply_PriorityBumpExactlyAtMax verifies that a bump equal to MaxPriorityBump
// is not clamped further — the boundary is inclusive.
func TestApply_PriorityBumpExactlyAtMax(t *testing.T) {
	const fixture = `# Goals

## Directives

### 1. Alpha

**Directive ID:** d-alpha
**Steer:** increase

### 2. Beta

**Directive ID:** d-beta
**Steer:** hold

### 3. Gamma

**Directive ID:** d-gamma
**Steer:** decrease
`
	policy := DefaultPolicy()
	policy.MaxPriorityBump = 2
	rec := Recommendation{
		DirectiveID:  "d-gamma",
		MutationType: MutationPriorityBump,
		PriorityBump: 2, // exactly MaxPriorityBump — must not be clamped further
	}
	_, outcome, err := Apply([]byte(fixture), policy, rec)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// d-gamma starts at #3; bumping by exactly 2 → position #1.
	if outcome.FromPosition != 3 {
		t.Errorf("outcome.FromPosition = %d, want 3", outcome.FromPosition)
	}
	if outcome.ToPosition != 1 {
		t.Errorf("outcome.ToPosition = %d, want 1 (bump=2 from #3, max=2 not clamped)", outcome.ToPosition)
	}
}

// TestApply_SteerFlipPreservesNonTargetSteerLines pins that a steer_flip on
// one directive never mutates another directive's **Steer:** line.
func TestApply_SteerFlipPreservesNonTargetSteerLines(t *testing.T) {
	const fixture = `# Goals

## Directives

### 1. Ship fast

**Directive ID:** d-ship-fast
**Steer:** increase

### 2. Stay secure

**Directive ID:** d-stay-secure
**Steer:** hold

### 3. Reduce debt

**Directive ID:** d-reduce-debt
**Steer:** decrease
`
	policy := DefaultPolicy()
	policy.AllowSteerFlip = true
	policy.AllowedMutationTypes = []string{MutationSteerFlip}

	rec := Recommendation{
		DirectiveID:  "d-ship-fast",
		MutationType: MutationSteerFlip,
	}
	patched, _, err := Apply([]byte(fixture), policy, rec)
	if err != nil {
		t.Fatalf("Apply(steer_flip): %v", err)
	}
	out := string(patched)

	// Target directive flipped: increase → decrease.
	if !strings.Contains(out, "**Directive ID:** d-ship-fast\n**Steer:** decrease") {
		t.Error("d-ship-fast Steer was not flipped to decrease")
	}
	// Non-target directives must keep their Steer values.
	if !strings.Contains(out, "**Directive ID:** d-stay-secure\n**Steer:** hold") {
		t.Error("d-stay-secure Steer changed, want hold preserved")
	}
	if !strings.Contains(out, "**Directive ID:** d-reduce-debt\n**Steer:** decrease") {
		t.Error("d-reduce-debt Steer changed, want decrease preserved")
	}
}
