// practices: [design-by-contract, code-complete]
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSkillsResolveRegistered asserts the cobra registration is reachable and
// carries the documented flags.
func TestSkillsResolveRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"skills", "resolve"})
	if err != nil {
		t.Fatalf("skills resolve not reachable: %v", err)
	}
	if cmd == nil || cmd.Use != "resolve" {
		t.Fatalf("expected resolve subcommand, got %+v", cmd)
	}
	if cmd.Parent() == nil || cmd.Parent().Use != "skills" {
		t.Fatalf("expected parent skills, got %+v", cmd.Parent())
	}
	for _, f := range []string{"json", "strict"} {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("missing flag --%s", f)
		}
	}
}

// TestSkillsResolve_JSONSchema runs resolve against a synthetic skills tree with
// two name-family overlapping skills and asserts the MECE report schema.
func TestSkillsResolve_JSONSchema(t *testing.T) {
	tmp := skillsResolveSyntheticTree(t)

	withCwd(t, tmp, func() {
		buf := runSkillsResolveCapture(t, []string{"--json"})
		var report struct {
			Generated    string           `json:"generated"`
			SkillsCount  int              `json:"skills_count"`
			Overlaps     []map[string]any `json:"me_candidate_overlaps"`
			CoverageGaps []map[string]any `json:"ce_coverage_flags"`
		}
		if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
			t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
		}
		if report.Generated == "" {
			t.Error("missing generated")
		}
		if report.SkillsCount < 2 {
			t.Errorf("expected >=2 skills, got %d", report.SkillsCount)
		}
		if len(report.Overlaps) == 0 {
			t.Fatalf("expected at least one ME overlap candidate, got none\noutput: %s", buf.String())
		}
		// Required keys per Overlap.
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
		err := invokeSkillsResolveCmd(t, []string{"--strict"})
		if err == nil {
			t.Fatal("expected non-nil error in --strict mode with overlaps present")
		}
	})
}

// helpers --------------------------------------------------------------------

// skillsResolveSyntheticTree builds skills/ + skills-codex/ (both required by
// resolveSkillsRoots) holding two name-family skills with near-identical
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

func runSkillsResolveCapture(t *testing.T, args []string) *bytes.Buffer {
	t.Helper()
	resetSkillsResolveFlags(t)
	buf := &bytes.Buffer{}
	skillsResolveCmd.SetOut(buf)
	skillsResolveCmd.SetErr(buf)
	skillsResolveCmd.SetArgs(args)
	if err := skillsResolveCmd.ParseFlags(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := runSkillsResolve(skillsResolveCmd, args); err != nil {
		t.Fatalf("runSkillsResolve error: %v", err)
	}
	return buf
}

func invokeSkillsResolveCmd(t *testing.T, args []string) error {
	t.Helper()
	resetSkillsResolveFlags(t)
	buf := &bytes.Buffer{}
	skillsResolveCmd.SetOut(buf)
	skillsResolveCmd.SetErr(buf)
	skillsResolveCmd.SetArgs(args)
	if err := skillsResolveCmd.ParseFlags(args); err != nil {
		return err
	}
	return runSkillsResolve(skillsResolveCmd, args)
}

// resetSkillsResolveFlags zeroes the package-level flag vars between tests so
// state from one test doesn't leak into the next.
func resetSkillsResolveFlags(t *testing.T) {
	t.Helper()
	skillsResolveJSON = false
	skillsResolveStrict = false
	for _, name := range []string{"json", "strict"} {
		if f := skillsResolveCmd.Flags().Lookup(name); f != nil {
			f.Changed = false
			_ = f.Value.Set(f.DefValue)
		}
	}
}
