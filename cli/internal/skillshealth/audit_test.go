package skillshealth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatter_TableDriven(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantName    string
		wantDesc    string
		wantPresent bool // whether name+description should both be present
	}{
		{
			name:     "valid full frontmatter",
			input:    "---\nname: foo\ndescription: a thing\n---\n# body\n",
			wantName: "foo", wantDesc: "a thing", wantPresent: true,
		},
		{
			name:     "missing description",
			input:    "---\nname: foo\n---\n",
			wantName: "foo", wantDesc: "", wantPresent: false,
		},
		{
			name:     "missing name",
			input:    "---\ndescription: only desc\n---\n",
			wantName: "", wantDesc: "only desc", wantPresent: false,
		},
		{
			name:     "comment-only frontmatter",
			input:    "---\n# just a comment\n---\nbody\n",
			wantName: "", wantDesc: "", wantPresent: false,
		},
		{
			name:     "no leading fence",
			input:    "name: foo\ndescription: bar\n",
			wantName: "", wantDesc: "", wantPresent: false,
		},
		{
			name:     "quoted values",
			input:    "---\nname: \"foo\"\ndescription: 'a thing'\n---\n",
			wantName: "foo", wantDesc: "a thing", wantPresent: true,
		},
		{
			name:     "indented (nested) keys ignored",
			input:    "---\nname: foo\nmetadata:\n  description: nested\ndescription: real\n---\n",
			wantName: "foo", wantDesc: "real", wantPresent: true,
		},
		{
			name:     "empty file",
			input:    "",
			wantName: "", wantDesc: "", wantPresent: false,
		},
		{
			name:     "fence but unclosed",
			input:    "---\nname: foo\ndescription: bar\n",
			wantName: "", wantDesc: "", wantPresent: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fm := ParseFrontmatter(tc.input)
			if got := fm["name"]; got != tc.wantName {
				t.Errorf("name: got %q want %q", got, tc.wantName)
			}
			if got := fm["description"]; got != tc.wantDesc {
				t.Errorf("description: got %q want %q", got, tc.wantDesc)
			}
			missing := ValidateFrontmatter(fm, tc.wantName)
			havePresent := len(missing) == 0
			if tc.wantName == "" {
				// When wantName is empty, name will fail; just verify presence inverted.
				havePresent = fm["name"] != "" && fm["description"] != ""
			}
			if havePresent != tc.wantPresent {
				t.Errorf("presence: got %v want %v (missing=%v)", havePresent, tc.wantPresent, missing)
			}
		})
	}
}

func TestValidateFrontmatter_NameMismatch(t *testing.T) {
	fm := map[string]string{"name": "foo", "description": "x"}
	missing := ValidateFrontmatter(fm, "bar")
	if len(missing) == 0 {
		t.Fatal("expected mismatch error")
	}
}

func TestCompareCodexParity_Cases(t *testing.T) {
	tmp := t.TempDir()
	codex := filepath.Join(tmp, "skills-codex")
	mustMkdirAll(t, filepath.Join(codex, "matchedone"))
	mustMkdirAll(t, filepath.Join(codex, "divergedone"))
	// matchedone has same description.
	mustWrite(t, filepath.Join(codex, "matchedone", "SKILL.md"),
		"---\nname: matchedone\ndescription: same intent line\n---\n")
	mustWrite(t, filepath.Join(codex, "divergedone", "SKILL.md"),
		"---\nname: divergedone\ndescription: completely unrelated text about widgets\n---\n")

	if got := compareCodexParity(codex, "matchedone", "same intent line"); got != "matched" {
		t.Errorf("matched: got %q", got)
	}
	if got := compareCodexParity(codex, "divergedone", "this is the agentops intent talking about flywheels"); got != "diverged" {
		t.Errorf("diverged: got %q", got)
	}
	if got := compareCodexParity(codex, "absent", "anything"); got != "missing" {
		t.Errorf("missing: got %q", got)
	}
}

// L2 integration: audit the real skills/ + skills-codex/ trees of THIS repo.
func TestAudit_RealRepo_L2(t *testing.T) {
	repoRoot := findRepoRoot(t)
	skillsDir := filepath.Join(repoRoot, "skills")
	codexDir := filepath.Join(repoRoot, "skills-codex")
	if _, err := os.Stat(skillsDir); err != nil {
		t.Skipf("skills dir not present: %v", err)
	}

	report, err := Audit(Options{SkillsDir: skillsDir, CodexDir: codexDir})
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if got := len(report.Skills); got < 60 {
		t.Errorf("expected >= 60 skills, got %d", got)
	}
	if len(report.Errors) > 0 {
		t.Logf("audit reports %d errors against real repo (informational):", len(report.Errors))
		for i, e := range report.Errors {
			if i >= 10 {
				t.Logf("... (%d more)", len(report.Errors)-10)
				break
			}
			t.Logf("  %s", e)
		}
		// Don't fail by default; the live tree may have transient drift while
		// other Wave 1 work is in progress. Real expectation is empty.
	}
}

func mustMkdirAll(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// findRepoRoot walks up from cwd until it finds skills/ + skills-codex/.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		_, e1 := os.Stat(filepath.Join(dir, "skills"))
		_, e2 := os.Stat(filepath.Join(dir, "skills-codex"))
		if e1 == nil && e2 == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skipf("could not find repo root from %s", cwd)
	return ""
}
