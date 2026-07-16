package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

type SuiteRuntime interface {
	Root() string
	ReadFile(string) ([]byte, error)
	RunStats([]string) ([]byte, error)
}

type SuiteService struct{ Runtime SuiteRuntime }
type SuiteVerdictRequest struct {
	SuiteID, Arms, InputsPath   string
	MDE                         float64
	BootstrapSamples, NRequired int
}
type SuiteVerdictResult struct {
	Raw    []byte
	Values map[string]any
}
type SuiteNRequiredRequest struct {
	BaselineRate, MDE, Alpha, Power float64
	Paired                          bool
}
type SuiteNRequiredResult struct {
	Raw       []byte
	NRequired int
}

func (service SuiteService) Verdict(_ context.Context, request SuiteVerdictRequest) (SuiteVerdictResult, error) {
	if request.InputsPath == "" {
		return SuiteVerdictResult{}, fmt.Errorf("eval suite verdict: --inputs is required")
	}
	suite, _ := service.loadSuite(request.SuiteID)
	arms := strings.Join(suiteArmIDs(suite, request.Arms), ",")
	if strings.TrimSpace(arms) == "" {
		return SuiteVerdictResult{}, fmt.Errorf("eval suite verdict: --arms required when suite has no varied_axis on disk")
	}
	rule, err := suiteDecisionRuleJSON(suite)
	if err != nil {
		return SuiteVerdictResult{}, fmt.Errorf("eval suite verdict: %w", err)
	}
	nRequired := request.NRequired
	if nRequired <= 0 {
		nRequired = service.derivedNRequired(suite)
	}
	args := []string{"-m", "_stats.cli", "verdict", "--suite-id", request.SuiteID, "--arms", arms, "--inputs", request.InputsPath, "--decision-rule", rule, "--n-required", fmt.Sprintf("%d", nRequired), "--B", fmt.Sprintf("%d", request.BootstrapSamples)}
	if request.MDE > 0 {
		args = append(args, "--mde", fmt.Sprintf("%g", request.MDE))
	}
	raw, err := service.Runtime.RunStats(args)
	if err != nil {
		return SuiteVerdictResult{}, err
	}
	values := map[string]any{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return SuiteVerdictResult{}, fmt.Errorf("eval suite verdict: parse stats output: %w", err)
	}
	return SuiteVerdictResult{Raw: raw, Values: values}, nil
}

func (service SuiteService) NRequired(_ context.Context, request SuiteNRequiredRequest) (SuiteNRequiredResult, error) {
	args := []string{"-m", "_stats.cli", "n-required", "--baseline-rate", fmt.Sprintf("%g", request.BaselineRate), "--mde", fmt.Sprintf("%g", request.MDE), "--alpha", fmt.Sprintf("%g", request.Alpha), "--power", fmt.Sprintf("%g", request.Power), "--paired", fmt.Sprintf("%v", request.Paired)}
	raw, err := service.Runtime.RunStats(args)
	if err != nil {
		return SuiteNRequiredResult{}, err
	}
	var parsed struct {
		NRequired int `json:"n_required"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return SuiteNRequiredResult{}, fmt.Errorf("eval suite n-required: parse: %w", err)
	}
	return SuiteNRequiredResult{Raw: raw, NRequired: parsed.NRequired}, nil
}

func (service SuiteService) loadSuite(id string) (*evalsubstrate.Suite, error) {
	path := id
	if !strings.HasSuffix(id, ".yaml") && !strings.HasSuffix(id, ".yml") {
		path = filepath.Join(service.Runtime.Root(), "suites", id, "suite.yaml")
	}
	raw, err := service.Runtime.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var suite evalsubstrate.Suite
	if err := yaml.Unmarshal(raw, &suite); err != nil {
		return nil, err
	}
	return &suite, nil
}

func (service SuiteService) derivedNRequired(suite *evalsubstrate.Suite) int {
	if suite == nil || suite.Stats.Power == nil {
		return 0
	}
	raw, err := service.Runtime.RunStats([]string{"-m", "_stats.cli", "n-required", "--baseline-rate", "0.5", "--mde", fmt.Sprintf("%g", suite.Stats.Power.MinimumDetectableEffect), "--alpha", fmt.Sprintf("%g", suite.Stats.Power.Alpha), "--power", "0.80", "--paired", "true"})
	if err != nil {
		return 0
	}
	var parsed struct {
		NRequired int `json:"n_required"`
	}
	if json.Unmarshal(raw, &parsed) != nil {
		return 0
	}
	return parsed.NRequired
}

func suiteArmIDs(suite *evalsubstrate.Suite, override string) []string {
	if override != "" {
		var ids []string
		for _, value := range strings.Split(override, ",") {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				ids = append(ids, trimmed)
			}
		}
		return ids
	}
	if suite != nil {
		return suite.VariedAxis.Values
	}
	return nil
}

func suiteDecisionRuleJSON(suite *evalsubstrate.Suite) (string, error) {
	rule := map[string]any{"kind": "ci_excludes_zero", "confidence": .95}
	if suite != nil {
		if suite.Stats.DecisionRule.Kind != "" {
			rule["kind"] = suite.Stats.DecisionRule.Kind
		}
		if suite.Stats.DecisionRule.Confidence > 0 {
			rule["confidence"] = suite.Stats.DecisionRule.Confidence
		}
		if suite.Stats.DecisionRule.MinDelta > 0 {
			rule["min_delta"] = suite.Stats.DecisionRule.MinDelta
		}
	}
	data, err := json.Marshal(rule)
	if err != nil {
		return "", fmt.Errorf("marshal decision rule: %w", err)
	}
	return string(data), nil
}
