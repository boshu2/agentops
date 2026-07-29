package gate

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	gateapp "github.com/boshu2/agentops/cli/internal/gate"
	"github.com/boshu2/agentops/cli/internal/gates"
)

type fakeChecks struct {
	calls    int
	lastReq  gateapp.CheckRequest
	planResp *gates.Plan
	report   *gates.Report
}

func (fake *fakeChecks) Execute(_ context.Context, req gateapp.CheckRequest) (gateapp.CheckResult, error) {
	fake.calls++
	fake.lastReq = req
	if req.Plan {
		plan := fake.planResp
		if plan == nil {
			plan = &gates.Plan{}
		}
		return gateapp.CheckResult{Plan: plan}, nil
	}
	report := fake.report
	if report == nil {
		report = &gates.Report{}
	}
	return gateapp.CheckResult{Report: report}, nil
}

func runCheck(t *testing.T, fake *fakeChecks, host clicontract.HostOptions, args ...string) string {
	t.Helper()
	command := NewModule(UseCases{Check: fake}, host).Command()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetArgs(append([]string{"check"}, args...))
	if err := command.Execute(); err != nil {
		t.Fatalf("execute check %v: %v", args, err)
	}
	return out.String()
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
	// No --dry-run seam wired means the module never requests a plan.
	if fake.lastReq.Plan {
		t.Fatalf("check requested a plan without a DryRun seam: %+v", fake.lastReq)
	}
}

func TestGateCheckDryRunRequestsAndRendersPlan(t *testing.T) {
	fake := &fakeChecks{planResp: &gates.Plan{
		Mode:  gates.Fast,
		Scope: gates.ScopeHead,
		Selected: []gates.PlanCheck{{
			Name: "go.vet", Tier: "fast,full", Blocking: true,
			Reason:          "selected: changed file \"cli/main.go\" matched \"cli/**\"",
			WorkflowBacking: "bash scripts/check-go.sh", ArtifactPath: "scripts/check-go.sh", RepairHint: "bash scripts/check-go.sh",
		}},
	}}
	host := clicontract.HostOptions{DryRun: func() bool { return true }}

	out := runCheck(t, fake, host)
	if !fake.lastReq.Plan {
		t.Fatalf("dry-run did not request a plan: %+v", fake.lastReq)
	}
	for _, want := range []string{"dry-run — no checks executed", "would run:", "RUN   go.vet", "blocking"} {
		if !strings.Contains(out, want) {
			t.Errorf("human plan output missing %q\n---\n%s", want, out)
		}
	}
}

func TestGateCheckDryRunJSONRendersPlan(t *testing.T) {
	fake := &fakeChecks{planResp: &gates.Plan{
		Mode:     gates.Fast,
		Scope:    gates.ScopeHead,
		Selected: []gates.PlanCheck{{Name: "go.vet", Tier: "fast,full", Blocking: true, Reason: "selected"}},
	}}
	host := clicontract.HostOptions{DryRun: func() bool { return true }}

	out := runCheck(t, fake, host, "--json")
	if !fake.lastReq.Plan {
		t.Fatalf("dry-run --json did not request a plan: %+v", fake.lastReq)
	}
	for _, want := range []string{"\"dry_run\": true", "\"name\": \"go.vet\"", "\"selected_count\": 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("json plan output missing %q\n---\n%s", want, out)
		}
	}
}

func TestGateCheckWithoutDryRunRunsAndDoesNotRequestPlan(t *testing.T) {
	fake := &fakeChecks{}
	host := clicontract.HostOptions{DryRun: func() bool { return false }}
	_ = runCheck(t, fake, host)
	if fake.calls != 1 {
		t.Fatalf("check calls = %d, want 1", fake.calls)
	}
	if fake.lastReq.Plan {
		t.Fatalf("non-dry-run must not request a plan: %+v", fake.lastReq)
	}
}
