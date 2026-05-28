// practices: [dora-metrics, lean-startup]
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/verdictledger"
)

// steerAutoFixtureMD is a GOALS.md fixture with three stable-ID directives plus
// prose and a Gates table — the surface the lossy renderer would mangle and
// that a non-lossy apply must preserve.
const steerAutoFixtureMD = `# Goals

Mission prose preserved verbatim.

## Directives

### 1. Ship fast

Deploy continuously.

**Directive ID:** d-ship-fast
**Steer:** increase

### 2. Reduce flaky tests

Stabilize the suite.

**Directive ID:** d-reduce-flaky
**Steer:** increase

### 3. Reduce debt

Pay down tech debt.

**Directive ID:** d-reduce-debt
**Steer:** decrease

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| gate-one | ` + "`exit 0`" + ` | 5 | Gate one |
`

// steerAutoEnv installs a temp project root: writes the GOALS.md fixture,
// chdirs into it, points goalsFile at it, and restores everything on cleanup.
func steerAutoEnv(t *testing.T) (dir, goalsPath string) {
	t.Helper()
	dir = t.TempDir()
	goalsPath = filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(steerAutoFixtureMD), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	oldFile, oldOut := goalsFile, output
	oldYes, oldAuto, oldPolicy := steerAutoYes, steerAutoAuto, steerAutoPolicyPath
	t.Cleanup(func() {
		_ = os.Chdir(wd)
		goalsFile, output = oldFile, oldOut
		steerAutoYes, steerAutoAuto, steerAutoPolicyPath = oldYes, oldAuto, oldPolicy
	})
	goalsFile = ""
	output = "table"
	steerAutoYes, steerAutoAuto, steerAutoPolicyPath = false, false, ""
	return dir, goalsPath
}

// seedFailureStreak appends a chronic failure streak for directiveID: enough
// iterations to clear minimum_evidence_count and a tail run of `fail` verdicts
// long enough to exceed failure_streak_length.
func seedFailureStreak(t *testing.T, root, directiveID string, total, failTail int) {
	t.Helper()
	writer := verdictledger.Writer{}
	base := time.Date(2026, 5, 17, 8, 0, 0, 0, time.UTC)
	for i := 0; i < total; i++ {
		verdict := verdictledger.VerdictPass
		if i >= total-failTail {
			verdict = verdictledger.VerdictFail
		}
		if _, err := writer.AppendIteration(root, verdictledger.IterationInput{
			DirectiveID:          directiveID,
			RunTime:              base.Add(time.Duration(i) * time.Hour),
			ScenarioVerdict:      verdict,
			ScenarioSatisfaction: 0.4,
			ScenarioCount:        3,
			EvaluatedCount:       3,
			RunID:                "run-" + directiveID,
		}); err != nil {
			t.Fatalf("seed iteration %d: %v", i, err)
		}
	}
}

// writeAutoApplyPolicy writes a re-steer policy with auto_apply set as given,
// at the default tracked path under root.
func writeAutoApplyPolicy(t *testing.T, root string, autoApply, allowFlip bool) {
	t.Helper()
	mutations := `["priority_bump", "setpoint_tighten", "setpoint_loosen"]`
	if allowFlip {
		mutations = `["priority_bump", "setpoint_tighten", "setpoint_loosen", "steer_flip"]`
	}
	policy := `{
  "minimum_evidence_count": 5,
  "failure_streak_length": 3,
  "cooldown_iterations": 5,
  "allowed_mutation_types": ` + mutations + `,
  "max_priority_bump": 3,
  "auto_apply": ` + boolStr(autoApply) + `,
  "allow_steer_flip": ` + boolStr(allowFlip) + `
}`
	path := filepath.Join(root, "docs", "re-steer-policy.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// TestSteerRecommend_SurfacesSeededStreak: a seeded eligible streak produces a
// printed priority-bump recommendation.
func TestSteerRecommend_SurfacesSeededStreak(t *testing.T) {
	dir, _ := steerAutoEnv(t)
	seedFailureStreak(t, dir, "d-reduce-flaky", 8, 4)

	var buf bytes.Buffer
	if err := runSteerRecommend(&buf, false); err != nil {
		t.Fatalf("runSteerRecommend: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "d-reduce-flaky") {
		t.Errorf("output missing recommendation directive id, got:\n%s", out)
	}
	if !strings.Contains(out, "priority_bump") {
		t.Errorf("output missing priority_bump mutation, got:\n%s", out)
	}
	if !strings.Contains(out, "recommendation-only") {
		t.Errorf("output missing recommendation-only notice, got:\n%s", out)
	}
}

// TestSteerRecommend_DoesNotMutateGoals: the default recommend run leaves
// GOALS.md byte-identical.
func TestSteerRecommend_DoesNotMutateGoals(t *testing.T) {
	dir, goalsPath := steerAutoEnv(t)
	seedFailureStreak(t, dir, "d-reduce-flaky", 8, 4)

	var buf bytes.Buffer
	if err := runSteerRecommend(&buf, false); err != nil {
		t.Fatalf("runSteerRecommend: %v", err)
	}
	after, err := os.ReadFile(goalsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != steerAutoFixtureMD {
		t.Error("recommend run modified GOALS.md; default mode must be recommendation-only")
	}
}

// TestSteerApply_BlockedWithoutAutoApplyPolicy: even with --yes, an apply is
// refused when policy auto_apply is false.
func TestSteerApply_BlockedWithoutAutoApplyPolicy(t *testing.T) {
	dir, goalsPath := steerAutoEnv(t)
	seedFailureStreak(t, dir, "d-reduce-flaky", 8, 4)
	writeAutoApplyPolicy(t, dir, false, false)
	steerAutoYes = true

	var buf bytes.Buffer
	err := runSteerApply(strings.NewReader(""), &buf, false)
	if err == nil {
		t.Fatal("expected apply to be blocked by auto_apply:false policy")
	}
	if !strings.Contains(err.Error(), "auto_apply") {
		t.Errorf("error = %q, want mention of auto_apply", err.Error())
	}
	after, _ := os.ReadFile(goalsPath)
	if string(after) != steerAutoFixtureMD {
		t.Error("GOALS.md mutated despite blocked apply")
	}
}

// TestSteerApply_DeclinedConfirmationKeepsGoals: an interactive run answered
// 'n' must not change GOALS.md.
func TestSteerApply_DeclinedConfirmationKeepsGoals(t *testing.T) {
	dir, goalsPath := steerAutoEnv(t)
	seedFailureStreak(t, dir, "d-reduce-flaky", 8, 4)
	writeAutoApplyPolicy(t, dir, true, false)

	var buf bytes.Buffer
	err := runSteerApply(strings.NewReader("n\n"), &buf, false)
	if err == nil {
		t.Fatal("expected declined confirmation to return an error")
	}
	if !strings.Contains(err.Error(), "declined") {
		t.Errorf("error = %q, want 'declined'", err.Error())
	}
	after, _ := os.ReadFile(goalsPath)
	if string(after) != steerAutoFixtureMD {
		t.Error("GOALS.md mutated despite declined confirmation")
	}
}

// TestSteerApply_NoRecommendationsLeavesGoals: a healthy ledger yields no
// recommendation and no mutation.
func TestSteerApply_NoRecommendationsLeavesGoals(t *testing.T) {
	dir, goalsPath := steerAutoEnv(t)
	// All-pass ledger: no failure streak.
	seedFailureStreak(t, dir, "d-reduce-flaky", 8, 0)
	writeAutoApplyPolicy(t, dir, true, false)
	steerAutoYes = true

	var buf bytes.Buffer
	if err := runSteerApply(strings.NewReader(""), &buf, false); err != nil {
		t.Fatalf("runSteerApply: %v", err)
	}
	after, _ := os.ReadFile(goalsPath)
	if string(after) != steerAutoFixtureMD {
		t.Error("GOALS.md mutated despite no recommendations")
	}
}

// TestSteerApply_ConfirmedAppliesPriorityBump: explicit consent + auto_apply
// policy applies the priority bump non-lossily and records a cooldown.
func TestSteerApply_ConfirmedAppliesPriorityBump(t *testing.T) {
	dir, goalsPath := steerAutoEnv(t)
	seedFailureStreak(t, dir, "d-reduce-flaky", 8, 5)
	writeAutoApplyPolicy(t, dir, true, false)
	steerAutoYes = true

	var buf bytes.Buffer
	if err := runSteerApply(strings.NewReader(""), &buf, false); err != nil {
		t.Fatalf("runSteerApply: %v", err)
	}
	after, err := os.ReadFile(goalsPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(after)
	if out == steerAutoFixtureMD {
		t.Fatal("GOALS.md was not modified despite confirmed apply")
	}
	// d-reduce-flaky was #2; a priority bump moves it up to #1.
	if !strings.Contains(out, "### 1. Reduce flaky tests") {
		t.Errorf("expected 'Reduce flaky tests' bumped to #1, got:\n%s", out)
	}
	// Non-target content preserved byte-for-byte.
	for _, want := range []string{
		"Mission prose preserved verbatim.",
		"| gate-one | `exit 0` | 5 | Gate one |",
		"**Directive ID:** d-ship-fast",
		"**Directive ID:** d-reduce-debt",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("non-target content %q not preserved", want)
		}
	}
	// A cooldown record was written.
	ledger, err := verdictledger.Load(dir)
	if err != nil {
		t.Fatalf("reload ledger: %v", err)
	}
	if !hasCooldownRecord(ledger, "d-reduce-flaky", verdictledger.CooldownApplied) {
		t.Error("expected an 'applied' cooldown record for d-reduce-flaky")
	}
}

// TestSteerApply_SteerFlipSuppressedWithoutDualOptIn: with a policy that
// proposes a steer_flip but no priority_bump, the apply is refused unless the
// dual opt-in holds — here it does, so it succeeds and flips Steer.
func TestSteerApply_SteerFlipWithDualOptIn(t *testing.T) {
	dir, goalsPath := steerAutoEnv(t)
	seedFailureStreak(t, dir, "d-ship-fast", 8, 5)
	// Policy: steer_flip-only allowed list + dual opt-in so the engine proposes
	// a steer_flip and Apply permits it.
	policy := `{
  "minimum_evidence_count": 5,
  "failure_streak_length": 3,
  "cooldown_iterations": 5,
  "allowed_mutation_types": ["steer_flip"],
  "max_priority_bump": 3,
  "auto_apply": true,
  "allow_steer_flip": true
}`
	path := filepath.Join(dir, "docs", "re-steer-policy.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	steerAutoYes = true

	var buf bytes.Buffer
	if err := runSteerApply(strings.NewReader(""), &buf, false); err != nil {
		t.Fatalf("runSteerApply: %v", err)
	}
	out, _ := os.ReadFile(goalsPath)
	if !strings.Contains(string(out), "**Steer:** decrease") {
		t.Errorf("expected d-ship-fast Steer flipped to decrease, got:\n%s", string(out))
	}
}

// TestSteerApply_SteerNeverFlippedWithoutDualOptIn pins ADR-0006 I-3 at the
// command layer: with allowed_mutation_types listing only "steer_flip" but
// allow_steer_flip:false, the dual opt-in is NOT satisfied, so the engine
// never proposes a steer_flip — it falls back to a priority_bump. The applied
// mutation must therefore leave every directive's **Steer:** line untouched.
func TestSteerApply_SteerNeverFlippedWithoutDualOptIn(t *testing.T) {
	dir, goalsPath := steerAutoEnv(t)
	seedFailureStreak(t, dir, "d-ship-fast", 8, 5)
	// allowed_mutation_types lists only steer_flip but allow_steer_flip is
	// false — the dual opt-in is NOT satisfied.
	policy := `{
  "minimum_evidence_count": 5,
  "failure_streak_length": 3,
  "cooldown_iterations": 5,
  "allowed_mutation_types": ["steer_flip"],
  "max_priority_bump": 3,
  "auto_apply": true,
  "allow_steer_flip": false
}`
	path := filepath.Join(dir, "docs", "re-steer-policy.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	steerAutoYes = true

	var buf bytes.Buffer
	if err := runSteerApply(strings.NewReader(""), &buf, false); err != nil {
		t.Fatalf("runSteerApply: %v", err)
	}
	after, _ := os.ReadFile(goalsPath)
	out := string(after)
	// No Steer line may have changed: increase stays increase, decrease stays
	// decrease (a flip would invert d-ship-fast's increase to decrease).
	if strings.Count(out, "**Steer:** increase") != strings.Count(steerAutoFixtureMD, "**Steer:** increase") {
		t.Error("a **Steer:** line was flipped despite missing dual opt-in (ADR-0006 I-3)")
	}
	if !strings.Contains(out, "### 1. Ship fast") {
		t.Errorf("expected fallback priority_bump leaving 'Ship fast' at #1, got:\n%s", out)
	}
}

// TestSteerRecommend_JSONOutput honors the global -o json convention.
func TestSteerRecommend_JSONOutput(t *testing.T) {
	dir, _ := steerAutoEnv(t)
	seedFailureStreak(t, dir, "d-reduce-flaky", 8, 4)

	var buf bytes.Buffer
	if err := runSteerRecommend(&buf, true); err != nil {
		t.Fatalf("runSteerRecommend json: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"recommendations"`) {
		t.Errorf("JSON output missing recommendations key, got:\n%s", out)
	}
	if !strings.Contains(out, `"d-reduce-flaky"`) {
		t.Errorf("JSON output missing directive id, got:\n%s", out)
	}
	if !strings.Contains(out, `"auto_apply"`) {
		t.Errorf("JSON output missing auto_apply key, got:\n%s", out)
	}
}

// TestGoalsSteer_HasAutoSubcommands pins the new recommend/apply subcommands.
func TestGoalsSteer_HasAutoSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, sub := range goalsSteerCmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"recommend", "apply"} {
		if !names[want] {
			t.Errorf("missing steer subcommand %q", want)
		}
	}
}

// hasCooldownRecord reports whether the ledger holds a cooldown record of the
// given kind for directiveID.
func hasCooldownRecord(l *verdictledger.Ledger, directiveID, kind string) bool {
	for _, r := range l.Records {
		if r.RecordType == verdictledger.RecordCooldown &&
			r.DirectiveID == directiveID && r.CooldownKind == kind {
			return true
		}
	}
	return false
}
