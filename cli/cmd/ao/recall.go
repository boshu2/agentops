// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/storage"
)

var recallLimit int

// recallCmd is the unified agent-memory front door. It recalls curated durable
// facts (learnings, patterns, findings, decisions) across BOTH memory tiers —
// the per-project repo .agents/ corpus and the per-machine ~/.agents/ hub —
// using ao's existing lexical + freshness-decay + maturity-weighted scorer.
//
// v1 is deliberately lexical-only (no embeddings, no CGO): dense retrieval is
// earned later by a miss-log, per docs/memory-v1.md. recall is harness-agnostic
// by construction — Claude, Codex, and Gemini all reach it as `ao recall`, which
// is the whole point: one memory every agent shares.
var recallCmd = &cobra.Command{
	Use:   "recall <query>",
	Short: "Recall curated memory across project + machine tiers (the unified agent-memory front door)",
	Long: `Recall durable curated facts across both memory tiers.

ao recall is the single, harness-agnostic memory interface: it queries the
per-project repo .agents/ corpus AND the per-machine ~/.agents/ global hub with
ao's existing lexical + freshness-decay + maturity scorer, merges and de-dupes,
and returns ranked, cited hits.

It intentionally recalls the CURATED corpus (learnings, patterns, findings,
decisions, research) — durable facts that change future agent behavior — not raw
session transcripts. Use ao search for transcript/session history.

Memory contract + done criteria: docs/memory-v1.md.`,
	Example: `  ao recall "why did we reject knowledge graphs"
  ao recall "LAW 0" --limit 5
  ao recall "memory architecture decision" --output json`,
	Args: cobra.ExactArgs(1),
	RunE: runRecall,
}

func init() {
	recallCmd.GroupID = "knowledge"
	rootCmd.AddCommand(recallCmd)
	recallCmd.Flags().IntVar(&recallLimit, "limit", 10, "Maximum results to return")
}

func runRecall(cmd *cobra.Command, args []string) error {
	query := args[0]

	if recallLimit <= 0 {
		return fmt.Errorf("--limit must be a positive integer (got %d)", recallLimit)
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would recall: %s\n", query)
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Project tier resolves from the PROJECT ROOT, not cwd: walk up to the nearest
	// ancestor that holds a .agents/ corpus so `ao recall` from any subdirectory
	// still finds the repo's memory (contract: per-project repo .agents/ corpus).
	projectRoot := recallProjectRoot(cwd)
	projectBase := filepath.Join(projectRoot, storage.DefaultBaseDir, storage.SessionsDir)

	var machineBase string
	if home, herr := os.UserHomeDir(); herr != nil {
		VerbosePrintf("recall: cannot resolve home dir for machine tier: %v\n", herr)
	} else {
		machineBase = filepath.Join(home, storage.DefaultBaseDir, storage.SessionsDir)
	}

	// Each tier is searched with a GENEROUS internal limit (not the user's
	// --limit) so the final ranking is GLOBAL across both tiers, not distorted by
	// per-tier truncation: with --limit 1, truncating each tier to 1 before the
	// merge could drop the globally-best hit. We over-fetch, merge, then cap once.
	internalLimit := recallLimit * 8
	if internalLimit < 64 {
		internalLimit = 64
	}

	// Tier is tagged at COLLECTION time (which corpus search produced the hit) —
	// authoritative provenance, not a fragile after-the-fact path guess. De-dupe is
	// keyed on the path RELATIVE to the corpus root (e.g. "learnings/x.md"): the
	// machine hub (~/.agents) is populated BY `ao harvest` FROM project corpora, so
	// the same relative path across tiers is overwhelmingly the SAME harvested item
	// and must not consume two --limit slots. On collision the higher score wins; on
	// a tie the project (closer, authoritative) tier wins.
	//
	// KNOWN v1 TRADEOFF (documented, not a bug): two GENUINELY DISTINCT memories that
	// happen to share a relative path across tiers also collapse, surfacing the
	// project copy. This is rare (harvest namespaces by origin) and the safer default
	// than showing duplicate-looking hits. Exact cross-tier content identity needs a
	// content hash — deferred to v2 (the dense/enrichment lane, docs/memory-v1.md),
	// not lexical v1.
	byKey := make(map[string]recallHit)
	collect := func(tier, base string) {
		for _, r := range searchRepoCuratedKnowledge(query, base, internalLimit) {
			key := corpusRelativeKey(r.Path)
			if prev, seen := byKey[key]; seen && prev.Score >= r.Score {
				continue
			}
			byKey[key] = recallHit{Tier: tier, Path: r.Path, Score: r.Score, Snippet: r.Context}
		}
	}
	// When the resolved project corpus IS the machine hub (e.g. recall run from a
	// non-repo dir under $HOME, where recallProjectRoot climbs to ~), there is no
	// DISTINCT project tier — collect machine-only, labeled correctly, rather than
	// mislabeling the hub as "project" and hiding that no project memory exists.
	projectIsMachine := machineBase != "" && projectBase == machineBase
	if !projectIsMachine {
		// Project tier: the repo's curated .agents/ corpus (NOT raw sessions —
		// recall serves durable curated memory; raw transcripts are `ao search`).
		collect("project", projectBase)
	}
	// Machine tier: the per-machine ~/.agents/ global hub (always labeled machine).
	if machineBase != "" {
		collect("machine", machineBase)
	}

	// Rank by score desc, then path asc (stable), and cap to the limit.
	hits := make([]recallHit, 0, len(byKey))
	for _, h := range byKey {
		hits = append(hits, h)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Path < hits[j].Path
	})
	if len(hits) > recallLimit {
		hits = hits[:recallLimit]
	}

	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(hits)
	}

	if len(hits) == 0 {
		fmt.Printf("No memory found for: %s\n", query)
		return nil
	}
	fmt.Printf("Recalled %d cited memory result(s) for: %s\n", len(hits), query)
	for i, h := range hits {
		fmt.Printf("\n%d. [%s] %s\n", i+1, h.Tier, h.Path)
		if s := firstNonEmptyLine(h.Snippet); s != "" {
			fmt.Printf("   %s\n", s)
		}
	}
	return nil
}

// recallHit is a tier-annotated, cited memory result — the memory-v1 output
// contract: {tier, path, score, snippet}. Tier distinguishes the per-project
// repo corpus from the per-machine ~/.agents/ hub; path is the cited source file.
type recallHit struct {
	Tier    string  `json:"tier"`
	Path    string  `json:"path"`
	Score   float64 `json:"score,omitempty"`
	Snippet string  `json:"snippet,omitempty"`
}

// recallProjectRoot walks up from start to the nearest ancestor containing a
// .agents/ corpus, so recall finds the repo's memory from any subdirectory.
// Falls back to start when no ancestor has one. The marker is the corpus ROOT
// (.agents) — the curated dirs (learnings/council/findings) live directly under
// it — not storage.DefaultBaseDir (.agents/ao), which is only the session store.
func recallProjectRoot(start string) string {
	home, _ := os.UserHomeDir()
	return recallProjectRootFrom(start, home)
}

// recallProjectRootFrom is recallProjectRoot with home injected for testability.
// It never treats home as a project root: ~/.agents is the MACHINE tier, not a
// per-repo corpus — so a repo/subdir under $HOME that lacks its own .agents falls
// back to the start dir (cwd) rather than mis-resolving the project root to $HOME
// and suppressing the project tier.
func recallProjectRootFrom(start, home string) string {
	corpusRoot := filepath.Dir(storage.DefaultBaseDir) // ".agents/ao" -> ".agents"
	dir := start
	for {
		if home != "" && dir == home {
			return start // reached the machine hub without a repo corpus
		}
		if fi, err := os.Stat(filepath.Join(dir, corpusRoot)); err == nil && fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}

// firstNonEmptyLine returns the first non-blank line of s (the cited snippet).
func firstNonEmptyLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			return t
		}
	}
	return ""
}

// corpusRelativeKey returns the path relative to its .agents/ corpus root (e.g.
// "learnings/x.md"), used to de-dupe the SAME logical memory across the project
// and machine tiers (repo/.agents/... vs ~/.agents/...). Falls back to the full
// path when no .agents/ segment is present.
func corpusRelativeKey(path string) string {
	marker := filepath.Dir(storage.DefaultBaseDir) + string(os.PathSeparator) // ".agents/"
	if i := strings.LastIndex(path, marker); i >= 0 {
		return path[i+len(marker):]
	}
	return path
}
