package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DreamRunLoopOptions struct {
	Cwd           string
	OutputDir     string
	RunID         string
	MaxIterations int
	WarnOnly      bool
	LogWriter     io.Writer
}

type DreamRunLoopResult struct {
	Raw             any  `json:"raw,omitempty"`
	IterationCount  int  `json:"iteration_count"`
	BudgetExhausted bool `json:"budget_exhausted"`
}

type DreamRunLoopFunc func(context.Context, DreamRunLoopOptions) (DreamRunLoopResult, error)

type DreamExecutorOptions struct {
	Cwd     string
	RunLoop DreamRunLoopFunc
	Now     func() time.Time
}

type DreamExecutor struct {
	cwd     string
	runLoop DreamRunLoopFunc
	now     func() time.Time
}

func NewDreamExecutor(opts DreamExecutorOptions) (*DreamExecutor, error) {
	if strings.TrimSpace(opts.Cwd) == "" {
		return nil, errors.New("dream executor: cwd is required")
	}
	runLoop := opts.RunLoop
	if runLoop == nil {
		return nil, errors.New("dream executor: run loop is required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &DreamExecutor{cwd: opts.Cwd, runLoop: runLoop, now: now}, nil
}

func (e *DreamExecutor) JobTypes() []JobType {
	return []JobType{JobTypeDreamRun}
}

// RunJob requires a non-nil ctx; callers passing nil will panic on first use.
func (e *DreamExecutor) RunJob(ctx context.Context, claim QueueClaim) (JobExecutionResult, error) {
	if claim.Job.JobType != JobTypeDreamRun {
		return JobExecutionResult{}, fmt.Errorf("dream executor does not support job type %s", claim.Job.JobType)
	}
	spec, err := DreamRunJobSpecFromPayload(claim.Job.Payload)
	if err != nil {
		return JobExecutionResult{}, err
	}
	artifacts := dreamRunArtifacts(spec.OutputDir)
	// soc-58q5.13 (W-C-18): refuse to traverse a symlink at the planned
	// output_dir. The path is operator-supplied via the job payload, so an
	// attacker who can pre-create a symlink at OutputDir could redirect
	// summary/log writes outside the intended sandbox. Lstat (not Stat) so we
	// see the symlink itself rather than its target.
	if info, err := os.Lstat(spec.OutputDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return JobExecutionResult{Artifacts: artifacts}, fmt.Errorf("dream output_dir is a symlink: %s", spec.OutputDir)
		}
	}
	// soc-58q5.13 (W-C-18): 0o700 (not 0o755) keeps overnight log + summary
	// readable only by the daemon's UID. Per-run dirs hold goal-bearing
	// content the operator may want kept private from other local users.
	if err := os.MkdirAll(spec.OutputDir, 0o700); err != nil {
		return JobExecutionResult{Artifacts: artifacts}, fmt.Errorf("create dream output_dir: %w", err)
	}
	startedAt := e.now().UTC()
	// soc-5of.9: O_APPEND (not O_TRUNC) so a daemon restart mid-dream cannot
	// truncate partial logs — Fournier-class "crash with notes" durability nit.
	// soc-58q5.14 (W-C-19): 0o600 (not 0o644) keeps the per-run overnight log
	// readable only by the daemon's UID. Logs may capture provider tokens or
	// other credentials emitted by tooling; world-readable is a leak path to
	// other local users.
	logFile, err := os.OpenFile(artifacts["overnight_log"], os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return JobExecutionResult{Artifacts: artifacts}, fmt.Errorf("open dream log: %w", err)
	}
	defer func() { _ = logFile.Close() }()

	runCtx, cancel, timeout, err := dreamRunExecutionContext(ctx, spec.ExecutionTimeout)
	if err != nil {
		return JobExecutionResult{Artifacts: artifacts}, err
	}
	defer cancel()

	result, runErr := e.runLoop(runCtx, DreamRunLoopOptions{
		Cwd:           e.cwd,
		OutputDir:     spec.OutputDir,
		RunID:         spec.DreamRunID,
		MaxIterations: spec.MaxIterations,
		WarnOnly:      true,
		LogWriter:     logFile,
	})
	if timeout > 0 && runCtx.Err() != nil && errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		runErr = fmt.Errorf("dream execution timeout after %s", timeout)
	}
	finishedAt := e.now().UTC()
	status := "completed"
	if runErr != nil {
		status = "failed"
	}
	summary := dreamExecutorSummary{
		SchemaVersion: 1,
		JobID:         claim.Job.JobID,
		DreamRunID:    spec.DreamRunID,
		Goal:          spec.Goal,
		Status:        status,
		OutputDir:     spec.OutputDir,
		StartedAt:     startedAt.Format(time.RFC3339Nano),
		FinishedAt:    finishedAt.Format(time.RFC3339Nano),
		Duration:      finishedAt.Sub(startedAt).Round(time.Millisecond).String(),
		Result:        result,
	}
	if runErr != nil {
		summary.Failure = runErr.Error()
		if err := writeDreamFailureReport(artifacts["failure_report"], summary); err != nil {
			return JobExecutionResult{Artifacts: artifacts}, err
		}
	} else {
		delete(artifacts, "failure_report")
	}
	if err := writeDreamExecutorSummary(artifacts, summary); err != nil {
		return JobExecutionResult{Artifacts: artifacts}, err
	}
	return JobExecutionResult{Artifacts: artifacts}, runErr
}

type dreamExecutorSummary struct {
	SchemaVersion int                `json:"schema_version"`
	JobID         string             `json:"job_id"`
	DreamRunID    string             `json:"dream_run_id"`
	Goal          string             `json:"goal,omitempty"`
	Status        string             `json:"status"`
	OutputDir     string             `json:"output_dir"`
	StartedAt     string             `json:"started_at"`
	FinishedAt    string             `json:"finished_at"`
	Duration      string             `json:"duration"`
	Failure       string             `json:"failure,omitempty"`
	Result        DreamRunLoopResult `json:"result"`
}

func dreamRunArtifacts(outputDir string) map[string]string {
	return map[string]string{
		"summary_json":     filepath.Join(outputDir, "summary.json"),
		"summary_markdown": filepath.Join(outputDir, "summary.md"),
		"overnight_log":    filepath.Join(outputDir, "overnight.log"),
		"failure_report":   filepath.Join(outputDir, "failure-report.md"),
	}
}

func dreamRunExecutionContext(ctx context.Context, rawTimeout string) (context.Context, context.CancelFunc, time.Duration, error) {
	if strings.TrimSpace(rawTimeout) == "" {
		return ctx, func() {}, 0, nil
	}
	timeout, err := time.ParseDuration(rawTimeout)
	if err != nil {
		return ctx, func() {}, 0, fmt.Errorf("parse dream execution timeout: %w", err)
	}
	if timeout <= 0 {
		return ctx, func() {}, 0, errors.New("dream execution timeout must be > 0")
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	return runCtx, cancel, timeout, nil
}

func writeDreamExecutorSummary(artifacts map[string]string, summary dreamExecutorSummary) error {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(artifacts["summary_json"], append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write dream summary json: %w", err)
	}
	return os.WriteFile(artifacts["summary_markdown"], []byte(renderDreamExecutorSummary(summary)), 0o644)
}

func writeDreamFailureReport(path string, summary dreamExecutorSummary) error {
	body := fmt.Sprintf("# Dream daemon failure\n\nrun: %s\njob: %s\nfailure: %s\n", summary.DreamRunID, summary.JobID, summary.Failure)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write dream failure report: %w", err)
	}
	return nil
}

func renderDreamExecutorSummary(summary dreamExecutorSummary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Dream daemon run\n\n")
	fmt.Fprintf(&b, "- run: %s\n", summary.DreamRunID)
	fmt.Fprintf(&b, "- job: %s\n", summary.JobID)
	fmt.Fprintf(&b, "- status: %s\n", summary.Status)
	if summary.Failure != "" {
		fmt.Fprintf(&b, "- failure: %s\n", summary.Failure)
	}
	fmt.Fprintf(&b, "- iterations: %d\n", summary.Result.IterationCount)
	fmt.Fprintf(&b, "- budget_exhausted: %t\n", summary.Result.BudgetExhausted)
	_, _ = io.WriteString(&b, "\n")
	return b.String()
}
