package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

func init() {
	handoffCmd.GroupID = "workflow"
	rootCmd.AddCommand(handoffCmd)

	handoffCmd.Flags().StringVar(&handoffGoal, "goal", "", "What the session was working on")
	handoffCmd.Flags().BoolVar(&handoffCollect, "collect", false, "Auto-collect git/bead state into the artifact")
	handoffCmd.Flags().IntVar(&handoffRPIPhase, "rpi-phase", 0, "RPI phase number (populates RPI context, sets type=rpi)")
	handoffCmd.Flags().StringVar(&handoffEpicID, "epic", "", "Epic ID for RPI context")
	handoffCmd.Flags().StringVar(&handoffRunID, "run-id", "", "Run ID for RPI context")
	handoffCmd.Flags().BoolVar(&handoffDryRun, "dry-run", false, "Print artifact to stdout without writing file")
	handoffCmd.Flags().BoolVar(&handoffNoKill, "no-kill", false, "Write artifact without restarting the session via tmux")
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
		artifact.State = collectHandoffState(cwd)
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
	state := &handoffState{}

	// Git branch
	if branch, err := getCurrentBranch(cwd); err == nil {
		state.GitBranch = branch
	}

	// Modified files
	modified := gitChangedFiles(cwd, 20)
	state.ModifiedFiles = modified
	state.GitDirty = len(modified) > 0

	// Active bead
	activeBead := runCommand(cwd, 1200*time.Millisecond, "bd", "current")
	if activeBead != "" {
		state.ActiveBead = activeBead
	}

	// Open beads count
	openCountStr := runCommand(cwd, 1200*time.Millisecond, "bd", "ready", "--json")
	if openCountStr != "" {
		// Parse JSON array and count entries
		var beads []json.RawMessage
		if json.Unmarshal([]byte(openCountStr), &beads) == nil {
			state.OpenBeadsCount = len(beads)
		}
	}

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
