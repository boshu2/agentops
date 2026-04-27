package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/types"
)

// ---------------------------------------------------------------------------
// badge.go: countSessions, printBadge, getEscapeStatus, makeProgressBar
// ---------------------------------------------------------------------------

func TestCountSessions(t *testing.T) {
	t.Run("no sessions dir", func(t *testing.T) {
		dir := t.TempDir()
		if got := countSessions(dir); got != 0 {
			t.Errorf("countSessions() = %d, want 0", got)
		}
	})
	t.Run("empty sessions dir", func(t *testing.T) {
		dir := t.TempDir()
		sessDir := filepath.Join(dir, storage.DefaultBaseDir, storage.SessionsDir)
		if err := os.MkdirAll(sessDir, 0755); err != nil {
			t.Fatal(err)
		}
		if got := countSessions(dir); got != 0 {
			t.Errorf("countSessions() = %d, want 0", got)
		}
	})
	t.Run("with session files", func(t *testing.T) {
		dir := t.TempDir()
		sessDir := filepath.Join(dir, storage.DefaultBaseDir, storage.SessionsDir)
		if err := os.MkdirAll(sessDir, 0755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"s1.jsonl", "s2.jsonl", "s3.jsonl"} {
			if err := os.WriteFile(filepath.Join(sessDir, name), []byte("{}"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		// Also create a non-jsonl file that shouldn't be counted
		_ = os.WriteFile(filepath.Join(sessDir, "notes.txt"), []byte("hi"), 0644)
		if got := countSessions(dir); got != 3 {
			t.Errorf("countSessions() = %d, want 3", got)
		}
	})
}

func TestGetEscapeStatus(t *testing.T) {
	tests := []struct {
		name      string
		sigmaRho  float64
		delta     float64
		wantText  string
		wantEmoji string
	}{
		{"escape velocity", 0.6, 30.0, "ESCAPE VELOCITY", "🚀"},
		{"approaching", 0.25, 30.0, "APPROACHING", "⚡"},
		{"building", 0.2, 30.0, "BUILDING", "📈"},
		{"starting", 0.05, 30.0, "STARTING", "🌱"},
		{"zero values", 0, 30.0, "STARTING", "🌱"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, emoji := getEscapeStatus(tt.sigmaRho, tt.delta)
			if text != tt.wantText {
				t.Errorf("text = %q, want %q", text, tt.wantText)
			}
			if emoji != tt.wantEmoji {
				t.Errorf("emoji = %q, want %q", emoji, tt.wantEmoji)
			}
		})
	}
}

func TestMakeProgressBar(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		width int
		want  string
	}{
		{"zero", 0, 5, "░░░░░"},
		{"full", 1.0, 5, "█████"},
		{"half", 0.5, 10, "█████░░░░░"},
		{"over one clamps", 1.5, 5, "█████"},
		{"negative clamps", -0.5, 5, "░░░░░"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeProgressBar(tt.value, tt.width)
			if got != tt.want {
				t.Errorf("makeProgressBar(%v, %d) = %q, want %q", tt.value, tt.width, got, tt.want)
			}
		})
	}
}

func TestPrintBadge(t *testing.T) {
	// Just ensure it doesn't panic with various inputs
	t.Run("nil metrics", func(t *testing.T) {
		printBadge(io.Discard, 0, nil)
	})
	t.Run("zero metrics", func(t *testing.T) {
		printBadge(io.Discard, 5, &FlywheelMetrics{
			Delta:      types.DefaultDelta * 100,
			TierCounts: map[string]int{"learning": 3, "pattern": 1},
		})
	})
	t.Run("escape velocity", func(t *testing.T) {
		printBadge(io.Discard, 10, &FlywheelMetrics{
			Sigma:               0.8,
			Rho:                 0.9,
			Delta:               30.0,
			SigmaRho:            0.72,
			TierCounts:          map[string]int{"learning": 10, "pattern": 5},
			CitationsThisPeriod: 3,
		})
	})
}

// ---------------------------------------------------------------------------
// store.go: collectArtifactFiles, walkIndexableFiles, appendToIndex,
//           searchIndex, computeIndexStats, indexFiles, printSearchResults,
//           printIndexStats
// ---------------------------------------------------------------------------

func TestWalkIndexableFiles(t *testing.T) {
	t.Run("empty dir", func(t *testing.T) {
		dir := t.TempDir()
		files, err := walkIndexableFiles(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 0 {
			t.Errorf("got %d files, want 0", len(files))
		}
	})
	t.Run("filters by extension", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"a.md", "b.jsonl", "c.txt", "d.go"} {
			_ = os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644)
		}
		files, err := walkIndexableFiles(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 2 {
			t.Errorf("got %d files, want 2 (a.md, b.jsonl)", len(files))
		}
	})
	t.Run("recursive", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "sub")
		_ = os.MkdirAll(sub, 0755)
		_ = os.WriteFile(filepath.Join(dir, "root.md"), []byte("r"), 0644)
		_ = os.WriteFile(filepath.Join(sub, "nested.md"), []byte("n"), 0644)
		files, err := walkIndexableFiles(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 2 {
			t.Errorf("got %d files, want 2", len(files))
		}
	})
}

func TestCollectArtifactFiles(t *testing.T) {
	t.Run("no agents dir", func(t *testing.T) {
		dir := t.TempDir()
		files := collectArtifactFiles(dir)
		if len(files) != 0 {
			t.Errorf("got %d files, want 0", len(files))
		}
	})
	t.Run("with learnings and patterns", func(t *testing.T) {
		dir := t.TempDir()
		for _, sub := range []string{"learnings", "patterns"} {
			subDir := filepath.Join(dir, ".agents", sub)
			_ = os.MkdirAll(subDir, 0755)
			_ = os.WriteFile(filepath.Join(subDir, "item.md"), []byte("content"), 0644)
		}
		files := collectArtifactFiles(dir)
		if len(files) != 2 {
			t.Errorf("got %d files, want 2", len(files))
		}
	})
}

func TestAppendToIndex(t *testing.T) {
	dir := t.TempDir()
	entry := &IndexEntry{
		Path:    "/test/learning.md",
		ID:      "test-1",
		Type:    "learning",
		Title:   "Test Learning",
		Content: "some content",
	}
	if err := appendToIndex(dir, entry); err != nil {
		t.Fatal(err)
	}
	// Verify the file was created
	indexPath := filepath.Join(dir, IndexDir, IndexFileName)
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	var decoded IndexEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != "test-1" {
		t.Errorf("ID = %q, want %q", decoded.ID, "test-1")
	}

	// Append another
	entry2 := &IndexEntry{ID: "test-2", Type: "pattern", Title: "Pattern"}
	if err := appendToIndex(dir, entry2); err != nil {
		t.Fatal(err)
	}
}

func TestSearchIndex(t *testing.T) {
	dir := t.TempDir()
	// Create index with entries
	entries := []IndexEntry{
		{ID: "1", Type: "learning", Title: "Go error handling", Content: "always wrap errors with context", Keywords: []string{"go", "error"}},
		{ID: "2", Type: "pattern", Title: "Retry pattern", Content: "exponential backoff for transient failures", Keywords: []string{"retry", "resilience"}},
		{ID: "3", Type: "learning", Title: "Test coverage", Content: "focus on pure functions for easy coverage", Keywords: []string{"test", "coverage"}},
	}
	for i := range entries {
		if err := appendToIndex(dir, &entries[i]); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("matching query", func(t *testing.T) {
		results, err := searchIndex(dir, "error handling", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) == 0 {
			t.Error("expected results for 'error handling'")
		}
	})
	t.Run("no match", func(t *testing.T) {
		results, err := searchIndex(dir, "zzzznonexistent", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 0 {
			t.Errorf("got %d results, want 0", len(results))
		}
	})
	t.Run("with limit", func(t *testing.T) {
		results, err := searchIndex(dir, "learning pattern", 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) > 1 {
			t.Errorf("got %d results, want <= 1", len(results))
		}
	})
	t.Run("index not found", func(t *testing.T) {
		emptyDir := t.TempDir()
		_, err := searchIndex(emptyDir, "anything", 10)
		if err == nil {
			t.Error("expected error for missing index")
		}
	})
}

func TestComputeIndexStats(t *testing.T) {
	t.Run("no index", func(t *testing.T) {
		dir := t.TempDir()
		stats, err := computeIndexStats(dir)
		if err != nil {
			t.Fatal(err)
		}
		if stats.TotalEntries != 0 {
			t.Errorf("TotalEntries = %d, want 0", stats.TotalEntries)
		}
	})
	t.Run("with entries", func(t *testing.T) {
		dir := t.TempDir()
		now := time.Now()
		entries := []IndexEntry{
			{ID: "1", Type: "learning", Utility: 0.8, IndexedAt: now.Add(-24 * time.Hour), ModifiedAt: now.Add(-48 * time.Hour)},
			{ID: "2", Type: "pattern", Utility: 0.6, IndexedAt: now, ModifiedAt: now.Add(-1 * time.Hour)},
			{ID: "3", Type: "learning", Utility: 0.0, IndexedAt: now.Add(-12 * time.Hour)},
		}
		for i := range entries {
			if err := appendToIndex(dir, &entries[i]); err != nil {
				t.Fatal(err)
			}
		}

		stats, err := computeIndexStats(dir)
		if err != nil {
			t.Fatal(err)
		}
		if stats.TotalEntries != 3 {
			t.Errorf("TotalEntries = %d, want 3", stats.TotalEntries)
		}
		if stats.ByType["learning"] != 2 {
			t.Errorf("learning count = %d, want 2", stats.ByType["learning"])
		}
		if stats.ByType["pattern"] != 1 {
			t.Errorf("pattern count = %d, want 1", stats.ByType["pattern"])
		}
		if stats.MeanUtility == 0 {
			t.Error("expected nonzero MeanUtility")
		}
	})
}

func TestPrintSearchResults(t *testing.T) {
	t.Run("empty results", func(t *testing.T) {
		printSearchResults("test query", nil)
	})
	t.Run("with results", func(t *testing.T) {
		results := []SearchResult{
			{Entry: IndexEntry{Title: "First", Type: "learning", Path: "/a.md"}, Score: 5.0, Snippet: "...context..."},
			{Entry: IndexEntry{Title: "Second", Type: "pattern", Path: "/b.md"}, Score: 3.0},
		}
		printSearchResults("error", results)
	})
}

func TestPrintIndexStats(t *testing.T) {
	t.Run("empty stats", func(t *testing.T) {
		stats := &IndexStats{
			ByType:    map[string]int{},
			IndexPath: "/fake/path",
		}
		printIndexStats(stats)
	})
	t.Run("populated stats", func(t *testing.T) {
		now := time.Now()
		stats := &IndexStats{
			TotalEntries: 42,
			ByType:       map[string]int{"learning": 30, "pattern": 12},
			MeanUtility:  0.75,
			OldestEntry:  now.Add(-720 * time.Hour),
			NewestEntry:  now,
			IndexPath:    "/test/index.jsonl",
		}
		printIndexStats(stats)
	})
}

func TestIndexFiles(t *testing.T) {
	dir := t.TempDir()
	// Create a valid markdown file
	artifactDir := filepath.Join(dir, ".agents", "learnings")
	_ = os.MkdirAll(artifactDir, 0755)
	mdPath := filepath.Join(artifactDir, "test-learning.md")
	_ = os.WriteFile(mdPath, []byte("# Test Learning\n\nSome content here.\n"), 0644)

	indexed := indexFiles(dir, []string{mdPath}, false)
	if indexed != 1 {
		t.Errorf("indexFiles() = %d, want 1", indexed)
	}
}

// ---------------------------------------------------------------------------
// batch_promote.go: recordPromoteSkip
// ---------------------------------------------------------------------------

func TestRecordPromoteSkip(t *testing.T) {
	result := &batchPromoteResult{}
	recordPromoteSkip(result, "cand-1", "too young")
	recordPromoteSkip(result, "cand-2", "low utility")

	if result.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", result.Skipped)
	}
	if len(result.Reasons) != 2 {
		t.Errorf("Reasons len = %d, want 2", len(result.Reasons))
	}
	if result.Reasons[0].CandidateID != "cand-1" {
		t.Errorf("first reason candidate = %q, want %q", result.Reasons[0].CandidateID, "cand-1")
	}
}

// ---------------------------------------------------------------------------
// rpi_phased_processing.go: classifyByVerdict, classifyByPhase
// ---------------------------------------------------------------------------

func TestClassifyByVerdict(t *testing.T) {
	tests := []struct {
		verdict string
		want    types.MemRLFailureClass
	}{
		{string(failReasonTimeout), types.MemRLFailureClassPhaseTimeout},
		{string(failReasonStall), types.MemRLFailureClassPhaseStall},
		{string(failReasonExit), types.MemRLFailureClassPhaseExitError},
		{"CUSTOM", types.MemRLFailureClass("custom")},
		{"", types.MemRLFailureClass("")},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("verdict_%s", tt.verdict), func(t *testing.T) {
			got := classifyByVerdict(tt.verdict)
			if got != tt.want {
				t.Errorf("classifyByVerdict(%q) = %q, want %q", tt.verdict, got, tt.want)
			}
		})
	}
}

func TestClassifyByPhase(t *testing.T) {
	tests := []struct {
		name     string
		phaseNum int
		verdict  string
		want     types.MemRLFailureClass
	}{
		{"phase1 FAIL", 1, "FAIL", types.MemRLFailureClassPreMortemFail},
		{"phase1 other", 1, "PASS", ""},
		{"phase2 BLOCKED", 2, "BLOCKED", types.MemRLFailureClassCrankBlocked},
		{"phase2 PARTIAL", 2, "PARTIAL", types.MemRLFailureClassCrankPartial},
		{"phase2 other", 2, "FAIL", ""},
		{"phase3 FAIL", 3, "FAIL", types.MemRLFailureClassVibeFail},
		{"phase3 other", 3, "PASS", ""},
		{"phase4 any", 4, "FAIL", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyByPhase(tt.phaseNum, tt.verdict)
			if got != tt.want {
				t.Errorf("classifyByPhase(%d, %q) = %q, want %q", tt.phaseNum, tt.verdict, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rpi_phased_stream.go: classifyStreamResult
// ---------------------------------------------------------------------------

func TestClassifyStreamResult(t *testing.T) {
	bg := context.Background()

	t.Run("nil errors and events", func(t *testing.T) {
		err := classifyStreamResult(bg, bg, "claude", 1, 10*time.Minute, nil, nil, 5)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})
	t.Run("zero events", func(t *testing.T) {
		err := classifyStreamResult(bg, bg, "claude", 1, 10*time.Minute, nil, nil, 0)
		if err == nil {
			t.Error("expected error for zero events")
		}
	})
	t.Run("timeout", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(bg, time.Now().Add(-1*time.Second))
		defer cancel()
		err := classifyStreamResult(ctx, bg, "claude", 1, 5*time.Minute, nil, nil, 0)
		if err == nil {
			t.Error("expected timeout error")
		}
	})
	t.Run("wait error", func(t *testing.T) {
		err := classifyStreamResult(bg, bg, "claude", 1, 10*time.Minute, errors.New("process failed"), nil, 5)
		if err == nil {
			t.Error("expected error from waitErr")
		}
	})
	t.Run("exit error", func(t *testing.T) {
		// Create an exec.ExitError by running a command that fails
		cmd := exec.Command("false")
		exitErr := cmd.Run()
		err := classifyStreamResult(bg, bg, "claude", 1, 10*time.Minute, exitErr, nil, 5)
		if err == nil {
			t.Error("expected error from exit error")
		}
	})
	t.Run("parse error", func(t *testing.T) {
		err := classifyStreamResult(bg, bg, "claude", 1, 10*time.Minute, nil, errors.New("parse failed"), 5)
		if err == nil {
			t.Error("expected error from parseErr")
		}
	})
}

// parseTimestamp tests are in context_test.go. Duplicate removed.

func TestExtractTextContent(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"empty", "", ""},
		{"plain string", `"hello world"`, "hello world"},
		{"array with text", `[{"type":"text","text":"content here"}]`, "content here"},
		{"array no text", `[{"type":"image"}]`, ""},
		{"invalid json", `not json`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextContent(json.RawMessage(tt.raw))
			if got != tt.want {
				t.Errorf("extractTextContent() = %q, want %q", got, tt.want)
			}
		})
	}
}
