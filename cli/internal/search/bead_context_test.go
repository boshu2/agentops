package search

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitLabels(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a, b, c", []string{"a", "b", "c"}},
		{"  a  ,,  b  ", []string{"a", "b"}},
		{"solo", []string{"solo"}},
	}
	for _, tc := range cases {
		got := SplitLabels(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: got %v, want %v", tc.in, got, tc.want)
		}
	}

	// Empty string: SplitLabels returns an empty (non-nil) slice.
	if got := SplitLabels(""); len(got) != 0 {
		t.Errorf("empty: got %v, want length 0", got)
	}
}

func TestBuildKeywords(t *testing.T) {
	ctx := &BeadContext{
		Title:  "Fix Auth Bug Authentication",
		Labels: []string{"bug", "auth", "AUTH"}, // last is duplicate after lower
	}
	keywords := BuildKeywords(ctx)

	// Duplicates and short words removed
	seen := map[string]bool{}
	for _, k := range keywords {
		seen[k] = true
	}
	if !seen["fix"] || !seen["auth"] || !seen["bug"] {
		t.Errorf("missing expected keywords: %v", keywords)
	}
	// Count of unique lowered words: fix, auth, bug, authentication
	if len(keywords) != 4 {
		t.Errorf("got %d unique: %v", len(keywords), keywords)
	}
}

func TestBuildKeywords_FiltersShort(t *testing.T) {
	ctx := &BeadContext{Title: "a an is the fix"}
	keywords := BuildKeywords(ctx)
	// Short words (<2 chars) stripped: "a" dropped; "an", "is" kept (>=2)
	for _, k := range keywords {
		if len(k) < 2 {
			t.Errorf("short kept: %q", k)
		}
	}
}

func TestResolveBeadContext_EmptyID(t *testing.T) {
	if got := ResolveBeadContext("", ""); got != nil {
		t.Error("empty ID should return nil")
	}
}

func TestResolveBeadContext_Minimal(t *testing.T) {
	tmp := t.TempDir()
	// Clear relevant env vars
	os.Unsetenv("HOOK_BEAD_TITLE")
	os.Unsetenv("HOOK_BEAD_LABELS")
	os.Unsetenv("HOOK_BEAD_PHASE")

	ctx := ResolveBeadContext("bd-1", tmp)
	if ctx == nil {
		t.Fatal("expected non-nil")
	}
	if ctx.ID != "bd-1" {
		t.Errorf("id = %q", ctx.ID)
	}
	if ctx.Title != "" {
		t.Errorf("title should be empty, got %q", ctx.Title)
	}
}

func TestResolveBeadContext_FromEnv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOOK_BEAD_TITLE", "Fix auth")
	t.Setenv("HOOK_BEAD_LABELS", "bug,auth")
	t.Setenv("HOOK_BEAD_PHASE", "discovery")

	ctx := ResolveBeadContext("bd-1", tmp)
	if ctx.Title != "Fix auth" {
		t.Errorf("title = %q", ctx.Title)
	}
	if len(ctx.Labels) != 2 {
		t.Errorf("labels = %v", ctx.Labels)
	}
	if ctx.Phase != "discovery" {
		t.Errorf("phase = %q", ctx.Phase)
	}
	if len(ctx.Keywords) == 0 {
		t.Error("keywords should be built")
	}
}

func TestResolveBeadContext_FromCache(t *testing.T) {
	tmp := t.TempDir()
	os.Unsetenv("HOOK_BEAD_TITLE")
	os.Unsetenv("HOOK_BEAD_LABELS")

	cacheDir := filepath.Join(tmp, ".agents", "ao")
	_ = os.MkdirAll(cacheDir, 0o755)
	cached := BeadContext{ID: "bd-42", Title: "Cached Bug", Labels: []string{"a"}}
	data, _ := json.Marshal(cached)
	_ = os.WriteFile(filepath.Join(cacheDir, BeadContextCacheFile), data, 0o600)

	ctx := ResolveBeadContext("bd-42", tmp)
	if ctx == nil {
		t.Fatal("expected non-nil")
	}
	if ctx.Title != "Cached Bug" {
		t.Errorf("title = %q", ctx.Title)
	}
	if len(ctx.Keywords) == 0 {
		t.Error("keywords should be built from cache")
	}
}

func TestReadBeadCache_WrongID(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, ".agents", "ao")
	_ = os.MkdirAll(cacheDir, 0o755)
	cached := BeadContext{ID: "bd-42"}
	data, _ := json.Marshal(cached)
	_ = os.WriteFile(filepath.Join(cacheDir, BeadContextCacheFile), data, 0o600)

	// Different bead ID should return nil (cache mismatch)
	if got := ReadBeadCache(tmp, "bd-99"); got != nil {
		t.Error("mismatching id should yield nil")
	}
}

func TestReadBeadCache_Missing(t *testing.T) {
	tmp := t.TempDir()
	if got := ReadBeadCache(tmp, "bd-1"); got != nil {
		t.Error("missing cache should yield nil")
	}
}

func TestReadBeadCache_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, ".agents", "ao")
	_ = os.MkdirAll(cacheDir, 0o755)
	_ = os.WriteFile(filepath.Join(cacheDir, BeadContextCacheFile), []byte("not json"), 0o600)
	if got := ReadBeadCache(tmp, "bd-1"); got != nil {
		t.Error("invalid json should yield nil")
	}
}
