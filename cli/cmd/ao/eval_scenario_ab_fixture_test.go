package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestScenarioABValidRedactedFixture_Auditable (age-wp1): the tracked redacted
// scorecard fixture must be present and carry the moat-eligible applied-OOD shape.
func TestScenarioABValidRedactedFixture_Auditable(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	fixture := filepath.Join(repo, "evals/scenarios/fixtures/scenario-ab-valid-redacted.scorecard.json")
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var card map[string]any
	if err := json.Unmarshal(raw, &card); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	for _, key := range []string{"verdict_class", "moat_eligible", "gate", "aggregate_delta"} {
		if _, ok := card[key]; !ok {
			t.Errorf("fixture missing key %q", key)
		}
	}
	if card["verdict_class"] != "applied-ood" {
		t.Errorf("verdict_class = %v, want applied-ood", card["verdict_class"])
	}
	if moat, _ := card["moat_eligible"].(bool); !moat {
		t.Errorf("moat_eligible = %v, want true", card["moat_eligible"])
	}
}
