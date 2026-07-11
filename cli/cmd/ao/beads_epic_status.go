// `ao beads epic-status <id> --terminal` — a deterministic group-terminality
// predicate (age-gascity-port-slate-irye.4). Replaces agent self-report ("the
// wave is done") with a machine-readable verdict at epic/wave granularity: the
// membrane's "no verdict = not done" applied to a group.
//
// The three guards live in the pure, table-tested cli/internal/epicstatus
// package. This file resolves the live br ledger (issues.jsonl, the git-backed
// source of truth), builds the epic's member set — id-prefix children (age-X.N)
// UNION beads with a parent-child dependency edge to the epic — and hands it to
// the predicate. A dangling family reference (a member id with no ledger
// record) becomes an unknown-status placeholder (guard 1).
//
// Consumers: /crank wave close, /validate completion audits, drive-loop exhaust
// checks. With --terminal the verdict maps to the process exit code so shell
// callers can branch:
//
//	0 — terminal (done)      2 — not terminal      3 — skipped (materializing)
//	1 — error (epic not found / ledger unreadable)
//
// practices: [tdd, dora-metrics]

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
	"github.com/boshu2/agentops/cli/internal/epicstatus"
	"github.com/spf13/cobra"
)

var (
	beadsEpicStatusTerminal bool
	beadsEpicStatusJSON     bool
)

var beadsEpicStatusCmd = &cobra.Command{
	Use:   "epic-status <epic-id>",
	Short: "Deterministic group-terminality verdict for an epic/wave",
	Long: `Emit a deterministic "is this epic/wave actually done" verdict, replacing
agent self-report. Members are the epic's id-prefix children (<epic>.N) UNION
any bead with a parent-child dependency edge to the epic.

Three guards (all must hold for a terminal/done verdict):
  1. an unresolved/missing member becomes an unknown-status placeholder that
     NEVER counts as done;
  2. a group with a deliberately-open descendant (a human-gate / checkpoint
     bead) is NOT complete;
  3. a zero-descendant, still-materializing group is skipped, not done.

Output: a human-readable reason by default, or a machine-readable verdict with
--json. With --terminal the verdict also maps to the exit code:

  0 — terminal (done)   2 — not terminal   3 — skipped (materializing)
  1 — error (epic not found, ledger unreadable)

Reads the live br ledger resolved via 'ao beads dir' (works from linked
worktrees). Read-only.`,
	Args: cobra.ExactArgs(1),
	RunE: runBeadsEpicStatus,
}

func init() {
	beadsEpicStatusCmd.Flags().BoolVar(&beadsEpicStatusTerminal, "terminal", false,
		"Map the verdict to the process exit code (0 terminal / 2 not-terminal / 3 skipped).")
	beadsEpicStatusCmd.Flags().BoolVar(&beadsEpicStatusJSON, "json", false,
		"Emit the verdict as a JSON object instead of a human-readable line.")
}

// ledgerBead is the subset of an issues.jsonl record the predicate needs.
type ledgerBead = beadsapp.LedgerBead

// ledgerDep is one dependency edge on a bead. A parent-child edge points from a
// child UP to its parent/epic (depends_on_id == epic).
type ledgerDep = beadsapp.LedgerDep

// beadsEpicStatusReadLedger is the seam for tests: it returns the raw
// newline-delimited issues.jsonl bytes for the resolved ledger dir. Tests
// override it to inject an in-memory ledger without touching a br/ao binary.
var beadsEpicStatusReadLedger = func(dir string) ([]byte, error) {
	return currentBeadsRuntime().ReadFile(filepath.Join(dir, "issues.jsonl"))
}

func runBeadsEpicStatus(cmd *cobra.Command, args []string) error {
	epic := strings.TrimSpace(args[0])
	if epic == "" {
		return fmt.Errorf("epic id is required")
	}

	ledger, err := currentBeadsTracker().BRLedger()
	if err != nil {
		return err
	}
	dir := ledger.Path
	raw, err := beadsEpicStatusReadLedger(dir)
	if err != nil {
		return fmt.Errorf("read ledger %s: %w", filepath.Join(dir, "issues.jsonl"), err)
	}
	beads, err := parseLedger(raw)
	if err != nil {
		return fmt.Errorf("parse ledger: %w", err)
	}

	members, epicPresent := buildMembers(epic, beads)
	if !epicPresent {
		return fmt.Errorf("epic %s not found in ledger %s", epic, dir)
	}

	result := epicstatus.Evaluate(epic, members)
	emitEpicStatus(cmd, result)

	if beadsEpicStatusTerminal {
		switch result.Verdict {
		case epicstatus.NotTerminal:
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			return &beadsExitError{code: 2}
		case epicstatus.Skipped:
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			return &beadsExitError{code: 3}
		}
	}
	return nil
}

// parseLedger parses newline-delimited issues.jsonl bytes into bead records.
// Blank lines are skipped; a malformed line is a hard error (fail closed —
// "unreachable is not absent").
func parseLedger(raw []byte) ([]ledgerBead, error) {
	return beadsapp.ParseLedger(raw)
}

// buildMembers computes an epic's member set from a ledger snapshot and reports
// whether the epic record itself is present.
//
// A member is any bead whose id is prefixed "<epic>." (id-prefix child) OR that
// carries a parent-child dependency edge to the epic. A family id ("<epic>.*")
// referenced by a dependency edge but absent from the ledger is a dangling
// reference → an unresolved (Present=false) member, so guard 1 fires.
func buildMembers(epic string, beads []ledgerBead) (members []epicstatus.Member, epicPresent bool) {
	return beadsapp.BuildMembers(epic, beads)
}

func emitEpicStatus(cmd *cobra.Command, r epicstatus.Result) {
	out := cmd.OutOrStdout()
	if beadsEpicStatusJSON {
		enc := json.NewEncoder(out)
		enc.SetEscapeHTML(false)
		// Marshalling a Result never fails; ignore the error deliberately.
		_ = enc.Encode(r)
		return
	}
	fmt.Fprintf(out, "%s: %s [%s]\n", r.Group, strings.ToUpper(string(r.Verdict)), r.Code)
	fmt.Fprintf(out, "  %s\n", r.Reason)
	for _, b := range r.Blockers {
		fmt.Fprintf(out, "  - blocker %s (%s): %s\n", b.ID, b.Status, b.Class)
	}
}
