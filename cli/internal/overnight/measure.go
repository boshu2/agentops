package overnight

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/corpus"
	"github.com/boshu2/agentops/cli/internal/daemon"
)

// MeasureResult is the output of a single MEASURE stage.
//
// MEASURE is read-only. Every field is either a derived metric or a
// diagnostic note. Downstream code uses FitnessSnapshot (not the raw
// corpus.FitnessVector) when computing deltas so the plateau/regression
// machinery stays source-agnostic.
type MeasureResult struct {
	// Fitness is the canonical FitnessVector produced by corpus.Compute.
	Fitness *corpus.FitnessVector

	// FitnessSnapshot is the normalized snapshot marshaled from Fitness
	// for use with PlateauState and FitnessSnapshot.Delta. See the
	// package-level "unresolved_findings inverted sign" comment on
	// RunMeasure for why the sign matters.
	FitnessSnapshot FitnessSnapshot

	// MetricsHealth is reserved for the ao metrics health integration.
	// Always nil in Wave 3; populated in a follow-up slice.
	MetricsHealth map[string]any

	// InjectVisibility mirrors corpus.FitnessVector.InjectVisibility for
	// direct access by callers that don't want to reach into Fitness.
	InjectVisibility float64

	// FindingsResolved is the delta between iteration-start and
	// iteration-end unresolved-findings counts. -1 means unknown (the
	// first slice does not track a baseline, so this stays at -1 for
	// now).
	FindingsResolved int

	// Degraded lists substage notes for soft-failed or deferred stages.
	Degraded []string

	// StageFailures maps substage name to error string for stages that
	// returned a hard error but did not propagate out of RunMeasure.
	StageFailures map[string]string

	// Duration is the wall-clock time RunMeasure took end-to-end.
	Duration time.Duration
}

type MeasureStageJobOptions struct {
	Spec                       daemon.DreamStageJobSpec
	RunOptions                 RunLoopOptions
	Checkpoint                 *Checkpoint
	CheckpointManifestPath     string
	PreviousSnapshot           *FitnessSnapshot
	Plateau                    *PlateauState
	ConsecutiveMeasureFailures int
	Log                        io.Writer
	Now                        func() time.Time
}

type MeasureFitnessOutput struct {
	Metrics    map[string]float64 `json:"metrics,omitempty"`
	CapturedAt string             `json:"captured_at,omitempty"`
}

type MeasureHaltKind string

const (
	MeasureHaltNone         MeasureHaltKind = "none"
	MeasureHaltRegression   MeasureHaltKind = "regression"
	MeasureHaltPlateau      MeasureHaltKind = "plateau"
	MeasureHaltMeasureError MeasureHaltKind = "measure-error"
)

type MeasureHaltInput struct {
	Current                    FitnessSnapshot
	Previous                   *FitnessSnapshot
	RegressionFloor            float64
	Plateau                    *PlateauState
	PlateauWindowK             int
	PlateauEpsilon             float64
	WarnOnly                   bool
	WarnOnlyBudgetRemaining    *int
	MeasureError               error
	ConsecutiveMeasureFailures int
	MaxConsecutiveFailures     int
}

type MeasureHaltOutput struct {
	Kind                       MeasureHaltKind    `json:"kind"`
	ShouldHalt                 bool               `json:"should_halt"`
	Reason                     string             `json:"reason,omitempty"`
	EffectiveWarnOnly          bool               `json:"effective_warn_only"`
	FitnessDelta               float64            `json:"fitness_delta"`
	Regressed                  bool               `json:"regressed"`
	Regressions                []MetricRegression `json:"regressions,omitempty"`
	PlateauReached             bool               `json:"plateau_reached"`
	PlateauReason              string             `json:"plateau_reason,omitempty"`
	WarnOnlyBudgetRemaining    *int               `json:"warn_only_budget_remaining,omitempty"`
	ConsecutiveMeasureFailures int                `json:"consecutive_measure_failures,omitempty"`
	MaxConsecutiveFailures     int                `json:"max_consecutive_measure_failures,omitempty"`
}

type MeasureStageJobResult struct {
	SchemaVersion          int                  `json:"schema_version"`
	DreamRunID             string               `json:"dream_run_id"`
	IterationID            string               `json:"iteration_id,omitempty"`
	Stage                  string               `json:"stage"`
	Status                 string               `json:"status"`
	Error                  string               `json:"error,omitempty"`
	OutputDir              string               `json:"output_dir"`
	ResultPath             string               `json:"result_path"`
	CheckpointManifestPath string               `json:"checkpoint_manifest_path,omitempty"`
	CheckpointPath         string               `json:"checkpoint_path,omitempty"`
	StartedAt              string               `json:"started_at"`
	CompletedAt            string               `json:"completed_at"`
	DurationMillis         int64                `json:"duration_millis"`
	Fitness                MeasureFitnessOutput `json:"fitness"`
	Halt                   MeasureHaltOutput    `json:"halt"`
	InjectVisibility       float64              `json:"inject_visibility"`
	FindingsResolved       int                  `json:"findings_resolved"`
	Degraded               []string             `json:"degraded,omitempty"`
	StageFailures          map[string]string    `json:"stage_failures,omitempty"`
}

func EvaluateMeasureHalt(input MeasureHaltInput) MeasureHaltOutput {
	out := MeasureHaltOutput{
		Kind:                       MeasureHaltNone,
		EffectiveWarnOnly:          effectiveMeasureWarnOnly(input.WarnOnly, input.WarnOnlyBudgetRemaining),
		WarnOnlyBudgetRemaining:    cloneIntPtr(input.WarnOnlyBudgetRemaining),
		ConsecutiveMeasureFailures: input.ConsecutiveMeasureFailures,
		MaxConsecutiveFailures:     input.MaxConsecutiveFailures,
	}
	if input.MeasureError != nil {
		out.Kind = MeasureHaltMeasureError
		out.Reason = input.MeasureError.Error()
		if input.MaxConsecutiveFailures != -1 &&
			input.ConsecutiveMeasureFailures >= input.MaxConsecutiveFailures {
			out.ShouldHalt = true
			out.Reason = fmt.Sprintf("%d consecutive MEASURE failures reached cap %d",
				input.ConsecutiveMeasureFailures, input.MaxConsecutiveFailures)
		}
		return out
	}
	if input.Previous == nil {
		return out
	}

	composite, regressions, regressed := input.Current.Delta(input.Previous, nil, input.RegressionFloor)
	out.FitnessDelta = composite
	out.Regressions = append([]MetricRegression(nil), regressions...)
	out.Regressed = regressed
	if regressed {
		out.Kind = MeasureHaltRegression
		out.Reason = fmt.Sprintf("%d metric(s) breached regression floor %g: %v",
			len(regressions), effectiveRegressionFloor(input.RegressionFloor), regressionNames(regressions))
		if !out.EffectiveWarnOnly {
			out.ShouldHalt = true
			return out
		}
	}

	if !regressed || out.EffectiveWarnOnly {
		plateau := input.Plateau
		if plateau == nil && input.PlateauWindowK >= 2 && input.PlateauEpsilon > 0 {
			plateau = NewPlateauState(input.PlateauWindowK, input.PlateauEpsilon)
		}
		if plateau != nil && plateau.Observe(composite) {
			out.PlateauReached = true
			out.PlateauReason = plateau.Reason()
			if out.Kind == MeasureHaltNone {
				out.Kind = MeasureHaltPlateau
				out.Reason = out.PlateauReason
			}
			if !out.EffectiveWarnOnly {
				out.Kind = MeasureHaltPlateau
				out.Reason = out.PlateauReason
				out.ShouldHalt = true
			}
		}
	}
	return out
}

// RunMeasure executes the parallel-safe MEASURE stage.
//
// MEASURE never mutates .agents/. Substages:
//
//  1. corpus.Compute(cwd) — fitness vector (LOAD-BEARING; errors propagate).
//  2. ao metrics health (deferred: in-process entry not yet wired).
//  3. ao retrieval-bench --live (deferred: see pm-012 — the metric
//     itself is deterministic; the in-process call via internal/bench
//     lands in a follow-up slice).
//  4. inject-visibility probe — corpus.Compute already produces this
//     as FitnessVector.InjectVisibility, so MeasureResult just copies
//     it forward.
//  5. findings resolution delta — pass-through of
//     corpus.FitnessVector.UnresolvedFindings (delta baseline
//     bookkeeping is a follow-up).
//
// The FitnessSnapshot returned in MeasureResult maps the seven corpus
// metrics into the snapshot's string-keyed Metrics map:
//
//	metrics["retrieval_precision"]            = vec.RetrievalPrecision
//	metrics["retrieval_recall"]               = vec.RetrievalRecall
//	metrics["maturity_provisional_or_higher"] = vec.MaturityProvisional
//	metrics["unresolved_findings"]            = -float64(vec.UnresolvedFindings)
//	metrics["citation_coverage"]              = vec.CitationCoverage
//	metrics["inject_visibility"]              = vec.InjectVisibility
//	metrics["cross_rig_dedup_ratio"]          = vec.CrossRigDedupRatio
//
// The negation on unresolved_findings is load-bearing: higher values
// in the snapshot always mean "better", so FitnessSnapshot.Delta can
// treat every metric uniformly when computing the composite. Without
// the flip, a drop in unresolved findings (good) would register as a
// regression.
func RunMeasure(ctx context.Context, opts RunLoopOptions, log io.Writer) (*MeasureResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if log == nil {
		log = io.Discard
	}
	started := time.Now()

	result := &MeasureResult{
		StageFailures:    map[string]string{},
		FindingsResolved: -1,
	}

	if opts.Cwd == "" {
		return result, fmt.Errorf("overnight: RunMeasure requires RunLoopOptions.Cwd")
	}

	// Substage 1: corpus.Compute (load-bearing).
	if err := ctxCheck(ctx); err != nil {
		return result, err
	}
	fmt.Fprintln(log, "overnight/measure: corpus.Compute start")
	vec, cDegraded, err := corpus.Compute(opts.Cwd)
	if err != nil {
		return result, fmt.Errorf("overnight/measure: corpus.Compute: %w", err)
	}
	result.Fitness = vec
	for _, d := range cDegraded {
		result.Degraded = append(result.Degraded,
			fmt.Sprintf("corpus: %s", d))
	}
	fmt.Fprintf(log, "overnight/measure: corpus.Compute done (unresolved=%d)\n", vec.UnresolvedFindings)

	// Marshal to a FitnessSnapshot for plateau/delta machinery.
	capturedAt := vec.ComputedAt
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	result.FitnessSnapshot = FitnessSnapshot{
		Metrics: map[string]float64{
			"retrieval_precision":            vec.RetrievalPrecision,
			"retrieval_recall":               vec.RetrievalRecall,
			"maturity_provisional_or_higher": vec.MaturityProvisional,
			// Invert: fewer unresolved findings is better, so the
			// snapshot stores the negated count so delta arithmetic
			// stays uniform (higher = better).
			"unresolved_findings":   -float64(vec.UnresolvedFindings),
			"citation_coverage":     vec.CitationCoverage,
			"inject_visibility":     vec.InjectVisibility,
			"cross_rig_dedup_ratio": vec.CrossRigDedupRatio,
		},
		CapturedAt: capturedAt,
	}

	// Substage 4 (data pass-through): inject visibility.
	result.InjectVisibility = vec.InjectVisibility

	// Substage 2: ao metrics health (deferred).
	if err := ctxCheck(ctx); err != nil {
		return result, err
	}
	result.Degraded = append(result.Degraded,
		"metrics-health: in-process entry deferred to follow-up")
	fmt.Fprintln(log, "overnight/measure: metrics-health deferred")

	// Substage 3: retrieval-bench --live (deferred).
	if err := ctxCheck(ctx); err != nil {
		return result, err
	}
	result.Degraded = append(result.Degraded,
		"retrieval-bench: internal/bench in-process entry deferred to follow-up")
	fmt.Fprintln(log, "overnight/measure: retrieval-bench deferred")

	// Substage 5: findings resolution delta (baseline bookkeeping deferred).
	if err := ctxCheck(ctx); err != nil {
		return result, err
	}
	result.Degraded = append(result.Degraded,
		"findings-resolved: iteration baseline tracking deferred to follow-up")

	result.Duration = stageDurationSince(started)
	fmt.Fprintf(log, "overnight/measure: done in %s\n", result.Duration)
	return result, nil
}

func RunMeasureStageJob(ctx context.Context, opts MeasureStageJobOptions) (MeasureStageJobResult, error) {
	spec := opts.Spec
	if err := spec.Validate(); err != nil {
		return MeasureStageJobResult{}, err
	}
	if spec.Stage != daemon.DreamStageMeasure {
		return MeasureStageJobResult{}, fmt.Errorf("overnight/measure: stage job has stage %q, want %q", spec.Stage, daemon.DreamStageMeasure)
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	started := now().UTC()
	runOpts := opts.RunOptions
	if runOpts.Cwd == "" {
		return MeasureStageJobResult{}, fmt.Errorf("overnight/measure: stage job requires RunOptions.Cwd")
	}
	if runOpts.OutputDir == "" {
		runOpts.OutputDir = spec.OutputDir
	}
	if runOpts.OutputDir == "" {
		return MeasureStageJobResult{}, fmt.Errorf("overnight/measure: stage job requires output_dir")
	}
	if runOpts.RunID == "" {
		runOpts.RunID = spec.DreamRunID
	}
	runOpts, normalizedNotes := runOpts.normalize()
	cp, manifestPath, err := measureStageCheckpoint(spec, runOpts, opts)
	if err != nil {
		return MeasureStageJobResult{}, err
	}
	if cp != nil {
		runOpts.Cwd = cp.StagingDir
	}

	measure, measureErr := RunMeasure(ctx, runOpts, opts.Log)
	if measure != nil && len(normalizedNotes) > 0 {
		measure.Degraded = append(measure.Degraded, normalizedNotes...)
	}
	completed := now().UTC()
	stageResult := buildMeasureStageJobResult(spec, runOpts, manifestPath, cp, opts, started, completed, measure, measureErr)
	stageResult.ResultPath = MeasureStageJobResultPath(runOpts.OutputDir)
	path, writeErr := WriteMeasureStageJobResult(runOpts.OutputDir, stageResult)
	stageResult.ResultPath = path
	if measureErr != nil {
		if writeErr != nil {
			return stageResult, fmt.Errorf("%w; write measure stage result: %v", measureErr, writeErr)
		}
		return stageResult, measureErr
	}
	if writeErr != nil {
		return stageResult, writeErr
	}
	return stageResult, nil
}

func MeasureStageJobResultPath(outputDir string) string {
	return filepath.Join(outputDir, "stages", "measure-result.json")
}

func WriteMeasureStageJobResult(outputDir string, result MeasureStageJobResult) (string, error) {
	if outputDir == "" {
		return "", fmt.Errorf("overnight/measure: output_dir is required")
	}
	path := MeasureStageJobResultPath(outputDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("overnight/measure: mkdir stage result dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("overnight/measure: marshal stage result: %w", err)
	}
	data = append(data, '\n')
	if err := writeFileAtomic(path, data, 0o644); err != nil {
		return "", fmt.Errorf("overnight/measure: write stage result: %w", err)
	}
	return path, nil
}

func measureStageCheckpoint(
	spec daemon.DreamStageJobSpec,
	runOpts RunLoopOptions,
	opts MeasureStageJobOptions,
) (*Checkpoint, string, error) {
	if opts.Checkpoint != nil {
		manifestPath := opts.CheckpointManifestPath
		if manifestPath == "" {
			manifestPath = filepath.Join(runOpts.OutputDir, "stages", CheckpointManifestFileName)
		}
		if err := WriteCheckpointManifest(manifestPath, opts.Checkpoint.Manifest()); err != nil {
			return nil, "", err
		}
		return opts.Checkpoint, manifestPath, nil
	}
	manifestPath := firstNonEmptyString(opts.CheckpointManifestPath, spec.CheckpointDir)
	if manifestPath == "" {
		return nil, "", nil
	}
	manifestPath = resolveCheckpointManifestPath(manifestPath)
	manifest, err := ReadCheckpointManifest(manifestPath)
	if err != nil {
		return nil, "", err
	}
	cp, err := CheckpointFromManifest(manifest)
	return cp, manifestPath, err
}

func buildMeasureStageJobResult(
	spec daemon.DreamStageJobSpec,
	runOpts RunLoopOptions,
	manifestPath string,
	cp *Checkpoint,
	opts MeasureStageJobOptions,
	started time.Time,
	completed time.Time,
	measure *MeasureResult,
	runErr error,
) MeasureStageJobResult {
	status := "completed"
	if runErr != nil {
		status = "failed"
	}
	result := MeasureStageJobResult{
		SchemaVersion:          1,
		DreamRunID:             spec.DreamRunID,
		IterationID:            spec.IterationID,
		Stage:                  string(daemon.DreamStageMeasure),
		Status:                 status,
		OutputDir:              runOpts.OutputDir,
		CheckpointManifestPath: manifestPath,
		StartedAt:              started.UTC().Format(time.RFC3339Nano),
		CompletedAt:            completed.UTC().Format(time.RFC3339Nano),
		DurationMillis:         completed.Sub(started).Milliseconds(),
	}
	if cp != nil {
		result.CheckpointPath = cp.StagingDir
	}
	if runErr != nil {
		result.Error = runErr.Error()
		consecutiveFailures := opts.ConsecutiveMeasureFailures
		if consecutiveFailures <= 0 {
			consecutiveFailures = 1
		}
		result.Halt = EvaluateMeasureHalt(MeasureHaltInput{
			WarnOnly:                   runOpts.WarnOnly,
			WarnOnlyBudgetRemaining:    warnOnlyBudgetRemaining(runOpts),
			MeasureError:               runErr,
			ConsecutiveMeasureFailures: consecutiveFailures,
			MaxConsecutiveFailures:     runOpts.MaxConsecutiveMeasureFailures,
		})
		return result
	}
	if measure == nil {
		result.Halt = EvaluateMeasureHalt(MeasureHaltInput{
			WarnOnly:                runOpts.WarnOnly,
			WarnOnlyBudgetRemaining: warnOnlyBudgetRemaining(runOpts),
			MaxConsecutiveFailures:  runOpts.MaxConsecutiveMeasureFailures,
		})
		return result
	}
	result.Fitness = MeasureFitnessOutput{
		Metrics:    cloneFloat64Map(measure.FitnessSnapshot.Metrics),
		CapturedAt: measure.FitnessSnapshot.CapturedAt.UTC().Format(time.RFC3339Nano),
	}
	result.Halt = EvaluateMeasureHalt(MeasureHaltInput{
		Current:                 measure.FitnessSnapshot,
		Previous:                opts.PreviousSnapshot,
		RegressionFloor:         runOpts.RegressionFloor,
		Plateau:                 opts.Plateau,
		PlateauWindowK:          runOpts.PlateauWindowK,
		PlateauEpsilon:          runOpts.PlateauEpsilon,
		WarnOnly:                runOpts.WarnOnly,
		WarnOnlyBudgetRemaining: warnOnlyBudgetRemaining(runOpts),
		MaxConsecutiveFailures:  runOpts.MaxConsecutiveMeasureFailures,
	})
	result.InjectVisibility = measure.InjectVisibility
	result.FindingsResolved = measure.FindingsResolved
	result.Degraded = append([]string(nil), measure.Degraded...)
	result.StageFailures = cloneStringMap(measure.StageFailures)
	return result
}

func effectiveMeasureWarnOnly(warnOnly bool, budgetRemaining *int) bool {
	if budgetRemaining != nil && *budgetRemaining <= 0 {
		return false
	}
	return warnOnly
}

func effectiveRegressionFloor(floor float64) float64 {
	if floor <= 0 {
		return DefaultRegressionFloor
	}
	return floor
}

func warnOnlyBudgetRemaining(opts RunLoopOptions) *int {
	if opts.WarnOnlyBudget == nil {
		return nil
	}
	value := opts.WarnOnlyBudget.Remaining
	return &value
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneFloat64Map(values map[string]float64) map[string]float64 {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]float64, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
