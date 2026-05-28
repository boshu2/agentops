// practices: [ai-assisted-dev, agile-manifesto]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetAutodevFlags resets cobra-persisted flag values for autodev.
func resetAutodevFlags(t *testing.T) {
	t.Helper()
	oldFile := autodevFile
	oldForce := autodevForce
	oldOutput := output
	oldJsonFlag := jsonFlag
	t.Cleanup(func() {
		autodevFile = oldFile
		autodevForce = oldForce
		output = oldOutput
		jsonFlag = oldJsonFlag
	})
	autodevFile = ""
	autodevForce = false
	output = "table"
	jsonFlag = false
}

func TestAutodev_Integration_ValidateValidProgram(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	resetAutodevFlags(t)

	programContent := `# PROGRAM.md

## Objective

Build a CLI tool for knowledge management.

## Mutable Scope

- cli/cmd/**

## Immutable Scope

- hooks/**

## Experiment Unit

One bounded TDD slice.

## Validation Commands

- ` + "`go test ./...`" + `

## Decision Policy

- Keep changes only when tests are green.

## Escalation Rules

Open a bead when required work escapes mutable scope.

## Stop Conditions

- Validation bundle is green.
`
	writeFile(t, filepath.Join(dir, "PROGRAM.md"), programContent)

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"autodev", "validate"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("expected valid program to pass validation, got error: %v\noutput:\n%s", err, out)
	}

	if !strings.Contains(out, "VALID") {
		t.Errorf("expected 'VALID' in output, got:\n%s", out)
	}

	if !strings.Contains(out, "PROGRAM.md") {
		t.Errorf("expected 'PROGRAM.md' path in output, got:\n%s", out)
	}
}

func TestAutodev_Integration_ValidateInvalidProgram(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	resetAutodevFlags(t)

	// Missing required sections
	writeFile(t, filepath.Join(dir, "PROGRAM.md"), "# PROGRAM.md\n\nJust some text, no sections.\n")

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"autodev", "validate"})
		return rootCmd.Execute()
	})

	// Validation should fail
	if err == nil {
		t.Error("expected validation to fail for invalid program")
	}

	if !strings.Contains(out, "INVALID") || !strings.Contains(out, "ERROR") {
		t.Errorf("expected 'INVALID' and 'ERROR' in output, got:\n%s", out)
	}
}

func TestAutodev_Integration_ValidateNoProgram(t *testing.T) {
	chdirTemp(t)
	resetAutodevFlags(t)

	_, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"autodev", "validate"})
		return rootCmd.Execute()
	})

	if err == nil {
		t.Error("expected error when no PROGRAM.md exists")
	}
}

func TestAutodev_Integration_ValidateJSON(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	resetAutodevFlags(t)

	programContent := `# PROGRAM.md

## Objective

Test objective.

## Mutable Scope

- src/**

## Immutable Scope

- config/**

## Experiment Unit

One slice.

## Validation Commands

- ` + "`make test`" + `

## Decision Policy

- Keep changes when green.

## Escalation Rules

Open a bead.

## Stop Conditions

- Tests pass.
`
	writeFile(t, filepath.Join(dir, "PROGRAM.md"), programContent)

	oldOutput := output
	output = "json"
	t.Cleanup(func() { output = oldOutput })

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"autodev", "validate"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("expected valid program, got error: %v", err)
	}

	var result autodevValidateResult
	if jsonErr := json.Unmarshal([]byte(out), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\nraw:\n%s", jsonErr, out)
	}

	if !result.Valid {
		t.Errorf("expected Valid=true, got false; errors: %v", result.Errors)
	}

	if result.Objective != "Test objective." {
		t.Errorf("expected objective 'Test objective.', got %q", result.Objective)
	}

	if result.MutableScopeCount != 1 {
		t.Errorf("expected 1 mutable scope item, got %d", result.MutableScopeCount)
	}
}

func TestAutodev_Integration_ShowProgram(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	resetAutodevFlags(t)

	programContent := `# PROGRAM.md

## Objective

Show command test.

## Mutable Scope

- alpha/**
- beta/**

## Immutable Scope

- gamma/**

## Experiment Unit

One slice.

## Validation Commands

- ` + "`make test`" + `
- ` + "`make lint`" + `

## Decision Policy

- Keep green.

## Escalation Rules

Open a bead.

## Stop Conditions

- All pass.
- No warnings.
`
	writeFile(t, filepath.Join(dir, "PROGRAM.md"), programContent)

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"autodev", "show"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("expected show to succeed, got error: %v", err)
	}

	if !strings.Contains(out, "Objective: Show command test.") {
		t.Errorf("expected 'Objective: Show command test.' in output, got:\n%s", out)
	}

	if !strings.Contains(out, "Mutable Scope: 2") {
		t.Errorf("expected 'Mutable Scope: 2', got:\n%s", out)
	}

	if !strings.Contains(out, "Validation Commands: 2") {
		t.Errorf("expected 'Validation Commands: 2', got:\n%s", out)
	}

	if !strings.Contains(out, "Stop Conditions: 2") {
		t.Errorf("expected 'Stop Conditions: 2', got:\n%s", out)
	}
}

func TestAutodev_Integration_InitCreatesFile(t *testing.T) {
	dir := chdirTemp(t)
	resetAutodevFlags(t)

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"autodev", "init", "My custom objective"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("expected init to succeed, got error: %v", err)
	}

	if !strings.Contains(out, "Created PROGRAM.md") {
		t.Errorf("expected 'Created PROGRAM.md' in output, got:\n%s", out)
	}

	data, readErr := os.ReadFile(filepath.Join(dir, "PROGRAM.md"))
	if readErr != nil {
		t.Fatalf("expected PROGRAM.md to exist: %v", readErr)
	}

	if !strings.Contains(string(data), "My custom objective") {
		t.Errorf("expected custom objective in file, got:\n%s", string(data))
	}
}

func TestAutodev_Integration_InitRefusesOverwrite(t *testing.T) {
	dir := chdirTemp(t)
	resetAutodevFlags(t)

	writeFile(t, filepath.Join(dir, "PROGRAM.md"), "# existing\n")

	_, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"autodev", "init"})
		return rootCmd.Execute()
	})

	if err == nil {
		t.Error("expected init to refuse overwriting existing PROGRAM.md")
	}
}
