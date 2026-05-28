// practices: [tdd]
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestLoopAppend_EmptyModeRejected(t *testing.T) {
	err := loopAppendRun(context.Background(), loopAppendOptions{
		result: "improved",
	})
	if err == nil {
		t.Fatal("expected error on empty mode")
	}
	if !strings.Contains(err.Error(), "--mode required") {
		t.Fatalf("error not informative: %v", err)
	}
}

func TestLoopAppend_EmptyResultRejected(t *testing.T) {
	err := loopAppendRun(context.Background(), loopAppendOptions{
		mode: "evolve",
	})
	if err == nil {
		t.Fatal("expected error on empty result")
	}
	if !strings.Contains(err.Error(), "--result required") {
		t.Fatalf("error not informative: %v", err)
	}
}

func TestLoopAppend_StubCalledWithEntry(t *testing.T) {
	stub := func(_ context.Context, opts loopAppendOptions) (ports.CycleEntry, error) {
		return ports.CycleEntry{
			Number:    opts.cycle,
			Mode:      opts.mode,
			Result:    opts.result,
			Commit:    opts.commit,
			Milestone: opts.milestone,
		}, nil
	}
	var buf bytes.Buffer
	err := loopAppendRun(context.Background(), loopAppendOptions{
		cycle:     42,
		mode:      "evolve",
		result:    "improved",
		commit:    "abc123",
		milestone: "test milestone",
		writer:    &buf,
		appendFn:  stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `cycle=42`) {
		t.Fatalf("missing cycle=42: %q", buf.String())
	}
	if !strings.Contains(buf.String(), `mode="evolve"`) {
		t.Fatalf("missing mode label: %q", buf.String())
	}
}

func TestLoopAppend_AutoAssignsCycleWhenZero(t *testing.T) {
	stub := func(_ context.Context, opts loopAppendOptions) (ports.CycleEntry, error) {
		// Simulate the port auto-assigning cycle 100
		return ports.CycleEntry{Number: 100, Mode: opts.mode, Result: opts.result}, nil
	}
	var buf bytes.Buffer
	err := loopAppendRun(context.Background(), loopAppendOptions{
		mode:     "evolve",
		result:   "improved",
		writer:   &buf,
		appendFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "cycle=100") {
		t.Fatalf("auto-assigned cycle not surfaced: %q", buf.String())
	}
}

// soc-y5vh.9: loadCycleTrace resolves --trace-json from inline JSON or a
// file path, and rejects malformed input.
func TestLoadCycleTrace_EmptyArgYieldsNil(t *testing.T) {
	tr, err := loadCycleTrace("")
	if err != nil {
		t.Fatalf("empty arg: %v", err)
	}
	if tr != nil {
		t.Fatalf("empty arg returned %+v, want nil", tr)
	}
}

func TestLoadCycleTrace_InlineJSON(t *testing.T) {
	tr, err := loadCycleTrace(`{"goal_hypothesis":"raise pass rate","ratchet_action":"record implement","bead_id":"soc-z4tl","acceptance_examples":["Scenario: closeout maps evidence"],"validation_commands":["go test ./cmd/ao -run TestLoadCycleTrace"],"closeout_verdict":"passed"}`)
	if err != nil {
		t.Fatalf("inline JSON: %v", err)
	}
	if tr == nil || tr.GoalHypothesis != "raise pass rate" || tr.RatchetAction != "record implement" {
		t.Fatalf("inline JSON parsed to %+v", tr)
	}
	if tr.BeadID != "soc-z4tl" || tr.CloseoutVerdict != "passed" {
		t.Fatalf("inline closeout fields parsed to %+v", tr)
	}
	if len(tr.AcceptanceExamples) != 1 || tr.AcceptanceExamples[0] != "Scenario: closeout maps evidence" {
		t.Fatalf("inline acceptance examples parsed to %+v", tr.AcceptanceExamples)
	}
	if len(tr.ValidationCommands) != 1 || tr.ValidationCommands[0] != "go test ./cmd/ao -run TestLoadCycleTrace" {
		t.Fatalf("inline validation commands parsed to %+v", tr.ValidationCommands)
	}
}

func TestLoadCycleTrace_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.json")
	if err := os.WriteFile(path, []byte(`{"exemption_reason":"trivial typo fix","bead_id":"soc-file","validation_commands":["bash scripts/check-loop-shape.sh --self-test"],"closeout_verdict":"exempt"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	tr, err := loadCycleTrace(path)
	if err != nil {
		t.Fatalf("file arg: %v", err)
	}
	if tr == nil || tr.ExemptionReason != "trivial typo fix" {
		t.Fatalf("file arg parsed to %+v", tr)
	}
	if tr.BeadID != "soc-file" || tr.CloseoutVerdict != "exempt" {
		t.Fatalf("file closeout fields parsed to %+v", tr)
	}
	if len(tr.ValidationCommands) != 1 || tr.ValidationCommands[0] != "bash scripts/check-loop-shape.sh --self-test" {
		t.Fatalf("file validation commands parsed to %+v", tr.ValidationCommands)
	}
}

func TestLoadCycleTrace_MalformedRejected(t *testing.T) {
	for _, bad := range []string{`{not json`, `[1,2,3]`, `"a string"`} {
		if _, err := loadCycleTrace(bad); err == nil {
			t.Errorf("loadCycleTrace(%q) accepted malformed input", bad)
		}
	}
}

func TestLoopAppend_StubErrorWrapped(t *testing.T) {
	stub := func(_ context.Context, _ loopAppendOptions) (ports.CycleEntry, error) {
		return ports.CycleEntry{}, errors.New("file locked")
	}
	err := loopAppendRun(context.Background(), loopAppendOptions{
		mode:     "x",
		result:   "y",
		appendFn: stub,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "loop append:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}
