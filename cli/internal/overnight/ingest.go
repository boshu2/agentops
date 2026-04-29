package overnight

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/boshu2/agentops/cli/internal/forge"
	"github.com/boshu2/agentops/cli/internal/harvest"
	"github.com/boshu2/agentops/cli/internal/mine"
	"github.com/boshu2/agentops/cli/internal/provenance"
)

// IngestResult is the output of a single INGEST stage.
//
// All counters are zero when the corresponding substage is skipped or
// deferred; Degraded entries explain each degradation in human-readable
// form so the morning report can surface them without interpretation.
type IngestResult struct {
	// HarvestPreviewCount is the number of artifacts that would be
	// promoted by harvest in a non-dry-run pass. Computed via
	// Promote(..., dryRun=true) so INGEST never mutates the learnings
	// tree.
	HarvestPreviewCount int

	// HarvestCatalog is the in-memory catalog built by INGEST and handed
	// off to REDUCE for the real promotion pass. Nil only when the
	// catalog could not be produced (which makes the stage a hard
	// failure).
	HarvestCatalog *harvest.Catalog

	// ForgeArtifactsMined is the count of transcript artifacts produced
	// by the forge pass. Zero when skipped or deferred.
	ForgeArtifactsMined int

	// ProvenanceAudited is the count of provenance entries refreshed by
	// the provenance pass. Zero when skipped or deferred.
	ProvenanceAudited int

	// MineFindingsNew is the count of new discoveries produced by the
	// ao mine drift/complexity pass. Zero when skipped or deferred.
	MineFindingsNew int

	// GeneratorCandidateCount is the total number of candidate work items
	// emitted by Dream read-side finding generators before dedup.
	GeneratorCandidateCount int

	// GeneratorDuplicateCount is the number of generator candidates already
	// present in next-work. It lets Dream measure generator yield without
	// allowing generators to write next-work directly.
	GeneratorDuplicateCount int

	// GeneratorDuplicateRate is DuplicateCount / CandidateCount. Zero when
	// there are no candidates.
	GeneratorDuplicateRate float64

	// GeneratorSidecarCount is the count of per-generator sidecars written
	// during INGEST.
	GeneratorSidecarCount int

	// GeneratorSoftFailCount is the count of generators that emitted a
	// soft-fail sidecar instead of completed candidates.
	GeneratorSoftFailCount int

	// GeneratorSidecarPath points at the latest generator sidecar written
	// during INGEST, relative or absolute according to RunLoopOptions.
	GeneratorSidecarPath string

	// GeneratorSidecarPaths points at every generator sidecar written during
	// INGEST, relative or absolute according to RunLoopOptions.
	GeneratorSidecarPaths []string

	// ExternalWatchlistEmitted is the number of held-for-review candidates
	// produced by the external-watchlist generator (RFC 0001 Proposal 2).
	// Lane-specific lookup so morning summaries can surface external-source
	// throughput without parsing every sidecar; the broader generator
	// counters above stay generator-agnostic.
	ExternalWatchlistEmitted int

	// Degraded lists human-readable degradation notes for substages that
	// were skipped, deferred, or soft-failed.
	Degraded []string

	// StageFailures maps substage name to error string for substages
	// that hard-failed in a way that did not propagate out of RunIngest
	// (the load-bearing harvest catalog still propagates errors).
	StageFailures map[string]string

	// Duration is the wall-clock time RunIngest took end-to-end.
	Duration time.Duration
}

type IngestStageJobOptions struct {
	Spec       daemon.DreamStageJobSpec
	RunOptions RunLoopOptions
	Log        io.Writer
	Now        func() time.Time
}

type IngestStageJobResult struct {
	SchemaVersion              int               `json:"schema_version"`
	DreamRunID                 string            `json:"dream_run_id"`
	Stage                      string            `json:"stage"`
	Status                     string            `json:"status"`
	OutputDir                  string            `json:"output_dir"`
	ResultPath                 string            `json:"result_path"`
	StartedAt                  string            `json:"started_at"`
	CompletedAt                string            `json:"completed_at"`
	DurationMillis             int64             `json:"duration_millis"`
	HarvestPreviewCount        int               `json:"harvest_preview_count"`
	ForgeArtifactsMined        int               `json:"forge_artifacts_mined"`
	ProvenanceAudited          int               `json:"provenance_audited"`
	MineFindingsNew            int               `json:"mine_findings_new"`
	GeneratorCandidateCount    int               `json:"generator_candidate_count"`
	GeneratorDuplicateCount    int               `json:"generator_duplicate_count"`
	GeneratorDuplicateRate     float64           `json:"generator_duplicate_rate"`
	GeneratorSidecarCount      int               `json:"generator_sidecar_count"`
	GeneratorSoftFailCount     int               `json:"generator_soft_fail_count"`
	ExternalWatchlistEmitted   int               `json:"external_watchlist_emitted"`
	GeneratorSidecarPaths      []string          `json:"generator_sidecar_paths,omitempty"`
	Degraded                   []string          `json:"degraded,omitempty"`
	StageFailures              map[string]string `json:"stage_failures,omitempty"`
	HarvestArtifactsExtracted  int               `json:"harvest_artifacts_extracted,omitempty"`
	HarvestPromotionCandidates int               `json:"harvest_promotion_candidates,omitempty"`
}

// RunIngest executes the parallel-safe INGEST stage.
//
// RunIngest never mutates the Dream corpus or next-work queue. It runs serial
// corpus substages, then bounded read-side finding generators may fan out and
// write per-run sidecars under OutputDir/generator-results so candidate yield
// is measurable without creating queue write races. The stage as a whole only
// returns a non-nil error when the harvest catalog cannot be produced — that
// catalog is the load-bearing output REDUCE needs to run its real promotion
// pass.
//
// Substage order:
//
//  1. harvest.DiscoverRigs + harvest.ExtractArtifacts +
//     harvest.BuildCatalog, scoped to opts.Cwd (load-bearing).
//  2. harvest.Promote(catalog, dest, dryRun=true) — preview count only.
//  3. forge.RunMinePass — in-process mining of forged session files
//     under .agents/sessions/ (Wave 2 Issue 5 wiring).
//  4. provenance.Audit — in-process audit of .agents/learnings/ for
//     stale/missing citations (Wave 2 Issue 5 wiring).
//  5. finding generator fanout — read-only candidate producers, currently
//     mine.Run with EmitWorkItems=false so INGEST never writes next-work.
//
// Substages 3-5 soft-fail independently: a single error degrades that
// substage only and the stage continues. This is honest degradation per
// pm-003 and skills/dream/SKILL.md.
func RunIngest(ctx context.Context, opts RunLoopOptions, log io.Writer) (*IngestResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if log == nil {
		log = io.Discard
	}
	started := time.Now()
	result := newIngestResult()
	if err := validateIngestInputs(opts); err != nil {
		return result, err
	}

	runner := &ingestRunner{ctx: ctx, opts: opts, log: log, result: result, started: started}
	if err := runner.runHarvestCatalog(); err != nil {
		return result, err
	}
	if err := runner.runHarvestPreview(); err != nil {
		return result, err
	}
	if err := runner.runForgeMine(); err != nil {
		return result, err
	}
	if err := runner.runProvenanceAudit(); err != nil {
		return result, err
	}
	if err := runner.runFindingGenerators(); err != nil {
		return result, err
	}

	result.Duration = stageDurationSince(started)
	fmt.Fprintf(log, "overnight/ingest: done in %s\n", result.Duration)
	return result, nil
}

func RunIngestStageJob(ctx context.Context, opts IngestStageJobOptions) (IngestStageJobResult, error) {
	spec := opts.Spec
	if err := spec.Validate(); err != nil {
		return IngestStageJobResult{}, err
	}
	if spec.Stage != daemon.DreamStageIngest {
		return IngestStageJobResult{}, fmt.Errorf("overnight/ingest: stage job has stage %q, want %q", spec.Stage, daemon.DreamStageIngest)
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	started := now().UTC()
	runOpts := opts.RunOptions
	if runOpts.Cwd == "" {
		return IngestStageJobResult{}, fmt.Errorf("overnight/ingest: stage job requires RunOptions.Cwd")
	}
	if runOpts.OutputDir == "" {
		runOpts.OutputDir = spec.OutputDir
	}
	if runOpts.OutputDir == "" {
		return IngestStageJobResult{}, fmt.Errorf("overnight/ingest: stage job requires output_dir")
	}
	if runOpts.RunID == "" {
		runOpts.RunID = spec.DreamRunID
	}
	result, err := RunIngest(ctx, runOpts, opts.Log)
	completed := now().UTC()
	stageResult := buildIngestStageJobResult(spec, runOpts.OutputDir, started, completed, result, err)
	path, writeErr := WriteIngestStageJobResult(runOpts.OutputDir, stageResult)
	stageResult.ResultPath = path
	if err != nil {
		if writeErr != nil {
			return stageResult, fmt.Errorf("%w; write ingest stage result: %v", err, writeErr)
		}
		return stageResult, err
	}
	if writeErr != nil {
		return stageResult, writeErr
	}
	return stageResult, nil
}

func IngestStageJobResultPath(outputDir string) string {
	return filepath.Join(outputDir, "stages", "ingest-result.json")
}

func WriteIngestStageJobResult(outputDir string, result IngestStageJobResult) (string, error) {
	if outputDir == "" {
		return "", fmt.Errorf("overnight/ingest: output_dir is required")
	}
	path := IngestStageJobResultPath(outputDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("overnight/ingest: mkdir stage result dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("overnight/ingest: marshal stage result: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ingest-result-*.tmp")
	if err != nil {
		return "", fmt.Errorf("overnight/ingest: create stage result temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("overnight/ingest: write stage result: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("overnight/ingest: sync stage result: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("overnight/ingest: close stage result: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("overnight/ingest: rename stage result: %w", err)
	}
	cleanup = false
	return path, nil
}

func buildIngestStageJobResult(
	spec daemon.DreamStageJobSpec,
	outputDir string,
	started time.Time,
	completed time.Time,
	ingest *IngestResult,
	runErr error,
) IngestStageJobResult {
	status := "completed"
	if runErr != nil {
		status = "failed"
	}
	result := IngestStageJobResult{
		SchemaVersion:  1,
		DreamRunID:     spec.DreamRunID,
		Stage:          string(daemon.DreamStageIngest),
		Status:         status,
		OutputDir:      outputDir,
		StartedAt:      started.UTC().Format(time.RFC3339Nano),
		CompletedAt:    completed.UTC().Format(time.RFC3339Nano),
		DurationMillis: completed.Sub(started).Milliseconds(),
	}
	if ingest == nil {
		return result
	}
	result.HarvestPreviewCount = ingest.HarvestPreviewCount
	result.ForgeArtifactsMined = ingest.ForgeArtifactsMined
	result.ProvenanceAudited = ingest.ProvenanceAudited
	result.MineFindingsNew = ingest.MineFindingsNew
	result.GeneratorCandidateCount = ingest.GeneratorCandidateCount
	result.GeneratorDuplicateCount = ingest.GeneratorDuplicateCount
	result.GeneratorDuplicateRate = ingest.GeneratorDuplicateRate
	result.GeneratorSidecarCount = ingest.GeneratorSidecarCount
	result.GeneratorSoftFailCount = ingest.GeneratorSoftFailCount
	result.ExternalWatchlistEmitted = ingest.ExternalWatchlistEmitted
	result.GeneratorSidecarPaths = append([]string(nil), ingest.GeneratorSidecarPaths...)
	result.Degraded = append([]string(nil), ingest.Degraded...)
	result.StageFailures = cloneStringMap(ingest.StageFailures)
	if ingest.HarvestCatalog != nil {
		result.HarvestArtifactsExtracted = ingest.HarvestCatalog.Summary.ArtifactsExtracted
		result.HarvestPromotionCandidates = ingest.HarvestCatalog.Summary.PromotionCandidates
	}
	return result
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

type ingestRunner struct {
	ctx     context.Context
	opts    RunLoopOptions
	log     io.Writer
	result  *IngestResult
	started time.Time
}

const defaultFindingGeneratorTimeout = 2 * time.Minute

type findingGenerator struct {
	name       string
	sourceEpic string
	timeout    time.Duration
	run        func(context.Context, RunLoopOptions) FindingGeneratorSidecar
}

type findingGeneratorRunResult struct {
	name    string
	sidecar FindingGeneratorSidecar
	path    string
	err     error
}

func newIngestResult() *IngestResult {
	return &IngestResult{StageFailures: map[string]string{}}
}

func validateIngestInputs(opts RunLoopOptions) error {
	if opts.Cwd == "" {
		return fmt.Errorf("overnight: RunIngest requires RunLoopOptions.Cwd")
	}
	return nil
}

func (r *ingestRunner) runHarvestCatalog() error {
	if err := ctxCheck(r.ctx); err != nil {
		return err
	}
	fmt.Fprintln(r.log, "overnight/ingest: harvest discovery start")
	walkOpts := harvest.DefaultWalkOptions()
	walkOpts.Roots = []string{r.opts.Cwd}
	walkOpts.SkipGlobalHub = true

	rigs, err := harvest.DiscoverRigs(walkOpts)
	if err != nil {
		return fmt.Errorf("overnight/ingest: discover rigs: %w", err)
	}
	allArtifacts, err := r.extractHarvestArtifacts(rigs, walkOpts)
	if err != nil {
		return err
	}
	catalog := harvest.BuildCatalog(allArtifacts, 0.5)
	if catalog == nil {
		return fmt.Errorf("overnight/ingest: BuildCatalog returned nil")
	}
	r.result.HarvestCatalog = catalog
	fmt.Fprintf(r.log, "overnight/ingest: harvest catalog built: rigs=%d artifacts=%d promoted_candidates=%d\n",
		len(rigs), catalog.Summary.ArtifactsExtracted, catalog.Summary.PromotionCandidates)
	if catalog.Summary.ArtifactsExtracted == 0 {
		r.result.Degraded = append(r.result.Degraded,
			"harvest: empty corpus (no artifacts extracted from local .agents/)")
	}
	return nil
}

func (r *ingestRunner) extractHarvestArtifacts(
	rigs []harvest.RigInfo,
	walkOpts harvest.WalkOptions,
) ([]harvest.Artifact, error) {
	var allArtifacts []harvest.Artifact
	for _, rig := range rigs {
		if err := ctxCheck(r.ctx); err != nil {
			return nil, err
		}
		arts, warnings := harvest.ExtractArtifacts(rig, walkOpts)
		allArtifacts = append(allArtifacts, arts...)
		for _, w := range warnings {
			r.result.Degraded = append(r.result.Degraded,
				fmt.Sprintf("harvest warning %s/%s: %s", w.Rig, w.Stage, w.Message))
		}
	}
	return allArtifacts, nil
}

func (r *ingestRunner) runHarvestPreview() error {
	if err := ctxCheck(r.ctx); err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	promotionDest := filepath.Join(home, ".agents", "learnings")
	previewCount, previewErr := harvest.Promote(r.result.HarvestCatalog, promotionDest, true)
	if previewErr != nil {
		r.result.StageFailures["harvest-preview"] = previewErr.Error()
		r.result.Degraded = append(r.result.Degraded,
			fmt.Sprintf("harvest-preview: dry-run promote soft-failed: %v", previewErr))
		return nil
	}
	r.result.HarvestPreviewCount = previewCount
	fmt.Fprintf(r.log, "overnight/ingest: harvest dry-run promote preview count=%d\n", previewCount)
	return nil
}

func (r *ingestRunner) runForgeMine() error {
	if err := ctxCheck(r.ctx); err != nil {
		return err
	}
	forgeOpts := forge.MineOpts{
		SessionsDir: filepath.Join(r.opts.Cwd, ".agents", "sessions"),
		Quiet:       true,
	}
	minedReport, forgeErr := forge.RunMinePass(r.opts.Cwd, forgeOpts)
	if forgeErr != nil {
		r.result.StageFailures["forge-mine"] = forgeErr.Error()
		r.result.Degraded = append(r.result.Degraded, fmt.Sprintf("forge-mine: %v", forgeErr))
		return nil
	}
	if minedReport == nil {
		return nil
	}
	r.result.ForgeArtifactsMined = len(minedReport.Learnings)
	for _, d := range minedReport.Degraded {
		r.result.Degraded = append(r.result.Degraded, fmt.Sprintf("forge-mine: %s", d))
	}
	fmt.Fprintf(r.log, "overnight/ingest: forge-mine learnings=%d sessions_read=%d\n",
		len(minedReport.Learnings), minedReport.SessionsRead)
	return nil
}

func (r *ingestRunner) runProvenanceAudit() error {
	if err := ctxCheck(r.ctx); err != nil {
		return err
	}
	auditReport, auditErr := provenance.Audit(r.opts.Cwd)
	if auditErr != nil {
		r.result.StageFailures["provenance-audit"] = auditErr.Error()
		r.result.Degraded = append(r.result.Degraded, fmt.Sprintf("provenance-audit: %v", auditErr))
		return nil
	}
	if auditReport == nil {
		return nil
	}
	r.result.ProvenanceAudited = auditReport.StaleCitations + auditReport.MissingSources
	for _, d := range auditReport.Degraded {
		r.result.Degraded = append(r.result.Degraded, fmt.Sprintf("provenance-audit: %s", d))
	}
	fmt.Fprintf(r.log, "overnight/ingest: provenance-audit stale=%d missing=%d\n",
		auditReport.StaleCitations, auditReport.MissingSources)
	return nil
}

func (r *ingestRunner) runFindingGenerators() error {
	if err := ctxCheck(r.ctx); err != nil {
		return err
	}
	generators := r.findingGenerators()
	if len(generators) == 0 {
		return nil
	}
	results := make(chan findingGeneratorRunResult, len(generators))
	for _, generator := range generators {
		generator := generator
		go func() {
			results <- runFindingGenerator(r.ctx, r.opts, generator)
		}()
	}
	ordered := make([]findingGeneratorRunResult, 0, len(generators))
	for range generators {
		ordered = append(ordered, <-results)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].name < ordered[j].name })
	for _, result := range ordered {
		r.recordFindingGeneratorResult(result)
	}
	return nil
}

func (r *ingestRunner) findingGenerators() []findingGenerator {
	return []findingGenerator{
		{
			name:       mineFindingsGeneratorName,
			sourceEpic: "compile-mine",
			timeout:    defaultFindingGeneratorTimeout,
			run:        runMineFindingGenerator,
		},
		{
			name:       externalWatchlistGeneratorName,
			sourceEpic: externalWatchlistSourceEpic,
			timeout:    defaultFindingGeneratorTimeout, // RFC 0001 §253: ≤ 2 min per source
			run:        runExternalWatchlistGenerator,
		},
	}
}

func runFindingGenerator(
	ctx context.Context,
	opts RunLoopOptions,
	generator findingGenerator,
) findingGeneratorRunResult {
	timeout := generator.timeout
	if timeout <= 0 {
		timeout = defaultFindingGeneratorTimeout
	}
	genCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	done := make(chan FindingGeneratorSidecar, 1)
	go func() {
		done <- generator.run(genCtx, opts)
	}()

	var sidecar FindingGeneratorSidecar
	select {
	case sidecar = <-done:
	case <-genCtx.Done():
		sidecar = buildFailedFindingGeneratorSidecar(
			opts,
			generator.name,
			generator.sourceEpic,
			started,
			time.Now(),
			genCtx.Err(),
		)
	}
	path, err := writeFindingGeneratorSidecar(opts, sidecar)
	return findingGeneratorRunResult{
		name:    generator.name,
		sidecar: sidecar,
		path:    path,
		err:     err,
	}
}

func runMineFindingGenerator(ctx context.Context, opts RunLoopOptions) FindingGeneratorSidecar {
	started := time.Now()
	mineOpts := mine.RunOpts{
		Context:       ctx,
		Sources:       []string{"git", "agents", "code"},
		Window:        26 * time.Hour,
		OutputDir:     filepath.Join(opts.OutputDir, "mine-findings"),
		Quiet:         true,
		EmitWorkItems: false,
	}
	mineReport, mineErr := mine.Run(opts.Cwd, mineOpts)
	if mineErr != nil {
		return buildFailedMineGeneratorSidecar(opts, started, time.Now(), mineErr)
	}
	existingIDs, dedupErr := mine.LoadExistingMineIDs(filepath.Join(opts.Cwd, ".agents", "rpi", "next-work.jsonl"))
	if dedupErr != nil {
		return buildFailedMineGeneratorSidecar(opts, started, time.Now(), fmt.Errorf("mine-generator-dedup: %w", dedupErr))
	}
	return buildMineGeneratorSidecar(opts, mineReport, started, time.Now(), existingIDs)
}

func (r *ingestRunner) recordFindingGeneratorResult(runResult findingGeneratorRunResult) {
	sidecar := runResult.sidecar
	if sidecar.Status == "soft-fail" {
		r.result.GeneratorSoftFailCount++
		if sidecar.Error != "" {
			r.result.StageFailures[runResult.name] = sidecar.Error
			r.result.Degraded = append(r.result.Degraded, fmt.Sprintf("%s: %s", runResult.name, sidecar.Error))
		}
	}
	if runResult.err != nil {
		r.result.StageFailures[runResult.name+"-sidecar"] = runResult.err.Error()
		r.result.Degraded = append(r.result.Degraded, fmt.Sprintf("%s-sidecar: %v", runResult.name, runResult.err))
		return
	}
	r.result.GeneratorCandidateCount += sidecar.CandidateCount
	r.result.GeneratorDuplicateCount += sidecar.DuplicateCount
	if r.result.GeneratorCandidateCount > 0 {
		r.result.GeneratorDuplicateRate = float64(r.result.GeneratorDuplicateCount) / float64(r.result.GeneratorCandidateCount)
	}
	r.result.GeneratorSidecarCount++
	r.result.GeneratorSidecarPath = runResult.path
	r.result.GeneratorSidecarPaths = append(r.result.GeneratorSidecarPaths, runResult.path)
	if sidecar.Generator == mineFindingsGeneratorName {
		r.result.MineFindingsNew = sidecar.NewCandidateCount
	}
	if sidecar.Generator == externalWatchlistGeneratorName {
		r.result.ExternalWatchlistEmitted += sidecar.NewCandidateCount
	}
	fmt.Fprintf(r.log, "overnight/ingest: %s sidecar candidates=%d duplicates=%d path=%s\n",
		sidecar.Generator, sidecar.CandidateCount, sidecar.DuplicateCount, runResult.path)
}

// ctxCheck returns ctx.Err() if ctx has been cancelled, or nil otherwise.
// Used at substage boundaries so every exported stage function respects
// cancellation deterministically.
func ctxCheck(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// countMineFindings returns the total number of "new findings this pass"
// extractable from a mine.Report. The count sums code-complexity
// hotspots, orphaned research files, git co-change clusters, and git
// recurring-fix patterns — the surfaces mine.Run exposes as actionable
// signal. Zero-valued sub-reports (e.g. under DryRun) contribute zero.
func countMineFindings(r *mine.Report) int {
	if r == nil {
		return 0
	}
	var n int
	if r.Code != nil {
		n += len(r.Code.Hotspots)
	}
	if r.Agents != nil {
		n += len(r.Agents.OrphanedResearch)
	}
	if r.Git != nil {
		n += len(r.Git.TopCoChangeFiles)
		n += len(r.Git.RecurringFixes)
	}
	if r.Events != nil {
		n += len(r.Events.ErrorEvents)
		n += len(r.Events.GateVerdicts)
	}
	return n
}
