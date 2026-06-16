// This file implements `ao wiki gold` — the publish step of the knowledge
// flywheel. It mines the private .agents/ corpus into a sanitized,
// durability-gated, OKF-compliant wiki under .ao/wiki/ (the gold layer).
//
// See cli/internal/wiki/gold.go for the GoldCompiler that does the work.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/wiki"
)

var (
	goldOut    string
	goldFloor  float64
	goldDryRun bool
	goldJSON   bool
)

var wikiGoldCmd = &cobra.Command{
	Use:   "gold",
	Short: "Compile .agents/ into a sanitized OKF wiki under .ao/wiki/ (experimental)",
	Long: `ao wiki gold is the raw-lead-to-gold bridge of the knowledge flywheel.

It mines the private .agents/ corpus (the "raw" layer) into a public-safe,
OKF-compliant wiki in .ao/wiki/ (the "gold" layer):

  MINE     promotion gate — only durable entries cross (maturity/tier/
           rewards, or confidence >= floor). Provisional noise is gated out,
           never silently: every rejection is counted with a reason.
  SANITIZE canonical secret + $HOME redaction (llm.Redact) plus session-UUID
           scrub, applied BEFORE anything is written.
  EMIT     OKF: per-doc frontmatter (type required + title/description/tags/
           timestamp/status/confidence/source_digest), file-path identity,
           reserved index.md (catalog) and log.md (history).
  LINT     minimal OKF conformance over the emitted tree.

OKF = Google Cloud's Open Knowledge Format, the portable interoperable
formalization of Karpathy's LLM-wiki pattern. Unlike ao compile (which renders
.agents/ -> .agents/compiled/ for in-repo agents), gold is the sanitized,
durability-gated, portable PUBLISH step.

EXPERIMENTAL.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		base, err := os.Getwd()
		if err != nil {
			return err
		}
		agentsDir := wiki.AgentsDirIn(base)
		if fi, err := os.Stat(agentsDir); err != nil || !fi.IsDir() {
			return fmt.Errorf("no .agents/ corpus at %s", agentsDir)
		}
		out := goldOut
		if !filepath.IsAbs(out) {
			out = filepath.Join(base, out)
		}
		gc := &wiki.GoldCompiler{
			AgentsDir:       agentsDir,
			OutDir:          out,
			ConfidenceFloor: goldFloor,
		}
		stats, err := gc.Compile(goldDryRun)
		if err != nil {
			return err
		}

		if goldJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(stats)
		}

		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "scanned   %d raw entries\n", stats.Scanned)
		fmt.Fprintf(w, "promoted  %d  (lead -> gold)\n", stats.Promoted)
		fmt.Fprintf(w, "gated out %d\n", stats.Rejected)
		reasons := make([]string, 0, len(stats.Rejections))
		for r := range stats.Rejections {
			reasons = append(reasons, r)
		}
		sort.Slice(reasons, func(a, b int) bool {
			return stats.Rejections[reasons[a]] > stats.Rejections[reasons[b]]
		})
		for _, r := range reasons {
			fmt.Fprintf(w, "            %3d  %s\n", stats.Rejections[r], r)
		}
		fmt.Fprintf(w, "redactions %d secret/private spans scrubbed\n", stats.Redactions)
		fmt.Fprintf(w, "links     %d OKF cross-links woven\n", stats.Links)
		fmt.Fprintf(w, "by OKF type: %v\n", stats.ByType)
		if !goldDryRun {
			if len(stats.Lint) > 0 {
				fmt.Fprintf(w, "LINT: %d problem(s):\n", len(stats.Lint))
				for _, p := range stats.Lint {
					fmt.Fprintf(w, "  - %s\n", p)
				}
			} else {
				fmt.Fprintln(w, "LINT: OKF-clean")
			}
			fmt.Fprintf(w, "wrote -> %s\n", out)
		}
		return nil
	},
}

func init() {
	wikiGoldCmd.Flags().StringVar(&goldOut, "out", ".ao/wiki", "output directory for the OKF gold wiki")
	wikiGoldCmd.Flags().Float64Var(&goldFloor, "confidence-floor", 0, "override the promotion confidence floor (default 0.70)")
	wikiGoldCmd.Flags().BoolVar(&goldDryRun, "dry-run", false, "report what would be promoted without writing")
	wikiGoldCmd.Flags().BoolVar(&goldJSON, "json", false, "emit stats as JSON")
	wikiCmd.AddCommand(wikiGoldCmd)
}
