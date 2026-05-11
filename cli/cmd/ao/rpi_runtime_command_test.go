// practices: [agile-manifesto, dora-metrics]
package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestSplitRuntimeCommand(t *testing.T) {
	t.Run("empty command", func(t *testing.T) {
		executable, args := splitRuntimeCommand("")
		if executable != "" {
			t.Fatalf("executable = %q, want empty", executable)
		}
		if len(args) != 0 {
			t.Fatalf("args = %v, want empty", args)
		}
	})

	t.Run("executable only", func(t *testing.T) {
		executable, args := splitRuntimeCommand("codex")
		if executable != "codex" {
			t.Fatalf("executable = %q, want codex", executable)
		}
		if len(args) != 0 {
			t.Fatalf("args = %v, want empty", args)
		}
	})

	t.Run("composite command", func(t *testing.T) {
		executable, args := splitRuntimeCommand("codex --profile ci")
		if executable != "codex" {
			t.Fatalf("executable = %q, want codex", executable)
		}
		want := []string{"--profile", "ci"}
		if !slices.Equal(args, want) {
			t.Fatalf("args = %v, want %v", args, want)
		}
	})
}

func TestRuntimeCommandArgs(t *testing.T) {
	t.Run("codex direct args", func(t *testing.T) {
		got := runtimeDirectCommandArgs("codex --profile ci", "do work")
		want := []string{"--profile", "ci", "exec", "do work"}
		if !slices.Equal(got, want) {
			t.Fatalf("runtimeDirectCommandArgs() = %v, want %v", got, want)
		}
	})

	t.Run("claude direct args", func(t *testing.T) {
		got := runtimeDirectCommandArgs("claude --model sonnet", "do work")
		want := []string{"--model", "sonnet", "-p", "do work"}
		if !slices.Equal(got, want) {
			t.Fatalf("runtimeDirectCommandArgs() = %v, want %v", got, want)
		}
	})

	t.Run("codex stream unsupported", func(t *testing.T) {
		_, err := runtimeStreamCommandArgs("codex --profile ci", "do work")
		if err == nil {
			t.Fatal("expected error for codex stream args")
		}
	})

	t.Run("claude stream args", func(t *testing.T) {
		got, err := runtimeStreamCommandArgs("claude --model sonnet", "do work")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"--model", "sonnet", "-p", "do work", "--output-format", "stream-json", "--verbose"}
		if !slices.Equal(got, want) {
			t.Fatalf("runtimeStreamCommandArgs() = %v, want %v", got, want)
		}
	})
}

func TestFormatRuntimePromptInvocationComposite(t *testing.T) {
	got := formatRuntimePromptInvocation("codex --profile ci", "hello")
	wantFragments := []string{
		"codex",
		"\"--profile\"",
		"\"ci\"",
		"\"exec\"",
		"\"hello\"",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(got, fragment) {
			t.Fatalf("formatRuntimePromptInvocation() = %q, missing %q", got, fragment)
		}
	}
}

func TestPreflightRuntimeAvailabilityCompositeCommand(t *testing.T) {
	var lookedUp []string
	mockLookPath := func(name string) (string, error) {
		lookedUp = append(lookedUp, name)
		if name == "codex" {
			return "/usr/bin/codex", nil
		}
		return "", fmt.Errorf("missing %s", name)
	}

	if err := preflightRuntimeAvailability("codex --profile ci", mockLookPath); err != nil {
		t.Fatalf("preflightRuntimeAvailability() error = %v, want nil", err)
	}
	if len(lookedUp) != 1 || lookedUp[0] != "codex" {
		t.Fatalf("lookedUp = %v, want [codex]", lookedUp)
	}
}

func TestPreflightRuntimeAvailabilityErrorIncludesExecutable(t *testing.T) {
	mockLookPath := func(name string) (string, error) {
		return "", fmt.Errorf("missing %s", name)
	}

	err := preflightRuntimeAvailability("codex --profile ci", mockLookPath)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "runtime executable \"codex\"") {
		t.Fatalf("error %q missing executable detail", msg)
	}
	if !strings.Contains(msg, "from \"codex --profile ci\"") {
		t.Fatalf("error %q missing source command detail", msg)
	}
}
