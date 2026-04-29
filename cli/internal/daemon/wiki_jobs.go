package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
	"github.com/boshu2/agentops/cli/internal/wikiworker"
)

const WikiJobSpecSchemaVersion = 1

type WikiForgeJobSpec struct {
	SchemaVersion int                    `json:"schema_version"`
	JobType       JobType                `json:"job_type"`
	DreamRunID    string                 `json:"dream_run_id,omitempty"`
	SourcePaths   []string               `json:"source_paths"`
	OutputDir     string                 `json:"output_dir"`
	WorkerKind    agentworker.WorkerKind `json:"worker_kind"`
	Provider      agentworker.Provider   `json:"provider"`
	Model         string                 `json:"model,omitempty"`
	CWD           string                 `json:"cwd,omitempty"`
	MaxAttempts   int                    `json:"max_attempts,omitempty"`
	QuarantineDir string                 `json:"quarantine_dir,omitempty"`
}

type WikiWorkerSessionRef struct {
	SourcePath string                    `json:"source_path"`
	Session    agentworker.SessionRef    `json:"session"`
	Terminal   agentworker.TerminalState `json:"terminal"`
}

type WikiForgeWorker interface {
	RunExtractionWithRetry(context.Context, wikiworker.ExtractionRequest, wikiworker.RetryOptions) (wikiworker.ExtractionResult, error)
}

type WikiForgeRunnerOptions struct {
	Queue         *Queue
	Worker        WikiForgeWorker
	Actor         string
	QuarantineDir string
}

type WikiForgeRunner struct {
	store         *Store
	queue         *Queue
	worker        WikiForgeWorker
	actor         string
	quarantineDir string
}

type WikiForgeJobRunResult struct {
	JobID          string                 `json:"job_id"`
	DreamRunID     string                 `json:"dream_run_id,omitempty"`
	Status         JobStatus              `json:"status"`
	Artifacts      map[string]string      `json:"artifacts,omitempty"`
	WorkerSessions []WikiWorkerSessionRef `json:"worker_sessions,omitempty"`
	Failure        *JobFailure            `json:"failure,omitempty"`
}

func NewWikiForgeJobSpec(dreamRunID, outputDir string, sourcePaths []string) WikiForgeJobSpec {
	return WikiForgeJobSpec{
		SchemaVersion: WikiJobSpecSchemaVersion,
		JobType:       JobTypeWikiForge,
		DreamRunID:    dreamRunID,
		SourcePaths:   append([]string{}, sourcePaths...),
		OutputDir:     outputDir,
		WorkerKind:    agentworker.WorkerKindCodex,
		Provider:      agentworker.ProviderGasCity,
		MaxAttempts:   2,
	}
}

func (spec WikiForgeJobSpec) Validate() error {
	if spec.SchemaVersion != WikiJobSpecSchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", spec.SchemaVersion, WikiJobSpecSchemaVersion)
	}
	if spec.JobType != JobTypeWikiForge {
		return fmt.Errorf("job_type = %q, want %q", spec.JobType, JobTypeWikiForge)
	}
	if len(spec.SourcePaths) == 0 {
		return fmt.Errorf("source_paths are required")
	}
	for i, path := range spec.SourcePaths {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("source_paths[%d] is empty", i)
		}
	}
	if strings.TrimSpace(spec.OutputDir) == "" {
		return fmt.Errorf("output_dir is required")
	}
	switch spec.WorkerKind {
	case agentworker.WorkerKindCodex, agentworker.WorkerKindClaude:
	default:
		return fmt.Errorf("unsupported worker_kind %q", spec.WorkerKind)
	}
	if spec.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if spec.MaxAttempts < 0 {
		return fmt.Errorf("max_attempts must be >= 0")
	}
	return nil
}

func (spec WikiForgeJobSpec) ToJobSpec(jobID string) (JobSpec, error) {
	if err := spec.Validate(); err != nil {
		return JobSpec{}, err
	}
	payload, err := structToMap(spec)
	if err != nil {
		return JobSpec{}, err
	}
	return JobSpec{ID: jobID, Type: JobTypeWikiForge, Payload: payload}, nil
}

func WikiForgeJobSpecFromPayload(payload map[string]any) (WikiForgeJobSpec, error) {
	var spec WikiForgeJobSpec
	if err := mapToStruct(payload, &spec); err != nil {
		return spec, err
	}
	if err := spec.Validate(); err != nil {
		return spec, err
	}
	return spec, nil
}

func NewWikiForgeRunner(store *Store, opts WikiForgeRunnerOptions) (*WikiForgeRunner, error) {
	if store == nil {
		return nil, fmt.Errorf("wiki forge runner: store is required")
	}
	if opts.Worker == nil {
		return nil, fmt.Errorf("wiki forge runner: worker is required")
	}
	queue := opts.Queue
	if queue == nil {
		queue = NewQueue(store, QueueOptions{})
	}
	actor := strings.TrimSpace(opts.Actor)
	if actor == "" {
		actor = "agentopsd-wiki"
	}
	return &WikiForgeRunner{
		store:         store,
		queue:         queue,
		worker:        opts.Worker,
		actor:         actor,
		quarantineDir: opts.QuarantineDir,
	}, nil
}

func (r *WikiForgeRunner) RunWikiForgeJob(ctx context.Context, jobID string) (WikiForgeJobRunResult, error) {
	claim, err := r.queue.ClaimJob(jobID, r.actor, QueueMutationOptions{})
	if err != nil {
		return WikiForgeJobRunResult{}, err
	}
	return r.runClaimedWikiForgeJob(ctx, claim)
}

func (r *WikiForgeRunner) runClaimedWikiForgeJob(ctx context.Context, claim QueueClaim) (WikiForgeJobRunResult, error) {
	if claim.Job.JobType != JobTypeWikiForge {
		return WikiForgeJobRunResult{}, fmt.Errorf("job %s type %s is not %s", claim.Job.JobID, claim.Job.JobType, JobTypeWikiForge)
	}
	spec, err := WikiForgeJobSpecFromPayload(claim.Job.Payload)
	if err != nil {
		return r.failWikiForgeJob(claim, JobFailure{Code: FailureRequestRejected, Message: err.Error()}), err
	}

	refs := make([]WikiWorkerSessionRef, 0, len(spec.SourcePaths))
	for _, sourcePath := range spec.SourcePaths {
		result, err := r.worker.RunExtractionWithRetry(ctx, wikiworker.ExtractionRequest{
			Prompt:    wikiForgePrompt(sourcePath),
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
			QuarantineDir: firstNonEmptyString(spec.QuarantineDir, r.quarantineDir, filepath.Join(r.store.root, ".agents", "quarantine", "agentworker")),
		})
		if err != nil {
			failure := JobFailure{Code: FailureRetryExhausted, Message: err.Error(), Retryable: false}
			return r.failWikiForgeJob(claim, failure), err
		}
		refs = append(refs, WikiWorkerSessionRef{
			SourcePath: sourcePath,
			Session:    result.Session,
			Terminal:   result.Terminal,
		})
	}
	refsPath, err := writeWikiWorkerSessionRefs(r.store.root, claim.Job.JobID, refs)
	if err != nil {
		failure := JobFailure{Code: FailureRequestRejected, Message: err.Error(), Retryable: false}
		return r.failWikiForgeJob(claim, failure), err
	}
	job, err := r.queue.CompleteJob(CompleteJobInput{
		JobID:      claim.Job.JobID,
		RequestID:  RequestID(claim.Job.RequestID),
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      r.actor,
		Artifacts:  map[string]string{"worker_session_refs": refsPath},
	}, QueueMutationOptions{})
	if err != nil {
		return WikiForgeJobRunResult{}, err
	}
	return WikiForgeJobRunResult{
		JobID:          job.JobID,
		DreamRunID:     spec.DreamRunID,
		Status:         job.Status,
		Artifacts:      job.Artifacts,
		WorkerSessions: refs,
	}, nil
}

func (r *WikiForgeRunner) failWikiForgeJob(claim QueueClaim, failure JobFailure) WikiForgeJobRunResult {
	job, err := r.queue.FailJob(FailJobInput{
		JobID:      claim.Job.JobID,
		RequestID:  RequestID(claim.Job.RequestID),
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      r.actor,
		Failure:    failure,
	}, QueueMutationOptions{})
	if err != nil {
		return WikiForgeJobRunResult{JobID: claim.Job.JobID, Status: claim.Job.Status, Failure: &failure}
	}
	return WikiForgeJobRunResult{JobID: job.JobID, Status: job.Status, Failure: job.Failure}
}

func wikiForgePrompt(sourcePath string) string {
	return "Extract reusable AgentOps wiki knowledge from Claude/Codex transcript: " + sourcePath
}

func writeWikiWorkerSessionRefs(root, jobID string, refs []WikiWorkerSessionRef) (string, error) {
	dir := filepath.Join(root, ".agents", "daemon", "wiki")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, sanitizeWikiArtifactName(jobID)+"-worker-sessions.json")
	data, err := json.MarshalIndent(struct {
		SchemaVersion  int                    `json:"schema_version"`
		JobID          string                 `json:"job_id"`
		WorkerSessions []WikiWorkerSessionRef `json:"worker_sessions"`
		WrittenAt      string                 `json:"written_at"`
	}{
		SchemaVersion:  1,
		JobID:          jobID,
		WorkerSessions: refs,
		WrittenAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return path, nil
}

func sanitizeWikiArtifactName(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "wiki-forge"
	}
	return out
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
