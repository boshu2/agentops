package main

import (
	"bufio"
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/resolver"
	"github.com/boshu2/agentops/cli/internal/types"
)

// validPhases is the set of canonical RPI phase values for source_phase.
var validPhases = map[string]bool{
	"research": true, "plan": true, "implement": true, "validate": true,
}

// sanitizeSourcePhase returns the phase if valid, or empty string if not.
func sanitizeSourcePhase(phase string) string {
	p := strings.ToLower(strings.TrimSpace(phase))
	if validPhases[p] {
		return p
	}
	return ""
}

// collectLearnings finds recent learnings from .agents/learnings/ and optionally ~/.agents/learnings/.
// Implements MemRL Two-Phase retrieval: Phase A (similarity/freshness) + Phase B (utility-weighted)
// With CASS integration: applies confidence decay when --apply-decay is set.
// Global learnings receive a post-scoring weight penalty (globalWeight, default 0.8).
func collectLearnings(cwd, query string, limit int, globalDir string, globalWeight float64) ([]learning, error) {
	files, err := findLearningFiles(cwd)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	queryLower := strings.ToLower(query)
	learnings := make([]learning, 0, len(files))

	for _, file := range files {
		l, ok := processLearningFile(file, queryLower, now)
		if !ok {
			continue
		}
		learnings = append(learnings, l)
	}

	// Build set of local file paths for dedup against global
	localPaths := make(map[string]bool, len(files))
	for _, f := range files {
		if abs, err := filepath.Abs(f); err == nil {
			localPaths[abs] = true
		}
	}

	// Collect global learnings (cross-repo knowledge)
	if globalDir != "" {
		globalFiles := globLearningFiles(globalDir)
		for _, file := range globalFiles {
			// Skip if already found in local collection (prevents duplicates)
			if abs, err := filepath.Abs(file); err == nil && localPaths[abs] {
				continue
			}
			l, ok := processLearningFile(file, queryLower, now)
			if !ok {
				continue
			}
			l.Global = true
			learnings = append(learnings, l)
		}
	}

	if len(learnings) == 0 {
		return nil, nil
	}

	rankLearnings(learnings)

	// Apply global weight penalty post-scoring
	if globalWeight > 0 && globalWeight < 1.0 {
		for i := range learnings {
			if learnings[i].Global {
				learnings[i].CompositeScore *= globalWeight
			}
		}
		// Re-sort after weight adjustment
		slices.SortFunc(learnings, func(a, b learning) int {
			return cmp.Compare(b.CompositeScore, a.CompositeScore)
		})
	}

	if len(learnings) > limit {
		learnings = learnings[:limit]
	}
	return learnings, nil
}

// globLearningFiles returns *.md and *.jsonl files under dir, including
// namespaced subdirectories used by global cross-repo stores.
func globLearningFiles(dir string) []string {
	return walkKnowledgeFiles(dir, ".md", ".jsonl")
}

// findLearningFiles discovers .md and .jsonl files in the learnings directory.
// Delegates to the shared LearningResolver for consistent file discovery.
func findLearningFiles(cwd string) ([]string, error) {
	return resolver.NewFileResolver(cwd).DiscoverAll()
}

// processLearningFile parses, filters, and scores a single learning file.
// Returns the learning and true if it should be included, false otherwise.
func processLearningFile(file, queryLower string, now time.Time) (learning, bool) {
	l, err := parseLearningFile(file)
	if err != nil {
		return l, false
	}
	if l.Superseded {
		VerbosePrintf("Skipping superseded learning: %s\n", l.ID)
		return l, false
	}
	if queryLower != "" && !strings.Contains(strings.ToLower(l.Title+" "+l.Summary), queryLower) &&
		!strings.Contains(strings.ToLower(l.BodyText), queryLower) {
		return l, false
	}

	applyFreshnessScore(&l, file, now)

	if l.Utility == 0 {
		l.Utility = types.InitialUtility
	}
	if injectApplyDecay {
		l = applyConfidenceDecay(l, file, now)
	}

	// Hard quality gate: filter out low-maturity or low-utility learnings
	if !passesQualityGate(l) {
		VerbosePrintf("Quality gate filtered: %s (maturity=%s, utility=%.3f)\n", l.ID, l.Maturity, l.Utility)
		return l, false
	}

	// Soft penalty: penalize unsourced learnings that passed the hard gate
	if l.SourceBead == "" {
		l.Utility *= 0.3
	}

	if l.Stability == "experimental" {
		fmt.Fprintf(os.Stderr, "WARNING: Skill %q is marked experimental — verify outputs carefully\n", l.Title)
	}

	return l, true
}

// passesQualityGate returns true if a learning meets minimum injection standards.
// Requires maturity >= provisional (provisional, candidate, or established) AND utility > 0.3.
// Empty maturity defaults to provisional (legacy learnings lack metadata).
func passesQualityGate(l learning) bool {
	mat := types.Maturity(l.Maturity)
	if mat == "" {
		mat = types.MaturityProvisional
	}
	switch mat {
	case types.MaturityProvisional, types.MaturityCandidate, types.MaturityEstablished:
		// maturity OK
	default:
		// "draft" or unknown → fail gate
		return false
	}
	return l.Utility > 0.3
}

// applyFreshnessScore sets the freshness score on a learning based on file modification time.
func applyFreshnessScore(l *learning, file string, now time.Time) {
	info, statErr := os.Stat(file)
	if info == nil {
		if statErr != nil {
			VerbosePrintf("Warning: stat %s: %v\n", file, statErr)
		}
		l.FreshnessScore = 0.5
		return
	}
	ageWeeks := now.Sub(info.ModTime()).Hours() / (24 * 7)
	l.AgeWeeks = ageWeeks
	l.FreshnessScore = freshnessScore(ageWeeks)
}

// rankLearnings applies composite scoring and sorts by score descending.
func rankLearnings(learnings []learning) {
	items := make([]scorable, len(learnings))
	for i := range learnings {
		items[i] = &learnings[i]
	}
	applyCompositeScoringTo(items, types.DefaultLambda)

	slices.SortFunc(learnings, func(a, b learning) int {
		return cmp.Compare(b.CompositeScore, a.CompositeScore)
	})
}

// applyConfidenceDecay applies time-based confidence decay to a learning.
// Confidence decays at 10%/week for learnings that haven't received recent feedback.
// Formula: confidence *= exp(-weeks_since_last_feedback * ConfidenceDecayRate)
// Supports both JSONL and Markdown (.md with YAML frontmatter) learning files.
func applyConfidenceDecay(l learning, filePath string, now time.Time) learning {
	if strings.HasSuffix(filePath, ".jsonl") {
		return applyConfidenceDecayJSONL(l, filePath, now)
	}
	if strings.HasSuffix(filePath, ".md") {
		return applyConfidenceDecayMarkdown(l, filePath, now)
	}
	return l
}

// applyConfidenceDecayJSONL applies confidence decay to a JSONL learning file.
func applyConfidenceDecayJSONL(l learning, filePath string, now time.Time) learning {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return l
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return l
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &data); err != nil {
		return l
	}

	confidence := jsonFloat(data, "confidence", 0.5)
	lastInteraction := jsonTimeField(data, "last_decay_at", "last_reward_at")
	if lastInteraction.IsZero() {
		return l
	}

	weeksSinceInteraction := now.Sub(lastInteraction).Hours() / (24 * 7)
	if weeksSinceInteraction <= 0 {
		return l
	}

	newConfidence := computeDecayedConfidence(confidence, weeksSinceInteraction)
	VerbosePrintf("Applied confidence decay to %s: %.3f -> %.3f (%.1f weeks)\n",
		l.ID, confidence, newConfidence, weeksSinceInteraction)

	writeDecayFields(data, newConfidence, now)
	if newJSON, marshalErr := json.Marshal(data); marshalErr == nil {
		lines[0] = string(newJSON)
		if writeErr := atomicWriteFile(filePath, []byte(strings.Join(lines, "\n")), 0600); writeErr != nil {
			VerbosePrintf("Warning: failed to write decay for %s: %v\n", l.ID, writeErr)
		}
	}

	l.Utility *= newConfidence / confidence
	return l
}

// applyConfidenceDecayMarkdown applies confidence decay to a Markdown learning file
// with YAML frontmatter containing confidence and last_reward_at/last_decay_at fields.
// Single read/modify/atomic-write to eliminate race window with concurrent sessions.
func applyConfidenceDecayMarkdown(l learning, filePath string, now time.Time) learning {
	// Single read — all parsing and modification uses this content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return l
	}

	// Parse frontmatter fields from the already-read content
	fields := parseFrontmatterFromContent(string(content), "confidence", "last_decay_at", "last_reward_at")

	confidence := 0.5
	if c, parseErr := strconv.ParseFloat(fields["confidence"], 64); parseErr == nil && c > 0 {
		confidence = c
	}

	// Find most recent interaction time from either field
	var lastInteraction time.Time
	for _, key := range []string{"last_decay_at", "last_reward_at"} {
		if v := fields[key]; v != "" {
			if t, parseErr := time.Parse(time.RFC3339, v); parseErr == nil {
				if lastInteraction.IsZero() || t.After(lastInteraction) {
					lastInteraction = t
				}
			}
		}
	}
	if lastInteraction.IsZero() {
		return l
	}

	weeksSinceInteraction := now.Sub(lastInteraction).Hours() / (24 * 7)
	if weeksSinceInteraction <= 0 {
		return l
	}

	newConfidence := computeDecayedConfidence(confidence, weeksSinceInteraction)
	VerbosePrintf("Applied confidence decay to %s: %.3f -> %.3f (%.1f weeks)\n",
		l.ID, confidence, newConfidence, weeksSinceInteraction)

	// Modify frontmatter from the same content (no second read)
	lines := strings.Split(string(content), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return l
	}

	// Find frontmatter end
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}
	if endIdx < 0 {
		return l
	}

	fmLines := lines[1:endIdx]
	updatedFM := updateFrontMatterFields(fmLines, map[string]string{
		"confidence":    fmt.Sprintf("%.4f", newConfidence),
		"last_decay_at": now.Format(time.RFC3339),
	})
	rebuilt := rebuildWithFrontMatter(updatedFM, lines[endIdx+1:])
	if writeErr := atomicWriteFile(filePath, []byte(rebuilt), 0600); writeErr != nil {
		VerbosePrintf("Warning: failed to write decay for %s: %v\n", l.ID, writeErr)
	}

	l.Utility *= newConfidence / confidence
	return l
}

// parseFrontmatterFromContent extracts specific fields from YAML frontmatter in a string.
// Like parseFrontmatterFields but operates on already-read content instead of opening the file.
func parseFrontmatterFromContent(content string, fields ...string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	dashCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			dashCount++
			if dashCount == 1 {
				inFrontmatter = true
				continue
			}
			if dashCount == 2 {
				break
			}
		}
		if inFrontmatter {
			for _, field := range fields {
				prefix := field + ":"
				if strings.HasPrefix(trimmed, prefix) {
					val := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
					val = strings.Trim(val, "\"'")
					result[field] = val
				}
			}
		}
	}
	return result
}

// jsonFloat extracts a float64 from a map, returning defaultVal if missing or non-positive.
func jsonFloat(data map[string]any, key string, defaultVal float64) float64 {
	if c, ok := data[key].(float64); ok && c > 0 {
		return c
	}
	return defaultVal
}

// jsonTimeField tries to parse a time.Time from the first non-empty string field found among keys.
func jsonTimeField(data map[string]any, keys ...string) time.Time {
	for _, k := range keys {
		if v, ok := data[k].(string); ok && v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

// computeDecayedConfidence applies exponential decay and clamps to a minimum of 0.1.
func computeDecayedConfidence(confidence, weeks float64) float64 {
	decayFactor := math.Exp(-weeks * types.ConfidenceDecayRate)
	result := confidence * decayFactor
	if result < 0.1 {
		return 0.1
	}
	return result
}

// writeDecayFields updates the data map with new confidence, timestamp, and incremented decay count.
func writeDecayFields(data map[string]any, newConfidence float64, now time.Time) {
	data["confidence"] = newConfidence
	data["last_decay_at"] = now.Format(time.RFC3339)
	decayCount := 0.0
	if dc, ok := data["decay_count"].(float64); ok {
		decayCount = dc
	}
	data["decay_count"] = decayCount + 1
}

// frontMatter holds parsed YAML front matter fields
type frontMatter struct {
	SupersededBy string
	PromotedTo   string
	Utility      float64
	HasUtility   bool
	SourceBead   string
	SourcePhase  string
	Maturity     string
	Stability    string
}

// parseFrontMatter extracts YAML front matter from markdown content
func parseFrontMatter(lines []string) (frontMatter, int) {
	var fm frontMatter

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, 0
	}

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			return fm, i + 1
		}
		parseFrontMatterLine(line, &fm)
	}
	return fm, 0
}

// parseFrontMatterLine parses a single YAML front matter line into fm fields.
func parseFrontMatterLine(line string, fm *frontMatter) {
	switch {
	case strings.HasPrefix(line, "superseded_by:"), strings.HasPrefix(line, "superseded-by:"):
		fm.SupersededBy = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "promoted_to:"), strings.HasPrefix(line, "promoted-to:"):
		fm.PromotedTo = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "utility:"):
		utilityStr := strings.TrimSpace(strings.TrimPrefix(line, "utility:"))
		if utility, err := strconv.ParseFloat(utilityStr, 64); err == nil && utility > 0 {
			fm.Utility = utility
			fm.HasUtility = true
		}
	case strings.HasPrefix(line, "source_bead:"), strings.HasPrefix(line, "source-bead:"):
		fm.SourceBead = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "source_phase:"), strings.HasPrefix(line, "source-phase:"):
		fm.SourcePhase = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "maturity:"):
		fm.Maturity = strings.TrimSpace(strings.TrimPrefix(line, "maturity:"))
	case strings.HasPrefix(line, "stability:"):
		fm.Stability = strings.TrimSpace(strings.TrimPrefix(line, "stability:"))
	}
}

// isInlineMetadata returns true for lines like "**ID**: L1" or "**Category**: process"
// that are formatting artifacts from older learning/pattern file formats, not actual content.
func isInlineMetadata(line string) bool {
	for _, field := range []string{"ID", "Category", "Confidence", "Date", "Source", "Type", "Status"} {
		if strings.HasPrefix(line, "**"+field+"**:") || strings.HasPrefix(line, "**"+field+":**") {
			return true
		}
	}
	return false
}

// extractSummary finds the first content paragraph after headings,
// skipping inline metadata lines like "**ID**: L1".
func extractSummary(lines []string, startIdx int) string {
	for i := startIdx; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") || isInlineMetadata(line) {
			continue
		}
		// Take first paragraph (up to 3 lines)
		summary := line
		for j := i + 1; j < len(lines) && j < i+3; j++ {
			nextLine := strings.TrimSpace(lines[j])
			if nextLine == "" || strings.HasPrefix(nextLine, "#") || isInlineMetadata(nextLine) {
				break
			}
			summary += " " + nextLine
		}
		return truncateText(summary, 200)
	}
	return ""
}

// isSuperseded returns true if the front matter indicates a superseded learning.
func isSuperseded(fm frontMatter) bool {
	return fm.SupersededBy != "" && fm.SupersededBy != "null" && fm.SupersededBy != "~"
}

// isPromoted returns true if the learning was promoted to a global location.
func isPromoted(fm frontMatter) bool {
	return fm.PromotedTo != "" && fm.PromotedTo != "null" && fm.PromotedTo != "~"
}

// parseLearningBody extracts title, ID, and maturity from markdown body lines into l.
func parseLearningBody(lines []string, start int, l *learning) {
	defaultID := filepath.Base(l.Source)
	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "# ") && l.Title == "" {
			l.Title = strings.TrimPrefix(line, "# ")
		} else if (strings.HasPrefix(line, "ID:") || strings.HasPrefix(line, "id:")) && l.ID == defaultID {
			l.ID = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
		// Extract maturity from inline metadata format: "**Maturity**: X" or "- **Maturity**: X"
		if l.Maturity == "" {
			trimmed := strings.TrimPrefix(line, "- ")
			if strings.HasPrefix(trimmed, "**Maturity**:") {
				l.Maturity = strings.TrimSpace(strings.TrimPrefix(trimmed, "**Maturity**:"))
			}
		}
	}
}

// parseLearningFile extracts learning info from a file
// Sets Superseded=true if superseded_by field is found
func parseLearningFile(path string) (learning, error) {
	if strings.HasSuffix(path, ".jsonl") {
		return parseLearningJSONL(path)
	}

	l := learning{
		ID:     filepath.Base(path),
		Source: path,
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return l, err
	}

	lines := strings.Split(string(content), "\n")
	fm, contentStart := parseFrontMatter(lines)

	if isSuperseded(fm) {
		l.Superseded = true
		return l, nil
	}
	if isPromoted(fm) {
		l.Superseded = true // reuse existing skip mechanism
		return l, nil
	}
	if fm.HasUtility {
		l.Utility = fm.Utility
	}
	l.SourceBead = fm.SourceBead
	l.SourcePhase = sanitizeSourcePhase(fm.SourcePhase)
	l.Maturity = fm.Maturity
	l.Stability = fm.Stability

	parseLearningBody(lines, contentStart, &l)
	l.Summary = extractSummary(lines, contentStart)
	// Store full body text for search (used by lookup --query body matching)
	l.BodyText = strings.Join(lines[contentStart:], "\n")

	if l.Title == "" {
		l.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	return l, nil
}

// populateLearningFromJSON fills learning fields from a parsed JSON map.
func populateLearningFromJSON(data map[string]any, l *learning) {
	if id, ok := data["id"].(string); ok {
		l.ID = id
	}
	if title, ok := data["title"].(string); ok {
		l.Title = title
	}
	if summary, ok := data["summary"].(string); ok {
		l.Summary = truncateText(summary, 200)
	}
	if content, ok := data["content"].(string); ok && l.Summary == "" {
		l.Summary = truncateText(content, 200)
	}
	if utility, ok := data["utility"].(float64); ok && utility > 0 {
		l.Utility = utility
	}
	if sb, ok := data["source_bead"].(string); ok {
		l.SourceBead = sb
	}
	if sp, ok := data["source_phase"].(string); ok {
		l.SourcePhase = sanitizeSourcePhase(sp)
	}
	if m, ok := data["maturity"].(string); ok {
		l.Maturity = m
	}
	if s, ok := data["stability"].(string); ok {
		l.Stability = s
	}
}

// parseLearningJSONL extracts learning from JSONL file
// Returns empty learning (with Superseded=true) if superseded_by field is set
func parseLearningJSONL(path string) (learning, error) {
	l := learning{
		ID:      filepath.Base(path),
		Source:  path,
		Utility: types.InitialUtility, // Default to 0.5
	}

	f, err := os.Open(path)
	if err != nil {
		return l, err
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // read-only learning load, close error non-fatal
	}()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return l, nil
	}

	var data map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &data); err != nil {
		VerbosePrintf("Warning: parse JSONL %s: %v\n", path, err)
		return l, nil
	}

	// F3: Filter superseded learnings - skip if superseded_by is set
	if supersededBy, ok := data["superseded_by"]; ok && supersededBy != nil && supersededBy != "" {
		l.Superseded = true
		return l, nil
	}

	populateLearningFromJSON(data, &l)
	// Populate body text from content field for search
	if content, ok := data["content"].(string); ok {
		l.BodyText = content
	}
	return l, nil
}

// quarantineLearning moves a learning file to .quarantine/ subdirectory with reason.
func quarantineLearning(path, reason string) error {
	dir := filepath.Dir(path)
	quarantineDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(quarantineDir, 0o755); err != nil {
		return err
	}
	base := filepath.Base(path)
	dest := filepath.Join(quarantineDir, base)
	return os.Rename(path, dest)
}
