package eval

import (
	"os"
	"path/filepath"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
)

type Runtime struct{}

func (Runtime) RunSuite(options aoeval.RunOptions) (*aoeval.RunRecord, error) {
	return aoeval.RunSuite(options)
}
func (Runtime) RunBaselineAB(options aoeval.RunOptions) (aoeval.DeltaScorecard, *aoeval.RunRecord, *aoeval.RunRecord, error) {
	return aoeval.RunBaselineAB(options)
}
func (Runtime) WriteDeltaScorecard(card aoeval.DeltaScorecard, path string) error {
	return aoeval.WriteDeltaScorecard(card, path)
}
func (Runtime) RunContextAB(options aoeval.RunOptions, contextOptions aoeval.ContextABOptions) (aoeval.ContextDeltaScorecard, *aoeval.RunRecord, *aoeval.RunRecord, error) {
	return aoeval.RunContextAB(options, contextOptions)
}
func (Runtime) WriteContextDeltaScorecard(card aoeval.ContextDeltaScorecard, path string) error {
	return aoeval.WriteContextDeltaScorecard(card, path)
}
func (Runtime) LoadRun(path string) (*aoeval.RunRecord, error) { return aoeval.LoadRun(path) }
func (Runtime) CompareRuns(candidate, baseline *aoeval.RunRecord, options aoeval.CompareOptions) (*aoeval.RunRecord, error) {
	return aoeval.CompareRuns(candidate, baseline, options)
}
func (Runtime) WorkDir() (string, error)        { return os.Getwd() }
func (Runtime) Abs(path string) (string, error) { return filepath.Abs(path) }
func (Runtime) PromoteBaseline(run *aoeval.RunRecord, options aoeval.BaselineOptions) (*aoeval.RunRecord, error) {
	return aoeval.PromoteBaseline(run, options)
}
func (Runtime) AuditBaselinePolicy(options aoeval.BaselineAuditOptions) (*aoeval.BaselineAuditReport, error) {
	return aoeval.AuditBaselinePolicy(options)
}
func (Runtime) BuildScorecard(candidate, baseline *aoeval.RunRecord, options aoeval.ScorecardOptions) (*aoeval.Scorecard, error) {
	return aoeval.BuildScorecard(candidate, baseline, options)
}
func (Runtime) WriteScorecard(path string, card *aoeval.Scorecard) error {
	return aoeval.WriteScorecard(path, card)
}
func (Runtime) BuildCoverageReport(options aoeval.CoverageOptions) (*aoeval.CoverageReport, error) {
	return aoeval.BuildCoverageReport(options)
}
