// practices: [microservices, sre]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/boshu2/agentops/cli/internal/openclaw"
	"github.com/spf13/cobra"
)

var (
	daemonSoakScenario        string
	daemonSoakDuration        time.Duration
	daemonSoakInterval        time.Duration
	daemonSoakRequireTerminal bool
)

var daemonSoakCmd = &cobra.Command{
	Use:   "soak",
	Short: "Run a daemon product proof scenario",
	Args:  cobra.NoArgs,
	RunE:  runAgentOpsDaemonSoakCommand,
}

type daemonSoakOptions struct {
	Scenario        string        `json:"scenario"`
	Duration        time.Duration `json:"-"`
	Interval        time.Duration `json:"-"`
	RequireTerminal bool          `json:"require_terminal"`
	RunID           string        `json:"run_id,omitempty"`
	Now             func() time.Time
}

type daemonSoakScenarioRecord struct {
	SchemaVersion   int    `json:"schema_version"`
	RunID           string `json:"run_id"`
	Scenario        string `json:"scenario"`
	Duration        string `json:"duration"`
	Interval        string `json:"interval"`
	RequireTerminal bool   `json:"require_terminal"`
	StartedAt       string `json:"started_at"`
}

type daemonSoakProofPaths struct {
	ScenarioJSON string `json:"scenario_json"`
	EventsJSONL  string `json:"events_jsonl"`
	ReportJSON   string `json:"soak_report_json"`
	SummaryMD    string `json:"summary_markdown"`
}

type daemonSoakReport struct {
	SchemaVersion   int                        `json:"schema_version"`
	RunID           string                     `json:"run_id"`
	Scenario        string                     `json:"scenario"`
	Status          string                     `json:"status"`
	StartedAt       string                     `json:"started_at"`
	FinishedAt      string                     `json:"finished_at"`
	Duration        string                     `json:"duration"`
	RequireTerminal bool                       `json:"require_terminal"`
	Jobs            []daemonpkg.QueueJobState  `json:"jobs"`
	OpenClawJobs    []openclaw.ResourceSummary `json:"openclaw_jobs"`
	EventCount      int                        `json:"event_count"`
	Proof           daemonSoakProofPaths       `json:"proof"`
	Failure         string                     `json:"failure,omitempty"`
}

func init() {
	daemonCmd.AddCommand(daemonSoakCmd)
	daemonSoakCmd.Flags().StringVar(&daemonSoakScenario, "scenario", "queue-only", "Soak scenario (queue-only, fake-executor, dream, plans-projection)")
	daemonSoakCmd.Flags().DurationVar(&daemonSoakDuration, "duration", 2*time.Minute, "Maximum scenario duration")
	daemonSoakCmd.Flags().DurationVar(&daemonSoakInterval, "interval", 15*time.Second, "Polling interval for scenario checks")
	daemonSoakCmd.Flags().BoolVar(&daemonSoakRequireTerminal, "require-terminal", false, "Fail unless scenario jobs reach terminal daemon state")
}

func runAgentOpsDaemonSoakCommand(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	report, err := runDaemonSoak(cobraContext(cmd), cwd, daemonSoakOptions{
		Scenario:        daemonSoakScenario,
		Duration:        daemonSoakDuration,
		Interval:        daemonSoakInterval,
		RequireTerminal: daemonSoakRequireTerminal,
	})
	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if encodeErr := enc.Encode(report); encodeErr != nil && err == nil {
			err = encodeErr
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "daemon soak %s: %s\nproof: %s\n", report.Scenario, report.Status, report.Proof.ReportJSON)
	}
	return err
}

func runDaemonSoak(ctx context.Context, cwd string, opts daemonSoakOptions) (daemonSoakReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = normalizeDaemonSoakOptions(opts)
	startedAt := opts.Now().UTC()
	proofDir := filepath.Join(cwd, ".agents", "daemon", "soaks", opts.RunID)
	proof := daemonSoakProofPaths{
		ScenarioJSON: filepath.Join(proofDir, "scenario.json"),
		EventsJSONL:  filepath.Join(proofDir, "events.jsonl"),
		ReportJSON:   filepath.Join(proofDir, "soak-report.json"),
		SummaryMD:    filepath.Join(proofDir, "summary.md"),
	}
	if err := os.MkdirAll(proofDir, 0o755); err != nil {
		return daemonSoakReport{}, err
	}
	record := daemonSoakScenarioRecord{
		SchemaVersion:   1,
		RunID:           opts.RunID,
		Scenario:        opts.Scenario,
		Duration:        opts.Duration.String(),
		Interval:        opts.Interval.String(),
		RequireTerminal: opts.RequireTerminal,
		StartedAt:       startedAt.Format(time.RFC3339Nano),
	}
	if err := writeDaemonSoakJSONFile(proof.ScenarioJSON, record); err != nil {
		return daemonSoakReport{}, err
	}

	store := daemonpkg.NewStore(cwd)
	queue := daemonpkg.NewQueue(store, daemonpkg.QueueOptions{})
	jobIDs, runErr := runDaemonSoakScenario(ctx, cwd, queue, opts, proofDir)
	events, eventsErr := store.ReadLedger()
	if eventsErr != nil && runErr == nil {
		runErr = eventsErr
	}
	filteredEvents := filterDaemonSoakEvents(events, jobIDs)
	if err := writeDaemonSoakEvents(proof.EventsJSONL, filteredEvents); err != nil && runErr == nil {
		runErr = err
	}
	snapshot, snapshotErr := queue.Snapshot()
	if snapshotErr != nil && runErr == nil {
		runErr = snapshotErr
	}
	jobs := filterDaemonSoakJobs(snapshot.Jobs, jobIDs)
	openClawJobs, ocErr := readOpenClawJobsFromStore(store)
	if ocErr != nil && runErr == nil {
		runErr = ocErr
	}
	openClawJobs = filterOpenClawJobs(openClawJobs, jobIDs)
	if terminalErr := validateDaemonSoakTerminal(opts, jobs); terminalErr != nil && runErr == nil {
		runErr = terminalErr
	}
	finishedAt := opts.Now().UTC()
	report := daemonSoakReport{
		SchemaVersion:   1,
		RunID:           opts.RunID,
		Scenario:        opts.Scenario,
		Status:          "pass",
		StartedAt:       startedAt.Format(time.RFC3339Nano),
		FinishedAt:      finishedAt.Format(time.RFC3339Nano),
		Duration:        finishedAt.Sub(startedAt).Round(time.Millisecond).String(),
		RequireTerminal: opts.RequireTerminal,
		Jobs:            jobs,
		OpenClawJobs:    openClawJobs,
		EventCount:      len(filteredEvents),
		Proof:           proof,
	}
	if runErr != nil {
		report.Status = "fail"
		report.Failure = runErr.Error()
	}
	if err := writeDaemonSoakJSONFile(proof.ReportJSON, report); err != nil && runErr == nil {
		runErr = err
	}
	if err := os.WriteFile(proof.SummaryMD, []byte(renderDaemonSoakSummary(report)), 0o644); err != nil && runErr == nil {
		runErr = err
	}
	return report, runErr
}

func normalizeDaemonSoakOptions(opts daemonSoakOptions) daemonSoakOptions {
	opts.Scenario = strings.TrimSpace(opts.Scenario)
	if opts.Scenario == "" {
		opts.Scenario = "queue-only"
	}
	if opts.Duration <= 0 {
		opts.Duration = 2 * time.Minute
	}
	if opts.Interval <= 0 {
		opts.Interval = 15 * time.Second
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if strings.TrimSpace(opts.RunID) == "" {
		opts.RunID = "soak-" + opts.Now().UTC().Format("20060102T150405Z")
	}
	return opts
}

func runDaemonSoakScenario(ctx context.Context, cwd string, queue *daemonpkg.Queue, opts daemonSoakOptions, proofDir string) ([]string, error) {
	switch opts.Scenario {
	case "queue-only":
		jobID := "job-" + opts.RunID + "-queue"
		_, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
			RequestID:      daemonpkg.RequestID("req-" + opts.RunID + "-queue"),
			JobID:          jobID,
			JobType:        daemonpkg.JobTypeOpenClawSnapshot,
			IdempotencyKey: "daemon-soak:" + opts.RunID + ":queue",
			Actor:          "ao daemon soak",
			Payload:        map[string]any{"scenario": opts.Scenario, "run_id": opts.RunID},
		}, daemonpkg.QueueMutationOptions{})
		return []string{jobID}, err
	case "fake-executor":
		jobID := "job-" + opts.RunID + "-fake"
		if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
			RequestID:      daemonpkg.RequestID("req-" + opts.RunID + "-fake"),
			JobID:          jobID,
			JobType:        daemonpkg.JobTypeOpenClawSnapshot,
			IdempotencyKey: "daemon-soak:" + opts.RunID + ":fake",
			Actor:          "ao daemon soak",
			Payload:        map[string]any{"scenario": opts.Scenario, "run_id": opts.RunID},
		}, daemonpkg.QueueMutationOptions{}); err != nil {
			return []string{jobID}, err
		}
		executor := daemonFakeOpenClawSnapshotExecutor{Artifacts: map[string]string{
			"soak_run_id": opts.RunID,
			"soak_report": filepath.Join(proofDir, "soak-report.json"),
		}}
		return []string{jobID}, runDaemonSoakClaimedJob(ctx, queue, jobID, executor)
	case "dream":
		jobID := "job-" + opts.RunID + "-dream"
		spec := daemonpkg.NewDreamRunJobSpec(opts.RunID+"-dream", filepath.Join(cwd, ".agents", "overnight", opts.RunID+"-dream"))
		spec.Goal = "daemon soak dream proof"
		spec.MaxIterations = 1
		spec.ExecutionTimeout = opts.Duration.String()
		job, err := spec.ToJobSpec(jobID)
		if err != nil {
			return []string{jobID}, err
		}
		if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
			RequestID:      daemonpkg.RequestID("req-" + opts.RunID + "-dream"),
			JobID:          job.ID,
			JobType:        job.Type,
			IdempotencyKey: "daemon-soak:" + opts.RunID + ":dream",
			Actor:          "ao daemon soak",
			Payload:        job.Payload,
		}, daemonpkg.QueueMutationOptions{}); err != nil {
			return []string{jobID}, err
		}
		executor, err := buildAgentOpsDaemonDreamExecutor(cwd)
		if err != nil {
			return []string{jobID}, err
		}
		return []string{jobID}, runDaemonSoakClaimedJob(ctx, queue, jobID, executor)
	case "plans-projection":
		jobID := "job-" + opts.RunID + "-plans"
		outputDir := filepath.Join(cwd, ".agents", "plans", "soak", opts.RunID)
		spec := daemonpkg.NewPlansProjectionJobSpec("soak-project", "soc", outputDir)
		specRaw, err := json.Marshal(spec)
		if err != nil {
			return []string{jobID}, err
		}
		var payload map[string]any
		if err := json.Unmarshal(specRaw, &payload); err != nil {
			return []string{jobID}, err
		}
		if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
			RequestID:      daemonpkg.RequestID("req-" + opts.RunID + "-plans"),
			JobID:          jobID,
			JobType:        daemonpkg.JobTypePlansProjection,
			IdempotencyKey: "daemon-soak:" + opts.RunID + ":plans",
			Actor:          "ao daemon soak",
			Payload:        payload,
		}, daemonpkg.QueueMutationOptions{}); err != nil {
			return []string{jobID}, err
		}
		// Soak scenario uses an in-process fake bd source that returns a small
		// deterministic epic set so the executor exercises the full
		// query → projection → atomic-write path without depending on Dolt.
		bdSource := plansProjectionSoakBdSource{}
		executor, err := daemonpkg.NewPlansProjectionExecutor(daemonpkg.PlansProjectionExecutorOptions{
			Store:    daemonpkg.NewStore(cwd),
			BdSource: bdSource,
			Now:      opts.Now,
		})
		if err != nil {
			return []string{jobID}, err
		}
		return []string{jobID}, runDaemonSoakClaimedJob(ctx, queue, jobID, executor)
	default:
		return nil, fmt.Errorf("unsupported daemon soak scenario %q", opts.Scenario)
	}
}

// plansProjectionSoakBdSource is the in-process bd source used by the
// plans-projection soak scenario. Returns a fixed 2-row epic set so the
// soak run is deterministic and free of Dolt connectivity assumptions.
type plansProjectionSoakBdSource struct{}

func (plansProjectionSoakBdSource) QueryEpics(ctx context.Context, projectID, issuePrefix string) ([]daemonpkg.PlansProjectionEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return []daemonpkg.PlansProjectionEntry{
		{BeadsID: "soc-soak-1", Title: "soak fixture epic 1", Status: "open", IssueType: "epic", UpdatedAt: now},
		{BeadsID: "soc-soak-2", Title: "soak fixture epic 2", Status: "closed", IssueType: "epic", UpdatedAt: now},
	}, nil
}

func runDaemonSoakClaimedJob(ctx context.Context, queue *daemonpkg.Queue, jobID string, executor daemonpkg.JobExecutor) error {
	claim, err := queue.ClaimJob(jobID, "daemon-soak", daemonpkg.QueueMutationOptions{})
	if err != nil {
		return err
	}
	result, execErr := executor.RunJob(ctx, claim)
	artifacts := result.Artifacts
	if execErr != nil {
		_, err := queue.FailJob(daemonpkg.FailJobInput{
			JobID:      claim.Job.JobID,
			RequestID:  daemonpkg.RequestID("req-" + claim.Job.JobID + "-fail"),
			ClaimToken: claim.ClaimToken,
			LeaseEpoch: claim.LeaseEpoch,
			Actor:      "daemon-soak",
			Failure: daemonpkg.JobFailure{
				Code:    daemonpkg.FailureRequestRejected,
				Message: execErr.Error(),
			},
			Artifacts: artifacts,
		}, daemonpkg.QueueMutationOptions{})
		return err
	}
	_, err = queue.CompleteJob(daemonpkg.CompleteJobInput{
		JobID:      claim.Job.JobID,
		RequestID:  daemonpkg.RequestID("req-" + claim.Job.JobID + "-complete"),
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "daemon-soak",
		Artifacts:  artifacts,
	}, daemonpkg.QueueMutationOptions{})
	return err
}

func validateDaemonSoakTerminal(opts daemonSoakOptions, jobs []daemonpkg.QueueJobState) error {
	if !opts.RequireTerminal {
		return nil
	}
	for _, job := range jobs {
		if !daemonJobIsTerminal(job.Status) {
			return fmt.Errorf("daemon soak require-terminal: job %s status is %s", job.JobID, job.Status)
		}
	}
	return nil
}

func readOpenClawJobsFromStore(store *daemonpkg.Store) ([]openclaw.ResourceSummary, error) {
	router := daemonpkg.NewReadOnlyRouter(store, daemonpkg.ServerOptions{})
	req := httptest.NewRequest(http.MethodGet, "/openclaw/v1/jobs", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		return nil, fmt.Errorf("OpenClaw jobs status = %d", resp.Code)
	}
	var jobs openclaw.JobsResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &jobs); err != nil {
		return nil, err
	}
	return jobs.Jobs, nil
}

func filterDaemonSoakEvents(events []daemonpkg.LedgerEvent, jobIDs []string) []daemonpkg.LedgerEvent {
	ids := daemonSoakStringSet(jobIDs)
	out := make([]daemonpkg.LedgerEvent, 0, len(events))
	for _, event := range events {
		if _, ok := ids[event.JobID]; ok {
			out = append(out, event)
		}
	}
	return out
}

func filterDaemonSoakJobs(jobs []daemonpkg.QueueJobState, jobIDs []string) []daemonpkg.QueueJobState {
	ids := daemonSoakStringSet(jobIDs)
	out := make([]daemonpkg.QueueJobState, 0, len(jobs))
	for _, job := range jobs {
		if _, ok := ids[job.JobID]; ok {
			out = append(out, job)
		}
	}
	return out
}

func filterOpenClawJobs(jobs []openclaw.ResourceSummary, jobIDs []string) []openclaw.ResourceSummary {
	ids := daemonSoakStringSet(jobIDs)
	out := make([]openclaw.ResourceSummary, 0, len(jobs))
	for _, job := range jobs {
		if _, ok := ids[job.JobID]; ok {
			out = append(out, job)
		}
	}
	return out
}

func daemonSoakStringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func writeDaemonSoakEvents(path string, events []daemonpkg.LedgerEvent) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	enc := json.NewEncoder(file)
	for _, event := range events {
		if err := enc.Encode(event); err != nil {
			return err
		}
	}
	return nil
}

func writeDaemonSoakJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func renderDaemonSoakSummary(report daemonSoakReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Daemon soak %s\n\n", report.RunID)
	fmt.Fprintf(&b, "- scenario: %s\n", report.Scenario)
	fmt.Fprintf(&b, "- status: %s\n", report.Status)
	fmt.Fprintf(&b, "- jobs: %d\n", len(report.Jobs))
	fmt.Fprintf(&b, "- openclaw_jobs: %d\n", len(report.OpenClawJobs))
	fmt.Fprintf(&b, "- events: %d\n", report.EventCount)
	if report.Failure != "" {
		fmt.Fprintf(&b, "- failure: %s\n", report.Failure)
	}
	return b.String()
}
