package main

import (
	"path/filepath"
	"testing"
)

func TestCanonicalSessionIDNormalization(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "already canonical timestamp",
			in:   "session-20260221-123456",
			want: "session-20260221-123456",
		},
		{
			name: "bare timestamp gets prefixed",
			in:   "20260221-123456",
			want: "session-20260221-123456",
		},
		{
			name: "uuid is deterministic",
			in:   "2D608ACE-E8E4-4649-8AC0-70AEBA0DCFEE",
			want: "session-uuid-2d608ace-e8e4-4649-8ac0-70aeba0dcfee",
		},
		{
			name: "custom id preserved",
			in:   "worker-7",
			want: "worker-7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canonicalSessionID(tt.in); got != tt.want {
				t.Fatalf("canonicalSessionID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSessionIDAliases(t *testing.T) {
	aliases := sessionIDAliases("session-20260221-123456")
	set := make(map[string]bool)
	for _, alias := range aliases {
		set[alias] = true
	}
	if !set["session-20260221-123456"] {
		t.Fatalf("expected canonical alias in set")
	}
	if !set["20260221-123456"] {
		t.Fatalf("expected timestamp alias in set")
	}
}

func TestCanonicalArtifactPath(t *testing.T) {
	baseDir := "/tmp/repo"
	rel := ".agents/learnings/L1.md"
	absWant := filepath.Clean("/tmp/repo/.agents/learnings/L1.md")
	if got := canonicalArtifactPath(baseDir, rel); got != absWant {
		t.Fatalf("canonicalArtifactPath(%q) = %q, want %q", rel, got, absWant)
	}
	if got := canonicalArtifactPath(baseDir, absWant); got != absWant {
		t.Fatalf("canonicalArtifactPath(abs) = %q, want %q", got, absWant)
	}
}
