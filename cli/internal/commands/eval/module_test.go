package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
	scenarioapp "github.com/boshu2/agentops/cli/internal/scenario"
)

// scenarioListSpy is a configurable ScenarioUseCases double whose List result
// the test controls, so both empty-state paths (missing directory / empty list)
// can be exercised.
type scenarioListSpy struct{ listResult aoeval.ScenarioListResult }

func (*scenarioListSpy) Add(context.Context, aoeval.ScenarioAddRequest) (*scenarioapp.CreateResult, error) {
	return &scenarioapp.CreateResult{}, nil
}
func (*scenarioListSpy) Init(context.Context) (string, error) { return ".agents/holdout", nil }
func (spy *scenarioListSpy) List(context.Context, string) (aoeval.ScenarioListResult, error) {
	return spy.listResult, nil
}
func (*scenarioListSpy) Validate(context.Context) (aoeval.ScenarioValidationResult, error) {
	return aoeval.ScenarioValidationResult{}, nil
}
func (*scenarioListSpy) Evaluate(context.Context, aoeval.ScenarioEvaluateRequest) (*aoeval.ScenarioEvaluateReport, error) {
	return &aoeval.ScenarioEvaluateReport{}, nil
}

// TestAnnotateSuiteParseError asserts that each of the three suite-load failure
// modes surfaced by LoadSuite gets the schema + example citation, and that an
// unrelated runtime error is passed through untouched.
func TestAnnotateSuiteParseError(t *testing.T) {
	cases := []struct {
		name     string
		in       error
		wantCite bool
	}{
		{name: "missing file", in: fmt.Errorf("read eval suite: open missing.json: no such file or directory"), wantCite: true},
		{name: "malformed json", in: fmt.Errorf("decode eval suite: invalid character '}' looking for beginning of value"), wantCite: true},
		{name: "schema invalid", in: fmt.Errorf("eval suite validation failed: schema_version must be 1"), wantCite: true},
		{name: "unrelated error", in: fmt.Errorf("runtime static: check failed"), wantCite: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := annotateSuiteParseError(tc.in)
			hasCite := strings.Contains(got.Error(), "schemas/eval-suite.v1.schema.json") && strings.Contains(got.Error(), "evals/agentops-core")
			if hasCite != tc.wantCite {
				t.Fatalf("annotateSuiteParseError(%q) = %q; wantCite=%v", tc.in, got, tc.wantCite)
			}
			if !errors.Is(got, tc.in) {
				t.Errorf("wrapped error must preserve the original: errors.Is == false")
			}
		})
	}
}

// TestModuleScenarioListJSONEmptyStates asserts that `eval scenario list` emits
// exactly one JSON document `[]` on stdout in BOTH empty states (missing holdout
// directory and empty scenario list), routes the human hint to stderr, and names
// the REAL init command. RED before the fix: empty-state prose is printed to
// stdout (breaking jq) and the missing-dir hint names 'ao scenario init', which
// does not exist.
func TestModuleScenarioListJSONEmptyStates(t *testing.T) {
	cases := []struct {
		name       string
		result     aoeval.ScenarioListResult
		wantStderr string
	}{
		{name: "missing directory", result: aoeval.ScenarioListResult{MissingDirectory: true}, wantStderr: "ao eval scenario init"},
		{name: "empty list", result: aoeval.ScenarioListResult{}, wantStderr: "No scenarios found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spy := &scenarioListSpy{listResult: tc.result}
			command := NewModule(UseCases{Core: &coreUseCasesSpy{}, Scenario: spy}, HostOptions{}).Command()
			command.SetArgs([]string{"scenario", "list"})
			var stdout, stderr bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&stderr)
			if err := command.Execute(); err != nil {
				t.Fatalf("Execute: %v", err)
			}

			dec := json.NewDecoder(&stdout)
			var decoded []aoeval.ScenarioSummary
			if err := dec.Decode(&decoded); err != nil {
				t.Fatalf("stdout is not one JSON document: %v (raw stdout: %q)", err, stdout.String())
			}
			if len(decoded) != 0 {
				t.Errorf("expected [] (0 scenarios), got %d", len(decoded))
			}
			if dec.More() {
				t.Error("stdout contains more than one JSON document")
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tc.wantStderr)
			}
			if strings.Contains(stderr.String(), "ao scenario init") {
				t.Errorf("stderr names the nonexistent 'ao scenario init': %q", stderr.String())
			}
		})
	}
}

type coreUseCasesSpy struct {
	runRequest aoeval.CoreRunRequest
}

type cleanupUseCasesSpy struct{ request aoeval.CleanupRequest }

type taskUseCasesSpy struct{ runRequest aoeval.TaskRunRequest }

type suiteUseCasesSpy struct{ verdictRequest aoeval.SuiteVerdictRequest }

type outcomesUseCasesSpy struct{ request aoeval.OutcomesIngestRequest }

type scenarioUseCasesSpy struct{ addRequest aoeval.ScenarioAddRequest }

type scenarioABUseCasesSpy struct{ request aoeval.ScenarioABRequest }

type aliasUseCasesSpy struct{ request aoeval.SessionOutcomeRequest }

type benchUseCasesSpy struct{ request aoeval.BenchRequest }

func (useCases *benchUseCasesSpy) Bench(_ context.Context, request aoeval.BenchRequest) (aoeval.AliasOutput, error) {
	useCases.request = request
	return aoeval.AliasOutput{Stdout: "bench\n"}, nil
}

func (useCases *aliasUseCasesSpy) SessionOutcome(_ context.Context, request aoeval.SessionOutcomeRequest) (aoeval.SessionOutcomeResult, error) {
	useCases.request = request
	return aoeval.SessionOutcomeResult{SessionID: "s", Reward: .5, Signals: []aoeval.SessionSignal{}}, nil
}
func (*aliasUseCasesSpy) Chaos(context.Context) (aoeval.AliasOutput, error) {
	return aoeval.AliasOutput{Stdout: "PASS smoke\n"}, nil
}

func (useCases *scenarioABUseCasesSpy) Run(_ context.Context, request aoeval.ScenarioABRequest) (aoeval.ScenarioABResult, error) {
	useCases.request = request
	return aoeval.ScenarioABResult{Card: aoeval.ScenarioDeltaScorecard{ScenarioID: "s-1", Gate: aoeval.ScenarioGate{Pass: true}}}, nil
}
func (*scenarioABUseCasesSpy) Moat(context.Context, aoeval.ScenarioMoatRequest) (aoeval.MoatClaimResult, error) {
	return aoeval.MoatClaimResult{}, nil
}

func (useCases *scenarioUseCasesSpy) Add(_ context.Context, request aoeval.ScenarioAddRequest) (*scenarioapp.CreateResult, error) {
	useCases.addRequest = request
	return &scenarioapp.CreateResult{Scenario: scenarioapp.Scenario{ID: "s-2026-01-01-001"}, Path: "path"}, nil
}
func (*scenarioUseCasesSpy) Init(context.Context) (string, error) { return ".agents/holdout", nil }
func (*scenarioUseCasesSpy) List(context.Context, string) (aoeval.ScenarioListResult, error) {
	return aoeval.ScenarioListResult{}, nil
}
func (*scenarioUseCasesSpy) Validate(context.Context) (aoeval.ScenarioValidationResult, error) {
	return aoeval.ScenarioValidationResult{}, nil
}
func (*scenarioUseCasesSpy) Evaluate(context.Context, aoeval.ScenarioEvaluateRequest) (*aoeval.ScenarioEvaluateReport, error) {
	return &aoeval.ScenarioEvaluateReport{}, nil
}

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

func TestModuleScenarioAddDelegatesClosureLocalFlags(t *testing.T) {
	useCases := &scenarioUseCasesSpy{}
	command := NewModule(UseCases{Core: &coreUseCasesSpy{}, Scenario: useCases}, HostOptions{}).Command()
	command.SetArgs([]string{"scenario", "add", "goal", "--threshold", "0.7", "--status", "active"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if useCases.addRequest.Goal != "goal" || useCases.addRequest.Threshold != .7 || useCases.addRequest.Status != "active" {
		t.Fatalf("request=%#v", useCases.addRequest)
	}
}

func TestModuleScenarioABDelegatesFlags(t *testing.T) {
	useCases := &scenarioABUseCasesSpy{}
	command := NewModule(UseCases{Core: &coreUseCasesSpy{}, ScenarioAB: useCases}, HostOptions{}).Command()
	command.SetArgs([]string{"scenario-ab", "--scenario", "s.json", "--token-budget", "9", "--control-only"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if useCases.request.ScenarioPath != "s.json" || useCases.request.TokenBudget != 9 || !useCases.request.ControlOnly {
		t.Fatalf("request=%#v", useCases.request)
	}
}

func TestModuleSessionOutcomeDelegatesFlags(t *testing.T) {
	useCases := &aliasUseCasesSpy{}
	command := NewModule(UseCases{Core: &coreUseCasesSpy{}, Aliases: useCases}, HostOptions{}).Command()
	command.SetArgs([]string{"session-outcome", "transcript.jsonl", "--session", "s-1"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if useCases.request.TranscriptPath != "transcript.jsonl" || useCases.request.SessionID != "s-1" {
		t.Fatalf("request=%#v", useCases.request)
	}
}

func TestModuleChaosRendersAdapterStreams(t *testing.T) {
	command := NewModule(UseCases{Core: &coreUseCasesSpy{}, Aliases: &aliasUseCasesSpy{}}, HostOptions{}).Command()
	command.SetArgs([]string{"chaos"})
	var stdout strings.Builder
	command.SetOut(&stdout)
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "PASS smoke") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestModuleBenchDelegatesClosureLocalFlags(t *testing.T) {
	useCases := &benchUseCasesSpy{}
	command := NewModule(UseCases{Core: &coreUseCasesSpy{}, Bench: useCases}, HostOptions{}).Command()
	command.SetArgs([]string{"bench", "--corpus", "fixture", "--k", "7", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if useCases.request.Corpus != "fixture" || useCases.request.K != 7 || !useCases.request.JSON || !useCases.request.KChanged {
		t.Fatalf("request=%#v", useCases.request)
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
