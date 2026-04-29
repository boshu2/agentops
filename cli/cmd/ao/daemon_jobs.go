package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	daemonJobWaitTimeout  time.Duration
	daemonJobCancelReason string
	daemonEventsAfter     string
)

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
	daemonJobsCmd.AddCommand(daemonJobsListCmd, daemonJobsShowCmd, daemonJobsWaitCmd, daemonJobsCancelCmd)
	daemonEventsCmd.AddCommand(daemonEventsTailCmd)

	daemonJobsCmd.PersistentFlags().StringVar(&daemonURL, "url", "", "Daemon base URL (defaults to activation file)")
	daemonJobsWaitCmd.Flags().DurationVar(&daemonJobWaitTimeout, "timeout", 30*time.Second, "Maximum time to wait for terminal job state")
	daemonJobsCancelCmd.Flags().StringVar(&daemonJobCancelReason, "reason", "", "Cancellation reason")
	daemonJobsCancelCmd.Flags().StringVar(&daemonToken, "token", "", "Mutation token for daemon write routes")
	daemonJobsCancelCmd.Flags().StringVar(&daemonTokenFile, "token-file", "", "Path to mutation token file")
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

func runAgentOpsDaemonJobsCancelCommand(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	baseURL, err := resolveDaemonURL(cwd, daemonURL)
	if err != nil {
		return err
	}
	token, err := resolveDaemonMutationToken(daemonToken, daemonTokenFile)
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
