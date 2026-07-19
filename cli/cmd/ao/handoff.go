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

// handoffArtifact is caller-authored session evidence. It deliberately carries
// no tracker, reservation, phase, retry, ownership, or next-work state.
type handoffArtifact struct {
	SchemaVersion int           `json:"schema_version"`
	ID            string        `json:"id"`
	CreatedAt     string        `json:"created_at"`
	Goal          string        `json:"goal,omitempty"`
	Summary       string        `json:"summary,omitempty"`
	Continuation  string        `json:"continuation,omitempty"`
	State         *handoffState `json:"state,omitempty"`
}

// handoffState contains optional read-only Git observations. Git availability
// never controls whether a handoff can be written or consumed.
type handoffState struct {
	GitBranch     string   `json:"git_branch,omitempty"`
	GitDirty      bool     `json:"git_dirty"`
	ModifiedFiles []string `json:"modified_files,omitempty"`
	RecentCommits []string `json:"recent_commits,omitempty"`
}

var (
	handoffGoal         string
	handoffContinuation string
	handoffCollect      bool
	handoffDryRun       bool
)

var handoffCmd = &cobra.Command{
	Use:   "handoff [summary]",
	Short: "Write caller-authored session evidence",
	Long: `Write a small handoff artifact without selecting work, claiming it,
deciding what happens next, or restarting a runtime. --collect adds only
best-effort, read-only Git observations.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHandoff,
}

func init() {
	// handoffCmd is attached to the session parent by newSessionCommand in
	// session_composition.go; the session parent now lives in the session module.
	handoffCmd.Flags().StringVar(&handoffGoal, "goal", "", "Caller-supplied goal")
	handoffCmd.Flags().StringVar(&handoffContinuation, "continuation", "", "Caller-supplied continuation note")
	handoffCmd.Flags().BoolVar(&handoffCollect, "collect", false, "Collect best-effort read-only Git observations")
	handoffCmd.Flags().BoolVar(&handoffDryRun, "dry-run", false, "Print the artifact without writing it")
}

func runHandoff(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	now := time.Now().UTC()
	artifact := handoffArtifact{
		SchemaVersion: 1,
		ID:            "handoff-" + now.Format("20060102T150405.000000000Z"),
		CreatedAt:     now.Format(time.RFC3339Nano),
		Goal:          handoffGoal,
		Continuation:  handoffContinuation,
	}
	if len(args) == 1 {
		artifact.Summary = args[0]
	}
	if handoffCollect {
		artifact.State = collectHandoffState(cwd)
	}

	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal handoff: %w", err)
	}
	data = append(data, '\n')
	if handoffDryRun {
		_, err = cmd.OutOrStdout().Write(data)
		return err
	}

	path, err := writeHandoffArtifact(cwd, &artifact, data)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Handoff written: %s\n", path)
	return nil
}

func collectHandoffState(cwd string) *handoffState {
	state := &handoffState{}
	if branch, err := getCurrentBranch(cwd); err == nil {
		state.GitBranch = branch
	}
	state.ModifiedFiles = gitChangedFiles(cwd, 20)
	state.GitDirty = len(state.ModifiedFiles) > 0
	command := exec.Command("git", "log", "--oneline", "-5", "--no-decorate")
	command.Dir = cwd
	command.Env = gitDiscoveryEnv()
	if output, err := command.Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				state.RecentCommits = append(state.RecentCommits, line)
			}
		}
	}
	return state
}

func writeHandoffArtifact(cwd string, artifact *handoffArtifact, data []byte) (string, error) {
	dir := filepath.Join(cwd, ".agents", "handoff")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create handoff directory: %w", err)
	}
	target := filepath.Join(dir, artifact.ID+".json")
	tmp, err := os.CreateTemp(dir, ".handoff-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create handoff temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write handoff: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("flush handoff: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close handoff: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return "", fmt.Errorf("publish handoff: %w", err)
	}
	return target, nil
}
