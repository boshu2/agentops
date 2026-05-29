// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestInMemoryOrchestration_SelectLadder(t *testing.T) {
	tests := []struct {
		name            string
		ntmAvailable    bool
		claudeAvailable bool
		work            WorkSpec
		want            Backend
	}{
		{
			name:            "default prefers ntm",
			ntmAvailable:    true,
			claudeAvailable: true,
			work:            WorkSpec{},
			want:            BackendNTM,
		},
		{
			name:            "ntm unavailable falls back to claude",
			ntmAvailable:    false,
			claudeAvailable: true,
			work:            WorkSpec{},
			want:            BackendClaude,
		},
		{
			name:            "ntm and claude unavailable degrades to beads floor",
			ntmAvailable:    false,
			claudeAvailable: false,
			work:            WorkSpec{},
			want:            BackendBeads,
		},
		{
			name:            "opt-out routes to beads despite availability",
			ntmAvailable:    true,
			claudeAvailable: true,
			work:            WorkSpec{OptOut: true},
			want:            BackendBeads,
		},
		{
			name:            "pin claude wins over ntm availability",
			ntmAvailable:    true,
			claudeAvailable: true,
			work:            WorkSpec{Pin: BackendClaude},
			want:            BackendClaude,
		},
		{
			name:            "pin beads wins",
			ntmAvailable:    true,
			claudeAvailable: true,
			work:            WorkSpec{Pin: BackendBeads},
			want:            BackendBeads,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			port := &InMemoryOrchestration{
				NTMAvailable:    tc.ntmAvailable,
				ClaudeAvailable: tc.claudeAvailable,
			}
			trace, err := port.Select(context.Background(), tc.work)
			if err != nil {
				t.Fatal(err)
			}
			if trace.Chosen != tc.want {
				t.Fatalf("Chosen = %q, want %q", trace.Chosen, tc.want)
			}
			if trace.Reason == "" {
				t.Fatal("Reason is empty, want a non-empty selection reason")
			}
		})
	}
}

func TestInMemoryOrchestration_PinOverridesOptOut(t *testing.T) {
	port := &InMemoryOrchestration{NTMAvailable: true, ClaudeAvailable: true}
	trace, err := port.Select(context.Background(), WorkSpec{Pin: BackendCodex, OptOut: true})
	if err != nil {
		t.Fatal(err)
	}
	if trace.Chosen != BackendCodex {
		t.Fatalf("Chosen = %q, want %q (pin must win over opt-out)", trace.Chosen, BackendCodex)
	}
}

func TestInMemoryOrchestration_ConsideredRecordsLadderSteps(t *testing.T) {
	port := &InMemoryOrchestration{NTMAvailable: false, ClaudeAvailable: false}
	trace, err := port.Select(context.Background(), WorkSpec{})
	if err != nil {
		t.Fatal(err)
	}
	want := []Backend{BackendNTM, BackendClaude, BackendBeads}
	if !reflect.DeepEqual(trace.Considered, want) {
		t.Fatalf("Considered = %v, want %v", trace.Considered, want)
	}
}

func TestInMemoryOrchestration_HonorsContextCancellation(t *testing.T) {
	port := &InMemoryOrchestration{NTMAvailable: true, ClaudeAvailable: true}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := port.Select(ctx, WorkSpec{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Select error = %v, want context.Canceled", err)
	}
}
