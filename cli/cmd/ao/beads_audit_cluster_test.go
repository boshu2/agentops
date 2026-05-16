// Tests for the PERF-C2 (soc-2grz) one-pass git capture + walk-once content
// index that replaced the per-bead `git log` forks and per-pattern repo walks
// in `ao beads audit`.

// practices: [dora-metrics, lean-startup]
package main

import (
	"errors"
	"testing"
	"time"
)

func mustGitTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed := parseGitTime(s)
	if parsed.IsZero() {
		t.Fatalf("parseGitTime(%q) returned zero time", s)
	}
	return parsed
}

func TestCaptureAuditCommits_ParsesRecords(t *testing.T) {
	orig := execGitLog
	t.Cleanup(func() { execGitLog = orig })

	// Two commits: one with a body + two files, one with an empty body.
	out := "\x1eabc123\x1f2026-05-10T00:00:00Z\x1ffix: thing\x1fCloses soc-xyz\x1f\n" +
		"cli/a.go\ncli/b.go\n" +
		"\x1edef456\x1f2026-05-12T00:00:00Z\x1ffeat: other\x1f\x1f\n" +
		"docs/c.md"
	execGitLog = func(args ...string) (string, error) { return out, nil }

	commits := captureAuditCommits()
	if len(commits) != 2 {
		t.Fatalf("captureAuditCommits parsed %d commits, want 2", len(commits))
	}
	if commits[0].shortSHA != "abc123" || commits[0].subject != "fix: thing" {
		t.Errorf("commit 0 = %+v, want sha abc123 subject 'fix: thing'", commits[0])
	}
	if commits[0].body != "Closes soc-xyz" {
		t.Errorf("commit 0 body = %q, want 'Closes soc-xyz'", commits[0].body)
	}
	if _, ok := commits[0].files["cli/a.go"]; !ok {
		t.Errorf("commit 0 files = %v, want cli/a.go", commits[0].files)
	}
	if commits[1].body != "" {
		t.Errorf("commit 1 body = %q, want empty", commits[1].body)
	}
	if !commits[1].commitAt.Equal(mustGitTime(t, "2026-05-12T00:00:00Z")) {
		t.Errorf("commit 1 commitAt = %v, want 2026-05-12", commits[1].commitAt)
	}
}

func TestCaptureAuditCommits_EmptyOnGitError(t *testing.T) {
	orig := execGitLog
	t.Cleanup(func() { execGitLog = orig })
	execGitLog = func(args ...string) (string, error) { return "", errors.New("git unavailable") }
	if commits := captureAuditCommits(); commits != nil {
		t.Errorf("captureAuditCommits on git error = %v, want nil", commits)
	}
}

func TestGrepCommitsForID(t *testing.T) {
	commits := []auditCommit{
		{shortSHA: "c1", subject: "fix: soc-aaa in subject"},
		{shortSHA: "c2", subject: "feat: thing", body: "also touches soc-aaa here"},
		{shortSHA: "c3", subject: "unrelated work"},
		{shortSHA: "c4", subject: "soc-aaa again"},
		{shortSHA: "c5", subject: "soc-aaa fourth match"},
	}
	got := grepCommitsForID(commits, "soc-aaa")
	want := "c1 fix: soc-aaa in subject\nc2 feat: thing\nc4 soc-aaa again"
	if got != want {
		t.Errorf("grepCommitsForID = %q, want %q (first 3 matches)", got, want)
	}
	if grepCommitsForID(commits, "soc-zzz") != "" {
		t.Error("grepCommitsForID found a match for an absent ID")
	}
	if grepCommitsForID(commits, "") != "" {
		t.Error("grepCommitsForID matched on an empty ID")
	}
}

func TestFileChangesSinceCommits(t *testing.T) {
	commits := []auditCommit{
		{shortSHA: "new1", subject: "edit a", commitAt: mustGitTime(t, "2026-05-12T00:00:00Z"),
			files: map[string]struct{}{"cli/a.go": {}}},
		{shortSHA: "old1", subject: "edit a old", commitAt: mustGitTime(t, "2026-04-01T00:00:00Z"),
			files: map[string]struct{}{"cli/a.go": {}}},
		{shortSHA: "new2", subject: "edit b", commitAt: mustGitTime(t, "2026-05-13T00:00:00Z"),
			files: map[string]struct{}{"cli/b.go": {}}},
	}
	// Bead created 2026-05-01: only commits after that count.
	got := fileChangesSinceCommits(commits, "2026-05-01T00:00:00Z", []string{"cli/a.go"})
	if got != "new1 edit a" {
		t.Errorf("fileChangesSinceCommits = %q, want 'new1 edit a' (old1 predates creation)", got)
	}
	// A path no commit touched yields no evidence.
	if got := fileChangesSinceCommits(commits, "2026-05-01T00:00:00Z", []string{"cli/missing.go"}); got != "" {
		t.Errorf("fileChangesSinceCommits for untouched path = %q, want empty", got)
	}
}

func TestPatternExistsInIndex(t *testing.T) {
	index := map[string]string{
		"cli/a.go":    "package main\nfunc Alpha() {}\n",
		"docs/b.md":   "# Heading\nsome prose\n",
		"skills/c.sh": "echo hello\n",
	}
	if !patternExistsInIndex("func Alpha", index) {
		t.Error("patternExistsInIndex missed a pattern present in the index")
	}
	if patternExistsInIndex("func Omega", index) {
		t.Error("patternExistsInIndex matched a pattern absent from the index")
	}
	if patternExistsInIndex("", index) {
		t.Error("patternExistsInIndex matched on an empty pattern")
	}
}

func TestRepoContentCacheBuildsOnce(t *testing.T) {
	cache := &repoContentCache{}
	first := cache.index()
	// Stamp a sentinel into the backing map. If index() rebuilds, the second
	// call drops the sentinel; if it reuses the memoized map, it survives.
	first["__sentinel__"] = "marker"
	second := cache.index()
	if _, ok := second["__sentinel__"]; !ok {
		t.Error("repoContentCache rebuilt its index on the second index() call")
	}
}
