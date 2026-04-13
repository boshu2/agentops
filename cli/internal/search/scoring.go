package search

import (
	"math"
	"strings"
	"unicode"
)

// Scorable is the interface for items that participate in MemRL Two-Phase
// composite scoring. Both learning and pattern implement this interface.
type Scorable interface {
	GetFreshness() float64
	GetUtility() float64
	GetMaturity() string
	SetComposite(float64)
}

// InjectMaturityWeights maps CASS maturity levels to scoring multipliers.
// Slightly softer than search weights (1.5/0.3) since inject has smaller pools.
var InjectMaturityWeights = map[string]float64{
	"established":  1.3,
	"candidate":    1.1,
	"provisional":  1.0,
	"anti-pattern": 0.4,
}

// MaturityWeight returns the scoring multiplier for a CASS maturity level.
func MaturityWeight(maturity string) float64 {
	if w, found := InjectMaturityWeights[maturity]; found {
		return w
	}
	return 1.0
}

// StabilityWeight returns the scoring multiplier for learning stability.
func StabilityWeight(stability string) float64 {
	switch strings.ToLower(strings.TrimSpace(stability)) {
	case "experimental":
		return 0.7
	default:
		return 1.0
	}
}

// FreshnessScore calculates decay-adjusted score: exp(-ageWeeks * decayRate)
// Based on knowledge decay rate δ = 0.17/week (Darr et al.)
func FreshnessScore(ageWeeks float64) float64 {
	const decayRate = 0.17
	score := math.Exp(-ageWeeks * decayRate)
	// Clamp to [0.1, 1.0] - old knowledge still has some value
	if score < 0.1 {
		return 0.1
	}
	return score
}

// Weighted scoring feature weights for shadow namespace experimentation.
const (
	WSubstringCoverage = 0.30 // existing matchRatio (fraction of tokens with substring hit)
	WHeadingMatch      = 0.25 // bonus when tokens appear in the section heading
	WExactToken        = 0.20 // fraction of tokens matching as whole words
	WAdjacency         = 0.15 // how close query tokens appear to each other
	WSectionProximity  = 0.10 // bonus for sections near the top of the document
)

// WeightedSectionScore computes a multi-feature relevance score for a section.
// tokens must be lowercased. heading and content are the section's heading and body.
// sectionIndex is the 0-based position of the section within the document.
// totalSections is the total number of sections in the document.
// Returns a score in [0, 1].
func WeightedSectionScore(tokens []string, heading, content string, sectionIndex, totalSections int) float64 {
	if len(tokens) == 0 {
		return 1.0
	}

	headingLower := strings.ToLower(heading)
	contentLower := strings.ToLower(content)
	combined := headingLower + " " + contentLower

	// Feature 1: Substring coverage (existing matchRatio logic)
	substringHits := 0
	for _, tok := range tokens {
		if strings.Contains(combined, tok) {
			substringHits++
		}
	}
	substringCoverage := float64(substringHits) / float64(len(tokens))

	// Feature 2: Heading match — fraction of tokens appearing in heading
	headingHits := 0
	for _, tok := range tokens {
		if strings.Contains(headingLower, tok) {
			headingHits++
		}
	}
	headingScore := float64(headingHits) / float64(len(tokens))

	// Feature 3: Exact token coverage — whole-word matches in content
	exactHits := 0
	words := TokenizeWords(contentLower)
	wordSet := make(map[string]bool, len(words))
	for _, w := range words {
		wordSet[w] = true
	}
	for _, tok := range tokens {
		if wordSet[tok] {
			exactHits++
		}
	}
	exactScore := float64(exactHits) / float64(len(tokens))

	// Feature 4: Adjacency — average minimum distance between query tokens
	adjacencyScore := ComputeAdjacency(tokens, words)

	// Feature 5: Section proximity — reward sections near the top
	proximityScore := 1.0
	if totalSections > 1 {
		proximityScore = 1.0 - (float64(sectionIndex) / float64(totalSections))
	}

	score := WSubstringCoverage*substringCoverage +
		WHeadingMatch*headingScore +
		WExactToken*exactScore +
		WAdjacency*adjacencyScore +
		WSectionProximity*proximityScore

	if score > 1.0 {
		score = 1.0
	}
	return score
}

// TokenizeWords splits text into lowercase word tokens, stripping punctuation.
func TokenizeWords(text string) []string {
	var words []string
	for _, field := range strings.Fields(text) {
		w := strings.TrimFunc(field, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if len(w) >= 2 {
			words = append(words, w)
		}
	}
	return words
}

// ComputeAdjacency measures how close query tokens appear to each other in the word list.
// Returns a score in [0, 1] where 1 means tokens are adjacent.
func ComputeAdjacency(tokens []string, words []string) float64 {
	if len(tokens) < 2 || len(words) == 0 {
		if len(tokens) == 1 {
			for _, w := range words {
				if w == tokens[0] {
					return 1.0
				}
			}
			return 0.0
		}
		return 0.0
	}

	// Find first occurrence position of each token
	positions := make(map[string]int, len(tokens))
	for _, tok := range tokens {
		for i, w := range words {
			if w == tok {
				positions[tok] = i
				break
			}
		}
	}

	if len(positions) < 2 {
		return 0.0
	}

	// Compute average pairwise distance between found tokens
	totalDist := 0
	pairs := 0
	posTokens := make([]string, 0, len(positions))
	for tok := range positions {
		posTokens = append(posTokens, tok)
	}
	for i := 0; i < len(posTokens); i++ {
		for j := i + 1; j < len(posTokens); j++ {
			d := positions[posTokens[i]] - positions[posTokens[j]]
			if d < 0 {
				d = -d
			}
			totalDist += d
			pairs++
		}
	}

	if pairs == 0 {
		return 0.0
	}

	avgDist := float64(totalDist) / float64(pairs)
	return math.Exp(-0.1 * (avgDist - 1))
}

// ApplyCompositeScoringTo implements MemRL Two-Phase scoring for any Scorable slice.
// Score = (z_norm(freshness) + λ × z_norm(utility)) × maturityWeight
func ApplyCompositeScoringTo(items []Scorable, lambda float64) {
	if len(items) == 0 {
		return
	}

	// With fewer than 3 items, z-normalization is statistically meaningless.
	if len(items) < 3 {
		for _, item := range items {
			score := item.GetFreshness() + lambda*item.GetUtility()
			item.SetComposite(score * MaturityWeight(item.GetMaturity()))
		}
		return
	}

	var sumF, sumU float64
	for _, item := range items {
		sumF += item.GetFreshness()
		sumU += item.GetUtility()
	}
	n := float64(len(items))
	meanF := sumF / n
	meanU := sumU / n

	var varF, varU float64
	for _, item := range items {
		f := item.GetFreshness()
		u := item.GetUtility()
		varF += (f - meanF) * (f - meanF)
		varU += (u - meanU) * (u - meanU)
	}
	stdF := math.Sqrt(varF / n)
	stdU := math.Sqrt(varU / n)

	if stdF < 0.001 {
		stdF = 0.001
	}
	if stdU < 0.001 {
		stdU = 0.001
	}

	for _, item := range items {
		zFresh := (item.GetFreshness() - meanF) / stdF
		zUtility := (item.GetUtility() - meanU) / stdU
		score := zFresh + lambda*zUtility
		item.SetComposite(score * MaturityWeight(item.GetMaturity()))
	}
}
