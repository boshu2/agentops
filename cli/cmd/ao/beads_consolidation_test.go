// Tests for the beads-* skill-cluster consolidation (ag-ez7y6): the four
// beads skills (beads, beads-br, beads-bv, beads-workflow) collapse to the
// surviving set {beads-br, beads-bv, beads-workflow} with the legacy `beads`
// umbrella retired --into beads-br. These are L2 structural-invariant tests:
// they read the real repo files (skills tree + disposition ledger) so a
// regression that re-adds the retired skill, drops the folded doctrine, or
// leaves the ledger dirty fails the build.

// practices: [pragmatic-programmer]
package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// beadsConsolidationSurvivors is the post-consolidation surviving set.
var beadsConsolidationSurvivors = []string{"beads-br", "beads-bv", "beads-workflow"}

// findBeadsRepoRoot resolves the repo root by a stable structural marker that
// only co-exists at the root: skills/ AND docs/contracts/. The shared
// findRepoRoot helper keys on .agents/, which the broader cmd/ao suite leaks
// into the package dir (cli/cmd/ao/.agents) — a marker present at the WRONG
// level makes that helper resolve the package dir as "root" and is order-
// dependent. Keying on skills/+docs/contracts/ is leak-proof.
func findBeadsRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for range 8 {
		_, skillsErr := os.Stat(filepath.Join(dir, "skills"))
		_, contractsErr := os.Stat(filepath.Join(dir, "docs", "contracts"))
		if skillsErr == nil && contractsErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate repo root (skills/ + docs/contracts/) from %s", filepath.Dir(thisFile))
	return ""
}

// TestBeadsConsolidation_RetiredUmbrellaGone asserts the legacy `beads`
// umbrella skill dir is removed and the three survivors remain on disk. No
// capability lives only in a removed dir.
func TestBeadsConsolidation_RetiredUmbrellaGone(t *testing.T) {
	root := findBeadsRepoRoot(t)
	beadsDir := filepath.Join(root, "skills", "beads")
	if _, err := os.Stat(beadsDir); err == nil {
		t.Fatalf("legacy umbrella skills/beads still exists; expected it retired --into beads-br")
	}
	for _, slug := range beadsConsolidationSurvivors {
		dir := filepath.Join(root, "skills", slug)
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
			t.Fatalf("survivor skills/%s/SKILL.md missing after consolidation: %v", slug, err)
		}
	}
}

// TestBeadsConsolidation_DoctrineFoldedIntoBR asserts the unique operating
// doctrine that lived ONLY in the `beads` umbrella survives in beads-br — the
// "no capability lost vs the 4-skill state" scenario. The umbrella's distinct
// value was issue-lifecycle discipline (scoped closure proof, parent
// reconciliation, narrow-the-umbrella-issue, normalize-stale-queue,
// live-reads-authoritative) — not the command surface beads-br already had.
func TestBeadsConsolidation_DoctrineFoldedIntoBR(t *testing.T) {
	root := findBeadsRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "skills", "beads-br", "SKILL.md"))
	if err != nil {
		t.Fatalf("read beads-br SKILL.md: %v", err)
	}
	body := strings.ToLower(string(data))
	// Each token marks a folded operating-doctrine capability. Tokens are
	// lower-cased and chosen to be robust to phrasing.
	required := map[string]string{
		"scoped closure proof":        "scoped closure proof",
		"parent reconciliation":       "reconcile",
		"narrow the umbrella issue":   "narrow",
		"normalize stale queue items": "stale",
		"live reads authoritative":    "authoritative",
	}
	for capability, token := range required {
		if !strings.Contains(body, token) {
			t.Errorf("beads-br SKILL.md lost folded capability %q (token %q not found)", capability, token)
		}
	}
}

// TestBeadsConsolidation_LedgerCleanMergedInto asserts the disposition ledger
// records `beads` in the historical: section as merged-into beads-br, and that
// no active dispositions row for `beads` remains — the "ledger + ripple clean"
// scenario.
func TestBeadsConsolidation_LedgerCleanMergedInto(t *testing.T) {
	root := findBeadsRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "docs", "contracts", "skill-dispositions.yaml"))
	if err != nil {
		t.Fatalf("read skill-dispositions.yaml: %v", err)
	}
	text := string(data)
	histIdx := strings.Index(text, "\nhistorical:")
	dispIdx := strings.Index(text, "\ndispositions:")
	if histIdx == -1 || dispIdx == -1 || histIdx > dispIdx {
		t.Fatalf("ledger missing historical:/dispositions: sections in expected order (hist=%d disp=%d)", histIdx, dispIdx)
	}
	historicalSection := text[histIdx:dispIdx]
	dispositionsSection := text[dispIdx:]

	// No active dispositions row for the retired umbrella.
	if strings.Contains(dispositionsSection, "- skill:            beads\n") ||
		strings.Contains(dispositionsSection, "- skill: beads\n") {
		t.Errorf("active dispositions row for retired skill `beads` still present")
	}
	// Historical row present, merged-into beads-br.
	if !strings.Contains(historicalSection, "\n  beads:\n") {
		t.Errorf("historical: section missing a `beads:` terminal-state row")
	}
	if !strings.Contains(historicalSection, "merged-into: beads-br") &&
		!strings.Contains(historicalSection, "merged-into:  beads-br") {
		t.Errorf("historical `beads` row not recorded as merged-into beads-br")
	}
}
