package main

import (
	"io"
	"strings"
	"testing"
)

func TestForgeTranscriptTier1FlagsRegistered(t *testing.T) {
	for _, name := range []string{"tier", "model", "llm-endpoint"} {
		if forgeTranscriptCmd.Flags().Lookup(name) == nil {
			t.Fatalf("forge transcript flag %q is not registered", name)
		}
	}
}

func TestRunForgeTier1RequiresModel(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", "")
	t.Setenv("AGENTOPS_DREAM_CURATOR_WORKER_DIR", "")

	oldModel := forgeTier1Model
	oldEndpoint := forgeLLMEndpoint
	oldQuiet := forgeQuiet
	t.Cleanup(func() {
		forgeTier1Model = oldModel
		forgeLLMEndpoint = oldEndpoint
		forgeQuiet = oldQuiet
	})

	forgeTier1Model = ""
	forgeLLMEndpoint = ""
	forgeQuiet = true

	err := runForgeTier1(io.Discard, []string{"session.jsonl"})
	if err == nil {
		t.Fatal("expected missing --model error")
	}
	if !strings.Contains(err.Error(), "--model") {
		t.Fatalf("expected --model error, got %v", err)
	}
}
