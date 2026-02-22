package main

import (
	"fmt"
	"testing"
)

func TestSpawnClaudePhase_UsesDirectSpawnFunction(t *testing.T) {
	origSpawnDirect := spawnDirectFn
	defer func() { spawnDirectFn = origSpawnDirect }()

	directCalled := false
	var gotPrompt, gotCwd string
	var gotPhase int
	spawnDirectFn = func(prompt, cwd string, phaseNum int) error {
		directCalled = true
		gotPrompt = prompt
		gotCwd = cwd
		gotPhase = phaseNum
		return nil
	}

	cwd := t.TempDir()
	if err := spawnClaudePhase("test prompt", cwd, "test-run", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !directCalled {
		t.Fatal("expected spawnDirectFn to be called")
	}
	if gotPrompt != "test prompt" {
		t.Errorf("prompt = %q, want %q", gotPrompt, "test prompt")
	}
	if gotCwd != cwd {
		t.Errorf("cwd = %q, want %q", gotCwd, cwd)
	}
	if gotPhase != 1 {
		t.Errorf("phase = %d, want %d", gotPhase, 1)
	}
}

func TestSpawnClaudePhase_DirectErrorPropagates(t *testing.T) {
	origSpawnDirect := spawnDirectFn
	defer func() { spawnDirectFn = origSpawnDirect }()

	expected := fmt.Errorf("runtime process crashed")
	spawnDirectFn = func(prompt, cwd string, phaseNum int) error {
		return expected
	}

	err := spawnClaudePhase("test prompt", t.TempDir(), "test-run", 1)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if err != expected {
		t.Fatalf("error = %v, want %v", err, expected)
	}
}
