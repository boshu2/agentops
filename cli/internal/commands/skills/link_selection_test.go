package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/skillsapp"
)

func selectionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"skills/alpha", "skills/beta", "skills/not-a-skill", "skills-codex"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"registry.json", "PRODUCT.md", "skills/alpha/SKILL.md", "skills/beta/SKILL.md"} {
		skillsTestWrite(t, filepath.Join(root, file), "fixture\n")
	}
	t.Chdir(root)
	return root
}

// Given a source checkout, selecting a specialist installs only that name,
// while the existing command without selectors still installs the full corpus.
func TestLinkSelectionInstallsOnlyNamedSkillsAndPreservesFullInstall(t *testing.T) {
	root := selectionFixture(t)
	dest := t.TempDir()
	for iteration := 0; iteration < 2; iteration++ {
		stdout, _, err := execSkills(t, "link", "--dest", dest, "--skill", "beta", "--skill", "beta", "--json")
		if err != nil {
			t.Fatalf("selected link: %v\n%s", err, stdout)
		}
		var results []skillsapp.LinkResult
		if err := json.Unmarshal([]byte(stdout), &results); err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Fatalf("results = %+v", results)
		}
		names := results[0].Linked
		if iteration == 1 {
			names = results[0].Present
			if len(results[0].Linked) != 0 {
				t.Fatalf("repeat linked again: %+v", results)
			}
		}
		if !reflect.DeepEqual(names, []string{"beta"}) {
			t.Fatalf("selected names = %v, want [beta]", names)
		}
	}
	if got, err := os.Readlink(filepath.Join(dest, "beta")); err != nil || got != filepath.Join(root, "skills", "beta") {
		t.Fatalf("selected target = %q, err %v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(dest, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("unselected skill installed: %v", err)
	}
	stdout, _, err := execSkills(t, "link", "--dest", dest, "--skill", "beta")
	if err != nil || !strings.Contains(stdout, "all requested skills") || strings.Contains(stdout, "all repo skills") {
		t.Fatalf("selected repeat claimed full corpus: %v\n%s", err, stdout)
	}
	stdout, _, err = execSkills(t, "link", "--dest", dest, "--json")
	if err != nil {
		t.Fatalf("full install: %v\n%s", err, stdout)
	}
	if _, err := os.Stat(filepath.Join(dest, "alpha", "SKILL.md")); err != nil {
		t.Fatalf("full install lost alpha: %v", err)
	}
}

func TestLinkSelectionAcceptsMultipleDistinctNames(t *testing.T) {
	selectionFixture(t)
	dest := t.TempDir()
	stdout, _, err := execSkills(t, "link", "--dest", dest, "--skill", "beta", "--skill", "alpha", "--json")
	if err != nil {
		t.Fatalf("multiple selected names: %v\n%s", err, stdout)
	}
	var results []skillsapp.LinkResult
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !reflect.DeepEqual(results[0].Linked, []string{"alpha", "beta"}) {
		t.Fatalf("multiple selected names = %+v", results)
	}
}

// The whole selection must be checked before even the first valid name can
// create a destination. Invalid names must not silently select the full corpus.
func TestLinkSelectionRejectsInvalidNamesBeforeAnyWrite(t *testing.T) {
	for _, name := range []string{"missing", "", " ", ".", "..", "../alpha", "alpha/beta", `alpha\beta`, "/alpha", "alpha,beta", "not-a-skill"} {
		t.Run(name, func(t *testing.T) {
			selectionFixture(t)
			dest := filepath.Join(t.TempDir(), "uncreated")
			stdout, stderr, err := execSkills(t, "link", "--dest", dest, "--skill", "alpha", "--skill", name, "--json")
			if err == nil {
				t.Fatalf("invalid selection %q succeeded: %s", name, stdout)
			}
			if !strings.Contains(err.Error()+stdout+stderr, "skill") {
				t.Fatalf("selection error does not explain correction: %v", err)
			}
			if _, err := os.Lstat(dest); !os.IsNotExist(err) {
				t.Fatalf("invalid selection wrote destination: %v", err)
			}
		})
	}
}
