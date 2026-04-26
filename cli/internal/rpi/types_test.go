package rpi

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNextWorkItem_ProbedFieldsRoundTrip verifies the probed_stale_at /
// probed_by fields survive a Marshal/Unmarshal round trip and end up in the
// expected JSON shape. Added for the 2026-04-26 nightly retro task 3 so a
// future schema change can't silently drop the field.
func TestNextWorkItem_ProbedFieldsRoundTrip(t *testing.T) {
	stamp := "2026-04-26T22:30:00Z"
	by := "nightly/2026-04-26-v3"
	in := NextWorkItem{
		Title:         "stale item",
		Type:          "tech-debt",
		Severity:      "low",
		Source:        "council-finding",
		Description:   "already done — probe matched",
		ProbedStaleAt: &stamp,
		ProbedBy:      &by,
	}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"probed_stale_at":"2026-04-26T22:30:00Z"`) {
		t.Errorf("probed_stale_at missing or wrong shape; json = %s", got)
	}
	if !strings.Contains(got, `"probed_by":"nightly/2026-04-26-v3"`) {
		t.Errorf("probed_by missing or wrong shape; json = %s", got)
	}

	var out NextWorkItem
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ProbedStaleAt == nil || *out.ProbedStaleAt != stamp {
		t.Errorf("ProbedStaleAt round-trip = %v, want %q", out.ProbedStaleAt, stamp)
	}
	if out.ProbedBy == nil || *out.ProbedBy != by {
		t.Errorf("ProbedBy round-trip = %v, want %q", out.ProbedBy, by)
	}
}

// TestNextWorkItem_ProbedFieldsOmittedWhenAbsent verifies omitempty
// suppresses the new fields for items that have never been probed.
func TestNextWorkItem_ProbedFieldsOmittedWhenAbsent(t *testing.T) {
	in := NextWorkItem{
		Title:    "fresh item",
		Type:     "task",
		Severity: "medium",
		Source:   "evolve-generator",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if strings.Contains(got, "probed_stale_at") {
		t.Errorf("expected probed_stale_at to be omitted when nil; json = %s", got)
	}
	if strings.Contains(got, "probed_by") {
		t.Errorf("expected probed_by to be omitted when nil; json = %s", got)
	}
}
