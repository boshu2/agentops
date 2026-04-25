package eval

import (
	"path/filepath"
	"strings"
)

func PromoteBaseline(run *RunRecord, opts BaselineOptions) (*RunRecord, error) {
	if err := ValidateRun(run); err != nil {
		return nil, err
	}
	promoted, err := cloneRun(run)
	if err != nil {
		return nil, err
	}
	now := opts.Now
	if now == nil {
		now = defaultNow
	}
	promotedAt := now().UTC()
	outputPath := opts.OutputPath
	if outputPath == "" {
		workDir := opts.WorkDir
		if workDir == "" {
			workDir = "."
		}
		outputPath = filepath.Join(workDir, ".agents", "evals", "baselines", sanitizeBaselineFilename(run.Suite.ID+"-"+run.RunID)+".json")
	}
	promoted.Baseline = &BaselineRecord{
		Mode:              BaselineModePromote,
		BaselineRunID:     run.RunID,
		BaselinePath:      outputPath,
		PromotedFromRunID: run.RunID,
		PromotedAt:        &promotedAt,
		PromotedBy:        opts.PromotedBy,
		Rationale:         opts.Rationale,
	}
	promoted.Artifacts = append(promoted.Artifacts, Artifact{
		Path:    outputPath,
		Purpose: "promoted eval baseline",
		Kind:    "baseline",
	})
	if err := WriteRun(outputPath, promoted); err != nil {
		return nil, err
	}
	return promoted, nil
}

func sanitizeBaselineFilename(value string) string {
	value = sanitizeRunID(value)
	value = strings.Trim(value, ".")
	if value == "" {
		return "baseline"
	}
	return value
}
