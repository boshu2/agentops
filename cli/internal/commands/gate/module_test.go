package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	gateapp "github.com/boshu2/agentops/cli/internal/gate"
	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/types"
)

type reviewUseCasesStub struct {
	pendingResult gateapp.PendingResult
	approveResult gateapp.ApproveResult
	approveInput  gateapp.ApproveInput
}

func (stub *reviewUseCasesStub) Pending(context.Context, gateapp.PendingRequest) (gateapp.PendingResult, error) {
	return stub.pendingResult, nil
}

func (stub *reviewUseCasesStub) Approve(_ context.Context, input gateapp.ApproveInput) (gateapp.ApproveResult, error) {
	stub.approveInput = input
	return stub.approveResult, nil
}

func (*reviewUseCasesStub) Reject(context.Context, gateapp.RejectInput) (gateapp.RejectResult, error) {
	return gateapp.RejectResult{}, nil
}

func (*reviewUseCasesStub) BulkApprove(context.Context, gateapp.BulkApproveInput) (gateapp.BulkApproveResult, error) {
	return gateapp.BulkApproveResult{}, nil
}

type checkUseCasesStub struct{ result gateapp.CheckResult }

func (stub checkUseCasesStub) Execute(context.Context, gateapp.CheckRequest) (gateapp.CheckResult, error) {
	return stub.result, nil
}

func TestModuleOwnsCompleteGateTreeAndFlags(t *testing.T) {
	command := NewModule(UseCases{}, HostOptions{}).Command()
	for _, path := range [][]string{{"pending"}, {"approve"}, {"reject"}, {"bulk-approve"}, {"run"}, {"check"}} {
		child, remaining, err := command.Find(path)
		if err != nil || child == command || len(remaining) != 0 {
			t.Fatalf("path=%v child=%v remaining=%v err=%v", path, child, remaining, err)
		}
	}
	flags := map[string][]string{
		"approve":      {"note"},
		"reject":       {"reason"},
		"bulk-approve": {"older-than", "tier"},
		"check":        {"fail-fast", "fast", "full", "github-annotations", "json", "require-workflow-parity", "scope", "workflow-coverage", "workflow-path"},
	}
	for name, names := range flags {
		child, _, _ := command.Find([]string{name})
		for _, flag := range names {
			if child.Flags().Lookup(flag) == nil {
				t.Errorf("gate %s missing --%s", name, flag)
			}
		}
	}
}

func TestPendingJSONPreservesFullPoolEntryWireShape(t *testing.T) {
	entry := gateapp.ReviewEntry{PoolEntry: types.PoolEntry{Candidate: types.Candidate{ID: "cand-1", Tier: types.TierBronze}, Status: types.PoolStatusPending}}
	command := NewModule(UseCases{
		Review: &reviewUseCasesStub{pendingResult: gateapp.PendingResult{Entries: []gateapp.ReviewEntry{entry}}},
	}, HostOptions{
		OutputFormat: func() string { return "json" },
	}).Command()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"pending"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(output.Bytes(), &rows); err != nil {
		t.Fatalf("json=%q err=%v", output.String(), err)
	}
	candidate, ok := rows[0]["candidate"].(map[string]any)
	if !ok || candidate["id"] != "cand-1" || rows[0]["status"] != string(types.PoolStatusPending) {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestPendingYAMLPreservesLegacyOperationalKeys(t *testing.T) {
	entry := gateapp.ReviewEntry{FilePath: "/tmp/cand-1.json", AgeString: "13h", ApproachingAutoPromote: true}
	command := NewModule(UseCases{
		Review: &reviewUseCasesStub{pendingResult: gateapp.PendingResult{Entries: []gateapp.ReviewEntry{entry}}},
	}, HostOptions{OutputFormat: func() string { return "yaml" }}).Command()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"pending"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	wire := output.String()
	for _, key := range []string{"filepath:", "age:", "agestring:", "approachingautopromote:"} {
		if !strings.Contains(wire, key) {
			t.Errorf("legacy YAML missing %q in:\n%s", key, wire)
		}
	}
	for _, drift := range []string{"file_path:", "age_string:", "approaching_auto_promote:"} {
		if strings.Contains(wire, drift) {
			t.Errorf("YAML contains drifted key %q in:\n%s", drift, wire)
		}
	}
}

func TestApproveDelegatesClosureLocalFlagsAndRenders(t *testing.T) {
	review := &reviewUseCasesStub{approveResult: gateapp.ApproveResult{CandidateID: "cand-1", Note: "good", Reviewer: "reviewer"}}
	command := NewModule(UseCases{Review: review}, HostOptions{}).Command()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"approve", "cand-1", "--note", "good"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if review.approveInput.CandidateID != "cand-1" || review.approveInput.Note != "good" || !strings.Contains(output.String(), "Approved: cand-1\nReviewer: reviewer") {
		t.Fatalf("input=%+v output=%q", review.approveInput, output.String())
	}
}

func TestCheckRendersJSONBeforeReturningTypedExit(t *testing.T) {
	report := &gates.Report{Mode: gates.Full, Scope: gates.ScopeHead, Results: []gates.CheckResult{{
		Check:   gates.Check{ID: "n.fail", Blocking: true},
		Verdict: ports.GateVerdict{Status: ports.GateStatusFail, Reason: "failed"},
	}}}
	command := NewModule(UseCases{Check: checkUseCasesStub{result: gateapp.CheckResult{Report: report, ExitCode: 1}}}, HostOptions{}).Command()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"check", "--full", "--json"})
	err := command.Execute()
	if err == nil {
		t.Fatal("expected typed exit")
	}
	exit, ok := err.(interface{ ExitCode() int })
	if !ok || exit.ExitCode() != 1 || !strings.Contains(output.String(), `"name": "n.fail"`) {
		t.Fatalf("err=%T %v output=%q", err, err, output.String())
	}
}
