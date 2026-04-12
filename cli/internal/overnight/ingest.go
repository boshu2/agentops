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
	result := &IngestResult{
		StageFailures: map[string]string{},
	}

	if opts.Cwd == "" {
		return result, fmt.Errorf("overnight: RunIngest requires RunLoopOptions.Cwd")
	}

	// Substage 1: harvest discovery + extract + BuildCatalog.
	if err := ctxCheck(ctx); err != nil {
		return result, err
	}
	fmt.Fprintln(log, "overnight/ingest: harvest discovery start")
	walkOpts := harvest.DefaultWalkOptions()
	// Scope to this repo only — see 2026-04-09 dream-slice-substrate-before-surface
	// learning. We must not eat the global workspace in the private lane.
	walkOpts.Roots = []string{opts.Cwd}
	// Private-lane hermeticity: skip the automatic ~/.agents/ global hub
	// include so Dream's INGEST stage sees only this repo's corpus.
	walkOpts.SkipGlobalHub = true

	rigs, err := harvest.DiscoverRigs(walkOpts)
	if err != nil {
		return result, fmt.Errorf("overnight/ingest: discover rigs: %w", err)
	}

	var allArtifacts []harvest.Artifact
	for _, rig := range rigs {
		if err := ctxCheck(ctx); err != nil {
			return result, err
		}
		arts, warnings := harvest.ExtractArtifacts(rig, walkOpts)
		allArtifacts = append(allArtifacts, arts...)
		for _, w := range warnings {
			result.Degraded = append(result.Degraded,
				fmt.Sprintf("harvest warning %s/%s: %s", w.Rig, w.Stage, w.Message))
		}
	}

	catalog := harvest.BuildCatalog(allArtifacts, 0.5)
	if catalog == nil {
		return result, fmt.Errorf("overnight/ingest: BuildCatalog returned nil")
	}
	result.HarvestCatalog = catalog
	fmt.Fprintf(log, "overnight/ingest: harvest catalog built: rigs=%d artifacts=%d promoted_candidates=%d\n",
		len(rigs), catalog.Summary.ArtifactsExtracted, catalog.Summary.PromotionCandidates)

	if catalog.Summary.ArtifactsExtracted == 0 {
		result.Degraded = append(result.Degraded,
			"harvest: empty corpus (no artifacts extracted from local .agents/)")
	}

	// Substage 2: dry-run Promote for preview count.
	if err := ctxCheck(ctx); err != nil {
		return result, err
	}
	home, _ := os.UserHomeDir()
	promotionDest := filepath.Join(home, ".agents", "learnings")
	// NOTE: destination is a stub default; Wave 4 wires a caller-supplied
	// destination through RunLoopOptions.
	previewCount, previewErr := harvest.Promote(catalog, promotionDest, true)
	if previewErr != nil {
		result.StageFailures["harvest-preview"] = previewErr.Error()
		result.Degraded = append(result.Degraded,
			fmt.Sprintf("harvest-preview: dry-run promote soft-failed: %v", previewErr))
	} else {
		result.HarvestPreviewCount = previewCount
		fmt.Fprintf(log, "overnight/ingest: harvest dry-run promote preview count=%d\n", previewCount)
	}

	// Substage 3: forge mine pass (Wave 2 Issue 5 wiring — replaces the
	// Wave 3 stub that logged "deferred to follow-up"). Uses the
	// in-process entry forge.RunMinePass added in Wave 1.
	if err := ctxCheck(ctx); err != nil {
		return result, err
	}
	forgeOpts := forge.MineOpts{
		SessionsDir: filepath.Join(opts.Cwd, ".agents", "sessions"),
		Quiet:       true,
	}
	minedReport, forgeErr := forge.RunMinePass(opts.Cwd, forgeOpts)
	if forgeErr != nil {
		result.StageFailures["forge-mine"] = forgeErr.Error()
		result.Degraded = append(result.Degraded,
			fmt.Sprintf("forge-mine: %v", forgeErr))
	} else if minedReport != nil {
		result.ForgeArtifactsMined = len(minedReport.Learnings)
		for _, d := range minedReport.Degraded {
			result.Degraded = append(result.Degraded,
				fmt.Sprintf("forge-mine: %s", d))
		}
		fmt.Fprintf(log, "overnight/ingest: forge-mine learnings=%d sessions_read=%d\n",
			len(minedReport.Learnings), minedReport.SessionsRead)
	}

	// Substage 4: provenance audit (Wave 2 Issue 5 wiring — replaces the
	// Wave 3 stub). Uses provenance.Audit from Wave 1.
	if err := ctxCheck(ctx); err != nil {
		return result, err
	}
	auditReport, auditErr := provenance.Audit(opts.Cwd)
	if auditErr != nil {
		result.StageFailures["provenance-audit"] = auditErr.Error()
		result.Degraded = append(result.Degraded,
			fmt.Sprintf("provenance-audit: %v", auditErr))
	} else if auditReport != nil {
		result.ProvenanceAudited = auditReport.StaleCitations + auditReport.MissingSources
		for _, d := range auditReport.Degraded {
			result.Degraded = append(result.Degraded,
				fmt.Sprintf("provenance-audit: %s", d))
		}
		fmt.Fprintf(log, "overnight/ingest: provenance-audit stale=%d missing=%d\n",
			auditReport.StaleCitations, auditReport.MissingSources)
	}

	// Substage 5: ao mine drift/complexity (Wave 2 Issue 5 wiring —
	// replaces the Wave 3 stub). Uses mine.Run from Wave 1 with
	// DryRun=true so INGEST remains read-only (no dated report or
	// work-item emission). MineEventsFn is nil — INGEST does not drive
	// the events source; the nil callback is a silent no-op per Wave 1's
	// dependency-injection contract.
	if err := ctxCheck(ctx); err != nil {
		return result, err
	}
	mineOpts := mine.RunOpts{
		Sources: []string{"git", "agents", "code"},
		Quiet:   true,
		DryRun:  true,
	}
	mineReport, mineErr := mine.Run(opts.Cwd, mineOpts)
	if mineErr != nil {
		result.StageFailures["mine-findings"] = mineErr.Error()
		result.Degraded = append(result.Degraded,
			fmt.Sprintf("mine-findings: %v", mineErr))
	} else if mineReport != nil {
		result.MineFindingsNew = countMineFindings(mineReport)
		fmt.Fprintf(log, "overnight/ingest: mine-findings new=%d\n",
			result.MineFindingsNew)
	}

	result.Duration = stageDurationSince(started)
	fmt.Fprintf(log, "overnight/ingest: done in %s\n", result.Duration)
	return result, nil
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
