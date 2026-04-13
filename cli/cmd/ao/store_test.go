package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestComputeSearchScore(t *testing.T) {
	tests := []struct {
		name       string
		entry      IndexEntry
		queryTerms []string
		wantMin    float64
	}{
		{
			name: "title match scores highest",
			entry: IndexEntry{
				Title:   "Mutex Pattern",
				Content: "unrelated content",
			},
			queryTerms: []string{"mutex"},
			wantMin:    3.0, // title match = 3.0
		},
		{
			name: "content match",
			entry: IndexEntry{
				Title:   "Some Title",
				Content: "Use mutex for shared state",
			},
			queryTerms: []string{"mutex"},
			wantMin:    1.0, // content match = 1.0
		},
		{
			name: "keyword match",
			entry: IndexEntry{
				Title:    "Title",
				Content:  "content",
				Keywords: []string{"mutex", "concurrency"},
			},
			queryTerms: []string{"mutex"},
			wantMin:    2.0, // keyword match = 2.0
		},
		{
			name: "all three match",
			entry: IndexEntry{
				Title:    "Mutex Pattern",
				Content:  "Use mutex for shared state",
				Keywords: []string{"mutex"},
			},
			queryTerms: []string{"mutex"},
			wantMin:    6.0, // 3 + 1 + 2 = 6.0
		},
		{
			name: "no match",
			entry: IndexEntry{
				Title:   "Database Pattern",
				Content: "pooling connections",
			},
			queryTerms: []string{"auth"},
			wantMin:    0.0,
		},
		{
			name: "utility boost",
			entry: IndexEntry{
				Title:   "Mutex Pattern",
				Content: "content",
				Utility: 0.9,
			},
			queryTerms: []string{"mutex"},
			// title match (3.0) weighted by utility: (1-0.5)*3 + 0.5*0.9*3 = 1.5 + 1.35 = 2.85
			wantMin: 2.8,
		},
		{
			name: "multiple query terms",
			entry: IndexEntry{
				Title:   "Mutex Pattern for Go",
				Content: "Use mutex and channels for concurrency",
			},
			queryTerms: []string{"mutex", "channels"},
			wantMin:    4.0, // mutex: title(3) + content(1), channels: content(1) = 5.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeSearchScore(tt.entry, tt.queryTerms)
			if got < tt.wantMin {
				t.Errorf("computeSearchScore() = %v, want >= %v", got, tt.wantMin)
			}
		})
	}

	// Test relative ordering
	t.Run("title match ranks higher than content match", func(t *testing.T) {
		titleEntry := IndexEntry{Title: "Mutex Pattern", Content: "something else"}
		contentEntry := IndexEntry{Title: "Something", Content: "mutex is useful"}

		titleScore := computeSearchScore(titleEntry, []string{"mutex"})
		contentScore := computeSearchScore(contentEntry, []string{"mutex"})

		if titleScore <= contentScore {
			t.Errorf("title match (%.2f) should rank higher than content match (%.2f)",
				titleScore, contentScore)
		}
	})
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "heading",
			content: "# My Document\n\nSome content",
			want:    "My Document",
		},
		{
			name:    "heading after front matter",
			content: "---\nid: test\n---\n# Real Title\n\nContent",
			want:    "Real Title",
		},
		{
			name:    "no heading falls back to first line",
			content: "This is the content\nSecond line",
			want:    "This is the content",
		},
		{
			name:    "empty content",
			content: "",
			want:    "Untitled",
		},
		{
			name:    "only separators",
			content: "---\n---\n",
			want:    "Untitled",
		},
		{
			name:    "long first line truncated",
			content: "This is an extremely long line that exceeds the eighty character limit and should be truncated to prevent overly long titles from appearing",
			want:    "This is an extremely long line that exceeds the eighty character limit and sh...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.content)
			if got != tt.want {
				t.Errorf("extractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractKeywords(t *testing.T) {
	t.Run("from content patterns", func(t *testing.T) {
		content := "This is a pattern: for handling errors. Also a fix: for the issue."
		got := extractKeywords(content)
		if len(got) == 0 {
			t.Error("expected keywords from content patterns")
		}
		// Should find "pattern" and "fix"
		gotMap := make(map[string]bool)
		for _, k := range got {
			gotMap[k] = true
		}
		if !gotMap["pattern"] {
			t.Error("expected 'pattern' keyword")
		}
		if !gotMap["fix"] {
			t.Error("expected 'fix' keyword")
		}
	})

	t.Run("from metadata tags", func(t *testing.T) {
		content := "# Title\n\n**Tags**: auth, database, security\n\nContent"
		got := extractKeywords(content)
		sort.Strings(got)
		found := false
		for _, k := range got {
			if k == "auth" || k == "database" || k == "security" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected tags from metadata, got %v", got)
		}
	})

	t.Run("no keywords", func(t *testing.T) {
		content := "Just some plain text without any special markers."
		got := extractKeywords(content)
		if len(got) != 0 {
			t.Errorf("expected no keywords, got %v", got)
		}
	})

	t.Run("empty content", func(t *testing.T) {
		got := extractKeywords("")
		if len(got) != 0 {
			t.Errorf("expected no keywords for empty content, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// createIndexEntry
// ---------------------------------------------------------------------------

func TestStoreCoverage_CreateIndexEntry(t *testing.T) {
	fixture := newStoreCoverageCreateIndexEntryFixture(t)

	t.Run("basic markdown file", func(t *testing.T) {
		content := "# Test Document\n\nSome content with pattern: stuff.\n"
		entry := fixture.createEntry(t, "doc.md", content, false)
		assertStoreCoverageBasicIndexEntry(t, entry, fixture.path("doc.md"), content)
	})

	t.Run("with categorize", func(t *testing.T) {
		content := "---\ncategory: testing\ntags: [go, unit]\n---\n# Title\n\nContent.\n"
		entry := fixture.createEntry(t, "categorized.md", content, true)
		assertStoreCoverageCategorizedIndexEntry(t, entry)
	})

	t.Run("without categorize has no category", func(t *testing.T) {
		content := "---\ncategory: testing\n---\n# Title\n"
		entry := fixture.createEntry(t, "nocat.md", content, false)
		if entry.Category != "" {
			t.Errorf("Category = %q, want empty when categorize=false", entry.Category)
		}
	})

	t.Run("with utility and maturity", func(t *testing.T) {
		content := "# Doc\n\n**Utility**: 0.8\n**Maturity**: established\n"
		entry := fixture.createEntry(t, "meta.md", content, false)
		assertStoreCoverageUtilityMetadata(t, entry)
	})

	t.Run("type from path", func(t *testing.T) {
		entry := fixture.createEntry(t, "learnings/lesson.md", "# Lesson\n", false)
		if entry.Type != "learning" {
			t.Errorf("Type = %q, want %q", entry.Type, "learning")
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		fixture.assertCreateEntryError(t, "nope.md", false)
	})
}

type storeCoverageCreateIndexEntryFixture struct {
	root string
}

func newStoreCoverageCreateIndexEntryFixture(t *testing.T) storeCoverageCreateIndexEntryFixture {
	t.Helper()
	return storeCoverageCreateIndexEntryFixture{root: t.TempDir()}
}

func (f storeCoverageCreateIndexEntryFixture) createEntry(t *testing.T, relPath, content string, categorize bool) *IndexEntry {
	t.Helper()
	path := f.writeFile(t, relPath, content)
	entry, err := createIndexEntry(path, categorize)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return entry
}

func (f storeCoverageCreateIndexEntryFixture) writeFile(t *testing.T, relPath, content string) string {
	t.Helper()
	path := f.path(relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func (f storeCoverageCreateIndexEntryFixture) path(relPath string) string {
	return filepath.Join(f.root, filepath.FromSlash(relPath))
}

func (f storeCoverageCreateIndexEntryFixture) assertCreateEntryError(t *testing.T, relPath string, categorize bool) {
	t.Helper()
	_, err := createIndexEntry(f.path(relPath), categorize)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func assertStoreCoverageBasicIndexEntry(t *testing.T, entry *IndexEntry, wantPath, wantContent string) {
	t.Helper()
	if entry.Path != wantPath {
		t.Errorf("Path = %q, want %q", entry.Path, wantPath)
	}
	if entry.ID != "doc.md" {
		t.Errorf("ID = %q, want %q", entry.ID, "doc.md")
	}
	if entry.Title != "Test Document" {
		t.Errorf("Title = %q, want %q", entry.Title, "Test Document")
	}
	if entry.Content != wantContent {
		t.Errorf("Content mismatch")
	}
	if entry.IndexedAt.IsZero() {
		t.Error("IndexedAt should not be zero")
	}
	if entry.ModifiedAt.IsZero() {
		t.Error("ModifiedAt should not be zero")
	}
}

func assertStoreCoverageCategorizedIndexEntry(t *testing.T, entry *IndexEntry) {
	t.Helper()
	if entry.Category != "testing" {
		t.Errorf("Category = %q, want %q", entry.Category, "testing")
	}
	if len(entry.Tags) == 0 {
		t.Error("expected tags to be populated")
	}
}

func assertStoreCoverageUtilityMetadata(t *testing.T, entry *IndexEntry) {
	t.Helper()
	if entry.Utility < 0.79 || entry.Utility > 0.81 {
		t.Errorf("Utility = %v, want ~0.8", entry.Utility)
	}
	if entry.Maturity != "established" {
		t.Errorf("Maturity = %q, want %q", entry.Maturity, "established")
	}
}

// ---------------------------------------------------------------------------
// appendToIndex
// ---------------------------------------------------------------------------

func TestStoreCoverage_AppendToIndex(t *testing.T) {
	tmp := t.TempDir()

	entry := &IndexEntry{
		Path:       "/fake/path.md",
		ID:         "test-id",
		Type:       "learning",
		Title:      "Test Title",
		Content:    "Some content",
		IndexedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}

	t.Run("creates index dir and file", func(t *testing.T) {
		if err := appendToIndex(tmp, entry); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		indexPath := filepath.Join(tmp, IndexDir, IndexFileName)
		data, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatalf("read index: %v", err)
		}
		if !strings.Contains(string(data), "test-id") {
			t.Error("expected index to contain entry ID")
		}
	})

	t.Run("appends multiple entries", func(t *testing.T) {
		entry2 := &IndexEntry{
			Path:       "/fake/path2.md",
			ID:         "test-id-2",
			Type:       "pattern",
			Title:      "Second",
			Content:    "More content",
			IndexedAt:  time.Now(),
			ModifiedAt: time.Now(),
		}
		if err := appendToIndex(tmp, entry2); err != nil {
			t.Fatalf("append second entry: %v", err)
		}

		indexPath := filepath.Join(tmp, IndexDir, IndexFileName)
		data, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatalf("read index: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) < 2 {
			t.Errorf("expected at least 2 lines, got %d", len(lines))
		}
	})
}

// ---------------------------------------------------------------------------
// searchIndex
// ---------------------------------------------------------------------------

func TestStoreCoverage_SearchIndex(t *testing.T) {
	tmp := t.TempDir()

	// Populate index
	entries := []*IndexEntry{
		{Path: "/a.md", ID: "a", Type: "learning", Title: "Mutex Pattern", Content: "Use mutex for shared state", Keywords: []string{"mutex", "concurrency"}, Utility: 0.8, IndexedAt: time.Now(), ModifiedAt: time.Now()},
		{Path: "/b.md", ID: "b", Type: "pattern", Title: "Error Handling", Content: "Always check errors in Go", Keywords: []string{"error", "go"}, Utility: 0.6, IndexedAt: time.Now(), ModifiedAt: time.Now()},
		{Path: "/c.md", ID: "c", Type: "research", Title: "Database Pooling", Content: "Connection pool setup guide", Keywords: []string{"database"}, Utility: 0.9, IndexedAt: time.Now(), ModifiedAt: time.Now()},
	}
	for _, e := range entries {
		if err := appendToIndex(tmp, e); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	t.Run("finds matching entries", func(t *testing.T) {
		results, err := searchIndex(tmp, "mutex", 10)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(results) == 0 {
			t.Error("expected results for 'mutex'")
		}
		if results[0].Entry.ID != "a" {
			t.Errorf("first result ID = %q, want %q", results[0].Entry.ID, "a")
		}
		if results[0].Score <= 0 {
			t.Error("expected positive score")
		}
	})

	t.Run("limit works", func(t *testing.T) {
		results, err := searchIndex(tmp, "the", 1)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(results) > 1 {
			t.Errorf("expected at most 1 result, got %d", len(results))
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		results, err := searchIndex(tmp, "zzzznonexistent", 10)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("no index returns error", func(t *testing.T) {
		_, err := searchIndex(filepath.Join(tmp, "nope"), "test", 10)
		if err == nil {
			t.Error("expected error for missing index")
		}
	})

	t.Run("results sorted by score descending", func(t *testing.T) {
		results, err := searchIndex(tmp, "error go", 10)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		for i := 1; i < len(results); i++ {
			if results[i].Score > results[i-1].Score {
				t.Errorf("results not sorted: score[%d]=%v > score[%d]=%v", i, results[i].Score, i-1, results[i-1].Score)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// computeIndexStats
// ---------------------------------------------------------------------------

func TestStoreCoverage_ComputeIndexStats(t *testing.T) {
	t.Run("with entries", func(t *testing.T) {
		tmp := t.TempDir()

		now := time.Now()
		earlier := now.Add(-24 * time.Hour)
		entries := []*IndexEntry{
			{Path: "/a.md", ID: "a", Type: "learning", Utility: 0.8, IndexedAt: earlier, ModifiedAt: earlier},
			{Path: "/b.md", ID: "b", Type: "pattern", Utility: 0.6, IndexedAt: now, ModifiedAt: now},
			{Path: "/c.md", ID: "c", Type: "learning", Utility: 0.0, IndexedAt: now, ModifiedAt: now},
		}
		for _, e := range entries {
			if err := appendToIndex(tmp, e); err != nil {
				t.Fatalf("append: %v", err)
			}
		}

		stats, err := computeIndexStats(tmp)
		if err != nil {
			t.Fatalf("computeIndexStats: %v", err)
		}
		if stats.TotalEntries != 3 {
			t.Errorf("TotalEntries = %d, want 3", stats.TotalEntries)
		}
		if stats.ByType["learning"] != 2 {
			t.Errorf("ByType[learning] = %d, want 2", stats.ByType["learning"])
		}
		if stats.ByType["pattern"] != 1 {
			t.Errorf("ByType[pattern] = %d, want 1", stats.ByType["pattern"])
		}
		// MeanUtility should be (0.8+0.6)/2 = 0.7 (zero utility not counted)
		if stats.MeanUtility < 0.69 || stats.MeanUtility > 0.71 {
			t.Errorf("MeanUtility = %v, want ~0.7", stats.MeanUtility)
		}
	})

	t.Run("no index file returns empty stats", func(t *testing.T) {
		tmp := t.TempDir()
		stats, err := computeIndexStats(tmp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.TotalEntries != 0 {
			t.Errorf("TotalEntries = %d, want 0", stats.TotalEntries)
		}
	})
}

// ---------------------------------------------------------------------------
// walkIndexableFiles
// ---------------------------------------------------------------------------

func TestStoreCoverage_WalkIndexableFiles(t *testing.T) {
	tmp := t.TempDir()

	// Create various files
	files := map[string]string{
		"a.md":        "test",
		"b.jsonl":     "test",
		"c.txt":       "test",
		"d.go":        "test",
		"sub/e.md":    "test",
		"sub/f.jsonl": "test",
	}
	for name, content := range files {
		path := filepath.Join(tmp, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := walkIndexableFiles(tmp)
	if err != nil {
		t.Fatalf("walkIndexableFiles: %v", err)
	}
	// Should find a.md, b.jsonl, sub/e.md, sub/f.jsonl (4 files)
	if len(got) != 4 {
		t.Errorf("found %d files, want 4; got %v", len(got), got)
	}
}

// ---------------------------------------------------------------------------
// collectArtifactFiles
// ---------------------------------------------------------------------------

func TestStoreCoverage_CollectArtifactFiles(t *testing.T) {
	tmp := t.TempDir()

	// Create .agents subdirectories
	for _, sub := range []string{"learnings", "patterns", "research", "retros", "candidates"} {
		dir := filepath.Join(tmp, ".agents", sub)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte("# Test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	files := collectArtifactFiles(tmp)
	if len(files) != 5 {
		t.Errorf("found %d files, want 5; got %v", len(files), files)
	}
}

// ---------------------------------------------------------------------------
// indexFiles
// ---------------------------------------------------------------------------

func TestStoreCoverage_IndexFiles(t *testing.T) {
	tmp := t.TempDir()

	// Create a couple of indexable files
	dir := filepath.Join(tmp, "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.md", "b.md"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("# Test\n\nSome content.\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	files := []string{
		filepath.Join(dir, "a.md"),
		filepath.Join(dir, "b.md"),
		filepath.Join(dir, "nonexistent.md"), // should be skipped
	}

	count := indexFiles(tmp, files, false)
	if count != 2 {
		t.Errorf("indexFiles returned %d, want 2", count)
	}

	// Verify index was written
	indexPath := filepath.Join(tmp, IndexDir, IndexFileName)
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 index lines, got %d", len(lines))
	}
}

// ---------------------------------------------------------------------------
// printSearchResults (smoke test — just ensure no panic)
// ---------------------------------------------------------------------------

func TestStoreCoverage_PrintSearchResults(t *testing.T) {
	t.Run("empty results", func(t *testing.T) {
		// Should not panic
		printSearchResults("test query", nil)
	})

	t.Run("with results", func(t *testing.T) {
		results := []SearchResult{
			{
				Entry:   IndexEntry{Title: "Test", Type: "learning", Path: "/test.md", Utility: 0.8},
				Score:   3.5,
				Snippet: "some snippet",
			},
		}
		printSearchResults("test query", results)
	})

	t.Run("result without snippet", func(t *testing.T) {
		results := []SearchResult{
			{
				Entry: IndexEntry{Title: "Test", Type: "pattern", Path: "/test.md"},
				Score: 1.0,
			},
		}
		printSearchResults("query", results)
	})
}

// ---------------------------------------------------------------------------
// printIndexStats (smoke test — just ensure no panic)
// ---------------------------------------------------------------------------

func TestStoreCoverage_PrintIndexStats(t *testing.T) {
	t.Run("empty stats", func(t *testing.T) {
		stats := &IndexStats{
			ByType:    make(map[string]int),
			IndexPath: "/fake/path",
		}
		printIndexStats(stats)
	})

	t.Run("with entries and times", func(t *testing.T) {
		stats := &IndexStats{
			TotalEntries: 5,
			ByType:       map[string]int{"learning": 3, "pattern": 2},
			MeanUtility:  0.75,
			OldestEntry:  time.Now().Add(-24 * time.Hour),
			NewestEntry:  time.Now(),
			IndexPath:    "/fake/path",
		}
		printIndexStats(stats)
	})

	t.Run("with zero oldest entry", func(t *testing.T) {
		stats := &IndexStats{
			TotalEntries: 0,
			ByType:       map[string]int{},
			IndexPath:    "/fake/path",
		}
		printIndexStats(stats)
	})
}

// ---------------------------------------------------------------------------
// searchIndex JSONL line parse resilience
// ---------------------------------------------------------------------------

func TestStoreCoverage_SearchIndexMalformedLines(t *testing.T) {
	tmp := t.TempDir()

	// Write index with one valid and one malformed line
	indexDir := filepath.Join(tmp, IndexDir)
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		t.Fatal(err)
	}
	valid := IndexEntry{
		Path: "/a.md", ID: "a", Type: "learning", Title: "Mutex",
		Content: "Use mutex", IndexedAt: time.Now(), ModifiedAt: time.Now(),
	}
	validJSON, _ := json.Marshal(valid)
	indexPath := filepath.Join(indexDir, IndexFileName)
	content := "NOT VALID JSON\n" + string(validJSON) + "\n"
	if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := searchIndex(tmp, "mutex", 10)
	if err != nil {
		t.Fatalf("searchIndex with malformed lines: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (skipping malformed), got %d", len(results))
	}
}
