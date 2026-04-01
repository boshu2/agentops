package main

import (
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestRerankContextBundleForPhase_PrefersWidelyReusedLearning(t *testing.T) {
	tmp := t.TempDir()
	if err := writeCitations(tmp, []types.CitationEvent{
		{ArtifactPath: tmp + "/.agents/learnings/reused.md", WorkspacePath: tmp, SessionID: "s1", CitedAt: time.Now().Add(-2 * time.Hour), CitationType: "applied", FeedbackGiven: true, FeedbackReward: 1},
		{ArtifactPath: tmp + "/.agents/learnings/reused.md", WorkspacePath: tmp, SessionID: "s2", CitedAt: time.Now().Add(-1 * time.Hour), CitationType: "reference"},
	}); err != nil {
		t.Fatal(err)
	}

	bundle := rankedContextBundle{
		CWD:   tmp,
		Query: "auth startup",
		Learnings: []learning{
			{ID: "L-fresh", Title: "Fresh auth note", Summary: "Recent auth startup note", Source: tmp + "/.agents/learnings/fresh.md", AgeWeeks: 1, CompositeScore: 0.9},
			{ID: "L-reused", Title: "Reused auth fix", Summary: "Auth startup fix used before", Source: tmp + "/.agents/learnings/reused.md", AgeWeeks: 3, CompositeScore: 0.6},
		},
	}

	ranked := rerankContextBundleForPhase(tmp, "auth startup", "startup", bundle)
	if got, want := ranked.Learnings[0].ID, "L-reused"; got != want {
		t.Fatalf("first learning = %q, want %q", got, want)
	}
}

func TestRerankContextBundleForPhase_UsesCassHitsForSessions(t *testing.T) {
	orig := runtimeSessionSearchFn
	runtimeSessionSearchFn = func(query string, limit int) ([]searchResult, error) {
		return []searchResult{
			{Path: "/tmp/repo/.agents/ao/sessions/matched.jsonl", Score: 9},
		}, nil
	}
	defer func() { runtimeSessionSearchFn = orig }()

	bundle := rankedContextBundle{
		CWD:   "/tmp/repo",
		Query: "startup context",
		RecentSessions: []session{
			{Path: "/tmp/repo/.agents/ao/sessions/older.jsonl", Summary: "Older summary"},
			{Path: "/tmp/repo/.agents/ao/sessions/matched.jsonl", Summary: "Matched lexical session"},
		},
	}

	ranked := rerankContextBundleForPhase("/tmp/repo", "startup context", "startup", bundle)
	if got, want := ranked.RecentSessions[0].Path, "/tmp/repo/.agents/ao/sessions/matched.jsonl"; got != want {
		t.Fatalf("first session = %q, want %q", got, want)
	}
}
