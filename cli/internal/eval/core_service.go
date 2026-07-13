package eval

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

type CoreRuntime interface {
	RunSuite(RunOptions) (*RunRecord, error)
	RunBaselineAB(RunOptions) (DeltaScorecard, *RunRecord, *RunRecord, error)
	WriteDeltaScorecard(DeltaScorecard, string) error
	RunContextAB(RunOptions, ContextABOptions) (ContextDeltaScorecard, *RunRecord, *RunRecord, error)
	WriteContextDeltaScorecard(ContextDeltaScorecard, string) error
	LoadRun(string) (*RunRecord, error)
	CompareRuns(*RunRecord, *RunRecord, CompareOptions) (*RunRecord, error)
	WorkDir() (string, error)
	Abs(string) (string, error)
	PromoteBaseline(*RunRecord, BaselineOptions) (*RunRecord, error)
	AuditBaselinePolicy(BaselineAuditOptions) (*BaselineAuditReport, error)
	BuildScorecard(*RunRecord, *RunRecord, ScorecardOptions) (*Scorecard, error)
	WriteScorecard(string, *Scorecard) error
	BuildCoverageReport(CoverageOptions) (*CoverageReport, error)
}

type CoreService struct{ Runtime CoreRuntime }

type CoreRunRequest struct {
	SuitePath, RunID, Runtime, OutputPath, BaselinePath              string
	BaselineMode, ContextMode, ContextOffDir, ContextOnDir, DeltaOut string
}

type CoreRunMode string

const (
	CoreRunSingle     CoreRunMode = "single"
	CoreRunBaselineAB CoreRunMode = "baseline-ab"
	CoreRunContextAB  CoreRunMode = "context-ab"
)

type CoreRunResult struct {
	Mode         CoreRunMode
	Run          *RunRecord
	FirstRun     *RunRecord
	SecondRun    *RunRecord
	Delta        *DeltaScorecard
	ContextDelta *ContextDeltaScorecard
	OutputPath   string
}

type CoreCompareRequest struct {
	CandidatePath, BaselinePath, OutputPath        string
	MaxAggregateRegression, MaxDimensionRegression float64
}

type CoreCompareResult struct {
	Candidate  *RunRecord
	Baseline   *RunRecord
	OutputPath string
}

type CoreBaselineRequest struct{ RunPath, OutputPath, PromotedBy, Rationale string }
type CoreBaselineAuditRequest struct {
	SuitePaths        []string
	Root, BaselineDir string
}
type CoreScorecardRequest struct {
	CandidatePath, BaselinePath, OutputPath, Kind string
	MaxCategoryRegression                         float64
}
type CoreCoverageRequest struct {
	SuitePaths                                                                   []string
	Root                                                                         string
	RequiredDomains, RequiredEvidenceKinds, RequiredDimensions, RequiredRuntimes []string
}

func (service CoreService) Run(_ context.Context, request CoreRunRequest) (CoreRunResult, error) {
	runtimeName, err := parseCoreRuntime(request.Runtime)
	if err != nil {
		return CoreRunResult{}, err
	}
	mode := ABBaselineMode(request.BaselineMode)
	if !IsValidBaselineMode(string(mode)) {
		return CoreRunResult{}, fmt.Errorf("invalid --baseline-mode %q (allowed: %s)", request.BaselineMode, strings.Join(AllBaselineModes(), ", "))
	}
	contextMode := ContextMode(request.ContextMode)
	if !IsValidContextMode(string(contextMode)) {
		return CoreRunResult{}, fmt.Errorf("invalid --context-mode %q (allowed: %s)", request.ContextMode, strings.Join(AllContextModes(), ", "))
	}
	options := RunOptions{SuitePath: request.SuitePath, RunID: request.RunID, Runtime: runtimeName, OutputPath: request.OutputPath, BaselinePath: request.BaselinePath}
	if contextMode == ContextModeAB {
		if mode != BaselineModeSkillOn {
			return CoreRunResult{}, fmt.Errorf("--context-mode=ab cannot be combined with --baseline-mode=%s", mode)
		}
		contextOptions := service.contextOptions(request.SuitePath, request.ContextOffDir, request.ContextOnDir)
		scorecard, offRun, onRun, err := service.Runtime.RunContextAB(options, contextOptions)
		if err != nil {
			return CoreRunResult{}, err
		}
		if err := service.Runtime.WriteContextDeltaScorecard(scorecard, request.DeltaOut); err != nil {
			return CoreRunResult{}, err
		}
		return CoreRunResult{Mode: CoreRunContextAB, FirstRun: offRun, SecondRun: onRun, ContextDelta: &scorecard, OutputPath: request.DeltaOut}, nil
	}
	if mode == BaselineModeBoth {
		scorecard, onRun, offRun, err := service.Runtime.RunBaselineAB(options)
		if err != nil {
			return CoreRunResult{}, err
		}
		if err := service.Runtime.WriteDeltaScorecard(scorecard, request.DeltaOut); err != nil {
			return CoreRunResult{}, err
		}
		return CoreRunResult{Mode: CoreRunBaselineAB, FirstRun: onRun, SecondRun: offRun, Delta: &scorecard, OutputPath: request.DeltaOut}, nil
	}
	options.OverrideDisableHooks = mode == BaselineModeSkillOff
	run, err := service.Runtime.RunSuite(options)
	if err != nil {
		return CoreRunResult{}, err
	}
	return CoreRunResult{Mode: CoreRunSingle, Run: run, OutputPath: request.OutputPath}, nil
}

func (service CoreService) Compare(_ context.Context, request CoreCompareRequest) (CoreCompareResult, error) {
	candidate, err := service.Runtime.LoadRun(request.CandidatePath)
	if err != nil {
		return CoreCompareResult{}, err
	}
	baseline, err := service.Runtime.LoadRun(request.BaselinePath)
	if err != nil {
		return CoreCompareResult{}, err
	}
	compared, err := service.Runtime.CompareRuns(candidate, baseline, CompareOptions{MaxAggregateRegression: request.MaxAggregateRegression, MaxDimensionRegression: request.MaxDimensionRegression, OutputPath: request.OutputPath})
	if err != nil {
		return CoreCompareResult{}, err
	}
	return CoreCompareResult{Candidate: compared, Baseline: baseline, OutputPath: request.OutputPath}, nil
}

func (service CoreService) PromoteBaseline(_ context.Context, request CoreBaselineRequest) (*RunRecord, error) {
	run, err := service.Runtime.LoadRun(request.RunPath)
	if err != nil {
		return nil, err
	}
	workDir, err := service.Runtime.WorkDir()
	if err != nil {
		return nil, err
	}
	return service.Runtime.PromoteBaseline(run, BaselineOptions{OutputPath: request.OutputPath, PromotedBy: request.PromotedBy, Rationale: request.Rationale, WorkDir: workDir})
}

func (service CoreService) AuditBaseline(_ context.Context, request CoreBaselineAuditRequest) (*BaselineAuditReport, error) {
	roots := []string{}
	if len(request.SuitePaths) == 0 {
		roots = append(roots, request.Root)
	}
	return service.Runtime.AuditBaselinePolicy(BaselineAuditOptions{SuitePaths: request.SuitePaths, Roots: roots, BaselineDir: request.BaselineDir})
}

func (service CoreService) Scorecard(_ context.Context, request CoreScorecardRequest) (*Scorecard, error) {
	kind, err := parseCoreScorecardKind(request.Kind)
	if err != nil {
		return nil, err
	}
	candidate, err := service.Runtime.LoadRun(request.CandidatePath)
	if err != nil {
		return nil, err
	}
	var baseline *RunRecord
	if request.BaselinePath != "" {
		baseline, err = service.Runtime.LoadRun(request.BaselinePath)
		if err != nil {
			return nil, err
		}
	}
	card, err := service.Runtime.BuildScorecard(candidate, baseline, ScorecardOptions{Kind: kind, MaxCategoryRegression: request.MaxCategoryRegression})
	if err != nil {
		return nil, err
	}
	if request.OutputPath != "" {
		if err := service.Runtime.WriteScorecard(request.OutputPath, card); err != nil {
			return nil, err
		}
	}
	return card, nil
}

func (service CoreService) Coverage(_ context.Context, request CoreCoverageRequest) (*CoverageReport, error) {
	roots := []string{}
	if len(request.SuitePaths) == 0 {
		roots = append(roots, request.Root)
	}
	return service.Runtime.BuildCoverageReport(CoverageOptions{SuitePaths: request.SuitePaths, Roots: roots, RequiredDomains: request.RequiredDomains, RequiredEvidenceKinds: request.RequiredEvidenceKinds, RequiredDimensions: request.RequiredDimensions, RequiredRuntimes: request.RequiredRuntimes})
}

func (service CoreService) contextOptions(suitePath, offDir, onDir string) ContextABOptions {
	if offDir == "" {
		offDir = defaultCoreContextDir(suitePath, "context-off")
	}
	if onDir == "" {
		onDir = defaultCoreContextDir(suitePath, "context-on")
	}
	if absolute, err := service.Runtime.Abs(offDir); err == nil {
		offDir = absolute
	}
	if absolute, err := service.Runtime.Abs(onDir); err == nil {
		onDir = absolute
	}
	return ContextABOptions{ContextOffAgentsDir: offDir, ContextOnAgentsDir: onDir, ContextOffLabel: "context-off", ContextOnLabel: "context-on"}
}

func defaultCoreContextDir(suitePath, leg string) string {
	base := filepath.Base(suitePath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(filepath.Dir(suitePath), "fixtures", name, leg, "agents")
}

func parseCoreRuntime(value string) (Runtime, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	switch Runtime(value) {
	case RuntimeStatic, RuntimeMock, RuntimeShell, RuntimeClaude, RuntimeCodex:
		return Runtime(value), nil
	default:
		return "", fmt.Errorf("unknown runtime %q (use static, mock, shell, claude, or codex)", value)
	}
}

func parseCoreScorecardKind(value string) (ScorecardKind, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return ScorecardKindRPI, nil
	}
	switch ScorecardKind(value) {
	case ScorecardKindRPI, ScorecardKindSkillChange:
		return ScorecardKind(value), nil
	default:
		return "", fmt.Errorf("unsupported scorecard kind %q (use rpi or skill-change)", value)
	}
}
