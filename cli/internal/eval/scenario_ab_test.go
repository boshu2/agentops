package eval

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/scenario"
)

// fakeRunner is the injected ScenarioRunner for tests — no live models, no
// hangs. It returns a configured outcome per arm.
type fakeRunner struct {
	with    ArmOutcome
	without ArmOutcome
	err     error
}

func (f fakeRunner) RunArm(_ context.Context, _ scenario.Scenario, withGold bool) (ArmOutcome, error) {
	if f.err != nil {
		return ArmOutcome{}, f.err
	}
	if withGold {
		return f.with, nil
	}
	return f.without, nil
}

// fakeJudge is the injected ScenarioJudge for tests. It returns a configured
// machine-readable verdict per arm.
type fakeJudge struct {
	with    JudgeVerdict
	without JudgeVerdict
	err     error
}

func (f fakeJudge) Judge(_ context.Context, _ scenario.Scenario, arm ScenarioArm, _ ArmOutcome) (JudgeVerdict, error) {
	if f.err != nil {
		return JudgeVerdict{}, f.err
	}
	if arm == ArmWithGold {
		return f.with, nil
	}
	return f.without, nil
}

// loadFixtureScenario builds a scenario through the PRODUCTION writer
// (scenario.Create) and reads it back through the PRODUCTION reader
// (scenario.Load), per the fixture-fidelity rule — never a hand-built struct.
func loadFixtureScenario(t *testing.T, threshold float64) (scenario.Scenario, string) {
	t.Helper()
	dir := t.TempDir()
	res, err := scenario.Create(scenario.CreateOptions{
		Goal:      "reuse a prior architectural decision when designing a reward mechanism",
		Threshold: threshold,
		Status:    "active",
		Source:    "human",
		Dir:       dir,
		Now:       func() time.Time { return time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("Create fixture: %v", err)
	}
	sc, err := scenario.Load(res.Path)
	if err != nil {
		t.Fatalf("Load fixture: %v", err)
	}
	return *sc, res.Path
}

func fixedNow() time.Time { return time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC) }

// TestRunScenarioAB_CeilingViolation is the age-707 validity certificate
// (LongMemEval-style): when the WITHOUT-gold control arm already clears the
// satisfaction threshold, the task has no headroom — the corpus cannot help and
// any delta is uninterpretable (exactly the KF-4 −0.34 trap). The run must abort
// as a ceiling violation, emit NO delta, and NOT grade the with-gold arm.
func TestRunScenarioAB_CeilingViolation(t *testing.T) {
	sc, path := loadFixtureScenario(t, 0.8)
	runner := fakeRunner{
		without: ArmOutcome{Output: "control already solved it", TokenCost: 100},
		with:    ArmOutcome{Output: "treatment", TokenCost: 100},
	}
	judge := fakeJudge{
		without: JudgeVerdict{AggregateScore: 0.9}, // control clears the 0.8 threshold → ceiling
		with:    JudgeVerdict{AggregateScore: 0.5},
	}
	card, err := RunScenarioAB(context.Background(), ScenarioABOptions{
		Scenario: sc, ScenarioPath: path, Runner: runner, Judge: judge, Now: fixedNow,
	})
	if err != nil {
		t.Fatalf("RunScenarioAB: %v", err)
	}
	if !card.CeilingViolation {
		t.Error("expected CeilingViolation=true when control clears the threshold")
	}
	if card.Gate.Pass {
		t.Error("ceiling violation must fail the gate")
	}
	if len(card.Gate.Reasons) == 0 || !strings.Contains(strings.ToLower(card.Gate.Reasons[0]), "ceiling") {
		t.Errorf("gate reason should name the ceiling violation; got %v", card.Gate.Reasons)
	}
	if card.AggregateDelta != 0 {
		t.Errorf("no delta should be emitted on ceiling violation; got %v", card.AggregateDelta)
	}
	if card.With.Score != 0 {
		t.Errorf("with-gold arm must NOT be graded on ceiling violation; got With.Score %v", card.With.Score)
	}
}

// TestRunScenarioAB_ZeroThresholdInvalid: a satisfaction_threshold <= 0 is a
// degenerate bar (every score satisfies it), so it can define neither success nor
// headroom — the run must reject it up-front without grading any arm, and emit no
// delta (refuter r1: the prior `thr > 0` guard let a zero-bar scenario bypass the
// validity screen and emit a meaningless positive delta).
func TestRunScenarioAB_ZeroThresholdInvalid(t *testing.T) {
	sc, path := loadFixtureScenario(t, 0)
	card, err := RunScenarioAB(context.Background(), ScenarioABOptions{
		Scenario: sc, ScenarioPath: path,
		Runner: fakeRunner{without: ArmOutcome{TokenCost: 1}, with: ArmOutcome{TokenCost: 1}},
		Judge:  fakeJudge{without: JudgeVerdict{AggregateScore: 0.1}, with: JudgeVerdict{AggregateScore: 0.9}},
		Now:    fixedNow,
	})
	if err != nil {
		t.Fatalf("RunScenarioAB: %v", err)
	}
	if !card.CeilingViolation || card.Gate.Pass {
		t.Errorf("threshold<=0 must be flagged invalid and fail the gate; got violation=%v pass=%v", card.CeilingViolation, card.Gate.Pass)
	}
	if card.AggregateDelta != 0 {
		t.Errorf("no delta on zero-threshold scenario; got %v", card.AggregateDelta)
	}
	if card.Without.Score != 0 || card.With.Score != 0 {
		t.Errorf("no arm should be graded on threshold<=0; got without %v with %v", card.Without.Score, card.With.Score)
	}
	if len(card.Gate.Reasons) == 0 || !strings.Contains(card.Gate.Reasons[0], "threshold") {
		t.Errorf("reason should name the degenerate threshold; got %v", card.Gate.Reasons)
	}
}

// TestRunScenarioAB_CeilingViolation_JSONShape locks the persisted contract: the
// ceiling_violation flag and the gate reason serialize, so a consumer keys on the
// flag and never reads the (zeroed) delta in isolation (refuter r1 JSON concern).
func TestRunScenarioAB_CeilingViolation_JSONShape(t *testing.T) {
	sc, path := loadFixtureScenario(t, 0.8)
	card, err := RunScenarioAB(context.Background(), ScenarioABOptions{
		Scenario: sc, ScenarioPath: path,
		Runner: fakeRunner{without: ArmOutcome{TokenCost: 1}, with: ArmOutcome{TokenCost: 1}},
		Judge:  fakeJudge{without: JudgeVerdict{AggregateScore: 0.9}, with: JudgeVerdict{AggregateScore: 0.5}},
		Now:    fixedNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	if !strings.Contains(js, `"ceiling_violation":true`) {
		t.Errorf("serialized scorecard must carry ceiling_violation:true; got %s", js)
	}
	if !strings.Contains(js, "ceiling violation") {
		t.Errorf("serialized gate reason must name the ceiling violation; got %s", js)
	}
}

func TestRunScenarioAB_GateVerdicts(t *testing.T) {
	tests := []struct {
		name        string
		threshold   float64
		withScore   float64
		withoutScore float64
		withTokens  int
		withoutTokens int
		budget      int
		wantDelta   float64
		wantPass    bool
	}{
		{
			name: "positive delta above threshold under budget passes",
			threshold: 0.8, withScore: 0.9, withoutScore: 0.5,
			withTokens: 1000, withoutTokens: 1000, budget: 200000,
			wantDelta: 0.4, wantPass: true,
		},
		{
			// both arms BELOW threshold → exercises the delta<=0 gate path, not the
			// ceiling pre-screen (which fires only when the control CLEARS threshold;
			// that case is covered by TestRunScenarioAB_CeilingViolation).
			name: "zero delta below ceiling fails (the spray returning)",
			threshold: 0.8, withScore: 0.5, withoutScore: 0.5,
			withTokens: 1000, withoutTokens: 1000, budget: 200000,
			wantDelta: 0.0, wantPass: false,
		},
		{
			name: "positive delta but below threshold fails",
			threshold: 0.8, withScore: 0.7, withoutScore: 0.3,
			withTokens: 1000, withoutTokens: 1000, budget: 200000,
			wantDelta: 0.4, wantPass: false,
		},
		{
			name: "over budget fails even with good delta",
			threshold: 0.8, withScore: 0.95, withoutScore: 0.4,
			withTokens: 150000, withoutTokens: 150000, budget: 200000,
			wantDelta: 0.55, wantPass: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sc, path := loadFixtureScenario(t, tc.threshold)
			runner := fakeRunner{
				with:    ArmOutcome{Output: "treatment output", TokenCost: tc.withTokens},
				without: ArmOutcome{Output: "control output", TokenCost: tc.withoutTokens},
			}
			judge := fakeJudge{
				with:    JudgeVerdict{AggregateScore: tc.withScore},
				without: JudgeVerdict{AggregateScore: tc.withoutScore},
			}
			card, err := RunScenarioAB(context.Background(), ScenarioABOptions{
				Scenario: sc, ScenarioPath: path,
				Runner: runner, Judge: judge,
				TokenBudget: tc.budget, Now: fixedNow,
			})
			if err != nil {
				t.Fatalf("RunScenarioAB: %v", err)
			}
			if card.AggregateDelta != tc.wantDelta {
				t.Errorf("AggregateDelta = %.4f, want %.4f", card.AggregateDelta, tc.wantDelta)
			}
			if card.Gate.Pass != tc.wantPass {
				t.Errorf("Gate.Pass = %v, want %v (reasons: %v)", card.Gate.Pass, tc.wantPass, card.Gate.Reasons)
			}
			if !tc.wantPass && len(card.Gate.Reasons) == 0 {
				t.Error("failed gate must record at least one reason")
			}
			if card.With.TokenCost != tc.withTokens || card.Without.TokenCost != tc.withoutTokens {
				t.Errorf("token costs = with %d/without %d, want with %d/without %d",
					card.With.TokenCost, card.Without.TokenCost, tc.withTokens, tc.withoutTokens)
			}
			if card.ScenarioID != sc.ID {
				t.Errorf("ScenarioID = %q, want %q", card.ScenarioID, sc.ID)
			}
		})
	}
}

func TestRunScenarioAB_RequiresRunnerAndJudge(t *testing.T) {
	sc, path := loadFixtureScenario(t, 0.8)
	if _, err := RunScenarioAB(context.Background(), ScenarioABOptions{
		Scenario: sc, ScenarioPath: path, Judge: fakeJudge{}, Now: fixedNow,
	}); err == nil {
		t.Error("expected error when runner is nil")
	}
	if _, err := RunScenarioAB(context.Background(), ScenarioABOptions{
		Scenario: sc, ScenarioPath: path, Runner: fakeRunner{}, Now: fixedNow,
	}); err == nil {
		t.Error("expected error when judge is nil")
	}
}

func TestRunScenarioAB_ScoreClampedAndWritten(t *testing.T) {
	sc, path := loadFixtureScenario(t, 0.8)
	runner := fakeRunner{
		with:    ArmOutcome{TokenCost: 10},
		without: ArmOutcome{TokenCost: 10},
	}
	// Judge returns an out-of-range score; the adapter must clamp to [0,1].
	judge := fakeJudge{
		with:    JudgeVerdict{AggregateScore: 1.5, Vectors: []VectorVerdict{{Dimension: "d1", Pass: true, Score: 1.0}}},
		without: JudgeVerdict{AggregateScore: -0.2},
	}
	card, err := RunScenarioAB(context.Background(), ScenarioABOptions{
		Scenario: sc, ScenarioPath: path, Runner: runner, Judge: judge, Now: fixedNow,
	})
	if err != nil {
		t.Fatalf("RunScenarioAB: %v", err)
	}
	if card.With.Score != 1.0 {
		t.Errorf("with score clamp = %.4f, want 1.0", card.With.Score)
	}
	if card.Without.Score != 0.0 {
		t.Errorf("without score clamp = %.4f, want 0.0", card.Without.Score)
	}

	out := filepath.Join(t.TempDir(), "scorecard.json")
	if err := WriteScenarioDeltaScorecard(card, out); err != nil {
		t.Fatalf("write scorecard: %v", err)
	}
	reread, err := scenario.Load(path) // sanity: fixture path still valid
	if err != nil || reread.ID != sc.ID {
		t.Fatalf("fixture reread mismatch: %v", err)
	}
}
