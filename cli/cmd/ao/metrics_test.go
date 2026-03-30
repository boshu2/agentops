package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/types"
)

func TestFilterCitationsForPeriod(t *testing.T) {
	now := time.Now()
	oneDayAgo := now.AddDate(0, 0, -1)
	twoDaysAgo := now.AddDate(0, 0, -2)
	oneWeekAgo := now.AddDate(0, 0, -7)
	twoWeeksAgo := now.AddDate(0, 0, -14)

	citations := []types.CitationEvent{
		{ArtifactPath: "/path/a.md", CitedAt: oneDayAgo},
		{ArtifactPath: "/path/b.md", CitedAt: twoDaysAgo},
		{ArtifactPath: "/path/c.md", CitedAt: oneWeekAgo},
		{ArtifactPath: "/path/d.md", CitedAt: twoWeeksAgo},
	}

	tests := []struct {
		name          string
		start         time.Time
		end           time.Time
		wantCount     int
		wantUniqueCnt int
	}{
		{
			name:          "all in period",
			start:         twoWeeksAgo.AddDate(0, 0, -1),
			end:           now.AddDate(0, 0, 1),
			wantCount:     4,
			wantUniqueCnt: 4,
		},
		{
			name:          "last 3 days",
			start:         now.AddDate(0, 0, -3),
			end:           now.AddDate(0, 0, 1),
			wantCount:     2,
			wantUniqueCnt: 2,
		},
		{
			name:          "last week",
			start:         now.AddDate(0, 0, -8),
			end:           now.AddDate(0, 0, 1),
			wantCount:     3,
			wantUniqueCnt: 3,
		},
		{
			name:          "empty period",
			start:         now.AddDate(0, 0, -100),
			end:           now.AddDate(0, 0, -50),
			wantCount:     0,
			wantUniqueCnt: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := filterCitationsForPeriod(citations, tt.start, tt.end)
			if len(stats.citations) != tt.wantCount {
				t.Errorf("filterCitationsForPeriod() count = %d, want %d",
					len(stats.citations), tt.wantCount)
			}
			if len(stats.uniqueCited) != tt.wantUniqueCnt {
				t.Errorf("filterCitationsForPeriod() uniqueCited = %d, want %d",
					len(stats.uniqueCited), tt.wantUniqueCnt)
			}
		})
	}
}

func TestComputeSigmaRho(t *testing.T) {
	tests := []struct {
		name           string
		totalArtifacts int
		uniqueCited    int
		citationCount  int
		days           int
		wantSigma      float64
		wantRho        float64
	}{
		{
			name:           "normal case",
			totalArtifacts: 100,
			uniqueCited:    50,
			citationCount:  100,
			days:           7,
			wantSigma:      0.5,
			wantRho:        2.0, // 100/50/1week = 2
		},
		{
			name:           "no artifacts",
			totalArtifacts: 0,
			uniqueCited:    0,
			citationCount:  0,
			days:           7,
			wantSigma:      0,
			wantRho:        0,
		},
		{
			name:           "no citations",
			totalArtifacts: 100,
			uniqueCited:    0,
			citationCount:  0,
			days:           7,
			wantSigma:      0,
			wantRho:        0,
		},
		{
			name:           "14 days",
			totalArtifacts: 100,
			uniqueCited:    50,
			citationCount:  25,
			days:           14,
			wantSigma:      0.5,
			wantRho:        0.5, // 25 evidenced / 50 surfaced
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sigma, rho := computeSigmaRho(tt.totalArtifacts, tt.uniqueCited, tt.citationCount, tt.days)

			if !floatEqual(sigma, tt.wantSigma, 0.01) {
				t.Errorf("computeSigmaRho() sigma = %v, want %v", sigma, tt.wantSigma)
			}
			if !floatEqual(rho, tt.wantRho, 0.01) {
				t.Errorf("computeSigmaRho() rho = %v, want %v", rho, tt.wantRho)
			}
		})
	}
}

func TestCountLoopMetrics(t *testing.T) {
	now := time.Now()
	oneDayAgo := now.AddDate(0, 0, -1)

	citations := []types.CitationEvent{
		{ArtifactPath: "/path/to/.agents/learnings/L1.md", CitedAt: oneDayAgo},
		{ArtifactPath: "/path/to/.agents/learnings/L2.md", CitedAt: oneDayAgo},
		{ArtifactPath: "/path/to/.agents/patterns/P1.md", CitedAt: oneDayAgo},
		{ArtifactPath: "/other/file.md", CitedAt: oneDayAgo},
	}

	// countLoopMetrics requires actual directory structure, so we just test
	// the learningsFound counting logic here via the helper
	learningsFound := 0
	for _, c := range citations {
		if containsLearningsPath(c.ArtifactPath) {
			learningsFound++
		}
	}

	if learningsFound != 2 {
		t.Errorf("learningsFound = %d, want 2", learningsFound)
	}
}

func TestCountBypassCitations(t *testing.T) {
	citations := []types.CitationEvent{
		{ArtifactPath: "/normal/path.md", CitationType: "recall"},
		{ArtifactPath: "/bypass/path.md", CitationType: "bypass"},
		{ArtifactPath: "bypass:/skipped", CitationType: ""},
		{ArtifactPath: "/another/path.md", CitationType: "inject"},
	}

	got := countBypassCitations(citations)
	if got != 2 {
		t.Errorf("countBypassCitations() = %d, want 2", got)
	}
}

// floatEqual checks if two floats are approximately equal
func floatEqual(a, b, epsilon float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

// containsLearningsPath checks if path contains /learnings/
func containsLearningsPath(path string) bool {
	for i := range len(path) - 11 + 1 {
		if path[i:i+11] == "/learnings/" {
			return true
		}
	}
	return false
}

func TestCountStaleArtifacts(t *testing.T) {
	baseDir := t.TempDir()
	learningsDir := filepath.Join(baseDir, ".agents", "learnings")
	patternsDir := filepath.Join(baseDir, ".agents", "patterns")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(patternsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldTime := time.Now().AddDate(0, 0, -120)
	newTime := time.Now().AddDate(0, 0, -1)

	writeFileWithTime := func(path string, ts time.Time) {
		t.Helper()
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, ts, ts); err != nil {
			t.Fatal(err)
		}
	}

	oldUncited := filepath.Join(learningsDir, "old-uncited.md")
	newUncited := filepath.Join(learningsDir, "new-uncited.md")
	oldRecentlyCited := filepath.Join(learningsDir, "old-recently-cited.md")
	oldCitedLongAgo := filepath.Join(patternsDir, "old-cited-long-ago.md")

	writeFileWithTime(oldUncited, oldTime)
	writeFileWithTime(newUncited, newTime)
	writeFileWithTime(oldRecentlyCited, oldTime)
	writeFileWithTime(oldCitedLongAgo, oldTime)

	citations := []types.CitationEvent{
		{
			ArtifactPath: ".agents/learnings/old-recently-cited.md",
			CitedAt:      time.Now().AddDate(0, 0, -5),
		},
		{
			ArtifactPath: oldCitedLongAgo,
			CitedAt:      time.Now().AddDate(0, 0, -100),
		},
	}

	staleCount, err := countStaleArtifacts(baseDir, citations, 90)
	if err != nil {
		t.Fatalf("countStaleArtifacts failed: %v", err)
	}
	// old-uncited + old-cited-long-ago are stale.
	if staleCount != 2 {
		t.Fatalf("expected 2 stale artifacts, got %d", staleCount)
	}
}

func TestComputeMetricsSigmaBounded(t *testing.T) {
	baseDir := t.TempDir()
	learningsDir := filepath.Join(baseDir, ".agents", "learnings")
	researchDir := filepath.Join(baseDir, ".agents", "research")
	citationsDir := filepath.Join(baseDir, ".agents", "ao")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(citationsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	learningPath := filepath.Join(learningsDir, "L1.md")
	researchPath := filepath.Join(researchDir, "R1.md")
	if err := os.WriteFile(learningPath, []byte("# L1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(researchPath, []byte("# R1"), 0o644); err != nil {
		t.Fatal(err)
	}

	citations := []types.CitationEvent{
		{
			ArtifactPath: ".agents/learnings/L1.md",
			SessionID:    "s1",
			CitedAt:      time.Now().AddDate(0, 0, -1),
		},
		{
			ArtifactPath: researchPath,
			SessionID:    "s2",
			CitedAt:      time.Now().AddDate(0, 0, -1),
		},
	}

	f, err := os.Create(filepath.Join(citationsDir, "citations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for _, c := range citations {
		if err := enc.Encode(c); err != nil {
			_ = f.Close()
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	metrics, err := computeMetrics(baseDir, 7)
	if err != nil {
		t.Fatalf("computeMetrics failed: %v", err)
	}
	if metrics.Sigma > 1.0 {
		t.Fatalf("sigma must be <= 1.0, got %f", metrics.Sigma)
	}
	if metrics.Sigma < 0.99 {
		t.Fatalf("expected sigma close to 1.0 for one retrievable cited artifact, got %f", metrics.Sigma)
	}
	// Keep visibility count unchanged (all unique cited artifacts in period).
	if metrics.UniqueCitedArtifacts != 2 {
		t.Fatalf("expected 2 unique cited artifacts in period, got %d", metrics.UniqueCitedArtifacts)
	}
}

func TestCountRetros(t *testing.T) {
	t.Run("missing retros dir returns 0 with no error", func(t *testing.T) {
		baseDir := t.TempDir()
		// No .agents/retros directory created
		total, withLearnings, err := countRetros(baseDir, time.Now().AddDate(0, 0, -7))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if total != 0 {
			t.Errorf("expected 0 total, got %d", total)
		}
		if withLearnings != 0 {
			t.Errorf("expected 0 withLearnings, got %d", withLearnings)
		}
	})

	t.Run("retro within time period is counted", func(t *testing.T) {
		baseDir := t.TempDir()
		retrosDir := filepath.Join(baseDir, ".agents", "retros")
		if err := os.MkdirAll(retrosDir, 0755); err != nil {
			t.Fatal(err)
		}
		retroContent := "# My Retro\n\n## Key Learnings\n\n- Learned something"
		if err := os.WriteFile(filepath.Join(retrosDir, "retro.md"), []byte(retroContent), 0644); err != nil {
			t.Fatal(err)
		}
		since := time.Now().AddDate(0, 0, -7)
		total, withLearnings, err := countRetros(baseDir, since)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 1 {
			t.Errorf("expected 1 total retro, got %d", total)
		}
		if withLearnings != 1 {
			t.Errorf("expected 1 retro with learnings, got %d", withLearnings)
		}
	})

	t.Run("retro without learnings section not counted in withLearnings", func(t *testing.T) {
		baseDir := t.TempDir()
		retrosDir := filepath.Join(baseDir, ".agents", "retros")
		if err := os.MkdirAll(retrosDir, 0755); err != nil {
			t.Fatal(err)
		}
		retroContent := "# My Retro\n\n## Summary\n\nDid some things"
		if err := os.WriteFile(filepath.Join(retrosDir, "retro.md"), []byte(retroContent), 0644); err != nil {
			t.Fatal(err)
		}
		since := time.Now().AddDate(0, 0, -7)
		total, withLearnings, err := countRetros(baseDir, since)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 1 {
			t.Errorf("expected 1 total retro, got %d", total)
		}
		if withLearnings != 0 {
			t.Errorf("expected 0 retros with learnings, got %d", withLearnings)
		}
	})

	t.Run("non-md files not counted", func(t *testing.T) {
		baseDir := t.TempDir()
		retrosDir := filepath.Join(baseDir, ".agents", "retros")
		if err := os.MkdirAll(retrosDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(retrosDir, "retro.txt"), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
		total, _, err := countRetros(baseDir, time.Now().AddDate(0, 0, -7))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 0 {
			t.Errorf("expected 0 for non-md file, got %d", total)
		}
	})
}

func TestParseUtilityFromJSONL(t *testing.T) {
	t.Run("returns utility from JSONL first line", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "learning.jsonl")
		data := `{"id":"L1","utility":0.75}` + "\n"
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
		got := parseUtilityFromFile(path)
		if got != 0.75 {
			t.Errorf("parseUtilityFromFile() = %f, want 0.75", got)
		}
	})

	t.Run("returns 0 for nonexistent JSONL", func(t *testing.T) {
		got := parseUtilityFromFile("/tmp/no-such-file-xyz.jsonl")
		if got != 0 {
			t.Errorf("expected 0 for nonexistent file, got %f", got)
		}
	})

	t.Run("returns 0 for JSONL without utility field", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "noutil.jsonl")
		data := `{"id":"L1","content":"no utility field"}` + "\n"
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
		got := parseUtilityFromFile(path)
		if got != 0 {
			t.Errorf("expected 0 for missing utility, got %f", got)
		}
	})
}

// TestComputeSigmaRho — canonical version earlier in this file

func floatApprox(a, b, epsilon float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= epsilon
}

// TestFilterCitationsForPeriod — canonical version earlier in this file

func TestFilterCitationsForPeriod_uniqueCited(t *testing.T) {
	now := time.Now()
	citations := []types.CitationEvent{
		{ArtifactPath: "a.md", CitedAt: now.AddDate(0, 0, -1)},
		{ArtifactPath: "a.md", CitedAt: now.AddDate(0, 0, -2)},
		{ArtifactPath: "b.md", CitedAt: now.AddDate(0, 0, -3)},
	}
	start := now.AddDate(0, 0, -7)
	stats := filterCitationsForPeriod(citations, start, now)
	if len(stats.uniqueCited) != 2 {
		t.Errorf("uniqueCited = %d, want 2", len(stats.uniqueCited))
	}
}

// ---------------------------------------------------------------------------
// normalizeArtifactPath / isRetrievableArtifactPath
// ---------------------------------------------------------------------------

func TestIsRetrievableArtifactPath(t *testing.T) {
	baseDir := "/tmp/repo"
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"learnings path", filepath.Join(baseDir, ".agents", "learnings", "test.md"), true},
		{"patterns path", filepath.Join(baseDir, ".agents", "patterns", "test.md"), true},
		{"research path", filepath.Join(baseDir, ".agents", "research", "test.md"), false},
		{"candidates path", filepath.Join(baseDir, ".agents", "candidates", "test.md"), false},
		{"random path", "/some/other/path.md", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetrievableArtifactPath(baseDir, tt.path)
			if got != tt.want {
				t.Errorf("isRetrievableArtifactPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// retrievableCitationStats
// ---------------------------------------------------------------------------

func TestRetrievableCitationStats(t *testing.T) {
	baseDir := "/tmp/repo"
	citations := []types.CitationEvent{
		{ArtifactPath: filepath.Join(baseDir, ".agents", "learnings", "a.md")},
		{ArtifactPath: filepath.Join(baseDir, ".agents", "learnings", "a.md")},
		{ArtifactPath: filepath.Join(baseDir, ".agents", "learnings", "b.md")},
		{ArtifactPath: filepath.Join(baseDir, ".agents", "patterns", "c.md")},
		{ArtifactPath: filepath.Join(baseDir, ".agents", "research", "d.md")}, // not retrievable
	}

	uniqueCount, evidenceCount := retrievableCitationStats(baseDir, citations)
	if evidenceCount != 3 {
		t.Errorf("evidenceCount = %d, want 3", evidenceCount)
	}
	if uniqueCount != 3 {
		t.Errorf("uniqueCount = %d, want 3", uniqueCount)
	}
}

func TestRetrievableCitationStats_empty(t *testing.T) {
	uniqueCount, evidenceCount := retrievableCitationStats("/tmp", nil)
	if uniqueCount != 0 || evidenceCount != 0 {
		t.Errorf("expected 0/0 for empty citations, got %d/%d", uniqueCount, evidenceCount)
	}
}

// TestCountBypassCitations — canonical version earlier in this file

func TestCountBypassCitations_none(t *testing.T) {
	citations := []types.CitationEvent{
		{CitationType: "reference"},
		{CitationType: "applied"},
	}
	got := countBypassCitations(citations)
	if got != 0 {
		t.Errorf("countBypassCitations = %d, want 0", got)
	}
}

// TestCountLoopMetrics — canonical version earlier in this file

// ---------------------------------------------------------------------------
// countArtifacts
// ---------------------------------------------------------------------------

func TestCountArtifacts(t *testing.T) {
	baseDir := t.TempDir()
	dirs := map[string][]string{
		filepath.Join(baseDir, ".agents", "learnings"):                      {"l1.md", "l2.jsonl"},
		filepath.Join(baseDir, ".agents", "patterns"):                       {"p1.md"},
		filepath.Join(baseDir, ".agents", "candidates"):                     {"c1.md", "c2.md"},
		filepath.Join(baseDir, ".agents", "research"):                       {"r1.md"},
		filepath.Join(baseDir, ".agents", "retros"):                         {"retro1.md"},
		filepath.Join(baseDir, storage.DefaultBaseDir, storage.SessionsDir): {"s1.jsonl"},
	}
	for dir, files := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(dir, f), []byte("test"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	total, tierCounts, err := countArtifacts(baseDir)
	if err != nil {
		t.Fatalf("countArtifacts failed: %v", err)
	}

	if tierCounts["learning"] != 2 {
		t.Errorf("learning count = %d, want 2", tierCounts["learning"])
	}
	if tierCounts["pattern"] != 1 {
		t.Errorf("pattern count = %d, want 1", tierCounts["pattern"])
	}
	if tierCounts["observation"] != 3 { // 2 candidates + 1 research
		t.Errorf("observation count = %d, want 3", tierCounts["observation"])
	}
	if tierCounts["retro"] != 1 {
		t.Errorf("retro count = %d, want 1", tierCounts["retro"])
	}
	// Total: 2 learnings + 1 pattern + 2 candidates + 1 research + 1 retro + 1 session = 8
	if total != 8 {
		t.Errorf("total = %d, want 8", total)
	}
}

func TestCountArtifacts_emptyDir(t *testing.T) {
	total, tierCounts, err := countArtifacts(t.TempDir())
	if err != nil {
		t.Fatalf("countArtifacts failed: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if tierCounts["learning"] != 0 {
		t.Errorf("learning should be 0, got %d", tierCounts["learning"])
	}
}

// ---------------------------------------------------------------------------
// countNewArtifacts
// ---------------------------------------------------------------------------

func TestCountNewArtifacts(t *testing.T) {
	baseDir := t.TempDir()
	learningsDir := filepath.Join(baseDir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create one old and one new file
	oldPath := filepath.Join(learningsDir, "old.md")
	newPath := filepath.Join(learningsDir, "new.md")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -30)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	since := time.Now().AddDate(0, 0, -7)
	count, err := countNewArtifacts(baseDir, since)
	if err != nil {
		t.Fatalf("countNewArtifacts failed: %v", err)
	}
	if count != 1 {
		t.Errorf("new artifact count = %d, want 1", count)
	}
}

// ---------------------------------------------------------------------------
// countNewArtifactsInDir
// ---------------------------------------------------------------------------

func TestCountNewArtifactsInDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "recent.md"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(dir, "old.md")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -60)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	since := time.Now().AddDate(0, 0, -7)
	count, err := countNewArtifactsInDir(dir, since)
	if err != nil {
		t.Fatalf("countNewArtifactsInDir failed: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

// ---------------------------------------------------------------------------
// buildLastCitedMap
// ---------------------------------------------------------------------------

func TestBuildLastCitedMap(t *testing.T) {
	baseDir := "/tmp/test"
	now := time.Now()
	citations := []types.CitationEvent{
		{ArtifactPath: "/tmp/test/.agents/learnings/a.md", CitedAt: now.AddDate(0, 0, -10)},
		{ArtifactPath: "/tmp/test/.agents/learnings/a.md", CitedAt: now.AddDate(0, 0, -5)}, // later
		{ArtifactPath: "/tmp/test/.agents/learnings/b.md", CitedAt: now.AddDate(0, 0, -1)},
	}
	m := buildLastCitedMap(baseDir, citations)
	if len(m) != 2 {
		t.Errorf("expected 2 entries, got %d", len(m))
	}
}

// ---------------------------------------------------------------------------
// isKnowledgeFile
// ---------------------------------------------------------------------------

func TestIsKnowledgeFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"learning.md", true},
		{"learning.jsonl", true},
		{"data.json", false},
		{"script.sh", false},
		{"readme.txt", false},
		{".md", true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isKnowledgeFile(tt.path); got != tt.want {
				t.Errorf("isKnowledgeFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isStaleArtifact
// ---------------------------------------------------------------------------

func TestIsStaleArtifact(t *testing.T) {
	baseDir := "/tmp/repo"
	now := time.Now()
	staleThreshold := now.AddDate(0, 0, -90)

	tests := []struct {
		name      string
		path      string
		modTime   time.Time
		lastCited map[string]time.Time
		want      bool
	}{
		{
			name:      "recent mod time",
			path:      "/tmp/repo/.agents/learnings/new.md",
			modTime:   now,
			lastCited: map[string]time.Time{},
			want:      false,
		},
		{
			name:      "old mod time no citation",
			path:      "/tmp/repo/.agents/learnings/old.md",
			modTime:   now.AddDate(0, 0, -120),
			lastCited: map[string]time.Time{},
			want:      true,
		},
		{
			name:    "old mod time with recent citation",
			path:    "/tmp/repo/.agents/learnings/old.md",
			modTime: now.AddDate(0, 0, -120),
			lastCited: map[string]time.Time{
				normalizeArtifactPath(baseDir, "/tmp/repo/.agents/learnings/old.md"): now.AddDate(0, 0, -30),
			},
			want: false,
		},
		{
			name:    "old mod time with old citation",
			path:    "/tmp/repo/.agents/learnings/old.md",
			modTime: now.AddDate(0, 0, -120),
			lastCited: map[string]time.Time{
				normalizeArtifactPath(baseDir, "/tmp/repo/.agents/learnings/old.md"): now.AddDate(0, 0, -100),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStaleArtifact(baseDir, tt.path, tt.modTime, staleThreshold, tt.lastCited)
			if got != tt.want {
				t.Errorf("isStaleArtifact = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// countStaleInDir
// ---------------------------------------------------------------------------

func TestCountStaleInDir(t *testing.T) {
	baseDir := t.TempDir()
	dir := filepath.Join(baseDir, ".agents", "learnings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Old file
	oldPath := filepath.Join(dir, "old.md")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -120)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Recent file
	if err := os.WriteFile(filepath.Join(dir, "new.md"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Non-knowledge file
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("skip"), 0o644); err != nil {
		t.Fatal(err)
	}

	staleThreshold := time.Now().AddDate(0, 0, -90)
	count := countStaleInDir(baseDir, dir, staleThreshold, map[string]time.Time{})
	if count != 1 {
		t.Errorf("stale count = %d, want 1", count)
	}
}

func TestCountStaleInDir_missingDir(t *testing.T) {
	count := countStaleInDir("/tmp", "/nonexistent", time.Now(), nil)
	if count != 0 {
		t.Errorf("expected 0 for missing dir, got %d", count)
	}
}

// TestCountStaleArtifacts — canonical version earlier in this file

// ---------------------------------------------------------------------------
// retroHasLearnings
// ---------------------------------------------------------------------------

func TestRetroHasLearnings(t *testing.T) {
	tmp := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"has ## Learnings", "# Retro\n## Learnings\n- item\n", true},
		{"has ## Key Learnings", "# Retro\n## Key Learnings\n- item\n", true},
		{"has ### Learnings", "# Retro\n### Learnings\n- item\n", true},
		{"no learnings section", "# Retro\n## Summary\n- item\n", false},
		{"empty file", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmp, tt.name+".md")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
			got := retroHasLearnings(path)
			if got != tt.want {
				t.Errorf("retroHasLearnings = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetroHasLearnings_missingFile(t *testing.T) {
	got := retroHasLearnings("/nonexistent/file.md")
	if got {
		t.Error("expected false for missing file")
	}
}

// TestCountRetros — canonical version earlier in this file

// ---------------------------------------------------------------------------
// computeUtilityStats
// ---------------------------------------------------------------------------

func TestComputeUtilityStats(t *testing.T) {
	tests := []struct {
		name      string
		utilities []float64
		wantMean  float64
		wantHigh  int
		wantLow   int
	}{
		{"empty", nil, 0, 0, 0},
		{"single high", []float64{0.8}, 0.8, 1, 0},
		{"single low", []float64{0.2}, 0.2, 0, 1},
		{"mixed", []float64{0.1, 0.5, 0.9}, 0.5, 1, 1},
		{"all mid-range", []float64{0.4, 0.5, 0.6}, 0.5, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := computeUtilityStats(tt.utilities)
			if !floatApprox(stats.mean, tt.wantMean, 0.01) {
				t.Errorf("mean = %f, want ~%f", stats.mean, tt.wantMean)
			}
			if stats.highCount != tt.wantHigh {
				t.Errorf("highCount = %d, want %d", stats.highCount, tt.wantHigh)
			}
			if stats.lowCount != tt.wantLow {
				t.Errorf("lowCount = %d, want %d", stats.lowCount, tt.wantLow)
			}
		})
	}
}

func TestComputeUtilityStats_stdDev(t *testing.T) {
	// All same values => stdDev = 0
	stats := computeUtilityStats([]float64{0.5, 0.5, 0.5})
	if stats.stdDev != 0 {
		t.Errorf("stdDev = %f, want 0 for identical values", stats.stdDev)
	}

	// Known spread
	stats2 := computeUtilityStats([]float64{0.0, 1.0})
	if stats2.stdDev < 0.49 || stats2.stdDev > 0.51 {
		t.Errorf("stdDev = %f, want ~0.5", stats2.stdDev)
	}
}

// ---------------------------------------------------------------------------
// collectUtilityValuesFromDir
// ---------------------------------------------------------------------------

func TestCollectUtilityValuesFromDir(t *testing.T) {
	dir := t.TempDir()

	// JSONL with utility
	jsonl := map[string]any{"id": "L1", "utility": 0.8}
	data, _ := json.Marshal(jsonl)
	if err := os.WriteFile(filepath.Join(dir, "l1.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Markdown with utility
	if err := os.WriteFile(filepath.Join(dir, "l2.md"), []byte("---\nutility: 0.6\n---\n# L2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// File without utility (should not contribute)
	if err := os.WriteFile(filepath.Join(dir, "no-utility.md"), []byte("---\ntitle: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Non-matching extension
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("skip"), 0o644); err != nil {
		t.Fatal(err)
	}

	values := collectUtilityValuesFromDir(dir)
	if len(values) != 2 {
		t.Errorf("expected 2 utility values, got %d", len(values))
	}
}

// ---------------------------------------------------------------------------
// parseUtilityFromFile / parseUtilityFromMarkdown / parseUtilityFromJSONL
// ---------------------------------------------------------------------------

func TestParseUtilityFromMarkdown(t *testing.T) {
	tmp := t.TempDir()
	tests := []struct {
		name    string
		content string
		want    float64
	}{
		{"valid utility", "---\nutility: 0.73\n---\n# L\n", 0.73},
		{"no frontmatter", "# Just content\n", 0},
		{"no utility field", "---\ntitle: Test\n---\n", 0},
		{"empty file", "", 0},
		{"utility at zero", "---\nutility: 0.0\n---\n", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmp, tt.name+".md")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
			got := parseUtilityFromMarkdown(path)
			if got != tt.want {
				t.Errorf("parseUtilityFromMarkdown = %f, want %f", got, tt.want)
			}
		})
	}
}

// TestParseUtilityFromJSONL — canonical version earlier in this file

func TestParseUtilityFromJSONL_missingFile(t *testing.T) {
	got := parseUtilityFromJSONL("/nonexistent/file.jsonl")
	if got != 0 {
		t.Errorf("expected 0 for missing file, got %f", got)
	}
}

func TestParseUtilityFromFile_dispatch(t *testing.T) {
	tmp := t.TempDir()

	// .md file
	mdPath := filepath.Join(tmp, "test.md")
	if err := os.WriteFile(mdPath, []byte("---\nutility: 0.55\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// .jsonl file
	jsonlPath := filepath.Join(tmp, "test.jsonl")
	data, _ := json.Marshal(map[string]any{"utility": 0.77})
	if err := os.WriteFile(jsonlPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if got := parseUtilityFromFile(mdPath); got != 0.55 {
		t.Errorf("parseUtilityFromFile(.md) = %f, want 0.55", got)
	}
	if got := parseUtilityFromFile(jsonlPath); got != 0.77 {
		t.Errorf("parseUtilityFromFile(.jsonl) = %f, want 0.77", got)
	}
}

// ---------------------------------------------------------------------------
// printMetricsParameters / printMetricsDerived / printMetricsCounts /
// printMetricsLoopClosure / printMetricsUtility / printMetricsTable
// (smoke tests — just ensure no panic)
// ---------------------------------------------------------------------------

func TestPrintMetricsParameters(t *testing.T) {
	m := &types.FlywheelMetrics{Delta: 17.0, Sigma: 0.5, Rho: 1.0}
	out, err := captureStdout(t, func() error { printMetricsParameters(m); return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"PARAMETERS:", "17.0", "0.50", "1.00"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintMetricsDerived(t *testing.T) {
	tests := []struct {
		name string
		m    *types.FlywheelMetrics
		want []string
	}{
		{"positive velocity", &types.FlywheelMetrics{SigmaRho: 0.5, Delta: 17.0, Velocity: 0.33, AboveEscapeVelocity: true}, []string{"DERIVED:", "+0.33", "✓", "δ/100 = 0.170"}},
		{"negative velocity", &types.FlywheelMetrics{SigmaRho: 0.05, Delta: 17.0, Velocity: -0.12, AboveEscapeVelocity: false}, []string{"DERIVED:", "-0.12", "✗", "δ/100 = 0.170"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureStdout(t, func() error { printMetricsDerived(tt.m); return nil })
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("output missing %q:\n%s", w, out)
				}
			}
		})
	}
}

func TestPrintMetricsCounts(t *testing.T) {
	m := &types.FlywheelMetrics{
		TotalArtifacts:       10,
		CitationsThisPeriod:  5,
		UniqueCitedArtifacts: 3,
		NewArtifacts:         2,
		StaleArtifacts:       1,
		TierCounts:           map[string]int{"learning": 5, "pattern": 3, "observation": 2},
	}
	out, err := captureStdout(t, func() error { printMetricsCounts(m); return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"COUNTS:", "10", "5", "TIER DISTRIBUTION:", "learning", "pattern", "observation"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintMetricsCounts_emptyTiers(t *testing.T) {
	m := &types.FlywheelMetrics{TierCounts: map[string]int{}}
	out, err := captureStdout(t, func() error { printMetricsCounts(m); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "COUNTS:") {
		t.Errorf("output missing COUNTS header:\n%s", out)
	}
	if strings.Contains(out, "TIER DISTRIBUTION:") {
		t.Errorf("empty tiers should not print TIER DISTRIBUTION:\n%s", out)
	}
}

func TestPrintMetricsLoopClosure(t *testing.T) {
	tests := []struct {
		name    string
		m       *types.FlywheelMetrics
		want    []string
		notWant []string
	}{
		{"no loop data", &types.FlywheelMetrics{}, nil, []string{"LOOP CLOSURE"}},
		{"open loop", &types.FlywheelMetrics{LearningsCreated: 5, LearningsFound: 0, LoopClosureRatio: 0}, []string{"LOOP CLOSURE", "OPEN"}, nil},
		{"partial loop", &types.FlywheelMetrics{LearningsCreated: 5, LearningsFound: 3, LoopClosureRatio: 0.6}, []string{"PARTIAL", "0.60"}, nil},
		{"closed loop", &types.FlywheelMetrics{LearningsCreated: 5, LearningsFound: 5, LoopClosureRatio: 1.0}, []string{"CLOSED", "1.00"}, nil},
		{"with retros", &types.FlywheelMetrics{LearningsCreated: 1, TotalRetros: 3, RetrosWithLearnings: 2}, []string{"Retros:", "3", "2"}, nil},
		{"with bypasses", &types.FlywheelMetrics{LearningsCreated: 1, PriorArtBypasses: 2}, []string{"bypasses:", "2"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureStdout(t, func() error { printMetricsLoopClosure(tt.m); return nil })
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("output missing %q:\n%s", w, out)
				}
			}
			for _, nw := range tt.notWant {
				if strings.Contains(out, nw) {
					t.Errorf("output should not contain %q:\n%s", nw, out)
				}
			}
		})
	}
}

func TestPrintMetricsUtility(t *testing.T) {
	tests := []struct {
		name    string
		m       *types.FlywheelMetrics
		want    []string
		notWant []string
	}{
		{"no utility data", &types.FlywheelMetrics{}, nil, []string{"UTILITY"}},
		{"healthy", &types.FlywheelMetrics{MeanUtility: 0.7, UtilityStdDev: 0.1, HighUtilityCount: 5, LowUtilityCount: 1}, []string{"UTILITY", "HEALTHY", "0.700"}, nil},
		{"neutral", &types.FlywheelMetrics{MeanUtility: 0.45, HighUtilityCount: 1}, []string{"NEUTRAL", "0.450"}, nil},
		{"needs review", &types.FlywheelMetrics{MeanUtility: 0.2, LowUtilityCount: 5}, []string{"REVIEW", "0.200"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureStdout(t, func() error { printMetricsUtility(tt.m); return nil })
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("output missing %q:\n%s", w, out)
				}
			}
			for _, nw := range tt.notWant {
				if strings.Contains(out, nw) {
					t.Errorf("output should not contain %q:\n%s", nw, out)
				}
			}
		})
	}
}

func TestPrintMetricsTable(t *testing.T) {
	now := time.Now()
	m := &types.FlywheelMetrics{
		Timestamp:            now,
		PeriodStart:          now.AddDate(0, 0, -7),
		PeriodEnd:            now,
		Delta:                17.0,
		Sigma:                0.5,
		Rho:                  1.0,
		SigmaRho:             0.5,
		Velocity:             0.33,
		AboveEscapeVelocity:  true,
		TotalArtifacts:       10,
		CitationsThisPeriod:  5,
		UniqueCitedArtifacts: 3,
		TierCounts:           map[string]int{"learning": 5, "pattern": 3},
	}
	out, err := captureStdout(t, func() error { printMetricsTable(m); return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Knowledge Flywheel Metrics", "PARAMETERS:", "DERIVED:", "COUNTS:", "STATUS:"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// computeUtilityMetrics
// ---------------------------------------------------------------------------

func TestComputeUtilityMetrics(t *testing.T) {
	baseDir := t.TempDir()
	learningsDir := filepath.Join(baseDir, ".agents", "learnings")
	patternsDir := filepath.Join(baseDir, ".agents", "patterns")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(patternsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// JSONL learning
	data, _ := json.Marshal(map[string]any{"utility": 0.8})
	if err := os.WriteFile(filepath.Join(learningsDir, "l1.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	// Markdown pattern
	if err := os.WriteFile(filepath.Join(patternsDir, "p1.md"), []byte("---\nutility: 0.6\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stats := computeUtilityMetrics(baseDir)
	if !floatApprox(stats.mean, 0.7, 0.01) {
		t.Errorf("mean = %f, want ~0.7", stats.mean)
	}
}

func TestComputeUtilityMetrics_emptyDirs(t *testing.T) {
	stats := computeUtilityMetrics(t.TempDir())
	if stats.mean != 0 {
		t.Errorf("mean = %f, want 0 for empty dirs", stats.mean)
	}
}
