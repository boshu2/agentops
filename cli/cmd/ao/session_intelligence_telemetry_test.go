package main

import (
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestBuildCitationAggregate_DedupesRepeatedSignals(t *testing.T) {
	baseDir := t.TempDir()
	now := time.Now()
	aggregate := buildCitationAggregate(baseDir, []types.CitationEvent{
		{ArtifactPath: ".agents/learnings/a.md", WorkspacePath: ".", SessionID: "s1", CitedAt: now.Add(-2 * time.Hour), CitationType: "retrieved"},
		{ArtifactPath: ".agents/learnings/a.md", WorkspacePath: ".", SessionID: "s1", CitedAt: now.Add(-1 * time.Hour), CitationType: "retrieved"},
		{ArtifactPath: ".agents/learnings/a.md", WorkspacePath: ".", SessionID: "s2", CitedAt: now, CitationType: "applied", FeedbackGiven: true, FeedbackReward: 1},
	})

	if aggregate.TotalEvents != 3 {
		t.Fatalf("TotalEvents = %d, want 3", aggregate.TotalEvents)
	}
	if aggregate.DedupedEvents != 2 {
		t.Fatalf("DedupedEvents = %d, want 2", aggregate.DedupedEvents)
	}

	signal := usageSignalForArtifact(baseDir, ".agents/learnings/a.md", aggregate)
	if signal.UniqueSessions != 2 {
		t.Fatalf("UniqueSessions = %d, want 2", signal.UniqueSessions)
	}
	if signal.UniqueWorkspaces != 1 {
		t.Fatalf("UniqueWorkspaces = %d, want 1", signal.UniqueWorkspaces)
	}
	if signal.AppliedCount != 1 || signal.RetrievedCount != 1 {
		t.Fatalf("counts = retrieved:%d applied:%d, want 1/1", signal.RetrievedCount, signal.AppliedCount)
	}
	if signal.FeedbackCount != 1 || signal.MeanReward != 1 {
		t.Fatalf("feedback = count:%d mean:%v, want 1/1", signal.FeedbackCount, signal.MeanReward)
	}
}

func TestWorkspacePathFromAgentArtifactPath(t *testing.T) {
	got := workspacePathFromAgentArtifactPath("/tmp/repo/.agents/ao/sessions/test.md")
	if want := "/tmp/repo"; got != want {
		t.Fatalf("workspacePathFromAgentArtifactPath = %q, want %q", got, want)
	}
}

func TestAnnotateCitationMatch_BucketsConfidenceAndPreservesProvenance(t *testing.T) {
	event := annotateCitationMatch(types.CitationEvent{}, 0.72, "lookup:query")
	if event.MatchConfidence != 0.9 {
		t.Fatalf("MatchConfidence = %v, want 0.9", event.MatchConfidence)
	}
	if event.MatchProvenance != "lookup:query" {
		t.Fatalf("MatchProvenance = %q, want %q", event.MatchProvenance, "lookup:query")
	}

	low := annotateCitationMatch(types.CitationEvent{}, 0.3, "search:session")
	if low.MatchConfidence != 0.5 {
		t.Fatalf("low MatchConfidence = %v, want 0.5", low.MatchConfidence)
	}
}

func TestNormalizeCitationMatchConfidence_Boundaries(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{name: "preserve high canonical bucket", in: 0.9, want: 0.9},
		{name: "preserve medium canonical bucket", in: 0.7, want: 0.7},
		{name: "preserve low canonical bucket", in: 0.5, want: 0.5},
		{name: "raw high score buckets upward", in: 0.72, want: 0.9},
		{name: "raw medium score buckets upward", in: 0.52, want: 0.7},
		{name: "raw low score buckets upward", in: 0.01, want: 0.5},
		{name: "zero confidence", in: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeCitationMatchConfidence(tt.in); got != tt.want {
				t.Fatalf("normalizeCitationMatchConfidence(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
