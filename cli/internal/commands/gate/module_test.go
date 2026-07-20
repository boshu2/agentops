package gate

import (
	"github.com/boshu2/agentops/cli/internal/clicontract"
	"context"
	"testing"

	gateapp "github.com/boshu2/agentops/cli/internal/gate"
	"github.com/boshu2/agentops/cli/internal/gates"
)

type fakeChecks struct{ calls int }

func (fake *fakeChecks) Execute(context.Context, gateapp.CheckRequest) (gateapp.CheckResult, error) {
	fake.calls++
	return gateapp.CheckResult{Report: &gates.Report{}}, nil
}

func TestGateExposesOnlyDeterministicCheck(t *testing.T) {
	fake := &fakeChecks{}
	command := NewModule(UseCases{Check: fake}, clicontract.HostOptions{}).Command()
	children := command.Commands()
	if len(children) != 1 || children[0].Name() != "check" {
		t.Fatalf("gate children = %v, want only check", children)
	}
	command.SetArgs([]string{"check"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 1 {
		t.Fatalf("check calls = %d, want 1", fake.calls)
	}
}
