// practices: [dora-metrics, lean-startup]
package main

import (
	"os"
	"path/filepath"
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

func TestWriteCitations(t *testing.T) {
	now := time.Now()

	t.Run("writes citations to file", func(t *testing.T) {
		dir := t.TempDir()
		citations := []types.CitationEvent{
			{ArtifactPath: "/a.md", SessionID: "s1", CitedAt: now, CitationType: "retrieved"},
			{ArtifactPath: "/b.md", SessionID: "s1", CitedAt: now, CitationType: "applied"},
		}

		if err := writeCitations(dir, citations); err != nil {
			t.Fatalf("writeCitations: %v", err)
		}

		citationsPath := filepath.Join(dir, ".agents", "ao", "citations.jsonl")
		content, err := os.ReadFile(citationsPath)
		if err != nil {
			t.Fatalf("read citations file: %v", err)
		}

		lines := splitNonEmpty(string(content))
		if len(lines) != 2 {
			t.Errorf("expected 2 lines, got %d", len(lines))
		}
	})

	t.Run("empty citations creates empty file", func(t *testing.T) {
		dir := t.TempDir()
		if err := writeCitations(dir, []types.CitationEvent{}); err != nil {
			t.Fatalf("writeCitations: %v", err)
		}
		citationsPath := filepath.Join(dir, ".agents", "ao", "citations.jsonl")
		if _, err := os.Stat(citationsPath); err != nil {
			t.Errorf("expected file to exist even for empty citations: %v", err)
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		dir := t.TempDir()
		// Write 2 citations first
		initial := []types.CitationEvent{
			{ArtifactPath: "/a.md", SessionID: "s1", CitedAt: now},
			{ArtifactPath: "/b.md", SessionID: "s1", CitedAt: now},
		}
		if err := writeCitations(dir, initial); err != nil {
			t.Fatalf("first write: %v", err)
		}

		// Overwrite with just 1 citation
		updated := []types.CitationEvent{
			{ArtifactPath: "/c.md", SessionID: "s2", CitedAt: now},
		}
		if err := writeCitations(dir, updated); err != nil {
			t.Fatalf("second write: %v", err)
		}

		citationsPath := filepath.Join(dir, ".agents", "ao", "citations.jsonl")
		content, _ := os.ReadFile(citationsPath)
		lines := splitNonEmpty(string(content))
		if len(lines) != 1 {
			t.Errorf("expected 1 line after overwrite, got %d", len(lines))
		}
	})
}
