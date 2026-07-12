package eval

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/scenario"
)

type ScenarioABRuntime interface {
	LoadScenario(string) (*scenario.Scenario, error)
	Runner(scenario.Scenario) ScenarioRunner
	Judge(scenario.Scenario) ScenarioJudge
	WriteScenarioCard(ScenarioDeltaScorecard, string) error
	LoadScenarioCard(string) (ScenarioDeltaScorecard, error)
	WriteMoatResult(string, MoatClaimResult) error
}

type ScenarioABService struct{ Runtime ScenarioABRuntime }
type ScenarioABRequest struct {
	ScenarioPath, OutputPath string
	TokenBudget              int
	Timeout                  time.Duration
	ControlOnly              bool
}
type ScenarioABResult struct{ Card ScenarioDeltaScorecard }
type ScenarioMoatRequest struct {
	ScorecardPaths []string
	OutputPath     string
}

type ScenarioABGateError struct {
	ScenarioID string
	Reasons    []string
}

func (failure *ScenarioABGateError) Error() string {
	return fmt.Sprintf("scenario-ab gate failed for %s", failure.ScenarioID)
}

func (service ScenarioABService) Run(ctx context.Context, request ScenarioABRequest) (ScenarioABResult, error) {
	if strings.TrimSpace(request.ScenarioPath) == "" {
		return ScenarioABResult{}, fmt.Errorf("--scenario <path> is required")
	}
	spec, err := service.Runtime.LoadScenario(request.ScenarioPath)
	if err != nil {
		return ScenarioABResult{}, err
	}
	judge := service.Runtime.Judge(*spec)
	if strings.TrimSpace(spec.AnswerKey) != "" {
		judge = AnswerKeyJudge{}
	}
	card, err := RunScenarioAB(ctx, ScenarioABOptions{Scenario: *spec, ScenarioPath: request.ScenarioPath, Runner: service.Runtime.Runner(*spec), Judge: judge, Timeout: request.Timeout, TokenBudget: request.TokenBudget, ControlOnly: request.ControlOnly})
	if err != nil {
		return ScenarioABResult{}, err
	}
	if err := service.Runtime.WriteScenarioCard(card, request.OutputPath); err != nil {
		return ScenarioABResult{}, err
	}
	result := ScenarioABResult{Card: card}
	if !card.Gate.Pass {
		return result, &ScenarioABGateError{ScenarioID: card.ScenarioID, Reasons: append([]string(nil), card.Gate.Reasons...)}
	}
	return result, nil
}

func (service ScenarioABService) Moat(_ context.Context, request ScenarioMoatRequest) (MoatClaimResult, error) {
	if len(request.ScorecardPaths) == 0 {
		return MoatClaimResult{}, fmt.Errorf("at least one --scorecard path is required")
	}
	cards := make([]ScenarioDeltaScorecard, 0, len(request.ScorecardPaths))
	for _, path := range request.ScorecardPaths {
		card, err := service.Runtime.LoadScenarioCard(path)
		if err != nil {
			return MoatClaimResult{}, err
		}
		cards = append(cards, card)
	}
	result, err := AggregateMoatClaim(cards)
	if err != nil {
		return MoatClaimResult{}, err
	}
	if err := service.Runtime.WriteMoatResult(request.OutputPath, result); err != nil {
		return MoatClaimResult{}, err
	}
	if result.Verdict == MoatVerdictInconclusive {
		return result, fmt.Errorf("moat claim inconclusive — cannot publish positive or honest null")
	}
	return result, nil
}
