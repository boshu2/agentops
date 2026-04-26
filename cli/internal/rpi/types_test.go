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

// TestNextWorkItem_ProposalTwoFieldsRoundTrip verifies status / requires /
// dedup_key — the first-class fields needed by RFC 0001 Proposal 2 — survive
// JSON round trip and stay omitted when absent.
func TestNextWorkItem_ProposalTwoFieldsRoundTrip(t *testing.T) {
	in := NextWorkItem{
		Title:    "watch competitor X release notes",
		Type:     "task",
		Severity: "medium",
		Source:   "evolve-generator",
		Status:   "proposed",
		Requires: []string{"human-review"},
		DedupKey: "external-watchlist|github.com/example/x|v1.2.0",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"status":"proposed"`,
		`"requires":["human-review"]`,
		`"dedup_key":"external-watchlist|github.com/example/x|v1.2.0"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in marshaled item; json = %s", want, got)
		}
	}

	var out NextWorkItem
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "proposed" {
		t.Errorf("Status = %q, want proposed", out.Status)
	}
	if len(out.Requires) != 1 || out.Requires[0] != "human-review" {
		t.Errorf("Requires = %v, want [human-review]", out.Requires)
	}
	if out.DedupKey != "external-watchlist|github.com/example/x|v1.2.0" {
		t.Errorf("DedupKey round-trip = %q", out.DedupKey)
	}

	bare := NextWorkItem{Title: "x", Type: "task", Severity: "low", Source: "evolve-generator"}
	bb, _ := json.Marshal(bare)
	bareJSON := string(bb)
	for _, k := range []string{"status", "requires", "dedup_key"} {
		if strings.Contains(bareJSON, k) {
			t.Errorf("expected %q to be omitted when empty; json = %s", k, bareJSON)
		}
	}
}

// TestIsQueueItemHeldForReview captures the selector contract for Proposal 2:
// status=proposed and any requires entry hold the item; status=ready and the
// empty/zero state remain selectable.
func TestIsQueueItemHeldForReview(t *testing.T) {
	cases := []struct {
		name string
		item NextWorkItem
		held bool
	}{
		{"empty item is released", NextWorkItem{}, false},
		{"explicit ready is released", NextWorkItem{Status: "ready"}, false},
		{"status proposed is held", NextWorkItem{Status: "proposed"}, true},
		{"unknown status is held (fail-safe)", NextWorkItem{Status: "experimental"}, true},
		{"requires human-review is held", NextWorkItem{Requires: []string{"human-review"}}, true},
		{"requires unknown gate is held", NextWorkItem{Requires: []string{"legal-review"}}, true},
		{"empty requires slice is released", NextWorkItem{Requires: []string{}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsQueueItemHeldForReview(tc.item); got != tc.held {
				t.Errorf("IsQueueItemHeldForReview(%+v) = %v, want %v", tc.item, got, tc.held)
			}
		})
	}
}

// TestIsQueueItemSelectable_RespectsHumanReviewHold verifies the existing
// selector entry point now honors Proposal 2 release semantics — a held item
// is never selectable even when claim_status is available and consumed=false.
func TestIsQueueItemSelectable_RespectsHumanReviewHold(t *testing.T) {
	available := NextWorkItem{Title: "x", Type: "task", Severity: "low", Source: "evolve-generator"}
	if !IsQueueItemSelectable(available) {
		t.Fatalf("expected baseline available item to be selectable")
	}
	held := available
	held.Status = "proposed"
	held.Requires = []string{"human-review"}
	if IsQueueItemSelectable(held) {
		t.Errorf("held item with status=proposed and requires=human-review must not be selectable")
	}
	heldStatusOnly := available
	heldStatusOnly.Status = "proposed"
	if IsQueueItemSelectable(heldStatusOnly) {
		t.Errorf("status=proposed alone must hold the item")
	}
	heldRequiresOnly := available
	heldRequiresOnly.Requires = []string{"human-review"}
	if IsQueueItemSelectable(heldRequiresOnly) {
		t.Errorf("requires=human-review alone must hold the item")
	}
}
