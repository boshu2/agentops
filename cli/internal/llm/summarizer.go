package llm

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// PromptTemplate is the empirically validated prompt from the W0-6 spike
// (.agents/rpi/spike-2026-04-11-gemma4-output-shape.md — 3/3 PASS on real
// nami session chunks with gemma2:9b). DO NOT edit this without re-running
// the spike on a representative chunk corpus; the spike validated this exact
// wording and the scoring contract depends on the section headers.
const PromptTemplate = `You are reviewing one turn of a Claude Code session transcript. Your job is to
produce structured notes for a knowledge wiki. The user controls the wiki via
markdown files and wikilinks; no embeddings.

INPUT:
%s

OUTPUT (strict markdown, no JSON):

### Intent
<1-5 word label>

### Summary
<1-3 sentences>

### Entities
- [[file:<path>]] (for each file path mentioned)
- [[bead:<id>]] (for each bead ID mentioned)
- [[concept:<slug>]] (for each named concept)

### Assistant condensed
<1-3 sentence paraphrase of what the assistant did/said>

Do NOT include raw error messages, tool invocations, or code blocks longer
than 3 lines. If the turn has no substantive content, output exactly "SKIP".

Produce the output NOW. Start with "### Intent" on the first line. No preamble.`

// Generator is the narrow interface Summarizer needs from an LLM backend.
// Satisfied by *OllamaClient and by test fakes.
type Generator interface {
	Generate(prompt string) (string, error)
	Digest() string
	ContextBudget() int
	ModelName() string
}

// ModelName adds the missing method to OllamaClient so it satisfies Generator.
func (c *OllamaClient) ModelName() string { return c.model }

// ChunkNote is the structured result of summarizing one TurnChunk.
type ChunkNote struct {
	// Index is the source chunk's 0-based position in the session.
	Index int

	// Skipped is true when the LLM returned exactly "SKIP" (non-substantive turn).
	Skipped bool

	// Intent is the 1-5 word label for what the turn was about.
	Intent string

	// Summary is the 1-3 sentence summary of the turn.
	Summary string

	// Entities is the list of wikilink targets extracted, in "file:/path",
	// "bead:id", "concept:slug" form (outer [[ ]] already stripped).
	Entities []string

	// AssistantCondensed is the 1-3 sentence paraphrase of the assistant response.
	AssistantCondensed string

	// Raw is the original LLM output, retained for debugging / audit trail.
	Raw string
}

// Summarizer turns TurnChunks into ChunkNotes by calling the LLM with the
// spike-validated PromptTemplate and parsing the strict-markdown output.
type Summarizer struct {
	llm Generator
}

// NewSummarizer constructs a Summarizer bound to an LLM backend.
func NewSummarizer(llm Generator) *Summarizer {
	return &Summarizer{llm: llm}
}

// SummarizeChunk renders the chunk into PromptTemplate, calls the LLM once,
// tolerates a wrapping ```markdown fence, and parses the result into a
// ChunkNote. Returns an error if any of the required sections are missing.
func (s *Summarizer) SummarizeChunk(chunk TurnChunk) (ChunkNote, error) {
	prompt := fmt.Sprintf(PromptTemplate, chunk.Prompt())
	raw, err := s.llm.Generate(prompt)
	if err != nil {
		return ChunkNote{}, fmt.Errorf("llm generate chunk %d: %w", chunk.Index, err)
	}

	stripped := strings.TrimSpace(raw)
	stripped = stripCodeFence(stripped)

	if stripped == "SKIP" {
		return ChunkNote{Index: chunk.Index, Skipped: true, Raw: raw}, nil
	}

	note := ChunkNote{Index: chunk.Index, Raw: raw}
	if err := parseStrictMarkdown(stripped, &note); err != nil {
		return ChunkNote{}, fmt.Errorf("parse chunk %d output: %w\nraw: %s", chunk.Index, err, truncate(raw, 400))
	}
	return note, nil
}

// stripCodeFence removes a leading ```markdown (or ```) fence and trailing
// ``` if present, matching the spike's score_output() tolerance.
func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	// Drop the first line (which is the opening fence).
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		s = s[nl+1:]
	}
	// Drop trailing fence.
	s = strings.TrimRight(s, " \n")
	if strings.HasSuffix(s, "```") {
		s = s[:len(s)-3]
	}
	return strings.TrimSpace(s)
}

var sectionSplitRe = regexp.MustCompile(`(?m)^###\s+`)

// parseStrictMarkdown walks the four expected sections in order and fills
// the note. Returns an error if any required section is missing.
func parseStrictMarkdown(body string, note *ChunkNote) error {
	required := map[string]bool{
		"Intent":              false,
		"Summary":             false,
		"Entities":            false,
		"Assistant condensed": false,
	}
	parts := sectionSplitRe.Split(body, -1)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// First line is the header name, rest is body.
		nl := strings.IndexByte(part, '\n')
		var header, content string
		if nl < 0 {
			header = part
			content = ""
		} else {
			header = strings.TrimSpace(part[:nl])
			content = strings.TrimSpace(part[nl+1:])
		}
		switch header {
		case "Intent":
			note.Intent = content
			required["Intent"] = true
		case "Summary":
			note.Summary = content
			required["Summary"] = true
		case "Entities":
			note.Entities = parseEntityList(content)
			required["Entities"] = true
		case "Assistant condensed":
			note.AssistantCondensed = content
			required["Assistant condensed"] = true
		}
	}
	var missing []string
	for k, ok := range required {
		if !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing sections: %v", missing)
	}
	if note.Intent == "" {
		return errors.New("Intent section is empty")
	}
	return nil
}

var wikilinkRe = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// parseEntityList extracts [[...]] wikilinks from an Entities section body.
// Returns each match with outer brackets stripped, preserving insertion order.
func parseEntityList(body string) []string {
	matches := wikilinkRe.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 && strings.TrimSpace(m[1]) != "" {
			out = append(out, strings.TrimSpace(m[1]))
		}
	}
	return out
}
