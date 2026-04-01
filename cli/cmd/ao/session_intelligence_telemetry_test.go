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
