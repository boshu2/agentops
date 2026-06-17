package eval

import (
	"errors"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAggregateMoatClaimRejectsIneligibleScorecard(t *testing.T) {
	plumbing := ScenarioDeltaScorecard{
		ScenarioID:            "s-plumbing-001",
		VerdictClass:          VerdictClassFactRecall,
		MoatEligible:          false,
		AggregateDelta:        1.0,
		SatisfactionThreshold: 0.8,
		Without:               ScenarioArmResult{Score: 0.1},
		With:                  ScenarioArmResult{Score: 1.0},
		Gate:                  ScenarioGate{Pass: true},
	}
	_, err := AggregateMoatClaim([]ScenarioDeltaScorecard{plumbing})
	if err == nil {
		t.Fatal("expected error for moat_eligible=false scorecard")
	}
	var ineligible ErrMoatIneligibleScorecard
	if !errors.As(err, &ineligible) {
		t.Fatalf("expected ErrMoatIneligibleScorecard, got %T: %v", err, err)
	}
	if ineligible.ScenarioID != "s-plumbing-001" {
		t.Errorf("ScenarioID = %q, want s-plumbing-001", ineligible.ScenarioID)
	}
}

func TestAggregateMoatClaimPositive(t *testing.T) {
	card := ScenarioDeltaScorecard{
		ScenarioID:            "s-applied-001",
		VerdictClass:          VerdictClassAppliedOOD,
		MoatEligible:          true,
		AggregateDelta:        0.57,
		SatisfactionThreshold: 0.8,
		Without:               ScenarioArmResult{Score: 0.35},
		With:                  ScenarioArmResult{Score: 0.92},
		Gate:                  ScenarioGate{Pass: true},
	}
	result, err := AggregateMoatClaim([]ScenarioDeltaScorecard{card})
	if err != nil {
		t.Fatalf("AggregateMoatClaim: %v", err)
	}
	if result.Verdict != MoatVerdictPositive {
		t.Errorf("Verdict = %q, want %q", result.Verdict, MoatVerdictPositive)
	}
}

func TestAggregateMoatClaimHonestNull(t *testing.T) {
	card := ScenarioDeltaScorecard{
		ScenarioID:            "s-applied-002",
		VerdictClass:          VerdictClassAppliedOOD,
		MoatEligible:          true,
		AggregateDelta:        -0.07,
		SatisfactionThreshold: 0.8,
		Without:               ScenarioArmResult{Score: 0.35},
		With:                  ScenarioArmResult{Score: 0.28},
		Gate:                  ScenarioGate{Pass: false, Reasons: []string{"aggregate_delta <= 0", "with-gold score < threshold"}},
	}
	result, err := AggregateMoatClaim([]ScenarioDeltaScorecard{card})
	if err != nil {
		t.Fatalf("AggregateMoatClaim: %v", err)
	}
	if result.Verdict != MoatVerdictHonestNull {
		t.Errorf("Verdict = %q, want %q", result.Verdict, MoatVerdictHonestNull)
	}
}

func TestAggregateMoatClaimExcludesCeilingViolation(t *testing.T) {
	card := ScenarioDeltaScorecard{
		ScenarioID:            "s-ceiling-001",
		VerdictClass:          VerdictClassAppliedOOD,
		MoatEligible:          true,
		CeilingViolation:      true,
		AggregateDelta:        0,
		SatisfactionThreshold: 0.8,
		Without:               ScenarioArmResult{Score: 0.95},
		With:                  ScenarioArmResult{Score: 0.95},
		Gate:                  ScenarioGate{Pass: false},
	}
	result, err := AggregateMoatClaim([]ScenarioDeltaScorecard{card})
	if err != nil {
		t.Fatalf("AggregateMoatClaim: %v", err)
	}
	if result.Verdict != MoatVerdictInconclusive {
		t.Errorf("Verdict = %q, want %q", result.Verdict, MoatVerdictInconclusive)
	}
	if len(result.ExcludedCeiling) != 1 {
		t.Errorf("ExcludedCeiling = %v, want [s-ceiling-001]", result.ExcludedCeiling)
	}
}

func TestLoadScenarioDeltaScorecardFromFixture(t *testing.T) {
	fixture := filepath.Join(repoRoot(t), "evals/scenarios/fixtures/scenario-ab-valid-redacted.scorecard.json")
	card, err := LoadScenarioDeltaScorecard(fixture)
	if err != nil {
		t.Fatalf("LoadScenarioDeltaScorecard: %v", err)
	}
	if !card.MoatEligible {
		t.Error("fixture scorecard should be moat_eligible")
	}
	result, err := AggregateMoatClaim([]ScenarioDeltaScorecard{card})
	if err != nil {
		t.Fatalf("AggregateMoatClaim: %v", err)
	}
	if result.Verdict != MoatVerdictPositive {
		t.Errorf("fixture Verdict = %q, want %q", result.Verdict, MoatVerdictPositive)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}
