// This file implements `ao wiki publish` — the membrane seam over the gold
// compiler (age-port-openkb-into-agentops-go-5qw.9, first slice).
//
// --dry-run compiles a fresh publish candidate, computes its stable content
// digest, and runs the canonical leak scan (corpusscan) over the candidate
// tree, FAILING CLOSED on any hit. This is the view-first product AND the first
// membrane signal: nothing is published, but the operator sees exactly what
// WOULD publish and whether it is leak-clean.
//
// Real (non-dry-run) publish is gated on a CONFIRMED verdict bound to the
// content digest. How that verdict is represented in the hash-chained
// provenance ledger is an open design decision (bead age-xf9r); until it lands,
// real publish is refused with a pointer to that bead rather than guessed at.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/corpusscan"
	"github.com/boshu2/agentops/cli/internal/wiki"
)

var (
	wikiPublishDryRun bool
	wikiPublishJSON   bool
	wikiPublishFloor  float64
)

var wikiPublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Compute + leak-scan a gold-wiki publish candidate (membrane seam; --dry-run)",
	Long: `ao wiki publish is the membrane gate over the gold wiki.

--dry-run (required today) compiles a fresh publish candidate from .agents/,
computes a STABLE content digest (the identity a publish verdict binds to), and
runs the canonical leak scan (the same marker registry as ` + "`ao corpus scan`" + `)
over the candidate tree. It FAILS CLOSED on any leak: a candidate carrying a
fleet/client/brand/myth marker is NOT publishable.

Real publish is gated on a CONFIRMED verdict for the content digest. Its ledger
representation is an open design decision (bead age-xf9r); until that lands,
real publish is refused rather than guessed at. Run with --dry-run.`,
	Args: cobra.NoArgs,
	RunE: runWikiPublish,
}

func runWikiPublish(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	if !wikiPublishDryRun {
		return fmt.Errorf("real publish is gated on a CONFIRMED verdict for the content digest, whose ledger representation is an open design decision (bead age-xf9r) — run with --dry-run to compute the candidate digest + leak scan")
	}

	base, err := os.Getwd()
	if err != nil {
		return err
	}
	agentsDir := wiki.AgentsDirIn(base)
	if fi, err := os.Stat(agentsDir); err != nil || !fi.IsDir() {
		return fmt.Errorf("no .agents/ corpus at %s", agentsDir)
	}

	cand, err := wiki.CompilePublishCandidate(agentsDir, wikiPublishFloor)
	if err != nil {
		return err
	}
	defer cand.Cleanup()

	rep, err := corpusscan.Scan(cand.OutDir)
	if err != nil {
		return fmt.Errorf("leak scan: %w", err)
	}

	out := cmd.OutOrStdout()
	if wikiPublishJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"digest":      cand.Digest,
			"promoted":    cand.Stats.Promoted,
			"rejected":    cand.Stats.Rejected,
			"redactions":  cand.Stats.Redactions,
			"leak_clean":  rep.Clean(),
			"leak_hits":   rep.HitCount(),
			"publishable": rep.Clean(),
		})
	} else {
		fmt.Fprintf(out, "candidate digest: %s\n", cand.Digest)
		fmt.Fprintf(out, "promoted %d · gated %d · redactions %d\n",
			cand.Stats.Promoted, cand.Stats.Rejected, cand.Stats.Redactions)
		if rep.Clean() {
			fmt.Fprintf(out, "leak-scan: CLEAN — publishable (real publish pending verdict gate, age-xf9r)\n")
		} else {
			fmt.Fprintf(out, "leak-scan: FAIL CLOSED — %d hit(s), NOT publishable\n", rep.HitCount())
			for _, f := range rep.Files {
				for _, h := range f.Hits {
					fmt.Fprintf(out, "  %s:%d  [%s/%s]\n", f.Path, h.Line, h.Class, h.Marker)
				}
			}
		}
	}

	// Fail closed: a leaked candidate exits non-zero so a caller (and a future
	// real-publish gate) can never treat a dirty candidate as publishable.
	if !rep.Clean() {
		return fmt.Errorf("publish candidate FAILED leak scan: %d hit(s) — not publishable", rep.HitCount())
	}
	return nil
}

func init() {
	wikiPublishCmd.Flags().BoolVar(&wikiPublishDryRun, "dry-run", false, "compute + leak-scan the candidate without publishing (required; real publish is gated on age-xf9r)")
	wikiPublishCmd.Flags().BoolVar(&wikiPublishJSON, "json", false, "emit the candidate report as JSON")
	wikiPublishCmd.Flags().Float64Var(&wikiPublishFloor, "confidence-floor", 0, "override the promotion confidence floor")
	wikiCmd.AddCommand(wikiPublishCmd)
}
