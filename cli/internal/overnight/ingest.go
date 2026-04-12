package overnight

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

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

// RunIngest executes the parallel-safe INGEST stage.
//
// RunIngest never mutates .agents/. It runs serial substages (no swarm,
// no goroutine fan-out in the first slice) and each substage degrades
// independently on failure. The stage as a whole only returns a non-nil
// error when the harvest catalog cannot be produced — that catalog is
// the load-bearing output REDUCE needs to run its real promotion pass.
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
//  5. mine.Run — in-process drift/complexity pass with DryRun=true so
//     INGEST remains read-only (Wave 2 Issue 5 wiring).
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
	if err := runner.runMineFindings(); err != nil {
		return result, err
	}

	result.Duration = stageDurationSince(started)
	fmt.Fprintf(log, "overnight/ingest: done in %s\n", result.Duration)
	return result, nil
}

type ingestRunner struct {
	ctx     context.Context
	opts    RunLoopOptions
	log     io.Writer
	result  *IngestResult
	started time.Time
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

func (r *ingestRunner) runMineFindings() error {
	if err := ctxCheck(r.ctx); err != nil {
		return err
	}
	mineOpts := mine.RunOpts{
		Sources: []string{"git", "agents", "code"},
		Quiet:   true,
		DryRun:  true,
	}
	mineReport, mineErr := mine.Run(r.opts.Cwd, mineOpts)
	if mineErr != nil {
		r.result.StageFailures["mine-findings"] = mineErr.Error()
		r.result.Degraded = append(r.result.Degraded, fmt.Sprintf("mine-findings: %v", mineErr))
		return nil
	}
	if mineReport != nil {
		r.result.MineFindingsNew = countMineFindings(mineReport)
		fmt.Fprintf(r.log, "overnight/ingest: mine-findings new=%d\n", r.result.MineFindingsNew)
	}
	return nil
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
