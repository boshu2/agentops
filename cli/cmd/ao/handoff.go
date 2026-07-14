// practices: [event-sourcing-cqrs, distributed-tracing]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/trackerexec"
	"github.com/spf13/cobra"
)

// handoffArtifact is the user-facing handoff artifact for session boundary isolation.
// Distinct from phaseHandoff (orchestrator-internal) — this is written by `ao handoff`.
type handoffArtifact struct {
	SchemaVersion     int           `json:"schema_version"`
	ID                string        `json:"id"`
	CreatedAt         string        `json:"created_at"`
	Type              string        `json:"type"` // manual, auto, rpi
	Goal              string        `json:"goal,omitempty"`
	Summary           string        `json:"summary,omitempty"`
	Continuation      string        `json:"continuation,omitempty"`
	ArtifactsProduced []string      `json:"artifacts_produced,omitempty"`
	DecisionsMade     []string      `json:"decisions_made,omitempty"`
	OpenRisks         []string      `json:"open_risks,omitempty"`
	RPI               *handoffRPI   `json:"rpi"`
	State             *handoffState `json:"state"`
	Consumed          bool          `json:"consumed"`
	ConsumedAt        *string       `json:"consumed_at,omitempty"`
	ConsumedBy        *string       `json:"consumed_by,omitempty"`
}

// handoffRPI captures RPI phase context for session handoffs.
type handoffRPI struct {
	Phase     int               `json:"phase"`
	PhaseName string            `json:"phase_name"`
	EpicID    string            `json:"epic_id,omitempty"`
	RunID     string            `json:"run_id,omitempty"`
	Verdicts  map[string]string `json:"verdicts,omitempty"`
}

// handoffState captures git and bead state for session handoffs.
type handoffState struct {
	GitBranch      string   `json:"git_branch,omitempty"`
	GitDirty       bool     `json:"git_dirty"`
	ModifiedFiles  []string `json:"modified_files,omitempty"`
	ActiveBead     string   `json:"active_bead,omitempty"`
	OpenBeadsCount int      `json:"open_beads_count,omitempty"`
	RecentCommits  []string `json:"recent_commits,omitempty"`
	// Reservations are the active Agent Mail file reservations in the project,
	// captured so a rehydrating agent restores its lock landscape (ag-8c00a /
	// epic ag-2jp7l rehydrate-completeness). Best-effort: empty when AM absent.
	Reservations []string `json:"reservations,omitempty"`
}

var (
	handoffGoal     string
	handoffCollect  bool
	handoffRPIPhase int
	handoffEpicID   string
	handoffRunID    string
	handoffDryRun   bool
	handoffNoKill   bool
)

var handoffCmd = &cobra.Command{
	Use:   "handoff [summary]",
	Short: "Write a structured handoff artifact for session boundary isolation",
	Long: `Write a structured JSON handoff artifact that captures session context
for the next session to consume.

The handoff artifact includes goal, summary, continuation guidance,
artifacts produced, decisions made, open risks, and optional RPI/state context.

Examples:
  ao handoff "implemented auth module, tests passing"
  ao handoff --goal "build auth" "completed JWT flow"
  ao handoff --collect "finished feature X"
  ao handoff --rpi-phase 2 --epic na-abc "phase 2 complete"
  ao handoff --dry-run "preview handoff"
  ao handoff --no-kill "write artifact without restarting session"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHandoff,
}

// handoffAliasCmd is a hidden back-compat alias for `ao handoff` (the canonical
// spelling is `ao session handoff` since age-focus-membrane-bookkeeper-m1wg.17).
// Smoke/ratchet scripts and bundled callers still invoke `ao handoff`; it shares
// runHandoff + the same package-global flag vars, so both spellings behave
// identically.
var handoffAliasCmd = &cobra.Command{
	Use:        "handoff [summary]",
	Short:      handoffCmd.Short,
	Long:       handoffCmd.Long,
	Args:       cobra.MaximumNArgs(1),
	Hidden:     true,
	Deprecated: "use `ao session handoff`",
	RunE:       runHandoff,
}

// registerHandoffFlags binds the handoff flags to a command's FlagSet. Both the
// canonical `ao session handoff` and the hidden `ao handoff` alias share the same
// package-global vars, so registering on each FlagSet is safe.
func registerHandoffFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&handoffGoal, "goal", "", "What the session was working on")
	cmd.Flags().BoolVar(&handoffCollect, "collect", false, "Auto-collect git/bead state into the artifact")
	cmd.Flags().IntVar(&handoffRPIPhase, "rpi-phase", 0, "RPI phase number (populates RPI context, sets type=rpi)")
	cmd.Flags().StringVar(&handoffEpicID, "epic", "", "Epic ID for RPI context")
	cmd.Flags().StringVar(&handoffRunID, "run-id", "", "Run ID for RPI context")
	cmd.Flags().BoolVar(&handoffDryRun, "dry-run", false, "Print artifact to stdout without writing file")
	cmd.Flags().BoolVar(&handoffNoKill, "no-kill", false, "Write artifact without restarting the session via tmux")
}

func init() {
	// Folded under `ao session` (age-focus-membrane-bookkeeper-m1wg.17). The
	// GroupID was dropped because sessionCmd defines no command groups (cobra
	// panics at Execute if a child carries a GroupID its parent doesn't define).
	sessionCmd.AddCommand(handoffCmd)
	registerHandoffFlags(handoffCmd)

	rootCmd.AddCommand(handoffAliasCmd)
	registerHandoffFlags(handoffAliasCmd)
}

func runHandoff(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	now := time.Now().UTC()
	timestamp := now.Format("20060102T150405Z")

	artifact := handoffArtifact{
		SchemaVersion: 1,
		ID:            "handoff-" + timestamp,
		CreatedAt:     now.Format(time.RFC3339),
		Type:          "manual",
		Goal:          handoffGoal,
		Consumed:      false,
	}

	// Positional arg[0] is summary
	if len(args) > 0 {
		artifact.Summary = args[0]
	}

	// --collect: populate state
	if handoffCollect {
		artifact.State = collectHandoffStateContext(cmd.Context(), cwd)
		// Populate the continuation pointer from the captured state so a
		// rehydrating agent knows the next action (ag-8c00a). Don't clobber an
		// explicitly-passed continuation.
		if artifact.Continuation == "" {
			artifact.Continuation = deriveContinuation(artifact.State)
		}
	}

	// --rpi-phase: populate RPI context
	if handoffRPIPhase > 0 {
		artifact.Type = "rpi"
		artifact.RPI = buildHandoffRPIContext(cwd, handoffRPIPhase, handoffEpicID, handoffRunID)
	}

	// CRITICAL: --dry-run check BEFORE any file write (pre-mortem finding #1)
	if handoffDryRun {
		data, err := json.MarshalIndent(artifact, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal artifact: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Write artifact
	path, err := writeHandoffArtifact(cwd, &artifact)
	if err != nil {
		return fmt.Errorf("write handoff: %w", err)
	}
	fmt.Printf("Handoff written: %s\n", path)

	// Bridge decisions to learnings
	if len(artifact.DecisionsMade) > 0 || len(artifact.OpenRisks) > 0 {
		if err := bridgeHandoffToLearnings(cwd, &artifact); err != nil {
			fmt.Fprintf(os.Stderr, "warn: handoff\u2192learnings bridge: %v\n", err)
		}
	}

	// --no-kill: skip session restart
	if handoffNoKill {
		return nil
	}

	// Attempt tmux session restart
	if err := killSessionViaTmux(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Not in tmux — restart manually:\n")
		fmt.Fprintf(os.Stderr, "  exit\n")
		fmt.Fprintf(os.Stderr, "  cd %s && claude\n", cwd)
	}

	return nil
}

// collectHandoffState gathers git and bead state for the handoff artifact.
func collectHandoffState(cwd string) *handoffState {
	return collectHandoffStateContext(context.Background(), cwd)
}

func collectHandoffStateContext(ctx context.Context, cwd string) *handoffState {
	state := &handoffState{}

	// Git branch
	if branch, err := getCurrentBranch(cwd); err == nil {
		state.GitBranch = branch
	}

	// Modified files
	modified := gitChangedFiles(cwd, 20)
	state.ModifiedFiles = modified
	state.GitDirty = len(modified) > 0

	resolution, resolutionErr := resolveTracker(cwd, os.Environ())
	if resolutionErr == nil {
		// The claimed/active bead is the most recently updated in-progress item.
		inProgress := runResolvedBeadsTracker(ctx, resolution, 1500*time.Millisecond, "list", "--status", "in_progress", "--json")
		if bead := parseInProgressBeadID(inProgress); bead != "" {
			state.ActiveBead = bead
		}

		readyOut := runResolvedBeadsTracker(ctx, resolution, 1500*time.Millisecond, "ready", "--json")
		state.OpenBeadsCount = parseReadyCount(readyOut)
	}

	// Held Agent Mail file reservations — so a rehydrating agent restores its
	// lock landscape. Best-effort; empty when AM is absent.
	resOut := runCommand(cwd, 1500*time.Millisecond, "am", "robot", "reservations", "--project", cwd)
	state.Reservations = parseReservationPaths(resOut)

	// Recent commits
	recentLog := runCommand(cwd, 2*time.Second, "git", "log", "--oneline", "-5", "--no-decorate")
	if recentLog != "" {
		lines := strings.Split(recentLog, "\n")
		trimmed := make([]string, 0, len(lines))
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" {
				trimmed = append(trimmed, l)
			}
		}
		state.RecentCommits = trimmed
	}

	return state
}

// runBeadsTracker preserves the background-compatible adapter used by callers
// that do not own a live Cobra context.
func runBeadsTracker(cwd string, timeout time.Duration, args ...string) string {
	resolution, err := resolveTracker(cwd, os.Environ())
	if err != nil {
		return ""
	}
	return runResolvedBeadsTracker(context.Background(), resolution, timeout, args...)
}

func runResolvedBeadsTracker(ctx context.Context, resolution trackerResolution, timeout time.Duration, args ...string) string {
	out, err := execHandoffTracker(ctx, resolution, timeout, args...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

var execHandoffTracker = func(ctx context.Context, resolution trackerResolution, timeout time.Duration, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := (trackerexec.Factory{}).Command(ctx, resolution, args, trackerexec.Streams{})
	return command.Output()
}

// brIssue is the subset of `br list/ready --json` we read.
type brIssue struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}

// decodeBrIssues parses both br JSON shapes: the {"issues":[...]} wrapper and a
// bare [...] array. Returns nil on any parse error (caller degrades).
func decodeBrIssues(jsonOut string) []brIssue {
	s := strings.TrimSpace(jsonOut)
	if s == "" {
		return nil
	}
	var wrapped struct {
		Issues []brIssue `json:"issues"`
	}
	if err := json.Unmarshal([]byte(s), &wrapped); err == nil && wrapped.Issues != nil {
		return wrapped.Issues
	}
	var bare []brIssue
	if err := json.Unmarshal([]byte(s), &bare); err == nil {
		return bare
	}
	return nil
}

// parseInProgressBeadID returns the lane's active bead from `br list --status
// in_progress --json`. Since br has no per-session "current" pointer and
// in_progress is global (often many stale claims across a fleet), we use the
// MOST-RECENTLY-UPDATED in_progress bead as the best heuristic for "what this
// lane is working on" — the freshly-claimed bead has the newest updated_at.
// Empty if none. (RFC3339 timestamps sort lexicographically, so a string max
// is correct.)
func parseInProgressBeadID(jsonOut string) string {
	bestID, bestTS := "", ""
	for _, i := range decodeBrIssues(jsonOut) {
		if i.ID == "" {
			continue
		}
		if i.Status != "in_progress" && i.Status != "" {
			continue
		}
		// First candidate, or a strictly-newer update, wins.
		if bestID == "" || i.UpdatedAt > bestTS {
			bestID, bestTS = i.ID, i.UpdatedAt
		}
	}
	return bestID
}

// parseReadyCount counts ready beads from `br ready --json`.
func parseReadyCount(jsonOut string) int {
	return len(decodeBrIssues(jsonOut))
}

// parseReservationPaths returns "path [agent]" strings for each active file
// reservation in `am robot reservations` output. Empty when AM is absent or the
// output is unparseable.
func parseReservationPaths(jsonOut string) []string {
	s := strings.TrimSpace(jsonOut)
	if s == "" {
		return nil
	}
	var parsed struct {
		AllActive []struct {
			Agent string `json:"agent"`
			Path  string `json:"path"`
		} `json:"all_active"`
	}
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return nil
	}
	out := make([]string, 0, len(parsed.AllActive))
	for _, r := range parsed.AllActive {
		if r.Path == "" {
			continue
		}
		if r.Agent != "" {
			out = append(out, fmt.Sprintf("%s [%s]", r.Path, r.Agent))
		} else {
			out = append(out, r.Path)
		}
	}
	return out
}

// deriveContinuation builds a next-action pointer from the captured state so a
// rehydrating agent knows where to resume. Empty when there is no active bead —
// never fabricate a next action.
func deriveContinuation(state *handoffState) string {
	if state == nil || state.ActiveBead == "" {
		return ""
	}
	return fmt.Sprintf("Resume %s (run `BEADS_DIR=\"$(ao beads dir)\" br show %s --json` for the full acceptance + notes). %d bead(s) ready.",
		state.ActiveBead, state.ActiveBead, state.OpenBeadsCount)
}

// buildHandoffRPIContext reads phased state and constructs RPI context for the handoff.
func buildHandoffRPIContext(cwd string, phase int, epicID, runID string) *handoffRPI {
	phaseNames := map[int]string{1: "discovery", 2: "implementation", 3: "validation"}

	rpi := &handoffRPI{
		Phase:     phase,
		PhaseName: phaseNames[phase],
		EpicID:    epicID,
		RunID:     runID,
	}

	// Read verdicts from phased-state.json using anonymous struct (pre-mortem finding #6)
	statePath := filepath.Join(cwd, ".agents", "rpi", "phased-state.json")
	if data, err := os.ReadFile(statePath); err == nil {
		var ps struct {
			Verdicts map[string]string `json:"verdicts"`
			EpicID   string            `json:"epic_id"`
			RunID    string            `json:"run_id"`
		}
		if json.Unmarshal(data, &ps) == nil {
			rpi.Verdicts = ps.Verdicts
			// Fill from state if not provided via flags
			if rpi.EpicID == "" {
				rpi.EpicID = ps.EpicID
			}
			if rpi.RunID == "" {
				rpi.RunID = ps.RunID
			}
		}
	}

	return rpi
}

// writeHandoffArtifact atomically writes a handoff artifact to .agents/handoff/.
func writeHandoffArtifact(cwd string, artifact *handoffArtifact) (string, error) {
	dir := filepath.Join(cwd, ".agents", "handoff")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create handoff dir: %w", err)
	}

	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal handoff: %w", err)
	}
	data = append(data, '\n')

	target := filepath.Join(dir, artifact.ID+".json")
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		// Fallback: direct write if rename fails (cross-device)
		_ = os.Remove(tmp)
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return "", fmt.Errorf("write handoff: %w", err)
		}
	}
	return target, nil
}

// killSessionViaTmux restarts the Claude session via tmux respawn-pane.
func killSessionViaTmux(cwd string) error {
	pane := os.Getenv("TMUX_PANE")
	if pane == "" {
		return fmt.Errorf("not in tmux")
	}

	// Build restart command with env propagation (pre-mortem finding #2)
	var envParts []string
	envVars := []string{"ANTHROPIC_API_KEY", "AWS_PROFILE", "AWS_REGION", "CLAUDE_CODE_USE_BEDROCK"}
	for _, key := range envVars {
		if val := os.Getenv(key); val != "" {
			envParts = append(envParts, fmt.Sprintf("export %s=%s;", key, shellQuote(val)))
		}
	}

	restartCmd := fmt.Sprintf("cd %s && exec claude", shellQuote(cwd))
	if len(envParts) > 0 {
		restartCmd = strings.Join(envParts, " ") + " " + restartCmd
	}

	cmd := exec.Command("tmux", "respawn-pane", "-k", "-t", pane, restartCmd)
	return cmd.Run()
}
