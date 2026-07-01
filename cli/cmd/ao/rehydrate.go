package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	rehydratePeek bool
	rehydrateJSON bool
)

var rehydrateCmd = &cobra.Command{
	Use:   "rehydrate",
	Short: "Restore a lane from its latest handoff (the read-side of handoff→clear→rehydrate)",
	Long: `Rehydrate emits a lane's re-bootstrap brief from the most recent handoff
artifact in .agents/handoff/ — the read-side complement to ` + "`ao handoff`" + `.

In the self-healing context loop, an agent (or a peer orchestrator that just
cleared this pane) runs ` + "`ao rehydrate`" + ` to restore the working thread without
re-deriving it: the goal, the active bead (with ` + "`BEADS_DIR=\"$(ao beads dir)\" br show`" + ` to read its
acceptance), the file reservations to re-acquire, the next action, and recent
commits. For full corpus orientation, follow with ` + "`ao session bootstrap`" + `.

By default the consumed handoff is marked consumed (feeding the mine→measure→
improve loop). Use --peek to read without consuming.

Examples:
  ao rehydrate            # restore from the latest handoff, mark consumed
  ao rehydrate --peek     # read the brief without consuming
  ao rehydrate --json     # emit the raw handoff artifact`,
	Args: cobra.NoArgs,
	RunE: runRehydrate,
}

// rehydrateAliasCmd is a hidden back-compat alias for `ao rehydrate` (the
// canonical spelling is `ao session rehydrate` since
// age-focus-membrane-bookkeeper-m1wg.17). Smoke/ratchet scripts and bundled
// callers still invoke `ao rehydrate`; it shares runRehydrate + the same
// package-global flag vars, so both spellings behave identically.
var rehydrateAliasCmd = &cobra.Command{
	Use:        "rehydrate",
	Short:      rehydrateCmd.Short,
	Long:       rehydrateCmd.Long,
	Args:       cobra.NoArgs,
	Hidden:     true,
	Deprecated: "use `ao session rehydrate`",
	RunE:       runRehydrate,
}

// registerRehydrateFlags binds the rehydrate flags to a command's FlagSet. Both
// the canonical `ao session rehydrate` and the hidden `ao rehydrate` alias share
// the same package-global vars, so registering on each FlagSet is safe.
func registerRehydrateFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&rehydratePeek, "peek", false, "Read the brief without marking the handoff consumed")
	cmd.Flags().BoolVar(&rehydrateJSON, "json", false, "Emit the raw handoff artifact as JSON")
}

func init() {
	sessionCmd.AddCommand(rehydrateCmd)
	registerRehydrateFlags(rehydrateCmd)

	rootCmd.AddCommand(rehydrateAliasCmd)
	registerRehydrateFlags(rehydrateAliasCmd)
}

func runRehydrate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	out := cmd.OutOrStdout()

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	path, err := pickLatestHandoff(cwd)
	if err != nil {
		// Graceful: no handoff is not an error condition — point at bootstrap.
		fmt.Fprintln(out, "rehydrate: no handoff to rehydrate from — run `ao session bootstrap` for a cold orientation.")
		return nil
	}

	data, err := os.ReadFile(path) // #nosec G304 -- path resolved from the repo's own .agents/handoff dir
	if err != nil {
		return fmt.Errorf("read handoff %s: %w", path, err)
	}
	var artifact handoffArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return fmt.Errorf("parse handoff %s: %w", path, err)
	}

	if rehydrateJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(artifact)
	}

	fmt.Fprintln(out, renderRehydrateBrief(&artifact))

	if !rehydratePeek && !artifact.Consumed {
		if err := markHandoffConsumed(path, &artifact); err != nil {
			// Non-fatal: the brief is what matters; consuming is the mining hook.
			fmt.Fprintf(out, "\n(note: could not mark handoff consumed: %v)\n", err)
		}
	}
	return nil
}

// pickLatestHandoff returns the path to the most recent handoff artifact under
// <cwd>/.agents/handoff/. Handoff names are timestamped (handoff-<RFC3339>.json)
// so they sort lexicographically; the max name is newest. Errors when none.
func pickLatestHandoff(cwd string) (string, error) {
	dir := filepath.Join(cwd, ".agents", "handoff")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("no handoff dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && strings.HasPrefix(n, "handoff-") && strings.HasSuffix(n, ".json") {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no handoff artifacts in %s", dir)
	}
	sort.Strings(names)
	return filepath.Join(dir, names[len(names)-1]), nil
}

// renderRehydrateBrief renders the lane's re-bootstrap brief from a handoff
// artifact. Never empty; omits lines for fields the handoff doesn't carry
// (don't invent a next action or a bead that isn't there).
func renderRehydrateBrief(a *handoffArtifact) string {
	var b strings.Builder
	b.WriteString("=== REHYDRATE — restore this lane ===\n")
	if a.Goal != "" {
		b.WriteString("Goal: " + a.Goal + "\n")
	}
	if a.State != nil {
		if a.State.ActiveBead != "" {
			fmt.Fprintf(&b, "Active bead: %s — run `BEADS_DIR=\"$(ao beads dir)\" br show %s --json` for its acceptance + notes.\n",
				a.State.ActiveBead, a.State.ActiveBead)
		}
		if len(a.State.Reservations) > 0 {
			b.WriteString("Re-acquire these file reservations (am file_reservations reserve):\n")
			for _, r := range a.State.Reservations {
				b.WriteString("  - " + r + "\n")
			}
		}
		if a.State.GitBranch != "" {
			b.WriteString("Branch: " + a.State.GitBranch + "\n")
		}
		if len(a.State.RecentCommits) > 0 {
			b.WriteString("Recent commits:\n")
			for _, c := range a.State.RecentCommits {
				b.WriteString("  " + c + "\n")
			}
		}
	}
	if a.Continuation != "" {
		b.WriteString("Next action: " + a.Continuation + "\n")
	}
	if a.Summary != "" {
		b.WriteString("\nPredecessor summary:\n" + a.Summary + "\n")
	}
	b.WriteString("\nFor full corpus orientation: `ao session bootstrap`.")
	return b.String()
}

// markHandoffConsumed sets consumed/consumed_at on the artifact and rewrites it
// atomically — the mine→measure→improve hook (a consumed handoff is evidence the
// flywheel can compound on).
func markHandoffConsumed(path string, a *handoffArtifact) error {
	a.Consumed = true
	now := time.Now().UTC().Format(time.RFC3339)
	a.ConsumedAt = &now
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil { // #nosec G306 -- non-secret handoff artifact
		return err
	}
	return os.Rename(tmp, path)
}
