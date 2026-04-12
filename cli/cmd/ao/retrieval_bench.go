package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/bench"
)

var (
	benchCorpus string
	benchJSON   bool
	benchK      int
	benchLive   bool
	benchGlobal bool
)

// benchCase defines a benchmark query and its labeled evaluation metadata.
type benchCase struct {
	Query           string   `json:"query"`
	Split           string   `json:"split"`
	Labels          []string `json:"labels,omitempty"`
	Expected        []string `json:"expected"` // IDs expected in top K
	BestID          string   `json:"best_id"`  // single best expected result for MRR
	ExpectedSection string   `json:"expected_section,omitempty"`
}

// benchResult holds the result of a single query benchmark.
type benchResult struct {
	Query           string   `json:"query"`
	Split           string   `json:"split,omitempty"`
	Labels          []string `json:"labels,omitempty"`
	ExpectedSection string   `json:"expected_section,omitempty"`
	PAtK            float64  `json:"precision_at_k"`
	MRR             float64  `json:"mrr"`
	SectionMRR      float64  `json:"section_mrr,omitempty"`
	SectionHit      bool     `json:"section_hit,omitempty"`
	Pass            bool     `json:"pass"`
	ResultIDs       []string `json:"result_ids"`
	SectionIDs      []string `json:"section_ids,omitempty"`
}

// benchSplitSummary holds aggregate results for a benchmark split.
type benchSplitSummary struct {
	Cases         int           `json:"cases"`
	SectionCases  int           `json:"section_cases,omitempty"`
	AvgPAtK       float64       `json:"avg_precision_at_k"`
	AvgMRR        float64       `json:"avg_mrr"`
	AvgSectionMRR float64       `json:"avg_section_mrr,omitempty"`
	Results       []benchResult `json:"results,omitempty"`
}

// benchReport holds the overall benchmark report.
type benchReport struct {
	Queries    int                          `json:"queries"`
	K          int                          `json:"k"`
	AvgPAtK    float64                      `json:"avg_precision_at_k"`
	AvgMRR     float64                      `json:"avg_mrr"`
	TargetPAtK float64                      `json:"target_precision_at_k"`
	TargetMRR  float64                      `json:"target_mrr"`
	Splits     map[string]benchSplitSummary `json:"splits,omitempty"`
	Results    []benchResult                `json:"results"`
}

type benchManifest struct {
	Cases []benchCase `json:"cases"`
}

type sectionCandidate struct {
	ID      string
	FileID  string
	Heading string
	Score   float64
}

// liveQueryResult holds the result of a single query against the live corpus.
type liveQueryResult struct {
	Query     string   `json:"query"`
	Count     int      `json:"count"`
	TopIDs    []string `json:"top_ids"`
	MinScore  float64  `json:"min_score"`
	MaxScore  float64  `json:"max_score"`
	MeanScore float64  `json:"mean_score"`
}

// liveReport holds results from benchmarking against the real .agents/learnings/ directory.
type liveReport struct {
	Mode            string            `json:"mode"`
	TotalLearnings  int               `json:"total_learnings"`
	Queries         int               `json:"queries"`
	K               int               `json:"k"`
	QueriesWithHits int               `json:"queries_with_hits"`
	Coverage        float64           `json:"coverage"`
	Results         []liveQueryResult `json:"results"`
}

// liveQueries are broad queries that exercise real-world retrieval patterns.
var liveQueries = []string{
	"CI pipeline",
	"session intelligence",
	"hook authoring",
	"flywheel",
	"testing",
	"refactor",
	"security",
	"performance",
	"debugging",
	"architecture",
}

var benchManifestFilenames = []string{
	"benchmark.json",
	"retrieval-bench.json",
	"bench.json",
}

func resolveRetrievalBenchCorpus(cwd, provided string) (string, error) {
	candidates := []string{}
	if strings.TrimSpace(provided) != "" {
		candidates = append(candidates, provided)
	} else {
		candidates = append(candidates,
			filepath.Join(cwd, "testdata", "retrieval-bench"),
			filepath.Join(cwd, "cli", "cmd", "ao", "testdata", "retrieval-bench"),
			filepath.Join(cwd, "cmd", "ao", "testdata", "retrieval-bench"),
		)
		if exe, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", "cmd", "ao", "testdata", "retrieval-bench"))
		}
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
	}

	if strings.TrimSpace(provided) != "" {
		return "", fmt.Errorf("benchmark corpus not found at %s", provided)
	}
	return "", fmt.Errorf("benchmark corpus not found; specify --corpus path")
}

func buildBenchReport(cwd, corpusDir string, k int) (benchReport, error) {
	queries, tmpDir, cleanup, err := prepareBenchWorkspace(corpusDir)
	if err != nil {
		return benchReport{}, err
	}
	defer cleanup()

	report := newBenchReport(len(queries), k)

	var sumPAtK, sumMRR float64
	for _, q := range queries {
		result, pAtK, mrr, err := buildBenchCaseResult(tmpDir, corpusDir, q, k, report.TargetPAtK, report.TargetMRR)
		if err != nil {
			return benchReport{}, err
		}

		report.Results = append(report.Results, result)
		sumPAtK += pAtK
		sumMRR += mrr
		addBenchSplitResult(report.Splits, result)
	}

	finalizeBenchReport(&report, sumPAtK, sumMRR)
	return report, nil
}

func prepareBenchWorkspace(corpusDir string) ([]benchCase, string, func(), error) {
	queries, err := loadBenchCases(corpusDir)
	if err != nil {
		return nil, "", func() {}, err
	}
	if len(queries) == 0 {
		queries = defaultBenchQueries()
	}

	tmpDir, err := os.MkdirTemp("", "retrieval-bench-*")
	if err != nil {
		return nil, "", func() {}, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	learningsDir := filepath.Join(tmpDir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		cleanup()
		return nil, "", func() {}, fmt.Errorf("creating learnings dir: %w", err)
	}
	if err := copyBenchCorpusLearnings(corpusDir, learningsDir); err != nil {
		cleanup()
		return nil, "", func() {}, err
	}
	return queries, tmpDir, cleanup, nil
}

func newBenchReport(queryCount, k int) benchReport {
	return benchReport{
		Queries:    queryCount,
		K:          k,
		TargetPAtK: 0.67,
		TargetMRR:  0.50,
		Splits:     make(map[string]benchSplitSummary),
	}
}

func buildBenchCaseResult(tmpDir, corpusDir string, q benchCase, k int, targetPAtK, targetMRR float64) (benchResult, float64, float64, error) {
	results, err := collectLearnings(tmpDir, q.Query, 10, "", 0)
	if err != nil {
		return benchResult{}, 0, 0, fmt.Errorf("collectLearnings(%q): %w", q.Query, err)
	}

	pAtK, mrr := scoreBenchResults(results, benchExpectedSet(q.Expected), q.BestID, benchCaseK(k, q.Expected))
	sectionCandidates, sectionMRR, sectionHit := scoreBenchSections(corpusDir, q.Query, q.ExpectedSection)

	result := benchResult{
		Query:           q.Query,
		Split:           normalizeBenchSplit(q.Split),
		Labels:          append([]string(nil), q.Labels...),
		ExpectedSection: q.ExpectedSection,
		PAtK:            pAtK,
		MRR:             mrr,
		SectionMRR:      sectionMRR,
		SectionHit:      sectionHit,
		Pass:            benchCasePasses(q, pAtK, mrr, sectionMRR, targetPAtK, targetMRR),
		ResultIDs:       benchLearningIDs(results),
		SectionIDs:      benchSectionIDs(sectionCandidates),
	}
	return result, pAtK, mrr, nil
}

func benchExpectedSet(expected []string) map[string]bool {
	expectedSet := make(map[string]bool, len(expected))
	for _, id := range expected {
		expectedSet[id] = true
	}
	return expectedSet
}

func benchCaseK(k int, expected []string) int {
	if len(expected) < k {
		return len(expected)
	}
	return k
}

func benchLearningIDs(results []learning) []string {
	ids := make([]string, 0, len(results))
	for _, r := range results {
		ids = append(ids, r.ID)
	}
	return ids
}

func benchSectionIDs(sectionCandidates []sectionCandidate) []string {
	sectionIDs := make([]string, 0, len(sectionCandidates))
	for _, s := range sectionCandidates {
		sectionIDs = append(sectionIDs, s.ID)
	}
	return sectionIDs
}

func benchCasePasses(q benchCase, pAtK, mrr, sectionMRR, targetPAtK, targetMRR float64) bool {
	pass := pAtK >= targetPAtK && mrr >= targetMRR
	if q.ExpectedSection != "" {
		return pass && sectionMRR > 0
	}
	return pass
}

func addBenchSplitResult(splits map[string]benchSplitSummary, result benchResult) {
	splitName := result.Split
	if splitName == "" {
		splitName = "holdout"
	}
	summary := splits[splitName]
	summary.Cases++
	summary.Results = append(summary.Results, result)
	summary.AvgPAtK += result.PAtK
	summary.AvgMRR += result.MRR
	if result.ExpectedSection != "" {
		summary.SectionCases++
		summary.AvgSectionMRR += result.SectionMRR
	}
	splits[splitName] = summary
}

func finalizeBenchReport(report *benchReport, sumPAtK, sumMRR float64) {
	if report.Queries > 0 {
		report.AvgPAtK = sumPAtK / float64(report.Queries)
		report.AvgMRR = sumMRR / float64(report.Queries)
	}

	for split, summary := range report.Splits {
		if summary.Cases > 0 {
			summary.AvgPAtK /= float64(summary.Cases)
			summary.AvgMRR /= float64(summary.Cases)
		}
		if summary.SectionCases > 0 {
			summary.AvgSectionMRR /= float64(summary.SectionCases)
		}
		report.Splits[split] = summary
	}

	sort.Slice(report.Results, func(i, j int) bool {
		if report.Results[i].Split != report.Results[j].Split {
			return report.Results[i].Split < report.Results[j].Split
		}
		return report.Results[i].Query < report.Results[j].Query
	})
}

func copyBenchCorpusLearnings(corpusDir, learningsDir string) error {
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		return fmt.Errorf("reading corpus: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(corpusDir, e.Name()))
		if err != nil {
			return fmt.Errorf("reading %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(learningsDir, e.Name()), data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", e.Name(), err)
		}
	}
	return nil
}

func loadBenchCases(corpusDir string) ([]benchCase, error) {
	for _, name := range benchManifestFilenames {
		path := filepath.Join(corpusDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read benchmark manifest %s: %w", path, err)
		}

		var manifest benchManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("parse benchmark manifest %s: %w", path, err)
		}
		for i := range manifest.Cases {
			manifest.Cases[i].Split = normalizeBenchSplit(manifest.Cases[i].Split)
			if manifest.Cases[i].Split == "" {
				manifest.Cases[i].Split = "holdout"
			}
			if manifest.Cases[i].BestID == "" && len(manifest.Cases[i].Expected) > 0 {
				manifest.Cases[i].BestID = manifest.Cases[i].Expected[0]
			}
		}
		return manifest.Cases, nil
	}
	return nil, nil
}

func normalizeBenchSplit(split string) string { return bench.NormalizeSplit(split) }

func scoreBenchResults(results []learning, expectedSet map[string]bool, bestID string, k int) (float64, float64) {
	ids := make([]string, len(results))
	for i, r := range results {
		ids[i] = r.ID
	}
	return bench.ScoreResults(ids, expectedSet, bestID, k)
}

func scoreBenchSections(corpusDir, query, expectedSection string) ([]sectionCandidate, float64, bool) {
	if expectedSection == "" {
		return nil, 0, false
	}
	files, err := os.ReadDir(corpusDir)
	if err != nil {
		return nil, 0, false
	}

	tokens := queryTokens(strings.ToLower(query))
	expected := normalizeBenchSection(expectedSection)
	var candidates []sectionCandidate
	for _, entry := range files {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(corpusDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		sections := splitMarkdownSections(stripBenchFrontMatter(string(data)))
		for _, section := range sections {
			heading := benchSectionHeading(section)
			if heading == "" {
				continue
			}
			score := benchSectionScore(tokens, section)
			candidates = append(candidates, sectionCandidate{
				ID:      fmt.Sprintf("%s#%s", entry.Name(), normalizeBenchSection(heading)),
				FileID:  entry.Name(),
				Heading: heading,
				Score:   score,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		if candidates[i].FileID != candidates[j].FileID {
			return candidates[i].FileID < candidates[j].FileID
		}
		return candidates[i].Heading < candidates[j].Heading
	})

	for i, candidate := range candidates {
		if strings.HasSuffix(candidate.ID, "#"+expected) {
			return candidates, 1.0 / float64(i+1), true
		}
	}
	return candidates, 0, false
}

func benchSectionScore(tokens []string, section string) float64 {
	if len(tokens) == 0 {
		return 1
	}
	heading := benchSectionHeading(section)
	text := strings.ToLower(section)
	score := matchRatio(tokens, heading, text, text)
	if score == 0 && heading != "" {
		score = matchRatio(tokens, heading, "", "")
	}
	return score
}

func benchSectionHeading(section string) string   { return bench.SectionHeading(section) }
func normalizeBenchSection(section string) string { return bench.NormalizeSection(section) }
func stripBenchFrontMatter(content string) string { return bench.StripFrontMatter(content) }

// runLiveBench benchmarks against the actual .agents/learnings/ directory.
// When global is true, benchmarks against ~/.agents/learnings/ (cross-rig aggregated store).
func buildLiveReport(cwd, globalDir, mode string, k int) (liveReport, error) {
	allLearnings, err := collectLearnings(cwd, "", 1000, globalDir, 1.0)
	if err != nil {
		return liveReport{}, fmt.Errorf("collecting all learnings: %w", err)
	}
	report := liveReport{
		Mode:           mode,
		TotalLearnings: len(allLearnings),
		Queries:        len(liveQueries),
		K:              k,
	}

	for _, q := range liveQueries {
		results, err := collectLearnings(cwd, q, k*3, globalDir, 1.0)
		if err != nil {
			return liveReport{}, fmt.Errorf("collectLearnings(%q): %w", q, err)
		}

		lr := liveQueryResult{
			Query: q,
			Count: len(results),
		}

		if len(results) > 0 {
			topN := k
			if topN > len(results) {
				topN = len(results)
			}
			lr.TopIDs = make([]string, 0, topN)
			for _, r := range results[:topN] {
				lr.TopIDs = append(lr.TopIDs, r.ID)
			}
			report.QueriesWithHits++

			lr.MinScore = results[len(results)-1].CompositeScore
			lr.MaxScore = results[0].CompositeScore
			var sum float64
			for _, r := range results {
				sum += r.CompositeScore
			}
			lr.MeanScore = sum / float64(len(results))
		}

		report.Results = append(report.Results, lr)
	}
	if report.Queries > 0 {
		report.Coverage = float64(report.QueriesWithHits) / float64(report.Queries)
	}
	return report, nil
}

func runLiveBench(k int, asJSON, global bool, corpusDir string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	globalDir := ""
	mode := "live-local"
	storeLabel := ".agents/learnings/"
	modeLabel := "Live (local)"
	benchCwd := cwd
	if corpusDir != "" {
		if _, err := os.Stat(corpusDir); err != nil {
			return fmt.Errorf("live benchmark corpus not found at %s", corpusDir)
		}
		tmpDir, err := os.MkdirTemp("", "retrieval-live-*")
		if err != nil {
			return fmt.Errorf("creating temp dir: %w", err)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()
		benchCwd = tmpDir
		globalDir = corpusDir
		mode = "live-corpus"
		storeLabel = corpusDir
		modeLabel = "Live (fixture)"
	} else if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolving home directory: %w", err)
		}
		globalDir = filepath.Join(home, ".agents", "learnings")
		if _, err := os.Stat(globalDir); err != nil {
			return fmt.Errorf("global knowledge store not found at %s — run 'ao harvest' first", globalDir)
		}
		mode = "live-global"
		storeLabel = "~/.agents/learnings/ (cross-rig)"
		modeLabel = "Live (global)"
	}

	report, err := buildLiveReport(benchCwd, globalDir, mode, k)
	if err != nil {
		return err
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	fmt.Printf("Retrieval Quality Report (%s)\n", modeLabel)
	fmt.Println("================================")
	fmt.Printf("Corpus:      %d learnings in %s\n", report.TotalLearnings, storeLabel)
	fmt.Printf("Queries:     %d\n", report.Queries)
	fmt.Printf("K:           %d\n", k)
	fmt.Printf("Coverage:    %.0f%% (%d/%d queries with hits)\n", report.Coverage*100, report.QueriesWithHits, report.Queries)
	fmt.Println()

	if report.TotalLearnings == 0 {
		fmt.Println("No learnings found. Run /retro or /post-mortem to populate the knowledge base.")
		return nil
	}

	fmt.Println("Per-query breakdown:")
	for _, r := range report.Results {
		if r.Count == 0 {
			fmt.Printf("  %-25s  hits=0  (no matches)\n", fmt.Sprintf("%q", r.Query))
		} else {
			fmt.Printf("  %-25s  hits=%-3d  score=[%.2f, %.2f]  mean=%.2f  top=%v\n",
				fmt.Sprintf("%q", r.Query), r.Count, r.MinScore, r.MaxScore, r.MeanScore, r.TopIDs)
		}
	}
	return nil
}

// defaultBenchQueries returns the built-in benchmark query set.
func defaultBenchQueries() []benchCase {
	return []benchCase{
		{Query: "CI pipeline", Split: "holdout", Labels: []string{"ci", "pipeline"}, Expected: []string{"ci-1.md", "ci-2.md", "ci-3.md"}, BestID: "ci-1.md"},
		{Query: "session intelligence", Split: "holdout", Labels: []string{"session-intelligence"}, Expected: []string{"si-1.md", "si-2.md", "si-3.md"}, BestID: "si-1.md"},
		{Query: "hook authoring", Split: "holdout", Labels: []string{"hooks"}, Expected: []string{"hook-1.md", "hook-2.md", "hook-3.md"}, BestID: "hook-1.md"},
		{Query: "database", Split: "holdout", Labels: []string{"database"}, Expected: []string{"db-1.md", "db-2.md", "db-3.md"}, BestID: "db-1.md"},
		{Query: "swarm", Split: "holdout", Labels: []string{"parallel-execution"}, Expected: []string{"swarm-1.md", "swarm-2.md"}, BestID: "swarm-1.md"},
	}
}

var retrievalBenchCmd = &cobra.Command{
	Use:   "retrieval-bench",
	Short: "Run retrieval quality benchmarks",
	Long: `Measure Precision@K and MRR against a curated corpus of learning artifacts.

Determinism: retrieval-bench --live --json is deterministic by construction.
The live query set is a hardcoded slice (liveQueries) in this file; the
corpus is either the real .agents/learnings/ directory, a fixture passed
via --corpus, or ~/.agents/learnings/ when --global is set. The underlying
retrieval pipeline (collectLearnings → rankLearnings) uses a stable
slices.SortFunc by CompositeScore and performs no random sampling — the
internal/bench package is a set of pure string/math helpers with no RNG.
For a fixed corpus state, precision@k, MRR, and top_ids are stable across
runs.

Dream's nightly compounder (ao overnight) relies on this contract in its
MEASURE stage to detect plateau deltas between runs. No --seed or
--eval-set flag is provided because the bench is already eval-set based
and deterministic; adding one would be dead code. If a future change
introduces randomness anywhere in the --live path (cli/internal/bench or
the retrieval engine called by collectLearnings), it is a contract
violation and must be reverted or reseeded to preserve plateau
detection.`,
	GroupID: "knowledge",
	RunE: func(cmd *cobra.Command, args []string) error {
		if benchLive {
			return runLiveBench(benchK, benchJSON, benchGlobal, benchCorpus)
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		corpusDir, err := resolveRetrievalBenchCorpus(cwd, benchCorpus)
		if err != nil {
			return err
		}
		report, err := buildBenchReport(corpusDir, corpusDir, benchK)
		if err != nil {
			return err
		}

		if benchJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}

		// Human-readable output
		fmt.Println("Retrieval Quality Report")
		fmt.Println("========================")
		fmt.Printf("Queries:     %d\n", report.Queries)
		fmt.Printf("Precision@%d: %.2f (target: %.2f)\n", benchK, report.AvgPAtK, report.TargetPAtK)
		fmt.Printf("MRR:         %.2f (target: %.2f)\n", report.AvgMRR, report.TargetMRR)
		if len(report.Splits) > 0 {
			fmt.Println("Splits:")
			splitNames := make([]string, 0, len(report.Splits))
			for split := range report.Splits {
				splitNames = append(splitNames, split)
			}
			sort.Strings(splitNames)
			for _, split := range splitNames {
				summary := report.Splits[split]
				line := fmt.Sprintf("  %-10s cases=%d  P@%d=%.2f  MRR=%.2f", split, summary.Cases, benchK, summary.AvgPAtK, summary.AvgMRR)
				if summary.SectionCases > 0 {
					line += fmt.Sprintf("  section-MRR=%.2f", summary.AvgSectionMRR)
				}
				fmt.Println(line)
			}
		}
		fmt.Println()
		fmt.Println("Per-query breakdown:")
		for _, r := range report.Results {
			status := "PASS"
			if !r.Pass {
				status = "WARN (below target)"
			}
			line := fmt.Sprintf("  %-30s P@%d=%.2f  MRR=%.2f", fmt.Sprintf("%q", r.Query), benchK, r.PAtK, r.MRR)
			if r.ExpectedSection != "" {
				line += fmt.Sprintf("  section-MRR=%.2f", r.SectionMRR)
			}
			line += fmt.Sprintf("  %s", status)
			fmt.Println(line)
		}
		return nil
	},
}

func init() {
	retrievalBenchCmd.GroupID = "knowledge"
	rootCmd.AddCommand(retrievalBenchCmd)
	retrievalBenchCmd.Flags().StringVar(&benchCorpus, "corpus", "", "Path to benchmark corpus directory")
	retrievalBenchCmd.Flags().BoolVar(&benchJSON, "json", false, "JSON output")
	retrievalBenchCmd.Flags().IntVar(&benchK, "k", 3, "K for Precision@K")
	retrievalBenchCmd.Flags().BoolVar(&benchLive, "live", false, "Benchmark against real .agents/learnings/ instead of synthetic corpus")
	retrievalBenchCmd.Flags().BoolVar(&benchGlobal, "global", false, "Include ~/.agents/learnings/ (cross-rig aggregated store, requires --live)")
}
