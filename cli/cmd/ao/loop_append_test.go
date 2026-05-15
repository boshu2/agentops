// practices: [tdd]
package main

import (
	"bytes"
	"context"
	"errors"
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
