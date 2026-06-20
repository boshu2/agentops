// This file implements `ao wiki publish` — the membrane gate over the gold
// compiler (age-port-openkb-into-agentops-go-5qw.9).
//
// --dry-run compiles a fresh publish candidate, computes its stable content
// digest, and runs the canonical leak scan (corpusscan) over the candidate
// tree, FAILING CLOSED on any hit. Nothing is published — the operator sees
// exactly what WOULD publish and whether it is leak-clean.
//
// Real publish (with --bead) is gated, per the cross-family council decision on
// bead age-xf9r (option c), on a CONFIRMED pawl verdict bound to the
// gold-PRODUCING COMMIT — reusing the live, fail-closed verdict->commit
// authority (scripts/pawl-verdict.sh check). No schema change, no new node
// type, no 6th ledger. The trust anchor is "this commit was reviewed", which
// the pawl gate already proves; gold is deterministically compiled from the
// corpus at that commit. Real publish ALSO recomputes the digest + re-runs the
// leak scan at publish time (never trusts a prior dry-run) and writes the EXACT
// reviewed candidate to the gold dir, so published == scanned == digested.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/corpusscan"
	"github.com/boshu2/agentops/cli/internal/wiki"
)

var (
	wikiPublishDryRun  bool
	wikiPublishJSON    bool
	wikiPublishFloor   float64
	wikiPublishBead    string
	wikiPublishOut     string
	wikiPublishExpect  string
)

// checkPawlVerdict reports whether a CONFIRMED, commit-current pawl verdict
// exists for (bead, HEAD). It reuses the canonical gate scripts/pawl-verdict.sh
// (the same fail-closed verdict->commit authority that gates push-to-main), so
// publish trust == the pawl trust. Injectable for tests.
var checkPawlVerdict = func(repoRoot, bead, head string) error {
	script := filepath.Join(repoRoot, "scripts", "pawl-verdict.sh")
	c := exec.Command("bash", script, "check", bead, "0", "--head", head) // #nosec G204 -- repo-owned script, fixed args
	c.Dir = repoRoot
	return c.Run()
}

// resolveHeadSHA returns the current HEAD commit. Injectable for tests.
var resolveHeadSHA = func(repoRoot string) (string, error) {
	c := exec.Command("git", "rev-parse", "HEAD")
	c.Dir = repoRoot
	out, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("resolve HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

var wikiPublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish the gold wiki behind the membrane gate (leak-scan + commit verdict)",
	Long: `ao wiki publish is the membrane gate over the gold wiki.

--dry-run compiles a fresh publish candidate from .agents/, computes a STABLE
content digest, and runs the canonical leak scan (the same marker registry as ` + "`ao corpus scan`" + `).
It FAILS CLOSED on any leak — nothing is published.

Real publish (--bead <id>) is gated on a CONFIRMED pawl verdict bound to the
current commit (cross-family decision age-xf9r): it recomputes the digest,
re-runs the leak scan (fail-closed), requires ` + "`pawl-verdict.sh check`" + ` to pass for
(bead, HEAD), then writes the EXACT reviewed candidate to the gold dir
(--out, default .ao/wiki). --expect-digest <hex> additionally fails closed
unless the recomputed digest matches (publish exactly what dry-run reviewed).`,
	Args: cobra.NoArgs,
	RunE: runWikiPublish,
}

// buildCandidate compiles + leak-scans a publish candidate, failing closed on a
// leak. The caller must Cleanup the returned candidate.
func buildCandidate(agentsDir string, floor float64) (wiki.PublishCandidate, corpusscan.Report, error) {
	cand, err := wiki.CompilePublishCandidate(agentsDir, floor)
	if err != nil {
		return wiki.PublishCandidate{}, corpusscan.Report{}, err
	}
	rep, err := corpusscan.Scan(cand.OutDir)
	if err != nil {
		cand.Cleanup()
		return wiki.PublishCandidate{}, corpusscan.Report{}, fmt.Errorf("leak scan: %w", err)
	}
	return cand, rep, nil
}

func runWikiPublish(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	base, err := os.Getwd()
	if err != nil {
		return err
	}
	agentsDir := wiki.AgentsDirIn(base)
	if fi, err := os.Stat(agentsDir); err != nil || !fi.IsDir() {
		return fmt.Errorf("no .agents/ corpus at %s", agentsDir)
	}

	cand, rep, err := buildCandidate(agentsDir, wikiPublishFloor)
	if err != nil {
		return err
	}
	defer cand.Cleanup()

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
			"dry_run":     wikiPublishDryRun,
		})
	} else {
		fmt.Fprintf(out, "candidate digest: %s\n", cand.Digest)
		fmt.Fprintf(out, "promoted %d · gated %d · redactions %d\n",
			cand.Stats.Promoted, cand.Stats.Rejected, cand.Stats.Redactions)
		if rep.Clean() {
			fmt.Fprintf(out, "leak-scan: CLEAN\n")
		} else {
			fmt.Fprintf(out, "leak-scan: FAIL CLOSED — %d hit(s), NOT publishable\n", rep.HitCount())
			for _, f := range rep.Files {
				for _, h := range f.Hits {
					fmt.Fprintf(out, "  %s:%d  [%s/%s]\n", f.Path, h.Line, h.Class, h.Marker)
				}
			}
		}
	}

	// Fail closed: a leaked candidate is never publishable (both paths).
	if !rep.Clean() {
		return fmt.Errorf("publish candidate FAILED leak scan: %d hit(s) — not publishable", rep.HitCount())
	}

	if wikiPublishDryRun {
		return nil
	}

	// --- real publish (gated) ---
	if wikiPublishBead == "" {
		return fmt.Errorf("real publish requires --bead <id> (the bead whose CONFIRMED verdict authorizes this commit); or use --dry-run")
	}
	// Optional digest pinning: publish exactly what dry-run reviewed.
	if wikiPublishExpect != "" && wikiPublishExpect != cand.Digest {
		return fmt.Errorf("digest mismatch: --expect-digest %s != recomputed %s — corpus changed since dry-run; not publishing", wikiPublishExpect, cand.Digest)
	}
	head, err := resolveHeadSHA(base)
	if err != nil {
		return err
	}
	// The membrane gate: a CONFIRMED, commit-current pawl verdict for (bead, HEAD).
	if err := checkPawlVerdict(base, wikiPublishBead, head); err != nil {
		return fmt.Errorf("publish refused: no CONFIRMED pawl verdict for bead=%s at HEAD=%s (pawl-verdict.sh check failed) — fail-closed (age-xf9r)", wikiPublishBead, head[:min(7, len(head))])
	}

	// Publish the EXACT reviewed candidate (byte-identical to what was scanned +
	// digested + verdict-gated). Resolve --out relative to the repo base.
	outDir := wikiPublishOut
	if !filepath.IsAbs(outDir) {
		outDir = filepath.Join(base, outDir)
	}
	// Guard the destructive clear: never RemoveAll a root/cwd/repo-base path.
	if clean := filepath.Clean(outDir); clean == "/" || clean == "." || clean == filepath.Clean(base) {
		return fmt.Errorf("refusing to publish into unsafe --out %q (resolves to %q)", wikiPublishOut, clean)
	}
	if err := os.RemoveAll(outDir); err != nil {
		return fmt.Errorf("clear gold dir %s: %w", outDir, err)
	}
	if err := os.CopyFS(outDir, os.DirFS(cand.OutDir)); err != nil {
		return fmt.Errorf("publish to %s: %w", outDir, err)
	}
	fmt.Fprintf(out, "PUBLISHED digest %s -> %s (verdict-gated on bead=%s HEAD=%s)\n",
		cand.Digest[:min(12, len(cand.Digest))], outDir, wikiPublishBead, head[:min(7, len(head))])
	return nil
}

func init() {
	wikiPublishCmd.Flags().BoolVar(&wikiPublishDryRun, "dry-run", false, "compute + leak-scan the candidate without publishing")
	wikiPublishCmd.Flags().BoolVar(&wikiPublishJSON, "json", false, "emit the candidate report as JSON")
	wikiPublishCmd.Flags().Float64Var(&wikiPublishFloor, "confidence-floor", 0, "override the promotion confidence floor")
	wikiPublishCmd.Flags().StringVar(&wikiPublishBead, "bead", "", "bead whose CONFIRMED verdict for HEAD authorizes real publish (required without --dry-run)")
	wikiPublishCmd.Flags().StringVar(&wikiPublishOut, "out", ".ao/wiki", "gold wiki output directory for real publish")
	wikiPublishCmd.Flags().StringVar(&wikiPublishExpect, "expect-digest", "", "fail closed unless the recomputed digest matches this (publish exactly what dry-run reviewed)")
	wikiCmd.AddCommand(wikiPublishCmd)
}
