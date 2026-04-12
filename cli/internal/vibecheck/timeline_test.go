package vibecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTimeline_Format(t *testing.T) {
	// Simulate git log --format="%H|||%aI|||%an|||%s" --numstat output.
	const delim = "|||"
	raw := `abc123|||2026-02-15T10:00:00-05:00|||Alice|||feat: add vibecheck types
3	1	cli/internal/vibecheck/types.go
2	0	cli/internal/vibecheck/timeline.go

def456|||2026-02-15T09:30:00-05:00|||Bob|||fix: correct timestamp parsing
1	1	cli/internal/vibecheck/timeline.go

`

	events, err := parseGitLog(raw, delim)
	if err != nil {
		t.Fatalf("parseGitLog returned error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Events should be sorted newest first.
	first := events[0]
	if first.SHA != "abc123" {
		t.Errorf("expected first event SHA abc123, got %s", first.SHA)
	}
	if first.Author != "Alice" {
		t.Errorf("expected author Alice, got %s", first.Author)
	}
	if first.Message != "feat: add vibecheck types" {
		t.Errorf("expected message 'feat: add vibecheck types', got %q", first.Message)
	}
	if first.FilesChanged != 2 {
		t.Errorf("expected 2 files changed, got %d", first.FilesChanged)
	}
	if first.Insertions != 5 {
		t.Errorf("expected 5 insertions, got %d", first.Insertions)
	}
	if first.Deletions != 1 {
		t.Errorf("expected 1 deletion, got %d", first.Deletions)
	}

	second := events[1]
	if second.SHA != "def456" {
		t.Errorf("expected second event SHA def456, got %s", second.SHA)
	}
	if second.FilesChanged != 1 {
		t.Errorf("expected 1 file changed, got %d", second.FilesChanged)
	}
	if second.Insertions != 1 {
		t.Errorf("expected 1 insertion, got %d", second.Insertions)
	}
	if second.Deletions != 1 {
		t.Errorf("expected 1 deletion, got %d", second.Deletions)
	}
}

func TestTimelineEvent_Fields(t *testing.T) {
	now := time.Now()
	event := TimelineEvent{
		Timestamp:    now,
		SHA:          "abc123def456",
		Author:       "Test Author",
		Message:      "feat: test message",
		FilesChanged: 3,
		Insertions:   10,
		Deletions:    5,
		Tags:         []string{"v1.0.0", "latest"},
	}

	if event.Timestamp != now {
		t.Error("Timestamp mismatch")
	}
	if event.SHA != "abc123def456" {
		t.Error("SHA mismatch")
	}
	if event.Author != "Test Author" {
		t.Error("Author mismatch")
	}
	if event.Message != "feat: test message" {
		t.Error("Message mismatch")
	}
	if event.FilesChanged != 3 {
		t.Error("FilesChanged mismatch")
	}
	if event.Insertions != 10 {
		t.Error("Insertions mismatch")
	}
	if event.Deletions != 5 {
		t.Error("Deletions mismatch")
	}
	if len(event.Tags) != 2 || event.Tags[0] != "v1.0.0" || event.Tags[1] != "latest" {
		t.Error("Tags mismatch")
	}
}

func TestParseTimeline_EmptyOutput(t *testing.T) {
	events, err := parseGitLog("", "|||")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events for empty input, got %d", len(events))
	}
}

func TestParseTimeline_NoTrailingNewline(t *testing.T) {
	// Git log output without trailing blank line.
	raw := `aaa111|||2026-02-15T08:00:00-05:00|||Carol|||chore: cleanup
1	0	README.md`

	events, err := parseGitLog(raw, "|||")
	if err != nil {
		t.Fatalf("parseGitLog returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].SHA != "aaa111" {
		t.Errorf("expected SHA aaa111, got %s", events[0].SHA)
	}
	if events[0].FilesChanged != 1 {
		t.Errorf("expected 1 file changed, got %d", events[0].FilesChanged)
	}
}

func TestParseGitLog_ConsecutiveHeaders(t *testing.T) {
	// Exercise the "flush pending event without trailing blank line" path
	// (line 67-69): two header lines back-to-back without a blank separator.
	raw := `abc111|||2026-02-15T10:00:00-05:00|||Alice|||feat: first commit
def222|||2026-02-15T09:00:00-05:00|||Bob|||feat: second commit
`

	events, err := parseGitLog(raw, "|||")
	if err != nil {
		t.Fatalf("parseGitLog returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	// Sorted newest first.
	if events[0].SHA != "abc111" {
		t.Errorf("expected first event SHA abc111, got %s", events[0].SHA)
	}
	if events[1].SHA != "def222" {
		t.Errorf("expected second event SHA def222, got %s", events[1].SHA)
	}
}

func TestParseGitLog_InvalidTimestamp(t *testing.T) {
	// Exercise the timestamp parse error path (line 72-74).
	raw := `abc111|||not-a-valid-timestamp|||Alice|||feat: bad ts
`

	_, err := parseGitLog(raw, "|||")
	if err == nil {
		t.Fatal("expected error for invalid timestamp")
	}
}

func TestParseGitLog_MalformedNumstat(t *testing.T) {
	// A line that is neither a header nor a valid 3-field numstat should be ignored.
	raw := "abc111|||2025-01-15T10:00:00Z|||Alice|||feat: test\nnot-a-numstat-line\n\n"
	events, err := parseGitLog(raw, "|||")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].FilesChanged != 0 {
		t.Errorf("expected 0 files changed (malformed numstat ignored), got %d", events[0].FilesChanged)
	}
}

func TestParseTimeline_IgnoresPollutedGitDiscoveryEnv(t *testing.T) {
	repo := t.TempDir()
	if err := initGitRepo(repo); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	commitTime := time.Now().Add(-1 * time.Hour)
	if err := createTestCommit(repo, "fixture.txt", "feat: add fixture", commitTime); err != nil {
		t.Fatalf("failed to create fixture commit: %v", err)
	}

	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "wrong.git"))
	t.Setenv("GIT_WORK_TREE", t.TempDir())
	t.Setenv("GIT_COMMON_DIR", filepath.Join(t.TempDir(), "common"))

	events, err := ParseTimeline(repo, commitTime.Add(-1*time.Minute))
	if err != nil {
		t.Fatalf("ParseTimeline returned error with polluted git env: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Message != "feat: add fixture" {
		t.Fatalf("expected fixture commit, got %q", events[0].Message)
	}
}

func TestGitDiscoveryEnv_StripsGitDiscoveryVariables(t *testing.T) {
	t.Setenv("GIT_DIR", "/tmp/git-dir")
	t.Setenv("GIT_WORK_TREE", "/tmp/work-tree")
	t.Setenv("GIT_COMMON_DIR", "/tmp/common-dir")
	t.Setenv("KEEP_ME", "still-here")

	env := gitDiscoveryEnv()
	envMap := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		envMap[key] = value
	}

	if _, ok := envMap["GIT_DIR"]; ok {
		t.Fatal("expected GIT_DIR to be stripped from git discovery env")
	}
	if _, ok := envMap["GIT_WORK_TREE"]; ok {
		t.Fatal("expected GIT_WORK_TREE to be stripped from git discovery env")
	}
	if _, ok := envMap["GIT_COMMON_DIR"]; ok {
		t.Fatal("expected GIT_COMMON_DIR to be stripped from git discovery env")
	}
	if got := envMap["KEEP_ME"]; got != "still-here" {
		t.Fatalf("expected unrelated env var to survive, got %q", got)
	}
}

func TestGitDiscoveryEnv_PreservesEnvironmentWithoutGitOverrides(t *testing.T) {
	t.Setenv("PATH", os.Getenv("PATH"))
	t.Setenv("HOME", "/tmp/vibecheck-home")

	env := gitDiscoveryEnv()
	envMap := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		envMap[key] = value
	}

	if got := envMap["HOME"]; got != "/tmp/vibecheck-home" {
		t.Fatalf("expected HOME to be preserved, got %q", got)
	}
	if _, ok := envMap["PATH"]; !ok {
		t.Fatal("expected PATH to be preserved")
	}
}
