package orchestrationselect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/orchestration"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// fakeRunner is an in-memory CommandRunner so the Select path can be
// exercised without shelling out to a real `ntm` binary.
type fakeRunner struct {
	out []byte
	err error
}

func (f fakeRunner) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return f.out, f.err
}

func TestWorkSpecFromFlags(t *testing.T) {
	tests := []struct {
		name    string
		pin     string
		optOut  bool
		wantPin ports.Backend
		wantOpt bool
	}{
		{name: "empty", pin: "", optOut: false, wantPin: "", wantOpt: false},
		{name: "pin trimmed", pin: "  claude  ", optOut: false, wantPin: ports.BackendClaude, wantOpt: false},
		{name: "opt-out", pin: "", optOut: true, wantPin: "", wantOpt: true},
		{name: "pin wins over opt-out flags", pin: "codex", optOut: true, wantPin: ports.BackendCodex, wantOpt: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := WorkSpecFromFlags(tc.pin, tc.optOut)
			if got.Pin != tc.wantPin {
				t.Fatalf("Pin: got %q, want %q", got.Pin, tc.wantPin)
			}
			if got.OptOut != tc.wantOpt {
				t.Fatalf("OptOut: got %v, want %v", got.OptOut, tc.wantOpt)
			}
		})
	}
}

// TestSelectResolvesBackends drives a real Selector with an injected fake
// runner across the ladder branches, asserting the chosen backend for each flag
// combination.
func TestSelectResolvesBackends(t *testing.T) {
	t.Setenv("AGENTOPS_ORCHESTRATION", "") // neutralize any operator override

	tests := []struct {
		name   string
		runner orchestration.CommandRunner
		pin    string
		optOut bool
		want   ports.Backend
	}{
		{
			name:   "ntm absent degrades to claude",
			runner: fakeRunner{err: errors.New("ntm: not found")},
			want:   ports.BackendClaude,
		},
		{
			name:   "ntm available selects ntm",
			runner: fakeRunner{out: []byte(`{"capabilities":["tmux","git"]}`)},
			want:   ports.BackendNTM,
		},
		{
			name:   "opt-out routes to beads floor",
			runner: fakeRunner{err: errors.New("ntm: not found")},
			optOut: true,
			want:   ports.BackendBeads,
		},
		{
			name:   "pin wins over availability",
			runner: fakeRunner{out: []byte(`{"capabilities":["tmux","git"]}`)},
			pin:    "claude",
			want:   ports.BackendClaude,
		},
		{
			name:   "pin codex (never auto-selected) honored",
			runner: fakeRunner{err: errors.New("ntm: not found")},
			pin:    "codex",
			want:   ports.BackendCodex,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			selector := orchestration.NewSelector(tc.runner)
			work := WorkSpecFromFlags(tc.pin, tc.optOut)
			trace, err := selector.Select(context.Background(), work)
			if err != nil {
				t.Fatalf("Select returned error: %v", err)
			}
			if trace.Chosen != tc.want {
				t.Fatalf("Chosen: got %q, want %q", trace.Chosen, tc.want)
			}
			if len(trace.Considered) == 0 {
				t.Fatal("Considered ladder must be recorded")
			}
		})
	}
}

// TestRenderSelectionTraceJSON asserts the JSON branch emits the trace verbatim
// and parses back into the port shape.
func TestRenderSelectionTraceJSON(t *testing.T) {
	trace := ports.SelectionTrace{
		Chosen:     ports.BackendBeads,
		Reason:     "WorkSpec.OptOut -> beads floor",
		Considered: []ports.Backend{"pin", "env", "optout"},
	}
	var buf bytes.Buffer

	if err := RenderSelectionTrace(&buf, trace, true); err != nil {
		t.Fatalf("RenderSelectionTrace: %v", err)
	}

	var got ports.SelectionTrace
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got.Chosen != ports.BackendBeads {
		t.Fatalf("Chosen: got %q, want %q", got.Chosen, ports.BackendBeads)
	}
}

// TestRenderSelectionTraceHuman asserts the human-readable branch renders the
// backend, reason, and ladder.
func TestRenderSelectionTraceHuman(t *testing.T) {
	trace := ports.SelectionTrace{
		Chosen:     ports.BackendClaude,
		Reason:     "NTM absent -> claude-native fallback",
		Considered: []ports.Backend{"pin", "env", "optout", "ntm", "claude", "beads"},
	}
	var buf bytes.Buffer

	if err := RenderSelectionTrace(&buf, trace, false); err != nil {
		t.Fatalf("RenderSelectionTrace: %v", err)
	}

	got := buf.String()
	for _, want := range []string{"Backend: claude", "Reason:  NTM absent", "pin -> env -> optout -> ntm -> claude -> beads"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}
