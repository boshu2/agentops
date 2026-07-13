// practices: [hexagonal-architecture, escape-corpus-self-improvement]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ports"
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
	membraneDigestDeltas              bool
	membraneDigestSince               string
)

var membraneDigestCmd = &cobra.Command{
	Use:   "digest [--top N] [--json] [--deltas --since <date>]",
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
nothing more — no compounding-moat or self-improvement claim.

DELTAS MODE (age-de5t): --deltas --since <ISO-date|RFC3339> is the producer-defect
register's honesty check — for each catch class it prints hits BEFORE vs hits SINCE
the cutoff (a producer fix's land date, e.g. ` + "`git show -s --format=%cI <sha>`" + `),
sorted still-recurring first (since DESC, before DESC); a 0-since class is marked
improved. READ-ONLY: it writes no checklist and never edits the register — the
post-mortem runner (BP.7) records the numbers into
docs/architecture/producer-defect-register.md. --top does not apply (every class is
shown, so the class you fixed is always visible); --json prints the same shape
machine-readably.`,
	RunE: runMembraneDigest,
}

func init() {
	membraneCmd.AddCommand(membraneDigestCmd)
	membraneDigestCmd.Flags().IntVar(&membraneDigestTopN, "top", catchDigestDefaultTopN, "How many top recurring catch classes to include")
	membraneDigestCmd.Flags().BoolVar(&membraneDigestJSON, "json", false, "Also print the ranked digest as JSON (the checklist file is written either way)")
	membraneDigestCmd.Flags().BoolVar(&membraneDigestIncludePlaceholders, "include-placeholders", false, "Include reason-less placeholder classes (e.g. \"pawl-review REFUTED (see evidence)\") for corpus auditing; excluded by default so the checklist stays actionable")
	membraneDigestCmd.Flags().BoolVar(&membraneDigestDeltas, "deltas", false, "Per-class recurrence before vs since --since (read-only; for the producer-defect register)")
	membraneDigestCmd.Flags().StringVar(&membraneDigestSince, "since", "", "Cutoff for --deltas: an ISO date (2026-07-08, UTC midnight) or RFC3339 timestamp — typically a producer fix's land date")
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
	// ProducerCandidates are advisory proposals backed by at least two distinct
	// objectives. Review retries inside one objective never inflate recurrence.
	ProducerCandidates []ports.ProducerRuleCandidate `json:"producer_candidates"`
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
func buildCatchDigest(all []yieldledger.Catch, topN int, includePlaceholders bool, now time.Time) (catchDigest, error) {
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
	candidates, err := newProductionFindingRecurrenceReducer().Reduce(context.Background(), findingObservationsForCatches(all))
	if err != nil {
		return catchDigest{}, fmt.Errorf("reconcile producer candidates: %w", err)
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
		ProducerCandidates:  candidates,
	}, nil
}

// findingObservationsForCatches projects one representative observation per
// (class, objective) from the catch corpus. Catch.Beads already contains unique
// objective IDs, so review rounds and new heads inside one objective collapse
// before the recurrence reducer sees them.
func findingObservationsForCatches(catches []yieldledger.Catch) []ports.FindingObservation {
	observations := make([]ports.FindingObservation, 0)
	for _, catch := range catches {
		if yieldledger.IsPlaceholderReason(catch.Reason) {
			continue
		}
		for _, objectiveID := range catch.Beads {
			observations = append(observations, ports.FindingObservation{
				ID:          catch.ClassKey + "@" + objectiveID,
				ClassKey:    catch.ClassKey,
				ObjectiveID: objectiveID,
				EvidenceRef: ".agents/yield/yield-ledger.jsonl#objective=" + objectiveID,
				Summary:     catch.Reason,
			})
		}
	}
	return observations
}

// catchDeltaEntry is one class's recurrence split around the --since cutoff: how
// often the membrane caught it BEFORE vs SINCE. It is the row the producer-defect
// register's "Recurrence before → after" column is filled from (age-de5t).
type catchDeltaEntry struct {
	ClassKey string   `json:"class_key"`
	Domain   string   `json:"domain"`
	Reason   string   `json:"reason"`
	Before   int      `json:"before"`
	Since    int      `json:"since"`
	Beads    []string `json:"beads,omitempty"`
	// Improved marks the loop's success shape: the class WAS being caught (before>0)
	// and has not recurred since the cutoff (since==0).
	Improved    bool `json:"improved"`
	Placeholder bool `json:"placeholder,omitempty"`
}

// catchDeltas is the whole deltas report — the --json shape and the render input.
type catchDeltas struct {
	GeneratedAt         string            `json:"generated_at"`
	Since               string            `json:"since"`
	TotalClasses        int               `json:"total_classes"`
	PlaceholderClasses  int               `json:"placeholder_classes"`
	IncludePlaceholders bool              `json:"include_placeholders"`
	Entries             []catchDeltaEntry `json:"entries"`
}

// parseDeltaCutoff parses the --since cutoff: a bare ISO date (UTC midnight) or a
// full RFC3339 timestamp. Deliberately git-free — resolving a ref would couple a
// pure ledger measurement to repo state; the runner gets a fix's land date with
// `git show -s --format=%cI <sha>` and pastes it. The result is normalized to UTC.
func parseDeltaCutoff(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("ao membrane digest: --since %q is not an ISO date (2026-07-08) or RFC3339 timestamp", s)
}

// buildCatchDeltas splits every class's round-collapsed instances around cutoff
// (before: ts < cutoff; since: ts >= cutoff) and sorts still-recurring classes
// first (Since DESC, then Before DESC, then ClassKey ASC) — so the classes whose
// producer fixes did NOT hold lead, and every 0-since class trails as improved.
// Placeholder classes are filtered exactly as in the default digest mode. Unlike
// the digest this is a flat measurement, not an attention ranking: no topN, and
// included placeholders sort inline (tagged), not below.
func buildCatchDeltas(all []yieldledger.Catch, cutoff time.Time, includePlaceholders bool, now time.Time) catchDeltas {
	d := catchDeltas{
		GeneratedAt:         now.UTC().Format(time.RFC3339),
		Since:               cutoff.UTC().Format(time.RFC3339),
		TotalClasses:        len(all),
		IncludePlaceholders: includePlaceholders,
		Entries:             []catchDeltaEntry{},
	}
	for _, c := range all {
		placeholder := yieldledger.IsPlaceholderReason(c.Reason)
		if placeholder {
			d.PlaceholderClasses++
			if !includePlaceholders {
				continue
			}
		}
		before, since := 0, 0
		for _, inst := range c.Instances {
			// Load validates every envelope ts as RFC3339, so this parse cannot fail
			// on a loaded ledger; if it ever did, count the hit as BEFORE — the
			// conservative side (it can only understate an improvement, never fake one).
			ts, err := time.Parse(time.RFC3339, inst.TS)
			if err != nil || ts.Before(cutoff) {
				before++
			} else {
				since++
			}
		}
		d.Entries = append(d.Entries, catchDeltaEntry{
			ClassKey:    c.ClassKey,
			Domain:      c.Domain,
			Reason:      c.Reason,
			Before:      before,
			Since:       since,
			Beads:       c.Beads,
			Improved:    before > 0 && since == 0,
			Placeholder: placeholder,
		})
	}
	sort.SliceStable(d.Entries, func(i, j int) bool {
		a, b := d.Entries[i], d.Entries[j]
		if a.Since != b.Since {
			return a.Since > b.Since
		}
		if a.Before != b.Before {
			return a.Before > b.Before
		}
		return a.ClassKey < b.ClassKey
	})
	return d
}

// runMembraneDigestDeltas is the --deltas lane: a READ-ONLY per-class
// before/since measurement printed for the post-mortem runner to record into the
// producer-defect register. It writes no checklist and never edits the register —
// a doc-mutating command is a bigger decision than this measurement (age-de5t).
func runMembraneDigestDeltas(cmd *cobra.Command) error {
	cutoff, err := parseDeltaCutoff(membraneDigestSince)
	if err != nil {
		return err
	}
	root, err := repoRootOrCwd()
	if err != nil {
		return err
	}
	ledger, err := yieldledger.Load(root)
	if err != nil {
		return err
	}
	d := buildCatchDeltas(yieldledger.DetectCatches(ledger), cutoff, membraneDigestIncludePlaceholders, time.Now())

	out := cmd.OutOrStdout()
	if membraneDigestJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}
	if len(d.Entries) == 0 {
		if d.PlaceholderClasses > 0 && !d.IncludePlaceholders {
			fmt.Fprintf(out, "membrane deltas: no actionable catch classes — filtered %d reason-less placeholder class(es) (--include-placeholders to audit)\n", d.PlaceholderClasses)
		} else {
			fmt.Fprintln(out, "membrane deltas: no catch classes in the ledger — nothing to measure")
		}
		return nil
	}
	fmt.Fprintf(out, "membrane deltas: %d catch class(es) vs cutoff %s — hits before vs since (0 since = improved)\n\n", len(d.Entries), d.Since)
	for _, e := range d.Entries {
		tag := ""
		if e.Improved {
			tag = " [improved]"
		}
		if e.Placeholder {
			tag += " (placeholder)"
		}
		fmt.Fprintf(out, "  before=%d since=%d  %s (%s)%s\n", e.Before, e.Since, e.Reason, e.Domain, tag)
	}
	fmt.Fprintf(out, "\nRecord as \"before → after\" in docs/architecture/producer-defect-register.md (BP.7).\n")
	return nil
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
	if len(d.ProducerCandidates) > 0 {
		b.WriteString("\n## Advisory producer-rule candidates\n\n")
		b.WriteString("These are bookkeeping candidates, not policy or blockers. Recurrence counts distinct objectives, not review retries.\n\n")
		for _, candidate := range d.ProducerCandidates {
			objectives := make([]string, 0, len(candidate.Evidence))
			for _, evidence := range candidate.Evidence {
				objectives = append(objectives, evidence.ObjectiveID)
			}
			fmt.Fprintf(&b, "- `%s` — recurrence=%d; objectives: `%s`; %s\n",
				candidate.ID, candidate.RecurrenceCount, strings.Join(objectives, "`, `"), candidate.Summary)
		}
	}
	b.WriteString("\nSource: `.agents/yield/yield-ledger.jsonl` (catch corpus).\n")
	return []byte(b.String())
}

// runMembraneDigest loads the yield ledger, detects catch classes, ranks them
// globally, and writes the checklist to the auto-mined pre-mortem-checks sink.
func runMembraneDigest(cmd *cobra.Command, _ []string) error {
	// --deltas/--since pair or neither: --since without --deltas would silently do
	// nothing, and --deltas without a cutoff has nothing to measure against.
	if membraneDigestSince != "" && !membraneDigestDeltas {
		return fmt.Errorf("ao membrane digest: --since requires --deltas")
	}
	if membraneDigestDeltas {
		if strings.TrimSpace(membraneDigestSince) == "" {
			return fmt.Errorf("ao membrane digest: --deltas requires --since <ISO-date|RFC3339> (the producer fix's land date)")
		}
		return runMembraneDigestDeltas(cmd)
	}
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
	digest, err := buildCatchDigest(yieldledger.DetectCatches(ledger), membraneDigestTopN, membraneDigestIncludePlaceholders, time.Now())
	if err != nil {
		return err
	}

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
