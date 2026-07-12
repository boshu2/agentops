package eval

import (
	"context"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/boshu2/agentops/cli/internal/scenario"
	"github.com/boshu2/agentops/cli/internal/scenarioresults"
)

type scenarioRuntimeSpy struct {
	created    scenario.CreateOptions
	files      map[string][]byte
	directives []goals.ParsedDirective
	dirs       []string
	appended   []scenarioresults.ScenarioResult
	entries    []ScenarioFileEntry
}

func (runtime *scenarioRuntimeSpy) Create(options scenario.CreateOptions) (*scenario.CreateResult, error) {
	runtime.created = options
	return &scenario.CreateResult{Scenario: scenario.Scenario{ID: "s-2026-01-01-001"}, Path: ".agents/holdout/s-2026-01-01-001.json"}, nil
}
func (*scenarioRuntimeSpy) MkdirAll(string, uint32) error { return nil }
func (*scenarioRuntimeSpy) Exists(string) (bool, error)   { return false, nil }
func (runtime *scenarioRuntimeSpy) ReadDir(string) ([]ScenarioFileEntry, error) {
	return runtime.entries, nil
}
func (runtime *scenarioRuntimeSpy) ReadFile(path string) ([]byte, error) {
	return runtime.files[path], nil
}
func (*scenarioRuntimeSpy) WriteFile(string, []byte, uint32) error { return nil }
func (runtime *scenarioRuntimeSpy) LoadDirectives(string, string) ([]goals.ParsedDirective, error) {
	return runtime.directives, nil
}
func (*scenarioRuntimeSpy) LoadGates(string) (map[string]string, error)    { return nil, nil }
func (runtime *scenarioRuntimeSpy) ScenarioDirs() []string                 { return runtime.dirs }
func (*scenarioRuntimeSpy) Measure(string, time.Duration) (string, string) { return "pass", "" }
func (*scenarioRuntimeSpy) CurrentIteration(string) int                    { return 1 }
func (runtime *scenarioRuntimeSpy) AppendResults(_ string, _ string, _ int, results []scenarioresults.ScenarioResult, _ time.Time) error {
	runtime.appended = results
	return nil
}
func (*scenarioRuntimeSpy) Now() time.Time { return time.Unix(1_000, 0).UTC() }

func TestScenarioServiceAddDelegatesValidatedDomainCreation(t *testing.T) {
	runtime := &scenarioRuntimeSpy{}
	result, err := (ScenarioService{Runtime: runtime}).Add(context.Background(), ScenarioAddRequest{Goal: "ship behavior", Threshold: .8, Status: "draft", Source: "human"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if runtime.created.Goal != "ship behavior" || runtime.created.Dir != ".agents/holdout" || result.Scenario.ID == "" {
		t.Fatalf("created=%#v result=%#v", runtime.created, result)
	}
}

func TestScenarioServiceEvaluateRecordsMechanicalPass(t *testing.T) {
	id := "s-2026-01-01-001"
	runtime := &scenarioRuntimeSpy{
		dirs:       []string{"spec"},
		directives: []goals.ParsedDirective{{StableID: "d-one", Scenarios: []string{id}}},
		files:      map[string][]byte{"spec/" + id + ".json": []byte(`{"id":"s-2026-01-01-001","status":"active","satisfaction_threshold":0.8,"acceptance_vectors":[{"dimension":"works","check":"true"}]}`)},
	}
	report, err := (ScenarioService{Runtime: runtime}).Evaluate(context.Background(), ScenarioEvaluateRequest{ProjectRoot: ".", GoalsPath: "GOALS.md", All: true, Timeout: time.Second, RunID: "run-1"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if report.Written != 1 || report.Evaluations[0].Verdict != scenarioresults.VerdictPass || len(runtime.appended) != 1 {
		t.Fatalf("report=%#v appended=%#v", report, runtime.appended)
	}
}

func TestScenarioServiceEvaluateRequiresExplicitScope(t *testing.T) {
	_, err := (ScenarioService{Runtime: &scenarioRuntimeSpy{}}).Evaluate(context.Background(), ScenarioEvaluateRequest{})
	if err == nil {
		t.Fatal("Evaluate accepted missing scope")
	}
}

func TestScenarioServiceEvaluateMixedVectorsNeverCertifiesPass(t *testing.T) {
	id := "s-2026-01-01-002"
	runtime := &scenarioRuntimeSpy{dirs: []string{"spec"}, directives: []goals.ParsedDirective{{StableID: "d-one", Scenarios: []string{id}}}, files: map[string][]byte{"spec/" + id + ".json": []byte(`{"id":"s-2026-01-01-002","status":"active","satisfaction_threshold":0.5,"acceptance_vectors":[{"dimension":"mechanical","check":"true"},{"dimension":"judgment"}]}`)}}
	report, err := (ScenarioService{Runtime: runtime}).Evaluate(context.Background(), ScenarioEvaluateRequest{All: true, Timeout: time.Second, RunID: "run"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if report.Evaluations[0].Verdict != scenarioresults.VerdictSkip || report.Evaluations[0].Shape != "judgment" {
		t.Fatalf("evaluation=%#v", report.Evaluations[0])
	}
}

func TestScenarioServiceValidateReportsMalformedScenario(t *testing.T) {
	runtime := &scenarioRuntimeSpy{entries: []ScenarioFileEntry{{Name: "bad.json"}}, files: map[string][]byte{".agents/holdout/bad.json": []byte(`{"id":"bad","status":"bogus","satisfaction_threshold":2}`)}}
	result, err := (ScenarioService{Runtime: runtime}).Validate(context.Background())
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result.Validated != 1 || len(result.Errors) < 2 {
		t.Fatalf("result=%#v", result)
	}
}
