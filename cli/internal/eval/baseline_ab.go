package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// BaselineMode names the run mode requested by `ao eval run --baseline-mode`.
type ABBaselineMode string

const (
	BaselineModeSkillOn  ABBaselineMode = "skill-on"
	BaselineModeSkillOff ABBaselineMode = "skill-off"
	BaselineModeBoth     ABBaselineMode = "both"
)

// AllBaselineModes returns the legal --baseline-mode values, used by
// cobra ValidArgs so invalid values are rejected (per pre-mortem Check 5).
func AllBaselineModes() []string {
	return []string{
		string(BaselineModeSkillOn),
		string(BaselineModeSkillOff),
		string(BaselineModeBoth),
	}
}

// IsValidBaselineMode reports whether s is a recognized --baseline-mode value.
func IsValidBaselineMode(s string) bool {
	switch ABBaselineMode(s) {
	case BaselineModeSkillOn, BaselineModeSkillOff, BaselineModeBoth:
		return true
	}
	return false
}

// AssertionDelta is the per-case (and per-assertion-id, when available) delta
// between the skill-on and skill-off runs. Delta is +1 when skill-on passes
// and skill-off fails, -1 when the inverse holds, and 0 when they agree.
type AssertionDelta struct {
	CaseID         string  `json:"case_id"`
	SkillOnStatus  Status  `json:"skill_on_status"`
	SkillOffStatus Status  `json:"skill_off_status"`
	SkillOnScore   float64 `json:"skill_on_score"`
	SkillOffScore  float64 `json:"skill_off_score"`
	Delta          int     `json:"delta"`
}

// DeltaScorecard is the artifact emitted by `ao eval run --baseline-mode=both`.
// SkillOnRunID and SkillOffRunID point at the per-leg run records on disk so
// the scorecard stays small and the original artifacts are preserved.
type DeltaScorecard struct {
	SchemaVersion  int              `json:"schema_version"`
	SuiteID        string           `json:"suite_id"`
	SuitePath      string           `json:"suite_path"`
	GeneratedAt    time.Time        `json:"generated_at"`
	SkillOnRunID   string           `json:"skill_on_run_id"`
	SkillOffRunID  string           `json:"skill_off_run_id"`
	SkillOnScore   float64          `json:"skill_on_aggregate"`
	SkillOffScore  float64          `json:"skill_off_aggregate"`
	AggregateDelta float64          `json:"aggregate_delta"`
	PerCase        []AssertionDelta `json:"per_case"`
}

// RunBaselineAB drives RunSuite twice over the same suite — once with skills
// loaded (OverrideDisableHooks=false) and once with hooks suppressed
// (OverrideDisableHooks=true) — then synthesises a DeltaScorecard. The two
// per-leg run records are also returned (and persisted by RunSuite if the
// caller supplies an OutputPath).
//
// Caller is responsible for distinguishing the two leg outputs if it sets
// OutputPath: the function does not mutate opts; instead each leg gets a
// derived OutputPath when opts.OutputPath != "".
func RunBaselineAB(opts RunOptions) (DeltaScorecard, *RunRecord, *RunRecord, error) {
	onOpts := opts
	onOpts.OverrideDisableHooks = false
	if opts.OutputPath != "" {
		onOpts.OutputPath = appendBaselineSuffix(opts.OutputPath, "skill-on")
	}
	if opts.RunID != "" {
		onOpts.RunID = opts.RunID + "-skill-on"
	}
	onRecord, err := RunSuite(onOpts)
	if err != nil {
		return DeltaScorecard{}, nil, nil, fmt.Errorf("skill-on run: %w", err)
	}

	offOpts := opts
	offOpts.OverrideDisableHooks = true
	if opts.OutputPath != "" {
		offOpts.OutputPath = appendBaselineSuffix(opts.OutputPath, "skill-off")
	}
	if opts.RunID != "" {
		offOpts.RunID = opts.RunID + "-skill-off"
	}
	offRecord, err := RunSuite(offOpts)
	if err != nil {
		return DeltaScorecard{}, nil, nil, fmt.Errorf("skill-off run: %w", err)
	}

	scorecard := computeDelta(opts.SuitePath, onRecord, offRecord)
	return scorecard, onRecord, offRecord, nil
}

// computeDelta builds a DeltaScorecard from two per-leg RunRecords.
// Exposed-internal so tests can drive it directly with hand-built records.
func computeDelta(suitePath string, on, off *RunRecord) DeltaScorecard {
	score := DeltaScorecard{
		SchemaVersion:  1,
		SuiteID:        on.Suite.ID,
		SuitePath:      suitePath,
		GeneratedAt:    time.Now().UTC(),
		SkillOnRunID:   on.RunID,
		SkillOffRunID:  off.RunID,
		SkillOnScore:   on.AggregateScore,
		SkillOffScore:  off.AggregateScore,
		AggregateDelta: on.AggregateScore - off.AggregateScore,
	}
	offByID := make(map[string]CaseResult, len(off.CaseResults))
	for _, c := range off.CaseResults {
		offByID[c.ID] = c
	}
	for _, onCase := range on.CaseResults {
		offCase, ok := offByID[onCase.ID]
		if !ok {
			// Off run missing this case — record on-side with a delta of 0
			// and a synthetic StatusInconclusive on the off side.
			score.PerCase = append(score.PerCase, AssertionDelta{
				CaseID:         onCase.ID,
				SkillOnStatus:  onCase.Status,
				SkillOnScore:   onCase.Score,
				SkillOffStatus: StatusInconclusive,
				Delta:          0,
			})
			continue
		}
		score.PerCase = append(score.PerCase, AssertionDelta{
			CaseID:         onCase.ID,
			SkillOnStatus:  onCase.Status,
			SkillOffStatus: offCase.Status,
			SkillOnScore:   onCase.Score,
			SkillOffScore:  offCase.Score,
			Delta:          deltaSign(onCase.Status, offCase.Status),
		})
	}
	return score
}

func deltaSign(on, off Status) int {
	switch {
	case on == StatusPass && off != StatusPass:
		return 1
	case off == StatusPass && on != StatusPass:
		return -1
	}
	return 0
}

func appendBaselineSuffix(path, suffix string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[:i] + "-" + suffix + path[i:]
		}
		if path[i] == '/' || path[i] == '\\' {
			break
		}
	}
	return path + "-" + suffix
}

// WriteDeltaScorecard persists the scorecard at outputPath. Empty path is a no-op.
func WriteDeltaScorecard(scorecard DeltaScorecard, outputPath string) error {
	if outputPath == "" {
		return nil
	}
	data, err := json.MarshalIndent(scorecard, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal delta scorecard: %w", err)
	}
	if err := os.WriteFile(outputPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write delta scorecard: %w", err)
	}
	return nil
}
