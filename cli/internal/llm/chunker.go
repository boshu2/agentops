package llm

import (
	"strings"

	"github.com/boshu2/agentops/cli/internal/types"
)

// DefaultMaxChars is the fallback chunk budget when callers pass maxChars <= 0.
// Chosen conservatively: the W0-6 spike validated 683-1591 char chunks cleanly
// on gemma2:9b; 2000 gives headroom without pushing into the larger-chunk
// degradation zone flagged by the spike's risk section.
const DefaultMaxChars = 2000

// minMessageChars is the short-message filter threshold lifted from the
// tool-noise filter spec. Messages shorter than this after content extraction
// are dropped as noise before turn assembly.
const minMessageChars = 20

// ChunkTurns groups a flat TranscriptMessage sequence into user→assistant
// turn chunks, applying the Level 1 tool-noise filter (keep only user/
// assistant roles, drop sub-minMessageChars messages) and a per-chunk char
// budget.
//
// maxChars caps the total UserText+AssistantText for each chunk; when a pair
// exceeds the budget, each side is truncated proportionally (40/60 favoring
// assistant, matching the spike's extract_chunks.py behavior). Callers should
// use the redactor BEFORE chunking (critical per pre-mortem F3); this
// function is a pure structural transform.
//
// Orphan messages (a user with no following assistant, or vice versa) are
// silently dropped. The returned slice is empty (not nil) when no valid
// turns exist.
func ChunkTurns(msgs []types.TranscriptMessage, maxChars int) []TurnChunk {
	if maxChars <= 0 {
		maxChars = DefaultMaxChars
	}

	// Step 1: filter to substantive user/assistant messages only.
	filtered := make([]types.TranscriptMessage, 0, len(msgs))
	for _, m := range msgs {
		role := strings.ToLower(m.Type)
		if role != "user" && role != "assistant" {
			// tool_use, tool_result, thinking, system, and other types are
			// dropped at this stage — they're handled separately (if at all)
			// by the summarizer.
			continue
		}
		content := strings.TrimSpace(m.Content)
		if len(content) < minMessageChars {
			continue
		}
		// Copy with trimmed content so the chunker owns its view.
		c := m
		c.Content = content
		filtered = append(filtered, c)
	}

	// Step 2: pair up user→assistant. Orphans drop.
	chunks := make([]TurnChunk, 0, len(filtered)/2)
	i := 0
	for i < len(filtered)-1 {
		u := filtered[i]
		a := filtered[i+1]
		if strings.ToLower(u.Type) == "user" && strings.ToLower(a.Type) == "assistant" {
			chunks = append(chunks, buildChunk(len(chunks), u, a, maxChars))
			i += 2
			continue
		}
		// Not a valid user→assistant boundary; advance one and retry.
		i++
	}
	return chunks
}

// buildChunk assembles one TurnChunk from a user/assistant pair, honoring the
// budget via proportional truncation.
func buildChunk(index int, u, a types.TranscriptMessage, maxChars int) TurnChunk {
	userText := u.Content
	assistantText := a.Content

	total := len(userText) + len(assistantText)
	if total > maxChars {
		// 40% user, 60% assistant — matches the spike extractor.
		uBudget := maxChars * 40 / 100
		aBudget := maxChars - uBudget
		userText = truncate(userText, uBudget)
		assistantText = truncate(assistantText, aBudget)
	}

	return TurnChunk{
		Index:         index,
		UserText:      userText,
		AssistantText: assistantText,
		Chars:         len(userText) + len(assistantText),
		StartIdx:      u.MessageIndex,
		EndIdx:        a.MessageIndex,
	}
}

// truncate returns s cut to at most n bytes. When truncation happens, an
// ellipsis marker is appended (within the budget) so downstream summarizers
// can see the cut without exceeding the allowed length.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	const marker = "…"
	if n <= len(marker) {
		return s[:n]
	}
	return strings.TrimRightFunc(s[:n-len(marker)], isSpaceOrNewline) + marker
}

func isSpaceOrNewline(r rune) bool {
	return r == ' ' || r == '\n' || r == '\t' || r == '\r'
}
