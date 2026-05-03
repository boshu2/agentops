package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type ContextMode string

const (
	ContextModeNone ContextMode = "none"
	ContextModeAB   ContextMode = "ab"
)

type ContextABOptions struct {
	ContextOffAgentsDir string
	ContextOnAgentsDir  string
	ContextOffLabel     string
	ContextOnLabel      string
}

func AllContextModes() []string {
	return []string{
		string(ContextModeNone),
		string(ContextModeAB),
	}
}

func IsValidContextMode(s string) bool {
	switch ContextMode(s) {
	case ContextModeNone, ContextModeAB:
		return true
	}
	return false
}

// RunContextAB drives the same suite through the context variant axis. It
// intentionally does not set OverrideDisableHooks: context-off means no
// context packet input, not skill/hook suppression.
func RunContextAB(opts RunOptions, contextOpts ContextABOptions) (ContextDeltaScorecard, *RunRecord, *RunRecord, error) {
	baseOpts, err := withContextABBaseRunID(opts)
	if err != nil {
		return ContextDeltaScorecard{}, nil, nil, err
	}
	offLabel := defaultContextRootLabel(contextOpts.ContextOffLabel, ContextVariantOff)
	onLabel := defaultContextRootLabel(contextOpts.ContextOnLabel, ContextVariantOn)
	offOpts := contextVariantRunOptions(baseOpts, ContextVariantOff, "context-off", contextOpts.ContextOffAgentsDir)
	offRecord, err := RunSuite(offOpts)
	if err != nil {
		return ContextDeltaScorecard{}, nil, nil, fmt.Errorf("context-off run: %w", err)
	}

	onOpts := contextVariantRunOptions(baseOpts, ContextVariantOn, "context-on", contextOpts.ContextOnAgentsDir)
	onRecord, err := RunSuite(onOpts)
	if err != nil {
		return ContextDeltaScorecard{}, nil, nil, fmt.Errorf("context-on run: %w", err)
	}

	scorecard := computeContextDelta(baseOpts.SuitePath, offRecord, onRecord, offLabel, onLabel)
	return scorecard, offRecord, onRecord, nil
}

func withContextABBaseRunID(opts RunOptions) (RunOptions, error) {
	if opts.OverrideDisableHooks {
		return RunOptions{}, fmt.Errorf("context A/B cannot run with OverrideDisableHooks=true")
	}
	suite, _, err := LoadSuite(opts.SuitePath)
	if err != nil {
		return RunOptions{}, err
	}
	if suite.Environment.DisableHooks {
		return RunOptions{}, fmt.Errorf("context A/B requires hooks to remain enabled")
	}
	if strings.TrimSpace(opts.RunID) != "" {
		return opts, nil
	}
	now := opts.Now
	if now == nil {
		now = defaultNow
	}
	derived := opts
	derived.RunID = defaultRunID(suite.ID, now().UTC())
	return derived, nil
}

func contextVariantRunOptions(opts RunOptions, variant ContextVariant, suffix, agentsDir string) RunOptions {
	derived := opts
	derived.OverrideDisableHooks = opts.OverrideDisableHooks
	runEnv := map[string]string{
		"AO_CONTEXT_VARIANT": string(variant),
	}
	if agentsDir != "" {
		runEnv["AO_AGENTS_DIR"] = agentsDir
	}
	derived.Env = mergeStringMaps(opts.Env, runEnv)
	if opts.OutputPath != "" {
		derived.OutputPath = appendBaselineSuffix(opts.OutputPath, suffix)
	}
	if opts.RunID != "" {
		derived.RunID = opts.RunID + "-" + suffix
	}
	return derived
}

func defaultContextRootLabel(label string, variant ContextVariant) string {
	if label != "" {
		return label
	}
	return string(variant)
}

func computeContextDelta(suitePath string, off, on *RunRecord, offLabel, onLabel string) ContextDeltaScorecard {
	score := ContextDeltaScorecard{
		SchemaVersion: 1,
		SuiteID:       off.Suite.ID,
		SuitePath:     suitePath,
		GeneratedAt:   time.Now().UTC(),
		ContextOff: ContextVariantRun{
			Variant:          ContextVariantOff,
			ContextRootLabel: offLabel,
			RunID:            off.RunID,
			AggregateScore:   off.AggregateScore,
			Status:           off.Status,
		},
		ContextOn: ContextVariantRun{
			Variant:          ContextVariantOn,
			ContextRootLabel: onLabel,
			RunID:            on.RunID,
			AggregateScore:   on.AggregateScore,
			Status:           on.Status,
		},
		AggregateDelta: roundDelta(on.AggregateScore - off.AggregateScore),
	}
	onByID := make(map[string]CaseResult, len(on.CaseResults))
	for _, c := range on.CaseResults {
		onByID[c.ID] = c
	}
	for _, offCase := range off.CaseResults {
		onCase, ok := onByID[offCase.ID]
		if !ok {
			onCase = CaseResult{ID: offCase.ID, Status: StatusInconclusive}
		}
		score.PerCase = append(score.PerCase, ContextCaseDelta{
			CaseID: offCase.ID,
			ContextOff: ContextCaseVariantResult{
				Variant:          ContextVariantOff,
				ContextRootLabel: offLabel,
				RunID:            off.RunID,
				Status:           offCase.Status,
				Score:            offCase.Score,
			},
			ContextOn: ContextCaseVariantResult{
				Variant:          ContextVariantOn,
				ContextRootLabel: onLabel,
				RunID:            on.RunID,
				Status:           onCase.Status,
				Score:            onCase.Score,
			},
			ScoreDelta:       roundDelta(onCase.Score - offCase.Score),
			StatusDelta:      deltaSign(onCase.Status, offCase.Status),
			DecisionEvidence: decisionEvidenceForContextDelta(offCase, onCase),
			DegradedReason:   degradedReasonForContextDelta(offCase, onCase),
		})
	}
	return score
}

func decisionEvidenceForContextDelta(offCase, onCase CaseResult) []ContextEvidence {
	switch {
	case onCase.Score > offCase.Score:
		return []ContextEvidence{{
			Summary: fmt.Sprintf("context_on improved case score from %.4f to %.4f", offCase.Score, onCase.Score),
		}}
	case onCase.Status == StatusPass && offCase.Status != StatusPass:
		return []ContextEvidence{{
			Summary: fmt.Sprintf("context_on passed while context_off ended %s", offCase.Status),
		}}
	default:
		return nil
	}
}

func degradedReasonForContextDelta(offCase, onCase CaseResult) string {
	if onCase.Score < offCase.Score {
		return fmt.Sprintf("context_on score regressed from %.4f to %.4f", offCase.Score, onCase.Score)
	}
	if offCase.Status == StatusPass && onCase.Status != StatusPass {
		return fmt.Sprintf("context_on status regressed from pass to %s", onCase.Status)
	}
	return ""
}

// WriteContextDeltaScorecard persists the scorecard at outputPath. Empty path is a no-op.
func WriteContextDeltaScorecard(scorecard ContextDeltaScorecard, outputPath string) error {
	if outputPath == "" {
		return nil
	}
	data, err := json.MarshalIndent(scorecard, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal context delta scorecard: %w", err)
	}
	if err := os.WriteFile(outputPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write context delta scorecard: %w", err)
	}
	return nil
}
