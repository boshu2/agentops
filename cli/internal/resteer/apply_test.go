// practices: [tdd, bdd, design-by-contract]
package resteer

import (
	"strings"
	"testing"
)

// applyFixtureMD is a GOALS.md fixture exercising the patcher-preserved
// surface: prose paragraphs, multiple directives with stable IDs, and a Gates
// table that the lossy renderer would mangle. Apply must preserve every byte
// outside the mutated directive heading numbers / Steer line.
const applyFixtureMD = `# Goals

Mission prose that the lossy renderer drops.

## Directives

### 1. Ship fast

Deploy continuously.

**Directive ID:** d-ship-fast
**Steer:** increase

### 2. Stay secure

No vulnerabilities.

**Directive ID:** d-stay-secure
**Steer:** hold

### 3. Reduce debt

Pay down tech debt.

**Directive ID:** d-reduce-debt
**Steer:** decrease

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| gate-one | ` + "`exit 0`" + ` | 5 | Gate one |
`

// directiveLineFor returns the "### N. Title" heading line containing title.
func directiveLineFor(t *testing.T, content, title string) string {
	t.Helper()
	for _, ln := range strings.Split(content, "\n") {
		if strings.Contains(ln, title) && strings.HasPrefix(strings.TrimSpace(ln), "### ") {
			return strings.TrimSpace(ln)
		}
	}
	t.Fatalf("no heading line for %q", title)
	return ""
}

// TestApply_PriorityBumpReordersNonLossily pins the core F5.3 contract: a
// priority_bump moves the directive block up and renumbers headings, while
// every non-directive byte (mission prose, Gates table) is preserved.
func TestApply_PriorityBumpReordersNonLossily(t *testing.T) {
	rec := Recommendation{
		DirectiveID:  "d-reduce-debt",
		MutationType: MutationPriorityBump,
		PriorityBump: 2,
	}
	patched, outcome, err := Apply([]byte(applyFixtureMD), DefaultPolicy(), rec)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out := string(patched)

	if outcome.FromPosition != 3 || outcome.ToPosition != 1 {
		t.Errorf("outcome positions = %d->%d, want 3->1", outcome.FromPosition, outcome.ToPosition)
	}
	// Reduce debt is now directive #1.
	if got := directiveLineFor(t, out, "Reduce debt"); got != "### 1. Reduce debt" {
		t.Errorf("Reduce debt heading = %q, want '### 1. Reduce debt'", got)
	}
	if got := directiveLineFor(t, out, "Ship fast"); got != "### 2. Ship fast" {
		t.Errorf("Ship fast heading = %q, want '### 2. Ship fast'", got)
	}
	if got := directiveLineFor(t, out, "Stay secure"); got != "### 3. Stay secure" {
		t.Errorf("Stay secure heading = %q, want '### 3. Stay secure'", got)
	}
	// Non-directive content preserved byte-for-byte.
	for _, want := range []string{
		"Mission prose that the lossy renderer drops.",
		"| gate-one | `exit 0` | 5 | Gate one |",
		"**Directive ID:** d-reduce-debt",
		"**Directive ID:** d-ship-fast",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("patched GOALS.md missing preserved content %q", want)
		}
	}
	// Exactly the three heading numbers changed: no other line differs in count.
	if strings.Count(out, "### ") != 3 {
		t.Errorf("heading count = %d, want 3", strings.Count(out, "### "))
	}
}

// TestApply_PriorityBumpClampedToMax pins that a recommendation requesting more
// positions than max_priority_bump is clamped.
func TestApply_PriorityBumpClampedToMax(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaxPriorityBump = 1
	rec := Recommendation{
		DirectiveID:  "d-reduce-debt",
		MutationType: MutationPriorityBump,
		PriorityBump: 5,
	}
	_, outcome, err := Apply([]byte(applyFixtureMD), policy, rec)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if outcome.ToPosition != 2 {
		t.Errorf("ToPosition = %d, want 2 (clamped bump of 1 from #3)", outcome.ToPosition)
	}
}

// TestApply_SteerFlipSuppressedWithoutDualOptIn pins ADR-0006 I-3: a steer_flip
// is refused under the default policy.
func TestApply_SteerFlipSuppressedWithoutDualOptIn(t *testing.T) {
	rec := Recommendation{
		DirectiveID:  "d-ship-fast",
		MutationType: MutationSteerFlip,
	}
	_, _, err := Apply([]byte(applyFixtureMD), DefaultPolicy(), rec)
	if err == nil {
		t.Fatal("expected steer_flip to be refused under default policy")
	}
	if !strings.Contains(err.Error(), "allow_steer_flip") {
		t.Errorf("error = %q, want mention of allow_steer_flip", err.Error())
	}
}

// TestApply_SteerFlipWithDualOptIn pins that a steer_flip IS applied when the
// policy satisfies the dual opt-in, and inverts increase<->decrease.
func TestApply_SteerFlipWithDualOptIn(t *testing.T) {
	policy := DefaultPolicy()
	policy.AllowSteerFlip = true
	policy.AllowedMutationTypes = append(policy.AllowedMutationTypes, MutationSteerFlip)

	rec := Recommendation{
		DirectiveID:  "d-ship-fast",
		MutationType: MutationSteerFlip,
	}
	patched, outcome, err := Apply([]byte(applyFixtureMD), policy, rec)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out := string(patched)
	if !strings.Contains(out, "**Steer:** decrease") {
		t.Error("expected Ship fast Steer flipped to decrease")
	}
	if !strings.Contains(outcome.Detail, "increase") || !strings.Contains(outcome.Detail, "decrease") {
		t.Errorf("outcome detail = %q, want mention of increase->decrease", outcome.Detail)
	}
	// Other directives' Steer untouched.
	if !strings.Contains(out, "**Steer:** hold") {
		t.Error("Stay secure Steer should be unchanged")
	}
}

// TestApply_UnknownDirective surfaces a corrective command in the error.
func TestApply_UnknownDirective(t *testing.T) {
	rec := Recommendation{DirectiveID: "d-nonexistent", MutationType: MutationPriorityBump}
	_, _, err := Apply([]byte(applyFixtureMD), DefaultPolicy(), rec)
	if err == nil {
		t.Fatal("expected error for unknown directive")
	}
	if !strings.Contains(err.Error(), "ao goals steer recommend") {
		t.Errorf("error = %q, want corrective command", err.Error())
	}
}

// TestApply_PriorityBumpAlreadyAtTop is a no-op when the directive is #1.
func TestApply_PriorityBumpAlreadyAtTop(t *testing.T) {
	rec := Recommendation{
		DirectiveID:  "d-ship-fast",
		MutationType: MutationPriorityBump,
		PriorityBump: 2,
	}
	patched, outcome, err := Apply([]byte(applyFixtureMD), DefaultPolicy(), rec)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if outcome.ToPosition != 1 || outcome.FromPosition != 1 {
		t.Errorf("positions = %d->%d, want 1->1 (no-op)", outcome.FromPosition, outcome.ToPosition)
	}
	if string(patched) != applyFixtureMD {
		t.Error("GOALS.md should be byte-identical for a no-op bump")
	}
}
