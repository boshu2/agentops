package lifecycle

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestStatusToMaturity(t *testing.T) {
	cases := map[string]types.Maturity{
		"completed":   types.MaturityEstablished,
		"in_progress": types.MaturityCandidate,
		"pending":     types.MaturityProvisional,
		"unknown":     types.MaturityProvisional,
		"":            types.MaturityProvisional,
	}
	for status, want := range cases {
		if got := StatusToMaturity(status); got != want {
			t.Errorf("StatusToMaturity(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestCloneStringAnyMap(t *testing.T) {
	if got := CloneStringAnyMap(nil); got != nil {
		t.Error("nil in should return nil")
	}
	if got := CloneStringAnyMap(map[string]any{}); got != nil {
		t.Error("empty in should return nil")
	}

	src := map[string]any{"a": 1, "b": "two"}
	clone := CloneStringAnyMap(src)
	if len(clone) != 2 {
		t.Errorf("clone len = %d", len(clone))
	}
	clone["a"] = 999
	if src["a"] != 1 {
		t.Error("clone should not alias source")
	}
}

func TestExtractContentBlocks(t *testing.T) {
	// Valid tool_use block
	data := map[string]any{
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "text", "text": "hi"},
				map[string]any{"type": "tool_use", "name": "T"},
				map[string]any{"type": "tool_use", "name": "U"},
			},
		},
	}
	blocks := ExtractContentBlocks(data)
	if len(blocks) != 2 {
		t.Errorf("got %d blocks, want 2", len(blocks))
	}

	// No message
	if got := ExtractContentBlocks(map[string]any{}); got != nil {
		t.Error("no message should return nil")
	}

	// Malformed content
	bad := map[string]any{"message": map[string]any{"content": "not a list"}}
	if got := ExtractContentBlocks(bad); got != nil {
		t.Error("non-list content should return nil")
	}
}

func TestParseTaskCreate(t *testing.T) {
	in := map[string]any{
		"subject":     "fix bug",
		"description": "investigate foo",
		"activeForm":  "Fixing bug",
		"metadata":    map[string]any{"priority": "high"},
	}
	subject, desc, active, meta := ParseTaskCreate(in)
	if subject != "fix bug" {
		t.Errorf("subject = %q", subject)
	}
	if desc != "investigate foo" {
		t.Errorf("desc = %q", desc)
	}
	if active != "Fixing bug" {
		t.Errorf("active = %q", active)
	}
	if meta["priority"] != "high" {
		t.Errorf("meta = %v", meta)
	}

	// Empty subject short-circuits
	subject2, _, _, _ := ParseTaskCreate(map[string]any{"subject": ""})
	if subject2 != "" {
		t.Error("empty subject should be empty")
	}
}

func TestApplyTaskUpdate(t *testing.T) {
	in := map[string]any{
		"status":      "completed",
		"subject":     "new subject",
		"description": "new desc",
		"owner":       "alice",
	}
	status, subj, desc, owner := ApplyTaskUpdate(in)
	if status != "completed" || subj != "new subject" || desc != "new desc" || owner != "alice" {
		t.Errorf("got status=%q subj=%q desc=%q owner=%q", status, subj, desc, owner)
	}

	// Partial input
	s2, _, _, _ := ApplyTaskUpdate(map[string]any{})
	if s2 != "" {
		t.Errorf("empty map should give empty status, got %q", s2)
	}
}

func TestProcessTranscriptLine(t *testing.T) {
	line := `{"sessionId":"s1","message":{"content":[{"type":"tool_use","name":"T"}]}}`
	newSID, blocks := ProcessTranscriptLine(line, "", "")
	if newSID != "s1" {
		t.Errorf("sessionId = %q", newSID)
	}
	if len(blocks) != 1 {
		t.Errorf("blocks len = %d", len(blocks))
	}

	// Filter by different session -> no blocks returned
	_, blocks2 := ProcessTranscriptLine(line, "other", "")
	if len(blocks2) != 0 {
		t.Errorf("filtered-out session should yield 0 blocks, got %d", len(blocks2))
	}

	// Invalid JSON preserves state
	sid3, blocks3 := ProcessTranscriptLine("not json", "", "prev")
	if sid3 != "prev" {
		t.Errorf("invalid json should preserve currentSessionID, got %q", sid3)
	}
	if blocks3 != nil {
		t.Errorf("invalid json should return nil blocks")
	}
}

func TestComputeTaskDistributions(t *testing.T) {
	statuses := []string{"pending", "pending", "completed"}
	maturities := []types.Maturity{types.MaturityProvisional, types.MaturityEstablished, types.MaturityEstablished}
	learningIDs := []string{"l1", "", "l2"}

	d := ComputeTaskDistributions(statuses, maturities, learningIDs)
	if d.StatusCounts["pending"] != 2 {
		t.Errorf("pending count = %d", d.StatusCounts["pending"])
	}
	if d.StatusCounts["completed"] != 1 {
		t.Errorf("completed count = %d", d.StatusCounts["completed"])
	}
	if d.MaturityCounts[types.MaturityEstablished] != 2 {
		t.Errorf("established count = %d", d.MaturityCounts[types.MaturityEstablished])
	}
	if d.WithLearnings != 2 {
		t.Errorf("WithLearnings = %d (expected 2; empty string excluded)", d.WithLearnings)
	}
}
