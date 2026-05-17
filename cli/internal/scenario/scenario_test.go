package scenario

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

func TestCreate_WritesSchemaCompliantScenario(t *testing.T) {
	dir := t.TempDir()
	res, err := Create(CreateOptions{
		Goal: "CLI authors a scenario", Threshold: 0.9, Status: "active",
		Source: "agent", DirectiveID: "d-target", Dir: dir,
		Now: fixedClock(time.Date(2026, 5, 17, 9, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if res.Scenario.ID != "s-2026-05-17-001" {
		t.Errorf("ID = %q, want s-2026-05-17-001", res.Scenario.ID)
	}
	if res.Scenario.DirectiveID != "d-target" {
		t.Errorf("DirectiveID = %q, want d-target", res.Scenario.DirectiveID)
	}
	if res.Scenario.Date != "2026-05-17" || res.Scenario.Status != "active" ||
		res.Scenario.SatisfactionThreshold != 0.9 {
		t.Errorf("scenario metadata = %+v", res.Scenario)
	}
	if res.Scenario.Narrative == "" || res.Scenario.ExpectedOutcome == "" {
		t.Error("Create must infer non-empty narrative and expected_outcome")
	}

	data, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("scenario file not written: %v", err)
	}
	var round Scenario
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("written scenario is not valid JSON: %v", err)
	}
	if round.ID != res.Scenario.ID || round.DirectiveID != "d-target" {
		t.Errorf("round-trip mismatch: %+v", round)
	}
}

func TestCreate_IncrementsSameDayID(t *testing.T) {
	dir := t.TempDir()
	clock := fixedClock(time.Date(2026, 5, 17, 9, 0, 0, 0, time.UTC))
	for want := 1; want <= 3; want++ {
		res, err := Create(CreateOptions{
			Goal: "g", Threshold: 0.8, Status: "draft", Source: "human", Dir: dir, Now: clock,
		})
		if err != nil {
			t.Fatalf("Create #%d: %v", want, err)
		}
		expect := fmt.Sprintf("s-2026-05-17-%03d", want)
		if res.Scenario.ID != expect {
			t.Errorf("Create #%d ID = %q, want %q", want, res.Scenario.ID, expect)
		}
	}
}

func TestCreate_RejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		opts CreateOptions
	}{
		{"empty goal", CreateOptions{Goal: "  ", Threshold: 0.8, Status: "draft", Source: "human"}},
		{"bad threshold", CreateOptions{Goal: "g", Threshold: 1.5, Status: "draft", Source: "human"}},
		{"bad status", CreateOptions{Goal: "g", Threshold: 0.8, Status: "blocked", Source: "human"}},
		{"bad source", CreateOptions{Goal: "g", Threshold: 0.8, Status: "draft", Source: "alien"}},
		{"bad directive id", CreateOptions{Goal: "g", Threshold: 0.8, Status: "draft", Source: "human", DirectiveID: "Bad ID"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.Dir = t.TempDir()
			if _, err := Create(tc.opts); err == nil {
				t.Errorf("Create(%s) succeeded, want a validation error", tc.name)
			}
		})
	}
}
