// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// Close-reason stamp dispositions. The stamp format is
// "[verdict:<sha7>:<disposition>]" — appended to the br close reason so the
// verdict-close-rate gate (scripts/check-verdict-close-rate.sh) can measure
// how many closes reference ledger proof.
const (
	doneStampConfirmed  = "CONFIRMED"
	doneStampWaived     = "waived-trivial"
	doneStampUnverified = "UNVERIFIED"
)

var (
	doneSHA           string
	doneReason        string
	doneForceNoVerdct bool
	doneJSON          bool
)

var doneCmd = &cobra.Command{
	Use:   "done <bead-id>",
	Short: "Close a bead with a verdict-referenced stamp — no verdict = not done",
	Long: `Close a bead through the membrane's bookkeeping half: the close reason is
stamped with the ledger proof that the landed commit was actually reviewed.

Resolution order for the target commit: --sha wins; otherwise the HEAD of the
git repository at the current working directory. The commit is looked up in
the committed provenance ledger (docs/provenance/ledger.jsonl):

  CONFIRMED verdict bound to the commit
      -> close via br with "[verdict:<sha7>:CONFIRMED]" appended to the reason.
  no verdict, and every changed file of the commit is under docs/provenance/
      -> the #trivial waiver class; close with "[verdict:<sha7>:waived-trivial]".
  no verdict otherwise
      -> REFUSE (non-zero exit) and name the command that produces one
         (ao verify / ao pawl review). --force-no-verdict is the escape hatch:
         it closes with an explicit, greppable "[verdict:<sha7>:UNVERIFIED]"
         stamp instead of blocking forever.

The close itself shells out to the br CLI on PATH with the environment as-is:
run 'ao done' where 'ao beads dir' resolves the live ledger (BEADS_DIR is
respected when already exported; in linked worktrees the resolution walks
git's common directory back to the canonical _beads — no path is hardcoded).

Posture: warn-first. Nothing intercepts a raw 'br close'; the lever is this
recommended close path plus the warn-only verdict-close-rate gate.

Examples:
  ao done ag-x31t.4                       # close against HEAD's verdict
  ao done ag-x31t.4 --sha 4f2a91c         # close against an explicit commit
  ao done ag-x31t.4 --force-no-verdict    # honest UNVERIFIED escape hatch`,
	Args: cobra.ExactArgs(1),
	RunE: runDone,
}

func init() {
	rootCmd.AddCommand(doneCmd)
	doneCmd.Flags().StringVar(&doneSHA, "sha", "", "Commit sha (or >=7-char prefix) the bead landed as (default: HEAD at cwd)")
	doneCmd.Flags().StringVarP(&doneReason, "reason", "r", "Done", "Close reason prose (the verdict stamp is appended)")
	doneCmd.Flags().BoolVar(&doneForceNoVerdct, "force-no-verdict", false, "Close without a verdict, stamping an explicit UNVERIFIED marker")
	doneCmd.Flags().BoolVar(&doneJSON, "json", false, "Emit machine-readable JSON (stdout-as-data)")
}

// doneReport is the structured output of ao done --json.
type doneReport struct {
	BeadID      string `json:"bead_id"`
	CommitSHA   string `json:"commit_sha"`
	Disposition string `json:"disposition"`
	Stamp       string `json:"stamp"`
	CloseReason string `json:"close_reason"`
	Closed      bool   `json:"closed"`
}

// doneVerdictLookup summarizes the verdict edges bound to one commit.
type doneVerdictLookup struct {
	// VerdictID is the node id of the deciding verdict: the latest CONFIRMED
	// one when Confirmed, else the latest verdict seen.
	VerdictID string
	// Confirmed reports whether any bound verdict carries disposition=CONFIRMED.
	Confirmed bool
	// Dispositions lists every bound verdict's disposition in ledger order
	// (empty string for a verdict record without one).
	Dispositions []string
	// ForeignBeads lists verdict node ids bound to this commit but belonging to
	// OTHER beads — never certifying, surfaced in refusal messages.
	ForeignBeads []string
}

// shaBindsCommit reports whether query (>=7 hex chars) and commitID name the
// same commit: either is a case-insensitive prefix of the other, matching the
// provenance_show prefix-resolution convention.
func shaBindsCommit(query, commitID string) bool {
	q, c := strings.ToLower(query), strings.ToLower(commitID)
	if len(q) < minShaPrefixLen || len(c) < minShaPrefixLen {
		return false
	}
	if !isHexToken(q) || !isHexToken(c) {
		return false
	}
	return strings.HasPrefix(q, c) || strings.HasPrefix(c, q)
}

// lookupDoneVerdicts scans the ledger for verdict --wasDerivedFrom--> commit
// edges bound to sha (the shape ao provenance emit-verdict writes) AND to the
// bead being closed: verdict node ids are `<bead>@<sha7>`, and a CONFIRMED
// verdict for a DIFFERENT bead on the same commit must never certify this one
// (wrong-object certification — pawl catch on this bead's own landing). A
// foreign-bead verdict is counted in ForeignBeads so the refusal can name it.
// Pure.
func lookupDoneVerdicts(edges []provenancegraph.Edge, beadID, sha string) doneVerdictLookup {
	var l doneVerdictLookup
	for _, e := range edges {
		if e.Relation != "wasDerivedFrom" || e.FromType != "verdict" || e.ToType != "commit" {
			continue
		}
		if !shaBindsCommit(sha, e.ToID) {
			continue
		}
		if vb, _, ok := strings.Cut(e.FromID, "@"); !ok || vb != beadID {
			l.ForeignBeads = append(l.ForeignBeads, e.FromID)
			continue
		}
		disp := parseDisposition(e.EvidenceRef)
		l.Dispositions = append(l.Dispositions, disp)
		if disp == doneStampConfirmed {
			l.Confirmed = true
			l.VerdictID = e.FromID
		} else if !l.Confirmed {
			l.VerdictID = e.FromID
		}
	}
	return l
}

// doneStamp formats the close-reason stamp: "[verdict:<sha7>:<disposition>]".
func doneStamp(sha, disposition string) string {
	return "[verdict:" + shortHash7(sha) + ":" + disposition + "]"
}

// doneResolveHead returns the full HEAD sha of the git repository at cwd.
func doneResolveHead(cwd string) (string, error) {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("resolve HEAD at %s (pass --sha to name the landed commit explicitly): %w", cwd, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// doneCommitProvenanceOnly reports whether every changed file of the commit is
// under docs/provenance/ — the sole waiver class, mirroring the #trivial
// semantics of scripts/check-pawl-pre-push.sh. Fail-closed: a failed diff-tree
// or an empty changed-file list cannot prove triviality, so it does NOT waive.
// --no-renames forces a rename INTO docs/provenance/ to expose its
// non-allowlisted source path.
func doneCommitProvenanceOnly(cwd, sha string) bool {
	out, err := exec.Command("git", "-C", cwd,
		"diff-tree", "--no-commit-id", "--no-renames", "--name-only", "-r", sha).Output()
	if err != nil {
		return false
	}
	var files []string
	// Strip only the line terminator, never spaces: a path literally named
	// " docs/provenance/x" (leading space) must NOT trim into the allowlist
	// (pawl catch: fail-open waiver via TrimSpace).
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.TrimRight(line, "\r"); f != "" {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !strings.HasPrefix(f, "docs/provenance/") {
			return false
		}
	}
	return true
}

// doneRefusalError builds the corrective refusal: no proof, no close.
func doneRefusalError(beadID, sha string, l doneVerdictLookup) error {
	var found string
	if len(l.Dispositions) > 0 {
		found = fmt.Sprintf("verdict(s) recorded for commit %s but none CONFIRMED (found: %s)",
			shortHash7(sha), strings.Join(l.Dispositions, ", "))
	} else if len(l.ForeignBeads) > 0 {
		found = fmt.Sprintf("no verdict for %s on commit %s — the verdict(s) there belong to OTHER bead(s): %s (a verdict certifies its own bead only)",
			beadID, shortHash7(sha), strings.Join(l.ForeignBeads, ", "))
	} else {
		found = fmt.Sprintf("no verdict recorded for commit %s", shortHash7(sha))
	}
	return fmt.Errorf(`%s — no verdict = not done; refusing to close %s
  produce one:  ao verify %s            (front door — writes the commit-bound verdict on CONFIRMED)
  advanced:     ao pawl review %s --scope head
  waiver:       only a commit whose changed files are all under docs/provenance/ closes as waived-trivial
  escape hatch: ao done %s --force-no-verdict   (closes with an explicit UNVERIFIED stamp)`,
		found, beadID, beadID, beadID, beadID)
}

func runDone(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	beadID := args[0]
	out := cmd.OutOrStdout()

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve cwd: %w", err)
	}

	sha := strings.TrimSpace(doneSHA)
	if sha == "" {
		sha, err = doneResolveHead(cwd)
		if err != nil {
			return err
		}
	}
	if len(sha) < minShaPrefixLen || !isHexToken(sha) {
		return fmt.Errorf("--sha %q is not a commit sha (need at least %d hex chars)", sha, minShaPrefixLen)
	}

	store := provenancegraph.NewStore(resolveLedgerPath())
	edges, err := store.Read()
	if err != nil {
		return fmt.Errorf("read provenance ledger: %w", err)
	}

	lookup := lookupDoneVerdicts(edges, beadID, sha)
	var disposition string
	switch {
	case lookup.Confirmed:
		disposition = doneStampConfirmed
	case len(lookup.Dispositions) == 0 && doneCommitProvenanceOnly(cwd, sha):
		disposition = doneStampWaived
	case doneForceNoVerdct:
		disposition = doneStampUnverified
	default:
		return doneRefusalError(beadID, sha, lookup)
	}

	stamp := doneStamp(sha, disposition)
	reason := strings.TrimSpace(doneReason)
	if reason == "" {
		reason = "Done"
	}
	closeReason := reason + " " + stamp

	// Shell out to the br CLI on PATH with the environment as-is (BEADS_DIR is
	// respected when exported; otherwise resolved the same way `ao beads dir`
	// resolves it — via git's common directory, never a hardcoded path).
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	brCmd := beadsTrackerCommandContext(ctx, "close", beadID, "-r", closeReason)
	brOut, err := brCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("br close %s: %w\n%s", beadID, err, strings.TrimSpace(string(brOut)))
	}

	if doneJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(doneReport{
			BeadID:      beadID,
			CommitSHA:   sha,
			Disposition: disposition,
			Stamp:       stamp,
			CloseReason: closeReason,
			Closed:      true,
		})
	}
	if msg := strings.TrimSpace(string(brOut)); msg != "" {
		fmt.Fprintln(out, msg)
	}
	fmt.Fprintf(out, "closed %s at %s %s\n", beadID, shortHash7(sha), stamp)
	if disposition == doneStampUnverified {
		fmt.Fprintf(out, "note: UNVERIFIED close — run 'ao verify %s' next time so done carries proof\n", beadID)
	}
	return nil
}
