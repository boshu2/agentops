package overnight

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadBudget_MissingReturnsDefault(t *testing.T) {
	repo := t.TempDir()
	state, reason := ReadBudget(repo)
	if reason != "" {
		t.Fatalf("missing file should rescue silently, got reason=%q", reason)
	}
	if state.Remaining != DefaultWarnOnlyBudget {
		t.Fatalf("Remaining=%d want %d", state.Remaining, DefaultWarnOnlyBudget)
	}
	if state.InitialBudget != DefaultWarnOnlyBudget {
		t.Fatalf("InitialBudget=%d want %d", state.InitialBudget, DefaultWarnOnlyBudget)
	}
	if state.Version != 1 {
		t.Fatalf("Version=%d want 1", state.Version)
	}
}

func TestReadBudget_CorruptReturnsDefaultWithReason(t *testing.T) {
	repo := t.TempDir()
	path := WarnOnlyBudgetPath(repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, reason := ReadBudget(repo)
	if reason == "" {
		t.Fatal("corrupt file must produce a rescue reason")
	}
	if !strings.Contains(reason, "corrupt") {
		t.Fatalf("rescue reason=%q should mention corruption", reason)
	}
	if state.Remaining != DefaultWarnOnlyBudget {
		t.Fatalf("Remaining=%d want %d", state.Remaining, DefaultWarnOnlyBudget)
	}
}

func TestReadBudget_OutOfRangeNegativeClamps(t *testing.T) {
	repo := t.TempDir()
	raw, _ := json.Marshal(WarnOnlyBudgetState{Version: 1, Remaining: -5, InitialBudget: 3})
	path := WarnOnlyBudgetPath(repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	state, reason := ReadBudget(repo)
	if state.Remaining != 0 {
		t.Fatalf("Remaining=%d want 0", state.Remaining)
	}
	if !strings.Contains(reason, "negative") {
		t.Fatalf("rescue reason=%q should mention negative", reason)
	}
}

func TestReadBudget_OutOfRangeOverCeilingClamps(t *testing.T) {
	repo := t.TempDir()
	raw, _ := json.Marshal(WarnOnlyBudgetState{Version: 1, Remaining: 99, InitialBudget: 3})
	path := WarnOnlyBudgetPath(repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	state, reason := ReadBudget(repo)
	if state.Remaining != 3 {
		t.Fatalf("Remaining=%d want 3", state.Remaining)
	}
	if !strings.Contains(reason, "out-of-range") {
		t.Fatalf("rescue reason=%q should mention out-of-range", reason)
	}
}

func TestWriteBudget_AtomicAndReadable(t *testing.T) {
	repo := t.TempDir()
	want := WarnOnlyBudgetState{
		Version:       1,
		Remaining:     2,
		InitialBudget: 3,
		LastResetAt:   "2026-04-10T12:00:00Z",
	}
	if err := WriteBudget(repo, want); err != nil {
		t.Fatalf("WriteBudget: %v", err)
	}
	got, reason := ReadBudget(repo)
	if reason != "" {
		t.Fatalf("round-trip rescue reason=%q", reason)
	}
	if got.Remaining != want.Remaining || got.InitialBudget != want.InitialBudget {
		t.Fatalf("round-trip mismatch: got=%+v want=%+v", got, want)
	}
	if got.LastResetAt != want.LastResetAt {
		t.Fatalf("LastResetAt round-trip: got=%q want=%q", got.LastResetAt, want.LastResetAt)
	}
	// Verify no stray .tmp files left behind by the atomic write.
	entries, err := os.ReadDir(filepath.Dir(WarnOnlyBudgetPath(repo)))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}

func TestResetBudget_WritesInitialAndTimestamp(t *testing.T) {
	repo := t.TempDir()
	state, err := ResetBudget(repo, 5)
	if err != nil {
		t.Fatalf("ResetBudget: %v", err)
	}
	if state.Remaining != 5 || state.InitialBudget != 5 {
		t.Fatalf("state=%+v want Remaining=5 InitialBudget=5", state)
	}
	if state.LastResetAt == "" {
		t.Fatal("LastResetAt should be populated after Reset")
	}
}

func TestResetBudget_ZeroFallsBackToDefault(t *testing.T) {
	repo := t.TempDir()
	state, err := ResetBudget(repo, 0)
	if err != nil {
		t.Fatal(err)
	}
	if state.Remaining != DefaultWarnOnlyBudget {
		t.Fatalf("Remaining=%d want %d", state.Remaining, DefaultWarnOnlyBudget)
	}
}

func TestDecrementBudget_CountdownThenExhausted(t *testing.T) {
	repo := t.TempDir()
	if _, err := ResetBudget(repo, 2); err != nil {
		t.Fatal(err)
	}

	// First rescue: Remaining 2 → 1, not exhausted.
	state, exhausted, err := DecrementBudget(repo)
	if err != nil {
		t.Fatalf("DecrementBudget 1: %v", err)
	}
	if exhausted {
		t.Fatal("budget should not be exhausted after first decrement")
	}
	if state.Remaining != 1 {
		t.Fatalf("Remaining=%d want 1", state.Remaining)
	}
	if state.LastDecrementAt == "" {
		t.Fatal("LastDecrementAt should be set after decrement")
	}

	// Second rescue: Remaining 1 → 0, exhausted=true.
	state, exhausted, err = DecrementBudget(repo)
	if err != nil {
		t.Fatalf("DecrementBudget 2: %v", err)
	}
	if !exhausted {
		t.Fatal("budget should be exhausted after second decrement")
	}
	if state.Remaining != 0 {
		t.Fatalf("Remaining=%d want 0", state.Remaining)
	}

	// Third attempt: already at 0, returns exhausted=true, no state change.
	state, exhausted, err = DecrementBudget(repo)
	if err != nil {
		t.Fatalf("DecrementBudget 3: %v", err)
	}
	if !exhausted {
		t.Fatal("zero budget must stay exhausted")
	}
	if state.Remaining != 0 {
		t.Fatalf("Remaining=%d want 0", state.Remaining)
	}
}

func TestWarnOnlyBudgetPath_UnderDotAgents(t *testing.T) {
	got := WarnOnlyBudgetPath("/tmp/repo")
	want := filepath.Join("/tmp/repo", ".agents", "overnight", "warn-only-budget.json")
	if got != want {
		t.Fatalf("path=%q want %q", got, want)
	}
}

func TestWarnOnlyBudgetPath_UsesAgentPathOverrides(t *testing.T) {
	repo := t.TempDir()
	agentsDir := filepath.Join(t.TempDir(), "agents")
	t.Setenv("AO_AGENTS_DIR", agentsDir)

	got := WarnOnlyBudgetPath(repo)
	want := filepath.Join(agentsDir, "overnight", WarnOnlyBudgetFilename)
	if got != want {
		t.Fatalf("path=%q want %q", got, want)
	}
}
