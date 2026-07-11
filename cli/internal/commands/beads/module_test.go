package beads

import (
	"reflect"
	"testing"

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
	module := NewModule(runner)
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
	command := NewModule(runner).Command()
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
