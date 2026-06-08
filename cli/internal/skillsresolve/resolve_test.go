package skillsresolve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, root, name, desc, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + desc + "\n---\n" + body
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasOverlap(r *Report, a, b string) bool {
	for _, o := range r.Overlaps {
		if (o.A == a && o.B == b) || (o.A == b && o.B == a) {
			return true
		}
	}
	return false
}

func TestResolve_ME_and_CE(t *testing.T) {
	root := t.TempDir()
	// Body padding clears the thin-bytes threshold WITHOUT polluting description
	// tokens (the tokenizer reads name + description only, never the body).
	body := strings.Repeat("lorem ipsum body content line.\n", 40)
	// Two near-duplicate skills (heavy shared description vocabulary) -> ME overlap.
	writeSkill(t, root, "fuzz-test-design", "design fuzz property randomized corpus tests and replay failures", body)
	writeSkill(t, root, "fuzz-test-harness", "design fuzz property randomized corpus tests and replay failures harness", body)
	// An unrelated skill -> must NOT overlap with the fuzz pair.
	writeSkill(t, root, "vercel-deploy", "deploy a nextjs application to the vercel hosting platform", body)
	// A thin, description-less skill -> CE coverage gap.
	writeSkill(t, root, "thin-one", "", "short")
	// A directory without SKILL.md -> ignored.
	if err := os.MkdirAll(filepath.Join(root, "_fixtures"), 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := Resolve(Options{SkillsDir: root})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if r.SkillsCount != 4 {
		t.Errorf("SkillsCount = %d, want 4 (_fixtures must be skipped)", r.SkillsCount)
	}
	if !hasOverlap(r, "fuzz-test-design", "fuzz-test-harness") {
		t.Errorf("expected ME overlap between the two fuzz skills; overlaps=%+v", r.Overlaps)
	}
	if hasOverlap(r, "fuzz-test-design", "vercel-deploy") {
		t.Error("vercel-deploy must not overlap the fuzz pair (false positive)")
	}
	gapped := false
	for _, g := range r.CoverageGaps {
		if g.Name == "thin-one" {
			gapped = true
			if g.HasDesc {
				t.Error("thin-one has no description; HasDesc should be false")
			}
		}
	}
	if !gapped {
		t.Errorf("expected thin-one in CE coverage flags; got %+v", r.CoverageGaps)
	}
	// Overlaps must be sorted by descending Jaccard.
	for i := 1; i < len(r.Overlaps); i++ {
		if r.Overlaps[i-1].Jaccard < r.Overlaps[i].Jaccard {
			t.Errorf("overlaps not sorted desc by jaccard at %d", i)
		}
	}
}

func TestResolve_MissingDir(t *testing.T) {
	if _, err := Resolve(Options{SkillsDir: filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Error("expected error for a missing skills dir")
	}
}
