package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSkillMeta_NameDescriptionAndBlockTriggers(t *testing.T) {
	content := `---
name: evolve
description: "Run autonomous improvement loops."
metadata:
  triggers:
    - autonomous loop
    - keep improving
practices:
- lean-startup
---

# body
`
	meta := parseSkillMeta(content)
	if meta.Name != "evolve" {
		t.Errorf("name: got %q want evolve", meta.Name)
	}
	if meta.Description != "Run autonomous improvement loops." {
		t.Errorf("description: got %q", meta.Description)
	}
	if len(meta.Triggers) != 2 || meta.Triggers[0] != "autonomous loop" || meta.Triggers[1] != "keep improving" {
		t.Errorf("triggers: got %v want [autonomous loop, keep improving]", meta.Triggers)
	}
}

func TestParseSkillMeta_InlineTriggers(t *testing.T) {
	content := "---\nname: foo\ndescription: bar\ntriggers: [alpha, \"beta gamma\"]\n---\n"
	meta := parseSkillMeta(content)
	if len(meta.Triggers) != 2 || meta.Triggers[0] != "alpha" || meta.Triggers[1] != "beta gamma" {
		t.Errorf("inline triggers: got %v", meta.Triggers)
	}
}

func TestParseSkillMeta_NoFrontmatter(t *testing.T) {
	meta := parseSkillMeta("# just a heading\nno frontmatter here\n")
	if meta.Name != "" || meta.Description != "" || len(meta.Triggers) != 0 {
		t.Errorf("expected empty meta for frontmatter-less content, got %+v", meta)
	}
}

func TestLoad_TempTreeFallsBackToDirName(t *testing.T) {
	dir := t.TempDir()
	// A skill whose frontmatter omits name: should fall back to the dir name.
	mk := func(name, body string) {
		sd := filepath.Join(dir, name)
		if err := os.MkdirAll(sd, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sd, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("named", "---\nname: named\ndescription: a described skill\n---\n")
	mk("anon", "---\ndescription: no name field\n---\n")
	// A directory without SKILL.md must be skipped, not error.
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	metas, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 skills (empty dir skipped), got %d: %+v", len(metas), metas)
	}
	// Sorted by name: "anon" (fallback) before "named".
	if metas[0].Name != "anon" {
		t.Errorf("expected dir-name fallback 'anon' first, got %q", metas[0].Name)
	}
	if metas[1].Name != "named" || metas[1].Description != "a described skill" {
		t.Errorf("named skill parsed wrong: %+v", metas[1])
	}
}

func TestLoad_MissingDirErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Error("expected error for missing skills dir, got nil")
	}
}

// TestLoad_LiveTreeNonEmpty asserts the loader reads the real skills/ tree when
// run from the repo. Per the pre-mortem, it does NOT assert exact skill names
// (the tree churns) — only that loading succeeds and finds skills.
func TestLoad_LiveTreeNonEmpty(t *testing.T) {
	root := repoSkillsDir(t)
	if root == "" {
		t.Skip("skills/ not found relative to test working dir")
	}
	metas, err := Load(root)
	if err != nil {
		t.Fatalf("Load(%s): %v", root, err)
	}
	if len(metas) == 0 {
		t.Errorf("expected the live skills/ tree to yield >0 skills, got 0")
	}
	for _, m := range metas {
		if m.Name == "" {
			t.Errorf("skill at %s has empty name", m.Path)
		}
	}
}

// repoSkillsDir walks up from the test working directory to locate skills/.
func repoSkillsDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for i := 0; i < 8; i++ {
		cand := filepath.Join(dir, "skills")
		// Require a skills-codex sibling to disambiguate the repo-root skills/
		// tree from this Go package directory (cli/internal/skills).
		if isDirTest(cand) && isDirTest(filepath.Join(dir, "skills-codex")) {
			return cand
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func isDirTest(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
