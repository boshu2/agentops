package skillshealth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceChecksBehavioralBoundaries(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	target := filepath.Join(root, "skills", "sample")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	base := "---\nname: sample\ndescription: Inspect caller-selected Git status.\nskill_api_version: 1\nmetadata:\n  disposition: keep_specialist\n---\nRun git status --short in the selected repository; report paths inline and stop. On failure, report the error. Do not mutate.\n"
	for _, tc := range []struct{ name, content, code string }{
		{"concise", base, ""},
		{"equivalent heading", base + "\n## Returned answer\nReport the command result inline.\n", ""},
		{"identity", strings.Replace(base, "name: sample", "name: other", 1), "NAME_MISMATCH"},
		{"malformed", strings.Replace(base, "name: sample", "name: [broken", 1), "INVALID_FRONTMATTER"},
		{"resource", base + "[needed](references/missing.md)", "DEAD_REF"},
		{"scaffold", strings.Replace(base, "  disposition:", "  authoring_state: scaffold\n  disposition:", 1), "INCOMPLETE_SCAFFOLD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			got, err := CheckSource(root, []string{"skills/sample"}, false)
			if err != nil {
				t.Fatal(err)
			}
			if tc.code == "" && len(got) != 0 {
				t.Fatalf("unexpected findings: %v", got)
			}
			if tc.code != "" && (len(got) == 0 || got[0].Code != tc.code) {
				t.Fatalf("want %s got %v", tc.code, got)
			}
			after, _ := os.ReadFile(filepath.Join(target, "SKILL.md"))
			if string(after) != tc.content {
				t.Fatal("check mutated source")
			}
		})
	}
	for _, target := range []string{"skills/sample/../sample", "skills/missing"} {
		if _, err := CheckSource(root, []string{target}, false); err == nil {
			t.Fatalf("accepted %s", target)
		}
	}
	if err := os.Symlink(target, filepath.Join(root, "skills", "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckSource(root, []string{"skills/alias"}, false); err == nil {
		t.Fatal("accepted symlink")
	}
	if err := os.Mkdir(filepath.Join(target, "references"), 0755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(root, "external.md")
	if err := os.WriteFile(external, []byte("secret fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(target, "references", "outside.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte(base+"[file](references/outside.md)"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := CheckSource(root, []string{"skills/sample"}, false)
	if err != nil || len(got) != 1 || got[0].Code != "UNSAFE_REF" {
		t.Fatalf("symlink: %v %v", got, err)
	}
}
