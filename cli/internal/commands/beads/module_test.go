package beads

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
	"github.com/spf13/cobra"
)

type recordingRunner struct {
	invocations []Invocation
}

func (runner *recordingRunner) Run(_ *cobra.Command, invocation Invocation) error {
	runner.invocations = append(runner.invocations, invocation)
	return nil
}

func TestModuleBuildsFreshCompleteTrees(t *testing.T) {
	runner := &recordingRunner{}
	module := NewModule(runner, nil, nil, nil)
	first := module.Command()
	second := module.Command()
	if first == second {
		t.Fatal("Command reused a Cobra tree")
	}
	want := []string{
		"audit", "cluster", "dir", "epic-status", "exec", "harvest", "lint",
		"resume", "scenarios", "stale-claims", "tracker", "verify", "verify-acceptance",
	}
	var got []string
	for _, command := range first.Commands() {
		got = append(got, command.Name())
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
}

func TestModuleParsesTypedInvocation(t *testing.T) {
	runner := &recordingRunner{}
	command := NewModule(runner, nil, nil, nil).Command()
	command.SetArgs([]string{"resume", "age-123", "--agent", "codex", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := Invocation{
		Operation: OperationResume,
		Args:      []string{"age-123"},
		Options: Options{
			Agent:  "codex",
			Ledger: "docs/provenance/ledger.jsonl",
			JSON:   true,
		},
	}
	if len(runner.invocations) != 1 || !reflect.DeepEqual(runner.invocations[0], want) {
		t.Fatalf("invocations = %#v, want %#v", runner.invocations, want)
	}
}

type fakeTrackerPorts struct {
	resolution beadsapp.TrackerResolution
	ledger     beadsapp.LedgerResolution
	snapshot   beadsapp.LedgerSnapshot
	override   bool
	executed   bool
	execErr    error
}

func (fake *fakeTrackerPorts) Resolve() (beadsapp.TrackerResolution, error) {
	return fake.resolution, nil
}
func (fake *fakeTrackerPorts) BRLedger() (beadsapp.LedgerResolution, error) {
	return fake.ledger, nil
}
func (fake *fakeTrackerPorts) BeadsDirOverride() bool { return fake.override }
func (fake *fakeTrackerPorts) InspectLedger(string) beadsapp.LedgerSnapshot {
	return fake.snapshot
}
func (fake *fakeTrackerPorts) Execute(_ context.Context, _ []string, streams beadsapp.ExecStreams) error {
	fake.executed = true
	_, _ = streams.Stdout.Write([]byte("executed\n"))
	return fake.execErr
}

func TestModuleOwnsDirectoryTrackerAndExecHandlers(t *testing.T) {
	ports := &fakeTrackerPorts{
		resolution: beadsapp.TrackerResolution{Tracker: beadsapp.TrackerBD, Binary: "/bin/bd", LedgerDir: "/repo/.beads", Source: "ledger"},
		snapshot:   beadsapp.LedgerSnapshot{Exists: true, Directory: true, Readable: true, Entries: []string{"beads.db"}},
	}

	t.Run("directory JSON", func(t *testing.T) {
		var output bytes.Buffer
		root := NewModule(nil, ports, ports, ports).Command()
		root.SetOut(&output)
		root.SetArgs([]string{"dir", "--require", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if got := output.String(); got != "{\"beads_dir\":\"/repo/.beads\",\"source\":\"ledger\"}\n" {
			t.Fatalf("output = %q", got)
		}
	})

	t.Run("tracker text", func(t *testing.T) {
		var output bytes.Buffer
		root := NewModule(nil, ports, ports, ports).Command()
		root.SetOut(&output)
		root.SetArgs([]string{"tracker"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if got := output.String(); got != "tracker     bd\nbinary      /bin/bd\nledger_dir  /repo/.beads\nsource      ledger\n" {
			t.Fatalf("output = %q", got)
		}
	})

	t.Run("exec help precedes resolution", func(t *testing.T) {
		ports.executed = false
		root := NewModule(nil, ports, ports, ports).Command()
		root.SetArgs([]string{"exec", "--help"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if ports.executed {
			t.Fatal("executor called for static help")
		}
	})

	t.Run("exec preserves typed exit", func(t *testing.T) {
		ports.executed = false
		ports.execErr = &beadsapp.ExitError{Code: 7}
		root := NewModule(nil, ports, ports, ports).Command()
		root.SetArgs([]string{"exec", "close", "age-x"})
		err := root.Execute()
		var exitError *beadsapp.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 7 {
			t.Fatalf("error = %v, want ExitError(7)", err)
		}
	})
}
