// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestInMemoryCloseout_CompletesRequestedActions(t *testing.T) {
	closeout := NewInMemoryCloseout()
	closeout.Artifacts = []string{".agents/handoff.md"}
	result, err := closeout.Close(context.Background(), CloseoutRequest{
		RunID:   "run-1",
		Actions: []CloseoutAction{CloseoutActionHandoff, CloseoutActionFlywheel},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Completed) != 2 || result.Completed[0] != CloseoutActionHandoff {
		t.Fatalf("Completed = %#v, want handoff + flywheel", result.Completed)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0] != ".agents/handoff.md" {
		t.Fatalf("Artifacts = %#v, want handoff artifact", result.Artifacts)
	}
	if len(closeout.Requests) != 1 || closeout.Requests[0].RunID != "run-1" {
		t.Fatalf("Requests = %#v, want recorded run-1 request", closeout.Requests)
	}
}

func TestInMemoryCloseout_RejectsEmptyActions(t *testing.T) {
	closeout := NewInMemoryCloseout()
	_, err := closeout.Close(context.Background(), CloseoutRequest{})
	if err == nil {
		t.Fatal("expected error for empty actions")
	}
	if !strings.Contains(err.Error(), "actions required") {
		t.Fatalf("error = %v, want actions required", err)
	}
}

func TestInMemoryCloseout_ReturnsDefensiveCopies(t *testing.T) {
	closeout := NewInMemoryCloseout()
	closeout.Artifacts = []string{"a"}
	first, err := closeout.Close(context.Background(), CloseoutRequest{Actions: []CloseoutAction{CloseoutActionDefrag}})
	if err != nil {
		t.Fatal(err)
	}
	first.Completed[0] = CloseoutActionMaintain
	first.Artifacts[0] = "mutated"
	second, err := closeout.Close(context.Background(), CloseoutRequest{Actions: []CloseoutAction{CloseoutActionDefrag}})
	if err != nil {
		t.Fatal(err)
	}
	if second.Completed[0] != CloseoutActionDefrag || second.Artifacts[0] != "a" {
		t.Fatalf("second result was mutated: %#v", second)
	}
}

func TestInMemoryCloseout_HonorsContextCancellation(t *testing.T) {
	closeout := NewInMemoryCloseout()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := closeout.Close(ctx, CloseoutRequest{Actions: []CloseoutAction{CloseoutActionHandoff}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
