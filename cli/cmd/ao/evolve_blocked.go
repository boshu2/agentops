// practices: [dora-metrics, lean-startup]
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/evolve"
	"github.com/spf13/cobra"
)

// evolveBlocked subcommand (soc-g34d) records typed blocked-events that the
// agent emits when the next-work ladder is exhausted (or any other blocking
// condition). The /evolve loop contract is "log, don't halt": agents append a
// structured record to .agents/evolve/blocked.jsonl rather than writing a
// STOP/DORMANT marker. Operators triage from the log.
//
// See docs/plans/2026-05-21-evolve-loop-epic-design.md §A6.

const (
	evolveBlockedRelDir  = ".agents/evolve"
	evolveBlockedLogName = "blocked.jsonl"
	evolveCronHistoryRel = ".agents/evolve/cron-history.jsonl"
	evolveBlockedDefTail = 10
)

var (
	evolveBlockedReason         string
	evolveBlockedBead           string
	evolveBlockedNeededContext  string
	evolveBlockedLadderStep     int
	evolveBlockedList           bool
	evolveBlockedTail           int
	evolveBlockedJSON           bool
	evolveBlockedClearCycleID   string
	evolveBlockedClear          bool
	evolveBlockedCycleOverride  string
	evolveBlockedTimestampClock func() time.Time
)

var evolveBlockedCmd = &cobra.Command{
	Use:   "blocked",
	Short: "Log or list typed blocked-events from the /evolve loop",
	Long: `Record or inspect typed blocked-events emitted by the /evolve loop.

The loop contract is "log, don't halt": when the agent can't make progress
(empty next-work ladder, missing context, ambiguous acceptance), it appends a
structured record to .agents/evolve/blocked.jsonl rather than writing a STOP
or DORMANT marker. Operators triage the log between cycles.

Three modes (mutually exclusive):

  Write:  ao evolve blocked --reason '<text>' [--bead <id>] [--needed-context '<text>']
  Read:   ao evolve blocked --list [--tail N] [--json]
  Clear:  ao evolve blocked --clear <cycle-id>   (operator-only)

Examples:
  ao evolve blocked --reason 'ladder exhausted' --bead soc-mlbm --needed-context 'undefined step 4 semantics'
  ao evolve blocked --list --tail 20 --json
  ao evolve blocked --clear 2026-05-21-cycle-42`,
	Args: cobra.NoArgs,
	RunE: runEvolveBlocked,
}

func init() {
	evolveBlockedTimestampClock = func() time.Time { return time.Now().UTC() }
	evolveBlockedCmd.Flags().StringVar(&evolveBlockedReason, "reason", "", "Reason text (write mode)")
	evolveBlockedCmd.Flags().StringVar(&evolveBlockedBead, "bead", "", "Bead id the agent was working on (write mode, optional)")
	evolveBlockedCmd.Flags().StringVar(&evolveBlockedNeededContext, "needed-context", "", "Missing context description (write mode, optional)")
	evolveBlockedCmd.Flags().IntVar(&evolveBlockedLadderStep, "ladder-step-failed", 0, "Ladder step that failed (write mode, optional)")
	evolveBlockedCmd.Flags().BoolVar(&evolveBlockedList, "list", false, "Read mode: list blocked events")
	evolveBlockedCmd.Flags().IntVar(&evolveBlockedTail, "tail", evolveBlockedDefTail, "Read mode: show last N entries")
	evolveBlockedCmd.Flags().BoolVar(&evolveBlockedJSON, "json", false, "Read mode: emit JSON instead of human-readable text")
	evolveBlockedCmd.Flags().StringVar(&evolveBlockedClearCycleID, "clear", "", "Clear mode: delete entries for the given cycle id (operator-only)")
	evolveBlockedCmd.Flags().StringVar(&evolveBlockedCycleOverride, "cycle", "", "Override cycle-id (write mode; defaults to date-derived counter)")
	evolveCmd.AddCommand(evolveBlockedCmd)
}

// BlockedEvent is the schema for one row in .agents/evolve/blocked.jsonl. All
// fields except cycle/timestamp/reason are optional; the JSONL reader rejects
// records missing those three.
type BlockedEvent struct {
	Cycle             string `json:"cycle"`
	Timestamp         string `json:"timestamp"`
	Bead              string `json:"bead,omitempty"`
	Reason            string `json:"reason"`
	NeededContext     string `json:"needed_context,omitempty"`
	LadderStepFailed  int    `json:"ladder_step_failed,omitempty"`
}

func runEvolveBlocked(cmd *cobra.Command, _ []string) error {
	mode, err := resolveBlockedMode(cmd)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	switch mode {
	case "write":
		return runEvolveBlockedWrite(cmd, cwd)
	case "list":
		return runEvolveBlockedList(cmd, cwd)
	case "clear":
		return runEvolveBlockedClear(cmd, cwd)
	default:
		return errors.New("ao evolve blocked: must pass one of --reason, --list, or --clear")
	}
}

// resolveBlockedMode picks which of the three modes the operator requested,
// rejecting combinations of mutually-exclusive flags.
func resolveBlockedMode(cmd *cobra.Command) (string, error) {
	writeFlag := cmd.Flags().Changed("reason")
	listFlag := evolveBlockedList || cmd.Flags().Changed("list")
	clearFlag := evolveBlockedClearCycleID != ""

	set := 0
	if writeFlag {
		set++
	}
	if listFlag {
		set++
	}
	if clearFlag {
		set++
	}
	if set == 0 {
		return "", errors.New("ao evolve blocked: must pass one of --reason, --list, or --clear")
	}
	if set > 1 {
		return "", errors.New("ao evolve blocked: --reason, --list, and --clear are mutually exclusive")
	}
	switch {
	case writeFlag:
		return "write", nil
	case listFlag:
		return "list", nil
	default:
		return "clear", nil
	}
}

func runEvolveBlockedWrite(cmd *cobra.Command, cwd string) error {
	if strings.TrimSpace(evolveBlockedReason) == "" {
		return errors.New("ao evolve blocked --reason: reason cannot be empty")
	}
	cycle := evolveBlockedCycleOverride
	if cycle == "" {
		cycle = nextBlockedCycleID(cwd, evolveBlockedTimestampClock())
	}
	event := BlockedEvent{
		Cycle:            cycle,
		Timestamp:        evolveBlockedTimestampClock().Format(time.RFC3339),
		Bead:             evolveBlockedBead,
		Reason:           evolveBlockedReason,
		NeededContext:    evolveBlockedNeededContext,
		LadderStepFailed: evolveBlockedLadderStep,
	}
	path := filepath.Join(cwd, evolveBlockedRelDir, evolveBlockedLogName)
	if err := appendBlockedEvent(path, event); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Logged blocked event for cycle %s\n", event.Cycle)
	return nil
}

func runEvolveBlockedList(cmd *cobra.Command, cwd string) error {
	path := filepath.Join(cwd, evolveBlockedRelDir, evolveBlockedLogName)
	events, err := readBlockedEvents(path)
	if err != nil {
		return err
	}
	tail := evolveBlockedTail
	if tail <= 0 {
		tail = evolveBlockedDefTail
	}
	if len(events) > tail {
		events = events[len(events)-tail:]
	}
	if evolveBlockedJSON {
		return writeBlockedEventsJSON(cmd.OutOrStdout(), events)
	}
	return writeBlockedEventsHuman(cmd.OutOrStdout(), events)
}

func runEvolveBlockedClear(cmd *cobra.Command, cwd string) error {
	// Operators may use --clear under any mode, but warn if mode_default=loop.
	prefs, prefsErr := evolve.LoadFromDir(cmd.Context(), cwd)
	if prefsErr == nil && prefs != nil && prefs.ModeDefault == evolveModeLoop {
		fmt.Fprintln(cmd.ErrOrStderr(), "ao evolve blocked --clear: warning — preferences indicate --mode=loop; clearing is operator-only")
	}
	path := filepath.Join(cwd, evolveBlockedRelDir, evolveBlockedLogName)
	events, err := readBlockedEvents(path)
	if err != nil {
		return err
	}
	kept := make([]BlockedEvent, 0, len(events))
	removed := 0
	for _, ev := range events {
		if ev.Cycle == evolveBlockedClearCycleID {
			removed++
			continue
		}
		kept = append(kept, ev)
	}
	if removed == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No blocked events matched cycle %s\n", evolveBlockedClearCycleID)
		return nil
	}
	if err := rewriteBlockedEvents(path, kept); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Cleared %d blocked event(s) for cycle %s\n", removed, evolveBlockedClearCycleID)
	return nil
}

// appendBlockedEvent serializes event as one JSON line and appends to path,
// creating the directory if needed.
func appendBlockedEvent(path string, event BlockedEvent) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create blocked log dir %s: %w", dir, err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open blocked log %s: %w", path, err)
	}
	defer f.Close()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal blocked event: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append blocked event: %w", err)
	}
	return nil
}

// readBlockedEvents loads all JSONL records from path. Missing file returns
// an empty slice (not an error). Malformed lines are rejected with a typed
// error referencing the line number.
func readBlockedEvents(path string) ([]BlockedEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []BlockedEvent{}, nil
		}
		return nil, fmt.Errorf("open blocked log %s: %w", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var events []BlockedEvent
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev BlockedEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return nil, fmt.Errorf("%s:%d: malformed JSONL: %w", path, lineNo, err)
		}
		if ev.Cycle == "" || ev.Timestamp == "" || ev.Reason == "" {
			return nil, fmt.Errorf("%s:%d: missing required field(s) cycle/timestamp/reason", path, lineNo)
		}
		events = append(events, ev)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan blocked log: %w", err)
	}
	return events, nil
}

// rewriteBlockedEvents writes events back to path, atomically (tmp + rename).
func rewriteBlockedEvents(path string, events []BlockedEvent) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open tmp %s: %w", tmp, err)
	}
	for _, ev := range events {
		data, err := json.Marshal(ev)
		if err != nil {
			f.Close()
			os.Remove(tmp)
			return fmt.Errorf("marshal blocked event: %w", err)
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			f.Close()
			os.Remove(tmp)
			return fmt.Errorf("write tmp: %w", err)
		}
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

// writeBlockedEventsJSON emits a JSON array of events.
func writeBlockedEventsJSON(w io.Writer, events []BlockedEvent) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(events); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

// writeBlockedEventsHuman emits a human-readable summary.
func writeBlockedEventsHuman(w io.Writer, events []BlockedEvent) error {
	if len(events) == 0 {
		fmt.Fprintln(w, "(no blocked events)")
		return nil
	}
	for _, ev := range events {
		fmt.Fprintf(w, "%s  cycle=%s", ev.Timestamp, ev.Cycle)
		if ev.Bead != "" {
			fmt.Fprintf(w, " bead=%s", ev.Bead)
		}
		if ev.LadderStepFailed > 0 {
			fmt.Fprintf(w, " step=%d", ev.LadderStepFailed)
		}
		fmt.Fprintf(w, "\n  reason: %s\n", ev.Reason)
		if ev.NeededContext != "" {
			fmt.Fprintf(w, "  needed-context: %s\n", ev.NeededContext)
		}
	}
	return nil
}

// nextBlockedCycleID derives the default cycle id from the date plus a counter
// read from .agents/evolve/cron-history.jsonl. The counter is the number of
// rows in cron-history.jsonl; if the file is absent we use 0. The format is
// "<YYYY-MM-DD>-cycle-<N>".
func nextBlockedCycleID(cwd string, now time.Time) string {
	counter := countCronHistoryRows(filepath.Join(cwd, evolveCronHistoryRel))
	return fmt.Sprintf("%s-cycle-%d", now.UTC().Format("2006-01-02"), counter)
}

// countCronHistoryRows returns the number of non-blank lines in path; 0 on
// missing or unreadable file (the cycle counter is a soft default).
func countCronHistoryRows(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			n++
		}
	}
	return n
}
