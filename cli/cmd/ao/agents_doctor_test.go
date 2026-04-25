package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
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
	if cmd.Flags().Lookup("json") == nil {
		t.Error("expected --json flag")
	}
}

func TestRunAgentsDoctor_JSONClean(t *testing.T) {
	repo := makeAgentsDoctorRepo(t, "ok", 0)
	origJSON := agentsDoctorJSON
	origProjectDir := testProjectDir
	t.Cleanup(func() {
		agentsDoctorJSON = origJSON
		testProjectDir = origProjectDir
	})
	agentsDoctorJSON = true
	testProjectDir = filepath.Join(repo, "cli")

	var stdout bytes.Buffer
	agentsDoctorCmd.SetOut(&stdout)
	t.Cleanup(func() { agentsDoctorCmd.SetOut(nil) })

	if err := runAgentsDoctor(agentsDoctorCmd, nil); err != nil {
		t.Fatalf("runAgentsDoctor: %v", err)
	}

	var got AgentsDoctorDiagnostics
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v\nGot: %s", err, stdout.String())
	}
	if got.LintStatus != "ok" {
		t.Errorf("LintStatus = %q, want ok", got.LintStatus)
	}
	if got.AllowlistCount != 1 {
		t.Errorf("AllowlistCount = %d, want 1", got.AllowlistCount)
	}
	if got.SkillOwnedCount != 1 {
		t.Errorf("SkillOwnedCount = %d, want 1", got.SkillOwnedCount)
	}
	if len(got.UnknownOnDiskDirs) != 0 {
		t.Errorf("UnknownOnDiskDirs = %v, want empty", got.UnknownOnDiskDirs)
	}
}

func TestRunAgentsDoctor_TextDriftExitCode(t *testing.T) {
	repo := makeAgentsDoctorRepo(t, "fail", 1)
	if err := os.MkdirAll(filepath.Join(repo, ".agents", "widgets"), 0o755); err != nil {
		t.Fatal(err)
	}

	origJSON := agentsDoctorJSON
	origProjectDir := testProjectDir
	t.Cleanup(func() {
		agentsDoctorJSON = origJSON
		testProjectDir = origProjectDir
	})
	agentsDoctorJSON = false
	testProjectDir = filepath.Join(repo, "cli")

	var stdout bytes.Buffer
	agentsDoctorCmd.SetOut(&stdout)
	t.Cleanup(func() { agentsDoctorCmd.SetOut(nil) })

	err := runAgentsDoctor(agentsDoctorCmd, nil)
	var doctorErr *AgentsDoctorError
	if !errors.As(err, &doctorErr) {
		t.Fatalf("expected *AgentsDoctorError, got %T: %v", err, err)
	}
	if doctorErr.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", doctorErr.ExitCode)
	}
	out := stdout.String()
	for _, want := range []string{
		"Contract:",
		"Catalogued surfaces: 1",
		"Skill-owned subdirs: 1",
		"Lint status: fail",
		"Unknown on-disk dirs: 1",
		"Next command: ao agents lint --json",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func makeAgentsDoctorRepo(t *testing.T, lintStatus string, lintExit int) string {
	t.Helper()
	repo := t.TempDir()
	if err := writeAgentsContract(filepath.Join(repo, defaultAgentsContract), []string{"ao"}); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{
		filepath.Join(repo, "cli"),
		filepath.Join(repo, "skills", "alpha"),
		filepath.Join(repo, ".agents", "ao"),
		filepath.Join(repo, ".agents", "alpha"),
		filepath.Join(repo, "scripts"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "skills", "alpha", "SKILL.md"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(repo, defaultAgentsLintScript)
	body := "#!/usr/bin/env bash\nprintf '{\"status\":\"" + lintStatus + "\",\"undocumented\":[]}'\nexit " + strconv.Itoa(lintExit) + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo
}
