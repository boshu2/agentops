// practices: [tdd]
package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// soc-y5vh.8: `ao loop hypothesis {list,append}` exposes the BC3
// HypothesisLedgerPort.

func TestLoopHypothesisAppend_EmptyIDRejected(t *testing.T) {
	err := loopHypothesisAppendRun(context.Background(), loopHypothesisAppendOptions{
		hypothesis: "raises pass rate",
		measure:    "count cycles",
	})
	if err == nil {
		t.Fatal("expected error on empty --id")
	}
	if !strings.Contains(err.Error(), "--id required") {
		t.Fatalf("error not informative: %v", err)
	}
}

func TestLoopHypothesisAppend_StubCalledWithRecord(t *testing.T) {
	var got ports.HypothesisRecord
	stub := func(_ context.Context, opts loopHypothesisAppendOptions) (ports.HypothesisRecord, error) {
		got = ports.HypothesisRecord{
			ID:           opts.id,
			Patch:        opts.patch,
			Hypothesis:   opts.hypothesis,
			Measure:      opts.measure,
			CycleLanded:  opts.cycleLanded,
			CheckAtCycle: opts.checkAtCycle,
			Verdict:      ports.HypothesisVerdict(opts.verdict),
		}
		return got, nil
	}
	var buf bytes.Buffer
	err := loopHypothesisAppendRun(context.Background(), loopHypothesisAppendOptions{
		id:           "H210.1",
		patch:        "Step 1.5 typed CI probe",
		hypothesis:   "removes gh shell-outs",
		measure:      "grep -c gh evolve hot path",
		cycleLanded:  210,
		checkAtCycle: 225,
		verdict:      "PENDING",
		writer:       &buf,
		appendFn:     stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "H210.1" || got.CheckAtCycle != 225 || got.Hypothesis != "removes gh shell-outs" {
		t.Fatalf("record mis-mapped: %+v", got)
	}
	if !strings.Contains(buf.String(), "H210.1") {
		t.Fatalf("append output missing id: %q", buf.String())
	}
}

func TestLoopHypothesisList_RendersRecordsAsJSONL(t *testing.T) {
	stub := func(_ context.Context, _ loopHypothesisListOptions) ([]ports.HypothesisRecord, error) {
		return []ports.HypothesisRecord{
			{ID: "H45.1", Verdict: ports.HypothesisVerdictPending},
			{ID: "H45.2", Verdict: ports.HypothesisVerdictFalsified},
		}, nil
	}
	var buf bytes.Buffer
	err := loopHypothesisListRun(context.Background(), loopHypothesisListOptions{
		writer: &buf,
		listFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Count(strings.TrimSpace(buf.String()), "\n") + 1
	if lines != 2 {
		t.Fatalf("expected 2 JSONL lines, got %d: %q", lines, buf.String())
	}
	if !strings.Contains(buf.String(), `"id":"H45.1"`) || !strings.Contains(buf.String(), `"id":"H45.2"`) {
		t.Fatalf("list output missing records: %q", buf.String())
	}
}

// L2: append then list round-trip through the production adapter at an
// explicit ledger path.
func TestLoopHypothesis_AppendListRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hypotheses.jsonl")
	rec := ports.HypothesisRecord{
		ID:           "H300.1",
		Hypothesis:   "round-trips",
		Measure:      "this test",
		CheckAtCycle: 315,
		Verdict:      ports.HypothesisVerdictPending,
	}
	if _, err := appendHypothesisAt(context.Background(), path, rec); err != nil {
		t.Fatalf("appendHypothesisAt: %v", err)
	}
	records, err := listHypothesesAt(context.Background(), path)
	if err != nil {
		t.Fatalf("listHypothesesAt: %v", err)
	}
	if len(records) != 1 || records[0].ID != "H300.1" || records[0].CheckAtCycle != 315 {
		t.Fatalf("round-trip mismatch: %+v", records)
	}
}
