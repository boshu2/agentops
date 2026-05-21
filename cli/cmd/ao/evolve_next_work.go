// practices: [dora-metrics, lean-startup]
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/evolve/ladder"
	"github.com/spf13/cobra"
)

// evolveNextWork subcommand (soc-mlbm) is the programmatic next-work ladder
// the /evolve loop consults each cycle. It traverses ready beads through the
// 5-step ladder in cli/internal/evolve/ladder, logs the decision, and emits
// either human-readable text or JSON for downstream consumers.
//
// See docs/plans/2026-05-21-evolve-loop-epic-design.md §A5.

const (
	evolveNextWorkLogRel = ".agents/evolve/next-work-decisions.jsonl"
)

var (
	evolveNextWorkMode             string
	evolveNextWorkIncludeOperator  bool
	evolveNextWorkJSON             bool
	evolveNextWorkBDBinary         string
	evolveNextWorkRunnerOverride   ladder.BeadRunner
	evolveNextWorkGrepOverride     ladder.GrepRunner
	evolveNextWorkClock            func() time.Time
)

var evolveNextWorkCmd = &cobra.Command{
	Use:   "next-work",
	Short: "Recommend the next bead to claim via the 5-step ladder",
	Long: `Run the 5-step next-work ladder and recommend a bead to claim.

The ladder filters operator-shape beads, enriches with sibling-pattern grep
hits, gates with the 3-question Primitive Test, falls back to cross-hop
discovered-from chains, and finally to the smallest bug. On exhaustion the
recommended bead is empty and the rationale tells the agent to call
'ao evolve blocked' instead of halting.

The full ladder spec lives at docs/plans/2026-05-21-evolve-loop-epic-design.md
§A5. Each cycle's decision is appended to .agents/evolve/next-work-decisions.jsonl.

Examples:
  ao evolve next-work
  ao evolve next-work --json
  ao evolve next-work --include-operator-shape
  ao evolve next-work --mode=loop`,
	Args: cobra.NoArgs,
	RunE: runEvolveNextWork,
}

func init() {
	evolveNextWorkClock = func() time.Time { return time.Now().UTC() }
	evolveNextWorkCmd.Flags().StringVar(&evolveNextWorkMode, "mode", evolveModeBurst, "Execution contract: 'burst' (default) or 'loop'")
	evolveNextWorkCmd.Flags().BoolVar(&evolveNextWorkIncludeOperator, "include-operator-shape", false, "Do not filter operator-shape beads at step 1")
	evolveNextWorkCmd.Flags().BoolVar(&evolveNextWorkJSON, "json", false, "Emit JSON instead of human-readable text")
	evolveNextWorkCmd.Flags().StringVar(&evolveNextWorkBDBinary, "bd-binary", "", "Override path to the 'bd' binary (default: resolves via PATH)")
	evolveCmd.AddCommand(evolveNextWorkCmd)
}

// nextWorkDecisionLogRow is the schema for one row in
// .agents/evolve/next-work-decisions.jsonl. The fields mirror the ladder
// Recommendation plus a timestamp.
type nextWorkDecisionLogRow struct {
	Timestamp         string   `json:"timestamp"`
	LadderStepMatched int      `json:"ladder_step_matched"`
	RecommendedBead   string   `json:"recommended_bead"`
	Alternatives      []string `json:"alternatives,omitempty"`
}

func runEvolveNextWork(cmd *cobra.Command, _ []string) error {
	if err := validateEvolveMode(evolveNextWorkMode); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	br := evolveNextWorkRunnerOverride
	if br == nil {
		br = ladder.ExecBeadRunner{BinaryPath: evolveNextWorkBDBinary}
	}
	gr := evolveNextWorkGrepOverride
	if gr == nil {
		gr = ladder.ExecGrepRunner{}
	}

	rec, err := ladder.Run(cmd.Context(), br, gr, ladder.Config{
		IncludeOperatorShape: evolveNextWorkIncludeOperator,
		RepoRoot:             cwd,
	})
	if err != nil {
		return fmt.Errorf("ladder run: %w", err)
	}

	if err := appendNextWorkDecision(cwd, rec, evolveNextWorkClock()); err != nil {
		// Logging failure should not block recommendation surfacing; surface
		// to stderr but proceed with stdout output per the principle that the
		// recommendation is the load-bearing artifact.
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: append next-work-decisions log: %v\n", err)
	}

	return writeNextWorkRecommendation(cmd.OutOrStdout(), rec, evolveNextWorkJSON)
}

// appendNextWorkDecision writes one row to the per-cycle decision log.
func appendNextWorkDecision(cwd string, rec ladder.Recommendation, now time.Time) error {
	row := nextWorkDecisionLogRow{
		Timestamp:         now.Format(time.RFC3339),
		LadderStepMatched: rec.LadderStepMatched,
		RecommendedBead:   rec.RecommendedBead,
		Alternatives:      rec.Alternatives,
	}
	path := filepath.Join(cwd, evolveNextWorkLogRel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	data, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("marshal log row: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write log row: %w", err)
	}
	return nil
}

// writeNextWorkRecommendation emits rec to w in JSON or human form.
func writeNextWorkRecommendation(w io.Writer, rec ladder.Recommendation, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rec); err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
		return nil
	}
	if rec.RecommendedBead == "" {
		fmt.Fprintf(w, "next-work: (none) — %s\n", rec.Rationale)
		return nil
	}
	fmt.Fprintf(w, "next-work: %s (step %d)\n", rec.RecommendedBead, rec.LadderStepMatched)
	fmt.Fprintf(w, "  rationale: %s\n", rec.Rationale)
	if len(rec.Alternatives) > 0 {
		fmt.Fprintf(w, "  alternatives: ")
		for i, a := range rec.Alternatives {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			fmt.Fprintf(w, "%s", a)
		}
		fmt.Fprintln(w)
	}
	return nil
}
