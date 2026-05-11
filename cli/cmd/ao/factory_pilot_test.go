// practices: [microservices, team-topologies]
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
)

func TestFactoryPilotJSONPlan(t *testing.T) {
	repo := t.TempDir()
	origProjectDir := testProjectDir
	testProjectDir = repo
	defer func() { testProjectDir = origProjectDir }()

	out, err := executeCommand("factory", "pilot", "--json", "--goal", "Compare worker counts", "--run-id", "pilot-test")
	if err != nil {
		t.Fatalf("factory pilot --json: %v\noutput: %s", err, out)
	}

	var result factoryPilotResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse factory pilot json: %v\noutput: %s", err, out)
	}
	if result.RunID != "pilot-test" {
		t.Fatalf("run_id = %q, want pilot-test", result.RunID)
	}
	if result.MaxConcurrency != 2 {
		t.Fatalf("max_concurrency = %d, want 2", result.MaxConcurrency)
	}
	if result.AutoMergeEnabled {
		t.Fatal("factory pilot must not enable auto merge")
	}
	if !result.ManualMergeRequired {
		t.Fatal("factory pilot must require manual merge")
	}
	if len(result.ValidationCommands) == 0 {
		t.Fatal("expected validation commands")
	}
	if len(result.Phases) != 2 {
		t.Fatalf("phases = %d, want 2", len(result.Phases))
	}
	if result.Phases[0].WorkerCount != 1 || result.Phases[1].WorkerCount != 2 {
		t.Fatalf("phase worker counts = %#v, want 1-vs-2", result.Phases)
	}
	if len(result.Slots) != 3 || len(result.Worktrees) != 3 {
		t.Fatalf("slots/worktrees = %d/%d, want 3/3", len(result.Slots), len(result.Worktrees))
	}
	for _, slot := range result.Slots {
		if slot.Provider != "openai" || slot.Runtime != "codex" || slot.Authority != daemonpkg.RoutingAuthorityDelegated {
			t.Fatalf("slot lane = %#v, want openai/codex/DELEGATED", slot)
		}
		if slot.RetentionPolicy != "retain_on_failure" || slot.MergeDisposition != "manual_pending" {
			t.Fatalf("slot retention/merge = %#v, want retained manual_pending", slot)
		}
	}
	if len(result.ExcludedLanes) != 1 || result.ExcludedLanes[0].Provider != "gascity" {
		t.Fatalf("excluded lanes = %#v, want disabled gascity lane", result.ExcludedLanes)
	}
	if !pilotHasEvent(result.EventTemplates, daemonpkg.EventFactoryYieldObservation) {
		t.Fatalf("event templates missing yield observation: %#v", result.EventTemplates)
	}
	if len(result.YieldObservationTemplates) != 2 {
		t.Fatalf("yield templates = %d, want baseline and treatment", len(result.YieldObservationTemplates))
	}
}

func TestBuildFactoryPilotPlanRejectsBlankValidationOverride(t *testing.T) {
	_, err := buildFactoryPilotPlan(t.TempDir(), factoryPilotOptions{
		Goal:               "Compare worker counts",
		RunID:              "pilot-test",
		ValidationCommands: []string{"  "},
	})
	if err == nil {
		t.Fatal("expected blank validation override to be rejected")
	}
	if !strings.Contains(err.Error(), "validation commands") {
		t.Fatalf("error = %v, want validation command failure", err)
	}
}

func TestBuildFactoryPilotPlanRejectsNonFrontierLane(t *testing.T) {
	policy := daemonpkg.DefaultFactoryRoutingPolicy()
	policy.Lanes[0].Provider = "anthropic"
	policy.Lanes[0].Runtime = "claude"

	_, err := buildFactoryPilotPlan(t.TempDir(), factoryPilotOptions{
		Goal:   "Compare worker counts",
		RunID:  "pilot-test",
		Now:    func() time.Time { return time.Date(2026, 5, 3, 21, 0, 0, 0, time.UTC) },
		Policy: policy,
	})
	if err == nil {
		t.Fatal("expected non-frontier lane to be rejected")
	}
	if !strings.Contains(err.Error(), "cloud/frontier codex lane") {
		t.Fatalf("error = %v, want cloud/frontier rejection", err)
	}
}

func pilotHasEvent(events []factoryPilotEventTemplate, eventType daemonpkg.EventType) bool {
	for _, event := range events {
		if event.EventType == eventType {
			return true
		}
	}
	return false
}
