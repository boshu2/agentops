package main

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/taxonomy"
	"github.com/boshu2/agentops/cli/internal/types"
)

var (
	poolIngestDir string
)

type poolIngestResult struct {
	FilesScanned     int      `json:"files_scanned"`
	CandidatesFound  int      `json:"candidates_found"`
	Added            int      `json:"added"`
	SkippedExisting  int      `json:"skipped_existing"`
	SkippedPromoted  int      `json:"skipped_promoted,omitempty"`
	SkippedMalformed int      `json:"skipped_malformed"`
	Errors           int      `json:"errors"`
	AddedIDs         []string `json:"added_ids,omitempty"`
}

var poolIngestCmd = &cobra.Command{
	Use:   "ingest [<files-or-globs...>]",
	Short: "Ingest pending markdown learnings into the pool",
	Long: `Ingest pending learnings into the quality pool.

This command bridges LLM-authored markdown learnings (typically written to
.agents/knowledge/pending/) into .agents/pool/pending/ as scored candidates.

If no args are provided, it ingests *.md from --dir (default: .agents/knowledge/pending)
and also scans legacy manual captures in .agents/knowledge/*.md.

Examples:
  ao pool ingest
  ao pool ingest --dir .agents/knowledge/pending
  ao pool ingest .agents/knowledge/pending/*.md
  ao pool ingest --dry-run --json`,
	RunE: runPoolIngest,
}

func init() {
	poolCmd.AddCommand(poolIngestCmd)
	poolIngestCmd.Flags().StringVar(&poolIngestDir, "dir", filepath.Join(".agents", "knowledge", "pending"), "Directory to ingest from when no args are provided")
}

// ingestFileBlocks processes all learning blocks from one file, updating res.
// Returns true if any block had an add error (not skipped or malformed).
func ingestFileBlocks(p *pool.Pool, blocks []learningBlock, f string, fileDate time.Time, sessionHint string, res *poolIngestResult, promotedContent map[string]bool, auditSkips bool) bool {
	hadError := false
	for _, b := range blocks {
		cand, scoring, ok := buildCandidateFromLearningBlock(b, f, fileDate, sessionHint)
		if !ok {
			res.SkippedMalformed++
			continue
		}
		if promotedContent[pool.ContentHash(cand.Content)] {
			res.SkippedPromoted++
			if auditSkips && !GetDryRun() {
				reason := "dedup: " + duplicatePromotedContentReason
				if err := p.RecordSkip(cand.ID, reason, "pool-ingest"); err != nil {
					VerbosePrintf("Warning: record duplicate skip %s: %v\n", cand.ID, err)
				}
			}
			continue
		}
		// Idempotency: skip if already present in any pool directory.
		if _, gerr := p.Get(cand.ID); gerr == nil {
			res.SkippedExisting++
			continue
		}
		if GetDryRun() {
			res.Added++
			res.AddedIDs = append(res.AddedIDs, cand.ID)
			continue
		}
		if err := p.AddAt(cand, scoring, cand.ExtractedAt); err != nil {
			res.Errors++
			hadError = true
			VerbosePrintf("Warning: add %s: %v\n", cand.ID, err)
			continue
		}
		res.Added++
		res.AddedIDs = append(res.AddedIDs, cand.ID)
	}
	return hadError
}

// moveIngestedFiles moves successfully processed files to the processed directory.
func moveIngestedFiles(cwd string, processedFiles []string) {
	processedDir := filepath.Join(cwd, ".agents", "knowledge", "processed")
	if err := os.MkdirAll(processedDir, 0750); err != nil {
		VerbosePrintf("Warning: create processed dir: %v\n", err)
		return
	}
	for _, f := range processedFiles {
		dst := filepath.Join(processedDir, filepath.Base(f))
		if merr := os.Rename(f, dst); merr != nil {
			VerbosePrintf("Warning: move %s to processed: %v\n", filepath.Base(f), merr)
		}
	}
}

func runPoolIngest(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	p := pool.NewPool(cwd)

	files, err := resolveIngestFiles(cwd, poolIngestDir, args)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Println("No new files to ingest")
		return nil
	}

	res := poolIngestResult{FilesScanned: len(files)}
	promotedContent := loadPromotedContent(cwd)

	pool.IterateIngestFiles(files, pool.IngestOrchestratorOpts{
		IngestFile: func(path string, data []byte) bool {
			fileDate, sessionHint := parsePendingFileHeader(string(data), path)
			blocks := parseLearningBlocks(string(data))
			res.CandidatesFound += len(blocks)
			return ingestFileBlocks(p, blocks, path, fileDate, sessionHint, &res, promotedContent, true)
		},
		TrackProcessed: !GetDryRun(),
		MoveProcessed: func(processed []string) {
			moveIngestedFiles(cwd, processed)
		},
		OnReadError: func(path string, err error) {
			res.Errors++
			VerbosePrintf("Warning: read %s: %v\n", filepath.Base(path), err)
		},
	})

	return outputPoolIngestResult(res)
}

func outputPoolIngestResult(res poolIngestResult) error {
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	default:
		fmt.Printf("Ingested %d candidate(s) from %d file(s)\n", res.Added, res.FilesScanned)
		if res.SkippedExisting > 0 {
			fmt.Printf("Skipped (existing): %d\n", res.SkippedExisting)
		}
		if res.SkippedPromoted > 0 {
			fmt.Printf("Skipped (already promoted): %d\n", res.SkippedPromoted)
		}
		if res.SkippedMalformed > 0 {
			fmt.Printf("Skipped (malformed): %d\n", res.SkippedMalformed)
		}
		if res.Errors > 0 {
			fmt.Printf("Errors: %d\n", res.Errors)
		}
		return nil
	}
}

func resolveIngestFiles(cwd, defaultDir string, args []string) ([]string, error) {
	var patterns []string
	if len(args) == 0 {
		patterns = []string{
			filepath.Join(cwd, defaultDir, "*.md"),
			// Legacy /learn captures were written directly to .agents/knowledge/.
			filepath.Join(cwd, ".agents", "knowledge", "*.md"),
		}
	} else {
		for _, a := range args {
			// Allow relative paths
			if !filepath.IsAbs(a) {
				patterns = append(patterns, filepath.Join(cwd, a))
			} else {
				patterns = append(patterns, a)
			}
		}
	}

	var files []string
	seen := make(map[string]bool)
	for _, pat := range patterns {
		matches, err := filepath.Glob(pat)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pat, err)
		}
		for _, m := range matches {
			if seen[m] {
				continue
			}
			seen[m] = true
			files = append(files, m)
		}
	}
	return files, nil
}

// learningBlock is an alias for pool.LearningBlock to keep cmd-layer callers stable.
type learningBlock = pool.LearningBlock

var (
	reDateMD      = regexp.MustCompile(`(?m)^\*\*Date:?\*\*:?\s*(\d{4}-\d{2}-\d{2})\s*$`)
	reDateYAML    = regexp.MustCompile(`(?m)^date:\s*(\d{4}-\d{2}-\d{2})\s*$`)
	reFrontmatter = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)
	reSessionHint = regexp.MustCompile(`\bag-[a-z0-9]+\b`)
)

func parseLearningBlocks(md string) []learningBlock {
	return pool.ParseLearningBlocks(md)
}

// dateStrategy is a function that attempts to extract a date from its inputs.
type dateStrategy func(md, path string) (time.Time, bool)

// dateFromFrontmatter extracts a date from YAML frontmatter.
func dateFromFrontmatter(md, _ string) (time.Time, bool) {
	fm := reFrontmatter.FindStringSubmatch(md)
	if len(fm) != 2 {
		return time.Time{}, false
	}
	m := reDateYAML.FindStringSubmatch(fm[1])
	if len(m) != 2 {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", strings.TrimSpace(m[1]))
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// dateFromMarkdownField extracts a date from a **Date** markdown field.
func dateFromMarkdownField(md, _ string) (time.Time, bool) {
	m := reDateMD.FindStringSubmatch(md)
	if len(m) != 2 {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", strings.TrimSpace(m[1]))
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// dateFromFilenamePrefix extracts a YYYY-MM-DD date from the filename prefix.
func dateFromFilenamePrefix(_, path string) (time.Time, bool) {
	base := filepath.Base(path)
	if len(base) < 10 {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", base[:10])
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// dateFromFileMtime uses the file's modification time as a fallback.
func dateFromFileMtime(_, path string) (time.Time, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime().UTC(), true
}

// dateStrategies defines the ordered list of date extraction strategies.
var dateStrategies = []dateStrategy{
	dateFromFrontmatter,
	dateFromMarkdownField,
	dateFromFilenamePrefix,
	dateFromFileMtime,
}

func parsePendingFileHeader(md, path string) (fileDate time.Time, sessionHint string) {
	for _, strategy := range dateStrategies {
		if t, ok := strategy(md, path); ok {
			fileDate = t
			break
		}
	}
	if fileDate.IsZero() {
		fileDate = time.Now().UTC()
	}

	sessionHint = extractSessionHint(md, path)
	return fileDate, sessionHint
}

// extractSessionHint finds an ag-xxxx session ID in the first ~2KB of the content,
// falling back to the filename base.
func extractSessionHint(md, path string) string {
	head := md
	if len(head) > 2048 {
		head = head[:2048]
	}
	if m := reSessionHint.FindString(head); m != "" {
		return m
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func buildCandidateFromLearningBlock(b learningBlock, srcPath string, fileDate time.Time, sessionHint string) (types.Candidate, types.Scoring, bool) {
	if strings.TrimSpace(b.Title) == "" || strings.TrimSpace(b.Body) == "" {
		return types.Candidate{}, types.Scoring{}, false
	}
	// Reject stub learnings that slipped through with no real content.
	if strings.Contains(strings.ToLower(b.Body), "no significant learnings") {
		return types.Candidate{}, types.Scoring{}, false
	}

	// Stable ID: prefer (file base + learning ID). Otherwise fall back to a content hash.
	base := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
	learningID := cmp.Or(strings.ToLower(strings.TrimSpace(b.ID)), "noid")

	// Dedup the three components when they are equal or already contained.
	// The unconditional "pend-{base}-{sessionHint}-{learningID}" form produced
	// triple-ID filenames whenever a learning's source file, session hint, and
	// frontmatter id all degenerated to the same string — and the resulting id
	// re-amplified on each close-loop pass.
	parts := []string{base}
	if sessionHint != "" && sessionHint != base && !strings.Contains(base, sessionHint) {
		parts = append(parts, sessionHint)
	}
	if learningID != "" && learningID != "noid" {
		joined := strings.Join(parts, "-")
		if !strings.Contains(joined, learningID) {
			parts = append(parts, learningID)
		}
	}
	id := slugify("pend-" + strings.Join(parts, "-"))
	if len(id) > 120 {
		// Keep a stable prefix, add a short hash to preserve uniqueness.
		h := sha256.Sum256([]byte(b.Body))
		id = slugify(id[:90] + "-" + hex.EncodeToString(h[:4]))
	}

	candType := inferKnowledgeType(b)
	confDim := confidenceToScore(b.Confidence)
	rubric := computeRubricScores(b.Body, confDim)
	weighted := rubricWeightedSum(rubric, taxonomy.DefaultRubricWeights)
	raw := (taxonomy.GetBaseScore(candType) + weighted) / 2.0

	// Pending learnings already reflect some human/LLM filtering (they were written intentionally),
	// so bias score upwards based on the declared confidence to reduce false "bronze" assignments.
	switch strings.ToLower(strings.TrimSpace(b.Confidence)) {
	case "high":
		raw += 0.15
	case "medium":
		raw += 0.07
	}

	if raw > 1.0 {
		raw = 1.0
	}
	if raw < 0.0 {
		raw = 0.0
	}

	tier := taxonomy.AssignTier(raw, taxonomy.DefaultTierConfigs)
	gateRequired := taxonomy.RequiresHumanGate(tier, taxonomy.DefaultTierConfigs)

	cand := types.Candidate{
		ID:          id,
		Type:        candType,
		Content:     strings.TrimSpace(b.Body),
		Source:      types.Source{TranscriptPath: srcPath, Timestamp: fileDate, SessionID: sessionHint, MessageIndex: 0},
		RawScore:    raw,
		Tier:        tier,
		ExtractedAt: fileDate,
		Metadata: map[string]any{
			"pending_category":   b.Category,
			"pending_confidence": b.Confidence,
			"pending_title":      b.Title,
		},
		IsCurrent:    true,
		ExpiryStatus: types.ExpiryStatusActive,
		Utility:      types.InitialUtility,
		Maturity:     types.MaturityProvisional,
		Confidence:   taxonomy.GetConfidence(tier, taxonomy.DefaultTierConfigs),
		LastDecayAt:  fileDate,
		DecayCount:   0,
		HelpfulCount: 0,
		HarmfulCount: 0,
		RewardCount:  0,
		LastReward:   0,
		LastRewardAt: time.Time{},
		ValidUntil:   "",
		Location:     "",
		LocationPath: "",
	}

	scoring := types.Scoring{
		RawScore:       raw,
		TierAssignment: tier,
		Rubric:         rubric,
		GateRequired:   gateRequired,
		ScoredAt:       time.Now(),
	}

	return cand, scoring, true
}

// inferKnowledgeType classifies a learning block by its category and content signals.
func inferKnowledgeType(b learningBlock) types.KnowledgeType {
	cat := strings.ToLower(strings.TrimSpace(b.Category))
	lower := strings.ToLower(b.Body)

	switch cat {
	case "decision", "pattern", "architectural-decision", "convention":
		return types.KnowledgeTypeDecision
	case "failure", "anti-pattern", "antipattern", "postmortem":
		return types.KnowledgeTypeFailure
	case "solution", "fix", "workaround":
		return types.KnowledgeTypeSolution
	case "reference", "doc", "documentation":
		return types.KnowledgeTypeReference
	}

	decisionSignals := 0
	for _, kw := range []string{"always ", "never ", "prefer ", "convention", "pattern:", "decision:", "we decided", "architectural"} {
		if strings.Contains(lower, kw) {
			decisionSignals++
		}
	}
	if decisionSignals >= 2 {
		return types.KnowledgeTypeDecision
	}

	failureSignals := 0
	for _, kw := range []string{"failed", "broke", "regression", "root cause", "post-mortem", "anti-pattern"} {
		if strings.Contains(lower, kw) {
			failureSignals++
		}
	}
	if failureSignals >= 2 {
		return types.KnowledgeTypeFailure
	}

	return types.KnowledgeTypeLearning
}

func confidenceToScore(s string) float64            { return pool.ConfidenceToScore(s) }
func isSlugAlphanumeric(r rune) bool                { return pool.IsSlugAlphanumeric(r) }
func computeSpecificityScore(b, l string) float64   { return pool.ComputeSpecificityScore(b, l) }
func computeActionabilityScore(body string) float64 { return pool.ComputeActionabilityScore(body) }
func computeNoveltyScore(body string) float64       { return pool.ComputeNoveltyScore(body) }
func computeContextScore(lower string) float64      { return pool.ComputeContextScore(lower) }

func computeRubricScores(body string, confidence float64) types.RubricScores {
	lower := strings.ToLower(body)
	return types.RubricScores{
		Specificity:   pool.ComputeSpecificityScore(body, lower),
		Actionability: pool.ComputeActionabilityScore(body),
		Novelty:       pool.ComputeNoveltyScore(body),
		Context:       pool.ComputeContextScore(lower),
		Confidence:    confidence,
	}
}

func rubricWeightedSum(r types.RubricScores, w taxonomy.RubricWeights) float64 {
	return r.Specificity*w.Specificity +
		r.Actionability*w.Actionability +
		r.Novelty*w.Novelty +
		r.Context*w.Context +
		r.Confidence*w.Confidence
}

func slugify(s string) string                           { return pool.Slugify(s) }
func parseYAMLFrontmatter(raw string) map[string]string { return pool.ParseYAMLFrontmatter(raw) }
func extractFirstHeadingText(body string) string        { return pool.ExtractFirstHeadingText(body) }
func parseLegacyFrontmatterLearning(md string) (learningBlock, bool) {
	return pool.ParseLegacyFrontmatterLearning(md)
}
