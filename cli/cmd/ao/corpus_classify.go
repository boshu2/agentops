//go:build flywheel

// practices: [fail-closed-safety, wiki-knowledge-surface]
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/corpus"
)

var (
	corpusClassifyApply bool
	corpusClassifyJSON  bool
)

// corpusClassifyCmd annotates learning records with the two promote-gate
// frontmatter defaults (sensitivity=unknown, publishable=false) — the S3 seam
// migration (epic ag-k7tq9). Dry-run by default; --apply writes.
var corpusClassifyCmd = &cobra.Command{
	Use:   "classify <dir>",
	Short: "Annotate learning frontmatter with promote-gate defaults (sensitivity, publishable)",
	Long: `Ensure every learning record under <dir> carries the two promote-gate
frontmatter fields with SAFE defaults:

  sensitivity: unknown   # un-triaged ceiling; not a capture property
  publishable: false     # promotion allowlist flag; inclusion is earned

This is the field-level seam migration from the corpus public/private council
verdict (.agents/council/2026-06-15-corpus-private-public-seam-verdict.md): the
corpus is lossless and private-by-default, and only sensitivity==public AND
publishable==true items may later be promoted to the public wiki (allowlist,
fail-closed — default excludes).

It is malformed-tolerant: it operates on the frontmatter fence textually and
never parses the (possibly broken) YAML body, so a single junk record cannot
abort the run. An existing real decision (any sensitivity/publishable value) is
never overwritten. Meta docs (CORPUS-POLICY.md, README.md, …) are skipped.

Dry-run by default — prints what WOULD change. Pass --apply to write.

  ao corpus classify .agents/learnings            # dry run
  ao corpus classify .agents/learnings --apply    # write defaults`,
	Args: cobra.ExactArgs(1),
	RunE: runCorpusClassify,
}

func init() {
	corpusCmd.AddCommand(corpusClassifyCmd)
	corpusClassifyCmd.Flags().BoolVar(&corpusClassifyApply, "apply", false, "Write the changes (default: dry run, report only)")
	corpusClassifyCmd.Flags().BoolVar(&corpusClassifyJSON, "json", false, "Emit the report as JSON")
}

// runCorpusClassify is the RunE entry point for `ao corpus classify`.
func runCorpusClassify(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	dir := args[0]
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("corpus classify: %q is not a directory", dir)
	}

	res, err := corpus.ClassifyDir(dir, corpusClassifyApply)
	if err != nil {
		return fmt.Errorf("corpus classify: %w", err)
	}

	if corpusClassifyJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	mode := "dry run — no files written (pass --apply to write)"
	if res.Applied {
		mode = "applied"
	}
	fmt.Printf("Corpus classify (%s):\n", mode)
	fmt.Printf("  scanned learnings: %d\n", res.Scanned)
	fmt.Printf("  skipped meta docs: %d\n", res.Skipped)
	fmt.Printf("  needing defaults:  %d\n", res.Changed)
	if res.Changed > 0 && !res.Applied {
		fmt.Println("\nWould annotate:")
		for _, f := range res.ChangedFiles {
			fmt.Printf("  - %s\n", f)
		}
	}
	return nil
}
