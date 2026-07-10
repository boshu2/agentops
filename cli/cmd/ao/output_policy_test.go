package main

import (
	"strings"
	"testing"
)

func TestOutputPolicyRejectsUnknownFormat(t *testing.T) {
	_, err := executeCommand("--output", "xml", "capabilities")
	if err == nil || !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("explicit --output=xml silently fell back: %v", err)
	}
}

func TestOutputPolicyRejectsConflictingExplicitFormats(t *testing.T) {
	_, err := executeCommand("--output", "yaml", "--json", "capabilities")
	if err == nil || !strings.Contains(err.Error(), "conflicting output formats") {
		t.Fatalf("explicit --json and --output=yaml silently chose one: %v", err)
	}
}
