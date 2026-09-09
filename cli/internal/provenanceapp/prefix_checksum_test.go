package provenanceapp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/parser"
	"github.com/boshu2/agentops/cli/internal/types"
)

func TestPrefixChecksumLegacyDigests(t *testing.T) {
	// Freeze the checksums produced by the original buffered implementation.
	// These are compatibility oracles, including its omission of invalid inputs.
	orderedInput := make(map[string]any)
	orderedInput["z"] = 2
	orderedInput["a"] = "first"
	reorderedInput := make(map[string]any)
	reorderedInput["a"] = "first"
	reorderedInput["z"] = 2
	cases := []struct {
		name     string
		messages []types.TranscriptMessage
		upto     int
		want     string
	}{
		{name: "empty", want: "e3b0c44298fc1c14"},
		{name: "excluded", messages: []types.TranscriptMessage{{MessageIndex: 3, Type: "assistant"}}, upto: 2, want: "e3b0c44298fc1c14"},
		{name: "inclusive_cutoff", messages: []types.TranscriptMessage{
			{MessageIndex: 1, Type: "user"},
			{MessageIndex: 2, Type: "assistant", Tools: []types.ToolCall{{Name: "Bash", Input: map[string]any{"cmd": "echo ok"}, Output: "ok\n"}}},
			{MessageIndex: 3, Type: "user"},
		}, upto: 2, want: "dcab0a0347e887c7"},
		{name: "traversal_order", messages: []types.TranscriptMessage{
			{MessageIndex: 2, Type: "assistant", Tools: []types.ToolCall{{Name: "A", Output: "second"}}},
			{MessageIndex: 99, Type: "skip"},
			{MessageIndex: 1, Type: "assistant", Tools: []types.ToolCall{{Name: "B", Output: "first"}}},
		}, upto: 2, want: "9212d8b8d71c71c3"},
		{name: "map_keys", messages: []types.TranscriptMessage{{MessageIndex: 1, Type: "assistant", Tools: []types.ToolCall{{Name: "Read", Input: orderedInput}}}}, upto: 1, want: "637e7a404f42e186"},
		{name: "reordered_map_keys", messages: []types.TranscriptMessage{{MessageIndex: 1, Type: "assistant", Tools: []types.ToolCall{{Name: "Read", Input: reorderedInput}}}}, upto: 1, want: "637e7a404f42e186"},
		{name: "unicode_and_controls", messages: []types.TranscriptMessage{{MessageIndex: 1, Type: "assistant\x1f", Tools: []types.ToolCall{{Name: "読み🛠\x00", Input: map[string]any{"text": "雪\n<&\u2028", "control": "\x00\t"}, Output: "\xffα\n\r\t\x00,\x1f"}}}}, upto: 1, want: "280de88e987a2895"},
		{name: "unmarshalable_input", messages: []types.TranscriptMessage{{MessageIndex: 1, Type: "assistant", Tools: []types.ToolCall{{Name: "Bash", Input: map[string]any{"bad": func() {}}, Output: "retained"}}}}, upto: 1, want: "9cf829d1a81d839f"},
		{name: "outputs_and_tool_order", messages: []types.TranscriptMessage{{MessageIndex: 1, Type: "tool_result", Tools: []types.ToolCall{{Name: "Z", Output: "a,b\x1fc\n"}, {Name: "A", Input: map[string]any{}, Output: "last"}}}}, upto: 1, want: "8f4e7534d0ba304d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := prefixChecksum(&parser.ParseResult{Messages: tc.messages}, tc.upto)
			if got != tc.want {
				t.Fatalf("prefixChecksum() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPrefixChecksumLegacyOutputBoundaries(t *testing.T) {
	for _, tc := range []struct {
		size int
		want string
	}{
		{4095, "ccf0f9ac29114770"}, {4096, "83aa79bd39127ea2"}, {4097, "711a4b809194e7b5"}, {32769, "6c978517d6dd9edf"},
	} {
		t.Run(fmt.Sprint(tc.size), func(t *testing.T) {
			result := &parser.ParseResult{Messages: []types.TranscriptMessage{{MessageIndex: 1, Type: "tool_result", Tools: []types.ToolCall{{Name: "Bash", Output: strings.Repeat("x", tc.size)}}}}}
			if got := prefixChecksum(result, 1); got != tc.want {
				t.Fatalf("prefixChecksum() = %q, want %q", got, tc.want)
			}
		})
	}
}

var benchmarkPrefixChecksum string

func BenchmarkPrefixChecksumOutput(b *testing.B) {
	for _, size := range []int{1 << 20, 8 << 20} {
		b.Run(fmt.Sprintf("%dMiB", size>>20), func(b *testing.B) {
			// Keep message count fixed and allocate transcript bytes outside timing.
			result := &parser.ParseResult{Messages: []types.TranscriptMessage{{MessageIndex: 1, Type: "tool_result", Tools: []types.ToolCall{{Name: "Bash", Output: strings.Repeat("x", size)}}}}}
			b.ReportAllocs()
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkPrefixChecksum = prefixChecksum(result, 1)
			}
		})
	}
}
