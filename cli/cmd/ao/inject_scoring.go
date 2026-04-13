package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/boshu2/agentops/cli/internal/search"
)

// scorable is a package-level alias for search.Scorable — kept for test compat.
type scorable = search.Scorable

// injectMaturityWeights kept as package-level alias for test compatibility.
var injectMaturityWeights = search.InjectMaturityWeights

// Thin wrappers — delegate to search package, kept for test compatibility.
func maturityWeight(maturity string) float64 { return search.MaturityWeight(maturity) }
func freshnessScore(ageWeeks float64) float64 { return search.FreshnessScore(ageWeeks) }
func tokenizeWords(text string) []string      { return search.TokenizeWords(text) }

func computeAdjacency(tokens []string, words []string) float64 {
	return search.ComputeAdjacency(tokens, words)
}

func weightedSectionScore(tokens []string, heading, content string, sectionIndex, totalSections int) float64 {
	return search.WeightedSectionScore(tokens, heading, content, sectionIndex, totalSections)
}

// Scoring weight constants — kept for test compatibility.
const (
	wSubstringCoverage = search.WSubstringCoverage
	wHeadingMatch      = search.WHeadingMatch
	wExactToken        = search.WExactToken
	wAdjacency         = search.WAdjacency
	wSectionProximity  = search.WSectionProximity
)

// applyCompositeScoringTo delegates to search.ApplyCompositeScoringTo.
func applyCompositeScoringTo(items []search.Scorable, lambda float64) {
	search.ApplyCompositeScoringTo(items, lambda)
}

// contentHash returns the SHA256 hex digest of the first 500 characters of content.
// Used for deduplication — items with identical leading content are considered duplicates.
func contentHash(content string) string {
	if len(content) > 500 {
		content = content[:500]
	}
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", sum)
}

// deduplicateByContentHash removes items whose first 500 chars of content hash to
// the same value as a previously seen item. The first occurrence wins.
// Items without content (empty string) are never de-duped against each other.
func deduplicateByContentHash(items []search.Scorable, getContent func(search.Scorable) string) []search.Scorable {
	seen := make(map[string]bool, len(items))
	out := make([]search.Scorable, 0, len(items))
	for _, item := range items {
		c := getContent(item)
		if c == "" {
			out = append(out, item)
			continue
		}
		h := contentHash(c)
		if seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, item)
	}
	return out
}

// indexMdWikilinks parses [[path]] wikilinks from an INDEX.md file.
// Returns a set of normalised path strings (lowercased, trimmed).
// If the file doesn't exist or can't be read, returns an empty set.
func indexMdWikilinks(cwd string) map[string]bool {
	indexPath := filepath.Join(cwd, ".agents", "INDEX.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil
	}
	re := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	matches := re.FindAllSubmatch(data, -1)
	links := make(map[string]bool, len(matches))
	for _, m := range matches {
		if len(m) >= 2 {
			links[strings.ToLower(strings.TrimSpace(string(m[1])))] = true
		}
	}
	return links
}

// applyIndexMdBoostToLearnings multiplies the CompositeScore of learnings whose path
// appears as a wikilink in INDEX.md by boostFactor. This implements the Karpathy
// "read index first" pattern at scoring time.
func applyIndexMdBoostToLearnings(learnings []learning, links map[string]bool, boostFactor float64) {
	if len(links) == 0 {
		return
	}
	for i := range learnings {
		p := strings.ToLower(strings.TrimSpace(learnings[i].Source))
		if p == "" {
			continue
		}
		// Match on the tail of the path — wikilinks rarely carry full absolute paths.
		for link := range links {
			if strings.HasSuffix(p, link) || strings.HasSuffix(p, link+".md") {
				learnings[i].CompositeScore *= boostFactor
				break
			}
		}
	}
}
