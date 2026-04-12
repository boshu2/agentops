package llm

import (
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/types"
)

// msg is a compact test helper for building TranscriptMessage fixtures.
func msg(idx int, msgType, content string) types.TranscriptMessage {
	return types.TranscriptMessage{
		Type:         msgType,
		Role:         msgType,
		Content:      content,
		MessageIndex: idx,
	}
}

func TestChunkTurns_EmptyInput(t *testing.T) {
	got := ChunkTurns(nil, 2000)
	if len(got) != 0 {
		t.Fatalf("nil input: want 0 chunks, got %d", len(got))
	}
	got = ChunkTurns([]types.TranscriptMessage{}, 2000)
	if len(got) != 0 {
		t.Fatalf("empty slice: want 0 chunks, got %d", len(got))
	}
}

func TestChunkTurns_SingleUserAssistantPair(t *testing.T) {
	msgs := []types.TranscriptMessage{
		msg(0, "user", "hello can you read main.go"),
		msg(1, "assistant", "sure, reading main.go now and looking at the entry point"),
	}
	chunks := ChunkTurns(msgs, 2000)
	if len(chunks) != 1 {
		t.Fatalf("want 1 chunk, got %d", len(chunks))
	}
	c := chunks[0]
	if c.Index != 0 {
		t.Errorf("Index: want 0, got %d", c.Index)
	}
	if !strings.Contains(c.UserText, "hello") {
		t.Errorf("UserText missing user content: %q", c.UserText)
	}
	if !strings.Contains(c.AssistantText, "main.go") {
		t.Errorf("AssistantText missing assistant content: %q", c.AssistantText)
	}
	if c.Chars != len(c.UserText)+len(c.AssistantText) {
		t.Errorf("Chars %d != sum of texts (%d + %d)", c.Chars, len(c.UserText), len(c.AssistantText))
	}
	if c.StartIdx != 0 || c.EndIdx != 1 {
		t.Errorf("StartIdx/EndIdx: want 0/1, got %d/%d", c.StartIdx, c.EndIdx)
	}
}

func TestChunkTurns_MultiplePairs(t *testing.T) {
	msgs := []types.TranscriptMessage{
		msg(0, "user", "first user message with enough content to pass the minimum length filter"),
		msg(1, "assistant", "first assistant reply body that is substantial enough to survive filtering"),
		msg(2, "user", "second user message with enough content to pass the minimum length filter"),
		msg(3, "assistant", "second assistant reply body that is substantial enough to survive filtering"),
		msg(4, "user", "third user message with enough content to pass the minimum length filter"),
		msg(5, "assistant", "third assistant reply body that is substantial enough to survive filtering"),
	}
	chunks := ChunkTurns(msgs, 2000)
	if len(chunks) != 3 {
		t.Fatalf("want 3 chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk %d: Index %d, want %d", i, c.Index, i)
		}
	}
}

func TestChunkTurns_BudgetTruncatesOversized(t *testing.T) {
	long := strings.Repeat("x", 6000)
	msgs := []types.TranscriptMessage{
		msg(0, "user", long),
		msg(1, "assistant", long),
	}
	chunks := ChunkTurns(msgs, 2000)
	if len(chunks) != 1 {
		t.Fatalf("oversized pair should still produce 1 chunk, got %d", len(chunks))
	}
	c := chunks[0]
	if c.Chars > 2000 {
		t.Errorf("Chars %d exceeds budget 2000", c.Chars)
	}
	if c.UserText == "" || c.AssistantText == "" {
		t.Errorf("truncation dropped a side: user=%d assistant=%d", len(c.UserText), len(c.AssistantText))
	}
}

func TestChunkTurns_BudgetBoundaries(t *testing.T) {
	// Same pair, three budgets; each should honor the budget.
	pair := []types.TranscriptMessage{
		msg(0, "user", strings.Repeat("u", 5000)),
		msg(1, "assistant", strings.Repeat("a", 5000)),
	}
	for _, budget := range []int{500, 2000, 4000} {
		chunks := ChunkTurns(pair, budget)
		if len(chunks) != 1 {
			t.Fatalf("budget=%d: want 1 chunk, got %d", budget, len(chunks))
		}
		if chunks[0].Chars > budget {
			t.Errorf("budget=%d: Chars %d exceeds budget", budget, chunks[0].Chars)
		}
		if chunks[0].UserText == "" || chunks[0].AssistantText == "" {
			t.Errorf("budget=%d: empty side after truncation", budget)
		}
	}
}

func TestChunkTurns_DropsShortMessages(t *testing.T) {
	// <20-char messages should be dropped per the tool-noise filter spec.
	msgs := []types.TranscriptMessage{
		msg(0, "user", "hi"), // too short
		msg(1, "assistant", "short"),
		msg(2, "user", "a proper user question about the codebase"),
		msg(3, "assistant", "a proper assistant answer that covers the user's question"),
	}
	chunks := ChunkTurns(msgs, 2000)
	if len(chunks) != 1 {
		t.Fatalf("want 1 chunk (short pair dropped), got %d", len(chunks))
	}
	if !strings.Contains(chunks[0].UserText, "proper user question") {
		t.Errorf("kept the wrong user text: %q", chunks[0].UserText)
	}
}

func TestChunkTurns_OrphanUserDropped(t *testing.T) {
	// A user message with no following assistant should not produce a chunk.
	msgs := []types.TranscriptMessage{
		msg(0, "user", "this user never got an answer before session ended"),
	}
	chunks := ChunkTurns(msgs, 2000)
	if len(chunks) != 0 {
		t.Fatalf("orphan user should drop, got %d chunks", len(chunks))
	}
}

func TestChunkTurns_OrphanAssistantDropped(t *testing.T) {
	msgs := []types.TranscriptMessage{
		msg(0, "assistant", "assistant message with no prior user turn should also be dropped"),
	}
	chunks := ChunkTurns(msgs, 2000)
	if len(chunks) != 0 {
		t.Fatalf("orphan assistant should drop, got %d chunks", len(chunks))
	}
}

func TestChunkTurns_SkipsToolUseMessages(t *testing.T) {
	// Only user+assistant messages contribute text; tool_use/tool_result are filtered.
	msgs := []types.TranscriptMessage{
		msg(0, "user", "please run the tests for me in the cli directory"),
		msg(1, "tool_use", "bash command invocation metadata that should not appear"),
		msg(2, "tool_result", "tool output with raw shell data that should not leak"),
		msg(3, "assistant", "running the tests now and reporting back the results"),
	}
	chunks := ChunkTurns(msgs, 2000)
	if len(chunks) != 1 {
		t.Fatalf("want 1 chunk, got %d", len(chunks))
	}
	c := chunks[0]
	if strings.Contains(c.UserText, "tool_use") || strings.Contains(c.AssistantText, "tool_use") {
		t.Errorf("tool_use leaked into chunk: %+v", c)
	}
	if strings.Contains(c.UserText, "tool_result") || strings.Contains(c.AssistantText, "tool_result") {
		t.Errorf("tool_result leaked into chunk: %+v", c)
	}
}

func TestTurnChunk_PromptFormat(t *testing.T) {
	c := TurnChunk{UserText: "hello", AssistantText: "world"}
	got := c.Prompt()
	want := "USER:\nhello\n\nASSISTANT:\nworld"
	if got != want {
		t.Errorf("Prompt():\n got %q\nwant %q", got, want)
	}
}

func TestChunkTurns_ZeroBudgetFallsBackToDefault(t *testing.T) {
	// maxChars <= 0 should not hang or produce empty chunks; it falls back
	// to the default budget so the chunker is safe for callers that forget
	// to pass a value.
	msgs := []types.TranscriptMessage{
		msg(0, "user", "hello i have a question about the codebase please"),
		msg(1, "assistant", "sure, happy to help you with the codebase question"),
	}
	chunks := ChunkTurns(msgs, 0)
	if len(chunks) != 1 {
		t.Fatalf("want 1 chunk with default budget, got %d", len(chunks))
	}
	if chunks[0].Chars == 0 {
		t.Errorf("chunk Chars should be nonzero")
	}
}
