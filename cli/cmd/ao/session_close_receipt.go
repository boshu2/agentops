// practices: [knowledge-flywheel-surface, compounding-receipt]
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

// corpusReuseReceipt is the READ-side payoff of the knowledge flywheel: how much
// prior corpus a session actually reused, paired with the WRITE-side counts the
// session-close table already reports. It makes compounding FELT — the moat is
// otherwise invisible at the one moment it matters (cf. bead ag-469h).
type corpusReuseReceipt struct {
	// Reused is the number of distinct prior corpus artifacts this session cited
	// (retrieved or applied) — the prior decisions this run stood on.
	Reused int
	// Titles is a short, human-readable sample of the reused artifacts (≤3).
	Titles []string
	// CorpusEntries is the current total size of the durable corpus
	// (.agents/learnings + .agents/patterns).
	CorpusEntries int
}

// reuseCorpusDirs are the durable-corpus sections counted as "entries" in the receipt.
var reuseCorpusDirs = []string{"learnings", "patterns"}

// datePrefixRe matches a leading ISO date prefix in artifact filenames
// (e.g. "2026-06-06-foo" → "foo") so reused titles read cleanly.
var datePrefixRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[-_]?`)

// computeReuseReceipt builds the per-session corpus-reuse receipt from the
// citation ledger (.agents/ao/citations.jsonl) and the on-disk corpus. It is
// best-effort and never fails the close: a missing ledger yields a zero receipt.
func computeReuseReceipt(cwd, sessionID string) corpusReuseReceipt {
	receipt := corpusReuseReceipt{CorpusEntries: countCorpusEntries(cwd)}
	if sessionID == "" {
		return receipt
	}

	citations, err := ratchet.LoadCitations(cwd)
	if err != nil {
		return receipt
	}

	seen := make(map[string]bool)
	var titles []string
	for _, c := range citations {
		if c.SessionID != sessionID {
			continue
		}
		// "reference" is a manual pointer, not a flywheel read-payoff; count
		// retrievals and applications — the events where prior corpus shaped the run.
		if c.CitationType == "reference" {
			continue
		}
		key := ratchet.CanonicalArtifactPath(cwd, c.ArtifactPath)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if len(titles) < 3 {
			titles = append(titles, reuseTitleFromPath(c.ArtifactPath))
		}
	}

	receipt.Reused = len(seen)
	receipt.Titles = titles
	return receipt
}

// countCorpusEntries counts durable markdown artifacts under .agents/learnings
// and .agents/patterns — the corpus whose growth the receipt reports.
func countCorpusEntries(cwd string) int {
	total := 0
	for _, sub := range reuseCorpusDirs {
		entries, err := os.ReadDir(filepath.Join(cwd, ".agents", sub))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(e.Name(), ".md") {
				total++
			}
		}
	}
	return total
}

// reuseTitleFromPath derives a compact, human-readable title from an artifact
// path: basename, drop the .md extension and any leading ISO date prefix.
func reuseTitleFromPath(p string) string {
	base := filepath.Base(p)
	base = strings.TrimSuffix(base, ".md")
	base = datePrefixRe.ReplaceAllString(base, "")
	if base == "" {
		return filepath.Base(p)
	}
	return base
}

// formatReuseTitles renders the reused-title sample as a parenthical suffix,
// stable-ordered for deterministic output.
func formatReuseTitles(titles []string) string {
	if len(titles) == 0 {
		return ""
	}
	sorted := append([]string(nil), titles...)
	sort.Strings(sorted)
	return " (" + strings.Join(sorted, ", ") + ")"
}
