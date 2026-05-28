// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/corpus"
)

var corpusFitnessJSON bool

// corpusCmd is the root for corpus-quality probes consumed by Dream and
// operators inspecting the local .agents/ corpus.
var corpusCmd = &cobra.Command{
	Use:   "corpus",
	Short: "Corpus-quality probes for Dream and the knowledge flywheel",
	Long: `Commands that inspect the local .agents/ corpus quality.

These commands are used by Dream's nightly MEASURE stage via in-process
calls and are also exposed for operators who want to inspect fitness
manually. They are deliberately NOT plumbed through the goals directive
subsystem — see docs/contracts/dream-run-contract.md for the delineation.`,
}

// corpusFitnessCmd computes and prints the corpus FitnessVector for the
// current working directory's .agents/ corpus.
var corpusFitnessCmd = &cobra.Command{
	Use:   "fitness",
	Short: "Compute the corpus-quality fitness vector for the current .agents/",
	RunE:  runCorpusFitness,
}

func init() {
	corpusCmd.GroupID = "knowledge"
	rootCmd.AddCommand(corpusCmd)
	corpusCmd.AddCommand(corpusFitnessCmd)
	corpusFitnessCmd.Flags().BoolVar(&corpusFitnessJSON, "json", false, "Emit the fitness vector as JSON")
}

// runCorpusFitness is the RunE entry point for `ao corpus fitness`.
func runCorpusFitness(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("corpus fitness: determining cwd: %w", err)
	}
	vec, degraded, err := corpus.Compute(cwd)
	if err != nil {
		return fmt.Errorf("corpus fitness: %w", err)
	}
	if corpusFitnessJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Fitness  *corpus.FitnessVector `json:"fitness"`
			Degraded []string              `json:"degraded,omitempty"`
		}{vec, degraded})
	}
	fmt.Printf("Corpus fitness (computed %s):\n", vec.ComputedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  retrieval_precision:            %.3f\n", vec.RetrievalPrecision)
	fmt.Printf("  retrieval_recall:               %.3f\n", vec.RetrievalRecall)
	fmt.Printf("  maturity_provisional_or_above:  %.3f\n", vec.MaturityProvisional)
	fmt.Printf("  unresolved_findings:            %d\n", vec.UnresolvedFindings)
	fmt.Printf("  citation_coverage:              %.3f\n", vec.CitationCoverage)
	fmt.Printf("  inject_visibility:              %.3f\n", vec.InjectVisibility)
	fmt.Printf("  cross_rig_dedup_ratio:          %.3f\n", vec.CrossRigDedupRatio)
	if len(degraded) > 0 {
		fmt.Println("\nDegraded:")
		for _, d := range degraded {
			fmt.Printf("  - %s\n", d)
		}
	}
	return nil
}
