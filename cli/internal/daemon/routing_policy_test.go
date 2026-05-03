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
	if !policy.ManualMergeByDefault {
		t.Fatal("default policy must require manual merge")
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
