// practices: [pragmatic-programmer, agile-manifesto]
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// firstVerdictCommand is THE exact command quick-start leaves the user one
// step away from. It must stay in lockstep with the README quickstart
// paragraph and scripts/probe-first-verdict.sh (the probe greps the README
// for this literal so a doc edit cannot silently break the golden path).
const firstVerdictCommand = "ao verify my-first-change"

// firstVerdictInfo is the machine-readable summary of the quick-start's final
// step: is the verdict ledger path ready, which reviewer families answered
// live, and either the exact next command (reviewer reachable) or the exact
// install one-liners (no reviewer reachable). Serialized under the
// quickstartResult "first_verdict" key so --json keeps teaching the same
// next action as the text output (Directive 13).
type firstVerdictInfo struct {
	LedgerPath      string   `json:"ledger_path"`
	LedgerReady     bool     `json:"ledger_ready"`
	ReviewerLive    []string `json:"reviewer_live"`
	NextCommand     string   `json:"next_command,omitempty"`
	ReviewerInstall []string `json:"reviewer_install,omitempty"`
}

// prepareFirstVerdict readies the first-verdict step without ever running a
// reviewer (a review costs the user's reviewer tokens; quick-start's job is
// to leave them one command away with everything verified ready):
//   - ensure the provenance ledger's parent directory exists at the SAME path
//     `ao verify`/emit-verdict resolves (resolveLedgerPath — never a second
//     path derivation);
//   - probe reviewer reachability through the SAME shared check `ao doctor`
//     runs (reviewerReachabilityChecks — never a duplicate probe).
func prepareFirstVerdict() *firstVerdictInfo {
	ledgerPath := resolveLedgerPath()
	info := &firstVerdictInfo{LedgerPath: ledgerPath}
	if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o755); err == nil {
		info.LedgerReady = true
	}
	_, live := reviewerHealthService.Check(context.Background(), reviewerProbeTimeout)
	info.ReviewerLive = live
	if len(live) > 0 {
		info.NextCommand = firstVerdictCommand
		return info
	}
	// No reviewer reachable: teach the exact doctor corrective install
	// one-liners (wedgeReviewers.installCmd is the shared source of truth),
	// never a pointer into diffuse docs.
	for _, reviewer := range reviewerHealthService.Catalog() {
		info.ReviewerInstall = append(info.ReviewerInstall, reviewer.Name+": "+reviewer.InstallCommand)
	}
	return info
}

// printFirstVerdictStep ends the quick-start on the first-value path: a real
// independent verdict on the user's own diff, one command away.
func printFirstVerdictStep(info *firstVerdictInfo) {
	fmt.Println("\n━━━ FINAL STEP: your first verdict ━━━")
	if info.LedgerReady {
		fmt.Printf("  ✓ Verdict ledger path ready: %s\n", info.LedgerPath)
	} else {
		fmt.Printf("  ⚠ Could not create the ledger directory for %s (check permissions)\n", info.LedgerPath)
	}
	if info.NextCommand != "" {
		fmt.Printf("  ✓ Reviewer reachable: %s\n", strings.Join(info.ReviewerLive, ", "))
		fmt.Println("\n  Commit a small change, then run:")
		fmt.Printf("\n      %s\n\n", info.NextCommand)
		fmt.Println("  A model that did not write the change reviews your latest commit and")
		fmt.Println("  returns CONFIRMED or REFUTED; the verdict is recorded in your repo's")
		fmt.Println("  ledger. It costs reviewer tokens, so nothing runs until you run it.")
		return
	}
	fmt.Println("  ⚠ No reviewer CLI reachable — 'ao verify' needs one. Install:")
	for _, line := range info.ReviewerInstall {
		fmt.Printf("      %s\n", line)
	}
	fmt.Println("  Then re-run 'ao quick-start' (or 'ao doctor' to re-check).")
}
