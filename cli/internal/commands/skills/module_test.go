// practices: [design-by-contract, code-complete]
package skills

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// execSkills constructs a fresh skills module (clean flag state) and runs the
// command tree with the given args, returning captured stdout and stderr
// separately so callers can assert the data/diagnostics contract. It replaces
// the former cmd/ao command-capture harness for this carved family.
func execSkills(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := NewModule(HostOptions{DryRun: func() bool { return false }}).Command()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errb.String(), err
}

func skillsTestWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func withCwd(t *testing.T, dir string, fn func()) {
	t.Helper()
	t.Chdir(dir)
	fn()
}

// TestSkillsCommandTreeRegistered asserts the module builds the full subcommand
// tree with the documented flags.
func TestSkillsCommandTreeRegistered(t *testing.T) {
	root := NewModule(HostOptions{DryRun: func() bool { return false }}).Command()
	if root.Name() != "skills" {
		t.Fatalf("root command = %q, want skills", root.Name())
	}
	want := map[string]bool{
		"check": true, "resolve": true, "find": true, "list": true,
		"consumers": true, "producers": true, "graph": true, "link": true, "unlink": true,
	}
	got := map[string]bool{}
	for _, c := range root.Commands() {
		got[c.Name()] = true
	}
	if len(got) != len(want) {
		t.Errorf("subcommand set = %v, want %v", got, want)
	}
	for name := range want {
		if !got[name] {
			t.Errorf("missing subcommand %q", name)
		}
	}

	check, _, err := root.Find([]string{"check"})
	if err != nil || check == nil || check.Use != "check" {
		t.Fatalf("check not reachable: %v (%+v)", err, check)
	}
	for _, f := range []string{"json", "strict", "skill"} {
		if check.Flags().Lookup(f) == nil {
			t.Errorf("check missing flag --%s", f)
		}
	}
	resolve, _, err := root.Find([]string{"resolve"})
	if err != nil || resolve == nil || resolve.Use != "resolve" {
		t.Fatalf("resolve not reachable: %v (%+v)", err, resolve)
	}
	for _, f := range []string{"json", "strict"} {
		if resolve.Flags().Lookup(f) == nil {
			t.Errorf("resolve missing flag --%s", f)
		}
	}
}

// TestSkillsCheck_JSONOutputSchema runs the command against a tiny synthetic
// skills tree and asserts the JSON output schema.
func TestSkillsCheck_JSONOutputSchema(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, "skills")
	codexDir := filepath.Join(tmp, "skills-codex")
	if err := os.MkdirAll(filepath.Join(skillsDir, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(codexDir, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	skillsTestWrite(t, filepath.Join(skillsDir, "alpha", "SKILL.md"),
		"---\nname: alpha\ndescription: alpha skill\n---\nbody\n")
	skillsTestWrite(t, filepath.Join(codexDir, "alpha", "SKILL.md"),
		"---\nname: alpha\ndescription: alpha skill\n---\n")

	// Run check by chdir-ing into the synthetic root; the module resolves
	// "skills" relative to cwd.
	withCwd(t, tmp, func() {
		stdout, _, err := execSkills(t, "check", "--json")
		if err != nil {
			t.Fatalf("check --json: %v", err)
		}
		var report struct {
			Skills      []map[string]any `json:"skills"`
			Errors      []string         `json:"errors"`
			ParityDrift []string         `json:"parity_drift"`
			Generated   string           `json:"generated_at"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
		}
		if len(report.Skills) != 1 {
			t.Errorf("expected 1 skill, got %d", len(report.Skills))
		}
		if report.Generated == "" {
			t.Error("missing generated_at")
		}
		got := report.Skills[0]
		for _, k := range []string{"name", "path", "frontmatter_valid", "codex_parity"} {
			if _, ok := got[k]; !ok {
				t.Errorf("missing key %q in skill status: %v", k, got)
			}
		}
		if v, _ := got["name"].(string); v != "alpha" {
			t.Errorf("name: got %q", v)
		}
		if v, _ := got["frontmatter_valid"].(bool); !v {
			t.Error("expected frontmatter_valid=true")
		}
	})
}

// TestSkillsCheck_StrictExitsNonZeroOnMissingFrontmatter ensures --strict
// returns an error when frontmatter is missing.
func TestSkillsCheck_StrictExitsNonZeroOnMissingFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, "skills")
	codexDir := filepath.Join(tmp, "skills-codex")
	if err := os.MkdirAll(filepath.Join(skillsDir, "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillsTestWrite(t, filepath.Join(skillsDir, "broken", "SKILL.md"),
		"---\nname: broken\n---\nbody\n")

	withCwd(t, tmp, func() {
		if _, _, err := execSkills(t, "check", "--strict"); err == nil {
			t.Fatal("expected non-nil error in --strict mode")
		}
	})
}

// skillsResolveSyntheticTree builds skills/ + skills-codex/ (both required by
// ResolveSkillsRoots) holding two name-family skills with near-identical
// descriptions, guaranteeing at least one ME overlap candidate.
func skillsResolveSyntheticTree(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, "skills")
	codexDir := filepath.Join(tmp, "skills-codex")
	for _, name := range []string{"alpha-one", "alpha-two"} {
		if err := os.MkdirAll(filepath.Join(skillsDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	desc := "audit overlapping skills and flag merge candidates for the corpus resolver"
	body := "\n# heading\n\nA sufficiently long body so the skill is not flagged as a thin coverage gap. " +
		"It describes auditing overlapping skills and flagging merge candidates for the corpus resolver in detail.\n"
	skillsTestWrite(t, filepath.Join(skillsDir, "alpha-one", "SKILL.md"),
		"---\nname: alpha-one\ndescription: "+desc+"\n---"+body)
	skillsTestWrite(t, filepath.Join(skillsDir, "alpha-two", "SKILL.md"),
		"---\nname: alpha-two\ndescription: "+desc+"\n---"+body)
	return tmp
}

// TestSkillsResolve_JSONSchema runs resolve against a synthetic skills tree with
// two name-family overlapping skills and asserts the MECE report schema.
func TestSkillsResolve_JSONSchema(t *testing.T) {
	tmp := skillsResolveSyntheticTree(t)

	withCwd(t, tmp, func() {
		stdout, _, err := execSkills(t, "resolve", "--json")
		if err != nil {
			t.Fatalf("resolve --json: %v", err)
		}
		var report struct {
			Generated    string           `json:"generated"`
			SkillsCount  int              `json:"skills_count"`
			Overlaps     []map[string]any `json:"me_candidate_overlaps"`
			CoverageGaps []map[string]any `json:"ce_coverage_flags"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
		}
		if report.Generated == "" {
			t.Error("missing generated")
		}
		if report.SkillsCount < 2 {
			t.Errorf("expected >=2 skills, got %d", report.SkillsCount)
		}
		if len(report.Overlaps) == 0 {
			t.Fatalf("expected at least one ME overlap candidate, got none\noutput: %s", stdout)
		}
		got := report.Overlaps[0]
		for _, k := range []string{"a", "b", "jaccard", "shared_stem"} {
			if _, ok := got[k]; !ok {
				t.Errorf("missing key %q in overlap: %v", k, got)
			}
		}
	})
}

// TestSkillsResolve_StrictExitsNonZeroOnOverlap ensures --strict returns an
// error when ME overlap candidates exist (the CI dedup gate contract).
func TestSkillsResolve_StrictExitsNonZeroOnOverlap(t *testing.T) {
	tmp := skillsResolveSyntheticTree(t)

	withCwd(t, tmp, func() {
		if _, _, err := execSkills(t, "resolve", "--strict"); err == nil {
			t.Fatal("expected non-nil error in --strict mode with overlaps present")
		}
	})
}
