package main

import (
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
  ao pool ingest --dry-run -o json`,
	RunE: runPoolIngest,
}

func init() {
	poolCmd.AddCommand(poolIngestCmd)
	poolIngestCmd.Flags().StringVar(&poolIngestDir, "dir", filepath.Join(".agents", "knowledge", "pending"), "Directory to ingest from when no args are provided")
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

	// Track files that were successfully processed (no read/add errors).
	var processedFiles []string

	for _, f := range files {
		data, rerr := os.ReadFile(f)
		if rerr != nil {
			res.Errors++
			VerbosePrintf("Warning: read %s: %v\n", filepath.Base(f), rerr)
			continue
		}

		fileDate, sessionHint := parsePendingFileHeader(string(data), f)
		blocks := parseLearningBlocks(string(data))
		res.CandidatesFound += len(blocks)

		fileHadError := false
		for _, b := range blocks {
			cand, scoring, ok := buildCandidateFromLearningBlock(b, f, fileDate, sessionHint)
			if !ok {
				res.SkippedMalformed++
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
				fileHadError = true
				VerbosePrintf("Warning: add %s: %v\n", cand.ID, err)
				continue
			}
			res.Added++
			res.AddedIDs = append(res.AddedIDs, cand.ID)
		}

		if !fileHadError && !GetDryRun() {
			processedFiles = append(processedFiles, f)
		}
	}

	// Move successfully processed files to .agents/knowledge/processed/.
	if len(processedFiles) > 0 {
		processedDir := filepath.Join(cwd, ".agents", "knowledge", "processed")
		if err := os.MkdirAll(processedDir, 0755); err != nil {
			VerbosePrintf("Warning: create processed dir: %v\n", err)
		} else {
			for _, f := range processedFiles {
				dst := filepath.Join(processedDir, filepath.Base(f))
				if merr := os.Rename(f, dst); merr != nil {
					VerbosePrintf("Warning: move %s to processed: %v\n", filepath.Base(f), merr)
				}
			}
		}
	}

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

type learningBlock struct {
	Title      string
	ID         string
	Category   string
	Confidence string
	Body       string
}

var (
	reLearningHeader = regexp.MustCompile(`(?m)^# Learning:\s*(.+)\s*$`)
	// Support both "**ID**: X" and "**ID:** X" (colon outside vs inside bold).
	reIDLine         = regexp.MustCompile(`(?m)^\*\*ID:?\*\*:?\s*(.+)\s*$`)
	reCategoryLine   = regexp.MustCompile(`(?m)^\*\*Category:?\*\*:?\s*(.+)\s*$`)
	reConfidenceLine = regexp.MustCompile(`(?m)^\*\*Confidence:?\*\*:?\s*(.+)\s*$`)
	reFrontmatter    = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)
	reDateMD         = regexp.MustCompile(`(?m)^\*\*Date:?\*\*:?\s*(\d{4}-\d{2}-\d{2})\s*$`)
	reDateYAML       = regexp.MustCompile(`(?m)^date:\s*(\d{4}-\d{2}-\d{2})\s*$`)
	reSessionHint    = regexp.MustCompile(`\bag-[a-z0-9]+\b`)
)

func parseLearningBlocks(md string) []learningBlock {
	locs := reLearningHeader.FindAllStringSubmatchIndex(md, -1)
	if len(locs) == 0 {
		if legacy, ok := parseLegacyFrontmatterLearning(md); ok {
			return []learningBlock{legacy}
		}
		return nil
	}

	var blocks []learningBlock
	for i, loc := range locs {
		// loc[0:2] is full match span, loc[2:4] is title group span.
		start := loc[0]
		end := len(md)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		title := strings.TrimSpace(md[loc[2]:loc[3]])
		body := strings.TrimSpace(md[start:end])

		b := learningBlock{
			Title: title,
			Body:  body,
		}

		if m := reIDLine.FindStringSubmatch(body); len(m) == 2 {
			b.ID = strings.TrimSpace(m[1])
		}
		if m := reCategoryLine.FindStringSubmatch(body); len(m) == 2 {
			b.Category = strings.TrimSpace(m[1])
		}
		if m := reConfidenceLine.FindStringSubmatch(body); len(m) == 2 {
			b.Confidence = strings.TrimSpace(m[1])
		}

		blocks = append(blocks, b)
	}
	return blocks
}

func parseLegacyFrontmatterLearning(md string) (learningBlock, bool) {
	fmMatch := reFrontmatter.FindStringSubmatchIndex(md)
	if len(fmMatch) < 4 {
		return learningBlock{}, false
	}

	fmRaw := md[fmMatch[2]:fmMatch[3]]
	body := strings.TrimSpace(md[fmMatch[1]:])
	if body == "" {
		return learningBlock{}, false
	}

	frontmatter := make(map[string]string)
	for _, line := range strings.Split(fmRaw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		frontmatter[key] = strings.Trim(val, `"'`)
	}

	// Legacy /learn files include type/source/date frontmatter. Require type to
	// avoid treating arbitrary markdown files as candidates.
	category := strings.TrimSpace(frontmatter["type"])
	if category == "" {
		return learningBlock{}, false
	}

	title := ""
	for _, line := range strings.Split(body, "\n") {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		l = strings.TrimPrefix(l, "#")
		l = strings.TrimSpace(l)
		if l != "" {
			title = l
			break
		}
	}
	if title == "" {
		return learningBlock{}, false
	}

	confidence := strings.TrimSpace(frontmatter["confidence"])
	if confidence == "" {
		confidence = "medium"
	}

	id := strings.TrimSpace(frontmatter["id"])
	if id == "" {
		id = "legacy"
	}

	return learningBlock{
		Title:      title,
		ID:         id,
		Category:   category,
		Confidence: confidence,
		Body:       body,
	}, true
}

func parsePendingFileHeader(md, path string) (fileDate time.Time, sessionHint string) {
	// 1) frontmatter date
	if fm := reFrontmatter.FindStringSubmatch(md); len(fm) == 2 {
		if m := reDateYAML.FindStringSubmatch(fm[1]); len(m) == 2 {
			if t, err := time.Parse("2006-01-02", strings.TrimSpace(m[1])); err == nil {
				fileDate = t.UTC()
			}
		}
	}

	// 2) markdown **Date**
	if fileDate.IsZero() {
		if m := reDateMD.FindStringSubmatch(md); len(m) == 2 {
			if t, err := time.Parse("2006-01-02", strings.TrimSpace(m[1])); err == nil {
				fileDate = t.UTC()
			}
		}
	}

	// 3) filename prefix YYYY-MM-DD
	if fileDate.IsZero() {
		base := filepath.Base(path)
		if len(base) >= 10 {
			if t, err := time.Parse("2006-01-02", base[:10]); err == nil {
				fileDate = t.UTC()
			}
		}
	}

	// Session hint: look for ag-xxxx in the first ~2KB (fast).
	head := md
	if len(head) > 2048 {
		head = head[:2048]
	}
	if m := reSessionHint.FindString(head); m != "" {
		sessionHint = m
	} else {
		// fallback: file base (safe slug)
		sessionHint = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	if fileDate.IsZero() {
		// best-effort fallback: use file mtime (UTC)
		if info, err := os.Stat(path); err == nil {
			fileDate = info.ModTime().UTC()
		} else {
			fileDate = time.Now().UTC()
		}
	}

	return fileDate, sessionHint
}

func buildCandidateFromLearningBlock(b learningBlock, srcPath string, fileDate time.Time, sessionHint string) (types.Candidate, types.Scoring, bool) {
	if strings.TrimSpace(b.Title) == "" || strings.TrimSpace(b.Body) == "" {
		return types.Candidate{}, types.Scoring{}, false
	}

	// Stable ID: prefer (file base + learning ID). Otherwise fall back to a content hash.
	base := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
	learningID := strings.ToLower(strings.TrimSpace(b.ID))
	if learningID == "" {
		learningID = "noid"
	}

	id := slugify(fmt.Sprintf("pend-%s-%s-%s", base, sessionHint, learningID))
	if len(id) > 120 {
		// Keep a stable prefix, add a short hash to preserve uniqueness.
		h := sha256.Sum256([]byte(b.Body))
		id = slugify(id[:90] + "-" + hex.EncodeToString(h[:4]))
	}

	confDim := confidenceToScore(b.Confidence)
	rubric := computeRubricScores(b.Body, confDim)
	weighted := rubricWeightedSum(rubric, taxonomy.DefaultRubricWeights)
	raw := (taxonomy.GetBaseScore(types.KnowledgeTypeLearning) + weighted) / 2.0

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
		Type:        types.KnowledgeTypeLearning,
		Content:     strings.TrimSpace(b.Body),
		Source:      types.Source{TranscriptPath: srcPath, Timestamp: fileDate, SessionID: sessionHint, MessageIndex: 0},
		RawScore:    raw,
		Tier:        tier,
		ExtractedAt: fileDate,
		Metadata: map[string]interface{}{
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

func confidenceToScore(s string) float64 {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high":
		return 0.9
	case "medium":
		return 0.7
	case "low":
		return 0.5
	default:
		return 0.6
	}
}

func computeRubricScores(body string, confidence float64) types.RubricScores {
	lower := strings.ToLower(body)

	spec := 0.4
	if strings.Contains(body, "`") || strings.Contains(body, "```") {
		spec += 0.2
	}
	if regexp.MustCompile(`\d`).MatchString(body) {
		spec += 0.2
	}
	if regexp.MustCompile(`\b[a-zA-Z0-9_./-]+\.(go|ts|js|py|sh|yaml|yml|json|md)\b`).MatchString(body) {
		spec += 0.2
	}
	if strings.Contains(lower, "line ") {
		spec += 0.1
	}
	if spec > 1.0 {
		spec = 1.0
	}

	act := 0.4
	if regexp.MustCompile(`(?m)^\s*[-*]\s+`).MatchString(body) {
		act += 0.2
	}
	if regexp.MustCompile(`(?i)\b(run|add|remove|use|ensure|check|grep|rg|fix|avoid|prefer|must|should)\b`).MatchString(body) {
		act += 0.2
	}
	if strings.Contains(body, "```") {
		act += 0.2
	}
	if act > 1.0 {
		act = 1.0
	}

	nov := 0.5
	if len(body) > 800 {
		nov += 0.1
	}
	if len(body) < 250 {
		nov -= 0.1
	}
	if nov > 1.0 {
		nov = 1.0
	}
	if nov < 0.0 {
		nov = 0.0
	}

	ctx := 0.5
	if strings.Contains(lower, "## source") || strings.Contains(lower, "**source**") {
		ctx += 0.2
	}
	if strings.Contains(lower, "## why it matters") {
		ctx += 0.1
	}
	if ctx > 1.0 {
		ctx = 1.0
	}

	return types.RubricScores{
		Specificity:   spec,
		Actionability: act,
		Novelty:       nov,
		Context:       ctx,
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

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_':
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		default:
			// collapse everything else to dash
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "cand"
	}
	return out
}
