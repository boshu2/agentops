// practices: [hexagonal-architecture, escape-corpus-self-improvement]
package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// `ao membrane digest` mines the ABUNDANT catch corpus (every REFUTED gate-verdict
// that carries a domain+reason) into ONE global, top-N recurring-defect checklist
// that the START of the loop consumes (age-xbmf, shift-left epic age-tc0l).
//
// It is the domain-LESS twin of `ao membrane recall --include-catches`: recall is a
// BOUNDED review-time query ("what has been caught in THIS domain"); digest ranks
// classes by HitCount ACROSS ALL DOMAINS so a reviewer/planner can front-load the
// most-recurring misses before touching anything. Reuse — not a re-build — of
// yieldledger.DetectCatches (same ClassKey/Reason/AffectedPaths/HitCount extraction
// recall uses); the only new behavior is the global ranking + the checklist sink.
//
// HONEST SCOPE (ADR-0004/0011): the digest is "recurring catch classes to watch
// for", nothing more. No compounding-moat claim, no self-improvement flywheel
// language — a recurring class is just a recurring class.

const (
	// catchDigestDefaultTopN is a sensible default cap: enough to surface the real
	// recurring classes without turning the checklist into a wall the reviewer skims.
	catchDigestDefaultTopN = 10
	// catchDigestRelPath is the AUTO-mined consumption sink — the SAME
	// .agents/pre-mortem-checks/ directory /pre-mortem loads (Step 1.4b) and
	// `ao membrane derive-checks` writes ESCAPE checks into. One well-known
	// filename (not one-file-per-class) so a re-run overwrites cleanly.
	catchDigestRelPath = ".agents/pre-mortem-checks/catch-digest.md"
)

var (
	membraneDigestTopN                int
	membraneDigestJSON                bool
	membraneDigestIncludePlaceholders bool
)

var membraneDigestCmd = &cobra.Command{
	Use:   "digest [--top N] [--json]",
	Short: "Mine the catch corpus into a GLOBAL top-N recurring-defect checklist the loop's START consumes",
	Long: `Mine the ABUNDANT catch corpus into a single GLOBAL top-N recurring-defect
checklist (age-xbmf). Every REFUTED gate-verdict carrying a domain+reason is a
catch; DetectCatches groups them into classes by ClassKey. digest ranks those
classes by HitCount ACROSS ALL DOMAINS (stable tie-break by ClassKey) and writes
the top N — each as a "<reason> -> watch for it ..." imperative line — to

    .agents/pre-mortem-checks/catch-digest.md

the SAME sink /pre-mortem loads at Step 1.4b and ` + "`ao membrane derive-checks`" + `
writes escape checks into, so the START of the loop front-loads the most-recurring
misses before touching anything. This is the domain-LESS twin of
` + "`ao membrane recall --include-catches`" + ` (a per-domain review-time query).

ACTIONABLE by default: reason-less PLACEHOLDER classes — a bare pawl verdict
("pawl-review REFUTED (see evidence)"), a bare token ("r"), disposition boilerplate
— carry no defect content, so a "watch for: pawl-review REFUTED" line is pure noise.
They are EXCLUDED by default so real-reason classes lead the checklist. Pass
--include-placeholders to restore them (for corpus auditing) — they always rank BELOW
every actionable class.

The file is (re)generated on every run — idempotent, safe to re-run. It is the
AUTO-mined sink and is kept SEPARATE from the human-curated
docs/gate/findings-ledger.md (the Standing Review Dimensions that behavior-first
planning reads): digest never writes the curated ledger, and the checklist is
marked "do not hand-edit". --json additionally prints the ranked list as JSON.

HONEST SCOPE (ADR-0004/0011): these are recurring catch classes to watch for,
nothing more — no compounding-moat or self-improvement claim.`,
	RunE: runMembraneDigest,
}

func init() {
	membraneCmd.AddCommand(membraneDigestCmd)
	membraneDigestCmd.Flags().IntVar(&membraneDigestTopN, "top", catchDigestDefaultTopN, "How many top recurring catch classes to include")
	membraneDigestCmd.Flags().BoolVar(&membraneDigestJSON, "json", false, "Also print the ranked digest as JSON (the checklist file is written either way)")
	membraneDigestCmd.Flags().BoolVar(&membraneDigestIncludePlaceholders, "include-placeholders", false, "Include reason-less placeholder classes (e.g. \"pawl-review REFUTED (see evidence)\") for corpus auditing; excluded by default so the checklist stays actionable")
}

// catchDigestEntry is one ranked recurring catch class in the digest.
type catchDigestEntry struct {
	Rank          int      `json:"rank"`
	ClassKey      string   `json:"class_key"`
	Domain        string   `json:"domain"`
	Reason        string   `json:"reason"`
	HitCount      int      `json:"hit_count"`
	Beads         []string `json:"beads,omitempty"`
	AffectedPaths []string `json:"affected_paths,omitempty"`
	// WatchFor is the deterministic "watch-for-this" imperative (no LLM call).
	WatchFor string `json:"watch_for"`
	// Placeholder marks a reason-less class (no defect content) — only ever surfaced
	// under --include-placeholders, and always ranked below the actionable classes.
	Placeholder bool `json:"placeholder,omitempty"`
}

// catchDigest is the whole ranked digest — the JSON shape and the render input.
type catchDigest struct {
	GeneratedAt string `json:"generated_at"`
	TopN        int    `json:"top_n"`
	// TotalClasses is the FULL corpus size (actionable + placeholder), so the digest
	// reports how much it summarizes even when placeholders are filtered out.
	TotalClasses int `json:"total_classes"`
	// ActionableClasses is the count of real-reason (non-placeholder) classes — what a
	// planner can actually act on. PlaceholderClasses is the reason-less remainder that
	// is filtered by default (age-7758).
	ActionableClasses   int                `json:"actionable_classes"`
	PlaceholderClasses  int                `json:"placeholder_classes"`
	IncludePlaceholders bool               `json:"include_placeholders"`
	TotalHits           int                `json:"total_hits"`
	Entries             []catchDigestEntry `json:"entries"`
}

// rankCatchDigest returns catches sorted by HitCount DESC, tie-broken by ClassKey
// ASC (stable + fully deterministic), truncated to topN (topN<=0 keeps all). Pure:
// it copies the input so DetectCatches' first-appearance order is never mutated.
func rankCatchDigest(catches []yieldledger.Catch, topN int) []yieldledger.Catch {
	ranked := make([]yieldledger.Catch, len(catches))
	copy(ranked, catches)
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].HitCount != ranked[j].HitCount {
			return ranked[i].HitCount > ranked[j].HitCount
		}
		return ranked[i].ClassKey < ranked[j].ClassKey
	})
	if topN > 0 && len(ranked) > topN {
		ranked = ranked[:topN]
	}
	return ranked
}

// catchWatchFor renders the deterministic "watch-for-this" imperative for a class:
// the reason (left of the arrow) names the defect; this directive tells a future
// reviewer to watch for it, scoped to the class's domain and — when present — the
// files it has recurred on. Deterministic transform, no external LLM call.
func catchWatchFor(c yieldledger.Catch) string {
	directive := "watch for it"
	if d := strings.TrimSpace(c.Domain); d != "" {
		directive += " when working in `" + d + "`"
	}
	if hint := digestPathsHint(c.AffectedPaths); hint != "" {
		directive += " (" + hint + ")"
	}
	return directive
}

// digestPathsHint joins up to 3 affected paths, appending "+N more" for the tail,
// so a checklist line stays scannable even for a class touching many files.
func digestPathsHint(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	const maxShown = 3
	shown := paths
	extra := 0
	if len(paths) > maxShown {
		shown = paths[:maxShown]
		extra = len(paths) - maxShown
	}
	s := strings.Join(shown, ", ")
	if extra > 0 {
		s += fmt.Sprintf(", +%d more", extra)
	}
	return s
}

// buildCatchDigest partitions catch classes into ACTIONABLE (a real defect reason)
// and PLACEHOLDER (reason-less boilerplate — "pawl-review REFUTED (see evidence)", a
// bare token) via yieldledger.IsPlaceholderReason, then ranks and assembles the digest.
//
// The whole point of the digest is an ACTIONABLE pre-mortem checklist, so placeholders
// are EXCLUDED by default: injecting "watch for: pawl-review REFUTED (see evidence)" is
// pure noise. --include-placeholders (includePlaceholders=true) restores them for
// corpus auditing, but always BELOW every actionable class — real-reason classes lead,
// then placeholders, then topN truncates the combined list (so it trims placeholders
// before any actionable class). now is injected for deterministic render under test;
// TotalClasses/TotalHits are over ALL classes so the checklist reports what it filtered.
func buildCatchDigest(all []yieldledger.Catch, topN int, includePlaceholders bool, now time.Time) catchDigest {
	var actionable, placeholders []yieldledger.Catch
	for _, c := range all {
		if yieldledger.IsPlaceholderReason(c.Reason) {
			placeholders = append(placeholders, c)
		} else {
			actionable = append(actionable, c)
		}
	}

	var ranked []yieldledger.Catch
	if includePlaceholders {
		// Actionable classes ALWAYS lead; placeholders trail. topN caps the combined
		// list, so it trims placeholders before it ever drops an actionable class.
		ranked = append(rankCatchDigest(actionable, 0), rankCatchDigest(placeholders, 0)...)
		if topN > 0 && len(ranked) > topN {
			ranked = ranked[:topN]
		}
	} else {
		ranked = rankCatchDigest(actionable, topN)
	}

	totalHits := 0
	for _, c := range all {
		totalHits += c.HitCount
	}
	entries := make([]catchDigestEntry, 0, len(ranked))
	for i, c := range ranked {
		entries = append(entries, catchDigestEntry{
			Rank:          i + 1,
			ClassKey:      c.ClassKey,
			Domain:        c.Domain,
			Reason:        c.Reason,
			HitCount:      c.HitCount,
			Beads:         c.Beads,
			AffectedPaths: c.AffectedPaths,
			WatchFor:      catchWatchFor(c),
			Placeholder:   yieldledger.IsPlaceholderReason(c.Reason),
		})
	}
	return catchDigest{
		GeneratedAt:         now.UTC().Format(time.RFC3339),
		TopN:                topN,
		TotalClasses:        len(all),
		ActionableClasses:   len(actionable),
		PlaceholderClasses:  len(placeholders),
		IncludePlaceholders: includePlaceholders,
		TotalHits:           totalHits,
		Entries:             entries,
	}
}

// renderCatchDigest serializes the digest into the .agents/pre-mortem-checks/*.md
// shape /pre-mortem's loader reads: YAML frontmatter (type: pre-mortem-check,
// status: active, applicable_when) then a `# Pre-Mortem Check:` heading and the
// ranked "<reason> -> <imperative>" checklist. Deterministic given the digest.
func renderCatchDigest(d catchDigest) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: \"catch-digest\"\n")
	b.WriteString("type: \"pre-mortem-check\"\n")
	b.WriteString("source: \"catch-corpus\"\n")
	b.WriteString("status: \"active\"\n")
	b.WriteString("generated_by: \"ao membrane digest\"\n")
	fmt.Fprintf(&b, "generated_at: %q\n", d.GeneratedAt)
	b.WriteString("applicable_when: [\"recurring-catch\"]\n")
	fmt.Fprintf(&b, "top_n: %d\n", d.TopN)
	fmt.Fprintf(&b, "total_classes: %d\n", d.TotalClasses)
	b.WriteString("---\n\n")

	b.WriteString("# Pre-Mortem Check: Recurring catch classes to watch for (catch-digest)\n\n")
	b.WriteString("The membrane's most-recurring REFUTED defect classes, ranked by hit count\n")
	b.WriteString("across ALL domains — mined from the catch corpus (`.agents/yield/yield-ledger.jsonl`)\n")
	b.WriteString("by `ao membrane digest`. **Advisory only:** these are recurring classes to watch\n")
	b.WriteString("for, NOT a compounding moat — a recurring class is just a recurring class. Before\n")
	b.WriteString("confirming a change, verify it does not re-introduce any of these.\n\n")
	b.WriteString("This is the AUTO-mined sink: it is regenerated on every `ao membrane digest` run\n")
	b.WriteString("and is kept separate from the human-curated `docs/gate/findings-ledger.md` (the\n")
	b.WriteString("Standing Review Dimensions). **Do not hand-edit this file.**\n\n")

	// Reason-less placeholder classes ("pawl-review REFUTED (see evidence)", bare "r")
	// carry no defect content, so they are filtered by default (age-7758). Report the
	// filtering honestly so a reader knows the corpus is larger than the checklist.
	if d.PlaceholderClasses > 0 && !d.IncludePlaceholders {
		fmt.Fprintf(&b, "_Filtered %d reason-less placeholder class(es) with no defect content "+
			"(run `ao membrane digest --include-placeholders` to audit them)._\n\n", d.PlaceholderClasses)
	}

	if len(d.Entries) == 0 {
		if d.TotalClasses == 0 {
			b.WriteString("_No classifiable catch classes recorded yet — clean corpus (or no data)._\n")
		} else {
			// Corpus is non-empty but ENTIRELY placeholders: the honest result is an
			// empty actionable checklist — the corpus needs real-reason catches to accrue.
			fmt.Fprintf(&b, "_No actionable catch classes yet — all %d recorded class(es) are reason-less "+
				"placeholders. The checklist becomes useful as real-reason catches accrue._\n", d.PlaceholderClasses)
		}
		return []byte(b.String())
	}

	for _, e := range d.Entries {
		// One imperative line per class: "<reason> -> watch for it ...". A placeholder
		// (only shown under --include-placeholders) is tagged so it is never mistaken
		// for an actionable line.
		tag := ""
		if e.Placeholder {
			tag = " _(placeholder — no defect content)_"
		}
		fmt.Fprintf(&b, "%d. **[×%d]** %s → %s%s\n", e.Rank, e.HitCount, e.Reason, e.WatchFor, tag)
	}
	b.WriteString("\nSource: `.agents/yield/yield-ledger.jsonl` (catch corpus).\n")
	return []byte(b.String())
}

// runMembraneDigest loads the yield ledger, detects catch classes, ranks them
// globally, and writes the checklist to the auto-mined pre-mortem-checks sink.
func runMembraneDigest(cmd *cobra.Command, _ []string) error {
	if membraneDigestTopN <= 0 {
		return fmt.Errorf("ao membrane digest: --top must be > 0 (got %d)", membraneDigestTopN)
	}
	// repoRootOrCwd (not resolveProjectDir): read the REPO-ROOT ledger even from a
	// subdir (cli/) or a linked worktree, matching recall/triage/catch — a raw-cwd
	// read would fail OPEN on an empty <cwd>/.agents/yield (age-6sg.1 class).
	root, err := repoRootOrCwd()
	if err != nil {
		return err
	}
	ledger, err := yieldledger.Load(root)
	if err != nil {
		return err
	}
	digest := buildCatchDigest(yieldledger.DetectCatches(ledger), membraneDigestTopN, membraneDigestIncludePlaceholders, time.Now())

	abs := filepath.Join(root, filepath.FromSlash(catchDigestRelPath))
	if err := writeFindingFileAtomic(abs, renderCatchDigest(digest), 0o644); err != nil {
		return fmt.Errorf("ao membrane digest: write %s: %w", catchDigestRelPath, err)
	}

	out := cmd.OutOrStdout()
	if membraneDigestJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(digest)
	}
	if len(digest.Entries) == 0 {
		if digest.PlaceholderClasses > 0 && !digest.IncludePlaceholders {
			fmt.Fprintf(out, "membrane digest: no ACTIONABLE catch classes yet — filtered %d reason-less placeholder class(es); wrote checklist to %s (--include-placeholders to audit)\n",
				digest.PlaceholderClasses, catchDigestRelPath)
		} else {
			fmt.Fprintf(out, "membrane digest: no classifiable catch classes yet — wrote empty checklist to %s\n", catchDigestRelPath)
		}
		return nil
	}
	denom := digest.ActionableClasses
	suffix := ""
	if digest.IncludePlaceholders {
		denom = digest.TotalClasses
		suffix = " (incl. placeholders)"
	} else if digest.PlaceholderClasses > 0 {
		suffix = fmt.Sprintf(" (filtered %d reason-less placeholder class(es))", digest.PlaceholderClasses)
	}
	fmt.Fprintf(out, "membrane digest: top %d of %d catch class(es)%s → %s\n\n",
		len(digest.Entries), denom, suffix, catchDigestRelPath)
	for _, e := range digest.Entries {
		tag := ""
		if e.Placeholder {
			tag = " (placeholder)"
		}
		fmt.Fprintf(out, "  %d. [×%d] %s → %s%s\n", e.Rank, e.HitCount, e.Reason, e.WatchFor, tag)
	}
	return nil
}
