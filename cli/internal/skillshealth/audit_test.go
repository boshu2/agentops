package skillshealth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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

func TestFindBrokenRefs_RelativeTargets(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		files []string
		want  []string
	}{
		{name: "local", body: "[local](references/local.md)", files: []string{"references/local.md"}},
		{name: "dot local", body: "[local](./references/local.md)", files: []string{"references/local.md"}},
		{name: "nested local", body: "[local](references/nested/local.md)", files: []string{"references/nested/local.md"}},
		{name: "sibling", body: "[shared](../agent-native/references/shared.md)", files: []string{"../agent-native/references/shared.md"}},
		{name: "angle sibling", body: "[shared](<../agent-native/references/shared.md>)", files: []string{"../agent-native/references/shared.md"}},
		{name: "normalized local", body: "[local](../current/references/local.md)", files: []string{"references/local.md"}},
		{name: "existing bare repo path", body: "grep text skills/current/references/local.md", files: []string{"references/local.md"}},
		{name: "missing local", body: "[missing](references/missing.md)", want: []string{"references/missing.md (linked but missing on disk)"}},
		{name: "missing sibling", body: "[missing](../agent-native/references/missing.md)", want: []string{"../agent-native/references/missing.md (linked but missing on disk)"}},
		{
			name:  "sibling does not link same-named local file",
			body:  "[shared](../agent-native/references/shared.md)",
			files: []string{"../agent-native/references/shared.md", "references/shared.md"},
			want:  []string{"references/shared.md (on disk but unlinked)"},
		},
		{
			name:  "local file does not satisfy missing sibling",
			body:  "[shared](../agent-native/references/shared.md)",
			files: []string{"references/shared.md"},
			want:  []string{"../agent-native/references/shared.md (linked but missing on disk)", "references/shared.md (on disk but unlinked)"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			skillDir := filepath.Join(t.TempDir(), "skills", "current")
			mustMkdirAll(t, skillDir)
			for _, ref := range tc.files {
				p := filepath.Join(skillDir, ref)
				mustMkdirAll(t, filepath.Dir(p))
				mustWrite(t, p, "reference\n")
			}
			if got := findBrokenRefs(skillDir, tc.body); !slices.Equal(got, tc.want) {
				t.Errorf("broken refs: got %q want %q", got, tc.want)
			}
		})
	}
}

// L2 integration: audit the real skills/ tree of THIS repo.
func TestAudit_RealRepo_L2(t *testing.T) {
	repoRoot := findRepoRoot(t)
	skillsDir := filepath.Join(repoRoot, "skills")
	report, err := Audit(Options{SkillsDir: skillsDir})
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	catalogRaw, err := os.ReadFile(filepath.Join(skillsDir, "catalog.json"))
	if err != nil {
		t.Fatalf("read generated catalog: %v", err)
	}
	var catalog struct {
		Skills []json.RawMessage `json:"skills"`
	}
	if err := json.Unmarshal(catalogRaw, &catalog); err != nil {
		t.Fatalf("parse generated catalog: %v", err)
	}
	if got, want := len(report.Skills), len(catalog.Skills); got != want {
		t.Errorf("audit/catalog skill count mismatch: got %d want %d", got, want)
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
		t.Fail()
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

// findRepoRoot walks up from cwd until it finds the agentops repo root
// (skills/ beside registry.json). It fails rather than skips: a silent skip
// would turn the L2 audit into a false green.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		_, e1 := os.Stat(filepath.Join(dir, "skills"))
		_, e2 := os.Stat(filepath.Join(dir, "registry.json"))
		if e1 == nil && e2 == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find repo root from %s", cwd)
	return ""
}
