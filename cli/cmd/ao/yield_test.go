package main

import (
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// TestEmitYieldEvent_AllKinds verifies each event kind decodes its JSON body and
// appends through the ledger, and that the bead-keyed projections see it.
func TestEmitYieldEvent_AllKinds(t *testing.T) {
	root := t.TempDir()
	ts := time.Date(2026, 6, 14, 18, 0, 0, 0, time.UTC)

	gvBody := `{"difficulty":3,"pawl_verdict_ref":{"bead_id":"ag-x","head_sha":"abc1234"},"disposition":"CONFIRMED","head_sha":"abc1234","attempt":1,"author_context_id":"ctx-1","refuter_families":["claude","gpt"],"author_family":"claude","cross_family":true,"author_ne_reviewer":true,"evidence_present":true}`
	if err := emitYieldEvent(root, yieldledger.EventGateVerdict, "ag-x", "r1", ts, gvBody); err != nil {
		t.Fatalf("emit gate-verdict: %v", err)
	}

	usageBody := `{"tokens_in":100,"tokens_out":20,"cost_usd":0.3,"wall_clock_s":60,"model":"claude-opus-4-8","phase":"implement","category_hint":"productive"}`
	if err := emitYieldEvent(root, yieldledger.EventUsage, "ag-x", "r1", ts, usageBody); err != nil {
		t.Fatalf("emit usage: %v", err)
	}

	acceptBody := `{"merge_sha":"def5678","merged_by":"orch","gate_verdict_ref":{"bead_id":"ag-x","head_sha":"abc1234"}}`
	if err := emitYieldEvent(root, yieldledger.EventAccept, "ag-x", "r1", ts, acceptBody); err != nil {
		t.Fatalf("emit accept: %v", err)
	}

	ledger, err := yieldledger.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := len(ledger.EventsFor("ag-x")); got != 3 {
		t.Fatalf("EventsFor(ag-x) = %d, want 3", got)
	}
	if got := len(ledger.AcceptsFor("ag-x")); got != 1 {
		t.Errorf("AcceptsFor(ag-x) = %d, want 1", got)
	}
	if got := len(ledger.GateVerdictsFor("ag-x")); got != 1 {
		t.Errorf("GateVerdictsFor(ag-x) = %d, want 1", got)
	}
	if got := len(ledger.UsageFor("ag-x")); got != 1 {
		t.Errorf("UsageFor(ag-x) = %d, want 1", got)
	}
}

// TestEmitYieldEvent_Rejects verifies envelope/body validation surfaces errors
// (so a malformed emit fails loudly even though callers swallow it with || true).
func TestEmitYieldEvent_Rejects(t *testing.T) {
	root := t.TempDir()
	ts := time.Now().UTC()
	cases := []struct {
		name                  string
		kind, bead, run, body string
	}{
		{"missing bead", yieldledger.EventUsage, "", "r1", `{"tokens_in":1,"tokens_out":1,"cost_usd":0.1,"wall_clock_s":1,"model":"m","phase":"implement"}`},
		{"missing run", yieldledger.EventUsage, "ag-x", "", `{"tokens_in":1,"tokens_out":1,"cost_usd":0.1,"wall_clock_s":1,"model":"m","phase":"implement"}`},
		{"missing body", yieldledger.EventUsage, "ag-x", "r1", ``},
		{"bad json", yieldledger.EventUsage, "ag-x", "r1", `{not json}`},
		{"invalid phase", yieldledger.EventUsage, "ag-x", "r1", `{"tokens_in":1,"tokens_out":1,"cost_usd":0.1,"wall_clock_s":1,"model":"m","phase":"bogus"}`},
		{"unknown kind", "bogus", "ag-x", "r1", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := emitYieldEvent(root, tc.kind, tc.bead, tc.run, ts, tc.body); err == nil {
				t.Error("emitYieldEvent accepted a bad input, want error")
			}
		})
	}
}
