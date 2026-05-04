package daemon

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestDefaultFactoryRoutingPolicyValidates(t *testing.T) {
	policy := DefaultFactoryRoutingPolicy()
	if err := ValidateRoutingPolicy(policy); err != nil {
		t.Fatalf("default policy should validate: %v", err)
	}
	if policy.AutoMergeEnabled {
		t.Fatal("default policy must disable auto merge")
	}
	if !policy.ManualMergeByDefault {
		t.Fatal("default policy must require manual merge")
	}
	lane, ok := policy.LaneByID("frontier-codex")
	if !ok {
		t.Fatal("expected frontier-codex lane")
	}
	if lane.MergeEligibility == nil {
		t.Fatal("frontier-codex must declare merge eligibility")
	}
	wantCommands := []string{
		"cd cli && go test ./internal/daemon -run 'RoutingPolicy|FactoryProjection'",
		"scripts/pre-push-gate.sh --fast",
	}
	if got := strings.Join(lane.MergeEligibility.ValidationCommands, "\n"); got != strings.Join(wantCommands, "\n") {
		t.Fatalf("validation commands = %#v, want %#v", lane.MergeEligibility.ValidationCommands, wantCommands)
	}
	if !lane.MergeEligibility.ManualMergeRequired {
		t.Fatal("merge eligibility must require manual merge")
	}
	if lane.MergeEligibility.ValidationFailureTerminalEvent != EventFactoryJobTerminal {
		t.Fatalf("validation failure terminal event = %q, want %q", lane.MergeEligibility.ValidationFailureTerminalEvent, EventFactoryJobTerminal)
	}
	if !lane.MergeEligibility.RetainArtifactsOnFailure {
		t.Fatal("validation failure must retain artifacts")
	}
	if !lane.MergeEligibility.RetainWorktreeOnValidationFailure {
		t.Fatal("validation failure must retain the worktree")
	}
	if lane, ok := policy.LaneByID("gascity-reference"); !ok {
		t.Fatal("expected disabled gascity-reference lane")
	} else if lane.Enabled {
		t.Fatal("gascity-reference must be disabled in milestone 1")
	}
}

func TestParseRoutingPolicyJSON(t *testing.T) {
	data, err := json.Marshal(DefaultFactoryRoutingPolicy())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	policy, err := ParseRoutingPolicyJSON(data)
	if err != nil {
		t.Fatalf("ParseRoutingPolicyJSON: %v", err)
	}
	if policy.PolicyID != "repo-default" {
		t.Fatalf("policy_id = %q, want repo-default", policy.PolicyID)
	}
}

func TestParseRoutingPolicyJSONRejectsUnknownFields(t *testing.T) {
	data := []byte(`{
		"schema_version": 1,
		"policy_id": "repo-default",
		"default_lane": "frontier-codex",
		"max_total_concurrency": 2,
		"auto_merge_enabled": false,
		"manual_merge_by_default": true,
		"unexpected": true,
		"lanes": []
	}`)
	if _, err := ParseRoutingPolicyJSON(data); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("ParseRoutingPolicyJSON unknown field error = %v, want unknown field rejection", err)
	}

	data = []byte(`{
		"schema_version": 1,
		"policy_id": "repo-default",
		"default_lane": "frontier-codex",
		"max_total_concurrency": 2,
		"auto_merge_enabled": false,
		"manual_merge_by_default": true,
		"lanes": [{
			"id": "frontier-codex",
			"enabled": true,
			"authority": "DELEGATED",
			"provider": "openai",
			"runtime": "codex",
			"model": "frontier-default",
			"task_classes": ["code_change"],
			"max_concurrency": 1,
			"unknown_lane_field": true,
			"merge_eligibility": {
				"manual_merge_required": true,
				"validation_commands": ["go test ./..."],
				"validation_failure_terminal_event": "factory.job_terminal",
				"retain_artifacts_on_failure": true,
				"retain_worktree_on_validation_failure": true
			}
		}]
	}`)
	if _, err := ParseRoutingPolicyJSON(data); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("ParseRoutingPolicyJSON nested unknown field error = %v, want unknown field rejection", err)
	}
}

func TestRoutingPolicyFixtures(t *testing.T) {
	valid, err := os.ReadFile("testdata/routing-policy/default.json")
	if err != nil {
		t.Fatalf("read valid fixture: %v", err)
	}
	if _, err := ParseRoutingPolicyJSON(valid); err != nil {
		t.Fatalf("valid routing policy fixture rejected: %v", err)
	}

	invalid, err := os.ReadFile("testdata/routing-policy/invalid-gascity-production.json")
	if err != nil {
		t.Fatalf("read invalid fixture: %v", err)
	}
	if _, err := ParseRoutingPolicyJSON(invalid); err == nil {
		t.Fatal("expected invalid GasCity production fixture to be rejected")
	}
}

func TestRoutingPolicyFixtureWireFields(t *testing.T) {
	valid, err := os.ReadFile("testdata/routing-policy/default.json")
	if err != nil {
		t.Fatalf("read valid fixture: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(valid, &raw); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	assertJSONKeys(t, raw, []string{
		"schema_version",
		"policy_id",
		"default_lane",
		"max_total_concurrency",
		"auto_merge_enabled",
		"manual_merge_by_default",
		"lanes",
	})

	lanes, ok := raw["lanes"].([]any)
	if !ok || len(lanes) != 3 {
		t.Fatalf("lanes = %#v, want three JSON objects", raw["lanes"])
	}
	for _, rawLane := range lanes {
		lane, ok := rawLane.(map[string]any)
		if !ok {
			t.Fatalf("lane = %#v, want object", rawLane)
		}
		assertJSONKeys(t, lane, []string{
			"id",
			"enabled",
			"authority",
			"provider",
			"runtime",
			"model",
			"task_classes",
			"max_concurrency",
			"cost_hint_usd_per_hour",
			"latency_hint",
			"quality_prior",
			"yield_gate",
			"promotion_gate",
			"merge_eligibility",
			"disabled_reason",
		})
		if eligibility, ok := lane["merge_eligibility"].(map[string]any); ok {
			assertJSONKeys(t, eligibility, []string{
				"manual_merge_required",
				"validation_commands",
				"validation_failure_terminal_event",
				"retain_artifacts_on_failure",
				"retain_worktree_on_validation_failure",
			})
		}
	}
}

func TestRoutingPolicySelectLane(t *testing.T) {
	policy := DefaultFactoryRoutingPolicy()

	lane, err := policy.SelectLane("code_change")
	if err != nil {
		t.Fatalf("SelectLane code_change: %v", err)
	}
	if lane.ID != "frontier-codex" {
		t.Fatalf("code_change lane = %q, want frontier-codex", lane.ID)
	}

	lane, err = policy.SelectLane("preflight")
	if err != nil {
		t.Fatalf("SelectLane preflight: %v", err)
	}
	if lane.ID != "local-observer" {
		t.Fatalf("preflight lane = %q, want local-observer", lane.ID)
	}

	if _, err := policy.SelectLane("unknown_task"); err == nil {
		t.Fatal("expected unknown task class to fail closed")
	}
}

func TestRoutingPolicyRejectsLocalDelegatedAuthority(t *testing.T) {
	policy := DefaultFactoryRoutingPolicy()
	policy.Lanes[1].Authority = RoutingAuthorityDelegated
	err := ValidateRoutingPolicy(policy)
	if err == nil {
		t.Fatal("expected local delegated lane to be rejected")
	}
	if !strings.Contains(err.Error(), "local provider authority") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoutingPolicyRejectsAutoMergeDefault(t *testing.T) {
	policy := DefaultFactoryRoutingPolicy()
	policy.ManualMergeByDefault = false
	err := ValidateRoutingPolicy(policy)
	if err == nil {
		t.Fatal("expected manual_merge_by_default=false to be rejected")
	}
	if !strings.Contains(err.Error(), "manual_merge_by_default") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoutingPolicyRejectsAutoMergeEnabled(t *testing.T) {
	policy := DefaultFactoryRoutingPolicy()
	policy.AutoMergeEnabled = true
	err := ValidateRoutingPolicy(policy)
	if err == nil {
		t.Fatal("expected auto_merge_enabled=true to be rejected")
	}
	if !strings.Contains(err.Error(), "auto_merge_enabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoutingPolicyRejectsMergeEligibleLaneWithoutValidationCommands(t *testing.T) {
	policy := DefaultFactoryRoutingPolicy()
	policy.Lanes[0].MergeEligibility.ValidationCommands = nil
	err := ValidateRoutingPolicy(policy)
	if err == nil {
		t.Fatal("expected missing validation commands to be rejected")
	}
	if !strings.Contains(err.Error(), "merge_eligibility.validation_commands") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoutingPolicyRejectsMergeEligibleLaneWithoutEligibility(t *testing.T) {
	policy := DefaultFactoryRoutingPolicy()
	policy.Lanes[0].MergeEligibility = nil
	err := ValidateRoutingPolicy(policy)
	if err == nil {
		t.Fatal("expected missing merge eligibility to be rejected")
	}
	if !strings.Contains(err.Error(), "merge_eligibility") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoutingPolicyRejectsInvalidMergeEligibilityTerminalEvent(t *testing.T) {
	policy := DefaultFactoryRoutingPolicy()
	policy.Lanes[0].MergeEligibility.ValidationFailureTerminalEvent = EventFactoryValidationCompleted
	err := ValidateRoutingPolicy(policy)
	if err == nil {
		t.Fatal("expected non-terminal validation failure event to be rejected")
	}
	if !strings.Contains(err.Error(), "validation_failure_terminal_event") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoutingPolicyRejectsValidationFailureWithoutRetentionSemantics(t *testing.T) {
	policy := DefaultFactoryRoutingPolicy()
	policy.Lanes[0].MergeEligibility.RetainArtifactsOnFailure = false
	err := ValidateRoutingPolicy(policy)
	if err == nil {
		t.Fatal("expected missing artifact retention to be rejected")
	}
	if !strings.Contains(err.Error(), "retain_artifacts_on_failure") {
		t.Fatalf("unexpected error: %v", err)
	}

	policy = DefaultFactoryRoutingPolicy()
	policy.Lanes[0].MergeEligibility.RetainWorktreeOnValidationFailure = false
	err = ValidateRoutingPolicy(policy)
	if err == nil {
		t.Fatal("expected missing worktree retention to be rejected")
	}
	if !strings.Contains(err.Error(), "retain_worktree_on_validation_failure") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoutingPolicyRejectsMergeEligibilityWithoutManualMerge(t *testing.T) {
	policy := DefaultFactoryRoutingPolicy()
	policy.Lanes[0].MergeEligibility.ManualMergeRequired = false
	err := ValidateRoutingPolicy(policy)
	if err == nil {
		t.Fatal("expected merge eligibility without manual merge to be rejected")
	}
	if !strings.Contains(err.Error(), "manual_merge_required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoutingPolicyRejectsGasCityProductionLane(t *testing.T) {
	policy := DefaultFactoryRoutingPolicy()
	policy.Lanes[2].Enabled = true
	policy.Lanes[2].MaxConcurrency = 1
	policy.Lanes[2].TaskClasses = []string{"code_change"}
	policy.Lanes[2].DisabledReason = ""
	err := ValidateRoutingPolicy(policy)
	if err == nil {
		t.Fatal("expected GasCity production lane to be rejected")
	}
	if !strings.Contains(err.Error(), "GasCity/Mt. Olympus production lanes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoutingPolicyRejectsDisabledLaneWithoutReason(t *testing.T) {
	policy := DefaultFactoryRoutingPolicy()
	policy.Lanes[2].DisabledReason = ""
	err := ValidateRoutingPolicy(policy)
	if err == nil {
		t.Fatal("expected disabled lane without reason to be rejected")
	}
	if !strings.Contains(err.Error(), "disabled lanes require disabled_reason") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoutingPolicyRejectsDuplicateLane(t *testing.T) {
	policy := DefaultFactoryRoutingPolicy()
	policy.Lanes[1].ID = policy.Lanes[0].ID
	if err := ValidateRoutingPolicy(policy); err == nil {
		t.Fatal("expected duplicate lane id to be rejected")
	}
}

func assertJSONKeys(t *testing.T, values map[string]any, allowed []string) {
	t.Helper()
	allowedSet := map[string]struct{}{}
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for key := range values {
		if _, ok := allowedSet[key]; !ok {
			t.Fatalf("unexpected JSON field %q in %#v", key, values)
		}
	}
}
