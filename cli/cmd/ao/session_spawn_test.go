// practices: [event-sourcing-cqrs, distributed-tracing]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSessionTemplate(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "test.toml")
	content := `
schema_version = 1
role = "test-role"
description = "A test session"

[identity]
beads_actor_template = "test-{{date}}"
session_name_template = "test-session-{{date}}"
agent = "claude"

[workspace]
cwd = "$HOME"

[[init.steps]]
name = "step-one"
cmd = "echo hello"
note = "a test step"

[tmux]
session_name = "{{session_name}}"

[[tmux.panes]]
position = "main"
cmd = "echo main"

[[tmux.panes]]
position = "right-30%"
cmd = "echo side"

[heartbeat]
cadence_minutes = 30
target_issue = "test-1"

[on_exit]
auto_handoff = true
handoff_path_template = "$HOME/.agents/handoff/test-{{date}}.md"
handoff_command = "ao handoff --collect"
`
	if err := os.WriteFile(tmplPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	tmpl, err := loadSessionTemplate(tmplPath)
	if err != nil {
		t.Fatalf("load template: %v", err)
	}
	if tmpl.Role != "test-role" {
		t.Fatalf("role = %q, want test-role", tmpl.Role)
	}
	if tmpl.Identity.Agent != "claude" {
		t.Fatalf("agent = %q, want claude", tmpl.Identity.Agent)
	}
	if len(tmpl.Init.Steps) != 1 {
		t.Fatalf("init steps = %d, want 1", len(tmpl.Init.Steps))
	}
	if tmpl.Init.Steps[0].Name != "step-one" {
		t.Fatalf("step name = %q, want step-one", tmpl.Init.Steps[0].Name)
	}
	if len(tmpl.Tmux.Panes) != 2 {
		t.Fatalf("panes = %d, want 2", len(tmpl.Tmux.Panes))
	}
	if tmpl.Heartbeat.CadenceMinutes != 30 {
		t.Fatalf("heartbeat cadence = %d, want 30", tmpl.Heartbeat.CadenceMinutes)
	}
	if !tmpl.OnExit.AutoHandoff {
		t.Fatal("auto_handoff = false, want true")
	}
}

func TestLoadSessionTemplateRejectsInvalid(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "bad schema version",
			content: `schema_version = 2` + "\n" + `role = "x"` + "\n" + `[identity]` + "\n" + `session_name_template = "x"`,
			wantErr: "unsupported schema_version",
		},
		{
			name:    "missing role",
			content: `schema_version = 1` + "\n" + `[identity]` + "\n" + `session_name_template = "x"`,
			wantErr: "missing required field: role",
		},
		{
			name:    "missing session name template",
			content: `schema_version = 1` + "\n" + `role = "x"` + "\n" + `[identity]`,
			wantErr: "missing required field: identity.session_name_template",
		},
	}

	for _, tc := range cases {
		p := filepath.Join(dir, tc.name+".toml")
		if err := os.WriteFile(p, []byte(tc.content), 0o644); err != nil {
			t.Fatalf("%s: write: %v", tc.name, err)
		}
		_, err := loadSessionTemplate(p)
		if err == nil {
			t.Fatalf("%s: expected error, got nil", tc.name)
		}
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Fatalf("%s: error = %q, want substring %q", tc.name, err.Error(), tc.wantErr)
		}
	}
}

func TestBuildTemplateVars(t *testing.T) {
	tmpl := &SessionTemplate{
		Identity: SessionIdentity{
			SessionNameTemplate: "validator-{{date}}-{{hostname}}",
		},
	}
	vars := buildTemplateVars(tmpl, "2026-05-05")

	if vars["{{date}}"] != "2026-05-05" {
		t.Fatalf("date = %q, want 2026-05-05", vars["{{date}}"])
	}
	if vars["{{hostname}}"] == "" {
		t.Fatal("hostname is empty")
	}
	if !strings.HasPrefix(vars["{{session_name}}"], "validator-2026-05-05-") {
		t.Fatalf("session_name = %q, want prefix validator-2026-05-05-", vars["{{session_name}}"])
	}
	if vars["$HOME"] == "" {
		t.Fatal("$HOME is empty")
	}
}

func TestExpandVars(t *testing.T) {
	vars := map[string]string{
		"{{date}}":         "2026-05-05",
		"{{session_name}}": "test-session",
		"$HOME":            "/home/testuser",
	}
	cases := []struct {
		input string
		want  string
	}{
		{"{{date}}", "2026-05-05"},
		{"session-{{date}}-log", "session-2026-05-05-log"},
		{"$HOME/.agents/{{session_name}}.md", "/home/testuser/.agents/test-session.md"},
		{"no-vars-here", "no-vars-here"},
	}
	for _, tc := range cases {
		got := expandVars(tc.input, vars)
		if got != tc.want {
			t.Fatalf("expandVars(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestRunInitStepsDryRun(t *testing.T) {
	steps := []SessionInitStep{
		{Name: "first", Cmd: "echo {{date}}", Note: "test note"},
		{Name: "second", Cmd: "echo done"},
	}
	vars := map[string]string{"{{date}}": "2026-05-05"}

	err := runInitSteps(steps, vars, t.TempDir(), "", true)
	if err != nil {
		t.Fatalf("dry-run init steps: %v", err)
	}
}

func TestRunInitStepsExecutes(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker.txt")
	steps := []SessionInitStep{
		{Name: "create-marker", Cmd: "echo hello > " + marker},
	}
	vars := map[string]string{}

	if err := runInitSteps(steps, vars, dir, "", false); err != nil {
		t.Fatalf("init steps: %v", err)
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Fatalf("marker = %q, want hello", string(data))
	}
}

func TestRunInitStepsFailsOnError(t *testing.T) {
	steps := []SessionInitStep{
		{Name: "will-fail", Cmd: "exit 1"},
	}
	err := runInitSteps(steps, map[string]string{}, t.TempDir(), "", false)
	if err == nil {
		t.Fatal("expected error from failing init step")
	}
	if !strings.Contains(err.Error(), "will-fail") {
		t.Fatalf("error = %q, want step name", err.Error())
	}
}

func TestRunInitStepsSetsBeadsActor(t *testing.T) {
	dir := t.TempDir()
	actorFile := filepath.Join(dir, "actor.txt")
	steps := []SessionInitStep{
		{Name: "first-step", Cmd: "true"},
		{Name: "record-actor", Cmd: `printf '%s' "$BEADS_ACTOR" > ` + actorFile},
	}

	if err := runInitSteps(steps, map[string]string{}, dir, "claude-validator", false); err != nil {
		t.Fatalf("init steps: %v", err)
	}
	data, err := os.ReadFile(actorFile)
	if err != nil {
		t.Fatalf("read actor file: %v", err)
	}
	if got := string(data); got != "claude-validator" {
		t.Fatalf("BEADS_ACTOR = %q, want %q (must be the template actor, not a step command)", got, "claude-validator")
	}
}

func TestSanitizeHostname(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "bushido-box", "bushido-box"},
		{"dotted fqdn", "host.example.local", "host.example.local"},
		{"underscore kept", "my_host", "my_host"},
		{"strips shell metachars", "host; rm -rf ~", "hostrm-rf"},
		{"strips command substitution", "h$(whoami)", "hwhoami"},
		{"strips quotes and spaces", `a b'c"`, "abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeHostname(tc.in); got != tc.want {
				t.Fatalf("sanitizeHostname(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCreateTmuxSessionDryRun(t *testing.T) {
	cfg := SessionTmux{
		SessionName: "test-{{date}}",
		Panes: []SessionPane{
			{Position: "main", Cmd: "echo main"},
			{Position: "right-30%", Cmd: "echo side"},
		},
	}
	vars := map[string]string{"{{date}}": "2026-05-05"}

	err := createTmuxSession(cfg, vars, t.TempDir(), true)
	if err != nil {
		t.Fatalf("dry-run tmux session: %v", err)
	}
}
