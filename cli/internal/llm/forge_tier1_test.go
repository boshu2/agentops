package llm

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

// writeFixtureJSONL writes a minimal Claude JSONL fixture at path and returns
// the session ID embedded in the content.
func writeFixtureJSONL(t *testing.T, dir, name string) (string, string) {
	t.Helper()
	path := filepath.Join(dir, name)
	sessionID := "l2-fixture-" + strings.TrimSuffix(name, filepath.Ext(name))
	lines := []map[string]any{
		{
			"type":      "user",
			"sessionId": sessionID,
			"cwd":       "/fixture/ws",
			"timestamp": "2026-04-11T12:00:00Z",
			"message": map[string]any{
				"role":    "user",
				"content": "Please implement the turn chunker for the forge tier1 path. My key is ghp_abcdefghijklmnopqrstuvwxyz0123456789XY.",
			},
		},
		{
			"type":      "assistant",
			"sessionId": sessionID,
			"cwd":       "/fixture/ws",
			"timestamp": "2026-04-11T12:00:05Z",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "I'll write the chunker at cli/internal/llm/chunker.go and add tests for the budget boundaries."},
				},
			},
		},
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	defer f.Close()
	for _, l := range lines {
		if err := json.NewEncoder(f).Encode(l); err != nil {
			t.Fatalf("encode fixture line: %v", err)
		}
	}
	return path, sessionID
}

// mockOllamaServer returns a server that validates stream:false and returns
// a spike-shaped response for every /api/generate request.
func mockOllamaServer(t *testing.T, wantModel string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, tagsResponseOK(wantModel, "sha256:deadbeef"))
	})
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"model_info":{"gemma2.context_length":8192}}`)
	})
	mux.HandleFunc("/api/generate", func(w http.ResponseWriter, r *http.Request) {
		// Assert stream:false in the body.
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		if stream, _ := req["stream"].(bool); stream {
			t.Errorf("generate request set stream:true — must be false")
		}
		_, _ = io.WriteString(w, `{"response":"### Intent\nImplement chunker\n\n### Summary\nThe assistant agrees to write the chunker with budget tests.\n\n### Entities\n- [[file:cli/internal/llm/chunker.go]]\n\n### Assistant condensed\nI will write the chunker and add tests for the budget boundaries.\n"}`)
	})
	return httptest.NewServer(mux)
}

func TestRunForgeTier1_EndToEnd(t *testing.T) {
	// Ensure the kill switch is OFF so the pipeline runs.
	t.Setenv(KillSwitchEnv, "")

	srv := mockOllamaServer(t, "gemma2:9b")
	defer srv.Close()

	tmp := t.TempDir()
	fixture, _ := writeFixtureJSONL(t, tmp, "session-abc.jsonl")
	outDir := filepath.Join(tmp, "out")

	opts := Tier1Options{
		SourcePaths: []string{fixture},
		OutputDir:   outDir,
		Model:       "gemma2:9b",
		Endpoint:    srv.URL,
		Quiet:       true,
		Workspace:   "/fixture/ws",
	}
	result, err := RunForgeTier1(opts)
	if err != nil {
		t.Fatalf("RunForgeTier1: %v", err)
	}
	if result.FilesProcessed != 1 {
		t.Errorf("FilesProcessed: want 1, got %d (errors: %+v)", result.FilesProcessed, result.Errors)
	}
	if len(result.SessionsWrote) != 1 {
		t.Fatalf("want 1 session written, got %d", len(result.SessionsWrote))
	}

	// Read the written page and assert full pipeline behavior.
	b, err := os.ReadFile(result.SessionsWrote[0])
	if err != nil {
		t.Fatalf("read session page: %v", err)
	}
	body := string(b)

	// Frontmatter must have all required fields.
	for _, f := range []string{
		"type: session",
		"status: draft",
		"tier: 1",
		"model: gemma2:9b",
		"model_digest: sha256:deadbeef",
		"ingested_by: ao-forge-tier1",
	} {
		if !strings.Contains(body, f) {
			t.Errorf("frontmatter missing %q", f)
		}
	}

	// Body must contain the parsed intent from the mock LLM output.
	if !strings.Contains(body, "Implement chunker") {
		t.Errorf("body missing 'Implement chunker' intent")
	}

	// CRITICAL: the GitHub token from the fixture MUST be redacted — the
	// chunker shouldn't have seen it, and it must not appear on disk.
	if strings.Contains(body, "ghp_abcdefghijklmn") {
		t.Fatalf("PRIVACY VIOLATION: GitHub token in written session page:\n%s", body)
	}
}

func TestRunForgeTier1_KillSwitch(t *testing.T) {
	t.Setenv(KillSwitchEnv, "1")
	opts := Tier1Options{
		SourcePaths: []string{"ignored.jsonl"},
		OutputDir:   t.TempDir(),
		Model:       "gemma2:9b",
	}
	_, err := RunForgeTier1(opts)
	if err == nil {
		t.Fatal("want error when kill switch is set")
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Errorf("want disabled error, got %v", err)
	}
}

func TestRunForgeTier1_MissingOutputDirErrors(t *testing.T) {
	t.Setenv(KillSwitchEnv, "")
	opts := Tier1Options{
		SourcePaths: []string{"file.jsonl"},
		Model:       "gemma2:9b",
	}
	_, err := RunForgeTier1(opts)
	if err == nil || !strings.Contains(err.Error(), "OutputDir") {
		t.Errorf("want OutputDir error, got %v", err)
	}
}

func TestRunForgeTier1_EmptySourcesErrors(t *testing.T) {
	t.Setenv(KillSwitchEnv, "")
	opts := Tier1Options{
		OutputDir: t.TempDir(),
		Model:     "gemma2:9b",
	}
	_, err := RunForgeTier1(opts)
	if err == nil || !strings.Contains(err.Error(), "source") {
		t.Errorf("want source paths error, got %v", err)
	}
}

func TestRunForgeTier1_FactoryInjection(t *testing.T) {
	// Demonstrates the test-injection path: substitute a fakeLLM without any
	// HTTP at all. This is the unit-level fallback if L2 httptest is too
	// heavy for some scenarios.
	t.Setenv(KillSwitchEnv, "")
	tmp := t.TempDir()
	fixture, _ := writeFixtureJSONL(t, tmp, "session-xyz.jsonl")

	opts := Tier1Options{
		SourcePaths: []string{fixture},
		OutputDir:   filepath.Join(tmp, "out"),
		Model:       "gemma2:9b",
		Endpoint:    "http://unused",
		Quiet:       true,
	}
	opts.SetGeneratorFactory(func(_ OllamaOptions) (Generator, error) {
		return &fakeLLM{responses: []string{validSpikeOutput}}, nil
	})
	result, err := RunForgeTier1(opts)
	if err != nil {
		t.Fatalf("RunForgeTier1: %v", err)
	}
	if result.FilesProcessed != 1 {
		t.Errorf("FilesProcessed: want 1, got %d", result.FilesProcessed)
	}
}
