package parser

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestParser_Parse(t *testing.T) {
	jsonl := `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"Hello"}}
{"type":"assistant","sessionId":"test","timestamp":"2026-01-24T10:00:10.000Z","uuid":"2","message":{"role":"assistant","content":"Hi there!"}}
`
	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result.TotalLines != 2 {
		t.Errorf("TotalLines = %d, want 2", result.TotalLines)
	}

	if len(result.Messages) != 2 {
		t.Fatalf("Messages count = %d, want 2", len(result.Messages))
	}

	if result.Messages[0].Role != "user" {
		t.Errorf("First message role = %q, want %q", result.Messages[0].Role, "user")
	}

	if result.Messages[1].Content != "Hi there!" {
		t.Errorf("Second message content = %q, want %q", result.Messages[1].Content, "Hi there!")
	}
}

func TestParser_SkipMalformed(t *testing.T) {
	jsonl := `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"Valid"}}
{malformed json
{"type":"assistant","sessionId":"test","timestamp":"2026-01-24T10:00:10.000Z","uuid":"2","message":{"role":"assistant","content":"Also valid"}}
`
	p := NewParser()
	p.SkipMalformed = true

	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result.MalformedLines != 1 {
		t.Errorf("MalformedLines = %d, want 1", result.MalformedLines)
	}

	if len(result.Messages) != 2 {
		t.Errorf("Messages count = %d, want 2", len(result.Messages))
	}
}

func TestParser_Truncation(t *testing.T) {
	longContent := strings.Repeat("x", 600)
	jsonl := `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"` + longContent + `"}}`

	p := NewParser()
	p.MaxContentLength = 500

	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("Messages count = %d, want 1", len(result.Messages))
	}

	content := result.Messages[0].Content
	if !strings.HasSuffix(content, "... [truncated]") {
		t.Errorf("Content not truncated correctly: %s", content)
	}

	// 500 chars + "... [truncated]" = ~515
	if len(content) > 520 {
		t.Errorf("Truncated content too long: %d chars", len(content))
	}
}

func TestParser_SkipNonMessageTypes(t *testing.T) {
	jsonl := `{"type":"file-history-snapshot","messageId":"123","snapshot":{}}
{"type":"progress","data":{"type":"hook_progress"}}
{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"Real message"}}
`
	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Messages) != 1 {
		t.Errorf("Messages count = %d, want 1 (should skip non-message types)", len(result.Messages))
	}
}

func TestParser_Parse_ClaudeTranscriptShape(t *testing.T) {
	jsonl := `{"type":"user","timestamp":"2026-02-22T03:56:39.705Z","content":"find shield ai recruiter chat"}
{"type":"tool_use","timestamp":"2026-02-22T03:56:47.577Z","tool_name":"skill","tool_input":{"name":"retro"}}
{"type":"tool_result","timestamp":"2026-02-22T03:56:47.647Z","tool_name":"skill","tool_input":{"name":"retro"},"tool_output":{"truncated":false}}
`

	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Messages) != 3 {
		t.Fatalf("Messages count = %d, want 3", len(result.Messages))
	}
	if result.Messages[0].Type != "user" || result.Messages[0].Content != "find shield ai recruiter chat" {
		t.Fatalf("unexpected first message: %+v", result.Messages[0])
	}
	if len(result.Messages[1].Tools) != 1 || result.Messages[1].Tools[0].Name != "skill" {
		t.Fatalf("unexpected tool_use message: %+v", result.Messages[1])
	}
	if len(result.Messages[2].Tools) != 1 || !strings.Contains(result.Messages[2].Tools[0].Output, `"truncated":false`) {
		t.Fatalf("unexpected tool_result message: %+v", result.Messages[2])
	}
}

func TestParser_Parse_CodexArchivedSessionShape(t *testing.T) {
	jsonl := `{"timestamp":"2026-03-05T20:20:42.160Z","type":"session_meta","payload":{"id":"019cbfa8-9155-7121-b18a-dfa3783cdd9e","timestamp":"2026-03-05T20:20:21.464Z"}}
{"timestamp":"2026-03-05T20:20:42.163Z","type":"event_msg","payload":{"type":"user_message","message":"find recruiter chat"}}
{"timestamp":"2026-03-05T20:20:54.239Z","type":"event_msg","payload":{"type":"agent_message","message":"I am auditing the repo."}}
{"timestamp":"2026-03-05T20:20:54.282Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"pwd\"}"}}
{"timestamp":"2026-03-05T20:20:54.381Z","type":"response_item","payload":{"type":"function_call_output","output":"Chunk ID: abc\\nOutput:\\n/worktree\\n"}}
`

	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Messages) != 5 {
		t.Fatalf("Messages count = %d, want 5", len(result.Messages))
	}
	if result.Messages[0].SessionID != "019cbfa8-9155-7121-b18a-dfa3783cdd9e" {
		t.Fatalf("session_meta did not capture session ID: %+v", result.Messages[0])
	}
	if result.Messages[1].Type != "user" || result.Messages[1].Content != "find recruiter chat" {
		t.Fatalf("unexpected user event message: %+v", result.Messages[1])
	}
	if result.Messages[2].Type != "assistant" || result.Messages[2].Content != "I am auditing the repo." {
		t.Fatalf("unexpected agent event message: %+v", result.Messages[2])
	}
	if len(result.Messages[3].Tools) != 1 || result.Messages[3].Tools[0].Name != "exec_command" {
		t.Fatalf("unexpected function_call message: %+v", result.Messages[3])
	}
	if len(result.Messages[4].Tools) != 1 || !strings.Contains(result.Messages[4].Tools[0].Output, "/worktree") {
		t.Fatalf("unexpected function_call_output message: %+v", result.Messages[4])
	}
}

func TestParser_ParseFile_Fixtures(t *testing.T) {
	fixtures := []struct {
		name        string
		minMessages int
	}{
		{"simple-decision.jsonl", 4},
		{"multi-extract.jsonl", 5},
		{"tool-heavy.jsonl", 5},
		{"long-session.jsonl", 100},
		{"edge-cases.jsonl", 5},
	}

	p := NewParser()
	fixtureDir := "../../testdata/transcripts"

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(fixtureDir, tc.name)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Skipf("Fixture not found: %s", path)
			}

			result, err := p.ParseFile(path)
			if err != nil {
				t.Fatalf("ParseFile failed: %v", err)
			}

			if len(result.Messages) < tc.minMessages {
				t.Errorf("Messages = %d, want at least %d", len(result.Messages), tc.minMessages)
			}
		})
	}
}

func TestParser_Unicode(t *testing.T) {
	jsonl := `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"你好世界 🚀 émojis"}}`

	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("Messages count = %d, want 1", len(result.Messages))
	}

	if !strings.Contains(result.Messages[0].Content, "你好世界") {
		t.Error("Unicode content not preserved")
	}

	if !strings.Contains(result.Messages[0].Content, "🚀") {
		t.Error("Emoji not preserved")
	}
}

func TestParser_ParseChannel(t *testing.T) {
	jsonl := `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"One"}}
{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:10.000Z","uuid":"2","message":{"role":"user","content":"Two"}}
{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:20.000Z","uuid":"3","message":{"role":"user","content":"Three"}}
`
	p := NewParser()
	msgCh, errCh := p.ParseChannel(strings.NewReader(jsonl))

	count := 0
	for range msgCh {
		count++
	}

	if err := <-errCh; err != nil {
		t.Fatalf("ParseChannel error: %v", err)
	}

	if count != 3 {
		t.Errorf("Message count = %d, want 3", count)
	}
}

func TestExtractor_Extract(t *testing.T) {
	e := NewExtractor()

	tests := []struct {
		name     string
		content  string
		wantType string
	}{
		{
			name:     "Decision pattern",
			content:  "**Decision:** Use context.WithCancel for graceful shutdown.",
			wantType: "decision",
		},
		{
			name:     "Solution pattern",
			content:  "**Solution:** Fixed the bug by adding null check.",
			wantType: "solution",
		},
		{
			name:     "Learning pattern",
			content:  "**Learning:** Always validate JWT expiration claims.",
			wantType: "learning",
		},
		{
			name:     "Failure pattern",
			content:  "**Failure:** Caching auth responses didn't work because of session isolation.",
			wantType: "failure",
		},
		{
			name:     "Reference with URL",
			content:  "See https://example.com/docs for more info.",
			wantType: "reference",
		},
		{
			name:     "No match",
			content:  "Just a regular message without any patterns.",
			wantType: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := createTestMessage(tc.content)
			results := e.Extract(msg)

			if tc.wantType == "" {
				if len(results) > 0 {
					t.Errorf("Expected no match, got %d", len(results))
				}
				return
			}

			found := false
			for _, r := range results {
				if string(r.Type) == tc.wantType {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected type %q not found in results", tc.wantType)
			}
		})
	}
}

func TestExtractor_ExtractBest(t *testing.T) {
	e := NewExtractor()

	// Message with multiple patterns - should return highest score
	content := "**Decision:** Use X. Also **Learning:** This teaches us Y."
	msg := createTestMessage(content)

	best := e.ExtractBest(msg)
	if best == nil {
		t.Fatal("Expected a result, got nil")
		return
	}

	// The pattern match should give higher score than keyword
	if best.Score < 0.6 {
		t.Errorf("Score = %f, want >= 0.6", best.Score)
	}
}

// createTestMessage creates a TranscriptMessage for testing.
func createTestMessage(content string) types.TranscriptMessage {
	return types.TranscriptMessage{
		Type:    "assistant",
		Role:    "assistant",
		Content: content,
	}
}

// --- New coverage tests below ---

func TestParseError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ParseError
		want string
	}{
		{
			name: "without column",
			err:  ParseError{Line: 5, Message: "bad json", ErrorType: "json"},
			want: "line 5: bad json (json)",
		},
		{
			name: "with column",
			err:  ParseError{Line: 3, Column: 12, Message: "unexpected end", ErrorType: "json"},
			want: "line 3, col 12: unexpected end (json)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Error()
			if got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"invalid character", errors.New("invalid character 'x'"), "json"},
		{"unexpected end", errors.New("unexpected end of JSON input"), "json"},
		{"cannot unmarshal", errors.New("cannot unmarshal string into int"), "schema"},
		{"invalid UTF-8", errors.New("invalid UTF-8 byte sequence"), "encoding"},
		{"generic error", errors.New("something else entirely"), "json"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyError(tc.err)
			if got != tc.want {
				t.Errorf("classifyError() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTruncateForError(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 5, "hello..."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateForError(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("truncateForError() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantStr string // expected time in RFC3339 or empty for zero
	}{
		{"RFC3339", "2026-01-24T10:00:00Z", "2026-01-24T10:00:00Z"},
		{"millisecond format", "2026-01-24T10:00:00.000Z", "2026-01-24T10:00:00Z"},
		{"invalid format", "not-a-timestamp", ""},
		{"empty string", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseTimestamp(tc.input)
			if tc.wantStr == "" {
				if !got.IsZero() {
					t.Errorf("parseTimestamp(%q) = %v, want zero time", tc.input, got)
				}
			} else {
				want, _ := time.Parse(time.RFC3339, tc.wantStr)
				if !got.Equal(want) {
					t.Errorf("parseTimestamp(%q) = %v, want %v", tc.input, got, want)
				}
			}
		})
	}
}

func TestIsValidMessageType(t *testing.T) {
	tests := []struct {
		msgType string
		want    bool
	}{
		{"user", true},
		{"assistant", true},
		{"tool_use", true},
		{"tool_result", true},
		{"progress", false},
		{"file-history-snapshot", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.msgType, func(t *testing.T) {
			got := isValidMessageType(tc.msgType)
			if got != tc.want {
				t.Errorf("isValidMessageType(%q) = %v, want %v", tc.msgType, got, tc.want)
			}
		})
	}
}

func TestParser_Truncate(t *testing.T) {
	tests := []struct {
		name      string
		maxLen    int
		input     string
		wantExact string
	}{
		{"zero max disables truncation", 0, "long text here", "long text here"},
		{"negative max disables truncation", -1, "long text here", "long text here"},
		{"within limit", 500, "short", "short"},
		{"exceeds limit", 5, "hello world", "hello... [truncated]"},
		{"multi-byte unicode at boundary", 3, "世界你好", "世界你... [truncated]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &Parser{MaxContentLength: tc.maxLen}
			got := p.truncate(tc.input)
			if got != tc.wantExact {
				t.Errorf("truncate() = %q, want %q", got, tc.wantExact)
			}
		})
	}
}

func TestParser_ParseContentBlocks(t *testing.T) {
	p := NewParser()

	t.Run("text block", func(t *testing.T) {
		blocks := []any{
			map[string]any{"type": "text", "text": "hello"},
		}
		content, tools := p.parseContentBlocks(blocks)
		if content != "hello" {
			t.Errorf("content = %q, want %q", content, "hello")
		}
		if len(tools) != 0 {
			t.Errorf("tools count = %d, want 0", len(tools))
		}
	})

	t.Run("tool_use block", func(t *testing.T) {
		blocks := []any{
			map[string]any{
				"type":  "tool_use",
				"name":  "Read",
				"input": map[string]any{"file_path": "/tmp/x"},
			},
		}
		content, tools := p.parseContentBlocks(blocks)
		if content != "" {
			t.Errorf("content = %q, want empty", content)
		}
		if len(tools) != 1 || tools[0].Name != "Read" {
			t.Errorf("tools = %+v, want 1 tool named Read", tools)
		}
	})

	t.Run("tool_result block", func(t *testing.T) {
		blocks := []any{
			map[string]any{
				"type":    "tool_result",
				"content": "result text",
			},
		}
		_, tools := p.parseContentBlocks(blocks)
		if len(tools) != 1 || tools[0].Output != "result text" {
			t.Errorf("tools = %+v, want 1 tool_result with output", tools)
		}
	})

	t.Run("non-map block is skipped", func(t *testing.T) {
		blocks := []any{"not a map", 42}
		content, tools := p.parseContentBlocks(blocks)
		if content != "" || len(tools) != 0 {
			t.Errorf("expected empty results for non-map blocks, got content=%q tools=%d", content, len(tools))
		}
	})

	t.Run("mixed blocks", func(t *testing.T) {
		blocks := []any{
			map[string]any{"type": "text", "text": "part1"},
			map[string]any{"type": "tool_use", "name": "Bash", "input": map[string]any{"command": "ls"}},
			map[string]any{"type": "text", "text": "part2"},
		}
		content, tools := p.parseContentBlocks(blocks)
		if content != "part1part2" {
			t.Errorf("content = %q, want %q", content, "part1part2")
		}
		if len(tools) != 1 {
			t.Errorf("tools count = %d, want 1", len(tools))
		}
	})
}

func TestParser_ParseToolUse(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name     string
		block    map[string]any
		wantNil  bool
		wantName string
	}{
		{
			name:     "valid tool_use",
			block:    map[string]any{"name": "Read", "input": map[string]any{"path": "/x"}},
			wantName: "Read",
		},
		{
			name:    "missing name",
			block:   map[string]any{"input": map[string]any{}},
			wantNil: true,
		},
		{
			name:    "empty name",
			block:   map[string]any{"name": ""},
			wantNil: true,
		},
		{
			name:     "no input field",
			block:    map[string]any{"name": "Bash"},
			wantName: "Bash",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := p.parseToolUse(tc.block)
			if tc.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil result")
				return
			}
			if got.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tc.wantName)
			}
		})
	}
}

func TestParser_ParseToolResult(t *testing.T) {
	p := NewParser()

	t.Run("string content", func(t *testing.T) {
		block := map[string]any{"content": "output text"}
		got := p.parseToolResult(block)
		if got.Output != "output text" {
			t.Errorf("Output = %q, want %q", got.Output, "output text")
		}
		if got.Error != "" {
			t.Errorf("Error = %q, want empty", got.Error)
		}
	})

	t.Run("error result", func(t *testing.T) {
		block := map[string]any{"is_error": true, "content": "failed"}
		got := p.parseToolResult(block)
		if got.Error != "tool error" {
			t.Errorf("Error = %q, want %q", got.Error, "tool error")
		}
	})

	t.Run("array content", func(t *testing.T) {
		block := map[string]any{
			"content": []any{
				map[string]any{"text": "line1"},
				map[string]any{"text": "line2"},
			},
		}
		got := p.parseToolResult(block)
		if got.Output != "line1line2" {
			t.Errorf("Output = %q, want %q", got.Output, "line1line2")
		}
	})

	t.Run("no content", func(t *testing.T) {
		block := map[string]any{}
		got := p.parseToolResult(block)
		if got.Name != "tool_result" {
			t.Errorf("Name = %q, want %q", got.Name, "tool_result")
		}
	})
}

func TestParser_ParseMalformedNotSkipped(t *testing.T) {
	jsonl := `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"Valid"}}
{malformed json
{"type":"assistant","sessionId":"test","timestamp":"2026-01-24T10:00:10.000Z","uuid":"2","message":{"role":"assistant","content":"Also valid"}}
`
	p := NewParser()
	p.SkipMalformed = false

	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result.MalformedLines != 1 {
		t.Errorf("MalformedLines = %d, want 1", result.MalformedLines)
	}

	if len(result.Errors) != 1 {
		t.Fatalf("Errors count = %d, want 1", len(result.Errors))
	}

	// Verify it's a ParseError
	var parseErr *ParseError
	if !errors.As(result.Errors[0], &parseErr) {
		t.Fatalf("Error type = %T, want *ParseError", result.Errors[0])
	}
	if parseErr.Line != 2 {
		t.Errorf("ParseError.Line = %d, want 2", parseErr.Line)
	}
	if parseErr.ErrorType != "json" {
		t.Errorf("ParseError.ErrorType = %q, want %q", parseErr.ErrorType, "json")
	}
	if parseErr.RawContent == "" {
		t.Error("ParseError.RawContent should not be empty")
	}
}

func TestParser_Checksum(t *testing.T) {
	jsonl := `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"Hello"}}`
	p := NewParser()

	r1, _ := p.Parse(strings.NewReader(jsonl))
	r2, _ := p.Parse(strings.NewReader(jsonl))

	if r1.Checksum == "" {
		t.Error("Checksum should not be empty")
	}
	if r1.Checksum != r2.Checksum {
		t.Errorf("Same input should produce same checksum: %q vs %q", r1.Checksum, r2.Checksum)
	}
	if len(r1.Checksum) != 16 {
		t.Errorf("Checksum length = %d, want 16 hex chars", len(r1.Checksum))
	}

	// Different input → different checksum
	r3, _ := p.Parse(strings.NewReader(`{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"Different"}}`))
	if r1.Checksum == r3.Checksum {
		t.Error("Different input should produce different checksum")
	}
}

func TestParser_ParsedAt(t *testing.T) {
	jsonl := `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"Hello"}}`
	p := NewParser()

	before := time.Now()
	result, _ := p.Parse(strings.NewReader(jsonl))
	after := time.Now()

	if result.ParsedAt.Before(before) || result.ParsedAt.After(after) {
		t.Errorf("ParsedAt = %v, want between %v and %v", result.ParsedAt, before, after)
	}
}

func TestParser_OnProgress(t *testing.T) {
	// Build 200+ lines to trigger OnProgress (fires every 100 lines)
	var lines []string
	for i := range 250 {
		lines = append(lines, fmt.Sprintf(`{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"%d","message":{"role":"user","content":"msg %d"}}`, i, i))
	}
	jsonl := strings.Join(lines, "\n")

	p := NewParser()
	progressCalls := 0
	p.OnProgress = func(linesProcessed, totalLines int) {
		progressCalls++
	}

	_, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if progressCalls < 2 {
		t.Errorf("OnProgress called %d times, want >= 2 (for 250 lines)", progressCalls)
	}
}

func TestParser_EmptyInput(t *testing.T) {
	p := NewParser()
	result, err := p.Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(result.Messages) != 0 {
		t.Errorf("Messages count = %d, want 0", len(result.Messages))
	}
	if result.TotalLines != 0 {
		t.Errorf("TotalLines = %d, want 0", result.TotalLines)
	}
}

func TestParser_BlankLines(t *testing.T) {
	jsonl := "\n\n" + `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"Hello"}}` + "\n\n"
	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Errorf("Messages count = %d, want 1", len(result.Messages))
	}
}

func TestParser_ContentBlocks_ViaFullParse(t *testing.T) {
	// Test assistant message with content blocks array (tool_use)
	jsonl := `{"type":"assistant","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"assistant","content":[{"type":"text","text":"Let me check."},{"type":"tool_use","name":"Read","input":{"file_path":"/tmp/x"}}]}}`

	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("Messages count = %d, want 1", len(result.Messages))
	}

	msg := result.Messages[0]
	if msg.Content != "Let me check." {
		t.Errorf("Content = %q, want %q", msg.Content, "Let me check.")
	}
	if len(msg.Tools) != 1 {
		t.Fatalf("Tools count = %d, want 1", len(msg.Tools))
	}
	if msg.Tools[0].Name != "Read" {
		t.Errorf("Tool name = %q, want %q", msg.Tools[0].Name, "Read")
	}
}

func TestParser_ToolResultBlock_ViaFullParse(t *testing.T) {
	jsonl := `{"type":"assistant","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"assistant","content":[{"type":"tool_result","content":"file contents here","is_error":false}]}}`

	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("Messages count = %d, want 1", len(result.Messages))
	}

	msg := result.Messages[0]
	if len(msg.Tools) != 1 {
		t.Fatalf("Tools count = %d, want 1", len(msg.Tools))
	}
	if msg.Tools[0].Output != "file contents here" {
		t.Errorf("Tool output = %q, want %q", msg.Tools[0].Output, "file contents here")
	}
}

func TestParser_ParseFile_NotFound(t *testing.T) {
	p := NewParser()
	_, err := p.ParseFile("/nonexistent/path/file.jsonl")
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
}

func TestParser_ParseFile_SetsFilePath(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")
	content := `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"Hello"}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	p := NewParser()
	result, err := p.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if result.FilePath != path {
		t.Errorf("FilePath = %q, want %q", result.FilePath, path)
	}
}

func TestParser_ParseChannel_MalformedNotSkipped(t *testing.T) {
	jsonl := `{malformed
{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"Valid"}}
`
	p := NewParser()
	p.SkipMalformed = false

	msgCh, errCh := p.ParseChannel(strings.NewReader(jsonl))

	// Drain messages (should get 0 because error stops processing)
	for range msgCh {
	}

	err := <-errCh
	if err == nil {
		t.Error("Expected error for malformed line with SkipMalformed=false")
	}
}

func TestParser_ParseChannel_SkipMalformed(t *testing.T) {
	jsonl := `{bad json
{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"Good"}}
`
	p := NewParser()
	p.SkipMalformed = true

	msgCh, errCh := p.ParseChannel(strings.NewReader(jsonl))

	count := 0
	for range msgCh {
		count++
	}

	if err := <-errCh; err != nil {
		t.Fatalf("ParseChannel error: %v", err)
	}

	if count != 1 {
		t.Errorf("Message count = %d, want 1", count)
	}
}

func TestParser_ParseChannel_EmptyLines(t *testing.T) {
	// Input with empty lines interspersed should be skipped
	jsonl := `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"One"}}

{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:10.000Z","uuid":"2","message":{"role":"user","content":"Two"}}

`
	p := NewParser()
	msgCh, errCh := p.ParseChannel(strings.NewReader(jsonl))

	count := 0
	for range msgCh {
		count++
	}

	if err := <-errCh; err != nil {
		t.Fatalf("ParseChannel error: %v", err)
	}

	if count != 2 {
		t.Errorf("Message count = %d, want 2 (empty lines should be skipped)", count)
	}
}

func TestExtractor_Extract_EmptyContent(t *testing.T) {
	e := NewExtractor()
	msg := types.TranscriptMessage{Content: ""}
	results := e.Extract(msg)
	if results != nil {
		t.Errorf("Expected nil for empty content, got %d results", len(results))
	}
}

func TestExtractor_ExtractBest_NoMatch(t *testing.T) {
	e := NewExtractor()
	msg := createTestMessage("Just a regular message.")
	best := e.ExtractBest(msg)
	if best != nil {
		t.Errorf("Expected nil for no match, got %+v", best)
	}
}

func TestExtractor_Extract_DeduplicatesByType(t *testing.T) {
	e := NewExtractor()
	// Content matches both keyword AND pattern for Decision → should deduplicate to 1 result
	msg := createTestMessage("**Decision:** decided to use context because it works")
	results := e.Extract(msg)

	decisionCount := 0
	for _, r := range results {
		if r.Type == types.KnowledgeTypeDecision {
			decisionCount++
		}
	}
	if decisionCount != 1 {
		t.Errorf("Decision results = %d, want 1 (should deduplicate)", decisionCount)
	}
}

func TestExtractor_Extract_PatternScoreHigherThanKeyword(t *testing.T) {
	e := NewExtractor()
	// This content matches both keyword and regex for Decision
	msg := createTestMessage("**Decision:** decided to use context because it works")
	results := e.Extract(msg)

	for _, r := range results {
		if r.Type == types.KnowledgeTypeDecision {
			// Pattern match gives +0.2 bonus, keyword gives +0.1
			// MinScore for decision is 0.6
			// So pattern match score = 0.8, keyword = 0.7
			if r.Score < 0.8 {
				t.Errorf("Decision score = %f, want >= 0.8 (pattern match should win dedup)", r.Score)
			}
			if r.MatchedPattern == "" {
				t.Error("Expected MatchedPattern to be set (pattern match wins)")
			}
			return
		}
	}
	t.Error("No decision result found")
}

func TestParser_ToolUseAndResultTypes(t *testing.T) {
	// Verify tool_use and tool_result are valid message types
	tests := []struct {
		msgType string
		role    string
	}{
		{"tool_use", "assistant"},
		{"tool_result", "user"},
	}

	for _, tc := range tests {
		t.Run(tc.msgType, func(t *testing.T) {
			jsonl := fmt.Sprintf(`{"type":%q,"sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":%q,"content":"test content"}}`, tc.msgType, tc.role)
			p := NewParser()
			result, err := p.Parse(strings.NewReader(jsonl))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			if len(result.Messages) != 1 {
				t.Errorf("Messages count = %d, want 1", len(result.Messages))
			}
		})
	}
}

func TestParser_MessageWithoutContent(t *testing.T) {
	// Message field present but no content
	jsonl := `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user"}}`
	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("Messages count = %d, want 1", len(result.Messages))
	}
	if result.Messages[0].Content != "" {
		t.Errorf("Content = %q, want empty", result.Messages[0].Content)
	}
}

func TestParser_NilMessage(t *testing.T) {
	// Valid type but no message field
	jsonl := `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1"}`
	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("Messages count = %d, want 1", len(result.Messages))
	}
	if result.Messages[0].Content != "" {
		t.Errorf("Content = %q, want empty", result.Messages[0].Content)
	}
}

func TestExtractBest_HigherScoreLaterInSlice(t *testing.T) {
	// Use a custom extractor with two patterns that have very different MinScores.
	// The first pattern (low score) will match, and the second (high score) will match.
	// We run ExtractBest many times; if it ever enters the r.Score > best.Score branch,
	// coverage is achieved. With two items in random map order, one iteration will hit it.
	e := &Extractor{
		Patterns: []ExtractionPattern{
			{
				Type:     types.KnowledgeTypeReference,
				Keywords: []string{"see also"},
				MinScore: 0.1, // Very low score (0.1 + 0.1 = 0.2)
			},
			{
				Type:     types.KnowledgeTypeSolution,
				Keywords: []string{"fixed by"},
				MinScore: 0.9, // Very high score (0.9 + 0.1 = 1.0)
			},
		},
	}

	msg := createTestMessage("see also the docs. fixed by restarting.")

	// Run many times to exercise both map iteration orders
	sawHighScore := false
	for range 100 {
		best := e.ExtractBest(msg)
		if best == nil {
			t.Fatal("Expected a result, got nil")
		}
		if best.Type == types.KnowledgeTypeSolution {
			sawHighScore = true
		}
	}
	if !sawHighScore {
		t.Error("ExtractBest never returned the higher-scored Solution result")
	}
}

func TestParse_LargeLine(t *testing.T) {
	hugeLine := strings.Repeat("x", 2*1024*1024)
	jsonl := fmt.Sprintf(`{"type":"user","sessionId":"big-session","timestamp":"2026-01-24T10:00:00.000Z","message":{"role":"user","content":"%s"}}`, hugeLine)
	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("expected large line to parse successfully, got: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("Messages count = %d, want 1", len(result.Messages))
	}
	if result.Messages[0].SessionID != "big-session" {
		t.Fatalf("session ID = %q, want big-session", result.Messages[0].SessionID)
	}
}

func TestParseChannel_LargeLine(t *testing.T) {
	hugeLine := strings.Repeat("x", 2*1024*1024)
	jsonl := fmt.Sprintf(`{"type":"user","sessionId":"big-session","timestamp":"2026-01-24T10:00:00.000Z","message":{"role":"user","content":"%s"}}`, hugeLine)
	p := NewParser()
	msgCh, errCh := p.ParseChannel(strings.NewReader(jsonl))

	count := 0
	for range msgCh {
		count++
	}

	err := <-errCh
	if err != nil {
		t.Fatalf("expected large line to parse successfully, got: %v", err)
	}
	if count != 1 {
		t.Fatalf("message count = %d, want 1", count)
	}
}

// --- Benchmarks ---

func BenchmarkParse_100Lines(b *testing.B) {
	var lines []string
	for i := range 100 {
		lines = append(lines, fmt.Sprintf(
			`{"type":"assistant","sessionId":"bench","timestamp":"2026-01-24T10:00:00.000Z","uuid":"%d","message":{"role":"assistant","content":"Response number %d with some content."}}`,
			i, i))
	}
	input := strings.Join(lines, "\n") + "\n"

	b.ResetTimer()
	for range b.N {
		p := NewParser()
		_, _ = p.Parse(strings.NewReader(input))
	}
}

func BenchmarkParse_1000Lines(b *testing.B) {
	var lines []string
	for i := range 1000 {
		lines = append(lines, fmt.Sprintf(
			`{"type":"user","sessionId":"bench","timestamp":"2026-01-24T10:00:00.000Z","uuid":"%d","message":{"role":"user","content":"Message %d"}}`,
			i, i))
	}
	input := strings.Join(lines, "\n") + "\n"

	b.ResetTimer()
	for range b.N {
		p := NewParser()
		_, _ = p.Parse(strings.NewReader(input))
	}
}

func BenchmarkExtract(b *testing.B) {
	e := NewExtractor()
	msg := createTestMessage("**Decision:** We decided to use context.WithCancel for graceful shutdown because it works best.")

	b.ResetTimer()
	for range b.N {
		e.Extract(msg)
	}
}

func BenchmarkExtractBest(b *testing.B) {
	e := NewExtractor()
	msg := createTestMessage("**Decision:** Use X. **Learning:** This teaches us Y. See https://example.com for details.")

	b.ResetTimer()
	for range b.N {
		e.ExtractBest(msg)
	}
}

func TestParseFile_CloseError(t *testing.T) {
	// Exercise the defer branch where f.Close() returns an error and err == nil.
	jsonl := `{"type":"user","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","uuid":"1","message":{"role":"user","content":"Hello"}}`

	closeErr := fmt.Errorf("injected close error")
	origOpen := openFileFunc
	openFileFunc = func(path string) (io.ReadCloser, error) {
		return io.NopCloser(nil), fmt.Errorf("should not be called")
	}
	// Override with a ReadCloser that reads fine but fails on Close.
	openFileFunc = func(_ string) (io.ReadCloser, error) {
		return &failCloseReader{Reader: strings.NewReader(jsonl), err: closeErr}, nil
	}
	defer func() { openFileFunc = origOpen }()

	p := NewParser()
	result, err := p.ParseFile("fake.jsonl")
	if err == nil {
		t.Fatal("expected close error, got nil")
	}
	if err != closeErr {
		t.Errorf("expected injected close error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result even with close error")
	}
	if len(result.Messages) != 1 {
		t.Errorf("Messages count = %d, want 1", len(result.Messages))
	}
}

// failCloseReader wraps a Reader and returns an error on Close.
type failCloseReader struct {
	io.Reader
	err error
}

func (f *failCloseReader) Close() error { return f.err }

func TestClassifyBlock_TextMissingTextField(t *testing.T) {
	// Exercise the path where type is "text" but the "text" field is not a string.
	p := NewParser()
	block := map[string]any{"type": "text", "text": 42} // not a string
	text, tool := p.classifyBlock(block)
	if text != "" {
		t.Errorf("expected empty text, got %q", text)
	}
	if tool != nil {
		t.Error("expected nil tool")
	}
}

func TestClassifyBlock_UnknownType(t *testing.T) {
	p := NewParser()
	block := map[string]any{"type": "unknown_type"}
	text, tool := p.classifyBlock(block)
	if text != "" {
		t.Errorf("expected empty text, got %q", text)
	}
	if tool != nil {
		t.Error("expected nil tool")
	}
}

func TestClassifyBlock_NoType(t *testing.T) {
	p := NewParser()
	block := map[string]any{"foo": "bar"}
	text, tool := p.classifyBlock(block)
	if text != "" {
		t.Errorf("expected empty text, got %q", text)
	}
	if tool != nil {
		t.Error("expected nil tool")
	}
}

func TestParser_ParseClaudeLegacyToolArray(t *testing.T) {
	jsonl := `{"type":"tool_use","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","message":{"role":"assistant","tools":[{"name":"Read","input":{"file_path":"/tmp/a.txt"},"output":"hello"}]}}`
	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("Messages count = %d, want 1", len(result.Messages))
	}
	if len(result.Messages[0].Tools) != 1 || result.Messages[0].Tools[0].Name != "Read" {
		t.Fatalf("unexpected tools: %+v", result.Messages[0].Tools)
	}
	if result.Messages[0].Tools[0].Output != "hello" {
		t.Fatalf("tool output = %q, want hello", result.Messages[0].Tools[0].Output)
	}
}

func TestParser_ParseClaudeLegacyToolArray_SkipsUnnamedTools(t *testing.T) {
	jsonl := `{"type":"tool_use","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","message":{"role":"assistant","tools":[{"input":{"file_path":"/tmp/ignored.txt"}},{"name":"Read","input":{"file_path":"/tmp/a.txt"}}]}}`
	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("Messages count = %d, want 1", len(result.Messages))
	}
	if len(result.Messages[0].Tools) != 1 || result.Messages[0].Tools[0].Name != "Read" {
		t.Fatalf("unexpected tools: %+v", result.Messages[0].Tools)
	}
}

func TestParser_ParseClaudeToolUseFallbackMessage(t *testing.T) {
	jsonl := `{"type":"tool_use","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","message":{"role":"assistant","content":"tool completed"}}`
	p := NewParser()
	result, err := p.Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("Messages count = %d, want 1", len(result.Messages))
	}
	if result.Messages[0].Content != "tool completed" {
		t.Fatalf("content = %q, want tool completed", result.Messages[0].Content)
	}
}

func TestParser_ParseClaudeToolResultFallbacks(t *testing.T) {
	t.Run("toolUseResult fallback", func(t *testing.T) {
		jsonl := `{"type":"tool_result","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","toolUseResult":{"status":"ok"}}`
		p := NewParser()
		result, err := p.Parse(strings.NewReader(jsonl))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if got := result.Messages[0].Tools[0].Output; !strings.Contains(got, `"status":"ok"`) {
			t.Fatalf("tool output = %q, want JSON fallback", got)
		}
	})

	t.Run("message content fallback", func(t *testing.T) {
		jsonl := `{"type":"tool_result","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z","message":{"role":"user","content":"result body"}}`
		p := NewParser()
		result, err := p.Parse(strings.NewReader(jsonl))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if got := result.Messages[0].Tools[0].Output; got != "result body" {
			t.Fatalf("tool output = %q, want result body", got)
		}
		if result.Messages[0].Role != "user" {
			t.Fatalf("role = %q, want user", result.Messages[0].Role)
		}
	})
}

func TestParser_ParseCodexEventAndResponseItemEdges(t *testing.T) {
	p := NewParser()

	for _, tc := range codexEventAndResponseItemEdgeCases() {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, p)
		})
	}
}

type codexEventAndResponseItemEdgeCase struct {
	name string
	run  func(t *testing.T, p *Parser)
}

func codexEventAndResponseItemEdgeCases() []codexEventAndResponseItemEdgeCase {
	return []codexEventAndResponseItemEdgeCase{
		{"unsupported event type is skipped", assertUnsupportedCodexEventTypeSkipped},
		{"invalid event payload returns error", assertInvalidCodexEventPayloadErrors},
		{"assistant response_item message parses", assertAssistantResponseItemMessageParses},
		{"developer response_item message is skipped", assertDeveloperResponseItemMessageSkipped},
		{"custom tool call parses invalid arguments as raw", assertCustomToolCallInvalidArgumentsRaw},
		{"custom tool call output parses", assertCustomToolCallOutputParses},
		{"invalid response_item payload returns error", assertInvalidResponseItemPayloadErrors},
		{"invalid session_meta payload returns error", assertInvalidSessionMetaPayloadErrors},
		{"unsupported response_item type is skipped", assertUnsupportedResponseItemTypeSkipped},
	}
}

func parseCodexEdgeLine(t *testing.T, p *Parser, jsonl string) *types.TranscriptMessage {
	t.Helper()

	msg, err := p.parseLine([]byte(jsonl), 1)
	if err != nil {
		t.Fatalf("parseLine failed: %v", err)
	}
	return msg
}

func parseCodexEdgeError(p *Parser, jsonl string) error {
	_, err := p.parseLine([]byte(jsonl), 1)
	return err
}

func assertUnsupportedCodexEventTypeSkipped(t *testing.T, p *Parser) {
	t.Helper()

	msg := parseCodexEdgeLine(t, p, `{"type":"event_msg","payload":{"type":"task_started","message":"ignore"}}`)
	if msg != nil {
		t.Fatalf("expected nil message, got %+v", msg)
	}
}

func assertInvalidCodexEventPayloadErrors(t *testing.T, p *Parser) {
	t.Helper()

	if err := parseCodexEdgeError(p, `{"type":"event_msg","payload":"bad"}`); err == nil {
		t.Fatal("expected error for invalid event payload")
	}
}

func assertAssistantResponseItemMessageParses(t *testing.T, p *Parser) {
	t.Helper()

	msg := parseCodexEdgeLine(t, p, `{"type":"response_item","timestamp":"2026-03-05T20:20:54.239Z","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}}`)
	if msg == nil || msg.Content != "done" || msg.Role != "assistant" {
		t.Fatalf("unexpected message: %+v", msg)
	}
}

func assertDeveloperResponseItemMessageSkipped(t *testing.T, p *Parser) {
	t.Helper()

	msg := parseCodexEdgeLine(t, p, `{"type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"ignore"}]}}`)
	if msg != nil {
		t.Fatalf("expected nil message, got %+v", msg)
	}
}

func assertCustomToolCallInvalidArgumentsRaw(t *testing.T, p *Parser) {
	t.Helper()

	msg := parseCodexEdgeLine(t, p, `{"type":"response_item","timestamp":"2026-03-05T20:20:54.282Z","payload":{"type":"custom_tool_call","name":"my_tool","arguments":"not-json"}}`)
	if msg == nil || len(msg.Tools) != 1 || msg.Tools[0].Input["raw"] != "not-json" {
		t.Fatalf("unexpected tool call: %+v", msg)
	}
}

func assertCustomToolCallOutputParses(t *testing.T, p *Parser) {
	t.Helper()

	msg := parseCodexEdgeLine(t, p, `{"type":"response_item","timestamp":"2026-03-05T20:20:54.381Z","payload":{"type":"custom_tool_call_output","output":"tool output"}}`)
	if msg == nil || len(msg.Tools) != 1 || msg.Tools[0].Output != "tool output" {
		t.Fatalf("unexpected tool output message: %+v", msg)
	}
}

func assertInvalidResponseItemPayloadErrors(t *testing.T, p *Parser) {
	t.Helper()

	if err := parseCodexEdgeError(p, `{"type":"response_item","payload":"bad"}`); err == nil {
		t.Fatal("expected error for invalid response_item payload")
	}
}

func assertInvalidSessionMetaPayloadErrors(t *testing.T, p *Parser) {
	t.Helper()

	if err := parseCodexEdgeError(p, `{"type":"session_meta","payload":"bad"}`); err == nil {
		t.Fatal("expected error for invalid session_meta payload")
	}
}

func assertUnsupportedResponseItemTypeSkipped(t *testing.T, p *Parser) {
	t.Helper()

	msg := parseCodexEdgeLine(t, p, `{"type":"response_item","payload":{"type":"reasoning"}}`)
	if msg != nil {
		t.Fatalf("expected nil message, got %+v", msg)
	}
}

func TestParser_HelperCoverage(t *testing.T) {
	p := NewParser()

	t.Run("extractTopLevelToolOutput", func(t *testing.T) {
		if got := p.extractTopLevelToolOutput(nil); got != "" {
			t.Fatalf("nil output = %q, want empty", got)
		}
		if got := p.extractTopLevelToolOutput("plain"); got != "plain" {
			t.Fatalf("string output = %q, want plain", got)
		}
		arr := []any{map[string]any{"text": "hello"}}
		if got := p.extractTopLevelToolOutput(arr); got != "hello" {
			t.Fatalf("array output = %q, want hello", got)
		}
		if got := p.extractTopLevelToolOutput(map[string]any{"ok": true}); !strings.Contains(got, `"ok":true`) {
			t.Fatalf("object output = %q, want JSON", got)
		}
		if got := p.extractTopLevelToolOutput(map[string]any{"bad": make(chan int)}); got != "" {
			t.Fatalf("marshal error output = %q, want empty", got)
		}
	})

	t.Run("parseCodexToolInput", func(t *testing.T) {
		if got := parseCodexToolInput(""); got != nil {
			t.Fatalf("empty input = %#v, want nil", got)
		}
		if got := parseCodexToolInput(`{"cmd":"pwd"}`); got["cmd"] != "pwd" {
			t.Fatalf("object input = %#v, want cmd", got)
		}
		if got := parseCodexToolInput(`[1,2]`); fmt.Sprint(got["value"]) != "[1 2]" {
			t.Fatalf("array input = %#v, want wrapped value", got)
		}
		if got := parseCodexToolInput(`oops`); got["raw"] != "oops" {
			t.Fatalf("invalid input = %#v, want raw fallback", got)
		}
	})

	t.Run("coalesce", func(t *testing.T) {
		if got := coalesce("", " ", "value"); got != "value" {
			t.Fatalf("coalesce = %q, want value", got)
		}
		if got := coalesce("", " "); got != "" {
			t.Fatalf("coalesce = %q, want empty", got)
		}
	})
}

type errorReader struct{}

func (errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("boom")
}

func TestParse_ReadError(t *testing.T) {
	p := NewParser()
	result, err := p.Parse(errorReader{})
	if err == nil {
		t.Fatal("expected read error")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !strings.Contains(err.Error(), "read jsonl") {
		t.Fatalf("error = %v, want read jsonl wrapper", err)
	}
}

func TestParseChannel_StrictMalformedStops(t *testing.T) {
	p := NewParser()
	p.SkipMalformed = false

	msgCh, errCh := p.ParseChannel(strings.NewReader("{bad json\n"))
	for range msgCh {
	}

	err := <-errCh
	if err == nil {
		t.Fatal("expected parse channel error")
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("error = %v, want line number", err)
	}
}

func TestParseChannel_ReadError(t *testing.T) {
	p := NewParser()

	msgCh, errCh := p.ParseChannel(errorReader{})
	for range msgCh {
	}

	err := <-errCh
	if err == nil {
		t.Fatal("expected read error")
	}
	if !strings.Contains(err.Error(), "read jsonl") {
		t.Fatalf("error = %v, want read jsonl wrapper", err)
	}
}

func TestParser_ParseClaudeToolUseWithoutToolDataReturnsNil(t *testing.T) {
	p := NewParser()
	msg, err := p.parseLine([]byte(`{"type":"tool_use","sessionId":"test","timestamp":"2026-01-24T10:00:00.000Z"}`), 1)
	if err != nil {
		t.Fatalf("parseLine failed: %v", err)
	}
	if msg != nil {
		t.Fatalf("expected nil message, got %+v", msg)
	}
}
