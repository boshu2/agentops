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

func TestOperatorRecord_EmptyKindRejected(t *testing.T) {
	err := operatorRecordRun(context.Background(), operatorOptions{
		kind: "",
	})
	if err == nil {
		t.Fatal("expected error on empty kind, got nil")
	}
	if !strings.Contains(err.Error(), "--kind required") {
		t.Fatalf("error not informative: %v", err)
	}
}

func TestOperatorRecord_StubCalled(t *testing.T) {
	called := false
	var gotOpts operatorOptions
	stub := func(_ context.Context, opts operatorOptions) error {
		called = true
		gotOpts = opts
		return nil
	}
	var buf bytes.Buffer
	err := operatorRecordRun(context.Background(), operatorOptions{
		kind:     "halt",
		subject:  "soc-test",
		note:     "investigate",
		writer:   &buf,
		recordFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("recordFn not invoked")
	}
	if gotOpts.kind != "halt" || gotOpts.subject != "soc-test" || gotOpts.note != "investigate" {
		t.Fatalf("opts mismatch: %+v", gotOpts)
	}
	if !strings.Contains(buf.String(), "recorded intent") {
		t.Fatalf("confirmation message missing: %q", buf.String())
	}
}

func TestOperatorRecord_StubErrorWrapped(t *testing.T) {
	stub := func(_ context.Context, _ operatorOptions) error {
		return errors.New("disk full")
	}
	err := operatorRecordRun(context.Background(), operatorOptions{
		kind:     "halt",
		recordFn: stub,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "operator record:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}

func TestOperatorList_StubReturnsIntents(t *testing.T) {
	stub := func(_ context.Context, _ operatorOptions) ([]ports.OperatorIntent, error) {
		return []ports.OperatorIntent{
			{Kind: "halt", Subject: "soc-1"},
			{Kind: "rescope", Subject: "soc-2", Note: "note text"},
		}, nil
	}
	var buf bytes.Buffer
	err := operatorListRun(context.Background(), operatorOptions{
		writer: &buf,
		listFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("len = %d, want 2", len(lines))
	}
	if !strings.Contains(lines[0], `"Kind":"halt"`) {
		t.Fatalf("missing Kind:halt: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"Note":"note text"`) {
		t.Fatalf("missing Note: %s", lines[1])
	}
}

func TestOperatorList_EmptyIntents(t *testing.T) {
	stub := func(_ context.Context, _ operatorOptions) ([]ports.OperatorIntent, error) {
		return []ports.OperatorIntent{}, nil
	}
	var buf bytes.Buffer
	err := operatorListRun(context.Background(), operatorOptions{
		writer: &buf,
		listFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("empty should emit 0 bytes, got %q", buf.String())
	}
}

func TestOperatorList_StubErrorWrapped(t *testing.T) {
	stub := func(_ context.Context, _ operatorOptions) ([]ports.OperatorIntent, error) {
		return nil, errors.New("file unreadable")
	}
	err := operatorListRun(context.Background(), operatorOptions{
		listFn: stub,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "operator list:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}
