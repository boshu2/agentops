package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentsDoctorCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"agents", "doctor"})
	if err != nil {
		t.Fatalf("agents doctor command not registered: %v", err)
	}
	if cmd.Name() != "doctor" {
		t.Fatalf("found %q, want %q", cmd.Name(), "doctor")
	}
	for _, flag := range []string{"json", "strict", "contract", "script", "agents-dir", "skills-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected --%s flag", flag)
		}
	}
}

func TestFindOrphanSkills(t *testing.T) {
	tmp := t.TempDir()
	agentsDir := filepath.Join(tmp, ".agents")
	mkdir(t, agentsDir, "harvest")
	mkdir(t, agentsDir, "evolve")

	cases := []struct {
		name   string
		skills []string
		want   []string
	}{
		{
			name:   "all skills have .agents subdir",
			skills: []string{"harvest", "evolve"},
			want:   []string{},
		},
		{
			name:   "missing subdir is reported",
			skills: []string{"harvest", "evolve", "lonely"},
			want:   []string{"lonely"},
		},
		{
			name:   "result is sorted",
			skills: []string{"zeta", "alpha", "harvest"},
			want:   []string{"alpha", "zeta"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findOrphanSkills(tc.skills, agentsDir)
			if !equalStringSlices(got, tc.want) {
				t.Errorf("findOrphanSkills = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFindUndocumentedDirs(t *testing.T) {
	tmp := t.TempDir()
	agentsDir := filepath.Join(tmp, ".agents")
	mkdir(t, agentsDir, "learnings") // catalogued
	mkdir(t, agentsDir, "harvest")   // skill-owned
	mkdir(t, agentsDir, "stray-dir") // undocumented
	mkdir(t, agentsDir, ".git")      // hidden, must be skipped
	mkdir(t, agentsDir, "another-orphan")

	if err := os.WriteFile(filepath.Join(agentsDir, "loose-file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	allowlist := []string{"learnings", "research"}
	skills := []string{"harvest", "evolve"}

	got := findUndocumentedDirs(agentsDir, allowlist, skills)

	wantNames := []string{"another-orphan", "stray-dir"}
	gotNames := make([]string, 0, len(got))
	for _, d := range got {
		gotNames = append(gotNames, d.Name)
	}
	if !equalStringSlices(gotNames, wantNames) {
		t.Fatalf("undocumented names = %v, want %v", gotNames, wantNames)
	}
	for _, d := range got {
		if !strings.Contains(d.FixHint, d.Name) {
			t.Errorf("fix hint for %q does not name the dir: %q", d.Name, d.FixHint)
		}
		if !strings.Contains(d.FixHint, "agents-write-surfaces.md") {
			t.Errorf("fix hint for %q does not point at the contract: %q", d.Name, d.FixHint)
		}
	}
}

func TestFindUndocumentedDirs_MissingAgentsDir(t *testing.T) {
	got := findUndocumentedDirs(filepath.Join(t.TempDir(), "nope"), nil, nil)
	if got != nil {
		t.Errorf("expected nil for missing agents dir, got %v", got)
	}
}

func TestRunAgentsDoctor_TextOutput(t *testing.T) {
	tmp := t.TempDir()
	contract := filepath.Join(tmp, "contract.md")
	contractBody := strings.Join([]string{
		"<!-- BEGIN agents-write-surfaces-allowlist -->",
		"learnings",
		"research",
		"<!-- END agents-write-surfaces-allowlist -->",
	}, "\n")
	if err := os.WriteFile(contract, []byte(contractBody), 0o644); err != nil {
		t.Fatal(err)
	}

	skillsDir := filepath.Join(tmp, "skills")
	for _, name := range []string{"harvest", "lonely"} {
		mkdir(t, skillsDir, name)
		if err := os.WriteFile(filepath.Join(skillsDir, name, "SKILL.md"), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	agentsDir := filepath.Join(tmp, ".agents")
	mkdir(t, agentsDir, "harvest") // skill present, has subdir
	mkdir(t, agentsDir, "stray")   // undocumented
	// "lonely" skill has no .agents/lonely/ subdir → orphan

	scriptPath := filepath.Join(tmp, "fake-lint.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	withDoctorFlags(t, contract, scriptPath, agentsDir, skillsDir, false, false)

	var stdout bytes.Buffer
	agentsDoctorCmd.SetOut(&stdout)
	t.Cleanup(func() { agentsDoctorCmd.SetOut(nil) })

	if err := runAgentsDoctor(agentsDoctorCmd, nil); err != nil {
		t.Fatalf("doctor unexpected error: %v", err)
	}

	out := stdout.String()
	wantSubstrings := []string{
		"ao agents doctor",
		"Catalogued surfaces: 2",
		"Skill-owned subdirs: 2",
		"Lint: PASS",
		"Orphan skills (skills/<name>/ with no .agents/<name>/): 1",
		"lonely",
		"Undocumented dirs (.agents/<name>/ not catalogued, not a skill): 1",
		"stray",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestRunAgentsDoctor_JSON(t *testing.T) {
	tmp := t.TempDir()
	contract := filepath.Join(tmp, "contract.md")
	if err := os.WriteFile(contract, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	mkdir(t, tmp, "skills")
	mkdir(t, tmp, ".agents")
	mkdir(t, tmp, ".agents", "stray")

	scriptPath := filepath.Join(tmp, "ok.sh")
	script := strings.Join([]string{
		"#!/usr/bin/env bash",
		"if [ \"${1:-}\" = \"--json\" ]; then",
		"  echo '{\"from_lint\":true}'",
		"else",
		"  echo 'lint text'",
		"fi",
		"exit 0",
		"",
	}, "\n")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	withDoctorFlags(t, contract, scriptPath,
		filepath.Join(tmp, ".agents"),
		filepath.Join(tmp, "skills"),
		true,  // json
		false, // strict
	)

	var stdout bytes.Buffer
	agentsDoctorCmd.SetOut(&stdout)
	t.Cleanup(func() { agentsDoctorCmd.SetOut(nil) })

	if err := runAgentsDoctor(agentsDoctorCmd, nil); err != nil {
		t.Fatalf("doctor unexpected error: %v", err)
	}

	var report AgentsDoctorReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\nbytes: %s", err, stdout.String())
	}
	if strings.Contains(stdout.String(), "from_lint") {
		t.Fatalf("doctor JSON included nested lint JSON: %s", stdout.String())
	}
	if !report.LintClean {
		t.Error("expected LintClean=true")
	}
	if report.LintExitCode != 0 {
		t.Errorf("LintExitCode = %d, want 0", report.LintExitCode)
	}
	if len(report.UndocumentedDirs) != 1 || report.UndocumentedDirs[0].Name != "stray" {
		t.Errorf("UndocumentedDirs = %#v, want one stray entry", report.UndocumentedDirs)
	}
}

func TestRunAgentsDoctor_LintFailureExitsNonZero(t *testing.T) {
	tmp := t.TempDir()
	contract := filepath.Join(tmp, "contract.md")
	if err := os.WriteFile(contract, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	mkdir(t, tmp, "skills")
	mkdir(t, tmp, ".agents")

	scriptPath := filepath.Join(tmp, "fail.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/usr/bin/env bash\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	withDoctorFlags(t, contract, scriptPath,
		filepath.Join(tmp, ".agents"),
		filepath.Join(tmp, "skills"),
		false, false,
	)

	var stdout bytes.Buffer
	agentsDoctorCmd.SetOut(&stdout)
	agentsDoctorCmd.SetErr(&stdout)
	t.Cleanup(func() {
		agentsDoctorCmd.SetOut(nil)
		agentsDoctorCmd.SetErr(nil)
	})

	err := runAgentsDoctor(agentsDoctorCmd, nil)
	if err == nil {
		t.Fatal("expected error from failing lint script")
	}
	var lintErr *AgentsLintError
	if !errors.As(err, &lintErr) {
		t.Fatalf("expected AgentsLintError, got %T: %v", err, err)
	}
	if lintErr.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", lintErr.ExitCode)
	}
}

func TestRunAgentsDoctor_StrictExitsOnOrphans(t *testing.T) {
	tmp := t.TempDir()
	contract := filepath.Join(tmp, "contract.md")
	if err := os.WriteFile(contract, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	skillsDir := filepath.Join(tmp, "skills")
	mkdir(t, skillsDir, "missing-subdir")
	if err := os.WriteFile(filepath.Join(skillsDir, "missing-subdir", "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mkdir(t, tmp, ".agents")

	scriptPath := filepath.Join(tmp, "ok.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	withDoctorFlags(t, contract, scriptPath,
		filepath.Join(tmp, ".agents"),
		skillsDir,
		false, // text
		true,  // strict
	)

	var stdout bytes.Buffer
	agentsDoctorCmd.SetOut(&stdout)
	t.Cleanup(func() { agentsDoctorCmd.SetOut(nil) })

	err := runAgentsDoctor(agentsDoctorCmd, nil)
	if err == nil {
		t.Fatal("strict mode should fail when orphan skills exist")
	}
	if !strings.Contains(err.Error(), "strict") || !strings.Contains(err.Error(), "orphan") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- helpers ---

func withDoctorFlags(t *testing.T, contract, script, agentsDir, skillsDir string, jsonOut, strict bool) {
	t.Helper()
	prevContract := agentsDoctorContract
	prevScript := agentsDoctorScript
	prevAgents := agentsDoctorAgentsDir
	prevSkills := agentsDoctorSkillsDir
	prevJSON := agentsDoctorJSON
	prevStrict := agentsDoctorStrict
	t.Cleanup(func() {
		agentsDoctorContract = prevContract
		agentsDoctorScript = prevScript
		agentsDoctorAgentsDir = prevAgents
		agentsDoctorSkillsDir = prevSkills
		agentsDoctorJSON = prevJSON
		agentsDoctorStrict = prevStrict
	})
	agentsDoctorContract = contract
	agentsDoctorScript = script
	agentsDoctorAgentsDir = agentsDir
	agentsDoctorSkillsDir = skillsDir
	agentsDoctorJSON = jsonOut
	agentsDoctorStrict = strict
}

func mkdir(t *testing.T, parts ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(parts...), 0o755); err != nil {
		t.Fatal(err)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
