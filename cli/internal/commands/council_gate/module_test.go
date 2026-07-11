package council_gate

import (
	"bytes"
	"context"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/councilgate"
)

type fakeUseCases struct{ result councilgate.Result }

func (fake fakeUseCases) Evaluate(context.Context, councilgate.Request) councilgate.Result {
	return fake.result
}

func TestModuleRendersPassAndDeclaresContract(t *testing.T) {
	module := NewModule(fakeUseCases{result: councilgate.Result{Outcome: councilgate.OutcomePass, Total: 2, Pass: 2, Contexts: 2, Families: 1}})
	command := module.Command()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"a", "b"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "COUNCIL PASS: 2/2 judges unanimous across 2 distinct contexts (1 model families)\n" {
		t.Fatalf("stdout = %q", got)
	}
	if err := clicontract.ValidateContract(module.Contract()); err != nil {
		t.Fatal(err)
	}
}

func TestModuleMapsDisagreementToEight(t *testing.T) {
	module := NewModule(fakeUseCases{result: councilgate.Result{Outcome: councilgate.OutcomeDisagreement, Total: 2, Pass: 1, Fail: 1}})
	command := module.Command()
	var stderr bytes.Buffer
	command.SetErr(&stderr)
	command.SetArgs([]string{"a", "b"})
	err := command.Execute()
	exit, ok := err.(interface{ ExitCode() int })
	if !ok || exit.ExitCode() != councilgate.ExitDisagree {
		t.Fatalf("error = %#v", err)
	}
	if got := stderr.String(); got != "DISAGREEMENT: 1 PASS / 1 FAIL - fail-closed; dispatch tie-break\n" {
		t.Fatalf("stderr = %q", got)
	}
}
