package bridge

import (
	"strings"
	"testing"
)

func TestNormalizeCodexLifecyclePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
		{"cleans path", "/tmp/../tmp/file.txt", "/tmp/file.txt"},
		{"trims whitespace", "  /tmp/file.txt  ", "/tmp/file.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeCodexLifecyclePath(tt.path)
			if tt.want != "" && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if tt.want == "" && got != "" {
				t.Errorf("got %q, want empty", got)
			}
		})
	}
}

func TestFirstNonEmptyTrimmed(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"first non-empty", []string{"", "  ", "hello", "world"}, "hello"},
		{"all empty", []string{"", "  "}, ""},
		{"no values", nil, ""},
		{"first is valid", []string{"first", "second"}, "first"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FirstNonEmptyTrimmed(tt.values...)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCodexStopAlreadyClosed(t *testing.T) {
	tests := []struct {
		name               string
		lastStopSessionID  string
		lastStopTranscript string
		sessionID          string
		transcriptPath     string
		want               bool
	}{
		{
			name:               "matching transcripts same session",
			lastStopSessionID:  "s1",
			lastStopTranscript: "/tmp/transcript.md",
			sessionID:          "s1",
			transcriptPath:     "/tmp/transcript.md",
			want:               true,
		},
		{
			name:               "matching transcripts no session ID",
			lastStopSessionID:  "",
			lastStopTranscript: "/tmp/transcript.md",
			sessionID:          "",
			transcriptPath:     "/tmp/transcript.md",
			want:               true,
		},
		{
			name:               "different transcripts",
			lastStopSessionID:  "s1",
			lastStopTranscript: "/tmp/old.md",
			sessionID:          "s1",
			transcriptPath:     "/tmp/new.md",
			want:               false,
		},
		{
			name:               "session ID match no transcripts",
			lastStopSessionID:  "s1",
			lastStopTranscript: "",
			sessionID:          "s1",
			transcriptPath:     "",
			want:               true,
		},
		{
			name:               "different session IDs no transcripts",
			lastStopSessionID:  "s1",
			lastStopTranscript: "",
			sessionID:          "s2",
			transcriptPath:     "",
			want:               false,
		},
		{
			name:               "no session IDs no transcripts",
			lastStopSessionID:  "",
			lastStopTranscript: "",
			sessionID:          "",
			transcriptPath:     "",
			want:               false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CodexStopAlreadyClosed(tt.lastStopSessionID, tt.lastStopTranscript, tt.sessionID, tt.transcriptPath)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureStopReason(t *testing.T) {
	if got := EnsureStopReason("already_closed"); !strings.Contains(got, "already recorded") {
		t.Errorf("unexpected reason for already_closed: %q", got)
	}
	if got := EnsureStopReason("new_close"); !strings.Contains(got, "recorded") {
		t.Errorf("unexpected reason for new_close: %q", got)
	}
}

func TestParseSemverParts(t *testing.T) {
	tests := []struct {
		version string
		want    [3]int
	}{
		{"1.2.3", [3]int{1, 2, 3}},
		{"v1.2.3", [3]int{1, 2, 3}},
		{"0.13.0", [3]int{0, 13, 0}},
		{"1.0.0-beta.1", [3]int{1, 0, 0}},
		{"2.0", [3]int{2, 0, 0}},
		{"", [3]int{0, 0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := ParseSemverParts(tt.version)
			if got != tt.want {
				t.Errorf("ParseSemverParts(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"0.13.0", "0.12.0", 1},
		{"0.12.0", "0.13.0", -1},
		{"v1.0.0", "1.0.0", 0},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := CompareSemver(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
