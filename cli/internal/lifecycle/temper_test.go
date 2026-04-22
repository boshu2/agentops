package lifecycle

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestIsContainedPath(t *testing.T) {
	tmp := t.TempDir()
	inside := filepath.Join(tmp, "a", "b.txt")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(inside, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cases := []struct {
		name string
		base string
		path string
		want bool
	}{
		{"file under base", tmp, inside, true},
		{"base equals path", tmp, tmp, true},
		{"escape with ..", tmp, filepath.Join(tmp, "..", "x"), false},
		{"sibling dir", tmp, filepath.Join(filepath.Dir(tmp), "other"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsContainedPath(tc.base, tc.path); got != tc.want {
				t.Errorf("IsContainedPath(%q,%q) = %v, want %v", tc.base, tc.path, got, tc.want)
			}
		})
	}
}

func TestIsArtifactFile(t *testing.T) {
	cases := map[string]bool{
		"foo.md":      true,
		"bar.jsonl":   true,
		"baz.txt":     false,
		"qux.json":    false,
		"notes.MD":    false, // case-sensitive
		"":            false,
	}
	for name, want := range cases {
		if got := IsArtifactFile(name); got != want {
			t.Errorf("IsArtifactFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestParseMarkdownField(t *testing.T) {
	cases := []struct {
		line     string
		field    string
		wantVal  string
		wantOK   bool
	}{
		{"**ID**: abc-123", "ID", "abc-123", true},
		{"**ID:** abc-123", "ID", "abc-123", true},
		{"- **Maturity**: established", "Maturity", "established", true},
		{"**Other**: x", "ID", "", false},
		{"no match here", "ID", "", false},
		{"**ID**:   spaced   ", "ID", "spaced", true},
	}
	for _, tc := range cases {
		gotVal, gotOK := ParseMarkdownField(tc.line, tc.field)
		if gotOK != tc.wantOK || gotVal != tc.wantVal {
			t.Errorf("ParseMarkdownField(%q,%q) = (%q,%v), want (%q,%v)", tc.line, tc.field, gotVal, gotOK, tc.wantVal, tc.wantOK)
		}
	}
}

func TestParseMarkdownMeta_Roundtrip(t *testing.T) {
	content := `
**ID**: learn-2026-04-22-abcd
**Maturity**: Established
**Utility**: 0.75
**Confidence**: 0.8
**Schema Version**: 1
**Status**: tempered
`
	meta := ParseMarkdownMeta(content)
	if meta.ID != "learn-2026-04-22-abcd" {
		t.Errorf("ID = %q", meta.ID)
	}
	if meta.Maturity != "established" {
		t.Errorf("Maturity = %q (expected lowered)", meta.Maturity)
	}
	if meta.Utility != 0.75 {
		t.Errorf("Utility = %v", meta.Utility)
	}
	if meta.Confidence != 0.8 {
		t.Errorf("Confidence = %v", meta.Confidence)
	}
	if meta.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d", meta.SchemaVersion)
	}
	if !meta.Tempered {
		t.Error("Tempered should be true")
	}
}

func TestApplyMarkdownLine_StatusLocked(t *testing.T) {
	meta := ArtifactMeta{}
	ApplyMarkdownLine("**Status**: locked", &meta)
	if !meta.Tempered {
		t.Error("locked status should set Tempered=true")
	}
}

func TestValidateArtifactMeta_MissingID(t *testing.T) {
	meta := ArtifactMeta{Maturity: "established", Utility: 1.0, FeedbackCount: 5, SchemaVersion: 1}
	issues, _ := ValidateArtifactMeta(meta, "candidate", 0.5, 3)
	found := false
	for _, i := range issues {
		if strings.Contains(i, "missing ID") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing-ID issue; got %v", issues)
	}
}

func TestValidateArtifactMeta_LowMaturity(t *testing.T) {
	meta := ArtifactMeta{ID: "x", Maturity: "provisional", Utility: 1.0, FeedbackCount: 10, SchemaVersion: 1}
	issues, _ := ValidateArtifactMeta(meta, "established", 0.0, 0)
	if len(issues) == 0 {
		t.Fatalf("expected maturity issue, got none")
	}
	joined := strings.Join(issues, "|")
	if !strings.Contains(joined, "maturity provisional") {
		t.Errorf("expected maturity-below complaint, got %v", issues)
	}
}

func TestValidateArtifactMeta_SchemaVersionWarning(t *testing.T) {
	meta := ArtifactMeta{ID: "x", Maturity: "established", Utility: 1.0, FeedbackCount: 10, SchemaVersion: 0}
	_, warnings := ValidateArtifactMeta(meta, "candidate", 0.0, 0)
	if len(warnings) == 0 {
		t.Fatal("expected schema version warning")
	}
	if !strings.Contains(warnings[0], "schema_version") {
		t.Errorf("warning should mention schema_version, got %q", warnings[0])
	}
}

func TestValidateArtifactMeta_UtilityAndFeedback(t *testing.T) {
	meta := ArtifactMeta{ID: "x", Maturity: "established", Utility: 0.1, FeedbackCount: 0, SchemaVersion: 1}
	issues, _ := ValidateArtifactMeta(meta, "candidate", 0.5, 3)
	if len(issues) < 2 {
		t.Errorf("expected at least 2 issues (utility + feedback), got %d: %v", len(issues), issues)
	}
}

func TestExpandGlobPattern_FiltersOutsideBase(t *testing.T) {
	tmp := t.TempDir()
	otherDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "a.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "b.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	matches, err := ExpandGlobPattern(tmp, filepath.Join(tmp, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match in base dir, got %d: %v", len(matches), matches)
	}
}

func TestExpandDirectoryRecursive_CollectsArtifacts(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	files := []string{
		filepath.Join(tmp, "top.md"),
		filepath.Join(sub, "deep.jsonl"),
		filepath.Join(sub, "ignore.txt"),
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ExpandDirectoryRecursive(tmp, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 artifact files (md+jsonl), got %d: %v", len(got), got)
	}
}

func TestExpandDirectoryFlat_NonRecursive(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "a.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sub", "b.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := ExpandDirectoryFlat(tmp)
	if len(got) != 1 {
		t.Errorf("flat should not descend; got %d files: %v", len(got), got)
	}
}

func TestExpandSinglePattern_RejectsEscapes(t *testing.T) {
	base := t.TempDir()
	_, err := ExpandSinglePattern(base, "../outside.md", false)
	if err == nil {
		t.Fatal("expected containment error")
	}
	if !strings.Contains(err.Error(), "outside allowed directory") {
		t.Errorf("error should mention containment, got %v", err)
	}
}

func TestExpandFilePatterns_MultiplePatterns(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "c.jsonl"} {
		if err := os.WriteFile(filepath.Join(base, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ExpandFilePatterns(base, []string{"*.md", "*.jsonl"}, false)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	if len(got) != 3 {
		t.Errorf("expected 3 files, got %d: %v", len(got), got)
	}
}
