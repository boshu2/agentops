package llmwiki

import (
	"context"
	"sort"
	"strings"
	"unicode"
)

// Compiler distills an ingested source body into the three derived signals the
// OpenKB-style CompileStage turns into wiki artifacts: a one-paragraph summary,
// a set of concepts, and a set of named entities.
//
// Implementations MUST be deterministic for a given input (the CompileStage
// relies on byte-stable output for its idempotency contract) and MUST honor
// ctx cancellation. The production implementation will wrap a real model call;
// the in-tree DeterministicCompiler derives everything mechanically so the
// pipeline is testable without an LLM.
type Compiler interface {
	// Summarize returns a short prose summary of source.
	Summarize(ctx context.Context, source string) (string, error)
	// Concepts returns the distinct concepts present in source. Order is not
	// guaranteed; callers that need stable output must sort.
	Concepts(ctx context.Context, source string) ([]string, error)
	// Entities returns the distinct named entities mentioned in source. Order
	// is not guaranteed; callers that need stable output must sort.
	Entities(ctx context.Context, source string) ([]string, error)
}

// DeterministicCompiler is a fake Compiler that derives summary/concepts/
// entities mechanically from the input text. It is used by tests and as the
// safe default for the CompileStage so the pipeline produces stable artifacts
// without invoking a model.
//
// Extraction rules (all deterministic):
//   - Summary: the first non-empty, non-heading line of the body.
//   - Concepts: the text of every markdown heading (# / ## / ###...).
//   - Entities: capitalized multi-character words that are not heading text,
//     deduplicated and sorted, excluding a small stop set.
type DeterministicCompiler struct{}

// NewDeterministicCompiler returns a ready-to-use DeterministicCompiler.
func NewDeterministicCompiler() *DeterministicCompiler {
	return &DeterministicCompiler{}
}

// entityStopWords are common capitalized sentence-starters we do not want to
// treat as named entities. Kept small and explicit.
var entityStopWords = map[string]bool{
	"The": true, "A": true, "An": true, "This": true, "That": true,
	"These": true, "Those": true, "It": true, "We": true, "You": true,
	"They": true, "I": true, "He": true, "She": true,
}

// Summarize returns the first non-empty, non-heading line of source. When the
// body is empty (or all headings) it returns an empty string.
func (c *DeterministicCompiler) Summarize(ctx context.Context, source string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return trimmed, nil
	}
	return "", nil
}

// Concepts returns the text of every markdown heading in source, deduplicated.
func (c *DeterministicCompiler) Concepts(ctx context.Context, source string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if heading == "" || seen[heading] {
			continue
		}
		seen[heading] = true
		out = append(out, heading)
	}
	sort.Strings(out)
	return out, nil
}

// Entities returns the distinct capitalized words in source's non-heading body,
// excluding stop words, deduplicated and sorted.
func (c *DeterministicCompiler) Entities(ctx context.Context, source string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue // skip heading text — those become concepts, not entities
		}
		for _, word := range strings.FieldsFunc(trimmed, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		}) {
			if !isCapitalizedEntity(word) || entityStopWords[word] {
				continue
			}
			if seen[word] {
				continue
			}
			seen[word] = true
			out = append(out, word)
		}
	}
	sort.Strings(out)
	return out, nil
}

// isCapitalizedEntity reports whether word starts with an uppercase letter and
// has length >= 2 (single letters are too noisy to treat as entities).
func isCapitalizedEntity(word string) bool {
	runes := []rune(word)
	if len(runes) < 2 {
		return false
	}
	return unicode.IsUpper(runes[0])
}
