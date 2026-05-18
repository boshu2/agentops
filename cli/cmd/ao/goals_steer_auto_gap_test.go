// practices: [dora-metrics, lean-startup]
// F5.T1 gap-fill — cmd/ao layer tests for auto re-steer.
//
// Fills genuine gaps not addressed by goals_steer_auto_test.go:
//   - steerAutoAuto flag (--auto) acts as pre-confirmation alongside --yes
//   - runSteerRecommend on a GOALS.md with no directives returns no recommendations
//   - cooldown record carries the correct mutation_type from the recommendation
//   - --auto alone is blocked when policy auto_apply=false
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/verdictledger"
)

// noDirectivesMD is a minimal GOALS.md with no ## Directives section.
const noDirectivesMD = `# Goals

No directives defined yet.
`

// TestSteerApply_AutoFlagActsAsPreconfirmation verifies that --auto (steerAutoAuto)
// satisfies the pre-confirmation gate the same way --yes does: with auto_apply:true
// and --auto set, the apply runs without reading stdin and applies the mutation.
func TestSteerApply_AutoFlagActsAsPreconfirmation(t *testing.T) {
	dir, goalsPath := steerAutoEnv(t)
	seedFailureStreak(t, dir, "d-reduce-flaky", 8, 5)
	writeAutoApplyPolicy(t, dir, true, false)
	// Use --auto instead of --yes.
	steerAutoAuto = true
	steerAutoYes = false

	var buf bytes.Buffer
	if err := runSteerApply(strings.NewReader(""), &buf, false); err != nil {
		t.Fatalf("runSteerApply(--auto): %v", err)
	}
	after, err := os.ReadFile(goalsPath)
	if err != nil {
		t.Fatal(err)
	}
	// Mutation applied: d-reduce-flaky bumped to #1.
	if !strings.Contains(string(after), "### 1. Reduce flaky tests") {
		t.Errorf("--auto flag did not apply priority bump; got:\n%s", string(after))
	}
}

// TestSteerRecommend_NoDirectivesReturnsNoRecommendation verifies that a GOALS.md
// with no directive blocks produces zero recommendations without error.
func TestSteerRecommend_NoDirectivesReturnsNoRecommendation(t *testing.T) {
	dir, _ := steerAutoEnv(t)
	// Overwrite GOALS.md with a header-only file (no ## Directives section).
	if err := os.WriteFile(filepath.Join(dir, "GOALS.md"), []byte(noDirectivesMD), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := runSteerRecommend(&buf, false); err != nil {
		t.Fatalf("runSteerRecommend(no-directives GOALS.md): %v", err)
	}
	out := buf.String()
	// Must not surface any recommendation content.
	if strings.Contains(out, "priority_bump") || strings.Contains(out, "steer_flip") {
		t.Errorf("expected no recommendation output for empty directive list, got:\n%s", out)
	}
}

// TestSteerApply_CooldownRecordCarriesMutationType verifies that the cooldown
// record written after a successful apply carries mutation_type = priority_bump —
// not a hard-coded fallback.
func TestSteerApply_CooldownRecordCarriesMutationType(t *testing.T) {
	dir, _ := steerAutoEnv(t)
	seedFailureStreak(t, dir, "d-reduce-flaky", 8, 5)
	writeAutoApplyPolicy(t, dir, true, false)
	steerAutoYes = true

	var buf bytes.Buffer
	if err := runSteerApply(strings.NewReader(""), &buf, false); err != nil {
		t.Fatalf("runSteerApply: %v", err)
	}

	ledger, err := verdictledger.Load(dir)
	if err != nil {
		t.Fatalf("reload ledger: %v", err)
	}
	found := false
	for _, r := range ledger.Records {
		if r.RecordType == verdictledger.RecordCooldown &&
			r.DirectiveID == "d-reduce-flaky" &&
			r.CooldownKind == verdictledger.CooldownApplied {
			found = true
			if r.MutationType != verdictledger.MutationPriorityBump {
				t.Errorf("cooldown record mutation_type = %q, want %q",
					r.MutationType, verdictledger.MutationPriorityBump)
			}
		}
	}
	if !found {
		t.Error("no applied cooldown record found for d-reduce-flaky")
	}
}

// TestSteerApply_AutoFlagBlockedByAutoApplyFalsePolicy verifies that --auto alone
// is not sufficient: policy.auto_apply must also be true. With auto_apply:false,
// the apply must be blocked and GOALS.md must be unchanged.
func TestSteerApply_AutoFlagBlockedByAutoApplyFalsePolicy(t *testing.T) {
	dir, goalsPath := steerAutoEnv(t)
	seedFailureStreak(t, dir, "d-reduce-flaky", 8, 5)
	writeAutoApplyPolicy(t, dir, false, false) // auto_apply: false
	steerAutoAuto = true
	steerAutoYes = false

	var buf bytes.Buffer
	err := runSteerApply(strings.NewReader(""), &buf, false)
	if err == nil {
		t.Fatal("expected apply blocked when policy auto_apply=false, even with --auto")
	}
	if !strings.Contains(err.Error(), "auto_apply") {
		t.Errorf("error = %q, want mention of auto_apply", err.Error())
	}
	after, _ := os.ReadFile(goalsPath)
	if string(after) != steerAutoFixtureMD {
		t.Error("GOALS.md mutated despite blocked apply")
	}
}
