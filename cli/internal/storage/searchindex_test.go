package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/boshu2/agentops/cli/internal/types"
)

// sortedKeywords returns a stable copy for order-independent comparison, since
// ExtractKeywords builds its result from a map and iteration order is random.
func sortedKeywords(kw []string) []string {
	out := append([]string(nil), kw...)
	sort.Strings(out)
	return out
}

func TestWalkIndexableFiles(t *testing.T) {
	root := t.TempDir()
	// Layout: two indexable extensions, one nested, plus non-indexable noise.
	files := map[string]string{
		"a.md":                "# A",
		"data.jsonl":          `{"x":1}`,
		"nested/deep/b.md":    "# B",
		"nested/notes.txt":    "ignored",
		"README":              "ignored",
		"skip.json":           "{}",
		"nested/deep/c.jsonl": "row",
	}
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := WalkIndexableFiles(root)
	if err != nil {
		t.Fatalf("WalkIndexableFiles: %v", err)
	}

	want := []string{
		filepath.Join(root, "a.md"),
		filepath.Join(root, "data.jsonl"),
		filepath.Join(root, "nested/deep/b.md"),
		filepath.Join(root, "nested/deep/c.jsonl"),
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WalkIndexableFiles = %v, want %v", got, want)
	}
}

func TestWalkIndexableFiles_MissingDirSurfacesError(t *testing.T) {
	// A missing root must surface the underlying lstat failure rather than
	// silently returning an empty list, so callers can distinguish "no
	// indexable files" from "the directory does not exist".
	_, err := WalkIndexableFiles(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected an error for a missing root, got nil")
	}
}

func TestArtifactTypeFromPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"learnings", "/x/learnings/foo.md", "learning"},
		{"patterns", "/x/patterns/foo.md", "pattern"},
		{"research", "/x/research/foo.md", "research"},
		{"retro singular", "/x/retro/foo.md", "retro"},
		// ArtifactSubdirs uses "retros" (plural), the real production directory
		// name (see doctor/fix_workspace.go mapping "retros" -> "retro"), so a
		// retros/ artifact must be typed "retro", not "unknown".
		{"retros plural", "/x/retros/foo.md", "retro"},
		{"candidates", "/x/candidates/foo.md", "candidate"},
		{"unknown", "/x/misc/foo.md", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ArtifactTypeFromPath(tt.path); got != tt.want {
				t.Errorf("ArtifactTypeFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestAppendCategoryKeywords(t *testing.T) {
	tests := []struct {
		name     string
		seed     []string
		category string
		tags     []string
		want     []string
	}{
		{"category and tags lowercased", nil, "Infra", []string{"Kube", " Net "}, []string{"infra", "kube", "net"}},
		{"empty category skipped", nil, "", []string{"A"}, []string{"a"}},
		{"blank tags skipped", []string{"seed"}, "C", []string{"", "  "}, []string{"seed", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AppendCategoryKeywords(tt.seed, tt.category, tt.tags)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AppendCategoryKeywords = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractTitle(t *testing.T) {
	long := ""
	for i := 0; i < 90; i++ {
		long += "x"
	}
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"h1 heading", "intro\n# Real Title\nbody", "Real Title"},
		{"first non-empty line", "First Line\nsecond", "First Line"},
		{"skips frontmatter fence", "---\nkey: v\nActual", "key: v"},
		{"truncates long line", long, long[:77] + "..."},
		{"empty is untitled", "", "Untitled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractTitle(tt.content); got != tt.want {
				t.Errorf("ExtractTitle = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			"pattern markers",
			"This has a fix: here and an error: there",
			[]string{"error", "fix"},
		},
		{
			"tags line",
			"**Tags**: alpha, beta , gamma",
			[]string{"alpha", "beta", "gamma"},
		},
		{
			"keywords line and markers combined",
			"decision: ship it\n**Keywords**: one, two",
			[]string{"decision", "one", "two"},
		},
		{
			"none",
			"plain prose with no markers",
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortedKeywords(ExtractKeywords(tt.content))
			want := sortedKeywords(tt.want)
			if len(got) == 0 && len(want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("ExtractKeywords = %v, want %v", got, want)
			}
		})
	}
}

func TestExtractFrontmatterMeta(t *testing.T) {
	tests := []struct {
		name         string
		lines        []string
		wantCategory string
		wantTags     []string
	}{
		{
			"category and bracketed tags",
			[]string{`category: "infra"`, "tags: [kube, net, ci]", "---", "body"},
			"infra",
			[]string{"kube", "net", "ci"},
		},
		{
			"stops at closing fence",
			[]string{"category: a", "---", "category: b"},
			"a",
			nil,
		},
		{
			"single-quoted category",
			[]string{"category: 'ops'"},
			"ops",
			nil,
		},
		{
			"no meta",
			[]string{"just text", "more text"},
			"",
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, tags := ExtractFrontmatterMeta(tt.lines)
			if cat != tt.wantCategory {
				t.Errorf("category = %q, want %q", cat, tt.wantCategory)
			}
			if !reflect.DeepEqual(tags, tt.wantTags) {
				t.Errorf("tags = %v, want %v", tags, tt.wantTags)
			}
		})
	}
}

func TestExtractCategoryAndTags(t *testing.T) {
	content := "---\ncategory: infra\ntags: [a, b]\n---\n# Title\n**Tags**: c, d\n"
	cat, tags := ExtractCategoryAndTags(content)
	if cat != "infra" {
		t.Errorf("category = %q, want infra", cat)
	}
	want := []string{"a", "b", "c", "d"}
	if !reflect.DeepEqual(tags, want) {
		t.Errorf("tags = %v, want %v", tags, want)
	}
}

func TestExtractMarkdownMeta(t *testing.T) {
	lines := []string{"# Title", "**Category**: Reliability", "**Tags**: sre, k8s"}
	cat, tags := ExtractMarkdownMeta(lines)
	if cat != "Reliability" {
		t.Errorf("category = %q, want Reliability", cat)
	}
	if !reflect.DeepEqual(tags, []string{"sre", "k8s"}) {
		t.Errorf("tags = %v, want [sre k8s]", tags)
	}
}

func TestParseBracketedList(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		// TrimSpace runs BEFORE Trim(quotes), so a quoted token with leading
		// space is fully unwrapped: " 'c'" -> TrimSpace -> "'c'" -> Trim -> "c".
		{"quoted tokens fully unwrapped", `["a", b, 'c']`, []string{"a", "b", "c"}},
		{"empty brackets", "[]", nil},
		{"not a list", "a, b", nil},
		{"blanks dropped", "[a, , b]", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseBracketedList(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseBracketedList(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSplitCSV(t *testing.T) {
	if got := SplitCSV("a, b ,,c"); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("SplitCSV = %v, want [a b c]", got)
	}
	if got := SplitCSV("   "); got != nil {
		t.Errorf("SplitCSV(blank) = %v, want nil", got)
	}
}

func TestParseMemRLMetadata(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantUtility float64
		wantMatur   string
	}{
		{"defaults", "no metadata here", types.InitialUtility, "provisional"},
		{"explicit values", "**Utility**: 0.8\n**Maturity**: stable", 0.8, "stable"},
		{"list-form values", "- **Utility**: 0.25\n- **Maturity**: mature", 0.25, "mature"},
		{"malformed utility keeps default", "**Utility**: not-a-number", types.InitialUtility, "provisional"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			util, matur := ParseMemRLMetadata(tt.content)
			if util != tt.wantUtility {
				t.Errorf("utility = %v, want %v", util, tt.wantUtility)
			}
			if matur != tt.wantMatur {
				t.Errorf("maturity = %q, want %q", matur, tt.wantMatur)
			}
		})
	}
}

func TestComputeSearchScore(t *testing.T) {
	tests := []struct {
		name  string
		entry SearchIndexEntry
		terms []string
		want  float64
	}{
		{
			"title content and keyword, no utility weighting",
			SearchIndexEntry{Title: "foo", Content: "foo bar", Keywords: []string{"foo"}, Utility: 0},
			[]string{"foo"},
			6.0, // title 3 + content 1 + keyword 2
		},
		{
			"content only",
			SearchIndexEntry{Title: "zzz", Content: "the bar here", Utility: 0},
			[]string{"bar"},
			1.0,
		},
		{
			"no match",
			SearchIndexEntry{Title: "a", Content: "b", Utility: 0},
			[]string{"zzz"},
			0.0,
		},
		{
			"utility weighting at lambda 0.5, utility 0.5",
			// raw = title 3 + content 1 = 4; weighted = 4*(0.5 + 0.5*0.5) = 3.0
			SearchIndexEntry{Title: "kube", Content: "kube runs", Utility: 0.5},
			[]string{"kube"},
			3.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ComputeSearchScore(tt.entry, tt.terms); got != tt.want {
				t.Errorf("ComputeSearchScore = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateSearchSnippet(t *testing.T) {
	tests := []struct {
		name    string
		content string
		query   string
		maxLen  int
		want    string
	}{
		{
			// Window ends at idx+maxLen (6+10=16), i.e. "alpha beta gamma",
			// then a trailing ellipsis because content extends beyond it.
			"match produces trailing ellipsis",
			"alpha beta gamma delta",
			"beta",
			10,
			"alpha beta gamma...",
		},
		{
			"no match, short content returned whole",
			"short",
			"zzz",
			100,
			"short",
		},
		{
			"no match, long content truncated",
			"abcdefghij",
			"zzz",
			5,
			"ab...",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreateSearchSnippet(tt.content, tt.query, tt.maxLen); got != tt.want {
				t.Errorf("CreateSearchSnippet = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateSearchIndexEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "learnings", "note.md")
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	body := "---\ncategory: infra\ntags: [kube]\n---\n# My Note\nfix: something\n**Utility**: 0.9\n**Maturity**: stable\n"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}

	entry, err := CreateSearchIndexEntry(path, true)
	if err != nil {
		t.Fatalf("CreateSearchIndexEntry: %v", err)
	}
	if entry.ID != "note.md" {
		t.Errorf("ID = %q, want note.md", entry.ID)
	}
	if entry.Type != "learning" {
		t.Errorf("Type = %q, want learning", entry.Type)
	}
	if entry.Title != "My Note" {
		t.Errorf("Title = %q, want My Note", entry.Title)
	}
	if entry.Category != "infra" {
		t.Errorf("Category = %q, want infra", entry.Category)
	}
	if entry.Utility != 0.9 {
		t.Errorf("Utility = %v, want 0.9", entry.Utility)
	}
	if entry.Maturity != "stable" {
		t.Errorf("Maturity = %q, want stable", entry.Maturity)
	}
	kw := sortedKeywords(entry.Keywords)
	// "fix" marker + categorize appends lowercased category "infra" and tag "kube".
	for _, must := range []string{"fix", "infra", "kube"} {
		found := false
		for _, k := range kw {
			if k == must {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("keywords %v missing %q", kw, must)
		}
	}
}

func TestCreateSearchIndexEntry_NoCategorize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte("---\ncategory: infra\n---\n# T\n"), 0600); err != nil {
		t.Fatal(err)
	}
	entry, err := CreateSearchIndexEntry(path, false)
	if err != nil {
		t.Fatalf("CreateSearchIndexEntry: %v", err)
	}
	if entry.Category != "" {
		t.Errorf("Category = %q, want empty when categorize=false", entry.Category)
	}
}

func TestCreateSearchIndexEntry_MissingFile(t *testing.T) {
	_, err := CreateSearchIndexEntry(filepath.Join(t.TempDir(), "nope.md"), true)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestAppendToSearchIndexAndSearchRoundTrip(t *testing.T) {
	base := t.TempDir()

	entries := []*SearchIndexEntry{
		{ID: "1", Type: "learning", Title: "kubernetes networking", Content: "how kube networking works", Keywords: []string{"kube"}, Utility: 0.5},
		{ID: "2", Type: "pattern", Title: "unrelated", Content: "totally different topic", Utility: 0.5},
		{ID: "3", Type: "learning", Title: "kube storage", Content: "kube volumes and storage", Keywords: []string{"storage"}, Utility: 0.9},
	}
	for _, e := range entries {
		if err := AppendToSearchIndex(base, e); err != nil {
			t.Fatalf("AppendToSearchIndex: %v", err)
		}
	}

	results, err := SearchIndex(base, "kube", 0)
	if err != nil {
		t.Fatalf("SearchIndex: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (%+v)", len(results), results)
	}
	// Entry "2" does not contain "kube" and is excluded. Entry "1" title/content/
	// keyword all hit ("kube" is a substring of "kubernetes") => raw 6, weighted
	// 6*(0.5+0.5*0.5)=4.5. Entry "3" hits title+content only => raw 4, weighted
	// 4*(0.5+0.5*0.9)=3.8. So "1" ranks above "3" on score despite lower utility.
	gotIDs := []string{results[0].Entry.ID, results[1].Entry.ID}
	if !reflect.DeepEqual(gotIDs, []string{"1", "3"}) {
		t.Errorf("ranked IDs = %v, want [1 3]", gotIDs)
	}
	if results[0].Score <= results[1].Score {
		t.Errorf("expected strict score ordering, got %v then %v", results[0].Score, results[1].Score)
	}
	for _, r := range results {
		if r.Score <= 0 {
			t.Errorf("result %q has non-positive score %v", r.Entry.ID, r.Score)
		}
		if r.Snippet == "" {
			t.Errorf("result %q missing snippet", r.Entry.ID)
		}
	}

	// limit caps the result count.
	limited, err := SearchIndex(base, "kube", 1)
	if err != nil {
		t.Fatalf("SearchIndex limited: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("limited results = %d, want 1", len(limited))
	}
}

func TestSearchIndex_MissingIndex(t *testing.T) {
	_, err := SearchIndex(t.TempDir(), "anything", 0)
	if err == nil {
		t.Fatal("expected error when index file is absent")
	}
}

func TestComputeSearchIndexStats(t *testing.T) {
	base := t.TempDir()
	entries := []*SearchIndexEntry{
		{ID: "1", Type: "learning", Utility: 0.4},
		{ID: "2", Type: "learning", Utility: 0.6},
		{ID: "3", Type: "pattern", Utility: 0},
	}
	for _, e := range entries {
		if err := AppendToSearchIndex(base, e); err != nil {
			t.Fatal(err)
		}
	}

	stats, err := ComputeSearchIndexStats(base)
	if err != nil {
		t.Fatalf("ComputeSearchIndexStats: %v", err)
	}
	if stats.TotalEntries != 3 {
		t.Errorf("TotalEntries = %d, want 3", stats.TotalEntries)
	}
	if stats.ByType["learning"] != 2 || stats.ByType["pattern"] != 1 {
		t.Errorf("ByType = %v, want learning:2 pattern:1", stats.ByType)
	}
	// Mean over the two entries with utility > 0: (0.4 + 0.6) / 2 = 0.5.
	if stats.MeanUtility != 0.5 {
		t.Errorf("MeanUtility = %v, want 0.5", stats.MeanUtility)
	}
}

func TestComputeSearchIndexStats_MissingIndexIsEmpty(t *testing.T) {
	stats, err := ComputeSearchIndexStats(t.TempDir())
	if err != nil {
		t.Fatalf("ComputeSearchIndexStats: %v", err)
	}
	if stats.TotalEntries != 0 {
		t.Errorf("TotalEntries = %d, want 0 for missing index", stats.TotalEntries)
	}
}

func TestAccumulateEntryStats(t *testing.T) {
	stats := &SearchIndexStats{ByType: make(map[string]int)}
	var total float64
	var count int

	AccumulateEntryStats(stats, SearchIndexEntry{Type: "learning", Utility: 0.4}, &total, &count)
	AccumulateEntryStats(stats, SearchIndexEntry{Type: "pattern", Utility: 0}, &total, &count)

	if stats.TotalEntries != 2 {
		t.Errorf("TotalEntries = %d, want 2", stats.TotalEntries)
	}
	if stats.ByType["learning"] != 1 || stats.ByType["pattern"] != 1 {
		t.Errorf("ByType = %v", stats.ByType)
	}
	if count != 1 {
		t.Errorf("utilityCount = %d, want 1 (only utility>0 counted)", count)
	}
	if total != 0.4 {
		t.Errorf("totalUtility = %v, want 0.4", total)
	}
}
