package context

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemainingPercent(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0.0, 1.0},
		{0.25, 0.75},
		{1.0, 0.0},
		{1.5, 0.0},
		{-0.1, 1.0},
	}
	for _, tc := range cases {
		if got := RemainingPercent(tc.in); got != tc.want {
			t.Errorf("RemainingPercent(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestReadinessForUsage(t *testing.T) {
	cases := []struct {
		usage float64
		want  string
	}{
		{0.0, ReadinessGreen},
		{0.2, ReadinessGreen},
		{0.35, ReadinessAmber},
		{0.55, ReadinessRed},
		{0.80, ReadinessCritical},
		{1.0, ReadinessCritical},
	}
	for _, tc := range cases {
		if got := ReadinessForUsage(tc.usage); got != tc.want {
			t.Errorf("ReadinessForUsage(%v) = %q, want %q", tc.usage, got, tc.want)
		}
	}
}

func TestReadinessAction(t *testing.T) {
	cases := map[string]string{
		ReadinessGreen:    "carry_on",
		ReadinessAmber:    "finish_current_scope",
		ReadinessRed:      "relief_on_station",
		ReadinessCritical: "immediate_relief",
		"UNKNOWN":         "immediate_relief",
	}
	for in, want := range cases {
		if got := ReadinessAction(in); got != want {
			t.Errorf("ReadinessAction(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReadinessRank(t *testing.T) {
	if ReadinessRank(ReadinessCritical) != 0 {
		t.Error("CRITICAL should rank 0")
	}
	if ReadinessRank(ReadinessRed) != 1 {
		t.Error("RED should rank 1")
	}
	if ReadinessRank(ReadinessAmber) != 2 {
		t.Error("AMBER should rank 2")
	}
	if ReadinessRank(ReadinessGreen) != 3 {
		t.Error("GREEN should rank 3")
	}
	if ReadinessRank("UNKNOWN") != 4 {
		t.Error("unknown should rank 4")
	}
	// Whitespace tolerated
	if ReadinessRank(" CRITICAL ") != 0 {
		t.Error("whitespace should not break ranking")
	}
}

func TestActionForStatus(t *testing.T) {
	// Stale + non-optimal -> recover
	if got := ActionForStatus("warning", true, "ok", "crit", "warning"); got != "recover_dead_session" {
		t.Errorf("got %q", got)
	}
	// Critical
	if got := ActionForStatus("crit", false, "ok", "crit", "warning"); got != "handoff_now" {
		t.Errorf("got %q", got)
	}
	// Warning
	if got := ActionForStatus("warning", false, "ok", "crit", "warning"); got != "checkpoint_and_prepare_handoff" {
		t.Errorf("got %q", got)
	}
	// Stale + optimal
	if got := ActionForStatus("ok", true, "ok", "crit", "warning"); got != "investigate_stale_session" {
		t.Errorf("got %q", got)
	}
	// Normal
	if got := ActionForStatus("ok", false, "ok", "crit", "warning"); got != "continue" {
		t.Errorf("got %q", got)
	}
}

func TestNonZeroOrDefault(t *testing.T) {
	if got := NonZeroOrDefault(5, 10); got != 5 {
		t.Errorf("got %d", got)
	}
	if got := NonZeroOrDefault(0, 10); got != 10 {
		t.Errorf("got %d", got)
	}
	if got := NonZeroOrDefault(-1, 10); got != 10 {
		t.Errorf("got %d", got)
	}
}

func TestTruncateDisplay(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"hello world", 8, "hello..."},
		{"abc", 2, "ab"}, // max<=3 just truncates
	}
	for _, tc := range cases {
		if got := TruncateDisplay(tc.in, tc.max); got != tc.want {
			t.Errorf("TruncateDisplay(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}

func TestDisplayOrDash(t *testing.T) {
	if got := DisplayOrDash(""); got != "-" {
		t.Errorf("empty: got %q", got)
	}
	if got := DisplayOrDash("   "); got != "-" {
		t.Errorf("whitespace: got %q", got)
	}
	if got := DisplayOrDash("value"); got != "value" {
		t.Errorf("got %q", got)
	}
}

func TestNormalizeLine(t *testing.T) {
	cases := map[string]string{
		"  hello\nworld\n":  "hello world",
		"a\tb\rc":           "a b c",
		"   multi   space ": "multi space",
		"":                  "",
	}
	for in, want := range cases {
		if got := NormalizeLine(in); got != want {
			t.Errorf("NormalizeLine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeForFilename(t *testing.T) {
	cases := map[string]string{
		"simple":           "simple",
		"foo/bar baz":      "foo-bar-baz",
		"multiple!!chars!!":"multiple-chars",
		"   ":              "session",
		"":                 "session",
		"---leading":       "leading",
	}
	for in, want := range cases {
		if got := SanitizeForFilename(in); got != want {
			t.Errorf("SanitizeForFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToRepoRelative(t *testing.T) {
	cwd := "/repo"
	if got := ToRepoRelative(cwd, ""); got != "" {
		t.Errorf("empty: got %q", got)
	}
	if got := ToRepoRelative(cwd, "/repo/a/b.txt"); got != "a/b.txt" {
		t.Errorf("got %q", got)
	}
}

func TestExtractIssueID(t *testing.T) {
	cases := map[string]string{
		"context ag-abc123 blah":  "ag-abc123",
		"context AG-XYZ99 blah":   "ag-xyz99",
		"no issue here":           "",
		"ag-alphanum99":           "ag-alphanum99",
	}
	for in, want := range cases {
		if got := ExtractIssueID(in); got != want {
			t.Errorf("ExtractIssueID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEstimateTokensFromChars(t *testing.T) {
	if got := EstimateTokensFromChars("", 4); got != 0 {
		t.Errorf("empty: got %d", got)
	}
	if got := EstimateTokensFromChars("0123456789ab", 4); got != 3 {
		t.Errorf("12 chars / 4 = 3; got %d", got)
	}
	// Small inputs round up to 1
	if got := EstimateTokensFromChars("xy", 4); got != 1 {
		t.Errorf("short: got %d", got)
	}
	// Default charsPerToken when <=0
	if got := EstimateTokensFromChars("01234567", 0); got != 2 {
		t.Errorf("default: got %d", got)
	}
}

func TestParseTimestamp(t *testing.T) {
	// RFC3339
	ts := ParseTimestamp("2026-04-22T12:00:00Z")
	if ts.IsZero() {
		t.Error("valid RFC3339 should parse")
	}
	// RFC3339Nano
	ts2 := ParseTimestamp("2026-04-22T12:00:00.123456789Z")
	if ts2.IsZero() {
		t.Error("valid RFC3339Nano should parse")
	}
	// Empty -> zero
	if !ParseTimestamp("").IsZero() {
		t.Error("empty should zero")
	}
	// Invalid -> zero
	if !ParseTimestamp("not a time").IsZero() {
		t.Error("invalid should zero")
	}
	// UTC
	ts3 := ParseTimestamp("2026-04-22T14:00:00+02:00")
	if ts3.Location() != time.UTC {
		t.Errorf("loc = %v", ts3.Location())
	}
}

func TestExtractTextContent_String(t *testing.T) {
	raw := json.RawMessage(`"  hello world  "`)
	if got := ExtractTextContent(raw); got != "hello world" {
		t.Errorf("got %q", got)
	}
}

func TestExtractTextContent_Array(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"first content"},{"type":"text","text":"second"}]`)
	if got := ExtractTextContent(raw); got != "first content" {
		t.Errorf("got %q", got)
	}
}

func TestExtractTextContent_Empty(t *testing.T) {
	if got := ExtractTextContent(json.RawMessage("")); got != "" {
		t.Errorf("empty: got %q", got)
	}
	if got := ExtractTextContent(json.RawMessage("   ")); got != "" {
		t.Errorf("whitespace: got %q", got)
	}
	// Empty array
	if got := ExtractTextContent(json.RawMessage(`[]`)); got != "" {
		t.Errorf("empty array: got %q", got)
	}
	// Invalid JSON
	if got := ExtractTextContent(json.RawMessage("not json")); got != "" {
		t.Errorf("invalid: got %q", got)
	}
}

func TestScanTailLines(t *testing.T) {
	data := []byte("line1\nline2\nline3")
	lines, err := ScanTailLines(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}
	if lines[0] != "line1" || lines[2] != "line3" {
		t.Errorf("lines = %v", lines)
	}
}

func TestReadFileTail_FullFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "f.txt")
	_ = os.WriteFile(path, []byte("abcdef"), 0o600)

	got, err := ReadFileTail(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "abcdef" {
		t.Errorf("got %q", string(got))
	}
}

func TestReadFileTail_TailsLargeFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "f.txt")
	content := "short1\nshort2\nshort3\nshort4\n"
	_ = os.WriteFile(path, []byte(content), 0o600)

	got, err := ReadFileTail(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Tail should be smaller than full content
	if len(got) > len(content) {
		t.Errorf("tail larger than file: %d > %d", len(got), len(content))
	}
	// Should NOT begin mid-line (empty is valid when starting byte falls on boundary)
	s := string(got)
	if s != "" && strings.HasPrefix(s, "ort") {
		t.Errorf("mid-line start: %q", s)
	}
}

func TestReadFileTail_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.txt")
	_ = os.WriteFile(path, []byte(""), 0o600)

	got, err := ReadFileTail(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %d bytes, want 0", len(got))
	}
}

func TestReadFileTail_MissingFile(t *testing.T) {
	_, err := ReadFileTail("/nonexistent/xyz.txt", 100)
	if err == nil {
		t.Error("expected error")
	}
}

func TestTmuxTargetFromPaneID(t *testing.T) {
	cases := map[string]string{
		"":           "",
		"in-process": "",
		"session:window.0": "session:window",
		"simple":     "simple",
	}
	for in, want := range cases {
		if got := TmuxTargetFromPaneID(in); got != want {
			t.Errorf("TmuxTargetFromPaneID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTmuxSessionFromTarget(t *testing.T) {
	cases := map[string]string{
		"":                  "",
		"my-session":        "my-session",
		"my-session:window": "my-session",
		"  spaced:win  ":    "spaced",
	}
	for in, want := range cases {
		if got := TmuxSessionFromTarget(in); got != want {
			t.Errorf("TmuxSessionFromTarget(%q) = %q, want %q", in, got, want)
		}
	}
}
