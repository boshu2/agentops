package eval

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
)

type coreUseCasesSpy struct {
	runRequest aoeval.CoreRunRequest
}

func (useCases *coreUseCasesSpy) Run(_ context.Context, request aoeval.CoreRunRequest) (aoeval.CoreRunResult, error) {
	useCases.runRequest = request
	return aoeval.CoreRunResult{Mode: aoeval.CoreRunSingle, Run: &aoeval.RunRecord{RunID: "run-1", Status: aoeval.StatusPass}}, nil
}
func (*coreUseCasesSpy) Compare(context.Context, aoeval.CoreCompareRequest) (aoeval.CoreCompareResult, error) {
	return aoeval.CoreCompareResult{}, nil
}
func (*coreUseCasesSpy) PromoteBaseline(context.Context, aoeval.CoreBaselineRequest) (*aoeval.RunRecord, error) {
	return &aoeval.RunRecord{}, nil
}
func (*coreUseCasesSpy) AuditBaseline(context.Context, aoeval.CoreBaselineAuditRequest) (*aoeval.BaselineAuditReport, error) {
	return &aoeval.BaselineAuditReport{}, nil
}
func (*coreUseCasesSpy) Scorecard(context.Context, aoeval.CoreScorecardRequest) (*aoeval.Scorecard, error) {
	return &aoeval.Scorecard{}, nil
}
func (*coreUseCasesSpy) Coverage(context.Context, aoeval.CoreCoverageRequest) (*aoeval.CoverageReport, error) {
	return &aoeval.CoverageReport{}, nil
}

func TestModuleRunParsesClosureLocalFlagsAndDelegates(t *testing.T) {
	useCases := &coreUseCasesSpy{}
	command := NewModule(useCases, HostOptions{}).Command()
	command.SetArgs([]string{"run", "suite.json", "--run-id", "run-1", "--runtime", "static", "--baseline-mode", "skill-on"})
	var output strings.Builder
	command.SetOut(&output)
	command.SetErr(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if useCases.runRequest.SuitePath != "suite.json" || useCases.runRequest.RunID != "run-1" || useCases.runRequest.Runtime != "static" {
		t.Fatalf("request = %#v", useCases.runRequest)
	}
	if !strings.Contains(output.String(), "Eval run-1: pass") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestModuleCommandsDoNotShareFlagState(t *testing.T) {
	first := NewModule(&coreUseCasesSpy{}, HostOptions{}).Command()
	second := NewModule(&coreUseCasesSpy{}, HostOptions{}).Command()
	firstRun, _, err := first.Find([]string{"run"})
	if err != nil {
		t.Fatal(err)
	}
	if err := firstRun.Flags().Set("run-id", "changed"); err != nil {
		t.Fatal(err)
	}
	secondRun, _, err := second.Find([]string{"run"})
	if err != nil {
		t.Fatal(err)
	}
	if got := secondRun.Flag("run-id").Value.String(); got != "" {
		t.Fatalf("second run-id = %q, want empty", got)
	}
}

func findChild(t *testing.T, command *cobra.Command, name string) *cobra.Command {
	t.Helper()
	child, _, err := command.Find([]string{name})
	if err != nil {
		t.Fatal(err)
	}
	return child
}
