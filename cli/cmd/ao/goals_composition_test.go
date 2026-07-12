package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoalsCompositionBuildsDormantExecutableModule(t *testing.T) {
	before := 0
	for _, child := range rootCmd.Commands() {
		if child.Name() == "goals" {
			before++
		}
	}
	if before != 1 {
		t.Fatalf("live goals owners before dormant composition = %d, want 1", before)
	}

	goalsPath, err := filepath.Abs(filepath.Join("..", "..", "internal", "goals", "testdata", "goals-spec-fixture.md"))
	if err != nil {
		t.Fatal(err)
	}
	command := newGoalsCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"--file", goalsPath, "validate"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "VALID:") {
		t.Fatalf("validate output = %q", output.String())
	}

	after := 0
	for _, child := range rootCmd.Commands() {
		if child.Name() == "goals" {
			after++
		}
	}
	if after != 1 {
		t.Fatalf("dormant composition registered a second live owner: %d", after)
	}
}
