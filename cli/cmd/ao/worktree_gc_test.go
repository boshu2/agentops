package main

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestRunIDFromWorktreePath(t *testing.T) {
	repoRoot := filepath.Join("/tmp", "agentops")

	got := runIDFromWorktreePath(repoRoot, filepath.Join("/tmp", "agentops-rpi-abc123"))
	if got != "abc123" {
		t.Fatalf("runIDFromWorktreePath = %q, want %q", got, "abc123")
	}

	if got := runIDFromWorktreePath(repoRoot, filepath.Join("/tmp", "otherrepo-rpi-abc123")); got != "" {
		t.Fatalf("unexpected run id for non-matching worktree: %q", got)
	}
}

func TestParseRPITmuxSessionRunID(t *testing.T) {
	tests := []struct {
		name      string
		session   string
		wantRunID string
		wantOK    bool
	}{
		{name: "valid p1", session: "ao-rpi-abc123-p1", wantRunID: "abc123", wantOK: true},
		{name: "valid hyphenated run id", session: "ao-rpi-my-run-42-p3", wantRunID: "my-run-42", wantOK: true},
		{name: "wrong phase", session: "ao-rpi-abc123-p4", wantOK: false},
		{name: "not rpi session", session: "convoy-123", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRunID, gotOK := parseRPITmuxSessionRunID(tt.session)
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotRunID != tt.wantRunID {
				t.Fatalf("runID = %q, want %q", gotRunID, tt.wantRunID)
			}
		})
	}
}

func TestParseTmuxSessionListOutput(t *testing.T) {
	now := time.Now().Unix()
	raw := "ao-rpi-run1-p1\t" + strconv.FormatInt(now-3600, 10)
	raw += "\nignore-this\t123"
	raw += "\nao-rpi-run2-p2\t" + strconv.FormatInt(now-7200, 10)

	sessions := parseTmuxSessionListOutput(raw)
	if len(sessions) != 2 {
		t.Fatalf("expected 2 parsed sessions, got %d", len(sessions))
	}
	if sessions[0].RunID != "run1" || sessions[1].RunID != "run2" {
		t.Fatalf("unexpected run IDs parsed: %+v", sessions)
	}
}

func TestShouldCleanupRPITmuxSession(t *testing.T) {
	now := time.Now()
	active := map[string]bool{"active-run": true}
	liveWorktrees := map[string]bool{"live-worktree-run": true}

	if shouldCleanupRPITmuxSession("active-run", now.Add(-2*time.Hour), now, time.Hour, active, liveWorktrees) {
		t.Fatal("should not cleanup active run session")
	}
	if shouldCleanupRPITmuxSession("live-worktree-run", now.Add(-2*time.Hour), now, time.Hour, active, liveWorktrees) {
		t.Fatal("should not cleanup session with live worktree")
	}
	if shouldCleanupRPITmuxSession("fresh-run", now.Add(-30*time.Minute), now, time.Hour, active, liveWorktrees) {
		t.Fatal("should not cleanup fresh session")
	}
	if !shouldCleanupRPITmuxSession("stale-orphan", now.Add(-2*time.Hour), now, time.Hour, active, liveWorktrees) {
		t.Fatal("expected stale orphan session to be cleaned")
	}
}
