// practices: [microservices, sre]
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	daemonJobWaitTimeout   time.Duration
	daemonJobCancelReason  string
	daemonEventsAfter      string
	daemonJobSubmitType    string
	daemonJobSubmitPayload string
)

// daemonJobSubmitUnknownTypeExitCode mirrors `timeout(1)`-style enum-validation
// failures: callers (scripts, evolve loops) can branch on it without parsing stderr.
const daemonJobSubmitUnknownTypeExitCode = 2

// knownDaemonJobTypes enumerates the JobType values a non-CLI client may submit.
// Sourced from cli/internal/daemon/types.go.
var knownDaemonJobTypes = []daemonpkg.JobType{
	daemonpkg.JobTypeRPIRun,
	daemonpkg.JobTypeRPIPhase,
	daemonpkg.JobTypeDreamRun,
	daemonpkg.JobTypeDreamStage,
	daemonpkg.JobTypeWikiBuild,
	daemonpkg.JobTypeWikiForge,
	daemonpkg.JobTypeFactoryAdmission,
	daemonpkg.JobTypeFactoryLocalPilot,
	daemonpkg.JobTypeOpenClawSnapshot,
	daemonpkg.JobTypePlansProjection,
	daemonpkg.JobTypeLLMWikiLoop,
	daemonpkg.JobTypeEvalSuite,
	daemonpkg.JobTypeEvalSkillDelta,
	daemonpkg.JobTypeSkillInvoke,
}

var daemonJobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "Inspect and control daemon jobs",
}

var daemonJobsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List daemon jobs",
	Args:  cobra.NoArgs,
	RunE:  runAgentOpsDaemonJobsListCommand,
}

var daemonJobsShowCmd = &cobra.Command{
	Use:   "show <job-id>",
	Short: "Show a daemon job",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentOpsDaemonJobsShowCommand,
}

var daemonJobsWaitCmd = &cobra.Command{
	Use:   "wait <job-id>",
	Short: "Wait for a daemon job to reach terminal state",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentOpsDaemonJobsWaitCommand,
}

var daemonJobsCancelCmd = &cobra.Command{
	Use:   "cancel <job-id>",
	Short: "Cancel a daemon job",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentOpsDaemonJobsCancelCommand,
}

var daemonJobsSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit a job to the daemon queue",
	Long: `Submit a job to the daemon queue.

The payload is sent verbatim under the JSON 'payload' key. --type must be one
of the known JobType values (cli/internal/daemon/types.go).

Examples:
  ao daemon jobs submit --type openclaw.snapshot --payload '{}' --json
  ao daemon jobs submit --type wiki.forge --payload @./payload.json
  ao daemon jobs submit --type rpi.phase --payload '{"phase":"discovery"}' --token-file ~/.agents/daemon/.token`,
	Args: cobra.NoArgs,
	RunE: runAgentOpsDaemonJobsSubmitCommand,
}

var daemonEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Inspect daemon events",
}

var daemonEventsTailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Show daemon events after an optional event id",
	Args:  cobra.NoArgs,
	RunE:  runAgentOpsDaemonEventsTailCommand,
}

func init() {
	daemonCmd.AddCommand(daemonJobsCmd, daemonEventsCmd)
	daemonJobsCmd.AddCommand(daemonJobsListCmd, daemonJobsShowCmd, daemonJobsWaitCmd, daemonJobsCancelCmd, daemonJobsSubmitCmd)
	daemonEventsCmd.AddCommand(daemonEventsTailCmd)

	daemonJobsCmd.PersistentFlags().StringVar(&daemonURL, "url", "", "Daemon base URL (defaults to activation file)")
	daemonJobsCmd.PersistentFlags().StringVar(&daemonToken, "token", "", "Mutation token for daemon write routes")
	daemonJobsCmd.PersistentFlags().StringVar(&daemonTokenFile, "token-file", "", "Path to mutation token file")
	daemonJobsWaitCmd.Flags().DurationVar(&daemonJobWaitTimeout, "timeout", 30*time.Second, "Maximum time to wait for terminal job state")
	daemonJobsCancelCmd.Flags().StringVar(&daemonJobCancelReason, "reason", "", "Cancellation reason")
	daemonJobsSubmitCmd.Flags().StringVar(&daemonJobSubmitType, "type", "", "Job type (required; one of "+strings.Join(daemonJobTypeStrings(), ", ")+")")
	daemonJobsSubmitCmd.Flags().StringVar(&daemonJobSubmitPayload, "payload", "", "JSON payload (required; '@-' for stdin, '@path' for file)")
	daemonEventsCmd.PersistentFlags().StringVar(&daemonURL, "url", "", "Daemon base URL (defaults to activation file)")
	daemonEventsTailCmd.Flags().StringVar(&daemonEventsAfter, "after", "", "Only show events after this event id")
}

type daemonJobsListResponse struct {
	Jobs        []daemonpkg.QueueJobState `json:"jobs"`
	LastEventID string                    `json:"last_event_id,omitempty"`
}

func runAgentOpsDaemonJobsListCommand(cmd *cobra.Command, args []string) error {
	status, err := loadDaemonStatusForCommand(cmd)
	if err != nil {
		return err
	}
	response := daemonJobsListResponse{Jobs: status.Queue.Jobs, LastEventID: status.Queue.LastEventID}
	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(response)
	}
	for _, job := range response.Jobs {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", job.JobID, job.JobType, job.Status, job.UpdatedAt)
	}
	return nil
}

func runAgentOpsDaemonJobsShowCommand(cmd *cobra.Command, args []string) error {
	status, err := loadDaemonStatusForCommand(cmd)
	if err != nil {
		return err
	}
	job, ok := findDaemonJob(status.Queue.Jobs, args[0])
	if !ok {
		return fmt.Errorf("daemon job not found: %s", args[0])
	}
	return renderDaemonJob(cmd, job)
}

func runAgentOpsDaemonJobsWaitCommand(cmd *cobra.Command, args []string) error {
	timeout := daemonJobWaitTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(cobraContext(cmd), timeout)
	defer cancel()
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	baseURL, err := resolveDaemonURL(cwd, daemonURL)
	if err != nil {
		return err
	}
	job, err := waitForDaemonJobStatus(ctx, baseURL, args[0], timeout)
	if err != nil {
		return err
	}
	return renderDaemonJob(cmd, job)
}

func waitForDaemonJobStatus(ctx context.Context, baseURL, jobID string, timeout time.Duration) (daemonpkg.QueueJobState, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	for {
		status, err := fetchDaemonStatus(ctx, baseURL)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return daemonpkg.QueueJobState{}, fmt.Errorf("timed out waiting for daemon job %s after %s", jobID, timeout)
			}
			return daemonpkg.QueueJobState{}, err
		}
		job, ok := findDaemonJob(status.Queue.Jobs, jobID)
		if !ok {
			return daemonpkg.QueueJobState{}, fmt.Errorf("daemon job not found: %s", jobID)
		}
		if daemonJobIsTerminal(job.Status) {
			return job, nil
		}
		select {
		case <-ctx.Done():
			return job, fmt.Errorf("timed out waiting for daemon job %s after %s", jobID, timeout)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// daemonJobTypeStrings returns the known JobType values as a sorted []string,
// for use in flag help text and unknown-type error messages.
func daemonJobTypeStrings() []string {
	out := make([]string, 0, len(knownDaemonJobTypes))
	for _, t := range knownDaemonJobTypes {
		out = append(out, string(t))
	}
	sort.Strings(out)
	return out
}

// unknownJobTypeError is a sentinel for the RunE wrapper so it can map invalid
// --type values to a distinct process exit code.
type unknownJobTypeError struct {
	got   string
	known []string
}

func (e unknownJobTypeError) Error() string {
	return fmt.Sprintf("unknown job type %q; known: %s", e.got, strings.Join(e.known, ", "))
}

func validateSubmitJobType(value string) error {
	for _, t := range knownDaemonJobTypes {
		if string(t) == value {
			return nil
		}
	}
	return unknownJobTypeError{got: value, known: daemonJobTypeStrings()}
}

// readSubmitPayload accepts inline JSON, '@-' for stdin, or '@path' for a file.
func readSubmitPayload(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("--payload is required")
	}
	var data []byte
	switch {
	case trimmed == "@-":
		buf, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read payload from stdin: %w", err)
		}
		data = buf
	case strings.HasPrefix(trimmed, "@"):
		buf, err := os.ReadFile(trimmed[1:])
		if err != nil {
			return nil, fmt.Errorf("read payload file: %w", err)
		}
		data = buf
	default:
		data = []byte(trimmed)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("--payload resolved to empty content")
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("payload is not a JSON object: %w", err)
	}
	return payload, nil
}

func runAgentOpsDaemonJobsSubmitCommand(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(daemonJobSubmitType) == "" {
		return errors.New("--type is required")
	}
	if err := validateSubmitJobType(daemonJobSubmitType); err != nil {
		var unk unknownJobTypeError
		if errors.As(err, &unk) {
			fmt.Fprintln(cmd.ErrOrStderr(), unk.Error())
			os.Exit(daemonJobSubmitUnknownTypeExitCode)
		}
		return err
	}
	payload, err := readSubmitPayload(daemonJobSubmitPayload)
	if err != nil {
		return err
	}
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	baseURL, err := resolveDaemonURL(cwd, daemonURL)
	if err != nil {
		return err
	}
	token, err := resolveAgentOpsDaemonClientMutationToken(cwd, daemonToken, daemonTokenFile)
	if err != nil {
		return err
	}
	response, err := submitDaemonJob(cobraContext(cmd), baseURL, token, daemonpkg.JobType(daemonJobSubmitType), payload)
	if err != nil {
		return err
	}
	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(response)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", response.JobID, response.JobType, response.Status)
	return nil
}

// submitDaemonJobResponse is the operator-facing shape for `ao daemon jobs submit`.
// Mirrors daemonpkg.SubmitJobResponse but adds JobType for clean tabular output
// (the daemon does not echo the requested type back).
type submitDaemonJobResponse struct {
	Accepted    bool                       `json:"accepted"`
	JobID       string                     `json:"job_id"`
	JobType     daemonpkg.JobType          `json:"job_type"`
	Status      daemonpkg.JobStatus        `json:"status"`
	RequestID   string                     `json:"request_id,omitempty"`
	LastEventID string                     `json:"last_event_id,omitempty"`
	Projection  daemonpkg.ProjectionStatus `json:"projection_status,omitempty"`
}

func submitDaemonJob(ctx context.Context, baseURL, token string, jobType daemonpkg.JobType, payload map[string]any) (submitDaemonJobResponse, error) {
	var raw daemonpkg.SubmitJobResponse
	request := daemonpkg.SubmitJobRequest{
		JobType: jobType,
		Payload: payload,
	}
	if err := postDaemonJSON(ctx, baseURL+"/v1/jobs", token, request, &raw); err != nil {
		return submitDaemonJobResponse{}, err
	}
	return submitDaemonJobResponse{
		Accepted:    raw.Accepted,
		JobID:       raw.JobID,
		JobType:     jobType,
		Status:      raw.Status,
		RequestID:   raw.RequestID,
		LastEventID: raw.LastEventID,
		Projection:  raw.ProjectionStatus,
	}, nil
}

func runAgentOpsDaemonJobsCancelCommand(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	baseURL, err := resolveDaemonURL(cwd, daemonURL)
	if err != nil {
		return err
	}
	token, err := resolveAgentOpsDaemonClientMutationToken(cwd, daemonToken, daemonTokenFile)
	if err != nil {
		return err
	}
	response, err := cancelDaemonJob(cobraContext(cmd), baseURL, token, args[0], daemonJobCancelReason)
	if err != nil {
		return err
	}
	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(response)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", response.Job.JobID, response.Job.Status, response.Outcome)
	return nil
}

func runAgentOpsDaemonEventsTailCommand(cmd *cobra.Command, args []string) error {
	events, err := loadDaemonEventsForCommand(cmd)
	if err != nil {
		return err
	}
	events.Events = filterDaemonEventsAfter(events.Events, daemonEventsAfter)
	if len(events.Events) > 0 {
		events.LastEventID = events.Events[len(events.Events)-1].EventID
	}
	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(events)
	}
	for _, event := range events.Events {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", event.EventID, event.EventType, event.JobID, event.OccurredAt)
	}
	return nil
}

func loadDaemonStatusForCommand(cmd *cobra.Command) (daemonpkg.ReadOnlyStatusResponse, error) {
	return loadDaemonStatusWithContext(cobraContext(cmd), cmd)
}

func loadDaemonStatusWithContext(ctx context.Context, cmd *cobra.Command) (daemonpkg.ReadOnlyStatusResponse, error) {
	cwd, err := resolveProjectDir()
	if err != nil {
		return daemonpkg.ReadOnlyStatusResponse{}, err
	}
	baseURL, err := resolveDaemonURL(cwd, daemonURL)
	if err != nil {
		return daemonpkg.ReadOnlyStatusResponse{}, err
	}
	return fetchDaemonStatus(ctx, baseURL)
}

func loadDaemonEventsForCommand(cmd *cobra.Command) (daemonpkg.ReadOnlyEventsResponse, error) {
	cwd, err := resolveProjectDir()
	if err != nil {
		return daemonpkg.ReadOnlyEventsResponse{}, err
	}
	baseURL, err := resolveDaemonURL(cwd, daemonURL)
	if err != nil {
		return daemonpkg.ReadOnlyEventsResponse{}, err
	}
	return fetchDaemonEvents(cobraContext(cmd), baseURL)
}

func renderDaemonJob(cmd *cobra.Command, job daemonpkg.QueueJobState) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(job)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "job: %s\n", job.JobID)
	fmt.Fprintf(cmd.OutOrStdout(), "type: %s\n", job.JobType)
	fmt.Fprintf(cmd.OutOrStdout(), "status: %s\n", job.Status)
	if job.Failure != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "failure: %s %s\n", job.Failure.Code, job.Failure.Message)
	}
	return nil
}

func findDaemonJob(jobs []daemonpkg.QueueJobState, jobID string) (daemonpkg.QueueJobState, bool) {
	for _, job := range jobs {
		if job.JobID == jobID {
			return job, true
		}
	}
	return daemonpkg.QueueJobState{}, false
}

func daemonJobIsTerminal(status daemonpkg.JobStatus) bool {
	switch status {
	case daemonpkg.JobStatusCompleted, daemonpkg.JobStatusFailed, daemonpkg.JobStatusCancelled:
		return true
	default:
		return false
	}
}

func cancelDaemonJob(ctx context.Context, baseURL, token, jobID, reason string) (daemonpkg.CancelJobResponse, error) {
	var response daemonpkg.CancelJobResponse
	request := daemonpkg.CancelJobRequest{
		JobID:  jobID,
		Reason: reason,
	}
	if err := postDaemonJSON(ctx, baseURL+"/v1/jobs/cancel", token, request, &response); err != nil {
		return response, err
	}
	return response, nil
}

func fetchDaemonEvents(ctx context.Context, baseURL string) (daemonpkg.ReadOnlyEventsResponse, error) {
	var events daemonpkg.ReadOnlyEventsResponse
	if err := fetchDaemonJSON(ctx, baseURL+"/v1/events", &events); err != nil {
		return events, err
	}
	return events, nil
}

func postDaemonJSON(ctx context.Context, url, token string, input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(token) != "" {
		req.Header.Set(daemonpkg.DefaultMutationTokenHeader, token)
	}
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return json.NewDecoder(resp.Body).Decode(output)
}

func filterDaemonEventsAfter(events []daemonpkg.LedgerEvent, after string) []daemonpkg.LedgerEvent {
	if after == "" {
		return events
	}
	for i, event := range events {
		if event.EventID == after {
			return events[i+1:]
		}
	}
	return nil
}
