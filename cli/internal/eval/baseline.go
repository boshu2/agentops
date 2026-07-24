package eval

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
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
		filename, err := baselineFilename(run.Suite.ID, run.RunID)
		if err != nil {
			return nil, fmt.Errorf("promote baseline path: %w", err)
		}
		outputPath = filepath.Join(workDir, ".agents", "evals", "baselines", filename+".json")
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

func baselineFilename(suiteID, runID string) (string, error) {
	suite, err := evalsubstrate.ParseIdentifier(evalsubstrate.IdentifierSuite, suiteID)
	if err != nil {
		return "", err
	}
	run, err := evalsubstrate.ParseIdentifier(evalsubstrate.IdentifierRun, runID)
	if err != nil {
		return "", err
	}
	filename := suite.StorageName() + "-" + run.StorageName()
	if len(filename) <= 200 {
		return filename, nil
	}
	digest := sha256.Sum256([]byte(filename))
	return filename[:160] + "-" + hex.EncodeToString(digest[:8]), nil
}
