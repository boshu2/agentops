// practices: [sre, distributed-tracing]
package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	watchDaemonURL string
	watchInterval  time.Duration
	watchSince     string
	watchOnce      bool
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch the agentops daemon event stream",
	Long: `Poll agentopsd /v1/events and print new ledger events as a TTY-friendly stream.

This intentionally uses plain stdout instead of a TUI library so it works in tmux,
logs, SSH sessions, and CI artifacts.`,
	Args: cobra.NoArgs,
	RunE: runAoWatchCommand,
}

func init() {
	watchCmd.GroupID = "core"
	watchCmd.Flags().StringVar(&watchDaemonURL, "url", "", "Daemon base URL (defaults to activation file)")
	watchCmd.Flags().StringVar(&watchSince, "since", "", "Only show events after this event id")
	watchCmd.Flags().DurationVar(&watchInterval, "interval", time.Second, "Polling interval")
	watchCmd.Flags().BoolVar(&watchOnce, "once", false, "Fetch once and exit")
	rootCmd.AddCommand(watchCmd)
}

func runAoWatchCommand(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	baseURL, err := resolveDaemonURL(cwd, watchDaemonURL)
	if err != nil {
		return err
	}
	interval := watchInterval
	if interval <= 0 {
		return fmt.Errorf("watch interval must be positive")
	}
	ctx := cobraContext(cmd)
	since := strings.TrimSpace(watchSince)
	out := cmd.OutOrStdout()
	if !watchOnce {
		fmt.Fprintf(out, "ao watch %s\n", strings.TrimRight(baseURL, "/"))
		fmt.Fprintln(out, "occurred_at\tevent_id\tevent_type\tjob_id\tjob_type\tactor")
	}
	for {
		events, err := fetchDaemonEventsSince(ctx, baseURL, since)
		if err != nil {
			return err
		}
		for _, event := range events.Events {
			fmt.Fprintln(out, formatWatchEvent(event))
			since = event.EventID
		}
		if watchOnce {
			return nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func fetchDaemonEventsSince(ctx context.Context, baseURL, since string) (daemonpkg.ReadOnlyEventsResponse, error) {
	eventsURL := strings.TrimRight(baseURL, "/") + "/v1/events"
	if strings.TrimSpace(since) != "" {
		parsed, err := url.Parse(eventsURL)
		if err != nil {
			return daemonpkg.ReadOnlyEventsResponse{}, err
		}
		query := parsed.Query()
		query.Set("since", since)
		parsed.RawQuery = query.Encode()
		eventsURL = parsed.String()
	}
	var events daemonpkg.ReadOnlyEventsResponse
	if err := fetchDaemonJSON(ctx, eventsURL, &events); err != nil {
		return events, err
	}
	return events, nil
}

func formatWatchEvent(event daemonpkg.LedgerEvent) string {
	fields := []string{
		watchValueOrDash(event.OccurredAt),
		watchValueOrDash(event.EventID),
		watchValueOrDash(string(event.EventType)),
		watchValueOrDash(event.JobID),
		watchValueOrDash(watchEventJobType(event.Payload)),
		watchValueOrDash(event.Actor),
	}
	for i, field := range fields {
		fields[i] = sanitizeWatchField(field)
	}
	return strings.Join(fields, "\t")
}

func watchEventJobType(payload map[string]any) string {
	if value, ok := payload["job_type"].(string); ok {
		return value
	}
	raw, ok := payload["job_payload"]
	if !ok {
		return ""
	}
	if values, ok := raw.(map[string]any); ok {
		if value, ok := values["job_type"].(string); ok {
			return value
		}
	}
	return ""
}

func watchValueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func sanitizeWatchField(value string) string {
	value = strings.ReplaceAll(value, "\t", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
