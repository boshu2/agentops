package llm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// PromptTemplate is aligned with the proven JS worker prompt on bushido
// (dream/worker.js INGEST_PROMPT_TEMPLATE) which has produced 9K+ successful
// session ingests with gemma4:e4b. The ollama client sends format:"json" so
// this prompt instructs the model to return a JSON object.
//
// The original spike prompt (gemma2:9b, strict-markdown) is preserved in
// .agents/rpi/spike-2026-04-11-gemma4-output-shape.md for reference but is
// NOT used in production because gemma4:e4b requires format:json mode.
const PromptTemplate = `You are a knowledge extractor (Tier 1). Below is a Claude Code session excerpt. Your job is NOT to summarize what happened — your job is to extract what was LEARNED, DECIDED, or DISCOVERED.

Ask yourself:
- What decision was made and WHY? (not "the user asked X" but "they chose Y over Z because...")
- What did the agent learn that wasn't known before? (a new pattern, a bug root cause, a config that works)
- What failed and what was the lesson?
- What reusable knowledge could help a future session?

If the session is just routine file reads with no decisions or learnings, say so honestly in the summary. Don't inflate thin sessions.

Return ONLY a JSON object with these exact keys:
- title: the key insight or decision in ≤12 words (NOT "Reading files" or "Working on X" — state the FINDING)
- summary: 2-4 sentences focused on WHAT WAS LEARNED OR DECIDED, not what tools were called
- entities: array of domain-specific proper nouns (file paths, epic IDs, service names, config keys). Exclude generic tool names like Read, Grep, Bash.
- concepts: array of technical patterns, architectural ideas, or reusable approaches observed
- decisions: array of explicit choices made with brief rationale ("chose X because Y")
- open_questions: array of unresolved items or things that need follow-up
- work_phase: one of research, plan, implement, verify, post-mortem, other

Only include entities/concepts that literally appear in the source text. Do not invent. Use empty arrays for fields with no content. If a session has no real decisions or learnings, title it honestly (e.g., "Routine file inspection with no decisions").

=== BEGIN TRANSCRIPT ===
%s
=== END TRANSCRIPT ===`

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

// ingestResponse is the JSON shape gemma4:e4b returns when format:json is set.
type ingestResponse struct {
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	Entities      []string `json:"entities"`
	Concepts      []string `json:"concepts"`
	Decisions     []string `json:"decisions"`
	OpenQuestions []string `json:"open_questions"`
	WorkPhase     string   `json:"work_phase"`
}

// ChunkNote is the structured result of summarizing one TurnChunk.
type ChunkNote struct {
	// Index is the source chunk's 0-based position in the session.
	Index int

	// Skipped is true when the LLM returned a thin/empty response.
	Skipped bool

	// Intent is the key insight or decision label.
	Intent string

	// Summary is the 2-4 sentence summary focused on learnings/decisions.
	Summary string

	// Entities is the list of domain-specific proper nouns extracted.
	Entities []string

	// AssistantCondensed is the condensed assistant action (derived from
	// decisions + summary for backward compat with the session writer).
	AssistantCondensed string

	// Concepts is the list of technical patterns/approaches observed.
	Concepts []string

	// Decisions is the list of explicit choices with rationale.
	Decisions []string

	// OpenQuestions is unresolved items needing follow-up.
	OpenQuestions []string

	// WorkPhase classifies the turn (research, plan, implement, etc).
	WorkPhase string

	// Raw is the original LLM output, retained for debugging / audit trail.
	Raw string
}

// Summarizer turns TurnChunks into ChunkNotes by calling the LLM with the
// proven PromptTemplate and parsing the JSON output.
type Summarizer struct {
	llm Generator
}

// NewSummarizer constructs a Summarizer bound to an LLM backend.
func NewSummarizer(llm Generator) *Summarizer {
	return &Summarizer{llm: llm}
}

func renderPrompt(template, input string) string {
	return strings.Replace(template, "%s", input, 1)
}

// SummarizeChunk renders the chunk into PromptTemplate, calls the LLM once,
// and parses the JSON result into a ChunkNote.
func (s *Summarizer) SummarizeChunk(chunk TurnChunk) (ChunkNote, error) {
	prompt := renderPrompt(PromptTemplate, chunk.Prompt())
	raw, err := s.llm.Generate(prompt)
	if err != nil {
		return ChunkNote{}, fmt.Errorf("llm generate chunk %d: %w", chunk.Index, err)
	}

	stripped := strings.TrimSpace(raw)
	if stripped == "" || stripped == "SKIP" || stripped == `{"skip":true}` {
		return ChunkNote{Index: chunk.Index, Skipped: true, Raw: raw}, nil
	}

	// Try JSON parse first (expected with format:json mode).
	note, err := parseJSONResponse(stripped, chunk.Index)
	if err == nil {
		note.Raw = raw
		return note, nil
	}

	// Fallback: try strict-markdown parse (for backward compat with
	// gemma2:9b spike prompt or models that ignore format:json).
	stripped = stripCodeFence(stripped)
	note2 := ChunkNote{Index: chunk.Index, Raw: raw}
	if err2 := parseStrictMarkdown(stripped, &note2); err2 == nil {
		return note2, nil
	}

	// Both parsers failed — return the JSON error (primary path).
	return ChunkNote{}, fmt.Errorf("parse chunk %d: %w\nraw: %s", chunk.Index, err, truncate(raw, 400))
}

// parseJSONResponse decodes the ingestResponse JSON shape.
func parseJSONResponse(raw string, index int) (ChunkNote, error) {
	var resp ingestResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return ChunkNote{}, fmt.Errorf("json decode: %w", err)
	}
	if resp.Title == "" && resp.Summary == "" {
		return ChunkNote{Index: index, Skipped: true}, nil
	}
	// Build AssistantCondensed from decisions if available, else summary.
	condensed := resp.Summary
	if len(resp.Decisions) > 0 {
		condensed = strings.Join(resp.Decisions, "; ")
	}
	return ChunkNote{
		Index:              index,
		Intent:             resp.Title,
		Summary:            resp.Summary,
		Entities:           resp.Entities,
		Concepts:           resp.Concepts,
		Decisions:          resp.Decisions,
		OpenQuestions:      resp.OpenQuestions,
		WorkPhase:          resp.WorkPhase,
		AssistantCondensed: condensed,
	}, nil
}

// stripCodeFence removes a leading ```markdown (or ```) fence and trailing
// ``` if present (backward compat with gemma2:9b).
func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		s = s[nl+1:]
	}
	s = strings.TrimRight(s, " \n")
	if strings.HasSuffix(s, "```") {
		s = s[:len(s)-3]
	}
	return strings.TrimSpace(s)
}

var sectionSplitRe = regexp.MustCompile(`(?m)^###\s+`)

// parseStrictMarkdown walks the four expected sections in order and fills
// the note. Returns an error if any required section is missing.
// Kept as fallback for gemma2:9b-style markdown output.
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
	return nil
}

var wikilinkRe = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// parseEntityList extracts [[...]] wikilinks from an Entities section body.
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
