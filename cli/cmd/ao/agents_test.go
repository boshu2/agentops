// practices: [wiki-knowledge-surface, design-by-contract]
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAgentsCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"agents"})
	if err != nil {
		t.Fatalf("agents command not registered: %v", err)
	}
	if cmd.Name() != "agents" {
		t.Fatalf("found %q, want %q", cmd.Name(), "agents")
	}
}

func TestAgentsInspectCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"agents", "inspect"})
	if err != nil {
		t.Fatalf("agents inspect command not registered: %v", err)
	}
	if cmd.Name() != "inspect" {
		t.Fatalf("found %q, want %q", cmd.Name(), "inspect")
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Error("expected --json flag")
	}
	if cmd.Flags().Lookup("contract") == nil {
		t.Error("expected --contract flag")
	}
}

func TestAgentsLintCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"agents", "lint"})
	if err != nil {
		t.Fatalf("agents lint command not registered: %v", err)
	}
	if cmd.Name() != "lint" {
		t.Fatalf("found %q, want %q", cmd.Name(), "lint")
	}
	if cmd.Flags().Lookup("script") == nil {
		t.Error("expected --script flag")
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Error("expected --json flag")
	}
}

func TestRunAgentsInspect_DefaultPathsResolveFromSubdir(t *testing.T) {
	repo := t.TempDir()
	if err := writeAgentsContract(filepath.Join(repo, defaultAgentsContract), []string{"ao", "patterns"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(repo, "skills", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("ok"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cliDir := filepath.Join(repo, "cli")
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		t.Fatal(err)
	}

	origJSON := agentsInspectJSON
	origContract := agentsInspectContract
	origProjectDir := testProjectDir
	t.Cleanup(func() {
		agentsInspectJSON = origJSON
		agentsInspectContract = origContract
		testProjectDir = origProjectDir
	})
	agentsInspectJSON = true
	agentsInspectContract = defaultAgentsContract
	testProjectDir = cliDir

	var buf bytes.Buffer
	agentsInspectCmd.SetOut(&buf)
	t.Cleanup(func() { agentsInspectCmd.SetOut(nil) })

	if err := runAgentsInspect(agentsInspectCmd, nil); err != nil {
		t.Fatalf("runAgentsInspect: %v", err)
	}

	var got AgentsInventory
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\nGot: %s", err, buf.String())
	}
	if got.Contract != filepath.Join(repo, defaultAgentsContract) {
		t.Errorf("Contract = %q, want repo-root path", got.Contract)
	}
	wantSkills := []string{"alpha", "beta"}
	if !reflect.DeepEqual(got.Skills, wantSkills) {
		t.Errorf("Skills = %v, want %v", got.Skills, wantSkills)
	}
}

func TestRunAgentsLint_DefaultScriptResolvesFromSubdir(t *testing.T) {
	repo := t.TempDir()
	if err := writeAgentsContract(filepath.Join(repo, defaultAgentsContract), []string{"ao"}); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(repo, defaultAgentsLintScript)
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/usr/bin/env bash\nif [ \"$1\" = \"--json\" ]; then echo '{\"status\":\"ok\"}'; else echo ok; fi\n"
	if err := os.WriteFile(scriptPath, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	cliDir := filepath.Join(repo, "cli")
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		t.Fatal(err)
	}

	origScript := agentsLintScript
	origJSON := agentsLintJSON
	origProjectDir := testProjectDir
	t.Cleanup(func() {
		agentsLintScript = origScript
		agentsLintJSON = origJSON
		testProjectDir = origProjectDir
	})
	agentsLintScript = defaultAgentsLintScript
	agentsLintJSON = true
	testProjectDir = cliDir

	var stdout bytes.Buffer
	agentsLintCmd.SetOut(&stdout)
	t.Cleanup(func() { agentsLintCmd.SetOut(nil) })

	if err := runAgentsLint(agentsLintCmd, nil); err != nil {
		t.Fatalf("runAgentsLint: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &got); err != nil {
		t.Fatalf("stdout not JSON: %v\nGot: %s", err, stdout.String())
	}
	if got["status"] != "ok" {
		t.Errorf("status = %q, want ok", got["status"])
	}
}

func writeAgentsContract(path string, entries []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("<!-- BEGIN agents-write-surfaces-allowlist -->\n")
	for _, entry := range entries {
		b.WriteString(entry)
		b.WriteByte('\n')
	}
	b.WriteString("<!-- END agents-write-surfaces-allowlist -->\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
