// Package llm contains the Tier 1 local-LLM summarization pipeline for
// translating Claude Code session JSONL transcripts into markdown session
// pages under .agents/ao/sessions/.
//
// Pipeline: parser → redactor → chunker → ollama client → summarizer → stitcher.
// v1 targets gemma2:9b via ollama HTTP API (empirically validated by
// .agents/rpi/spike-2026-04-11-gemma4-output-shape.md).
package llm

// TurnChunk is one user→assistant turn, budget-bounded, ready for per-chunk
// LLM summarization. The chunker groups raw TranscriptMessage sequences into
// these chunks; the summarizer sends each chunk to the local LLM.
type TurnChunk struct {
	// Index is the chunk's position in the session (0-based).
	Index int

	// UserText is the user message content for this turn (post-filter).
	UserText string

	// AssistantText is the assistant response content for this turn (post-filter).
	AssistantText string

	// Chars is the sum of len(UserText) + len(AssistantText). Used by the
	// chunker to enforce the maxChars budget.
	Chars int

	// StartIdx is the MessageIndex of the first message contributing to this chunk.
	StartIdx int

	// EndIdx is the MessageIndex of the last message contributing to this chunk.
	EndIdx int
}

// Prompt returns the chunk rendered for LLM input, in the exact shape the
// spike validator used (USER:/ASSISTANT:). Callers are expected to format
// this into the full prompt template (see summarizer.PromptTemplate).
func (c TurnChunk) Prompt() string {
	return "USER:\n" + c.UserText + "\n\nASSISTANT:\n" + c.AssistantText
}

