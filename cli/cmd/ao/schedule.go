package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/boshu2/agentops/cli/internal/schedule"
	"github.com/spf13/cobra"
)

// schedule.go (soc-8inr.10) wires the operator-facing CLI for managing
// agentopsd recurring schedules. Routes:
//
//   - ao schedule add --file <path>   -> POST /v1/schedules (per template)
//   - ao schedule list [--json]       -> GET  /v1/schedules
//   - ao schedule run <name>          -> client-side fan-out:
//                                          GET /v1/schedules, find <name>,
//                                          POST /v1/jobs with the template's
//                                          job_type+payload (one-shot fire).
//   - ao schedule remove <name>       -> DELETE /v1/schedules/{name}
//
// Mutation auth (--token / --token-file) is required for add / run / remove.
// list is read-only and skips auth.

var (
	scheduleFile     string
	scheduleListJSON bool
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage daemon schedules",
}

var scheduleAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add schedules from a YAML file",
	Args:  cobra.NoArgs,
	RunE:  runScheduleAddCommand,
}

var scheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active schedules",
	Args:  cobra.NoArgs,
	RunE:  runScheduleListCommand,
}

var scheduleRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "One-shot test fire of a named schedule (immediate Queue.Submit)",
	Args:  cobra.ExactArgs(1),
	RunE:  runScheduleRunCommand,
}

var scheduleRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a schedule by name",
	Args:  cobra.ExactArgs(1),
	RunE:  runScheduleRemoveCommand,
}

func init() {
	rootCmd.AddCommand(scheduleCmd)
	scheduleCmd.AddCommand(scheduleAddCmd, scheduleListCmd, scheduleRunCmd, scheduleRemoveCmd)

	scheduleCmd.PersistentFlags().StringVar(&daemonURL, "url", "", "Daemon base URL (defaults to activation file)")

	scheduleAddCmd.Flags().StringVar(&scheduleFile, "file", "", "Path to schedule YAML file (required)")
	_ = scheduleAddCmd.MarkFlagRequired("file")
	scheduleAddCmd.Flags().StringVar(&daemonToken, "token", "", "Mutation token for daemon write routes")
	scheduleAddCmd.Flags().StringVar(&daemonTokenFile, "token-file", "", "Path to mutation token file")

	scheduleListCmd.Flags().BoolVar(&scheduleListJSON, "json", false, "Emit machine-readable JSON output")

	scheduleRunCmd.Flags().StringVar(&daemonToken, "token", "", "Mutation token for daemon write routes")
	scheduleRunCmd.Flags().StringVar(&daemonTokenFile, "token-file", "", "Path to mutation token file")

	scheduleRemoveCmd.Flags().StringVar(&daemonToken, "token", "", "Mutation token for daemon write routes")
	scheduleRemoveCmd.Flags().StringVar(&daemonTokenFile, "token-file", "", "Path to mutation token file")
}

// scheduleClient bundles the resolved daemon base URL + (optional) mutation token.
type scheduleClient struct {
	baseURL string
	token   string
}

// resolveScheduleClient resolves the daemon URL via the standard activation-file
// fallback path. token is empty when not required.
func resolveScheduleClient(includeToken bool) (scheduleClient, error) {
	cwd, err := resolveProjectDir()
	if err != nil {
		return scheduleClient{}, err
	}
	baseURL, err := resolveDaemonURL(cwd, daemonURL)
	if err != nil {
		return scheduleClient{}, err
	}
	c := scheduleClient{baseURL: baseURL}
	if includeToken {
		token, err := resolveAgentOpsDaemonClientMutationToken(cwd, daemonToken, daemonTokenFile)
		if err != nil {
			return scheduleClient{}, err
		}
		c.token = token
	}
	return c, nil
}

func runScheduleAddCommand(cmd *cobra.Command, _ []string) error {
	if strings.TrimSpace(scheduleFile) == "" {
		return fmt.Errorf("--file is required")
	}
	templates, err := schedule.Load(scheduleFile)
	if err != nil {
		return err
	}
	if len(templates) == 0 {
		return fmt.Errorf("no schedules found in %s", scheduleFile)
	}
	client, err := resolveScheduleClient(true)
	if err != nil {
		return err
	}
	ctx := cobraContext(cmd)
	added := make([]string, 0, len(templates))
	for _, tmpl := range templates {
		var resp daemonpkg.CreateScheduleResponse
		if err := postDaemonJSON(ctx, client.baseURL+"/v1/schedules", client.token, tmpl, &resp); err != nil {
			return fmt.Errorf("add schedule %q: %w", tmpl.Name, err)
		}
		added = append(added, resp.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "added: %s\n", resp.Name)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "added %d schedule(s)\n", len(added))
	return nil
}

func runScheduleListCommand(cmd *cobra.Command, _ []string) error {
	client, err := resolveScheduleClient(false)
	if err != nil {
		return err
	}
	ctx := cobraContext(cmd)
	resp, err := fetchSchedules(ctx, client.baseURL)
	if err != nil {
		return err
	}
	if scheduleListJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}
	if len(resp.Schedules) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no schedules")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", "NAME", "CRON", "JOB_TYPE", "BACKPRESSURE")
	for _, s := range resp.Schedules {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n",
			s.Name, s.Cron, s.JobType, formatBackpressure(s.Backpressure))
	}
	return nil
}

func runScheduleRunCommand(cmd *cobra.Command, args []string) error {
	name := strings.TrimSpace(args[0])
	if name == "" {
		return fmt.Errorf("schedule name is required")
	}
	client, err := resolveScheduleClient(true)
	if err != nil {
		return err
	}
	ctx := cobraContext(cmd)
	resp, err := fetchSchedules(ctx, client.baseURL)
	if err != nil {
		return err
	}
	tmpl, ok := findScheduleByName(resp.Schedules, name)
	if !ok {
		return fmt.Errorf("schedule not found: %s", name)
	}
	submit, err := buildSubmitFromTemplate(tmpl)
	if err != nil {
		return err
	}
	var submitResp daemonpkg.SubmitJobResponse
	if err := postDaemonJSON(ctx, client.baseURL+"/v1/jobs", client.token, submit, &submitResp); err != nil {
		return fmt.Errorf("run schedule %q: %w", name, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "fired: %s\tjob_id=%s\tstatus=%s\n",
		name, submitResp.JobID, submitResp.Status)
	return nil
}

func runScheduleRemoveCommand(cmd *cobra.Command, args []string) error {
	name := strings.TrimSpace(args[0])
	if name == "" {
		return fmt.Errorf("schedule name is required")
	}
	client, err := resolveScheduleClient(true)
	if err != nil {
		return err
	}
	ctx := cobraContext(cmd)
	var resp daemonpkg.DeleteScheduleResponse
	endpoint := client.baseURL + "/v1/schedules/" + url.PathEscape(name)
	if err := deleteDaemonJSON(ctx, endpoint, client.token, &resp); err != nil {
		return fmt.Errorf("remove schedule %q: %w", name, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "removed: %s\tdeleted=%v\n", resp.Name, resp.Deleted)
	return nil
}

func fetchSchedules(ctx context.Context, baseURL string) (daemonpkg.ListSchedulesResponse, error) {
	var out daemonpkg.ListSchedulesResponse
	if err := fetchDaemonJSON(ctx, baseURL+"/v1/schedules", &out); err != nil {
		return out, err
	}
	return out, nil
}

func findScheduleByName(schedules []daemonpkg.RecurringJobTemplate, name string) (daemonpkg.RecurringJobTemplate, bool) {
	for _, s := range schedules {
		if s.Name == name {
			return s, true
		}
	}
	return daemonpkg.RecurringJobTemplate{}, false
}

// buildSubmitFromTemplate decodes the template's JSON payload into a map and
// emits a SubmitJobRequest tagged with a one-shot idempotency key derived from
// the schedule name + a wall-clock nanosecond. The recurrence supervisor uses a
// submission_id key for cron ticks; this manual fire intentionally uses a
// distinct prefix so it never collides with a scheduled tick.
func buildSubmitFromTemplate(tmpl daemonpkg.RecurringJobTemplate) (daemonpkg.SubmitJobRequest, error) {
	req := daemonpkg.SubmitJobRequest{
		JobType:        tmpl.JobType,
		IdempotencyKey: fmt.Sprintf("schedule-run:%s:%d", tmpl.Name, time.Now().UnixNano()),
	}
	if len(tmpl.Payload) > 0 {
		var payload map[string]any
		if err := json.Unmarshal(tmpl.Payload, &payload); err != nil {
			return daemonpkg.SubmitJobRequest{}, fmt.Errorf("decode payload for schedule %q: %w", tmpl.Name, err)
		}
		req.Payload = payload
	}
	return req, nil
}

func formatBackpressure(bp daemonpkg.RecurrenceBackpressure) string {
	return fmt.Sprintf("skip_if_running=%v,max_queue_depth=%d", bp.SkipIfRunning, bp.MaxQueueDepth)
}

// deleteDaemonJSON issues a DELETE against target with the mutation token in
// the standard header and decodes the JSON response into out (when non-nil).
// It mirrors postDaemonJSON's error contract.
func deleteDaemonJSON(ctx context.Context, target, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, target, nil)
	if err != nil {
		return err
	}
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
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
