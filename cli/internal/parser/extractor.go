// Package parser provides extraction patterns for knowledge discovery.
package parser

import (
	"regexp"
	"strings"

	"github.com/boshu2/agentops/cli/internal/types"
)

// ExtractionPattern defines a pattern for identifying knowledge types.
type ExtractionPattern struct {
	// Type is the knowledge type this pattern identifies.
	Type types.KnowledgeType

	// Keywords are phrases that suggest this knowledge type.
	Keywords []string

	// Patterns are regex patterns to match.
	Patterns []*regexp.Regexp

	// MinScore is the minimum confidence for extraction (0.0-1.0).
	MinScore float64
}

// DefaultPatterns provides the standard extraction patterns.
var DefaultPatterns = []ExtractionPattern{
	{
		Type: types.KnowledgeTypeDecision,
		Keywords: []string{
			// Explicit markers
			"**Decision:**",
			"**decision**:",
			"decision:",
			// Choice language
			"decided to",
			"we chose",
			"the approach is",
			"going with",
			"will use",
			"architecture decision",
			"design choice",
			// Additional patterns from premortem
			"went with",
			"opted for",
			"selected",
			"picked",
			"settled on",
			"landed on",
			"choosing",
			"the plan is",
			"we'll go with",
			"moving forward with",
			"implementing with",
			"using instead",
			"prefer to",
			"better approach",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\*\*Decision\*\*:`),
			regexp.MustCompile(`(?i)decided to use \w+`),
			regexp.MustCompile(`(?i)chose (\w+) (over|instead of) \w+`),
			regexp.MustCompile(`(?i)going with \w+ because`),
			regexp.MustCompile(`(?i)went with \w+ (because|since|as)`),
			regexp.MustCompile(`(?i)opted for \w+ (over|instead)`),
			regexp.MustCompile(`(?i)selected \w+ (as|for)`),
		},
		MinScore: 0.6,
	},
	{
		Type: types.KnowledgeTypeSolution,
		Keywords: []string{
			"**Solution:**",
			"fixed by",
			"the fix is",
			"resolved by",
			"solved by",
			"workaround:",
			"here's how to",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\*\*Solution\*\*:`),
			regexp.MustCompile(`(?i)fixed? (the|this) (issue|bug|problem) by`),
			regexp.MustCompile(`(?i)resolved by`),
			regexp.MustCompile(`(?i)the fix (is|was)`),
		},
		MinScore: 0.7,
	},
	{
		Type: types.KnowledgeTypeLearning,
		Keywords: []string{
			"**Learning:**",
			"learned that",
			"TIL",
			"insight:",
			"key takeaway",
			"important to note",
			"what I learned",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\*\*Learning\*\*:`),
			regexp.MustCompile(`(?i)learned? that`),
			regexp.MustCompile(`(?i)key (takeaway|insight|learning)`),
			regexp.MustCompile(`(?i)important to (note|remember|know)`),
		},
		MinScore: 0.5,
	},
	{
		Type: types.KnowledgeTypeFailure,
		Keywords: []string{
			"**Failure:**",
			"didn't work",
			"failed because",
			"mistake was",
			"wrong approach",
			"shouldn't have",
			"anti-pattern",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\*\*Failure\*\*:`),
			regexp.MustCompile(`(?i)(didn't|doesn't|won't) work`),
			regexp.MustCompile(`(?i)failed? because`),
			regexp.MustCompile(`(?i)(the|my) mistake was`),
		},
		MinScore: 0.6,
	},
	{
		Type: types.KnowledgeTypeReference,
		Keywords: []string{
			"**Reference:**",
			"see also",
			"refer to",
			"documentation at",
			"https://",
			"link:",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\*\*Reference\*\*:`),
			regexp.MustCompile(`(?i)see (also|the documentation)`),
			regexp.MustCompile(`https?://[^\s]+`),
		},
		MinScore: 0.4,
	},
}

// ExtractionResult represents a potential knowledge extraction.
type ExtractionResult struct {
	// Type is the identified knowledge type.
	Type types.KnowledgeType

	// Score is the extraction confidence (0.0-1.0).
	Score float64

	// MatchedKeyword is which keyword triggered the match.
	MatchedKeyword string

	// MatchedPattern is which pattern matched (if any).
	MatchedPattern string

	// StartIndex is where in the content the match begins.
	StartIndex int

	// EndIndex is where the match ends.
	EndIndex int
}

// Extractor identifies knowledge patterns in messages.
type Extractor struct {
	// Patterns are the extraction patterns to use.
	Patterns []ExtractionPattern
}

// NewExtractor creates an extractor with default patterns.
func NewExtractor() *Extractor {
	return &Extractor{
		Patterns: DefaultPatterns,
	}
}

// Extract identifies potential knowledge in a message.
func (e *Extractor) Extract(msg types.TranscriptMessage) []ExtractionResult {
	if msg.Content == "" {
		return nil
	}

	var results []ExtractionResult
	for _, pattern := range e.Patterns {
		results = append(results, matchPattern(msg.Content, pattern)...)
	}

	return deduplicateByType(results)
}

// matchPattern returns extraction results from keyword and regex matching for a single pattern.
func matchPattern(content string, pattern ExtractionPattern) []ExtractionResult {
	var results []ExtractionResult
	for _, keyword := range pattern.Keywords {
		if idx := strings.Index(strings.ToLower(content), strings.ToLower(keyword)); idx >= 0 {
			results = append(results, ExtractionResult{
				Type:           pattern.Type,
				Score:          pattern.MinScore + 0.1,
				MatchedKeyword: keyword,
				StartIndex:     idx,
				EndIndex:       idx + len(keyword),
			})
			break
		}
	}
	for _, re := range pattern.Patterns {
		if loc := re.FindStringIndex(content); loc != nil {
			results = append(results, ExtractionResult{
				Type:           pattern.Type,
				Score:          pattern.MinScore + 0.2,
				MatchedPattern: re.String(),
				StartIndex:     loc[0],
				EndIndex:       loc[1],
			})
			break
		}
	}
	return results
}

// deduplicateByType keeps only the highest-scoring result per knowledge type.
func deduplicateByType(results []ExtractionResult) []ExtractionResult {
	seen := make(map[types.KnowledgeType]ExtractionResult)
	for _, r := range results {
		if existing, ok := seen[r.Type]; !ok || r.Score > existing.Score {
			seen[r.Type] = r
		}
	}
	final := make([]ExtractionResult, 0, len(seen))
	for _, r := range seen {
		final = append(final, r)
	}
	return final
}

// ExtractBest returns the single best extraction from a message, or nil if none.
func (e *Extractor) ExtractBest(msg types.TranscriptMessage) *ExtractionResult {
	results := e.Extract(msg)
	if len(results) == 0 {
		return nil
	}

	best := results[0]
	for _, r := range results[1:] {
		if r.Score > best.Score {
			best = r
		}
	}
	return &best
}
