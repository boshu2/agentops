package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/boshu2/agentops/cli/internal/scenario"
)

// ScenarioArm identifies one side of the with/without-gold knowledge A/B.
type ScenarioArm string

const (
	// ArmWithoutGold is the control arm: the agent runs without the gold pull.
	ArmWithoutGold ScenarioArm = "without_gold"
	// ArmWithGold is the treatment arm: the agent runs with the gold pull on.
	ArmWithGold ScenarioArm = "with_gold"
)

// ArmOutcome is what a ScenarioRunner produces for one arm. Output is the raw
// agent output the judge grades; TokenCost is the arm's measured token spend.
type ArmOutcome struct {
	Output    string
	TokenCost int
}

// ScenarioRunner executes one arm of the scenario. It is injected so tests use
// a deterministic fake (no live models, no hangs) and the live path (KF-C) uses
// a codex-exec-backed implementation. withGold toggles the decision-point gold
// pull for the treatment arm.
type ScenarioRunner interface {
	RunArm(ctx context.Context, sc scenario.Scenario, withGold bool) (ArmOutcome, error)
}

// VectorVerdict is a judge's machine-readable grade for one acceptance vector.
type VectorVerdict struct {
	Dimension string  `json:"dimension"`
	Pass      bool    `json:"pass"`
	Score     float64 `json:"score"`
}

// JudgeVerdict is the structured grade for one arm. AggregateScore is in [0,1].
// The judge MUST emit machine-readable output (this struct) so the gate stays
// deterministic over a stochastic (cross-family) judge.
type JudgeVerdict struct {
	Vectors        []VectorVerdict `json:"vectors"`
	AggregateScore float64         `json:"aggregate_score"`
}

// ScenarioJudge grades an arm's outcome against the scenario's acceptance
// vectors. Injected for the same reason as ScenarioRunner.
type ScenarioJudge interface {
	Judge(ctx context.Context, sc scenario.Scenario, arm ScenarioArm, outcome ArmOutcome) (JudgeVerdict, error)
}

// ScenarioArmResult records one arm's machine-readable verdict and token cost.
type ScenarioArmResult struct {
	Arm       ScenarioArm     `json:"arm"`
	Score     float64         `json:"score"`
	TokenCost int             `json:"token_cost"`
	Vectors   []VectorVerdict `json:"vectors"`
}

// ScenarioGate is the deterministic verdict computed over the (stochastic)
// judge output. Pass is false (fail-loud) when the treatment did not beat the
// control, missed the satisfaction threshold, or blew the token budget.
type ScenarioGate struct {
	Pass    bool     `json:"pass"`
	Reasons []string `json:"reasons,omitempty"`
}

// ScenarioDeltaScorecard mirrors ContextDeltaScorecard's shape for the
// with/without-gold scenario A/B. It is the persisted evidence artifact.
type ScenarioDeltaScorecard struct {
	SchemaVersion         int               `json:"schema_version"`
	ScenarioID            string            `json:"scenario_id"`
	ScenarioPath          string            `json:"scenario_path"`
	GeneratedAt           time.Time         `json:"generated_at"`
	Without               ScenarioArmResult `json:"without_gold"`
	With                  ScenarioArmResult `json:"with_gold"`
	AggregateDelta        float64           `json:"aggregate_delta"`
	SatisfactionThreshold float64           `json:"satisfaction_threshold"`
	TokenBudget           int               `json:"token_budget"`
	// CeilingViolation is true when the control (without-gold) arm already cleared
	// the satisfaction threshold — the task has no headroom, so the A/B is invalid
	// and no delta is emitted (LongMemEval-style validity certificate; age-707).
	CeilingViolation bool         `json:"ceiling_violation,omitempty"`
	Gate             ScenarioGate `json:"gate"`
}

// ScenarioABOptions configures RunScenarioAB. Timeout bounds each arm; the gate
// fails if the summed arm token cost exceeds TokenBudget (the ADR-0002 bound).
type ScenarioABOptions struct {
	Scenario     scenario.Scenario
	ScenarioPath string
	Runner       ScenarioRunner
	Judge        ScenarioJudge
	Timeout      time.Duration
	TokenBudget  int
	Now          func() time.Time
}

// defaultScenarioTokenBudget is the fail-loud spend ceiling for one A/B run.
const defaultScenarioTokenBudget = 200000

func (o ScenarioABOptions) withDefaults() ScenarioABOptions {
	if o.Now == nil {
		o.Now = func() time.Time { return time.Now().UTC() }
	}
	if o.Timeout <= 0 {
		o.Timeout = 5 * time.Minute
	}
	if o.TokenBudget <= 0 {
		o.TokenBudget = defaultScenarioTokenBudget
	}
	return o
}

// RunScenarioAB runs the control (without-gold) then treatment (with-gold) arm,
// grades each via the judge, and builds a scorecard with a deterministic gate.
// It does NOT exit the process — callers inspect Gate.Pass and map a failed
// gate to a non-zero exit (the CLI does this).
func RunScenarioAB(ctx context.Context, opts ScenarioABOptions) (ScenarioDeltaScorecard, error) {
	opts = opts.withDefaults()
	if opts.Runner == nil {
		return ScenarioDeltaScorecard{}, fmt.Errorf("scenario A/B requires a runner")
	}
	if opts.Judge == nil {
		return ScenarioDeltaScorecard{}, fmt.Errorf("scenario A/B requires a judge")
	}

	thr := opts.Scenario.SatisfactionThreshold
	// invalidCard builds a no-delta scorecard for a scenario that can't be validly
	// A/B'd. CeilingViolation is the consumer's signal to IGNORE the delta entirely.
	invalidCard := func(without ScenarioArmResult, reason string) ScenarioDeltaScorecard {
		return ScenarioDeltaScorecard{
			SchemaVersion:         1,
			ScenarioID:            opts.Scenario.ID,
			ScenarioPath:          opts.ScenarioPath,
			GeneratedAt:           opts.Now(),
			Without:               without,
			SatisfactionThreshold: thr,
			TokenBudget:           opts.TokenBudget,
			CeilingViolation:      true,
			Gate:                  ScenarioGate{Pass: false, Reasons: []string{reason}},
		}
	}

	// A threshold <= 0 is a degenerate bar: every score "satisfies" it, so it can
	// define neither task success nor headroom. Reject up-front (before spending an
	// arm) — NOT a sentinel-skip (the prior `thr > 0` guard let a zero-bar scenario
	// bypass the validity screen and emit a meaningless positive delta — refuter r1).
	if thr <= 0 {
		return invalidCard(ScenarioArmResult{}, fmt.Sprintf(
			"satisfaction_threshold %.4f <= 0: a zero/absent bar cannot define task success or headroom — scenario invalid for an A/B (set a threshold in (0,1]). No delta emitted.", thr)), nil
	}

	without, err := runAndJudgeArm(ctx, opts, ArmWithoutGold, false)
	if err != nil {
		return ScenarioDeltaScorecard{}, fmt.Errorf("without-gold arm: %w", err)
	}

	// Ceiling pre-screen (the validity certificate): run the control FIRST and, if
	// it already clears the satisfaction threshold, abort — the task has no headroom
	// for the corpus to help, so any delta is uninterpretable (this is exactly the
	// KF-4 −0.34 trap, where without-gold scored 0.89). Emit no delta; do NOT grade
	// the treatment arm. A valid A/B needs an out-of-distribution task the model
	// FAILS without the corpus.
	if without.Score >= thr {
		return invalidCard(without, fmt.Sprintf(
			"ceiling violation: without-gold floor %.4f >= satisfaction_threshold %.4f — the task has no headroom for the corpus to help, so any delta would be uninterpretable. Use an out-of-distribution task the model fails without the corpus. No delta emitted.",
			without.Score, thr)), nil
	}

	with, err := runAndJudgeArm(ctx, opts, ArmWithGold, true)
	if err != nil {
		return ScenarioDeltaScorecard{}, fmt.Errorf("with-gold arm: %w", err)
	}

	card := ScenarioDeltaScorecard{
		SchemaVersion:         1,
		ScenarioID:            opts.Scenario.ID,
		ScenarioPath:          opts.ScenarioPath,
		GeneratedAt:           opts.Now(),
		Without:               without,
		With:                  with,
		AggregateDelta:        roundDelta(with.Score - without.Score),
		SatisfactionThreshold: opts.Scenario.SatisfactionThreshold,
		TokenBudget:           opts.TokenBudget,
	}
	card.Gate = evaluateScenarioGate(card)
	return card, nil
}

func runAndJudgeArm(ctx context.Context, opts ScenarioABOptions, arm ScenarioArm, withGold bool) (ScenarioArmResult, error) {
	armCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		armCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	outcome, err := opts.Runner.RunArm(armCtx, opts.Scenario, withGold)
	if err != nil {
		return ScenarioArmResult{}, fmt.Errorf("run: %w", err)
	}
	verdict, err := opts.Judge.Judge(armCtx, opts.Scenario, arm, outcome)
	if err != nil {
		return ScenarioArmResult{}, fmt.Errorf("judge: %w", err)
	}
	return ScenarioArmResult{
		Arm:       arm,
		Score:     clamp01(verdict.AggregateScore),
		TokenCost: outcome.TokenCost,
		Vectors:   verdict.Vectors,
	}, nil
}

// evaluateScenarioGate is the DETERMINISTIC gate over the machine-readable judge
// output. It fails loudly (Pass=false) on no positive delta (the ADR-0002 spray
// returning), on the treatment missing the satisfaction threshold, or on the
// summed arm token cost exceeding the budget.
func evaluateScenarioGate(card ScenarioDeltaScorecard) ScenarioGate {
	var reasons []string
	if card.AggregateDelta <= 0 {
		reasons = append(reasons, fmt.Sprintf("aggregate_delta %.4f <= 0: with-gold did not beat without-gold (the ADR-0002 spray returning — fail loud here, not at S5)", card.AggregateDelta))
	}
	if card.With.Score < card.SatisfactionThreshold {
		reasons = append(reasons, fmt.Sprintf("with-gold score %.4f < satisfaction_threshold %.4f", card.With.Score, card.SatisfactionThreshold))
	}
	total := card.Without.TokenCost + card.With.TokenCost
	if card.TokenBudget > 0 && total > card.TokenBudget {
		reasons = append(reasons, fmt.Sprintf("token cost %d > budget %d", total, card.TokenBudget))
	}
	return ScenarioGate{Pass: len(reasons) == 0, Reasons: reasons}
}

func clamp01(f float64) float64 {
	switch {
	case f < 0:
		return 0
	case f > 1:
		return 1
	default:
		return f
	}
}

// WriteScenarioDeltaScorecard persists the scorecard at outputPath. Empty path
// is a no-op.
func WriteScenarioDeltaScorecard(card ScenarioDeltaScorecard, outputPath string) error {
	if outputPath == "" {
		return nil
	}
	data, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal scenario delta scorecard: %w", err)
	}
	if err := os.WriteFile(outputPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write scenario delta scorecard: %w", err)
	}
	return nil
}
