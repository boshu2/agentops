// practices: [pragmatic-programmer, design-patterns]
package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// ---------------------------------------------------------------------------
// flywheel_close_loop.go: filterAntiPatternTransitions
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// flywheel_close_loop.go: loadExistingIndexEntries
// ---------------------------------------------------------------------------

func TestHelper2_loadExistingIndexEntries(t *testing.T) {
	t.Run("file does not exist", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "nonexistent.jsonl")
		entries := loadExistingIndexEntries(p)
		if len(entries) != 0 {
			t.Fatalf("expected empty map for missing file, got %d entries", len(entries))
		}
	})

	t.Run("valid JSONL entries", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "index.jsonl")

		entry1 := IndexEntry{Path: "/a/b.md", ID: "id1", Type: "learning"}
		entry2 := IndexEntry{Path: "/c/d.md", ID: "id2", Type: "pattern"}
		data1, _ := json.Marshal(entry1)
		data2, _ := json.Marshal(entry2)
		writeFile(t, p, string(data1)+"\n"+string(data2)+"\n")

		entries := loadExistingIndexEntries(p)
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries["/a/b.md"].ID != "id1" {
			t.Fatalf("expected id1, got %q", entries["/a/b.md"].ID)
		}
		if entries["/c/d.md"].ID != "id2" {
			t.Fatalf("expected id2, got %q", entries["/c/d.md"].ID)
		}
	})

	t.Run("skips malformed lines", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "index.jsonl")
		entry := IndexEntry{Path: "/ok.md", ID: "ok", Type: "learning"}
		data, _ := json.Marshal(entry)
		writeFile(t, p, string(data)+"\nnot json\n{\"bad\": true}\n")

		entries := loadExistingIndexEntries(p)
		// Only the first line has a valid Path; the third line has no Path field
		if len(entries) != 1 {
			t.Fatalf("expected 1 valid entry, got %d", len(entries))
		}
	})
}

// ---------------------------------------------------------------------------
// flywheel_close_loop.go: upsertIndexPaths
// ---------------------------------------------------------------------------

func TestHelper2_upsertIndexPaths(t *testing.T) {
	t.Run("skips empty and nonexistent paths", func(t *testing.T) {
		existing := make(map[string]IndexEntry)
		indexed := upsertIndexPaths(existing, []string{"", "/nonexistent/path.md"}, false)
		if indexed != 0 {
			t.Fatalf("expected 0 indexed, got %d", indexed)
		}
	})

	t.Run("indexes real files", func(t *testing.T) {
		tmp := t.TempDir()
		f1 := filepath.Join(tmp, "learning1.md")
		f2 := filepath.Join(tmp, "learning2.md")
		writeFile(t, f1, "---\nid: L001\n---\nSome learning content\n")
		writeFile(t, f2, "---\nid: L002\n---\nAnother learning\n")

		existing := make(map[string]IndexEntry)
		indexed := upsertIndexPaths(existing, []string{f1, f2}, false)
		if indexed != 2 {
			t.Fatalf("expected 2 indexed, got %d", indexed)
		}
		if _, ok := existing[f1]; !ok {
			t.Fatalf("expected entry for %s", f1)
		}
		if _, ok := existing[f2]; !ok {
			t.Fatalf("expected entry for %s", f2)
		}
	})

	t.Run("upserts overwrite existing", func(t *testing.T) {
		tmp := t.TempDir()
		f1 := filepath.Join(tmp, "learning.md")
		writeFile(t, f1, "---\nid: L001\n---\nOriginal content\n")

		existing := map[string]IndexEntry{
			f1: {Path: f1, ID: "old-id"},
		}
		indexed := upsertIndexPaths(existing, []string{f1}, false)
		if indexed != 1 {
			t.Fatalf("expected 1 indexed, got %d", indexed)
		}
		// The entry should have been replaced
		if existing[f1].Path != f1 {
			t.Fatalf("expected entry path to be %s", f1)
		}
	})
}

// ---------------------------------------------------------------------------
// maturity.go: parseFrontmatterFields
// ---------------------------------------------------------------------------

func TestHelper2_parseFrontmatterFields(t *testing.T) {
	t.Run("extracts requested fields", func(t *testing.T) {
		tmp := t.TempDir()
		f := filepath.Join(tmp, "test.md")
		writeFile(t, f, "---\nvalid_until: 2026-06-30\nexpiry_status: active\nmaturity: provisional\n---\nBody content\n")

		fields, err := parseFrontmatterFields(f, "valid_until", "expiry_status")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fields["valid_until"] != "2026-06-30" {
			t.Fatalf("valid_until: got %q", fields["valid_until"])
		}
		if fields["expiry_status"] != "active" {
			t.Fatalf("expiry_status: got %q", fields["expiry_status"])
		}
	})

	t.Run("handles quoted values", func(t *testing.T) {
		tmp := t.TempDir()
		f := filepath.Join(tmp, "test.md")
		writeFile(t, f, "---\nvalid_until: \"2026-06-30\"\n---\n")

		fields, err := parseFrontmatterFields(f, "valid_until")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fields["valid_until"] != "2026-06-30" {
			t.Fatalf("expected quotes stripped, got %q", fields["valid_until"])
		}
	})

	t.Run("no frontmatter", func(t *testing.T) {
		tmp := t.TempDir()
		f := filepath.Join(tmp, "test.md")
		writeFile(t, f, "Just regular content\n")

		fields, err := parseFrontmatterFields(f, "valid_until")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(fields) != 0 {
			t.Fatalf("expected empty fields, got %v", fields)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := parseFrontmatterFields("/nonexistent.md", "valid_until")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

// ---------------------------------------------------------------------------
// maturity.go: isEvictionEligible
// ---------------------------------------------------------------------------

func TestHelper2_isEvictionEligible(t *testing.T) {
	tests := []struct {
		name       string
		utility    float64
		confidence float64
		maturity   string
		want       bool
	}{
		{"established never eligible", 0.1, 0.1, "established", false},
		{"utility too high", 0.5, 0.1, "provisional", false},
		{"confidence too high", 0.1, 0.5, "provisional", false},
		{"eligible provisional", 0.1, 0.1, "provisional", true},
		{"eligible candidate", 0.2, 0.1, "candidate", true},
		{"boundary utility 0.3", 0.3, 0.1, "provisional", false},
		{"boundary confidence 0.3", 0.1, 0.3, "provisional", false},
		{"just under thresholds", 0.29, 0.29, "candidate", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEvictionEligible(tt.utility, tt.confidence, tt.maturity)
			if got != tt.want {
				t.Fatalf("isEvictionEligible(%.2f, %.2f, %q) = %v, want %v",
					tt.utility, tt.confidence, tt.maturity, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// maturity.go: evictionCitationStatus
// ---------------------------------------------------------------------------

func TestHelper2_evictionCitationStatus(t *testing.T) {
	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		file      string
		lastCited map[string]time.Time
		wantStr   string
		wantOK    bool
	}{
		{
			name:      "never cited",
			file:      "/a.md",
			lastCited: map[string]time.Time{},
			wantStr:   "never",
			wantOK:    true,
		},
		{
			name:      "cited before cutoff",
			file:      "/a.md",
			lastCited: map[string]time.Time{"/a.md": cutoff.Add(-24 * time.Hour)},
			wantStr:   "2025-12-31",
			wantOK:    true,
		},
		{
			name:      "cited after cutoff blocks eviction",
			file:      "/a.md",
			lastCited: map[string]time.Time{"/a.md": cutoff.Add(24 * time.Hour)},
			wantStr:   "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str, ok := evictionCitationStatus(tt.file, tt.lastCited, cutoff)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if str != tt.wantStr {
				t.Fatalf("str = %q, want %q", str, tt.wantStr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// maturity.go: floatValueFromData, nonEmptyStringFromData
// ---------------------------------------------------------------------------

func TestHelper2_floatValueFromData(t *testing.T) {
	data := map[string]any{
		"utility":    0.75,
		"not_float":  "hello",
		"confidence": 0.0,
	}

	if got := floatValueFromData(data, "utility", 0.5); got != 0.75 {
		t.Fatalf("expected 0.75, got %v", got)
	}
	if got := floatValueFromData(data, "not_float", 0.5); got != 0.5 {
		t.Fatalf("expected default 0.5 for non-float, got %v", got)
	}
	if got := floatValueFromData(data, "missing", 0.3); got != 0.3 {
		t.Fatalf("expected default 0.3 for missing key, got %v", got)
	}
	if got := floatValueFromData(data, "confidence", 0.5); got != 0.0 {
		t.Fatalf("expected 0.0, got %v", got)
	}
}

func TestHelper2_nonEmptyStringFromData(t *testing.T) {
	data := map[string]any{
		"maturity": "provisional",
		"empty":    "",
		"number":   42,
	}

	if got := nonEmptyStringFromData(data, "maturity", "default"); got != "provisional" {
		t.Fatalf("expected provisional, got %q", got)
	}
	if got := nonEmptyStringFromData(data, "empty", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback for empty string, got %q", got)
	}
	if got := nonEmptyStringFromData(data, "number", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback for non-string, got %q", got)
	}
	if got := nonEmptyStringFromData(data, "missing", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback for missing key, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// pool.go / flywheel_close_loop.go: isEligibleTier (promotionContext)
// ---------------------------------------------------------------------------

func TestHelper2_isEligibleTier(t *testing.T) {
	tests := []struct {
		name        string
		tier        types.Tier
		includeGold bool
		want        bool
	}{
		{"silver always eligible", types.TierSilver, false, true},
		{"silver with gold enabled", types.TierSilver, true, true},
		{"gold eligible when included", types.TierGold, true, true},
		{"gold not eligible when excluded", types.TierGold, false, false},
		{"bronze never eligible", types.TierBronze, true, false},
		{"discard never eligible", types.TierDiscard, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &promotionContext{includeGold: tt.includeGold}
			got := ctx.isEligibleTier(tt.tier)
			if got != tt.want {
				t.Fatalf("isEligibleTier(%q, includeGold=%v) = %v, want %v",
					tt.tier, tt.includeGold, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// pool.go: truncateID
// ---------------------------------------------------------------------------

func TestHelper2_truncateID(t *testing.T) {
	tests := []struct {
		id   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"exactlength", 11, "exactlength"},
		{"this-is-a-very-long-id", 10, "this-is..."},
		{"abc", 3, "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := truncateID(tt.id, tt.max)
			if got != tt.want {
				t.Fatalf("truncateID(%q, %d) = %q, want %q", tt.id, tt.max, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// pool.go: repeat
// ---------------------------------------------------------------------------

func TestHelper2_repeat(t *testing.T) {
	if got := repeat("=", 3); got != "===" {
		t.Fatalf("repeat('=', 3) = %q", got)
	}
	if got := repeat("ab", 2); got != "abab" {
		t.Fatalf("repeat('ab', 2) = %q", got)
	}
	if got := repeat("x", 0); got != "" {
		t.Fatalf("repeat('x', 0) = %q", got)
	}
}
