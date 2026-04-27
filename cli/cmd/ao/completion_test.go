package main

import (
	"strings"
	"testing"
)

func TestCompletion_CommandExists(t *testing.T) {
	if completionCmd == nil {
		t.Fatal("completionCmd should not be nil")
	}
	if completionCmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Errorf("completionCmd.Use = %q, want %q", completionCmd.Use, "completion [bash|zsh|fish|powershell]")
	}
	if completionCmd.GroupID != "config" {
		t.Errorf("completionCmd.GroupID = %q, want %q", completionCmd.GroupID, "config")
	}
}

func TestCompletion_ValidArgs(t *testing.T) {
	expected := []string{"bash", "zsh", "fish", "powershell"}
	if len(completionCmd.ValidArgs) != len(expected) {
		t.Fatalf("ValidArgs length = %d, want %d", len(completionCmd.ValidArgs), len(expected))
	}
	for i, got := range completionCmd.ValidArgs {
		if got != expected[i] {
			t.Errorf("ValidArgs[%d] = %q, want %q", i, got, expected[i])
		}
	}
}

func TestCompletion_RegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "completion" {
			found = true
			break
		}
	}
	if !found {
		t.Error("completionCmd should be registered on rootCmd")
	}
}

func TestCompletion_BashOutputContainsShellMarker(t *testing.T) {
	out, err := executeCommand("completion", "bash")
	if err != nil {
		t.Fatalf("ao completion bash returned error: %v", err)
	}
	if out == "" {
		t.Fatal("bash completion output is empty")
	}
	// Bash v2 completion scripts start with a comment header or bash builtins.
	if !strings.Contains(out, "bash") && !strings.Contains(out, "__start_ao") && !strings.Contains(out, "_ao_") {
		t.Errorf("bash completion output does not contain expected shell markers, got: %.200s...", out)
	}
}

func TestCompletion_ZshOutputContainsShellMarker(t *testing.T) {
	out, err := executeCommand("completion", "zsh")
	if err != nil {
		t.Fatalf("ao completion zsh returned error: %v", err)
	}
	if out == "" {
		t.Fatal("zsh completion output is empty")
	}
	// Zsh completions contain compdef or #compdef directives.
	if !strings.Contains(out, "compdef") && !strings.Contains(out, "zsh") {
		t.Errorf("zsh completion output does not contain expected shell markers, got: %.200s...", out)
	}
}

func TestCompletion_FishOutputContainsShellMarker(t *testing.T) {
	out, err := executeCommand("completion", "fish")
	if err != nil {
		t.Fatalf("ao completion fish returned error: %v", err)
	}
	if out == "" {
		t.Fatal("fish completion output is empty")
	}
	// Fish completions use the 'complete' command.
	if !strings.Contains(out, "complete") {
		t.Errorf("fish completion output does not contain 'complete' command, got: %.200s...", out)
	}
}

func TestCompletion_PowerShellOutputContainsShellMarker(t *testing.T) {
	out, err := executeCommand("completion", "powershell")
	if err != nil {
		t.Fatalf("ao completion powershell returned error: %v", err)
	}
	if out == "" {
		t.Fatal("powershell completion output is empty")
	}
	if !strings.Contains(out, "Register-ArgumentCompleter") {
		t.Errorf("powershell completion output does not contain expected shell marker, got: %.200s...", out)
	}
}

func TestCompletion_OutputIsNonTrivial(t *testing.T) {
	// Each shell's completion output should be substantial (not just a few bytes).
	shells := []string{"bash", "zsh", "fish", "powershell"}
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			out, err := executeCommand("completion", shell)
			if err != nil {
				t.Fatalf("ao completion %s returned error: %v", shell, err)
			}
			// Completion scripts are typically hundreds of lines.
			if len(out) < 100 {
				t.Errorf("completion output for %s is suspiciously short (%d bytes)", shell, len(out))
			}
		})
	}
}

func TestCompletion_NoArgsReturnsError(t *testing.T) {
	_, err := executeCommand("completion")
	if err == nil {
		t.Error("ao completion with no args should return an error")
	}
}

func TestCompletion_InvalidShellReturnsError(t *testing.T) {
	_, err := executeCommand("completion", "nu")
	if err == nil {
		t.Error("ao completion with invalid shell should return an error")
	}
}

func TestCompletion_BashContainsCommandName(t *testing.T) {
	// The generated completion script should reference the root command name "ao".
	out, err := executeCommand("completion", "bash")
	if err != nil {
		t.Fatalf("ao completion bash returned error: %v", err)
	}
	if !strings.Contains(out, "ao") {
		t.Errorf("bash completion output should reference 'ao' command name, got: %.200s...", out)
	}
}
