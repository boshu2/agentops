// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	rpiStreamRunID  string
	rpiStreamFmt    string
	rpiStreamFollow bool
)

func init() {
	streamCmd := &cobra.Command{
		Use:   "stream",
		Short: "Stream normalized RPI C2 events",
		Long: `Read normalized per-run C2 events from events.jsonl.

Formats:
  human  readable table-like lines
  json   JSONL passthrough (one event per line)
  sse    server-sent-event framing for lightweight subscribers`,
		RunE: runRPIStream,
	}
	streamCmd.Flags().StringVar(&rpiStreamRunID, "run-id", "", "Run ID to stream (defaults to latest phased state)")
	streamCmd.Flags().StringVar(&rpiStreamFmt, "format", "human", "Output format: human|json|sse")
	streamCmd.Flags().BoolVar(&rpiStreamFollow, "follow", false, "Follow for newly appended events")
	addRPISubcommand(streamCmd)
}

func runRPIStream(cmd *cobra.Command, args []string) error {
	format := strings.ToLower(strings.TrimSpace(rpiStreamFmt))
	switch format {
	case "human", "json", "sse":
	default:
		return fmt.Errorf("--format must be one of human|json|sse")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	runID, _, root, err := resolveNudgeRun(cwd, strings.TrimSpace(rpiStreamRunID))
	if err != nil {
		return err
	}

	seen := 0
	for {
		events, err := loadRPIC2Events(root, runID)
		if err != nil {
			return err
		}
		for ; seen < len(events); seen++ {
			if err := writeStreamEvent(os.Stdout, events[seen], format); err != nil {
				return err
			}
		}
		if !rpiStreamFollow {
			break
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func writeStreamEvent(out *os.File, event RPIC2Event, format string) error {
	if out == nil {
		return fmt.Errorf("output file is nil")
	}
	switch format {
	case "json":
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	case "sse":
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "id: %s\n", event.EventID); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "event: %s\n", event.Type); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "data: %s\n\n", string(data)); err != nil {
			return err
		}
		return nil
	default: // human
		phase := "-"
		if event.Phase > 0 {
			phase = fmt.Sprintf("%d", event.Phase)
		}
		worker := "-"
		if strings.TrimSpace(event.WorkerID) != "" {
			worker = event.WorkerID
		}
		msg := strings.TrimSpace(event.Message)
		if msg == "" {
			msg = "-"
		}
		_, err := fmt.Fprintf(out, "%s phase=%s worker=%s type=%s source=%s msg=%s\n",
			event.Timestamp,
			phase,
			worker,
			event.Type,
			fallbackValue(event.Source, "-"),
			msg,
		)
		return err
	}
}

func fallbackValue(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
