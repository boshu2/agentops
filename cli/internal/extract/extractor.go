package extract

// This file implements Extract: the typed structured-output extraction pass.
// It chunks input text above a budget, calls the LLM client with the compiled
// schema per chunk, parses each chunk's structured JSON into the template's
// typed shape, FILTERS unparseable / empty ("None") chunk results while
// preserving index alignment of the survivors (the _filter_none_results trick
// from the Hyper-Extract steal), and merges the survivors.
//
// Design: .agents/plans/2026-06-17-native-structured-extraction.md.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// DefaultChunkBudget is the per-chunk character budget when none is supplied.
// It is a conservative character count (not tokens) used purely to split large
// inputs; the real model context budget is advisory.
const DefaultChunkBudget = 8000

// Record is one extracted object — an entity or a relation. Fields are dynamic
// (driven by the template's typed Output), so a record is a string->value map.
type Record map[string]any

// Result is the merged, typed extraction output: the entities and relations
// surfaced across all surviving chunks, in chunk order.
type Result struct {
	// Entities is the merged list of extracted entity records.
	Entities []Record
	// Relations is the merged list of extracted relation records.
	Relations []Record
	// SurvivingChunks lists the 0-based source-chunk indices whose output
	// parsed successfully (index alignment of survivors is preserved here).
	SurvivingChunks []int
}

// chunkOutput is the per-chunk structured JSON shape the model emits, matching
// the schema CompileSchema produces.
type chunkOutput struct {
	Entities  []Record `json:"entities"`
	Relations []Record `json:"relations"`
}

// Chunk splits text into <=budget-sized character chunks, preferring to break
// on whitespace boundaries so a chunk does not split mid-word where avoidable.
// Input within budget yields a single chunk. budget<=0 uses DefaultChunkBudget.
func Chunk(text string, budget int) []string {
	if budget <= 0 {
		budget = DefaultChunkBudget
	}
	runes := []rune(text)
	if len(runes) <= budget {
		return []string{text}
	}
	var chunks []string
	for start := 0; start < len(runes); {
		end := start + budget
		if end >= len(runes) {
			chunks = append(chunks, string(runes[start:]))
			break
		}
		// Prefer the last whitespace within [start,end) so we break cleanly.
		split := end
		for i := end; i > start; i-- {
			if runes[i-1] == ' ' || runes[i-1] == '\n' || runes[i-1] == '\t' {
				split = i
				break
			}
		}
		chunks = append(chunks, string(runes[start:split]))
		start = split
	}
	return chunks
}

// extractPrompt renders the per-chunk extraction prompt: the template's
// guideline (HOW) plus the chunk text. The model is told to return ONLY the
// structured JSON object the schema constrains.
func extractPrompt(tmpl *Template, chunk string) string {
	var b strings.Builder
	b.WriteString(tmpl.Guideline)
	b.WriteString("\n\nReturn ONLY a JSON object with \"entities\" and \"relations\" arrays matching the provided schema. Only include items that literally appear in the text below; do not invent.\n\n=== BEGIN TEXT ===\n")
	b.WriteString(chunk)
	b.WriteString("\n=== END TEXT ===")
	return b.String()
}

// parseChunkResult parses one chunk's raw model output into a chunkOutput.
// codex/ollama may wrap the JSON in prose, so the first {...last } object is
// extracted. An empty / unparseable result returns ok=false (a "None" result to
// be filtered), NOT an error — malformed output is filtered, not fatal.
func parseChunkResult(raw string) (chunkOutput, bool) {
	stripped := strings.TrimSpace(raw)
	if stripped == "" {
		return chunkOutput{}, false
	}
	start := strings.Index(stripped, "{")
	end := strings.LastIndex(stripped, "}")
	if start < 0 || end <= start {
		return chunkOutput{}, false
	}
	var out chunkOutput
	if err := json.Unmarshal([]byte(stripped[start:end+1]), &out); err != nil {
		return chunkOutput{}, false
	}
	// A successfully-parsed object with zero entities AND zero relations is a
	// genuine empty result; it still "survives" (the chunk was processed) but
	// contributes nothing. We keep it as a survivor so index alignment counts
	// it, matching _filter_none_results (which filters only None/unparseable).
	return out, true
}

// Extract runs the typed extraction over text using template and client. It
// chunks the input, calls the client with the compiled schema per chunk, parses
// each result, filters unparseable chunk results (preserving the surviving
// chunks' index alignment), and merges the survivors into a typed Result.
//
// A nil error is returned even when some chunks fail to parse — those are
// dropped. An error is returned only for a setup failure (nil args, schema
// compile failure, or a hard client/transport error).
func Extract(ctx context.Context, text string, tmpl *Template, client *Client) (*Result, error) {
	return extractWithBudgetCtx(ctx, client, text, tmpl, DefaultChunkBudget)
}

// extractWithBudget is the budget-parameterized core, exported within the
// package so tests can force a deterministic chunk count. It uses a background
// context.
func extractWithBudget(client *Client, text string, tmpl *Template, budget int) (*Result, error) {
	return extractWithBudgetCtx(context.Background(), client, text, tmpl, budget)
}

func extractWithBudgetCtx(ctx context.Context, client *Client, text string, tmpl *Template, budget int) (*Result, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("extract: nil template")
	}
	if client == nil {
		return nil, fmt.Errorf("extract: nil client")
	}
	schema, err := CompileSchema(tmpl)
	if err != nil {
		return nil, fmt.Errorf("extract: compile schema: %w", err)
	}

	chunks := Chunk(text, budget)
	result := &Result{}
	for idx, chunk := range chunks {
		prompt := extractPrompt(tmpl, chunk)
		raw, err := client.Generate(ctx, prompt, schema)
		if err != nil {
			// A hard transport error is fatal — we cannot distinguish it from a
			// systematic failure, and silently dropping every chunk would mask it.
			return nil, fmt.Errorf("extract: chunk %d generate: %w", idx, err)
		}
		parsed, ok := parseChunkResult(raw)
		if !ok {
			// Unparseable / None result — filter it, preserve alignment of the
			// rest (we simply do not record idx as a survivor).
			continue
		}
		result.SurvivingChunks = append(result.SurvivingChunks, idx)
		result.Entities = append(result.Entities, parsed.Entities...)
		result.Relations = append(result.Relations, parsed.Relations...)
	}
	return result, nil
}
