// practices: [pragmatic-programmer, agile-manifesto]
package demo

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

func TestModule_Contract(t *testing.T) {
	contract := NewModule().Contract()
	if contract.ID != "ao.demo" {
		t.Fatalf("contract ID = %q, want ao.demo", contract.ID)
	}
	if contract.Output != clicontract.OutputText {
		t.Fatalf("output = %v, want OutputText", contract.Output)
	}
	if contract.Effects != clicontract.EffectPure {
		t.Fatalf("effects = %v, want EffectPure", contract.Effects)
	}
}

func TestModule_CommandAttributes(t *testing.T) {
	command := NewModule().Command()
	if command.Use != "demo" {
		t.Fatalf("Use = %q, want demo", command.Use)
	}
	if command.GroupID != "start" {
		t.Fatalf("GroupID = %q, want start", command.GroupID)
	}
}

// conditionalReviewRule is the AGENTS.md / docs/agent-workflow-reference.md
// review rule (#1189, ADR-0017 2026-10-03 amendment) as the demo states it.
const conditionalReviewRule = "Checks and CI gate an ordinary change; obtain one fresh author-distinct read only when the caller asks, a mistake cannot be cheaply undone, or no deterministic check covers the change."

// requestedVerdictRule scopes NOT_PROVEN to a requested binding verdict.
const requestedVerdictRule = "A requested binding verdict reports missing proof or unchecked acceptance as NOT_PROVEN."

// repairRule is the one-round repair rule: a check confirms a repair.
const repairRule = "Repair findings, confirm each with a check; a repair does not start another review."

// retiredReviewClaims are the pre-#1189 always-review claims no demo view
// may print.
var retiredReviewClaims = []string{
	"checks alone do not prove completion",
	"checks alone is insufficient",
	"convergence law",
	"repair_rounds",
	"re-validates",
	"required for accepted completion",
}

// normalizeSpace collapses line wrapping so a pinned sentence matches however
// the demo wraps it.
func normalizeSpace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func assertNoRetiredReviewClaims(t *testing.T, view, text string) {
	t.Helper()
	for _, retired := range retiredReviewClaims {
		if strings.Contains(text, retired) {
			t.Errorf("%s keeps retired review claim %q:\n%s", view, retired, text)
		}
	}
}

func TestDemoShowsBoundedRepairBoundary(t *testing.T) {
	var out bytes.Buffer
	if err := rpiDemo(&out); err != nil {
		t.Fatal(err)
	}
	text := normalizeSpace(out.String())
	for _, want := range []string{"AGENTOPS RPI DEMO", "existing intent source", "runtime derives", "subject-manifest.v1", conditionalReviewRule, "fresh validation result", "persistence is optional", repairRule} {
		if !strings.Contains(text, want) {
			t.Fatalf("demo missing %q:\n%s", want, out.String())
		}
	}
	assertNoRetiredReviewClaims(t, "demo --rpi", text)
	for _, forbidden := range []string{"PlanPacket", "CandidatePacket", "RevisionPacket"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("demo advertises removed %s contract:\n%s", forbidden, out.String())
		}
	}
}

func TestDemoConceptsExcludeLifecycleAuthority(t *testing.T) {
	var out bytes.Buffer
	if err := showConcepts(&out); err != nil {
		t.Fatal(err)
	}
	text := normalizeSpace(out.String())
	for _, want := range []string{conditionalReviewRule, "does not own retries", "Git", "delivery", "native coding agent", "optional"} {
		if !strings.Contains(text, want) {
			t.Fatalf("concepts missing %q:\n%s", want, out.String())
		}
	}
	assertNoRetiredReviewClaims(t, "demo --concepts", text)
}

func TestPublishedDemoQuick(t *testing.T) {
	command := NewModule().Command()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetArgs([]string{"--quick"})
	if err := command.Execute(); err != nil {
		t.Fatalf("ao demo --quick failed: %v\n%s", err, out.String())
	}
	text := normalizeSpace(out.String())
	for _, want := range []string{"AGENTOPS NATIVE DEMO", "ao gate check", conditionalReviewRule, "binding PASS", "unchanged acceptance", requestedVerdictRule, "checked/not_checked", "ao demo --rpi"} {
		if !strings.Contains(text, want) {
			t.Fatalf("demo output missing %q:\n%s", want, out.String())
		}
	}
	assertNoRetiredReviewClaims(t, "demo --quick", text)
}

func TestDemoHelpStatesConditionalReview(t *testing.T) {
	text := normalizeSpace(NewModule().Command().Long)
	if !strings.Contains(text, "one fresh read only where a mistake is costly") {
		t.Errorf("demo help missing the conditional fresh read, got:\n%s", text)
	}
	if strings.Contains(text, "fresh independent judgment and delivery") {
		t.Errorf("demo help keeps the always-review path, got:\n%s", text)
	}
}

func TestDemoConceptsFlagRendersBoundary(t *testing.T) {
	command := NewModule().Command()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetArgs([]string{"--concepts"})
	if err := command.Execute(); err != nil {
		t.Fatalf("ao demo --concepts failed: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "AGENTOPS PRODUCT BOUNDARY") {
		t.Fatalf("demo --concepts missing product boundary:\n%s", out.String())
	}
}
