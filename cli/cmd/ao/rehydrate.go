package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var rehydrateJSON bool

var rehydrateCmd = &cobra.Command{
	Use:   "rehydrate",
	Short: "Read the latest caller-authored handoff",
	Long:  "Read a handoff without consuming it, claiming work, or choosing a next action.",
	Args:  cobra.NoArgs,
	RunE:  runRehydrate,
}

func init() {
	sessionCmd.AddCommand(rehydrateCmd)
	rehydrateCmd.Flags().BoolVar(&rehydrateJSON, "json", false, "Emit the stored artifact as JSON")
}

func runRehydrate(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	path, err := pickLatestHandoff(cwd)
	if err != nil {
		fmt.Fprintln(cmd.OutOrStdout(), "rehydrate: no handoff found")
		return nil
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is selected from the local handoff directory
	if err != nil {
		return fmt.Errorf("read handoff: %w", err)
	}
	var artifact handoffArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return fmt.Errorf("parse handoff: %w", err)
	}
	if rehydrateJSON {
		_, err = cmd.OutOrStdout().Write(data)
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), renderRehydrateBrief(&artifact))
	return nil
}

func pickLatestHandoff(cwd string) (string, error) {
	dir := filepath.Join(cwd, ".agents", "handoff")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasPrefix(name, "handoff-") && strings.HasSuffix(name, ".json") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no handoff artifacts")
	}
	sort.Strings(names)
	return filepath.Join(dir, names[len(names)-1]), nil
}

func renderRehydrateBrief(artifact *handoffArtifact) string {
	var lines []string
	if artifact.Goal != "" {
		lines = append(lines, "Goal: "+artifact.Goal)
	}
	if artifact.Summary != "" {
		lines = append(lines, "Summary: "+artifact.Summary)
	}
	if artifact.Continuation != "" {
		lines = append(lines, "Caller continuation: "+artifact.Continuation)
	}
	if artifact.State != nil && artifact.State.GitBranch != "" {
		lines = append(lines, "Observed branch: "+artifact.State.GitBranch)
	}
	if len(lines) == 0 {
		return "Handoff contains no caller-authored brief."
	}
	return strings.Join(lines, "\n")
}
