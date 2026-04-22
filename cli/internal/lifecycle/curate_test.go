package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseFrontmatter_Basic(t *testing.T) {
	doc := "---\nid: abc\nutility: 0.8\n---\nbody goes here\n"
	fm, body := ParseFrontmatter(doc)
	if fm["id"] != "abc" {
		t.Errorf("id = %v", fm["id"])
	}
	if body != "body goes here" {
		t.Errorf("body = %q", body)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	doc := "just a body\n"
	fm, body := ParseFrontmatter(doc)
	if len(fm) != 0 {
		t.Errorf("fm should be empty, got %v", fm)
	}
	if body != doc {
		t.Errorf("body should equal input")
	}
}

func TestParseFrontmatter_UnclosedDelimiter(t *testing.T) {
	doc := "---\nid: abc\nno close here"
	fm, body := ParseFrontmatter(doc)
	if len(fm) != 0 {
		t.Errorf("fm should be empty when delimiter unclosed, got %v", fm)
	}
	if body != doc {
		t.Errorf("body should equal input when no close delimiter")
	}
}

func TestParseFrontmatter_InvalidYAML(t *testing.T) {
	doc := "---\nid: [unclosed\n---\nbody\n"
	fm, body := ParseFrontmatter(doc)
	if len(fm) != 0 {
		t.Errorf("invalid yaml should produce empty fm, got %v", fm)
	}
	if body != "body" {
		t.Errorf("body = %q", body)
	}
}

func TestFrontmatterString_Types(t *testing.T) {
	now := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	fm := map[string]any{
		"s":    "hello",
		"n":    42,
		"time": now,
		"nil":  nil,
	}
	if got := FrontmatterString(fm, "s"); got != "hello" {
		t.Errorf("string = %q", got)
	}
	if got := FrontmatterString(fm, "n"); got != "42" {
		t.Errorf("int = %q", got)
	}
	if got := FrontmatterString(fm, "time"); got != "2026-04-22" {
		t.Errorf("time = %q", got)
	}
	if got := FrontmatterString(fm, "missing"); got != "" {
		t.Errorf("missing = %q", got)
	}
	if got := FrontmatterString(fm, "nil"); got != "" {
		t.Errorf("nil = %q", got)
	}
}

func TestGenerateArtifactID_DeterministicAndPrefixed(t *testing.T) {
	a := GenerateArtifactID("learning", "2026-04-22", "hello world")
	b := GenerateArtifactID("learning", "2026-04-22", "hello world")
	if a != b {
		t.Errorf("ID should be deterministic; got %q vs %q", a, b)
	}
	if a[:6] != "learn-" {
		t.Errorf("ID should be prefixed with 'learn-', got %q", a)
	}

	c := GenerateArtifactID("decision", "2026-04-22", "hello world")
	if c[:6] != "decis-" {
		t.Errorf("decision prefix, got %q", c)
	}
	f := GenerateArtifactID("failure", "2026-04-22", "x")
	if f[:5] != "fail-" {
		t.Errorf("failure prefix, got %q", f)
	}
	p := GenerateArtifactID("pattern", "2026-04-22", "x")
	if p[:5] != "patt-" {
		t.Errorf("pattern prefix, got %q", p)
	}
}

func TestGenerateArtifactID_DifferentContent(t *testing.T) {
	a := GenerateArtifactID("learning", "2026-04-22", "one")
	b := GenerateArtifactID("learning", "2026-04-22", "two")
	if a == b {
		t.Error("different content should yield different IDs")
	}
}

func TestArtifactDir(t *testing.T) {
	if ArtifactDir("pattern") != ".agents/patterns" {
		t.Errorf("pattern dir wrong")
	}
	if ArtifactDir("learning") != ".agents/learnings" {
		t.Errorf("learning dir wrong")
	}
	if ArtifactDir("decision") != ".agents/learnings" {
		t.Errorf("decision dir should default to learnings")
	}
}

func TestResolveCurateGoalsFile_FindsFirst(t *testing.T) {
	tmp := t.TempDir()
	wd, _ := os.Getwd()
	defer func() { _ = os.Chdir(wd) }()
	_ = os.Chdir(tmp)

	if _, err := ResolveCurateGoalsFile(); err == nil {
		t.Fatalf("expected error when no goals file present")
	}

	if err := os.WriteFile("GOALS.md", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveCurateGoalsFile()
	if err != nil {
		t.Fatalf("ResolveCurateGoalsFile: %v", err)
	}
	if got != "GOALS.md" {
		t.Errorf("got %q, want GOALS.md", got)
	}
}

func TestCountArtifactsInDir(t *testing.T) {
	tmp := t.TempDir()

	writeArtifact := func(name, typeStr, curatedAt string) {
		a := CurateArtifact{ID: name, Type: typeStr, CuratedAt: curatedAt}
		data, _ := json.Marshal(a)
		if err := os.WriteFile(filepath.Join(tmp, name+".json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeArtifact("l1", "learning", "2026-04-20T10:00:00Z")
	writeArtifact("l2", "learning", "2026-04-22T10:00:00Z")
	writeArtifact("d1", "decision", "2026-04-21T10:00:00Z")

	_ = os.WriteFile(filepath.Join(tmp, "not-json.txt"), []byte("x"), 0o600)

	counts, latest := CountArtifactsInDir(tmp)
	if counts["learning"] != 2 {
		t.Errorf("learning count = %d, want 2", counts["learning"])
	}
	if counts["decision"] != 1 {
		t.Errorf("decision count = %d, want 1", counts["decision"])
	}
	expected, _ := time.Parse(time.RFC3339, "2026-04-22T10:00:00Z")
	if !latest.Equal(expected) {
		t.Errorf("latest = %v, want %v", latest, expected)
	}
}

func TestCountArtifactsInDir_NonExistentDir(t *testing.T) {
	counts, latest := CountArtifactsInDir("/nonexistent/path/xyz")
	if len(counts) != 0 {
		t.Errorf("counts should be empty, got %v", counts)
	}
	if !latest.IsZero() {
		t.Errorf("latest should be zero, got %v", latest)
	}
}

func TestCountArtifactsSince(t *testing.T) {
	tmp := t.TempDir()
	learnDir := filepath.Join(tmp, "learnings")
	patDir := filepath.Join(tmp, "patterns")
	_ = os.MkdirAll(learnDir, 0o755)
	_ = os.MkdirAll(patDir, 0o755)

	writeArt := func(dir, name, ts string) {
		a := CurateArtifact{ID: name, Type: "learning", CuratedAt: ts}
		data, _ := json.Marshal(a)
		_ = os.WriteFile(filepath.Join(dir, name+".json"), data, 0o600)
	}
	writeArt(learnDir, "old", "2026-04-01T00:00:00Z")
	writeArt(learnDir, "new", "2026-04-22T00:00:00Z")
	writeArt(patDir, "newpat", "2026-04-23T00:00:00Z")

	since, _ := time.Parse(time.RFC3339, "2026-04-15T00:00:00Z")
	count := CountArtifactsSince(learnDir, patDir, since)
	if count != 2 {
		t.Errorf("count since = %d, want 2", count)
	}
}

func TestValidArtifactTypes(t *testing.T) {
	for _, t1 := range []string{"learning", "decision", "failure", "pattern"} {
		if !ValidArtifactTypes[t1] {
			t.Errorf("%q should be valid", t1)
		}
	}
	if ValidArtifactTypes["bogus"] {
		t.Error("'bogus' should not be valid")
	}
}
