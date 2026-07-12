// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	doneapp "github.com/boshu2/agentops/cli/internal/done"
	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// hasVerdictEdgeForDisposition reports whether the ledger has a verdict->commit edge bound to sha
// whose disposition EXACTLY matches `disposition`. Reconcile must match the verdict FILE's own
// disposition — a CONFIRMED verdict is bound by a CONFIRMED edge, a REBOUND verdict by a REBOUND
// edge. hasConfirmedVerdictEdge is CONFIRMED-ONLY, so using it for a REBOUND verdict falsely reads
// UNBOUND (cross-family refute, Gemini, 2026-07-11). Same exact-token, sha-bound discipline as the
// push gate (parseDisposition + shaBindsCommit) — never a substring.
func hasVerdictEdgeForDisposition(ledgerPath, sha, disposition string) bool {
	if sha == "" || disposition == "" {
		return false
	}
	edges, err := provenancegraph.NewStore(ledgerPath).Read()
	if err != nil {
		return false
	}
	for _, e := range edges {
		if e.Relation != "wasDerivedFrom" || e.FromType != "verdict" || e.ToType != "commit" {
			continue
		}
		if !doneapp.SHABindsCommit(sha, e.ToID) {
			continue
		}
		if parseDisposition(e.EvidenceRef) == disposition {
			return true
		}
	}
	return false
}

var (
	provReconcileDir   string
	provReconcileEmit  bool
	provReconcileForce bool
	provReconcileJSON  bool
)

// reconcileExitError carries the reconcile process exit code (0 clean / 1 unbound
// or emit-failed / 2 usage-precondition), mapped in root.go's Execute switch.
type reconcileExitError struct{ code int }

func (e *reconcileExitError) Error() string { return "" }
func (e *reconcileExitError) ExitCode() int { return e.code }

// ledgerWorktreeDirty reports whether the committed ledger path has uncommitted
// working-tree changes (a real desync-repair must not sweep another lane's rows).
func ledgerWorktreeDirty(root, ledgerPath string) (bool, error) {
	rel, err := filepath.Rel(root, ledgerPath)
	if err != nil {
		rel = ledgerPath
	}
	out, err := gitOutput(root, "status", "--porcelain", "--", rel)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// reconcileVerdict is the minimal shape read from a pawl-verdict JSON artifact —
// only the fields reconcile needs to match a verdict to its ledger edge.
type reconcileVerdict struct {
	BeadID      string `json:"bead_id"`
	HeadSHA     string `json:"head_sha"`
	Disposition string `json:"disposition"`
}

// reconcileRow is one verdict-vs-edge finding.
type reconcileRow struct {
	File        string `json:"file"`
	BeadID      string `json:"bead_id"`
	HeadSHA     string `json:"head_sha"`
	Disposition string `json:"disposition"`
	EdgeBound   bool   `json:"edge_bound"`
	Emitted     bool   `json:"emitted,omitempty"`
}

var provenanceReconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Detect (and optionally repair) verdict-file / ledger-edge desyncs",
	Long: `Scan the pawl-verdicts dir (.agents/pawl-verdicts/*.json) and, for every
CONFIRMED or REBOUND verdict, report whether a matching verdict->commit edge is
bound in the committed ledger (docs/provenance/ledger.jsonl). "No verdict = not
done" is enforced on the EDGE (ao done / the push gate read the ledger, not the
verdict file), so a verdict FILE with no EDGE is a silent desync — ao done will
refuse the close. This surfaces those in one pass instead of one-refused-close
at a time.

Matching reuses the EXACT recognizer the push gate and ao done use
(hasConfirmedVerdictEdge): relation=wasDerivedFrom, from_type=verdict,
to_type=commit, sha-bound, exact-token disposition — never a substring.

--emit re-emits the missing edges via the same 'ao provenance emit-verdict'
path; it is idempotent (an already-bound verdict is never re-emitted — only the
UNBOUND ones detected this pass are acted on). --force allows running with an
uncommitted ledger (default: refuse, to avoid sweeping another lane's rows).

Exit status:
  0   every scanned verdict's edge is bound (clean), or --emit bound them all
  1   one or more verdicts are UNBOUND (report mode), or an emit failed
  2   usage / precondition (dirty ledger without --force, unreadable dir)

Examples:
  ao provenance reconcile
  ao provenance reconcile --json
  ao provenance reconcile --emit`,
	Args: cobra.NoArgs,
	RunE: runProvenanceReconcile,
}

func init() {
	provenanceCmd.AddCommand(provenanceReconcileCmd)
	// The reconcile outcome IS the exit code (0/1/2); the command prints its own
	// report/reason, so suppress cobra's empty "Error:" line on the typed exit error.
	provenanceReconcileCmd.SilenceErrors = true
	provenanceReconcileCmd.Flags().StringVar(&provReconcileDir, "dir", "", "Verdicts dir (default: <repo>/.agents/pawl-verdicts)")
	provenanceReconcileCmd.Flags().BoolVar(&provReconcileEmit, "emit", false, "Re-emit missing ledger edges for unbound verdicts")
	provenanceReconcileCmd.Flags().BoolVar(&provReconcileForce, "force", false, "Run even with an uncommitted ledger worktree")
	provenanceReconcileCmd.Flags().BoolVar(&provReconcileJSON, "json", false, "Emit the reconcile result as JSON")
}

// reconcileVerdictsDir resolves the verdicts dir: --dir override, else
// <repo>/.agents/pawl-verdicts (matching pawl-verdict.sh's default).
func reconcileVerdictsDir(root string) string {
	if provReconcileDir != "" {
		return provReconcileDir
	}
	return filepath.Join(root, ".agents", "pawl-verdicts")
}

// scanReconcileVerdicts reads every *.json in dir into reconcileRows, computing
// edge-bound status against ledgerPath. It is a pure function of the filesystem
// (no mutation) — the report is derived, --emit acts on it separately.
func scanReconcileVerdicts(dir, ledgerPath string) ([]reconcileRow, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no verdicts dir yet => nothing to reconcile (clean)
		}
		return nil, err
	}
	var rows []reconcileRow
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil, fmt.Errorf("reading %s: %w", p, rerr)
		}
		var v reconcileVerdict
		if jerr := json.Unmarshal(b, &v); jerr != nil {
			// A malformed verdict file is itself a desync signal — surface it as
			// unbound with an empty disposition rather than aborting the scan.
			rows = append(rows, reconcileRow{File: e.Name(), EdgeBound: false})
			continue
		}
		// Only CONFIRMED / REBOUND verdicts authorize; a REFUTED/HOLD verdict is
		// not expected to carry an authorizing edge, so it is not a desync.
		if v.Disposition != "CONFIRMED" && v.Disposition != "REBOUND" {
			continue
		}
		rows = append(rows, reconcileRow{
			File:        e.Name(),
			BeadID:      v.BeadID,
			HeadSHA:     v.HeadSHA,
			Disposition: v.Disposition,
			// Match the verdict's OWN disposition (CONFIRMED->CONFIRMED, REBOUND->REBOUND) —
			// a CONFIRMED-only check would falsely flag every REBOUND verdict as UNBOUND.
			EdgeBound: hasVerdictEdgeForDisposition(ledgerPath, v.HeadSHA, v.Disposition),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].File < rows[j].File })
	return rows, nil
}

func runProvenanceReconcile(cmd *cobra.Command, _ []string) error {
	root, err := repoRootOrCwd()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "reconcile: resolving repo root: %v\n", err)
		return &reconcileExitError{2}
	}
	ledgerPath := resolveLedgerPath()

	// Dirty-ledger guard (age-7krl): --emit sweeps the ledger; refuse to run against an
	// uncommitted ledger unless --force, so another lane's rows are never disturbed.
	if provReconcileEmit && !provReconcileForce {
		if dirty, derr := ledgerWorktreeDirty(root, ledgerPath); derr == nil && dirty {
			fmt.Fprintln(cmd.ErrOrStderr(), "reconcile: the provenance ledger has uncommitted changes — "+
				"refusing --emit (another lane's rows could be swept). Commit/stash the ledger, or pass --force.")
			return &reconcileExitError{2}
		}
	}

	rows, err := scanReconcileVerdicts(reconcileVerdictsDir(root), ledgerPath)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "reconcile: scanning verdicts: %v\n", err)
		return &reconcileExitError{2}
	}

	emitFailed := false
	if provReconcileEmit {
		for i := range rows {
			if rows[i].EdgeBound {
				continue // idempotent: never re-emit an already-bound edge
			}
			if rows[i].HeadSHA == "" {
				continue // malformed/unbindable — nothing to emit
			}
			provEmitVerdictFile = filepath.Join(reconcileVerdictsDir(root), rows[i].File)
			if eerr := runProvenanceEmitVerdict(cmd, nil); eerr != nil {
				emitFailed = true
				fmt.Fprintf(cmd.ErrOrStderr(), "reconcile: emit failed for %s: %v\n", rows[i].File, eerr)
				continue
			}
			rows[i].EdgeBound = true
			rows[i].Emitted = true
		}
	}

	unbound := 0
	for _, r := range rows {
		if !r.EdgeBound {
			unbound++
		}
	}

	if provReconcileJSON {
		out, _ := json.MarshalIndent(map[string]any{
			"scanned": len(rows), "unbound": unbound, "emitted": provReconcileEmit, "rows": rows,
		}, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
	} else {
		for _, r := range rows {
			mark := "bound"
			if !r.EdgeBound {
				mark = "UNBOUND"
			} else if r.Emitted {
				mark = "emitted"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-40s %-9s %s\n", r.Disposition, shortSHAOrDash(r.HeadSHA), mark, r.BeadID)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "reconcile: %d verdict(s) scanned, %d unbound\n", len(rows), unbound)
	}

	if emitFailed || unbound > 0 {
		return &reconcileExitError{1}
	}
	return nil
}

func shortSHAOrDash(sha string) string {
	if len(sha) >= 12 {
		return sha[:12]
	}
	if sha == "" {
		return "-"
	}
	return sha
}
