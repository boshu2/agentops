package orchestration

import (
	"context"
	"errors"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// selectUpRunner returns a fakeRunner that reports NTM present with all
// hard deps, so ProbeNTM reports Available=true.
func selectUpRunner() *fakeRunner {
	return &fakeRunner{
		out: []byte(`{"capabilities":["agent-CLIs","git","persistent-host","tmux"]}`),
	}
}

// selectDownRunner returns a fakeRunner that errors, so ProbeNTM reports
// Available=false (the NTM-absent degradation signal).
func selectDownRunner() *fakeRunner {
	return &fakeRunner{
		err: errors.New(`exec: "ntm": executable file not found in $PATH`),
	}
}

// TestSelect_Ladder reproduces the six degradation-ladder cases the
// shell self-test in lib/orchestrate-select.sh asserts, adapted to the
// typed port's resolution order.
func TestSelect_Ladder(t *testing.T) {
	tests := []struct {
		name   string
		env    string // "" => unset
		setEnv bool
		runner *fakeRunner
		work   ports.WorkSpec
		want   ports.Backend
	}{
		{
			name:   "default (NTM up) -> ntm",
			runner: selectUpRunner(),
			want:   ports.BackendNTM,
		},
		{
			name:   "NTM down -> degrade to claude",
			runner: selectDownRunner(),
			want:   ports.BackendClaude,
		},
		{
			name:   "NTM down + OptOut -> beads floor",
			runner: selectDownRunner(),
			work:   ports.WorkSpec{OptOut: true},
			want:   ports.BackendBeads,
		},
		{
			name:   "env=off -> beads (explicit opt-out)",
			env:    "off",
			setEnv: true,
			runner: selectUpRunner(),
			want:   ports.BackendBeads,
		},
		{
			name:   "Pin=claude -> claude",
			runner: selectUpRunner(),
			work:   ports.WorkSpec{Pin: ports.BackendClaude},
			want:   ports.BackendClaude,
		},
		{
			name:   "Pin=beads -> beads",
			runner: selectUpRunner(),
			work:   ports.WorkSpec{Pin: ports.BackendBeads},
			want:   ports.BackendBeads,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure the env var is in a known state for each case.
			if tt.setEnv {
				t.Setenv(selectEnvVar, tt.env)
			} else {
				t.Setenv(selectEnvVar, "")
			}

			sel := NewSelector(tt.runner)
			trace, err := sel.Select(context.Background(), tt.work)
			if err != nil {
				t.Fatalf("Select returned error: %v", err)
			}
			if trace.Chosen != tt.want {
				t.Fatalf("Chosen = %q, want %q", trace.Chosen, tt.want)
			}
			if trace.Reason == "" {
				t.Errorf("Reason is empty, want a non-empty explanation")
			}
			if len(trace.Considered) == 0 {
				t.Errorf("Considered is empty, want the evaluated ladder steps")
			}
		})
	}
}

// TestSelect_PinBeatsEverything asserts the contract that a non-empty
// Pin wins over both OptOut and the env override.
func TestSelect_PinBeatsEverything(t *testing.T) {
	t.Setenv(selectEnvVar, "off") // env says beads...
	sel := NewSelector(selectDownRunner())

	trace, err := sel.Select(context.Background(), ports.WorkSpec{
		Pin:    ports.BackendNTM, // ...but Pin says ntm.
		OptOut: true,             // ...and OptOut says beads.
	})
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if trace.Chosen != ports.BackendNTM {
		t.Fatalf("Chosen = %q, want %q (Pin must win)", trace.Chosen, ports.BackendNTM)
	}
}

// TestSelect_EnvPinNTM asserts AGENTOPS_ORCHESTRATION=ntm pins NTM even
// when the probe would report it down.
func TestSelect_EnvPinNTM(t *testing.T) {
	t.Setenv(selectEnvVar, "ntm")
	sel := NewSelector(selectDownRunner()) // probe would say down

	trace, err := sel.Select(context.Background(), ports.WorkSpec{})
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if trace.Chosen != ports.BackendNTM {
		t.Fatalf("Chosen = %q, want %q (env pin must win over probe)", trace.Chosen, ports.BackendNTM)
	}
}

// TestSelect_EnvUnknownFallsThrough asserts an unrecognized env value
// falls through to the availability ladder rather than pinning.
func TestSelect_EnvUnknownFallsThrough(t *testing.T) {
	t.Setenv(selectEnvVar, "bogus")
	sel := NewSelector(selectUpRunner()) // NTM up

	trace, err := sel.Select(context.Background(), ports.WorkSpec{})
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if trace.Chosen != ports.BackendNTM {
		t.Fatalf("Chosen = %q, want %q (unknown env -> auto ladder, NTM up)", trace.Chosen, ports.BackendNTM)
	}
}

// TestSelect_ContextCanceled asserts cancellation is honored before the
// probe runs.
func TestSelect_ContextCanceled(t *testing.T) {
	t.Setenv(selectEnvVar, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sel := NewSelector(selectUpRunner())
	_, err := sel.Select(ctx, ports.WorkSpec{})
	if err == nil {
		t.Fatalf("Select returned nil error on canceled context, want error")
	}
}
