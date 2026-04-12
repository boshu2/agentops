package overnight

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/boshu2/agentops/cli/internal/corpus"
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
