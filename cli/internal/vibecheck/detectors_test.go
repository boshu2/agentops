package vibecheck

import (
	"testing"
	"time"
)

// makeEvent is a helper to create test TimelineEvents.
func makeEvent(sha string, minutesOffset int, msg string, files []string, ins, del int) TimelineEvent {
	base := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	return TimelineEvent{
		SHA:          sha,
		Timestamp:    base.Add(time.Duration(minutesOffset) * time.Minute),
		Author:       "Test Author",
		Message:      msg,
		FilesChanged: len(files),
		Files:        files,
		Insertions:   ins,
		Deletions:    del,
	}
}

// --- Tests Passing Lie ---

func TestDetectTestsLie_ClaimThenFix(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "feat: auth working now", []string{"auth.go"}, 10, 2),
		makeEvent("bbb", 15, "fix: auth edge case", []string{"auth.go"}, 3, 1),
	}

	findings := DetectTestsLie(events)
	if len(findings) == 0 {
		t.Fatal("expected at least one finding for claim+fix pattern")
	}
	if findings[0].Category != "tests-passing-lie" {
		t.Errorf("expected category tests-passing-lie, got %s", findings[0].Category)
	}
	if findings[0].Severity != "critical" {
		t.Errorf("expected severity critical, got %s", findings[0].Severity)
	}
}

func TestDetectTestsLie_TentativeNotFlagged(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "WIP: trying auth working", []string{"auth.go"}, 10, 2),
		makeEvent("bbb", 15, "fix: auth edge case", []string{"auth.go"}, 3, 1),
	}

	findings := DetectTestsLie(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings for tentative commit, got %d", len(findings))
	}
}

func TestDetectTestsLie_DifferentFilesNoLie(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "feat: auth done", []string{"auth.go"}, 10, 2),
		makeEvent("bbb", 15, "fix: unrelated bug", []string{"other.go"}, 3, 1),
	}

	findings := DetectTestsLie(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings for different files, got %d", len(findings))
	}
}

func TestDetectTestsLie_EmptyFilesNoLie(t *testing.T) {
	// File data unavailable (empty lists): the same-file relation cannot be
	// confirmed, so a fix must NOT be treated as contradicting the claim. This
	// pins the fix for age-mr1c: the prior empty/empty => true fail-open, combined
	// with a timeline parser bug that left every commit's Files empty, produced
	// tests-passing-lie false positives on unrelated temporally-adjacent commits.
	events := []TimelineEvent{
		makeEvent("aaa", 0, "feat: auth done", nil, 10, 2),
		makeEvent("bbb", 15, "fix: auth edge case", nil, 3, 1),
	}
	findings := DetectTestsLie(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings when file data is empty (cannot confirm same-file), got %d", len(findings))
	}
}

func TestDetectTestsLie_OutsideWindow(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "feat: all tests pass", []string{"auth.go"}, 10, 2),
		makeEvent("bbb", 60, "fix: auth bug", []string{"auth.go"}, 3, 1),
	}

	findings := DetectTestsLie(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings outside 30min window, got %d", len(findings))
	}
}

func TestDetectTestsLie_TooFewCommits(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "feat: all tests pass", []string{"auth.go"}, 10, 2),
	}

	findings := DetectTestsLie(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings for single commit, got %d", len(findings))
	}
}

// --- Context Amnesia ---

func TestDetectContextAmnesia_RepeatedEdits(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "feat: add handler", []string{"handler.go"}, 20, 0),
		makeEvent("bbb", 10, "fix: handler null check", []string{"handler.go"}, 5, 2),
		makeEvent("ccc", 20, "fix: handler again", []string{"handler.go"}, 5, 3),
		makeEvent("ddd", 30, "fix: handler edge case", []string{"handler.go"}, 4, 2),
	}

	findings := DetectContextAmnesia(events)
	if len(findings) == 0 {
		t.Fatal("expected findings for repeated edits to same file")
	}
	if findings[0].Category != "context-amnesia" {
		t.Errorf("expected category context-amnesia, got %s", findings[0].Category)
	}
	if findings[0].File != "handler.go" {
		t.Errorf("expected file handler.go, got %s", findings[0].File)
	}
}

func TestDetectContextAmnesia_DifferentFiles(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "feat: add handler", []string{"handler.go"}, 20, 0),
		makeEvent("bbb", 10, "feat: add router", []string{"router.go"}, 15, 0),
		makeEvent("ccc", 20, "feat: add service", []string{"service.go"}, 25, 0),
	}

	findings := DetectContextAmnesia(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings for different files, got %d", len(findings))
	}
}

func TestDetectContextAmnesia_OutsideWindow(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "feat: add handler", []string{"handler.go"}, 20, 0),
		makeEvent("bbb", 90, "fix: handler", []string{"handler.go"}, 5, 2),
		makeEvent("ccc", 180, "fix: handler again", []string{"handler.go"}, 5, 3),
	}

	findings := DetectContextAmnesia(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings outside 1hr window, got %d", len(findings))
	}
}

func TestDetectContextAmnesia_TooFewEvents(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "feat: add handler", []string{"handler.go"}, 20, 0),
		makeEvent("bbb", 10, "fix: handler", []string{"handler.go"}, 5, 2),
	}

	findings := DetectContextAmnesia(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings for too few events, got %d", len(findings))
	}
}

// --- Instruction Drift ---

func TestDetectInstructionDrift_RepeatedConfigEdits(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "chore: update CLAUDE.md", []string{"CLAUDE.md"}, 5, 2),
		makeEvent("bbb", 10, "chore: fix CLAUDE.md", []string{"CLAUDE.md"}, 3, 1),
		makeEvent("ccc", 20, "chore: tweak CLAUDE.md", []string{"CLAUDE.md"}, 2, 1),
	}

	findings := DetectInstructionDrift(events)
	if len(findings) == 0 {
		t.Fatal("expected findings for repeated CLAUDE.md edits")
	}
	if findings[0].Category != "instruction-drift" {
		t.Errorf("expected category instruction-drift, got %s", findings[0].Category)
	}
}

func TestDetectInstructionDrift_SKILLmd(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "chore: update skill", []string{"skills/vibe/SKILL.md"}, 5, 2),
		makeEvent("bbb", 10, "chore: fix skill", []string{"skills/vibe/SKILL.md"}, 3, 1),
		makeEvent("ccc", 20, "chore: tweak skill", []string{"skills/vibe/SKILL.md"}, 2, 1),
	}

	findings := DetectInstructionDrift(events)
	if len(findings) == 0 {
		t.Fatal("expected findings for repeated SKILL.md edits")
	}
}

func TestDetectInstructionDrift_NormalCode(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "feat: add handler", []string{"handler.go"}, 20, 0),
		makeEvent("bbb", 10, "fix: handler", []string{"handler.go"}, 5, 2),
		makeEvent("ccc", 20, "feat: add router", []string{"router.go"}, 15, 0),
	}

	findings := DetectInstructionDrift(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings for normal code, got %d", len(findings))
	}
}

func TestDetectInstructionDrift_BelowThreshold(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "chore: update CLAUDE.md", []string{"CLAUDE.md"}, 5, 2),
		makeEvent("bbb", 10, "feat: add handler", []string{"handler.go"}, 20, 0),
	}

	findings := DetectInstructionDrift(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings below threshold, got %d", len(findings))
	}
}

// --- Logging Only ---

func TestDetectLoggingOnly_ConsecutiveDebug(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "fix: add debug logging", nil, 5, 0),
		makeEvent("bbb", 5, "fix: more console.log", nil, 3, 0),
		makeEvent("ccc", 10, "fix: temp debug trace", nil, 4, 0),
	}

	findings := DetectLoggingOnly(events)
	if len(findings) == 0 {
		t.Fatal("expected findings for consecutive logging commits")
	}
	if findings[0].Category != "logging-only" {
		t.Errorf("expected category logging-only, got %s", findings[0].Category)
	}
}

func TestDetectLoggingOnly_InterruptedByFeature(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "fix: add debug logging", nil, 5, 0),
		makeEvent("bbb", 5, "fix: console.log", nil, 3, 0),
		makeEvent("ccc", 10, "feat: add new endpoint", nil, 50, 0),
		makeEvent("ddd", 15, "fix: temp debug", nil, 4, 0),
	}

	findings := DetectLoggingOnly(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings when streak interrupted, got %d", len(findings))
	}
}

func TestDetectLoggingOnly_LargeDiffNotFlagged(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "fix: add debug logging", nil, 50, 10),
		makeEvent("bbb", 5, "fix: more debug logging", nil, 40, 5),
		makeEvent("ccc", 10, "fix: logging investigation", nil, 60, 20),
	}

	findings := DetectLoggingOnly(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings for large diffs, got %d", len(findings))
	}
}

func TestDetectLoggingOnly_HealthyBaseline(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("aaa", 0, "feat: add handler", nil, 30, 0),
		makeEvent("bbb", 10, "feat: add router", nil, 25, 0),
		makeEvent("ccc", 20, "test: add unit tests", nil, 40, 0),
	}

	findings := DetectLoggingOnly(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings for healthy baseline, got %d", len(findings))
	}
}

// --- Aggregator ---

func TestDetectRunDetectors_AggregatesAll(t *testing.T) {
	events := []TimelineEvent{
		// Tests lie pattern: claim + fix.
		makeEvent("a1", 0, "feat: auth working now", []string{"auth.go"}, 10, 2),
		makeEvent("a2", 15, "fix: auth edge case", []string{"auth.go"}, 3, 1),
		// Logging pattern (3 consecutive small debug commits).
		makeEvent("b1", 30, "fix: add debug logging", nil, 5, 0),
		makeEvent("b2", 35, "fix: more console.log", nil, 3, 0),
		makeEvent("b3", 40, "fix: temp debug trace", nil, 4, 0),
	}

	findings := RunDetectors(events)
	if len(findings) < 2 {
		t.Errorf("expected at least 2 findings from aggregation, got %d", len(findings))
	}

	// Check that we have findings from multiple categories.
	cats := make(map[string]bool)
	for _, f := range findings {
		cats[f.Category] = true
	}
	if !cats["tests-passing-lie"] {
		t.Error("expected tests-passing-lie finding")
	}
	if !cats["logging-only"] {
		t.Error("expected logging-only finding")
	}
}

func TestDetectClassifyHealth_Critical(t *testing.T) {
	findings := []Finding{
		{Severity: "critical", Category: "tests-passing-lie", Message: "lie detected"},
		{Severity: "warning", Category: "logging-only", Message: "debug spiral"},
	}

	health := ClassifyHealth(findings)
	if health != "critical" {
		t.Errorf("expected critical, got %s", health)
	}
}

func TestDetectClassifyHealth_Warning(t *testing.T) {
	findings := []Finding{
		{Severity: "warning", Category: "logging-only", Message: "debug spiral"},
	}

	health := ClassifyHealth(findings)
	if health != "warning" {
		t.Errorf("expected warning, got %s", health)
	}
}

func TestDetectClassifyHealth_Healthy(t *testing.T) {
	findings := []Finding{}

	health := ClassifyHealth(findings)
	if health != "healthy" {
		t.Errorf("expected healthy, got %s", health)
	}
}

func TestHasFileOverlap_EmptyLists(t *testing.T) {
	// Exercise the len(a) == 0 || len(b) == 0 early return path (line 74-76).
	if hasFileOverlap(nil, []string{"auth.go"}) {
		t.Error("expected false when a is nil")
	}
	if hasFileOverlap([]string{"auth.go"}, nil) {
		t.Error("expected false when b is nil")
	}
	if hasFileOverlap([]string{}, []string{"auth.go"}) {
		t.Error("expected false when a is empty")
	}
	if hasFileOverlap([]string{"auth.go"}, []string{}) {
		t.Error("expected false when b is empty")
	}
}

func TestDetectTestsLie_NonFixFollowUp(t *testing.T) {
	// Exercise the !isFixMessage continue path (line 117-118).
	// Success claim followed by a non-fix commit within window, then nothing else.
	events := []TimelineEvent{
		makeEvent("aaa", 0, "feat: auth working now", []string{"auth.go"}, 10, 2),
		makeEvent("bbb", 10, "feat: add dashboard", []string{"auth.go"}, 20, 0), // NOT a fix
	}

	findings := DetectTestsLie(events)
	if len(findings) != 0 {
		t.Errorf("expected no findings when follow-up is not a fix, got %d", len(findings))
	}
}

func TestFilesRelated_BothEmpty(t *testing.T) {
	// Both empty: the same-file relation cannot be confirmed, so filesRelated is
	// FALSE. Corrected from a prior empty/empty => true fail-open that (with the
	// timeline parser leaving Files empty) produced tests-passing-lie false
	// positives on unrelated temporally-adjacent commits (age-mr1c).
	if filesRelated(nil, nil) {
		t.Error("expected filesRelated(nil, nil) == false (cannot confirm same-file relation)")
	}
	if filesRelated([]string{}, []string{}) {
		t.Error("expected filesRelated([], []) == false (cannot confirm same-file relation)")
	}
}

func TestFilesRelated_OneEmptyOneNot(t *testing.T) {
	// One empty, one not: no overlap, not both empty.
	if filesRelated(nil, []string{"a.go"}) {
		t.Error("expected filesRelated(nil, [a.go]) == false")
	}
	if filesRelated([]string{"a.go"}, nil) {
		t.Error("expected filesRelated([a.go], nil) == false")
	}
}

func TestFilesRelated_Overlap(t *testing.T) {
	if !filesRelated([]string{"a.go", "b.go"}, []string{"b.go", "c.go"}) {
		t.Error("expected overlap to return true")
	}
}

func TestFilesRelated_NoOverlap(t *testing.T) {
	if filesRelated([]string{"a.go"}, []string{"b.go"}) {
		t.Error("expected no overlap to return false")
	}
}

// --- isConfigFile ---

func TestIsConfigFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"CLAUDE.md", true},
		{"project/CLAUDE.md", true},
		{"skills/vibe/SKILL.md", true},
		{".claude/settings.json", true},
		{".github/workflows/ci.yml", true},
		{".agents/ao/state.json", true},
		{"tsconfig.json", true},
		{".eslintrc.json", true},
		{".prettierrc", true},
		{"renovate.json", true},
		{"handler.go", false},
		{"main.py", false},
		{"README.md", false},
	}
	for _, tt := range tests {
		got := isConfigFile(tt.path)
		if got != tt.want {
			t.Errorf("isConfigFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// --- isLoggingMessage ---

func TestIsLoggingMessage(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"add debug logging", true},
		{"adding log statements", true},
		{"debugging the issue", true},
		{"print statement cleanup", true},
		{"trace: enable tracing", true},
		{"console.log output", true},
		{"temporary workaround", true},
		{"temp fix for CI", true},
		{"WIP: investigating", true},
		{"diagnosis of failure", true},
		{"feat: add new endpoint", false},
		{"fix: correct auth logic", false},
		{"refactor: clean up", false},
	}
	for _, tt := range tests {
		got := isLoggingMessage(tt.msg)
		if got != tt.want {
			t.Errorf("isLoggingMessage(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}

// --- isSmallLoggingCommit ---

func TestIsSmallLoggingCommit(t *testing.T) {
	small := makeEvent("a", 0, "add debug logging", nil, 5, 3)
	if !isSmallLoggingCommit(small) {
		t.Error("expected small logging commit to match")
	}

	large := makeEvent("b", 0, "add debug logging", nil, 50, 10)
	if isSmallLoggingCommit(large) {
		t.Error("expected large diff to not match")
	}

	nonLogging := makeEvent("c", 0, "feat: new endpoint", nil, 5, 3)
	if isSmallLoggingCommit(nonLogging) {
		t.Error("expected non-logging message to not match")
	}

	zeroDiff := makeEvent("d", 0, "add debug logging", nil, 0, 0)
	if isSmallLoggingCommit(zeroDiff) {
		t.Error("expected zero-diff to not match")
	}
}

// --- maxConsecutiveRun ---

func TestMaxConsecutiveRun(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("a", 0, "debug: test", nil, 3, 0),
		makeEvent("b", 5, "debug: more", nil, 3, 0),
		makeEvent("c", 10, "feat: real work", nil, 30, 0),
		makeEvent("d", 15, "debug: again", nil, 3, 0),
	}
	pred := func(ev TimelineEvent) bool { return isLoggingMessage(ev.Message) }
	got := maxConsecutiveRun(events, pred)
	if got != 2 {
		t.Errorf("maxConsecutiveRun = %d, want 2", got)
	}

	if maxConsecutiveRun(nil, pred) != 0 {
		t.Error("expected 0 for nil events")
	}
}

// --- claimsSuccess / isTentative / isFixMessage ---

func TestClaimsSuccess(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"fixed the bug", true},
		{"it's working now", true},
		{"done with auth", true},
		{"tests pass", true},
		{"all tests green", true},
		{"successfully deployed", true},
		{"should work now", true},
		{"it works", true},
		{"feat: add new thing", false},
		{"refactor: clean up", false},
	}
	for _, tt := range tests {
		got := claimsSuccess(tt.msg)
		if got != tt.want {
			t.Errorf("claimsSuccess(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}

func TestIsTentative(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"try this approach", true},
		{"attempt to fix", true},
		{"maybe this works", true},
		{"WIP: auth flow", true},
		{"work in progress", true},
		{"experiment with cache", true},
		{"testing new approach", true},
		{"debug the issue", true},
		{"investigating failure", true},
		{"feat: add login", false},
		{"fix: correct typo", false},
	}
	for _, tt := range tests {
		got := isTentative(tt.msg)
		if got != tt.want {
			t.Errorf("isTentative(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}

func TestIsFixMessage(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"fix: auth bug", true},
		{"Fix: capital", true},
		{"fixed the issue", true},
		{"bugfix: memory leak", true},
		{"hotfix: critical error", true},
		{"patch: version bump", true},
		{"feat: new feature", false},
		{"refactor: clean up", false},
	}
	for _, tt := range tests {
		got := isFixMessage(tt.msg)
		if got != tt.want {
			t.Errorf("isFixMessage(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}

// --- countConfigEdits ---

func TestCountConfigEdits(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("a", 0, "update config", []string{"CLAUDE.md", "handler.go"}, 5, 2),
		makeEvent("b", 10, "fix config", []string{"CLAUDE.md"}, 3, 1),
		makeEvent("c", 20, "feat: code", []string{"main.go"}, 20, 0),
	}
	counts := countConfigEdits(events)
	if counts["CLAUDE.md"] != 2 {
		t.Errorf("expected CLAUDE.md count 2, got %d", counts["CLAUDE.md"])
	}
	if counts["handler.go"] != 0 {
		t.Errorf("expected handler.go count 0, got %d", counts["handler.go"])
	}
}

// --- buildFileEdits ---

func TestBuildFileEdits(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("a1", 0, "edit handler", []string{"handler.go", "util.go"}, 10, 2),
		makeEvent("a2", 10, "fix handler", []string{"handler.go"}, 5, 1),
	}
	edits := buildFileEdits(events)
	if len(edits["handler.go"]) != 2 {
		t.Errorf("expected 2 edits for handler.go, got %d", len(edits["handler.go"]))
	}
	if len(edits["util.go"]) != 1 {
		t.Errorf("expected 1 edit for util.go, got %d", len(edits["util.go"]))
	}
}

// --- ErrRepoPathRequired ---

func TestErrRepoPathRequired(t *testing.T) {
	if ErrRepoPathRequired == nil {
		t.Fatal("ErrRepoPathRequired should not be nil")
	}
	if ErrRepoPathRequired.Error() != "RepoPath is required" {
		t.Errorf("unexpected error message: %s", ErrRepoPathRequired.Error())
	}
}

// --- DetectLoggingOnly edge cases ---

func TestDetectLoggingOnly_EmptyEvents(t *testing.T) {
	findings := DetectLoggingOnly(nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings for nil events, got %d", len(findings))
	}
}

// --- DetectContextAmnesia with exactly amnesiaMinEdits ---

func TestDetectContextAmnesia_ExactThreshold(t *testing.T) {
	events := []TimelineEvent{
		makeEvent("a", 0, "edit handler", []string{"handler.go"}, 10, 0),
		makeEvent("b", 15, "fix handler", []string{"handler.go"}, 5, 2),
		makeEvent("c", 30, "fix handler again", []string{"handler.go"}, 5, 3),
	}
	findings := DetectContextAmnesia(events)
	if len(findings) == 0 {
		t.Error("expected finding for exactly 3 edits within window")
	}
}
