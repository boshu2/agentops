package eval

import (
	"context"
	"testing"

	"github.com/boshu2/agentops/cli/internal/scenario"
)

type scenarioABRuntimeSpy struct {
	scenario scenario.Scenario
	runner   ScenarioRunner
	judge    ScenarioJudge
	wrote    bool
	card     ScenarioDeltaScorecard
}

func (runtime *scenarioABRuntimeSpy) LoadScenario(string) (*scenario.Scenario, error) {
	return &runtime.scenario, nil
}
func (runtime *scenarioABRuntimeSpy) Runner(scenario.Scenario) ScenarioRunner { return runtime.runner }
func (runtime *scenarioABRuntimeSpy) Judge(scenario.Scenario) ScenarioJudge   { return runtime.judge }
func (runtime *scenarioABRuntimeSpy) WriteScenarioCard(ScenarioDeltaScorecard, string) error {
	runtime.wrote = true
	return nil
}
func (*scenarioABRuntimeSpy) LoadScenarioCard(string) (ScenarioDeltaScorecard, error) {
	return ScenarioDeltaScorecard{}, nil
}

func TestScenarioABServiceMoatRejectsPlumbingCard(t *testing.T) {
	runtime := &scenarioABRuntimeSpy{}
	runtime.card = ScenarioDeltaScorecard{ScenarioID: "s-1", MoatEligible: false, VerdictClass: "fact-recall"}
	// Override through a tiny runtime wrapper so the service receives the card.
	_, err := (ScenarioABService{Runtime: scenarioABCardRuntime{scenarioABRuntimeSpy: runtime}}).Moat(context.Background(), ScenarioMoatRequest{ScorecardPaths: []string{"card.json"}})
	if err == nil {
		t.Fatal("Moat accepted moat_eligible=false card")
	}
}

type scenarioABCardRuntime struct{ *scenarioABRuntimeSpy }

func (runtime scenarioABCardRuntime) LoadScenarioCard(string) (ScenarioDeltaScorecard, error) {
	return runtime.card, nil
}
func (*scenarioABRuntimeSpy) WriteMoatResult(string, MoatClaimResult) error { return nil }

type fixedArmRunner struct{}

func (fixedArmRunner) RunArm(_ context.Context, _ scenario.Scenario, withGold bool) (ArmOutcome, error) {
	output := "miss"
	if withGold {
		output = "answer"
	}
	return ArmOutcome{Output: output, TokenCost: 1}, nil
}

func TestScenarioABServiceRequiresScenarioPath(t *testing.T) {
	_, err := (ScenarioABService{Runtime: &scenarioABRuntimeSpy{}}).Run(context.Background(), ScenarioABRequest{})
	if err == nil {
		t.Fatal("Run accepted missing scenario path")
	}
}

func TestScenarioABServiceUsesAnswerKeyJudgeAndPersistsCard(t *testing.T) {
	runtime := &scenarioABRuntimeSpy{scenario: scenario.Scenario{ID: "s-1", Goal: "g", Narrative: "n", AnswerKey: "answer", SatisfactionThreshold: .5}, runner: fixedArmRunner{}}
	result, err := (ScenarioABService{Runtime: runtime}).Run(context.Background(), ScenarioABRequest{ScenarioPath: "scenario.json", OutputPath: "out.json"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !runtime.wrote || !result.Card.Gate.Pass {
		t.Fatalf("wrote=%v result=%#v", runtime.wrote, result)
	}
}
