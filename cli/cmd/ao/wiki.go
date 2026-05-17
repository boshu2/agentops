// practices: [wiki-knowledge-surface, refactoring]
//
// This file implements the `ao wiki` command group — the Wave 5 surface of
// the unified wiki bounded context (epic soc-behj). It is a thin cobra layer:
// every subcommand delegates to the Wave 1-4 cli/internal/wiki package APIs
// (FrontmatterCodec, CorpusLocator, WikiIndex, WikiPipeline, FreshnessPolicy).
//
// Strangler note: this group is ACCRETIVE. The legacy commands (`ao inject`,
// `ao lookup`, `ao compile`, `ao index`, ...) remain registered and behavior-
// identical; `ao wiki` is a new, experimental consolidation surface that does
// not replace them. The new surface is marked experimental in its help text.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/wiki"
)

// defaultWikiIndexFile is the JSONL file the WikiIndex persists to, relative
// to the resolved .agents/ corpus directory.
const defaultWikiIndexFile = "wiki-index.jsonl"

// wikiSearchMaxResults caps how many ranked hits `ao wiki search` prints.
const wikiSearchMaxResults = 20

// wikiCmd is the root of the experimental `ao wiki` command group.
var wikiCmd = &cobra.Command{
	Use:   "wiki",
	Short: "Unified wiki knowledge surface (experimental)",
	Long: `ao wiki is the experimental unified surface for the wiki bounded context.

It consolidates ao's .agents/-touching logic — frontmatter parsing, corpus
location, indexing, the LLM-wiki pipeline, and claim freshness — behind one
command group.

EXPERIMENTAL: this surface is new in this release. The legacy commands
(ao inject, ao lookup, ao compile, ao index, ...) remain fully supported and
behavior-identical; ao wiki does not replace them yet.

Subcommands:
  index     Build/refresh the persistent document index
  search    Rank indexed documents against a query
  inject    Show how to assemble just-in-time context (delegates to ao inject)
  lint      Run the wiki pipeline LINT stage
  query     Run the wiki pipeline QUERY stage
  promote   Run the wiki pipeline PROMOTE stage
  doctor    Diagnose the wiki corpus and index`,
}

// wikiIndexCmd builds or refreshes the persistent WikiIndex.
var wikiIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "Build or refresh the persistent wiki document index (experimental)",
	Long: `Scan the .agents/ corpus and update the persistent JSONL document index.

The index is content-addressed and incremental: only changed files are
rewritten. ao wiki search reads this index, so run ao wiki index first.`,
	RunE:         runWikiIndex,
	SilenceUsage: true,
}

// wikiSearchCmd ranks indexed documents against a query.
var wikiSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Rank indexed wiki documents against a query (experimental)",
	Long: `Rank documents in the wiki index against a free-text query.

Results are ordered by term-match score. Run ao wiki index first to build
the index; pass --reindex to rebuild it inline before searching.`,
	Args:         cobra.MinimumNArgs(1),
	RunE:         runWikiSearch,
	SilenceUsage: true,
}

// wikiInjectCmd is the strangler alias entry for context injection.
var wikiInjectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Assemble just-in-time context (experimental; see ao inject)",
	Long: `Assemble just-in-time .agents/ context.

This is the experimental wiki-surface entry point for context injection. The
canonical, fully-flagged implementation is the legacy ao inject command, which
remains supported and behavior-identical. ao wiki inject delegates to it.`,
	RunE:         runWikiInject,
	SilenceUsage: true,
}

// wikiLintCmd runs the pipeline LINT stage.
var wikiLintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Run the wiki pipeline LINT stage (experimental)",
	Long: `Walk the wiki tree and write a dated lint report.

Delegates to wiki.WikiPipeline's LINT stage. The vault root defaults to the
current directory; override with --vault.`,
	RunE:         runWikiLint,
	SilenceUsage: true,
}

// wikiQueryCmd runs the pipeline QUERY stage.
var wikiQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "Run the wiki pipeline QUERY stage (experimental)",
	Long: `Answer a pending wiki question into wiki/synthesis/.

Delegates to wiki.WikiPipeline's QUERY stage. The vault root defaults to the
current directory; override with --vault.`,
	RunE:         runWikiQuery,
	SilenceUsage: true,
}

// wikiPromoteCmd runs the pipeline PROMOTE stage.
var wikiPromoteCmd = &cobra.Command{
	Use:   "promote",
	Short: "Run the wiki pipeline PROMOTE stage (experimental)",
	Long: `Promote mature wiki pages into authored content.

Delegates to wiki.WikiPipeline's PROMOTE stage. PROMOTE needs a promoter the
wiki package does not own, so on this surface it reports the stage's
not-configured skip rather than performing a promotion.`,
	RunE:         runWikiPromote,
	SilenceUsage: true,
}

// wikiDoctorCmd diagnoses the wiki corpus and index.
var wikiDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose the wiki corpus and index (experimental)",
	Long: `Report on the wiki corpus directory and the persistent index.

Shows the resolved .agents/ corpus root, whether it exists, the index path,
and how many documents are currently indexed.`,
	RunE:         runWikiDoctor,
	SilenceUsage: true,
}

func init() {
	wikiCmd.GroupID = "knowledge"
	rootCmd.AddCommand(wikiCmd)

	wikiCmd.AddCommand(wikiIndexCmd)
	wikiCmd.AddCommand(wikiSearchCmd)
	wikiCmd.AddCommand(wikiInjectCmd)
	wikiCmd.AddCommand(wikiLintCmd)
	wikiCmd.AddCommand(wikiQueryCmd)
	wikiCmd.AddCommand(wikiPromoteCmd)
	wikiCmd.AddCommand(wikiDoctorCmd)

	wikiIndexCmd.Flags().String("base", "", "Corpus base directory (default: current directory)")

	wikiSearchCmd.Flags().String("base", "", "Corpus base directory (default: current directory)")
	wikiSearchCmd.Flags().Bool("reindex", false, "Rebuild the index before searching")
	wikiSearchCmd.Flags().Int("limit", wikiSearchMaxResults, "Maximum results to print")

	wikiLintCmd.Flags().String("vault", "", "Vault root (default: current directory)")
	wikiQueryCmd.Flags().String("vault", "", "Vault root (default: current directory)")
	wikiPromoteCmd.Flags().String("vault", "", "Vault root (default: current directory)")
	wikiDoctorCmd.Flags().String("base", "", "Corpus base directory (default: current directory)")
}

// wikiResolveBase resolves the corpus base directory for a wiki subcommand.
// An explicit --base flag wins; otherwise the current working directory is
// used. The returned path is the base, not the .agents/ dir — callers pass it
// through wiki.CorpusLocator.
func wikiResolveBase(cmd *cobra.Command) (string, error) {
	base, _ := cmd.Flags().GetString("base")
	if strings.TrimSpace(base) != "" {
		return base, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return cwd, nil
}

// wikiResolveVault resolves the vault root for a pipeline-stage subcommand.
// An explicit --vault flag wins; otherwise the current working directory is
// used.
func wikiResolveVault(cmd *cobra.Command) (string, error) {
	vault, _ := cmd.Flags().GetString("vault")
	if strings.TrimSpace(vault) != "" {
		return vault, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return cwd, nil
}

// wikiIndexPathFor returns the JSONL index path for a corpus base, located
// inside the CorpusLocator-resolved .agents/ directory.
func wikiIndexPathFor(base string) string {
	return filepath.Join(wiki.AgentsDirIn(base), defaultWikiIndexFile)
}

// runWikiIndex builds or refreshes the persistent WikiIndex for the corpus
// base, delegating to wiki.WikiIndex.Reindex.
func runWikiIndex(cmd *cobra.Command, _ []string) error {
	base, err := wikiResolveBase(cmd)
	if err != nil {
		return err
	}
	idx, err := wiki.NewWikiIndex(wikiIndexPathFor(base), base)
	if err != nil {
		return fmt.Errorf("construct wiki index: %w", err)
	}
	result, err := idx.Reindex(cmd.Context())
	if err != nil {
		return fmt.Errorf("reindex wiki corpus: %w", err)
	}
	records, err := idx.Records()
	if err != nil {
		return fmt.Errorf("read wiki index: %w", err)
	}
	fmt.Printf("wiki index: %d documents indexed (%d added, %d updated, %d removed)\n",
		len(records), len(result.Added), len(result.Updated), len(result.Removed))
	return nil
}

// wikiSearchHit is one ranked document match produced by runWikiSearch.
type wikiSearchHit struct {
	// Path is the absolute path of the matched document.
	Path string
	// Score is the cumulative term-match score; higher ranks first.
	Score int
}

// runWikiSearch ranks indexed documents against a query. It uses
// wiki.WikiIndex for the document set (the bounded-context index) and scores
// each document by counting query-term occurrences in its content.
func runWikiSearch(cmd *cobra.Command, args []string) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		return fmt.Errorf("wiki search: a query is required")
	}
	base, err := wikiResolveBase(cmd)
	if err != nil {
		return err
	}
	limit, _ := cmd.Flags().GetInt("limit")
	if limit <= 0 {
		limit = wikiSearchMaxResults
	}

	idx, err := wiki.NewWikiIndex(wikiIndexPathFor(base), base)
	if err != nil {
		return fmt.Errorf("construct wiki index: %w", err)
	}

	reindex, _ := cmd.Flags().GetBool("reindex")
	records, err := idx.Records()
	if err != nil {
		return fmt.Errorf("read wiki index: %w", err)
	}
	if reindex || len(records) == 0 {
		// An empty index would never return a hit; build it inline so a
		// first-run `ao wiki search` is still useful.
		if _, err := idx.Reindex(cmd.Context()); err != nil {
			return fmt.Errorf("reindex wiki corpus: %w", err)
		}
		records, err = idx.Records()
		if err != nil {
			return fmt.Errorf("read wiki index: %w", err)
		}
	}

	hits := rankWikiRecords(records, query)
	if len(hits) == 0 {
		fmt.Printf("wiki search: no matches for %q (%d documents indexed)\n", query, len(records))
		return nil
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	fmt.Printf("wiki search: %d match(es) for %q\n", len(hits), query)
	for i, hit := range hits {
		fmt.Printf("%2d. [score %d] %s\n", i+1, hit.Score, hit.Path)
	}
	return nil
}

// rankWikiRecords scores each indexed record against the query and returns
// the matching documents ordered by descending score. Scoring is a simple
// case-insensitive term-occurrence count over each document's content; ties
// break on path for deterministic output.
func rankWikiRecords(records []ports.WikiIndexRecord, query string) []wikiSearchHit {
	terms := wikiQueryTerms(query)
	if len(terms) == 0 {
		return nil
	}
	var hits []wikiSearchHit
	for _, rec := range records {
		data, err := os.ReadFile(rec.Path) //nolint:gosec // path is index-bounded
		if err != nil {
			continue
		}
		haystack := strings.ToLower(string(data) + " " + filepath.Base(rec.Path))
		score := 0
		for _, term := range terms {
			score += strings.Count(haystack, term)
		}
		if score > 0 {
			hits = append(hits, wikiSearchHit{Path: rec.Path, Score: score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Path < hits[j].Path
	})
	return hits
}

// wikiQueryTerms splits a query into lowercased, de-duplicated search terms.
func wikiQueryTerms(query string) []string {
	seen := make(map[string]struct{})
	var terms []string
	for _, raw := range strings.Fields(strings.ToLower(query)) {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		terms = append(terms, t)
	}
	return terms
}

// runWikiInject delegates context injection to the legacy ao inject command,
// preserving its behavior exactly. The wiki surface does not reimplement the
// inject ranker — it routes to the canonical implementation.
func runWikiInject(cmd *cobra.Command, args []string) error {
	injectCmd.SetContext(cmd.Context())
	return runInject(injectCmd, args)
}

// runWikiStage runs a single WikiPipeline stage against the resolved vault and
// prints its outcome. It is the shared body of the lint/query/promote
// subcommands.
func runWikiStage(cmd *cobra.Command, stage wiki.PipelineStage) error {
	vault, err := wikiResolveVault(cmd)
	if err != nil {
		return err
	}
	pipeline := wiki.NewWikiPipeline()
	outcome, err := pipeline.RunStage(cmd.Context(), vault, stage, 1)
	if err != nil {
		return fmt.Errorf("run wiki %s stage: %w", stage, err)
	}
	if outcome.Skipped {
		fmt.Printf("wiki %s: skipped (%s)\n", stage, outcome.SkipReason)
		return nil
	}
	fmt.Printf("wiki %s: complete (%d artifact(s))\n", stage, len(outcome.Artifacts))
	for _, art := range outcome.Artifacts {
		fmt.Printf("  - %s\n", art)
	}
	return nil
}

// runWikiLint runs the WikiPipeline LINT stage.
func runWikiLint(cmd *cobra.Command, _ []string) error {
	return runWikiStage(cmd, wiki.StageLint)
}

// runWikiQuery runs the WikiPipeline QUERY stage.
func runWikiQuery(cmd *cobra.Command, _ []string) error {
	return runWikiStage(cmd, wiki.StageQuery)
}

// runWikiPromote runs the WikiPipeline PROMOTE stage. PROMOTE has no promoter
// wired in the wiki package, so this reports the stage's not-configured skip.
func runWikiPromote(cmd *cobra.Command, _ []string) error {
	return runWikiStage(cmd, wiki.StagePromote)
}

// runWikiDoctor reports on the wiki corpus directory and the persistent index.
func runWikiDoctor(cmd *cobra.Command, _ []string) error {
	base, err := wikiResolveBase(cmd)
	if err != nil {
		return err
	}
	agentsDir := wiki.AgentsDirIn(base)
	indexPath := wikiIndexPathFor(base)

	corpusState := "missing"
	if info, statErr := os.Stat(agentsDir); statErr == nil && info.IsDir() {
		corpusState = "present"
	}

	fmt.Println("wiki doctor:")
	fmt.Printf("  corpus root : %s (%s)\n", agentsDir, corpusState)
	fmt.Printf("  index path  : %s\n", indexPath)

	idx, err := wiki.NewWikiIndex(indexPath, base)
	if err != nil {
		return fmt.Errorf("construct wiki index: %w", err)
	}
	records, err := idx.Records()
	if err != nil {
		return fmt.Errorf("read wiki index: %w", err)
	}
	if len(records) == 0 {
		fmt.Println("  indexed docs: 0 (run `ao wiki index` to build the index)")
	} else {
		fmt.Printf("  indexed docs: %d\n", len(records))
	}
	return nil
}
