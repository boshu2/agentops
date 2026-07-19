// Package sessionapp owns the filesystem effects behind the `ao session`
// evidence commands. It reports local orientation files (bootstrap) and reads
// the latest caller-authored handoff without consuming it (rehydrate), keeping
// the session command module a thin Cobra presentation seam that performs no
// direct filesystem effect.
package sessionapp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// orientationCandidates is the fixed, ordered set of local orientation files a
// bootstrap report probes. The order is the reported order.
var orientationCandidates = []string{"AGENTS.md", "README.md", "PRODUCT.md", "GOALS.md", "PROGRAM.md"}

// BootstrapStatus is the machine-readable bootstrap report. It carries only the
// workspace directory and the orientation files that exist there; it never
// starts runtimes, probes trackers, selects work, or inspects queues.
type BootstrapStatus struct {
	Workspace        string   `json:"workspace"`
	OrientationFiles []string `json:"orientation_files"`
}

// BootstrapOptions carries the presentation choices resolved by the command
// module. The working directory is resolved inside Bootstrap so the module
// never performs a direct filesystem effect.
type BootstrapOptions struct {
	// JSON selects machine-readable output when true.
	JSON bool
	// Stdout receives the rendered report.
	Stdout io.Writer
}

// Bootstrap reports the local orientation files available in the working
// directory. It renders JSON under opts.JSON, otherwise a human list.
func Bootstrap(opts BootstrapOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	status := BootstrapStatus{Workspace: cwd, OrientationFiles: []string{}}
	for _, relative := range orientationCandidates {
		if info, err := os.Stat(filepath.Join(cwd, relative)); err == nil && info.Mode().IsRegular() {
			status.OrientationFiles = append(status.OrientationFiles, relative)
		}
	}
	if opts.JSON {
		encoder := json.NewEncoder(opts.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(status)
	}
	fmt.Fprintf(opts.Stdout, "workspace: %s\n", status.Workspace)
	for _, relative := range status.OrientationFiles {
		fmt.Fprintf(opts.Stdout, "- %s\n", relative)
	}
	return nil
}

// storedHandoff decodes the caller-authored handoff artifact for the brief. It
// mirrors the on-disk handoff shape's read side; rehydrate never writes, so the
// writer-side type stays with the `ao session handoff` command.
type storedHandoff struct {
	Goal         string             `json:"goal,omitempty"`
	Summary      string             `json:"summary,omitempty"`
	Continuation string             `json:"continuation,omitempty"`
	State        *storedHandoffState `json:"state,omitempty"`
}

// storedHandoffState is the optional read-only Git observation block. Only the
// branch is surfaced in the human brief.
type storedHandoffState struct {
	GitBranch string `json:"git_branch,omitempty"`
}

// RehydrateOptions carries the presentation choices resolved by the command
// module. The working directory is resolved inside Rehydrate so the module
// never performs a direct filesystem effect.
type RehydrateOptions struct {
	// JSON emits the stored artifact bytes verbatim (or `{}` for the empty
	// state) so `ao session rehydrate --json | jq` never breaks.
	JSON bool
	// Stdout receives the artifact or brief.
	Stdout io.Writer
	// Stderr receives the human "no handoff found" hint under JSON.
	Stderr io.Writer
}

// Rehydrate reads the latest caller-authored handoff without consuming it,
// claiming work, or choosing a next action. Under JSON the empty state is
// exactly one `{}` document on stdout with the hint on stderr; either way it
// exits without error.
func Rehydrate(opts RehydrateOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	path, err := pickLatestHandoff(cwd)
	if err != nil {
		// Under --json, stdout must be exactly one JSON document (`{}` for the
		// empty state) so `ao session rehydrate --json | jq` never breaks; the
		// human hint goes to stderr. Exit 0 either way.
		if opts.JSON {
			fmt.Fprintln(opts.Stderr, "rehydrate: no handoff found")
			fmt.Fprintln(opts.Stdout, "{}")
			return nil
		}
		fmt.Fprintln(opts.Stdout, "rehydrate: no handoff found")
		return nil
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is selected from the local handoff directory
	if err != nil {
		return fmt.Errorf("read handoff: %w", err)
	}
	var artifact storedHandoff
	if err := json.Unmarshal(data, &artifact); err != nil {
		return fmt.Errorf("parse handoff: %w", err)
	}
	if opts.JSON {
		_, err = opts.Stdout.Write(data)
		return err
	}
	fmt.Fprintln(opts.Stdout, renderBrief(&artifact))
	return nil
}

// pickLatestHandoff returns the newest handoff artifact by lexical name order.
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

// renderBrief renders the caller-authored brief. It surfaces only the goal,
// summary, continuation, and observed branch; it never invents lifecycle state.
func renderBrief(artifact *storedHandoff) string {
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
