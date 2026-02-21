package main

import (
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestDeduplicateCitations(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		citations []types.CitationEvent
		wantCount int
	}{
		{
			name:      "empty",
			citations: []types.CitationEvent{},
			wantCount: 0,
		},
		{
			name: "no duplicates",
			citations: []types.CitationEvent{
				{ArtifactPath: "/a.md", SessionID: "s1", CitedAt: now},
				{ArtifactPath: "/b.md", SessionID: "s1", CitedAt: now},
			},
			wantCount: 2,
		},
		{
			name: "with duplicates",
			citations: []types.CitationEvent{
				{ArtifactPath: "/a.md", SessionID: "s1", CitedAt: now},
				{ArtifactPath: "/a.md", SessionID: "s2", CitedAt: now},
				{ArtifactPath: "/b.md", SessionID: "s1", CitedAt: now},
			},
			wantCount: 2,
		},
		{
			name: "all duplicates",
			citations: []types.CitationEvent{
				{ArtifactPath: "/a.md", SessionID: "s1", CitedAt: now},
				{ArtifactPath: "/a.md", SessionID: "s2", CitedAt: now},
				{ArtifactPath: "/a.md", SessionID: "s3", CitedAt: now},
			},
			wantCount: 1,
		},
		{
			name: "mixed relative and absolute path aliases",
			citations: []types.CitationEvent{
				{ArtifactPath: ".agents/learnings/L1.md", SessionID: "s1", CitedAt: now},
				{ArtifactPath: "/tmp/repo/.agents/learnings/L1.md", SessionID: "s2", CitedAt: now},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateCitations("/tmp/repo", tt.citations)
			if len(got) != tt.wantCount {
				t.Errorf("deduplicateCitations() returned %d, want %d", len(got), tt.wantCount)
			}
		})
	}
}
