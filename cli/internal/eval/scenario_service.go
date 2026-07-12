package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/boshu2/agentops/cli/internal/goalsfitness"
	"github.com/boshu2/agentops/cli/internal/scenario"
	"github.com/boshu2/agentops/cli/internal/scenarioresults"
)

const ScenarioHoldoutDir = ".agents/holdout"

type ScenarioFileEntry struct {
	Name  string
	IsDir bool
}

type ScenarioRuntime interface {
	Create(scenario.CreateOptions) (*scenario.CreateResult, error)
	MkdirAll(string, uint32) error
	Exists(string) (bool, error)
	ReadDir(string) ([]ScenarioFileEntry, error)
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, uint32) error
	LoadDirectives(string, string) ([]goals.ParsedDirective, error)
	LoadGates(string) (map[string]string, error)
	ScenarioDirs() []string
	Measure(string, time.Duration) (string, string)
	CurrentIteration(string) int
	AppendResults(string, string, int, []scenarioresults.ScenarioResult, time.Time) error
	Now() time.Time
}

type ScenarioService struct{ Runtime ScenarioRuntime }
type ScenarioAddRequest struct {
	Goal, Narrative, ExpectedOutcome string
	Threshold                        float64
	Status, Source                   string
}
type ScenarioSummary struct {
	ID     string `json:"id"`
	Goal   string `json:"goal"`
	Status string `json:"status"`
	Date   string `json:"date"`
}
type ScenarioListResult struct {
	Scenarios        []ScenarioSummary `json:"scenarios"`
	MissingDirectory bool              `json:"-"`
}
type ScenarioValidationResult struct {
	Validated        int
	Errors           []string
	MissingDirectory bool
}
type ScenarioEvaluateRequest struct {
	ProjectRoot, GoalsPath, DirectiveID string
	All                                 bool
	Timeout                             time.Duration
	RunID                               string
}
type ScenarioEvaluation struct {
	ScenarioID  string   `json:"scenario_id"`
	DirectiveID string   `json:"directive_id"`
	Shape       string   `json:"shape,omitempty"`
	Verdict     string   `json:"verdict,omitempty"`
	Score       float64  `json:"score"`
	Threshold   float64  `json:"threshold"`
	Evidence    []string `json:"evidence,omitempty"`
	Recorded    bool     `json:"recorded"`
	Note        string   `json:"note,omitempty"`
}
type ScenarioEvaluateReport struct {
	RunID       string               `json:"run_id"`
	Iteration   int                  `json:"iteration"`
	Artifact    string               `json:"artifact"`
	Written     int                  `json:"written"`
	Evaluations []ScenarioEvaluation `json:"evaluations"`
}

func (service ScenarioService) Add(_ context.Context, request ScenarioAddRequest) (*scenario.CreateResult, error) {
	return service.Runtime.Create(scenario.CreateOptions{Goal: request.Goal, Narrative: request.Narrative, ExpectedOutcome: request.ExpectedOutcome, Threshold: request.Threshold, Status: request.Status, Source: request.Source, Dir: filepath.FromSlash(ScenarioHoldoutDir), Now: service.Runtime.Now})
}

func (service ScenarioService) Init(_ context.Context) (string, error) {
	dir := filepath.FromSlash(ScenarioHoldoutDir)
	if err := service.Runtime.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating holdout directory: %w", err)
	}
	readme := filepath.Join(dir, "README.md")
	exists, err := service.Runtime.Exists(readme)
	if err != nil {
		return "", err
	}
	if !exists {
		body := []byte("# Holdout Scenarios\n\nThis directory contains behavioral validation scenarios that implementing agents cannot see.\n")
		if err := service.Runtime.WriteFile(readme, body, 0o644); err != nil {
			return "", fmt.Errorf("writing README: %w", err)
		}
	}
	return dir, nil
}

func (service ScenarioService) List(_ context.Context, status string) (ScenarioListResult, error) {
	entries, err := service.Runtime.ReadDir(filepath.FromSlash(ScenarioHoldoutDir))
	if err != nil {
		if isMissingScenarioError(err) {
			return ScenarioListResult{MissingDirectory: true}, nil
		}
		return ScenarioListResult{}, fmt.Errorf("reading holdout directory: %w", err)
	}
	result := ScenarioListResult{Scenarios: []ScenarioSummary{}}
	for _, entry := range entries {
		if entry.IsDir || filepath.Ext(entry.Name) != ".json" {
			continue
		}
		raw, err := service.Runtime.ReadFile(filepath.Join(filepath.FromSlash(ScenarioHoldoutDir), entry.Name))
		if err != nil {
			continue
		}
		var summary ScenarioSummary
		if json.Unmarshal(raw, &summary) != nil || status != "" && summary.Status != status {
			continue
		}
		result.Scenarios = append(result.Scenarios, summary)
	}
	sort.Slice(result.Scenarios, func(i, j int) bool { return result.Scenarios[i].ID < result.Scenarios[j].ID })
	return result, nil
}

func (service ScenarioService) Validate(_ context.Context) (ScenarioValidationResult, error) {
	entries, err := service.Runtime.ReadDir(filepath.FromSlash(ScenarioHoldoutDir))
	if err != nil {
		if isMissingScenarioError(err) {
			return ScenarioValidationResult{MissingDirectory: true}, nil
		}
		return ScenarioValidationResult{}, fmt.Errorf("reading holdout directory: %w", err)
	}
	result := ScenarioValidationResult{}
	for _, entry := range entries {
		if entry.IsDir || filepath.Ext(entry.Name) != ".json" {
			continue
		}
		raw, err := service.Runtime.ReadFile(filepath.Join(filepath.FromSlash(ScenarioHoldoutDir), entry.Name))
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: read error: %v", entry.Name, err))
			continue
		}
		var object map[string]any
		if err := json.Unmarshal(raw, &object); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: invalid JSON: %v", entry.Name, err))
			continue
		}
		for _, field := range []string{"id", "version", "date", "goal", "narrative", "expected_outcome", "satisfaction_threshold", "status"} {
			if _, ok := object[field]; !ok {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: missing required field '%s'", entry.Name, field))
			}
		}
		if id, ok := object["id"].(string); ok && !scenario.IDPattern.MatchString(id) {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: id '%s' does not match pattern s-YYYY-MM-DD-NNN or auto-*", entry.Name, id))
		}
		if value, ok := object["status"].(string); ok && !scenario.ValidStatus(value) {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: invalid status '%s' (must be active, draft, or retired)", entry.Name, value))
		}
		if value, ok := object["source"].(string); ok && !scenario.ValidSource(value) {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: invalid source '%s' (must be human, agent, or prod-telemetry)", entry.Name, value))
		}
		if value, ok := object["satisfaction_threshold"].(float64); ok && (value < 0 || value > 1) {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: satisfaction_threshold %.2f out of range [0, 1]", entry.Name, value))
		}
		result.Validated++
	}
	return result, nil
}

func (service ScenarioService) Evaluate(_ context.Context, request ScenarioEvaluateRequest) (*ScenarioEvaluateReport, error) {
	if !request.All && request.DirectiveID == "" {
		return nil, fmt.Errorf("choose a scope: --all for every directive, or --directive <stable-id>")
	}
	directives, err := service.Runtime.LoadDirectives(request.GoalsPath, request.DirectiveID)
	if err != nil {
		return nil, fmt.Errorf("loading directives: %w", err)
	}
	if len(directives) == 0 {
		return nil, fmt.Errorf("no directive matches --directive %q", request.DirectiveID)
	}
	gates, err := service.Runtime.LoadGates(request.GoalsPath)
	if err != nil {
		return nil, fmt.Errorf("loading gates table: %w", err)
	}
	var evaluations []ScenarioEvaluation
	for _, directive := range directives {
		for _, id := range directive.Scenarios {
			evaluations = append(evaluations, service.evaluateOne(directive, id, gates, request.Timeout))
		}
	}
	report := &ScenarioEvaluateReport{RunID: request.RunID, Artifact: scenarioresults.ArtifactRelPath, Evaluations: evaluations}
	if report.Evaluations == nil {
		report.Evaluations = []ScenarioEvaluation{}
	}
	results := service.persistable(evaluations)
	report.Written = len(results)
	if len(results) > 0 {
		report.Iteration = service.Runtime.CurrentIteration(request.ProjectRoot)
		if err := service.Runtime.AppendResults(request.ProjectRoot, request.RunID, report.Iteration, results, service.Runtime.Now()); err != nil {
			return nil, fmt.Errorf("writing scenario results: %w", err)
		}
	}
	return report, nil
}

func (service ScenarioService) evaluateOne(directive goals.ParsedDirective, id string, gates map[string]string, timeout time.Duration) ScenarioEvaluation {
	evaluation := ScenarioEvaluation{ScenarioID: id, DirectiveID: directive.StableID}
	spec, err := service.loadSpec(id)
	if err != nil {
		evaluation.Note = fmt.Sprintf("unreadable scenario spec: %v", err)
		return evaluation
	}
	if spec == nil {
		evaluation.Note = "no scenario file found on the search path"
		return evaluation
	}
	if spec.Status == "retired" {
		evaluation.Note = "scenario is retired; not evaluated"
		return evaluation
	}
	if evaluation.DirectiveID == "" {
		evaluation.DirectiveID = spec.DirectiveID
	}
	if !scenarioresults.ValidDirectiveID(evaluation.DirectiveID) || !scenarioresults.ValidScenarioID(id) {
		evaluation.Note = "no valid stable directive/scenario ID; result would not validate"
		return evaluation
	}
	evaluation.Threshold = normalizeScenarioThreshold(spec.SatisfactionThreshold)
	if spec.SatisfactionThreshold < 0 || spec.SatisfactionThreshold > 1 {
		evaluation.Shape = "judgment"
		evaluation.Verdict = scenarioresults.VerdictSkip
		evaluation.Evidence = []string{fmt.Sprintf("invalid satisfaction_threshold %g (must be in (0,1]); spec must be fixed before this scenario can certify", spec.SatisfactionThreshold)}
		evaluation.Recorded = true
		return evaluation
	}
	checks := mechanicalScenarioChecks(spec.AcceptanceVectors)
	if len(checks) == 0 {
		evaluation.Shape = "judgment"
		evaluation.Verdict = scenarioresults.VerdictSkip
		evaluation.Evidence = []string{"attestation-needed: no mechanical acceptance-vector check; requires judge/human attestation"}
		evaluation.Recorded = true
		return evaluation
	}
	verdict, score, evidence := service.runChecks(checks, gates, evaluation.Threshold, timeout)
	evaluation.Score, evaluation.Evidence, evaluation.Recorded = score, evidence, true
	if len(checks) < len(spec.AcceptanceVectors) {
		evaluation.Shape = "judgment"
		evaluation.Verdict = scenarioresults.VerdictSkip
		evaluation.Evidence = append([]string{fmt.Sprintf("attestation-needed: %d of %d acceptance vectors carry no mechanical check; mechanical subset ran for evidence only", len(spec.AcceptanceVectors)-len(checks), len(spec.AcceptanceVectors))}, evidence...)
		return evaluation
	}
	evaluation.Shape, evaluation.Verdict = "gate", verdict
	return evaluation
}

func (service ScenarioService) loadSpec(id string) (*scenario.Scenario, error) {
	for _, dir := range service.Runtime.ScenarioDirs() {
		raw, err := service.Runtime.ReadFile(filepath.Join(dir, id+".json"))
		if err != nil {
			if isMissingScenarioError(err) {
				continue
			}
			return nil, err
		}
		var spec scenario.Scenario
		if err := json.Unmarshal(raw, &spec); err != nil {
			return nil, err
		}
		return &spec, nil
	}
	return nil, nil
}

func (service ScenarioService) runChecks(checks []scenario.AcceptanceVector, gates map[string]string, threshold float64, timeout time.Duration) (string, float64, []string) {
	passed, skipped := 0, 0
	evidence := make([]string, 0, len(checks))
	for _, vector := range checks {
		command := strings.TrimSpace(vector.Check)
		if id, ok := strings.CutPrefix(command, "gate:"); ok {
			resolved, found := gates[strings.TrimSpace(id)]
			if !found {
				skipped++
				evidence = append(evidence, fmt.Sprintf("%s: skip — unresolvable gate reference %q (not in the GOALS.md Gates table)", vector.Dimension, command))
				continue
			}
			command = resolved
		}
		outcome, detail := service.Runtime.Measure(command, timeout)
		switch outcome {
		case "pass":
			passed++
		case "skip":
			skipped++
		}
		text := fmt.Sprintf("`%s` -> %s", command, outcome)
		if outcome != "pass" && detail != "" {
			text += ": " + detail
		}
		evidence = append(evidence, fmt.Sprintf("%s: %s — %s", vector.Dimension, outcome, text))
	}
	score := float64(passed) / float64(len(checks))
	if skipped > 0 {
		return scenarioresults.VerdictSkip, score, evidence
	}
	if score >= threshold {
		return scenarioresults.VerdictPass, score, evidence
	}
	return scenarioresults.VerdictFail, score, evidence
}

func (service ScenarioService) persistable(evaluations []ScenarioEvaluation) []scenarioresults.ScenarioResult {
	judged := service.Runtime.Now().UTC().Format(time.RFC3339)
	var results []scenarioresults.ScenarioResult
	for _, evaluation := range evaluations {
		if evaluation.Recorded {
			results = append(results, scenarioresults.ScenarioResult{ScenarioID: evaluation.ScenarioID, DirectiveID: evaluation.DirectiveID, Score: evaluation.Score, Threshold: evaluation.Threshold, Verdict: evaluation.Verdict, JudgedAt: judged, Evidence: evaluation.Evidence})
		}
	}
	return results
}

func mechanicalScenarioChecks(vectors []scenario.AcceptanceVector) []scenario.AcceptanceVector {
	var checks []scenario.AcceptanceVector
	for _, vector := range vectors {
		if strings.TrimSpace(vector.Check) != "" {
			checks = append(checks, vector)
		}
	}
	return checks
}
func normalizeScenarioThreshold(value float64) float64 {
	if value <= 0 || value > 1 {
		return goalsfitness.DefaultScenarioThreshold
	}
	return value
}

func isMissingScenarioError(err error) bool { return errors.Is(err, os.ErrNotExist) }
