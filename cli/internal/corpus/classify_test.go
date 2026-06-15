package corpus

import (
	"strings"
	"testing"
)

func TestAnnotateLearning(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantChanged bool
		// substrings the output frontmatter must contain
		wantContains []string
		// substrings the output must NOT contain (e.g. duplicate keys)
		wantAbsent []string
	}{
		{
			name:         "fenced record missing both fields gets both defaults",
			in:           "---\ndate: 2026-06-14\nstatus: reviewed\n---\n\n# Learning\nbody\n",
			wantChanged:  true,
			wantContains: []string{"sensitivity: unknown", "publishable: false", "date: 2026-06-14"},
		},
		{
			name:        "record already carrying a real decision is left untouched",
			in:          "---\nsensitivity: public\npublishable: true\ndate: 2026-06-14\n---\nbody\n",
			wantChanged: false,
			// must not flip a real public decision back to the default
			wantAbsent: []string{"sensitivity: unknown", "publishable: false"},
		},
		{
			name:         "partial — only the missing field is added",
			in:           "---\nsensitivity: private\ndate: 2026-06-14\n---\nbody\n",
			wantChanged:  true,
			wantContains: []string{"sensitivity: private", "publishable: false"},
			wantAbsent:   []string{"sensitivity: unknown"},
		},
		{
			name:         "no frontmatter fence — minimal block is prepended",
			in:           "# Orchestration spike\n\nSource session: foo.\n",
			wantChanged:  true,
			wantContains: []string{"---\nsensitivity: unknown\npublishable: false\n---", "# Orchestration spike"},
		},
		{
			name:         "malformed — opening fence but no closing fence still lands keys in header zone",
			in:           "---\ndate: 2026-06-14\nbroken yaml here with no close fence\n",
			wantChanged:  true,
			wantContains: []string{"sensitivity: unknown", "publishable: false"},
		},
		{
			name:        "nested sensitivity key under a block does not count as present",
			in:          "---\ndate: 2026-06-14\nmeta:\n  sensitivity: high\n---\nbody\n",
			wantChanged: true,
			// the top-level field must still be added despite the nested one
			wantContains: []string{"date: 2026-06-14"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := AnnotateLearning(tt.in)
			if changed != tt.wantChanged {
				t.Fatalf("changed = %v, want %v\noutput:\n%s", changed, tt.wantChanged, got)
			}
			for _, sub := range tt.wantContains {
				if !strings.Contains(got, sub) {
					t.Errorf("output missing %q\ngot:\n%s", sub, got)
				}
			}
			for _, sub := range tt.wantAbsent {
				if strings.Contains(got, sub) {
					t.Errorf("output unexpectedly contains %q\ngot:\n%s", sub, got)
				}
			}
		})
	}
}

// TestAnnotateLearning_Idempotent guards the migration's re-run safety: applying
// twice must be a no-op the second time (round-trips the real first-pass output).
func TestAnnotateLearning_Idempotent(t *testing.T) {
	in := "---\ndate: 2026-06-14\n---\nbody\n"
	once, changed1 := AnnotateLearning(in)
	if !changed1 {
		t.Fatal("first pass should change an unclassified record")
	}
	twice, changed2 := AnnotateLearning(once)
	if changed2 {
		t.Errorf("second pass changed an already-classified record (not idempotent)\nfirst:\n%s", once)
	}
	if once != twice {
		t.Errorf("re-annotation altered content:\nfirst:\n%s\nsecond:\n%s", once, twice)
	}
	// exactly one of each key
	if n := strings.Count(twice, "sensitivity: "); n != 1 {
		t.Errorf("sensitivity key count = %d, want 1", n)
	}
	if n := strings.Count(twice, "publishable: "); n != 1 {
		t.Errorf("publishable key count = %d, want 1", n)
	}
}

func TestIsLearningFile(t *testing.T) {
	learnings := []string{"2026-06-14-foo.md", "research/bar.md"}
	meta := []string{"CORPUS-POLICY.md", "README.md", "MEMORY.md", "INDEX.md"}
	for _, f := range learnings {
		base := f
		if i := strings.LastIndexByte(f, '/'); i >= 0 {
			base = f[i+1:]
		}
		if !IsLearningFile(base) {
			t.Errorf("IsLearningFile(%q) = false, want true", base)
		}
	}
	for _, f := range meta {
		if IsLearningFile(f) {
			t.Errorf("IsLearningFile(%q) = true, want false (meta doc)", f)
		}
	}
}
