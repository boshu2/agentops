package eval

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

type coreUseCasesSpy struct {
	runRequest aoeval.CoreRunRequest
}

type cleanupUseCasesSpy struct{ request aoeval.CleanupRequest }

type taskUseCasesSpy struct{ runRequest aoeval.TaskRunRequest }

type suiteUseCasesSpy struct{ verdictRequest aoeval.SuiteVerdictRequest }

type outcomesUseCasesSpy struct{ request aoeval.OutcomesIngestRequest }

func (*outcomesUseCasesSpy) Compile(context.Context, string) (evalsubstrate.Rubric, error) {
	return evalsubstrate.Rubric{}, nil
}
func (useCases *outcomesUseCasesSpy) Ingest(_ context.Context, request aoeval.OutcomesIngestRequest) (aoeval.OutcomesIngestResult, error) {
	useCases.request = request
	return aoeval.OutcomesIngestResult{Verdict: aoeval.OutcomesVerdict{Verdict: "PASS"}}, nil
}

func (useCases *suiteUseCasesSpy) Verdict(_ context.Context, request aoeval.SuiteVerdictRequest) (aoeval.SuiteVerdictResult, error) {
	useCases.verdictRequest = request
	return aoeval.SuiteVerdictResult{Values: map[string]any{"verdict": "improved"}}, nil
}
func (*suiteUseCasesSpy) NRequired(context.Context, aoeval.SuiteNRequiredRequest) (aoeval.SuiteNRequiredResult, error) {
	return aoeval.SuiteNRequiredResult{NRequired: 42}, nil
}

func (*taskUseCasesSpy) Add(context.Context, aoeval.TaskAddRequest) (aoeval.TaskAddResult, error) {
	return aoeval.TaskAddResult{}, nil
}
func (*taskUseCasesSpy) List(context.Context) (aoeval.TaskListResult, error) {
	return aoeval.TaskListResult{}, nil
}
func (*taskUseCasesSpy) Show(context.Context, string) (*evalsubstrate.Task, error) {
	return &evalsubstrate.Task{}, nil
}
func (useCases *taskUseCasesSpy) Run(_ context.Context, request aoeval.TaskRunRequest) (aoeval.TaskRunResult, error) {
	useCases.runRequest = request
	return aoeval.TaskRunResult{DryRun: true}, nil
}

func (useCases *cleanupUseCasesSpy) Execute(_ context.Context, request aoeval.CleanupRequest) (aoeval.CleanupReport, error) {
	useCases.request = request
	return aoeval.CleanupReport{TransitionsAborted: 1, Touched: []string{"run-1:pending->aborted"}}, nil
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
	command := NewModule(UseCases{Core: useCases}, HostOptions{}).Command()
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
	first := NewModule(UseCases{Core: &coreUseCasesSpy{}}, HostOptions{}).Command()
	second := NewModule(UseCases{Core: &coreUseCasesSpy{}}, HostOptions{}).Command()
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

func TestModuleCleanupDelegatesClosureLocalOptions(t *testing.T) {
	cleanup := &cleanupUseCasesSpy{}
	command := NewModule(UseCases{Core: &coreUseCasesSpy{}, Cleanup: cleanup}, HostOptions{Verbose: func(*cobra.Command) bool { return true }}).Command()
	command.SetArgs([]string{"cleanup", "--delete", "--tmp-files", "--tmp-age", "7", "--dry-run"})
	var output strings.Builder
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !cleanup.request.Delete || !cleanup.request.TmpFiles || !cleanup.request.DryRun || cleanup.request.TmpAgeSeconds != 7 {
		t.Fatalf("request = %#v", cleanup.request)
	}
	if !strings.Contains(output.String(), "transitions->aborted: 1") || !strings.Contains(output.String(), "Touched:") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestModuleTaskRunDelegatesClosureLocalFlags(t *testing.T) {
	task := &taskUseCasesSpy{}
	command := NewModule(UseCases{Core: &coreUseCasesSpy{}, Task: task}, HostOptions{}).Command()
	command.SetArgs([]string{"task", "run", "task-1", "--suite", "suite-1", "--seeds", "1,2,3", "--rig-id", "rig-1", "--dry-run"})
	var output strings.Builder
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if task.runRequest.TaskID != "task-1" || task.runRequest.SuiteRef != "suite-1" || task.runRequest.Seeds != "1,2,3" || !task.runRequest.DryRun {
		t.Fatalf("request = %#v", task.runRequest)
	}
	if !strings.Contains(output.String(), "Dry run: gates passed") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestModuleSuiteVerdictDelegatesFlags(t *testing.T) {
	suite := &suiteUseCasesSpy{}
	command := NewModule(UseCases{Core: &coreUseCasesSpy{}, Suite: suite}, HostOptions{}).Command()
	command.SetArgs([]string{"suite", "verdict", "suite-1", "--arms", "a,b", "--inputs", "in.json", "--B", "99"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if suite.verdictRequest.SuiteID != "suite-1" || suite.verdictRequest.Arms != "a,b" || suite.verdictRequest.BootstrapSamples != 99 {
		t.Fatalf("request = %#v", suite.verdictRequest)
	}
}

func TestModuleOutcomesIngestDelegatesSafetyFlags(t *testing.T) {
	outcomes := &outcomesUseCasesSpy{}
	command := NewModule(UseCases{Core: &coreUseCasesSpy{}, Outcomes: outcomes}, HostOptions{}).Command()
	command.SetArgs([]string{"outcomes", "ingest", "score.json", "--expect-judge-hash", "hash", "--burn-ledger", "burn.json", "--manifest-out", "runs"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if outcomes.request.ScorePath != "score.json" || outcomes.request.ExpectedJudgeHash != "hash" || outcomes.request.BurnLedgerPath != "burn.json" || outcomes.request.ManifestDir != "runs" {
		t.Fatalf("request = %#v", outcomes.request)
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
