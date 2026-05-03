package daemon

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/boshu2/agentops/cli/internal/wikiworker"
)

type WikiForgeExecutorOptions struct {
	Store         *Store
	Worker        WikiForgeWorker
	QuarantineDir string
}

type WikiForgeExecutor struct {
	store         *Store
	worker        WikiForgeWorker
	quarantineDir string
}

func NewWikiForgeExecutor(opts WikiForgeExecutorOptions) (*WikiForgeExecutor, error) {
	if opts.Store == nil {
		return nil, fmt.Errorf("wiki forge executor: store is required")
	}
	if opts.Worker == nil {
		return nil, fmt.Errorf("wiki forge executor: worker is required")
	}
	return &WikiForgeExecutor{
		store:         opts.Store,
		worker:        opts.Worker,
		quarantineDir: opts.QuarantineDir,
	}, nil
}

func (e *WikiForgeExecutor) JobTypes() []JobType {
	return []JobType{JobTypeWikiForge}
}

func (e *WikiForgeExecutor) RunJob(ctx context.Context, claim QueueClaim) (JobExecutionResult, error) {
	// Honor cancellation issued mid-claim before performing any work
	// (payload parse, source expansion, or worker spawn). Without this
	// check, a context canceled between QueueClaim and execution start
	// is not observed until the underlying call blocks. See bd soc-58q5.12.
	if err := ctx.Err(); err != nil {
		return JobExecutionResult{}, err
	}
	if claim.Job.JobType != JobTypeWikiForge {
		return JobExecutionResult{}, fmt.Errorf("wiki forge executor does not support job type %s", claim.Job.JobType)
	}
	spec, err := WikiForgeJobSpecFromPayload(claim.Job.Payload)
	if err != nil {
		return JobExecutionResult{}, err
	}
	if err := validateWikiForgeSourcePathsContainment(e.store.root, spec.SourcePaths); err != nil {
		return JobExecutionResult{}, err
	}
	expandedSources, err := expandWikiForgeSourcePaths(spec.SourcePaths)
	if err != nil {
		return JobExecutionResult{}, err
	}
	if err := validateWikiForgeSourcePathsContainment(e.store.root, expandedSources); err != nil {
		return JobExecutionResult{}, err
	}
	refs := make([]WikiWorkerSessionRef, 0, len(expandedSources))
	for _, sourcePath := range expandedSources {
		// Re-check cancellation between worker invocations — a cancel
		// signaled while a previous source was processing should stop
		// the next spawn instead of silently continuing.
		if err := ctx.Err(); err != nil {
			return JobExecutionResult{}, err
		}
		promptCtx, err := newWikiForgePromptContext(claim, spec, sourcePath)
		if err != nil {
			return JobExecutionResult{}, err
		}
		result, err := e.worker.RunExtractionWithRetry(ctx, wikiworker.ExtractionRequest{
			Prompt:    wikiForgePrompt(promptCtx),
			JobID:     claim.Job.JobID,
			AttemptID: fmt.Sprintf("%d", claim.Job.Attempt),
			RequestID: claim.Job.RequestID,
			Worker:    spec.WorkerKind,
			Provider:  spec.Provider,
			Model:     spec.Model,
			CWD:       spec.CWD,
			Metadata: map[string]string{
				"title":         "wiki forge " + filepath.Base(sourcePath),
				"source_path":   sourcePath,
				"dream_run_id":  spec.DreamRunID,
				"daemon_job_id": claim.Job.JobID,
			},
		}, wikiworker.RetryOptions{
			MaxAttempts:   firstPositive(spec.MaxAttempts, 1),
			QuarantineDir: firstNonEmptyString(spec.QuarantineDir, e.quarantineDir, filepath.Join(e.store.root, ".agents", "quarantine", "agentworker")),
		})
		if err != nil {
			return JobExecutionResult{Artifacts: wikiForgeFailureArtifacts(result, err)}, err
		}
		refs = append(refs, WikiWorkerSessionRef{
			SourcePath: sourcePath,
			Session:    result.Session,
			Terminal:   result.Terminal,
		})
	}
	refsArtifact, err := writeWikiWorkerSessionRefs(e.store.root, claim.Job.JobID, refs)
	if err != nil {
		return JobExecutionResult{}, err
	}
	return JobExecutionResult{
		Artifacts:    wikiForgeSuccessArtifacts(refs),
		ArtifactRefs: wikiForgeSuccessArtifactRefs(refsArtifact),
	}, nil
}

func wikiForgeSuccessArtifacts(refs []WikiWorkerSessionRef) map[string]string {
	if len(refs) == 0 {
		return nil
	}
	artifacts := map[string]string{}
	last := refs[len(refs)-1]
	artifacts["provider_request_id"] = last.Session.ProviderRequestID
	artifacts["session_id"] = last.Session.SessionID
	artifacts["event_cursor"] = last.Session.EventCursor
	artifacts["terminal_status"] = string(last.Terminal.Status)
	return artifacts
}

func wikiForgeSuccessArtifactRefs(refsArtifact ArtifactRef) map[string]ArtifactRef {
	return map[string]ArtifactRef{"worker_session_refs": refsArtifact}
}

func wikiForgeFailureArtifacts(result wikiworker.ExtractionResult, err error) map[string]string {
	artifacts := map[string]string{}
	if result.Session.SessionID != "" {
		artifacts["provider_request_id"] = result.Session.ProviderRequestID
		artifacts["session_id"] = result.Session.SessionID
		artifacts["event_cursor"] = result.Session.EventCursor
	}
	if result.Terminal.Status != "" {
		artifacts["terminal_status"] = string(result.Terminal.Status)
	}
	var quarantineErr *wikiworker.QuarantineError
	if errors.As(err, &quarantineErr) {
		artifacts["quarantine_path"] = quarantineErr.Path
	}
	if len(artifacts) == 0 {
		return nil
	}
	return artifacts
}
