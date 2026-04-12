package main

import (
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
	"github.com/boshu2/agentops/cli/internal/search"
	"github.com/boshu2/agentops/cli/internal/types"
)

// validPhases — canonical definition in internal/search.
var validPhases = search.ValidPhases

// sanitizeSourcePhase delegates to search.SanitizeSourcePhase.
func sanitizeSourcePhase(phase string) string { return search.SanitizeSourcePhase(phase) }

// nowFunc is the package-level clock used by collectLearnings for
// freshness scoring. Production callers get time.Now; the replay
// determinism test (C6, Micro-epic 7) overrides it via
// t.Cleanup(func() { nowFunc = time.Now })` so every run against the
// frozen-mtime fixture produces byte-identical ranking output.
//
// ANTI-GOAL per pre-mortem: do NOT thread a `now time.Time` parameter
// through collectLearnings / processLearningFile / ApplyFreshnessToLearning.
// The 8 production callers of collectLearnings (retrieval_bench.go:200
// and 406,418; lookup.go:101,164; inject.go:340; codex.go:594;
// context_ranked_intel.go:51; flywheel_gate.go:80) would all need
// signature updates, inflating the scope way past the Micro-epic 7
// budget for no real gain. A single package-level var is the minimum
// change that delivers determinism.
//
// The variable is package-private so non-test callers cannot mutate
// it; only files in package main (which means only in-tree tests) can
// reach it.
var nowFunc = time.Now

// collectLearnings finds recent learnings from .agents/learnings/ and optionally ~/.agents/learnings/.
// Implements MemRL Two-Phase retrieval: Phase A (similarity/freshness) + Phase B (utility-weighted)
// With CASS integration: applies confidence decay when --apply-decay is set.
// Global learnings receive a post-scoring weight penalty (globalWeight, default 0.8).
func collectLearnings(cwd, query string, limit int, globalDir string, globalWeight float64) ([]learning, error) {
	files, err := findLearningFiles(cwd)
	if err != nil {
		return nil, err
	}

	now := nowFunc()
	tokens := queryTokens(strings.ToLower(query))
	learnings := collectLocalLearnings(files, tokens, now)

	if globalDir != "" {
		localPaths, localTitles := localLearningDedupeSets(files, learnings)
		learnings = appendGlobalLearnings(learnings, globalDir, localPaths, localTitles, tokens, now)
	}

	if len(learnings) == 0 {
		return nil, nil
	}

	rankLearnings(learnings)
	applyGlobalLearningWeight(learnings, globalWeight)

	return limitCollectedLearnings(learnings, limit), nil
}

func collectLocalLearnings(files []string, tokens []string, now time.Time) []learning {
	learnings := make([]learning, 0, len(files))
	for _, file := range files {
		l, ok := processLearningFile(file, tokens, now)
		if !ok {
			continue
		}
		learnings = append(learnings, l)
	}
	return learnings
}

func localLearningDedupeSets(files []string, learnings []learning) (map[string]bool, map[string]bool) {
	localPaths := make(map[string]bool, len(files))
	localTitles := make(map[string]bool, len(learnings))
	for _, f := range files {
		if abs, err := filepath.Abs(f); err == nil {
			localPaths[abs] = true
		}
	}
	for _, l := range learnings {
		if l.Title != "" {
			localTitles[strings.ToLower(l.Title)] = true
		}
	}
	return localPaths, localTitles
}

func appendGlobalLearnings(learnings []learning, globalDir string, localPaths, localTitles map[string]bool, tokens []string, now time.Time) []learning {
	globalFiles := globLearningFiles(globalDir)
	for _, file := range globalFiles {
		if globalLearningDuplicate(file, localPaths) {
			continue
		}
		l, ok := processLearningFile(file, tokens, now)
		if !ok {
			continue
		}
		if l.Title != "" && localTitles[strings.ToLower(l.Title)] {
			continue
		}
		l.Global = true
		learnings = append(learnings, l)
	}
	return learnings
}

func globalLearningDuplicate(file string, localPaths map[string]bool) bool {
	abs, err := filepath.Abs(file)
	return err == nil && localPaths[abs]
}

func applyGlobalLearningWeight(learnings []learning, globalWeight float64) {
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
}

func limitCollectedLearnings(learnings []learning, limit int) []learning {
	if len(learnings) > limit {
		return learnings[:limit]
	}
	return learnings
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

// Thin wrappers — canonical definitions in internal/search/learnings.go.
func queryTokens(queryLower string) []string { return search.QueryTokens(queryLower) }
func matchesQuery(tokens []string, title, summary, body string) bool {
	return search.MatchesQuery(tokens, title, summary, body)
}
func matchRatio(tokens []string, title, summary, body string) float64 {
	return search.MatchRatio(tokens, title, summary, body)
}

const (
	sectionCoverageBonusCap    = search.SectionCoverageBonusCap
	sectionCoverageBonusWeight = search.SectionCoverageBonusWeight
	sectionSnippetMaxChars     = search.SectionSnippetMaxChars
)

type learningSectionCandidate = search.LearningSectionCandidate

func applyLearningSectionEvidence(l *learning, queryTokensList []string) bool {
	l.SectionHeading = ""
	l.SectionLocator = ""
	l.MatchedSnippet = ""
	l.MatchConfidence = 0
	l.MatchProvenance = ""

	if len(queryTokensList) == 0 {
		return true
	}

	query := strings.Join(queryTokensList, " ")
	sections := buildLearningSectionCandidates(*l)
	if len(sections) == 0 {
		ratio := matchRatio(queryTokensList, l.Title, l.Summary, l.BodyText)
		if ratio == 0 {
			return false
		}
		l.MatchConfidence = ratio
		l.MatchProvenance = "whole-file"
		l.MatchedSnippet = compactText(createSearchSnippet(strings.TrimSpace(l.BodyText), query, sectionSnippetMaxChars))
		return true
	}

	useWeighted := !isPrimaryMetricNamespace(defaultCitationMetricNamespace())
	matched := make([]learningSectionCandidate, 0, len(sections))
	for _, section := range sections {
		if useWeighted {
			section.Score = weightedSectionScore(queryTokensList, section.Heading, section.Content, section.Index, len(sections))
		} else {
			section.Score = matchRatio(queryTokensList, section.Heading, "", section.Content)
		}
		if section.Score == 0 {
			continue
		}
		section.Snippet = compactText(createSearchSnippet(section.Content, query, sectionSnippetMaxChars))
		matched = append(matched, section)
	}
	if len(matched) == 0 {
		return false
	}

	slices.SortFunc(matched, func(a, b learningSectionCandidate) int {
		if diff := cmp.Compare(b.Score, a.Score); diff != 0 {
			return diff
		}
		return cmp.Compare(a.Index, b.Index)
	})

	best := matched[0]
	corroboratingScore := 0.0
	for _, section := range matched[1:] {
		if section.Locator != best.Locator {
			corroboratingScore = section.Score
			break
		}
	}

	confidence := best.Score + math.Min(sectionCoverageBonusCap, corroboratingScore*sectionCoverageBonusWeight)
	if confidence > 1.0 {
		confidence = 1.0
	}

	l.SectionHeading = best.Heading
	l.SectionLocator = best.Locator
	l.MatchedSnippet = best.Snippet
	l.MatchConfidence = confidence
	if useWeighted {
		l.MatchProvenance = "section-rollup-weighted"
	} else {
		l.MatchProvenance = "section-rollup"
	}
	return true
}

func buildLearningSectionCandidates(l learning) []learningSectionCandidate {
	body := strings.TrimSpace(l.BodyText)
	if body == "" {
		body = strings.TrimSpace(strings.Join([]string{l.Title, l.Summary}, "\n\n"))
	}
	if body == "" {
		return nil
	}

	rawSections := splitMarkdownSections(body)
	if len(rawSections) == 0 {
		rawSections = []string{body}
	}

	candidates := make([]learningSectionCandidate, 0, len(rawSections))
	seenLocators := make(map[string]int, len(rawSections))
	for idx, raw := range rawSections {
		heading, content := extractLearningSectionHeading(raw, l.Title, idx)
		locator := buildLearningSectionLocator(heading, idx, seenLocators)
		candidates = append(candidates, learningSectionCandidate{
			Heading: heading,
			Locator: locator,
			Content: content,
			Index:   idx,
		})
	}
	return candidates
}

func extractLearningSectionHeading(section, fallbackTitle string, index int) (string, string) {
	return search.ExtractLearningSectionHeading(section, fallbackTitle, index)
}

func buildLearningSectionLocator(heading string, index int, seen map[string]int) string {
	return search.BuildLearningSectionLocator(heading, index, seen)
}

func slugifyLearningSectionHeading(heading string) string {
	return search.SlugifyLearningSectionHeading(heading)
}

// processLearningFile parses, filters, and scores a single learning file.
// Returns the learning and true if it should be included, false otherwise.
func processLearningFile(file string, queryTokensList []string, now time.Time) (learning, bool) {
	l, err := parseLearningFile(file)
	if err != nil {
		return l, false
	}
	if l.Superseded {
		VerbosePrintf("Skipping superseded learning: %s\n", l.ID)
		return l, false
	}
	if !applyLearningSectionEvidence(&l, queryTokensList) {
		return l, false
	}

	applyFreshnessScore(&l, file, now)

	// Partial matches (OR fallback) get proportionally lower utility.
	// Full AND match = 1.0x, half tokens matched = 0.5x.
	if l.MatchConfidence > 0 && l.MatchConfidence < 1.0 {
		l.Utility *= l.MatchConfidence
	}

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

	// Soft penalty: penalize unsourced learnings that passed the hard gate.
	// 0.7x is enough to rank sourced learnings higher without creating a cliff
	// that kills learnings at the quality gate (utility * 0.3 ≤ 0.3 for most).
	if l.SourceBead == "" {
		l.Utility *= 0.7
	}

	if l.Stability == "experimental" {
		fmt.Fprintf(os.Stderr, "WARNING: Skill %q is marked experimental — verify outputs carefully\n", l.Title)
	}

	return l, true
}

// passesQualityGate delegates to search.PassesQualityGate.
func passesQualityGate(l learning) bool { return search.PassesQualityGate(l) }

// applyFreshnessScore delegates to search.ApplyFreshnessToLearning.
func applyFreshnessScore(l *learning, file string, now time.Time) {
	search.ApplyFreshnessToLearning(l, file, now)
}

// rankLearnings delegates to search.RankLearnings + sort.
func rankLearnings(learnings []learning) {
	search.RankLearnings(learnings)
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

// Thin wrappers — canonical definitions in internal/search/learnings.go.
func parseFrontmatterFromContent(content string, fields ...string) map[string]string {
	return search.ParseFrontmatterFromContent(content, fields...)
}
func jsonFloat(data map[string]any, key string, defaultVal float64) float64 {
	return search.JSONFloat(data, key, defaultVal)
}
func jsonTimeField(data map[string]any, keys ...string) time.Time {
	return search.JSONTimeField(data, keys...)
}
func computeDecayedConfidence(confidence, weeks float64) float64 {
	return search.ComputeDecayedConfidence(confidence, weeks)
}
func writeDecayFields(data map[string]any, newConfidence float64, now time.Time) {
	search.WriteDecayFields(data, newConfidence, now)
}

// Type alias + thin wrappers — canonical definitions in internal/search/learnings.go.
type frontMatter = search.FrontMatter

func parseFrontMatter(lines []string) (frontMatter, int) { return search.ParseFrontMatter(lines) }
func parseFrontMatterLine(line string, fm *frontMatter)  { search.ParseFrontMatterLine(line, fm) }
func isInlineMetadata(line string) bool                  { return search.IsInlineMetadata(line) }
func extractSummary(lines []string, startIdx int) string {
	return search.ExtractSummary(lines, startIdx)
}
func isSuperseded(fm frontMatter) bool { return search.IsSuperseded(fm) }
func isPromoted(fm frontMatter) bool   { return search.IsPromoted(fm) }

func parseLearningBody(lines []string, start int, l *learning) {
	search.ParseLearningBody(lines, start, l)
}
func parseLearningFile(path string) (learning, error) { return search.ParseLearningFile(path) }
func populateLearningFromJSON(data map[string]any, l *learning) {
	search.PopulateLearningFromJSON(data, l)
}

// parseLearningJSONL wraps search.ParseLearningJSONL with verbose logging.
func parseLearningJSONL(path string) (learning, error) {
	l, err := search.ParseLearningJSONL(path)
	return l, err
}

// quarantineLearning delegates to search.QuarantineLearning.
func quarantineLearning(path, _ string) error { return search.QuarantineLearning(path) }
