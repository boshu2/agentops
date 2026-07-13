package eval

import (
	"context"
	"strings"
	"testing"
)

type coreRuntimeSpy struct {
	runCalls int
}

func (runtime *coreRuntimeSpy) RunSuite(options RunOptions) (*RunRecord, error) {
	runtime.runCalls++
	return &RunRecord{RunID: options.RunID, Status: StatusPass}, nil
}
func (*coreRuntimeSpy) RunBaselineAB(RunOptions) (DeltaScorecard, *RunRecord, *RunRecord, error) {
	return DeltaScorecard{}, &RunRecord{}, &RunRecord{}, nil
}
func (*coreRuntimeSpy) WriteDeltaScorecard(DeltaScorecard, string) error { return nil }
func (*coreRuntimeSpy) RunContextAB(RunOptions, ContextABOptions) (ContextDeltaScorecard, *RunRecord, *RunRecord, error) {
	return ContextDeltaScorecard{}, &RunRecord{}, &RunRecord{}, nil
}
func (*coreRuntimeSpy) WriteContextDeltaScorecard(ContextDeltaScorecard, string) error { return nil }
func (*coreRuntimeSpy) LoadRun(string) (*RunRecord, error)                             { return &RunRecord{}, nil }
func (*coreRuntimeSpy) CompareRuns(*RunRecord, *RunRecord, CompareOptions) (*RunRecord, error) {
	return &RunRecord{}, nil
}
func (*coreRuntimeSpy) WorkDir() (string, error)        { return "/work", nil }
func (*coreRuntimeSpy) Abs(path string) (string, error) { return "/abs/" + path, nil }
func (*coreRuntimeSpy) PromoteBaseline(*RunRecord, BaselineOptions) (*RunRecord, error) {
	return &RunRecord{}, nil
}
func (*coreRuntimeSpy) AuditBaselinePolicy(BaselineAuditOptions) (*BaselineAuditReport, error) {
	return &BaselineAuditReport{}, nil
}
func (*coreRuntimeSpy) BuildScorecard(*RunRecord, *RunRecord, ScorecardOptions) (*Scorecard, error) {
	return &Scorecard{}, nil
}
func (*coreRuntimeSpy) WriteScorecard(string, *Scorecard) error { return nil }
func (*coreRuntimeSpy) BuildCoverageReport(CoverageOptions) (*CoverageReport, error) {
	return &CoverageReport{}, nil
}

func TestCoreServiceRunRejectsUnknownRuntimeBeforeEffects(t *testing.T) {
	runtime := &coreRuntimeSpy{}
	service := CoreService{Runtime: runtime}
	_, err := service.Run(context.Background(), CoreRunRequest{
		SuitePath: "suite.json", Runtime: "bogus", BaselineMode: "skill-on", ContextMode: "none",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown runtime") {
		t.Fatalf("Run error = %v, want unknown runtime", err)
	}
	if runtime.runCalls != 0 {
		t.Fatalf("RunSuite calls = %d, want 0", runtime.runCalls)
	}
}

func TestCoreServiceRunDelegatesSingleRun(t *testing.T) {
	runtime := &coreRuntimeSpy{}
	service := CoreService{Runtime: runtime}
	result, err := service.Run(context.Background(), CoreRunRequest{
		SuitePath: "suite.json", RunID: "run-1", Runtime: "static", BaselineMode: "skill-off", ContextMode: "none",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Mode != CoreRunSingle || result.Run == nil || result.Run.RunID != "run-1" {
		t.Fatalf("result = %#v", result)
	}
	if runtime.runCalls != 1 {
		t.Fatalf("RunSuite calls = %d, want 1", runtime.runCalls)
	}
}
