package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForgeTranscriptTier1FlagsRegistered(t *testing.T) {
	for _, name := range []string{"tier", "model", "llm-endpoint", "max-chars"} {
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

func TestRunForgeTier1MaxCharsPassesLargeBudget(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", filepath.Join(t.TempDir(), "missing-config.yaml"))
	t.Setenv("AGENTOPS_DREAM_CURATOR_WORKER_DIR", "")
	t.Setenv("AGENTOPS_FORGE_TIER1_DISABLE", "")

	tmp := t.TempDir()
	t.Chdir(tmp)
	sourcePath := filepath.Join(tmp, "large-session.jsonl")
	writeLargeTier1Fixture(t, sourcePath)

	var capturedPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{
					"name":   "gemma4:e4b",
					"digest": "sha256:largechunk",
				}},
			})
		case "/api/show":
			_, _ = io.WriteString(w, `{"model_info":{"gemma.context_length":8192}}`)
		case "/api/generate":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode generate request: %v", err)
			}
			capturedPrompt, _ = req["prompt"].(string)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"response": `{"title":"Large chunk accepted","summary":"The large chunk reached the model prompt.","entities":[],"concepts":[],"decisions":[],"open_questions":[],"work_phase":"verify"}`,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldModel := forgeTier1Model
	oldEndpoint := forgeLLMEndpoint
	oldQuiet := forgeQuiet
	oldMaxChars := forgeTier1MaxChars
	t.Cleanup(func() {
		forgeTier1Model = oldModel
		forgeLLMEndpoint = oldEndpoint
		forgeQuiet = oldQuiet
		forgeTier1MaxChars = oldMaxChars
	})

	forgeTier1Model = "gemma4:e4b"
	forgeLLMEndpoint = server.URL
	forgeQuiet = true
	forgeTier1MaxChars = 8000

	if err := runForgeTier1(io.Discard, []string{sourcePath}); err != nil {
		t.Fatalf("runForgeTier1: %v", err)
	}

	if !strings.Contains(capturedPrompt, strings.Repeat("userword ", 300)) {
		t.Fatalf("prompt did not include the enlarged user-side chunk budget")
	}
	if !strings.Contains(capturedPrompt, strings.Repeat("assistantword ", 300)) {
		t.Fatalf("prompt did not include the enlarged assistant-side chunk budget")
	}
	if strings.Contains(capturedPrompt, strings.Repeat("userword ", 500)) ||
		strings.Contains(capturedPrompt, strings.Repeat("assistantword ", 400)) {
		t.Fatalf("prompt exceeded the configured 8000-char turn budget")
	}
}

func writeLargeTier1Fixture(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	defer f.Close()

	lines := []map[string]any{
		{
			"type":      "user",
			"sessionId": "large-chunk-session",
			"cwd":       "/fixture/ws",
			"timestamp": "2026-04-13T12:00:00Z",
			"message": map[string]any{
				"role":    "user",
				"content": strings.Repeat("userword ", 900),
			},
		},
		{
			"type":      "assistant",
			"sessionId": "large-chunk-session",
			"cwd":       "/fixture/ws",
			"timestamp": "2026-04-13T12:00:05Z",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": strings.Repeat("assistantword ", 700)},
				},
			},
		},
	}
	for _, line := range lines {
		if err := json.NewEncoder(f).Encode(line); err != nil {
			t.Fatalf("encode fixture: %v", err)
		}
	}
}
