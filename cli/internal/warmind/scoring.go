package warmind

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Scorer scores warmind candidates against existing team knowledge.
type Scorer struct {
	Config        ScoringConfig
	LearningsDir  string
	existingCache []Learning
}

// NewScorer creates a new scorer.
func NewScorer(cfg ScoringConfig, learningsDir string) *Scorer {
	return &Scorer{
		Config:       cfg,
		LearningsDir: learningsDir,
	}
}

// Score scores a candidate and returns the result.
func (s *Scorer) Score(candidate Candidate) ScoringResult {
	// Load existing learnings for novelty comparison
	existing := s.loadExistingLearnings()

	// Calculate each dimension
	novelty := s.scoreNovelty(candidate, existing)
	specificity := s.scoreSpecificity(candidate)
	actionability := s.scoreActionability(candidate)
	confidence := s.scoreConfidence(candidate)

	// Weighted composite
	composite := s.Config.NoveltyWeight*novelty +
		s.Config.SpecificityWeight*specificity +
		s.Config.ActionabilityWeight*actionability +
		s.Config.ConfidenceWeight*confidence

	// Clamp to [0, 1]
	if composite > 1.0 {
		composite = 1.0
	}
	if composite < 0 {
		composite = 0
	}

	return ScoringResult{
		Novelty:         novelty,
		Specificity:     specificity,
		Actionability:   actionability,
		ConfidenceScore: confidence,
		CompositeScore:  composite,
		Tier:            TierFromScore(composite),
		ScoredAt:        time.Now(),
	}
}

// scoreNovelty measures how different the candidate is from existing learnings.
// Uses Jaccard similarity on keywords (fallback when no embedding model available).
func (s *Scorer) scoreNovelty(candidate Candidate, existing []Learning) float64 {
	if len(existing) == 0 {
		// First learning gets high novelty but not perfect (bootstrap penalty)
		return 0.85
	}

	candidateKeywords := extractKeywords(candidate.Content)
	if len(candidateKeywords) == 0 {
		return 0.5 // No keywords = medium novelty
	}

	// Find max similarity to any existing learning
	maxSimilarity := 0.0
	for _, learn := range existing {
		existingKeywords := extractKeywords(learn.Content)
		sim := jaccardSimilarity(candidateKeywords, existingKeywords)
		if sim > maxSimilarity {
			maxSimilarity = sim
		}
	}

	// Novelty = 1 - similarity
	novelty := 1.0 - maxSimilarity

	// Apply softmax-style smoothing to avoid extreme values
	return smoothScore(novelty)
}

// scoreSpecificity measures how concrete vs vague the content is.
func (s *Scorer) scoreSpecificity(candidate Candidate) float64 {
	content := candidate.Content
	score := 0.5 // Start neutral

	// Positive signals: code blocks, file paths, specific values
	if strings.Contains(content, "```") {
		score += 0.15 // Has code examples
	}
	if strings.Contains(content, "/") && strings.Contains(content, ".") {
		score += 0.1 // Has file paths
	}
	if hasSpecificValues(content) {
		score += 0.1 // Has specific numbers, configs, etc.
	}
	if hasConcreteExamples(content) {
		score += 0.1 // Has examples with actual values
	}

	// Negative signals: vague language
	vaguePatterns := []string{
		"might", "maybe", "could", "possibly",
		"in general", "typically", "usually",
		"something like", "kind of", "sort of",
	}
	vagueCount := 0
	lowerContent := strings.ToLower(content)
	for _, pattern := range vaguePatterns {
		if strings.Contains(lowerContent, pattern) {
			vagueCount++
		}
	}
	score -= float64(vagueCount) * 0.05

	return clamp(score, 0, 1)
}

// scoreActionability measures how directly applicable the content is.
func (s *Scorer) scoreActionability(candidate Candidate) float64 {
	content := candidate.Content
	score := 0.4 // Start slightly low

	// Positive signals: imperative verbs, step-by-step instructions
	actionVerbs := []string{
		"use", "add", "create", "implement", "configure",
		"set", "run", "install", "update", "remove",
		"check", "ensure", "verify", "avoid", "prefer",
	}
	lowerContent := strings.ToLower(content)
	actionCount := 0
	for _, verb := range actionVerbs {
		if strings.Contains(lowerContent, verb+" ") {
			actionCount++
		}
	}
	score += float64(actionCount) * 0.05

	// Has numbered steps
	if regexp.MustCompile(`\d+\.\s`).MatchString(content) {
		score += 0.15
	}

	// Has bullet points
	if strings.Contains(content, "- ") || strings.Contains(content, "* ") {
		score += 0.1
	}

	// Has "do" and "don't" guidance
	if strings.Contains(lowerContent, "don't") || strings.Contains(lowerContent, "do not") {
		score += 0.1
	}
	if strings.Contains(lowerContent, "always") || strings.Contains(lowerContent, "never") {
		score += 0.05
	}

	return clamp(score, 0, 1)
}

// scoreConfidence uses the author's stated confidence.
func (s *Scorer) scoreConfidence(candidate Candidate) float64 {
	// Use the author's confidence directly, with a small floor
	conf := candidate.Confidence
	if conf < 0.3 {
		conf = 0.3 // Floor to avoid penalizing humble authors too much
	}
	return conf
}

// loadExistingLearnings loads all learnings from the warmind directory.
func (s *Scorer) loadExistingLearnings() []Learning {
	if s.existingCache != nil {
		return s.existingCache
	}

	var learnings []Learning

	files, err := os.ReadDir(s.LearningsDir)
	if err != nil {
		return learnings
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		path := filepath.Join(s.LearningsDir, file.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		learnings = append(learnings, Learning{
			ID:       strings.TrimSuffix(file.Name(), ".md"),
			Content:  string(content),
			FilePath: path,
		})
	}

	s.existingCache = learnings
	return learnings
}

// --- Helper functions ---

// extractKeywords extracts significant words from content.
func extractKeywords(content string) map[string]bool {
	keywords := make(map[string]bool)

	// Remove code blocks (keep them for specificity but not for keyword matching)
	cleaned := regexp.MustCompile("```[\\s\\S]*?```").ReplaceAllString(content, "")

	// Tokenize
	words := strings.Fields(strings.ToLower(cleaned))

	// Stop words to ignore
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true,
		"this": true, "that": true, "these": true, "those": true,
		"it": true, "its": true, "if": true, "then": true, "else": true,
		"when": true, "where": true, "which": true, "who": true, "what": true,
		"how": true, "why": true, "can": true, "not": true, "no": true,
		"yes": true, "all": true, "any": true, "some": true, "more": true,
		"most": true, "other": true, "into": true, "over": true, "such": true,
	}

	for _, word := range words {
		// Clean punctuation
		word = strings.Trim(word, ".,;:!?()[]{}\"'`")

		// Skip short words and stop words
		if len(word) < 3 || stopWords[word] {
			continue
		}

		keywords[word] = true
	}

	return keywords
}

// jaccardSimilarity computes Jaccard similarity between two keyword sets.
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	intersection := 0
	for k := range a {
		if b[k] {
			intersection++
		}
	}

	union := len(a)
	for k := range b {
		if !a[k] {
			union++
		}
	}

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// hasSpecificValues checks for specific numbers, configs, etc.
func hasSpecificValues(content string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\d+\.\d+`),          // Version numbers
		regexp.MustCompile(`\d{2,}`),            // Numbers with 2+ digits
		regexp.MustCompile(`[A-Z_]{3,}=`),       // Environment variables
		regexp.MustCompile(`"[^"]+": `),         // JSON keys
		regexp.MustCompile(`\w+:\s*\d+`),        // Config values
		regexp.MustCompile(`--\w+`),             // CLI flags
		regexp.MustCompile(`[a-z]+_[a-z]+`),     // Snake_case identifiers
		regexp.MustCompile(`[a-z]+[A-Z][a-z]+`), // camelCase identifiers
	}

	for _, p := range patterns {
		if p.MatchString(content) {
			return true
		}
	}
	return false
}

// hasConcreteExamples checks for examples with actual values.
func hasConcreteExamples(content string) bool {
	lowerContent := strings.ToLower(content)
	exampleIndicators := []string{
		"for example", "e.g.", "such as", "like this",
		"example:", "sample:", "output:", "result:",
	}

	for _, indicator := range exampleIndicators {
		if strings.Contains(lowerContent, indicator) {
			return true
		}
	}
	return false
}

// smoothScore applies a sigmoid-like smoothing to avoid extreme values.
func smoothScore(x float64) float64 {
	// Soft sigmoid that keeps values in reasonable range
	return 0.1 + 0.8/(1+math.Exp(-5*(x-0.5)))
}

// clamp restricts a value to [min, max].
func clamp(x, min, max float64) float64 {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}
