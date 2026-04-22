package search

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestFindingMatchesQuery(t *testing.T) {
	f := KnowledgeFinding{
		ID: "f-1", Title: "Null Pointer", Summary: "Nil deref in parser",
		SourceSkill: "code-review", Severity: "high",
		ScopeTags: []string{"go", "parser"},
	}

	if !FindingMatchesQuery(f, "") {
		t.Error("empty query should match all")
	}
	if !FindingMatchesQuery(f, "null") {
		t.Error("should match title")
	}
	if !FindingMatchesQuery(f, "go") {
		t.Error("should match scope tag")
	}
	if !FindingMatchesQuery(f, "high") {
		t.Error("should match severity")
	}
	if FindingMatchesQuery(f, "nonexistent") {
		t.Error("unknown query shouldn't match")
	}
	// Note: caller is expected to pre-lowercase the query. Upper-case queries won't match.
	if FindingMatchesQuery(f, "NULL") {
		t.Error("query is expected to be pre-lowered; upper-case should not match")
	}
}

func TestFindingStatusActiveForRetrieval(t *testing.T) {
	cases := map[string]bool{
		"":           true,
		"active":     true,
		"retired":    false,
		"RETIRED":    false,
		"superseded": false,
		"unknown":    true,
	}
	for in, want := range cases {
		if got := FindingStatusActiveForRetrieval(in); got != want {
			t.Errorf("%q: got %v, want %v", in, got, want)
		}
	}
}

func TestParseFindingTime(t *testing.T) {
	ts, ok := ParseFindingTime("2026-04-22T12:00:00Z")
	if !ok {
		t.Fatal("expected ok=true for valid RFC3339")
	}
	if ts.Year() != 2026 {
		t.Errorf("year = %d", ts.Year())
	}

	_, ok = ParseFindingTime("")
	if ok {
		t.Error("empty should fail")
	}

	_, ok = ParseFindingTime("not a time")
	if ok {
		t.Error("invalid should fail")
	}
}

func TestTrimField(t *testing.T) {
	cases := map[string]string{
		"key: value":       "value",
		`key: "quoted"`:    "quoted",
		"key: 'single'":    "single",
		"key:   spaced   ": "spaced",
		"no colon here":    "",
	}
	for in, want := range cases {
		if got := TrimField(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestParseListField(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"[a, b, c]", []string{"a", "b", "c"}},
		{"a, b", []string{"a", "b"}},
		{"", nil},
		{"[]", nil},
		{`["quoted", "also"]`, []string{"quoted", "also"}},
	}
	for _, tc := range cases {
		got := ParseListField(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: got %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseIntField(t *testing.T) {
	if got := ParseIntField("42"); got != 42 {
		t.Errorf("got %d", got)
	}
	if got := ParseIntField("not a number"); got != 0 {
		t.Errorf("got %d", got)
	}
	if got := ParseIntField("  7  "); got != 7 {
		t.Errorf("trimmed: got %d", got)
	}
}

func TestApplyFindingField(t *testing.T) {
	var f KnowledgeFinding
	ApplyFindingField(&f, "id: f-123")
	ApplyFindingField(&f, "title: My Title")
	ApplyFindingField(&f, "severity: high")
	ApplyFindingField(&f, "status: retired")
	ApplyFindingField(&f, "scope_tags: [go, rust]")
	ApplyFindingField(&f, "hit_count: 5")

	if f.ID != "f-123" || f.Title != "My Title" || f.Severity != "high" || f.Status != "retired" {
		t.Errorf("fields wrong: %+v", f)
	}
	if len(f.ScopeTags) != 2 || f.ScopeTags[0] != "go" {
		t.Errorf("scope tags wrong: %v", f.ScopeTags)
	}
	if f.HitCount != 5 {
		t.Errorf("hit count = %d", f.HitCount)
	}
}

func TestApplyFindingField_Hyphenated(t *testing.T) {
	// Both hyphen and underscore forms should be supported
	var f KnowledgeFinding
	ApplyFindingField(&f, "source-skill: my-skill")
	ApplyFindingField(&f, "applicable-languages: [python]")
	if f.SourceSkill != "my-skill" {
		t.Errorf("source-skill = %q", f.SourceSkill)
	}
	if len(f.ApplicableLanguages) != 1 {
		t.Errorf("langs = %v", f.ApplicableLanguages)
	}
}

func TestParseFindingTitle_FromHeading(t *testing.T) {
	f := &KnowledgeFinding{}
	lines := []string{"---", "id: x", "---", "", "# Discovered Bug", "body"}
	ParseFindingTitle(f, lines, 3, "/x/finding.md")
	if f.Title != "Discovered Bug" {
		t.Errorf("got %q", f.Title)
	}
}

func TestParseFindingTitle_FallbackToFilename(t *testing.T) {
	f := &KnowledgeFinding{}
	lines := []string{"---", "---", "body without heading"}
	ParseFindingTitle(f, lines, 2, "/x/my-finding.md")
	if f.Title != "my-finding" {
		t.Errorf("got %q", f.Title)
	}
}

func TestParseFindingTitle_PreservesExistingTitle(t *testing.T) {
	f := &KnowledgeFinding{Title: "Already Set"}
	lines := []string{"# Would be overwritten"}
	ParseFindingTitle(f, lines, 0, "/x/file.md")
	if f.Title != "Already Set" {
		t.Errorf("got %q", f.Title)
	}
}

func TestApplyFindingFreshness(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "f.md")
	_ = os.WriteFile(file, []byte("content"), 0o600)
	old := time.Now().Add(-4 * 7 * 24 * time.Hour)
	_ = os.Chtimes(file, old, old)

	f := &KnowledgeFinding{}
	now := time.Now()
	ApplyFindingFreshness(f, file, now)
	if f.AgeWeeks < 3 || f.AgeWeeks > 5 {
		t.Errorf("age = %v", f.AgeWeeks)
	}
	if f.FreshnessScore <= 0 || f.FreshnessScore > 1 {
		t.Errorf("freshness = %v", f.FreshnessScore)
	}
	if f.Utility == 0 {
		t.Error("utility should be initialized")
	}
}

func TestApplyFindingFreshness_MissingFile(t *testing.T) {
	f := &KnowledgeFinding{}
	ApplyFindingFreshness(f, "/nonexistent/xyz.md", time.Now())
	if f.FreshnessScore != 0.5 {
		t.Errorf("missing file should default to 0.5, got %v", f.FreshnessScore)
	}
}

func TestParseFindingFile(t *testing.T) {
	tmp := t.TempDir()
	content := `---
id: f-abc-123
title: My Finding
severity: high
source-skill: vibe
---

# Override Title

Summary paragraph here.
`
	path := filepath.Join(tmp, "f-abc-123.md")
	_ = os.WriteFile(path, []byte(content), 0o600)

	f, err := ParseFindingFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.ID != "f-abc-123" {
		t.Errorf("id = %q", f.ID)
	}
	if f.Title != "My Finding" {
		t.Errorf("frontmatter title should win, got %q", f.Title)
	}
	if f.Severity != "high" {
		t.Errorf("severity = %q", f.Severity)
	}
	if f.SourceSkill != "vibe" {
		t.Errorf("source = %q", f.SourceSkill)
	}
}

func TestParseFindingFile_MissingFile(t *testing.T) {
	_, err := ParseFindingFile("/nonexistent/finding.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
