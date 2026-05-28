// practices: [design-by-contract, code-complete]
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/skills"
)

var (
	skillsFindJSON  bool
	skillsFindLimit int
)

var skillsFindCmd = &cobra.Command{
	Use:   "find <intent>",
	Short: "Rank skills by how well they match a free-text intent",
	Long: `Score every skills/<name>/SKILL.md against a free-text intent and
print the best matches by name, description, and score. Turns the skill
catalog from oral tradition into a queryable surface — give an agent a
discovery API instead of memorizing every skill name.

Scoring is a transparent token-overlap: a query word hitting the skill
name counts most, a declared trigger next, a description word least, with
light plural/stem tolerance. Scores are normalized to [0,1]. The catalog
is read from the tree on every run, so a newly added skill is found with
no index rebuild.

Examples:
  ao skills find "close the loop"
  ao skills find autonomous improvement loop --limit 3
  ao skills find "release validation" --json`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSkillsFind,
}

func init() {
	skillsCmd.AddCommand(skillsFindCmd)
	skillsFindCmd.Flags().BoolVar(&skillsFindJSON, "json", false, "Emit machine-readable JSON on stdout")
	skillsFindCmd.Flags().IntVar(&skillsFindLimit, "limit", 5, "Maximum number of results to return")
}

func runSkillsFind(cmd *cobra.Command, args []string) error {
	if skillsFindLimit < 1 {
		cmd.SilenceUsage = true
		return fmt.Errorf("--limit must be >= 1 (got %d); rerun with e.g. --limit 5", skillsFindLimit)
	}

	query := joinArgs(args)
	skillsDir, _ := resolveSkillsRoots()
	metas, err := skills.Load(skillsDir)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("load skills from %s: %w; run `ao skills check` to inspect the tree", skillsDir, err)
	}

	ranked := skills.Score(query, metas)
	top := capResults(ranked, skillsFindLimit)

	if skillsFindJSON {
		return renderFindJSON(cmd, top)
	}
	return renderFindText(cmd, query, top)
}

// joinArgs concatenates positional args into a single query string so both
// quoted ("close the loop") and bare (close the loop) forms work.
func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}

// capResults returns at most limit matches.
func capResults(matches []skills.Match, limit int) []skills.Match {
	if len(matches) > limit {
		return matches[:limit]
	}
	return matches
}

// renderFindJSON writes the matches as a JSON array on stdout (data only;
// diagnostics, if any, go to stderr).
func renderFindJSON(cmd *cobra.Command, top []skills.Match) error {
	if top == nil {
		top = []skills.Match{}
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(top)
}

// renderFindText prints the positive-score matches as a ranked list on stdout.
// When nothing scores above zero it reports gracefully on stderr and exits 0 —
// an unmatched intent is not an error.
func renderFindText(cmd *cobra.Command, query string, top []skills.Match) error {
	shown := 0
	out := cmd.OutOrStdout()
	for _, m := range top {
		if m.Score <= 0 {
			continue
		}
		shown++
		fmt.Fprintf(out, "%d. %-28s %.3f  %s\n", shown, m.Name, m.Score, m.Description)
	}
	if shown == 0 {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"no strong matches for %q — try a broader intent or fewer words\n", query)
	}
	return nil
}
