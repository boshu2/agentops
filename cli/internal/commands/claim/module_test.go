package claim

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	claimapp "github.com/boshu2/agentops/cli/internal/claim"
	"github.com/boshu2/agentops/cli/internal/claimproof"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/ports"
)

type fakeUseCases struct {
	claimed string
	bound   claimapp.BindRequest
	list    []ports.EvidenceBinding
	report  claimproof.Report
}

func (fake *fakeUseCases) Claim(_ context.Context, id string, _ claimapp.Streams) error {
	fake.claimed = id
	return nil
}
func (fake *fakeUseCases) Bind(_ context.Context, request claimapp.BindRequest) error {
	fake.bound = request
	return nil
}
func (fake *fakeUseCases) List(context.Context) ([]ports.EvidenceBinding, error) {
	return fake.list, nil
}
func (fake *fakeUseCases) Check(context.Context, string, bool) (claimproof.Report, error) {
	return fake.report, nil
}

func TestModuleBuildsFreshTreeAndDelegates(t *testing.T) {
	fake := &fakeUseCases{}
	module := NewModule(fake, func() string { return "json" })
	first, second := module.Command(), module.Command()
	if first == second {
		t.Fatal("Command returned shared Cobra state")
	}
	command := module.Command()
	command.SetArgs([]string{"age-1"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if fake.claimed != "age-1" {
		t.Fatalf("claimed %q", fake.claimed)
	}
	if err := clicontract.ValidateContract(module.Contract()); err != nil {
		t.Fatal(err)
	}
}

func TestBindListAndCheckRenderThroughUseCases(t *testing.T) {
	fake := &fakeUseCases{
		list:   []ports.EvidenceBinding{{Claim: "A", Path: "p", Level: ports.EvidenceLevelPG2}},
		report: claimproof.Report{Summary: claimproof.Summary{Base: "base", Verdicts: map[string]int{}}},
	}
	module := NewModule(fake, func() string { return "json" })
	bind := module.Command()
	bind.SetArgs([]string{"bind", "--claim", "A", "--path", "p", "--level", "pg2"})
	if err := bind.Execute(); err != nil {
		t.Fatal(err)
	}
	if fake.bound.Level != "pg2" {
		t.Fatalf("bound = %+v", fake.bound)
	}

	list := module.Command()
	var listOut bytes.Buffer
	list.SetOut(&listOut)
	list.SetArgs([]string{"list"})
	if err := list.Execute(); err != nil {
		t.Fatal(err)
	}
	var binding ports.EvidenceBinding
	if err := json.Unmarshal(listOut.Bytes(), &binding); err != nil || binding.Claim != "A" {
		t.Fatalf("list JSON err=%v binding=%+v", err, binding)
	}

	check := module.Command()
	var checkOut bytes.Buffer
	check.SetOut(&checkOut)
	check.SetArgs([]string{"check", "--changed", "--base", "base"})
	if err := check.Execute(); err != nil {
		t.Fatal(err)
	}
	var report claimproof.Report
	if err := json.Unmarshal(checkOut.Bytes(), &report); err != nil || report.Summary.Base != "base" {
		t.Fatalf("check JSON err=%v report=%+v", err, report)
	}
}
