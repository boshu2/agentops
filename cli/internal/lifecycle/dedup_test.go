package lifecycle

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCollectDedupFiles_NoDirs(t *testing.T) {
	tmp := t.TempDir()
	files, err := CollectDedupFiles(tmp)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if files != nil {
		t.Errorf("expected nil for empty dirs, got %v", files)
	}
}

func TestCollectDedupFiles_WithFiles(t *testing.T) {
	tmp := t.TempDir()
	learnings := filepath.Join(tmp, ".agents", "learnings")
	patterns := filepath.Join(tmp, ".agents", "patterns")
	if err := os.MkdirAll(learnings, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(patterns, 0o755); err != nil {
		t.Fatal(err)
	}
	writes := []string{
		filepath.Join(learnings, "a.md"),
		filepath.Join(learnings, "b.jsonl"),
		filepath.Join(patterns, "p.md"),
		filepath.Join(learnings, "ignore.txt"),
	}
	for _, f := range writes {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files, err := CollectDedupFiles(tmp)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(files) != 3 {
		t.Errorf("expected 3 artifact files, got %d: %v", len(files), files)
	}
}

func TestHashNormalizedContent_NormalizesWhitespaceAndCase(t *testing.T) {
	a := HashNormalizedContent("Hello   World")
	b := HashNormalizedContent("hello world")
	if a != b {
		t.Errorf("normalization should equate %q and %q", "Hello   World", "hello world")
	}
	c := HashNormalizedContent("# heading **bold** `code`")
	d := HashNormalizedContent("heading bold code")
	if c != d {
		t.Errorf("markdown punctuation should be normalized away")
	}
}

func TestExtractMarkdownBody(t *testing.T) {
	text := "---\nutility: 0.5\n---\nthe body\nmore body\n"
	body := ExtractMarkdownBody(text)
	if strings.Contains(body, "utility") {
		t.Errorf("body should not contain frontmatter, got %q", body)
	}
	if !strings.Contains(body, "the body") {
		t.Errorf("body missing content, got %q", body)
	}

	// No frontmatter -> returns original
	plain := "just text\n"
	if ExtractMarkdownBody(plain) != plain {
		t.Errorf("no frontmatter should return original")
	}
}

func TestExtractJSONLBody(t *testing.T) {
	body := ExtractJSONLBody(`{"content":"the content","title":"a title"}`)
	if body != "the content" {
		t.Errorf("content key preferred, got %q", body)
	}

	body2 := ExtractJSONLBody(`{"title":"only title"}`)
	if body2 != "only title" {
		t.Errorf("title fallback, got %q", body2)
	}

	if ExtractJSONLBody("not json") != "" {
		t.Errorf("invalid json should return empty")
	}
	if ExtractJSONLBody("") != "" {
		t.Errorf("empty should return empty")
	}
}

func TestReadUtilityFromFrontmatter(t *testing.T) {
	good := "---\nutility: 0.75\nmaturity: candidate\n---\nbody\n"
	if got := ReadUtilityFromFrontmatter(good, 0.5); got != 0.75 {
		t.Errorf("got %v, want 0.75", got)
	}

	missing := "---\nmaturity: candidate\n---\nbody\n"
	if got := ReadUtilityFromFrontmatter(missing, 0.5); got != 0.5 {
		t.Errorf("missing should return default, got %v", got)
	}

	noFM := "just body"
	if got := ReadUtilityFromFrontmatter(noFM, 0.42); got != 0.42 {
		t.Errorf("no frontmatter should return default, got %v", got)
	}
}

func TestReadUtilityFromJSONL(t *testing.T) {
	if got := ReadUtilityFromJSONL(`{"utility":0.9}`, 0.5); got != 0.9 {
		t.Errorf("float: got %v", got)
	}
	if got := ReadUtilityFromJSONL(`{"utility":"0.3"}`, 0.5); got != 0.3 {
		t.Errorf("string-encoded float: got %v", got)
	}
	if got := ReadUtilityFromJSONL(`not json`, 0.5); got != 0.5 {
		t.Errorf("invalid json: got %v", got)
	}
	if got := ReadUtilityFromJSONL(`{"other":1}`, 0.5); got != 0.5 {
		t.Errorf("missing utility: got %v", got)
	}
}

func TestReadUtilityFromFile(t *testing.T) {
	tmp := t.TempDir()
	mdPath := filepath.Join(tmp, "a.md")
	jsonlPath := filepath.Join(tmp, "a.jsonl")

	if err := os.WriteFile(mdPath, []byte("---\nutility: 0.8\n---\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonlPath, []byte(`{"utility":0.2}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ReadUtilityFromFile(mdPath); got != 0.8 {
		t.Errorf("md utility: got %v, want 0.8", got)
	}
	if got := ReadUtilityFromFile(jsonlPath); got != 0.2 {
		t.Errorf("jsonl utility: got %v, want 0.2", got)
	}
	if got := ReadUtilityFromFile(filepath.Join(tmp, "missing.md")); got != 0.5 {
		t.Errorf("missing file should return default, got %v", got)
	}
}

func TestPickHighestUtility(t *testing.T) {
	tmp := t.TempDir()
	files := []string{
		filepath.Join(tmp, "a.md"),
		filepath.Join(tmp, "b.md"),
		filepath.Join(tmp, "c.md"),
	}
	contents := map[string]string{
		files[0]: "---\nutility: 0.3\n---\nbody\n",
		files[1]: "---\nutility: 0.9\n---\nbody\n",
		files[2]: "---\nutility: 0.5\n---\nbody\n",
	}
	for p, c := range contents {
		if err := os.WriteFile(p, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	kept, archived := PickHighestUtility(files)
	if kept != files[1] {
		t.Errorf("kept = %q, want %q", kept, files[1])
	}
	if len(archived) != 2 {
		t.Errorf("archived len = %d, want 2", len(archived))
	}
}

func TestGroupByContentHash_GroupsDuplicates(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.md")
	b := filepath.Join(tmp, "b.md")
	c := filepath.Join(tmp, "c.md")
	_ = os.WriteFile(a, []byte("---\n---\nsame body content here\n"), 0o600)
	_ = os.WriteFile(b, []byte("---\n---\nSame Body Content Here\n"), 0o600)
	_ = os.WriteFile(c, []byte("---\n---\nentirely different stuff\n"), 0o600)

	groups := GroupByContentHash([]string{a, b, c})
	if len(groups) != 2 {
		t.Errorf("expected 2 hash buckets, got %d", len(groups))
	}
	dupCount := 0
	for _, files := range groups {
		if len(files) > 1 {
			dupCount++
		}
	}
	if dupCount != 1 {
		t.Errorf("expected 1 duplicate group, got %d", dupCount)
	}
}

func TestBuildDedupResult(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.md")
	b := filepath.Join(tmp, "b.md")
	c := filepath.Join(tmp, "c.md")
	for _, p := range []string{a, b, c} {
		_ = os.WriteFile(p, []byte("x"), 0o600)
	}

	hashes := map[string][]string{
		"hash1234567890abcdef": {a, b},
		"hash9999999999999999": {c},
	}
	r := BuildDedupResult(hashes, 3, tmp)
	if r.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d", r.TotalFiles)
	}
	if r.UniqueContent != 2 {
		t.Errorf("UniqueContent = %d", r.UniqueContent)
	}
	if r.DuplicateGroups != 1 {
		t.Errorf("DuplicateGroups = %d", r.DuplicateGroups)
	}
	if r.DuplicateFiles != 2 {
		t.Errorf("DuplicateFiles = %d", r.DuplicateFiles)
	}
	if len(r.Groups) != 1 {
		t.Errorf("Groups len = %d", len(r.Groups))
	}
	if len(r.Groups) > 0 && len(r.Groups[0].Hash) != 12 {
		t.Errorf("Hash should be truncated to 12 chars, got %q", r.Groups[0].Hash)
	}
}

func TestMergeDedupGroups_DryRun(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.md")
	b := filepath.Join(tmp, "b.md")
	_ = os.WriteFile(a, []byte("---\nutility: 0.3\n---\nbody\n"), 0o600)
	_ = os.WriteFile(b, []byte("---\nutility: 0.7\n---\nbody\n"), 0o600)

	groups := map[string][]string{"h": {a, b}}
	if err := MergeDedupGroups(groups, tmp, true); err != nil {
		t.Fatalf("err = %v", err)
	}

	// In dry-run both files should still exist
	if _, err := os.Stat(a); err != nil {
		t.Errorf("a should still exist: %v", err)
	}
	if _, err := os.Stat(b); err != nil {
		t.Errorf("b should still exist: %v", err)
	}
	archiveDir := filepath.Join(tmp, ".agents", "archive", "dedup")
	if _, err := os.Stat(archiveDir); err == nil {
		t.Errorf("dry-run should not create archive dir")
	}
}

func TestMergeDedupGroups_ActuallyArchives(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.md")
	b := filepath.Join(tmp, "b.md")
	_ = os.WriteFile(a, []byte("---\nutility: 0.9\n---\nbody\n"), 0o600)
	_ = os.WriteFile(b, []byte("---\nutility: 0.1\n---\nbody\n"), 0o600)

	groups := map[string][]string{"h": {a, b}}
	if err := MergeDedupGroups(groups, tmp, false); err != nil {
		t.Fatalf("err = %v", err)
	}

	// Highest utility (a) should be kept
	if _, err := os.Stat(a); err != nil {
		t.Errorf("a should still exist: %v", err)
	}
	if _, err := os.Stat(b); err == nil {
		t.Errorf("b should have been archived")
	}
	archived := filepath.Join(tmp, ".agents", "archive", "dedup", "b.md")
	if _, err := os.Stat(archived); err != nil {
		t.Errorf("b should exist at archive path %q: %v", archived, err)
	}
}

// unused sort import guard
var _ = sort.Strings
