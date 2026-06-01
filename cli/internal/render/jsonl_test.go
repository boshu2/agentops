package render

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/sessionstore"
)

func TestNewJSONLFormatter(t *testing.T) {
	f := NewJSONLFormatter()
	if f == nil {
		t.Fatal("NewJSONLFormatter returned nil")
	}
	if f.Pretty {
		t.Error("Pretty should be false by default")
	}
}

func TestJSONLFormatter_Extension(t *testing.T) {
	f := NewJSONLFormatter()
	ext := f.Extension()
	if ext != ".jsonl" {
		t.Errorf("Extension() = %q, want .jsonl", ext)
	}
}

func TestJSONLFormatter_Format_FullSession(t *testing.T) {
	f := NewJSONLFormatter()
	session := &sessionstore.Session{
		ID:      "test-session-001",
		Date:    time.Date(2026, 1, 25, 10, 0, 0, 0, time.UTC),
		Summary: "Test session summary",
		Decisions: []string{
			"Use Go for implementation",
			"Follow TDD approach",
		},
		Knowledge: []string{
			"Learned about context.WithCancel",
		},
		FilesChanged: []string{
			"cmd/main.go",
			"internal/handler.go",
		},
		Issues: []string{"ol-001", "ol-002"},
		ToolCalls: map[string]int{
			"Read":  10,
			"Write": 5,
		},
		Tokens: sessionstore.TokenUsage{
			Input:  1000,
			Output: 500,
			Total:  1500,
		},
		TranscriptPath: "/path/to/transcript.jsonl",
	}

	var buf bytes.Buffer
	err := f.Format(&buf, session)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Parse the output
	var output map[string]any
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse output: %v\nOutput: %s", err, buf.String())
	}

	// Verify fields
	if output["session_id"] != "test-session-001" {
		t.Errorf("session_id = %v, want test-session-001", output["session_id"])
	}
	if output["date"] != "2026-01-25" {
		t.Errorf("date = %v, want 2026-01-25", output["date"])
	}
	if output["summary"] != "Test session summary" {
		t.Errorf("summary = %v, want Test session summary", output["summary"])
	}

	// Check array fields
	decisions := output["decisions"].([]any)
	if len(decisions) != 2 {
		t.Errorf("decisions length = %d, want 2", len(decisions))
	}

	// Check tokens
	tokens := output["tokens"].(map[string]any)
	if int(tokens["total"].(float64)) != 1500 {
		t.Errorf("tokens.total = %v, want 1500", tokens["total"])
	}
}

func TestJSONLFormatter_Format_MinimalSession(t *testing.T) {
	f := NewJSONLFormatter()
	session := &sessionstore.Session{
		ID:      "minimal-session",
		Date:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Summary: "Minimal",
	}

	var buf bytes.Buffer
	err := f.Format(&buf, session)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	var output map[string]any
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if output["session_id"] != "minimal-session" {
		t.Errorf("session_id = %v, want minimal-session", output["session_id"])
	}

	// Tokens should be omitted when zero
	if _, ok := output["tokens"]; ok {
		t.Error("tokens should be omitted for zero values")
	}
}

func TestJSONLFormatter_Format_EmptyFields(t *testing.T) {
	f := NewJSONLFormatter()
	session := &sessionstore.Session{
		ID:   "empty-fields",
		Date: time.Now(),
	}

	var buf bytes.Buffer
	err := f.Format(&buf, session)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Should still produce valid JSON
	var output map[string]any
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}
}

func TestJSONLFormatter_Format_SpecialCharacters(t *testing.T) {
	f := NewJSONLFormatter()
	session := &sessionstore.Session{
		ID:      "special-chars",
		Date:    time.Now(),
		Summary: "Test with <html> & \"quotes\" and unicode: 日本語",
		Knowledge: []string{
			"Code: func() { return \"value\" }",
			"Path: /usr/local/<name>",
		},
	}

	var buf bytes.Buffer
	err := f.Format(&buf, session)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	var output map[string]any
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse output with special chars: %v", err)
	}

	// Verify special characters are preserved
	if output["summary"] != session.Summary {
		t.Errorf("summary not preserved: got %q, want %q", output["summary"], session.Summary)
	}
}

func TestJSONLFormatter_Format_Pretty(t *testing.T) {
	f := NewJSONLFormatter()
	f.Pretty = true

	session := &sessionstore.Session{
		ID:      "pretty-test",
		Date:    time.Now(),
		Summary: "Pretty formatted",
	}

	var buf bytes.Buffer
	err := f.Format(&buf, session)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Pretty output should contain indentation
	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("\n  ")) {
		t.Errorf("Pretty output should contain indentation:\n%s", output)
	}
}

func TestJSONLFormatter_buildOutput(t *testing.T) {
	f := NewJSONLFormatter()

	t.Run("with tokens", func(t *testing.T) {
		session := &sessionstore.Session{
			ID:   "test",
			Date: time.Now(),
			Tokens: sessionstore.TokenUsage{
				Input:  100,
				Output: 50,
				Total:  150,
			},
		}
		output := f.buildOutput(session)
		if output.Tokens == nil {
			t.Error("Tokens should not be nil when Total > 0")
		}
	})

	t.Run("without tokens", func(t *testing.T) {
		session := &sessionstore.Session{
			ID:   "test",
			Date: time.Now(),
			Tokens: sessionstore.TokenUsage{
				Total: 0,
			},
		}
		output := f.buildOutput(session)
		if output.Tokens != nil {
			t.Error("Tokens should be nil when Total = 0")
		}
	})
}
