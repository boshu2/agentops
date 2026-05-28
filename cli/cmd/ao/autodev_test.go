// practices: [ai-assisted-dev, agile-manifesto]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleProgramMarkdown = `# PROGRAM.md

## Objective

Build a PROGRAM.md-driven autodev loop.

## Mutable Scope

- cli/cmd/ao/**

## Immutable Scope

- hooks/**

## Experiment Unit

One bounded TDD slice.

## Validation Commands

- ` + "`cd cli && go test ./cmd/ao/...`" + `

## Decision Policy

- Keep changes only when tests pass.

## Escalation Rules

Open a bead for cross-cutting work.

## Stop Conditions

- Tests are green.
`

func TestCobraAutodevInitCreatesProgram(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	out, err := executeCommand("autodev", "init", "Build a repo-native autodev loop")
	if err != nil {
		t.Fatalf("ao autodev init failed: %v\noutput: %s", err, out)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "PROGRAM.md"))
	if err != nil {
		t.Fatalf("PROGRAM.md not created: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "Build a repo-native autodev loop") {
		t.Fatalf("PROGRAM.md missing objective override:\n%s", text)
	}
	for _, heading := range []string{
		"## Objective",
		"## Mutable Scope",
		"## Immutable Scope",
		"## Experiment Unit",
		"## Validation Commands",
		"## Decision Policy",
		"## Escalation Rules",
		"## Stop Conditions",
	} {
		if !strings.Contains(text, heading) {
			t.Fatalf("PROGRAM.md missing %q:\n%s", heading, text)
		}
	}
}

func TestCobraAutodevValidateJSON(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	if err := os.WriteFile(filepath.Join(tmp, "PROGRAM.md"), []byte(sampleProgramMarkdown), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("autodev", "validate", "--json")
	if err != nil {
		t.Fatalf("ao autodev validate --json failed: %v\noutput: %s", err, out)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if valid, _ := result["valid"].(bool); !valid {
		t.Fatalf("validate JSON valid=false: %#v", result)
	}
	if path, _ := result["path"].(string); path != "PROGRAM.md" {
		t.Fatalf("validate JSON path = %q, want PROGRAM.md", path)
	}
}

func TestCobraAutodevShowJSON(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	if err := os.WriteFile(filepath.Join(tmp, "PROGRAM.md"), []byte(sampleProgramMarkdown), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("autodev", "show", "--json")
	if err != nil {
		t.Fatalf("ao autodev show --json failed: %v\noutput: %s", err, out)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if objective, _ := result["objective"].(string); !strings.Contains(objective, "PROGRAM.md-driven autodev loop") {
		t.Fatalf("objective = %q", objective)
	}
}

func TestCobraAutodevValidateFallsBackToAUTODEVJSON(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	if err := os.WriteFile(filepath.Join(tmp, "AUTODEV.md"), []byte(sampleProgramMarkdown), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("autodev", "validate", "--json")
	if err != nil {
		t.Fatalf("ao autodev validate --json failed: %v\noutput: %s", err, out)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if valid, _ := result["valid"].(bool); !valid {
		t.Fatalf("validate JSON valid=false: %#v", result)
	}
	if path, _ := result["path"].(string); path != "AUTODEV.md" {
		t.Fatalf("validate JSON path = %q, want AUTODEV.md", path)
	}
}

func TestCobraAutodevShowPrefersPROGRAMOverAUTODEV(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	autodevText := strings.Replace(sampleProgramMarkdown, "PROGRAM.md-driven autodev loop", "AUTODEV fallback contract", 1)
	if err := os.WriteFile(filepath.Join(tmp, "AUTODEV.md"), []byte(autodevText), 0o644); err != nil {
		t.Fatal(err)
	}

	programText := strings.Replace(sampleProgramMarkdown, "PROGRAM.md-driven autodev loop", "PROGRAM preferred contract", 1)
	if err := os.WriteFile(filepath.Join(tmp, "PROGRAM.md"), []byte(programText), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("autodev", "show", "--json")
	if err != nil {
		t.Fatalf("ao autodev show --json failed: %v\noutput: %s", err, out)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if objective, _ := result["objective"].(string); !strings.Contains(objective, "PROGRAM preferred contract") {
		t.Fatalf("objective = %q, want PROGRAM preferred contract", objective)
	}
}

// ---------------------------------------------------------------------------
// outputAutodevValidateResult
// ---------------------------------------------------------------------------

func TestOutputAutodevValidateResult_Human_Valid(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	result := autodevValidateResult{
		Path:                 "/tmp/program.yaml",
		Valid:                true,
		Format:               "yaml",
		MutableScopeCount:    3,
		ImmutableScopeCount:  2,
		ValidationCommandCnt: 5,
		StopConditionCount:   1,
	}

	out, err := captureStdout(t, func() error {
		return outputAutodevValidateResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "VALID: /tmp/program.yaml (yaml)") {
		t.Errorf("missing valid header, got: %q", out)
	}
	if !strings.Contains(out, "Mutable scope: 3") {
		t.Errorf("missing mutable scope, got: %q", out)
	}
	if !strings.Contains(out, "Immutable scope: 2") {
		t.Errorf("missing immutable scope, got: %q", out)
	}
	if !strings.Contains(out, "Validation commands: 5") {
		t.Errorf("missing validation commands, got: %q", out)
	}
	if !strings.Contains(out, "Stop conditions: 1") {
		t.Errorf("missing stop conditions, got: %q", out)
	}
}

func TestOutputAutodevValidateResult_Human_Invalid(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	result := autodevValidateResult{
		Path:   "/tmp/bad.yaml",
		Valid:  false,
		Errors: []string{"missing objective", "no stop conditions"},
	}

	out, err := captureStdout(t, func() error {
		return outputAutodevValidateResult(result)
	})
	if err == nil {
		t.Fatal("expected error for invalid result")
	}
	if !strings.Contains(out, "INVALID: /tmp/bad.yaml") {
		t.Errorf("missing invalid header, got: %q", out)
	}
	if !strings.Contains(out, "ERROR: missing objective") {
		t.Errorf("missing first error, got: %q", out)
	}
	if !strings.Contains(out, "ERROR: no stop conditions") {
		t.Errorf("missing second error, got: %q", out)
	}
}

func TestOutputAutodevValidateResult_JSON(t *testing.T) {
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	result := autodevValidateResult{
		Path:  "/tmp/test.yaml",
		Valid: true,
	}

	out, err := captureStdout(t, func() error {
		return outputAutodevValidateResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed autodevValidateResult
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !parsed.Valid {
		t.Error("Valid should be true")
	}
}

// ---------------------------------------------------------------------------
// displayProgramPath
// ---------------------------------------------------------------------------

func TestDisplayProgramPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"/tmp/something.yaml", "/tmp/something.yaml"},
	}
	for _, tt := range tests {
		got := displayProgramPath(tt.input)
		if tt.input == "" && got != "" {
			t.Errorf("displayProgramPath(%q) = %q, want empty", tt.input, got)
		}
		if tt.input != "" && got == "" {
			t.Errorf("displayProgramPath(%q) = empty, want non-empty", tt.input)
		}
	}
}
