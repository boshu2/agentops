// practices: [wiki-knowledge-surface, design-by-contract]
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

func TestRunAgentsLint_MissingScript(t *testing.T) {
	origScript := agentsLintScript
	origJSON := agentsLintJSON
	t.Cleanup(func() {
		agentsLintScript = origScript
		agentsLintJSON = origJSON
	})
	agentsLintScript = filepath.Join(t.TempDir(), "missing.sh")
	agentsLintJSON = false

	var stderr bytes.Buffer
	agentsLintCmd.SetErr(&stderr)
	agentsLintCmd.SetOut(&stderr)
	t.Cleanup(func() {
		agentsLintCmd.SetErr(nil)
		agentsLintCmd.SetOut(nil)
	})

	err := runAgentsLint(agentsLintCmd, nil)
	if err == nil {
		t.Fatal("expected error when script is missing")
	}
	if !strings.Contains(err.Error(), "lint script not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunAgentsLint_PassthroughExitCodes(t *testing.T) {
	origScript := agentsLintScript
	origJSON := agentsLintJSON
	t.Cleanup(func() {
		agentsLintScript = origScript
		agentsLintJSON = origJSON
	})

	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{
			name:     "exit zero passes",
			body:     "#!/usr/bin/env bash\nexit 0\n",
			wantCode: 0,
		},
		{
			name:     "exit one surfaces as AgentsLintError 1",
			body:     "#!/usr/bin/env bash\necho violation >&2\nexit 1\n",
			wantCode: 1,
		},
		{
			name:     "exit two surfaces as AgentsLintError 2",
			body:     "#!/usr/bin/env bash\necho misuse >&2\nexit 2\n",
			wantCode: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath := filepath.Join(t.TempDir(), "fake-lint.sh")
			if err := os.WriteFile(scriptPath, []byte(tt.body), 0o755); err != nil {
				t.Fatal(err)
			}
			agentsLintScript = scriptPath
			agentsLintJSON = false

			var stdout, stderr bytes.Buffer
			agentsLintCmd.SetOut(&stdout)
			agentsLintCmd.SetErr(&stderr)
			t.Cleanup(func() {
				agentsLintCmd.SetOut(nil)
				agentsLintCmd.SetErr(nil)
			})

			err := runAgentsLint(agentsLintCmd, nil)
			if tt.wantCode == 0 {
				if err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
				return
			}
			var lintErr *AgentsLintError
			if !errors.As(err, &lintErr) {
				t.Fatalf("expected *AgentsLintError, got %T: %v", err, err)
			}
			if lintErr.ExitCode != tt.wantCode {
				t.Errorf("ExitCode = %d, want %d", lintErr.ExitCode, tt.wantCode)
			}
			if lintErr.Script != scriptPath {
				t.Errorf("Script = %q, want %q", lintErr.Script, scriptPath)
			}
		})
	}
}

func TestRunAgentsLint_ForwardsJSONFlag(t *testing.T) {
	origScript := agentsLintScript
	origJSON := agentsLintJSON
	t.Cleanup(func() {
		agentsLintScript = origScript
		agentsLintJSON = origJSON
	})

	scriptPath := filepath.Join(t.TempDir(), "echo-json.sh")
	body := "#!/usr/bin/env bash\nif [ \"$1\" = \"--json\" ]; then echo '{\"got\":\"json\"}'; else echo 'no flag'; fi\n"
	if err := os.WriteFile(scriptPath, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	agentsLintScript = scriptPath
	agentsLintJSON = true

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
	if got["got"] != "json" {
		t.Errorf("script did not receive --json flag (stdout: %s)", stdout.String())
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

func TestAgentsLintError_Message(t *testing.T) {
	err := &AgentsLintError{ExitCode: 7, Script: "/path/to/lint.sh"}
	got := err.Error()
	if !strings.Contains(got, "/path/to/lint.sh") {
		t.Errorf("message missing script path: %q", got)
	}
	if !strings.Contains(got, "7") {
		t.Errorf("message missing exit code: %q", got)
	}
}
