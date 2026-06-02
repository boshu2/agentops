package orchestration

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// fakeRunner is an in-memory CommandRunner. It records the invocation
// and returns canned output/error so tests never shell out.
type fakeRunner struct {
	out []byte
	err error

	gotName string
	gotArgs []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.gotName = name
	f.gotArgs = args
	return f.out, f.err
}

func TestNTMProbe_PresentReportsCapabilities(t *testing.T) {
	// ntm present and reports all hard deps as capabilities.
	runner := &fakeRunner{
		out: []byte(`{"capabilities":["activity","dashboard","mail","send","spawn"]}`),
	}

	got, err := ProbeNTM(context.Background(), runner)
	if err != nil {
		t.Fatalf("ProbeNTM returned error: %v", err)
	}
	if !got.Available {
		t.Fatalf("Available = false, want true")
	}

	wantCaps := []string{"activity", "dashboard", "mail", "send", "spawn"}
	if !reflect.DeepEqual(got.Capabilities, wantCaps) {
		t.Fatalf("Capabilities = %v, want %v", got.Capabilities, wantCaps)
	}
	if len(got.MissingDeps) != 0 {
		t.Fatalf("MissingDeps = %v, want empty", got.MissingDeps)
	}

	// Verify the probe asked by capability, not via command -v.
	if runner.gotName != "ntm" {
		t.Errorf("invoked %q, want %q", runner.gotName, "ntm")
	}
	wantArgs := []string{"--robot-capabilities"}
	if !reflect.DeepEqual(runner.gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", runner.gotArgs, wantArgs)
	}
}

func TestNTMProbe_PresentWithMissingHardDeps(t *testing.T) {
	// ntm present but only reports a subset of hard deps.
	runner := &fakeRunner{
		out: []byte(`{"capabilities":["activity","dashboard"]}`),
	}

	got, err := ProbeNTM(context.Background(), runner)
	if err != nil {
		t.Fatalf("ProbeNTM returned error: %v", err)
	}
	if !got.Available {
		t.Fatalf("Available = false, want true")
	}

	wantMissing := []string{"mail", "send", "spawn"}
	if !reflect.DeepEqual(got.MissingDeps, wantMissing) {
		t.Fatalf("MissingDeps = %v, want %v", got.MissingDeps, wantMissing)
	}
}

func TestNTMProbe_CurrentRobotCommandsPayload(t *testing.T) {
	runner := &fakeRunner{
		out: []byte(`{"success":true,"commands":[{"name":"spawn"},{"name":"send"},{"name":"mail"},{"name":"dashboard"},{"name":"activity"}]}`),
	}

	got, err := ProbeNTM(context.Background(), runner)
	if err != nil {
		t.Fatalf("ProbeNTM returned error: %v", err)
	}
	if !got.Available {
		t.Fatalf("Available = false, want true")
	}
	wantCaps := []string{"activity", "dashboard", "mail", "send", "spawn"}
	if !reflect.DeepEqual(got.Capabilities, wantCaps) {
		t.Fatalf("Capabilities = %v, want %v", got.Capabilities, wantCaps)
	}
	if len(got.MissingDeps) != 0 {
		t.Fatalf("MissingDeps = %v, want empty", got.MissingDeps)
	}
}

func TestNTMProbe_AbsentDegradesGracefully(t *testing.T) {
	// runner returns an error => ntm is absent. Must NOT panic, must
	// return Available=false and a nil error.
	runner := &fakeRunner{
		err: errors.New("exec: \"ntm\": executable file not found in $PATH"),
	}

	got, err := ProbeNTM(context.Background(), runner)
	if err != nil {
		t.Fatalf("ProbeNTM returned error on absent ntm: %v, want nil", err)
	}
	if got.Available {
		t.Fatalf("Available = true, want false when ntm absent")
	}
	if len(got.Capabilities) != 0 {
		t.Fatalf("Capabilities = %v, want empty when ntm absent", got.Capabilities)
	}
	if len(got.MissingDeps) != 0 {
		t.Fatalf("MissingDeps = %v, want empty when ntm absent", got.MissingDeps)
	}
}

func TestNTMProbe_PresentButUnparsableIsHardError(t *testing.T) {
	// ntm present but emits garbage: a genuine contract violation, so a
	// hard error is correct (distinct from graceful absence).
	runner := &fakeRunner{
		out: []byte("not json at all"),
	}

	_, err := ProbeNTM(context.Background(), runner)
	if err == nil {
		t.Fatalf("ProbeNTM returned nil error on unparsable output, want error")
	}
}
