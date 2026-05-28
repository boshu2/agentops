// practices: [design-by-contract, code-complete]
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSkillsCommandRegistered asserts the cobra registration is reachable.
func TestSkillsCommandRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"skills", "check"})
	if err != nil {
		t.Fatalf("skills check not reachable: %v", err)
	}
	if cmd == nil || cmd.Use != "check" {
		t.Fatalf("expected check subcommand, got %+v", cmd)
	}
	if cmd.Parent() == nil || cmd.Parent().Use != "skills" {
		t.Fatalf("expected parent skills, got %+v", cmd.Parent())
	}
	// Verify both flags exist.
	for _, f := range []string{"json", "strict", "skill"} {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("missing flag --%s", f)
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

	// Run check by chdir-ing into the synthetic root, since the cobra
	// command resolves "skills" relative to cwd.
	withCwd(t, tmp, func() {
		buf := runSkillsCheckCapture(t, []string{"--json"})
		var report struct {
			Skills      []map[string]any `json:"skills"`
			Errors      []string         `json:"errors"`
			ParityDrift []string         `json:"parity_drift"`
			Generated   string           `json:"generated_at"`
		}
		if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
			t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
		}
		if len(report.Skills) != 1 {
			t.Errorf("expected 1 skill, got %d", len(report.Skills))
		}
		if report.Generated == "" {
			t.Error("missing generated_at")
		}
		// Required keys per SkillStatus.
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
	// Frontmatter missing description.
	skillsTestWrite(t, filepath.Join(skillsDir, "broken", "SKILL.md"),
		"---\nname: broken\n---\nbody\n")

	withCwd(t, tmp, func() {
		err := invokeSkillsCheckCmd(t, []string{"--strict"})
		if err == nil {
			t.Fatal("expected non-nil error in --strict mode")
		}
	})
}

// helpers --------------------------------------------------------------------

func skillsTestWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func withCwd(t *testing.T, dir string, fn func()) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()
	fn()
}

func runSkillsCheckCapture(t *testing.T, args []string) *bytes.Buffer {
	t.Helper()
	resetSkillsCheckFlags(t)
	buf := &bytes.Buffer{}
	skillsCheckCmd.SetOut(buf)
	skillsCheckCmd.SetErr(buf)
	skillsCheckCmd.SetArgs(args)
	if err := skillsCheckCmd.ParseFlags(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := runSkillsCheck(skillsCheckCmd, args); err != nil {
		t.Fatalf("runSkillsCheck error: %v", err)
	}
	return buf
}

func invokeSkillsCheckCmd(t *testing.T, args []string) error {
	t.Helper()
	resetSkillsCheckFlags(t)
	buf := &bytes.Buffer{}
	skillsCheckCmd.SetOut(buf)
	skillsCheckCmd.SetErr(buf)
	skillsCheckCmd.SetArgs(args)
	if err := skillsCheckCmd.ParseFlags(args); err != nil {
		return err
	}
	return runSkillsCheck(skillsCheckCmd, args)
}

// resetSkillsCheckFlags zeroes the package-level flag vars between tests so
// state from one test doesn't leak into the next.
func resetSkillsCheckFlags(t *testing.T) {
	t.Helper()
	skillsCheckJSON = false
	skillsCheckStrict = false
	skillsCheckOnly = ""
	for _, name := range []string{"json", "strict", "skill"} {
		if f := skillsCheckCmd.Flags().Lookup(name); f != nil {
			f.Changed = false
			_ = f.Value.Set(f.DefValue)
		}
	}
}
