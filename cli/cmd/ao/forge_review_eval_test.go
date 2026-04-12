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

	"github.com/boshu2/agentops/cli/internal/llm"
)

const forgeReviewEvalDraftPage = `---
type: session
session_id: forge-review-eval-draft
status: draft
tier: 1
confidence: 0.85
---

# Session notes

### Build eval harness

Summary of the work done.

**Entities:**
- [[file:cli/internal/llm/review.go]]

**Assistant:** The assistant added the eval harness.
`

const forgeReviewEvalLowConfidencePage = `---
type: session
session_id: forge-review-eval-low
status: draft
tier: 1
confidence: 0.01
---

# Session notes

### Chunk 0 - SKIP
`

func TestForgeReviewEvalJSONDoesNotMutateSessions(t *testing.T) {
	sessionsDir := t.TempDir()
	writeForgeReviewEvalFile(t, sessionsDir, "promote.md", forgeReviewEvalDraftPage)
	writeForgeReviewEvalFile(t, sessionsDir, "skip.md", forgeReviewEvalLowConfidencePage)

	manifestPath := filepath.Join(t.TempDir(), "forge-review-eval.json")
	writeForgeReviewEvalManifest(t, manifestPath, llm.ReviewEvalManifest{
		ID: "cli-forge-review-eval",
		Cases: []llm.ReviewEvalCase{
			{ID: "promote", Path: "promote.md", Expected: "promote"},
			{ID: "skip", Path: "skip.md", Expected: "skip"},
		},
	})

	out, err := executeCommand("forge", "review", "--sessions-dir", sessionsDir, "--eval", manifestPath, "--json")
	if err != nil {
		t.Fatalf("forge review --eval --json: %v\noutput:\n%s", err, out)
	}

	var report llm.ReviewEvalReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("parse JSON report: %v\noutput:\n%s", err, out)
	}
	if report.Cases != 2 || report.Passed != 2 || report.Failed != 0 {
		t.Fatalf("unexpected report summary: %+v", report)
	}

	data, err := os.ReadFile(filepath.Join(sessionsDir, "promote.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "status: reviewed") {
		t.Fatalf("eval command mutated session page:\n%s", string(data))
	}
}

func TestForgeReviewReviewerModelJSONPromotes(t *testing.T) {
	sessionsDir := t.TempDir()
	writeForgeReviewEvalFile(t, sessionsDir, "promote.md", forgeReviewEvalDraftPage)
	srv := newForgeReviewOllamaServer(t, "gemma2:9b", "DECISION: promote\nREASON: useful page")
	defer srv.Close()

	out, err := executeCommand(
		"forge", "review",
		"--sessions-dir", sessionsDir,
		"--reviewer-model", "gemma2:9b",
		"--reviewer-endpoint", srv.URL,
		"--json",
	)
	if err != nil {
		t.Fatalf("forge review --reviewer-model --json: %v\noutput:\n%s", err, out)
	}

	var result struct {
		Reviewed int `json:"reviewed"`
		Skipped  int `json:"skipped"`
		Errors   int `json:"errors"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse JSON review result: %v\noutput:\n%s", err, out)
	}
	if result.Reviewed != 1 || result.Skipped != 0 || result.Errors != 0 {
		t.Fatalf("unexpected review result: %+v", result)
	}
	data, err := os.ReadFile(filepath.Join(sessionsDir, "promote.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, "status: reviewed") {
		t.Fatalf("reviewer did not promote page:\n%s", body)
	}
	if !strings.Contains(body, "reviewed_by: ao-forge-tier2-llm-gemma2:9b") {
		t.Fatalf("reviewer id not recorded:\n%s", body)
	}
}

func TestForgeReviewEvalUsesReviewerModel(t *testing.T) {
	sessionsDir := t.TempDir()
	writeForgeReviewEvalFile(t, sessionsDir, "skip.md", forgeReviewEvalDraftPage)
	manifestPath := filepath.Join(t.TempDir(), "forge-review-eval.json")
	writeForgeReviewEvalManifest(t, manifestPath, llm.ReviewEvalManifest{
		ID: "cli-forge-review-eval-with-reviewer",
		Cases: []llm.ReviewEvalCase{
			{ID: "skip", Path: "skip.md", Expected: "skip"},
		},
	})
	srv := newForgeReviewOllamaServer(t, "gemma2:9b", "DECISION: skip\nREASON: too vague")
	defer srv.Close()

	out, err := executeCommand(
		"forge", "review",
		"--sessions-dir", sessionsDir,
		"--eval", manifestPath,
		"--reviewer-model", "gemma2:9b",
		"--reviewer-endpoint", srv.URL,
		"--json",
	)
	if err != nil {
		t.Fatalf("forge review --eval --reviewer-model --json: %v\noutput:\n%s", err, out)
	}

	var report llm.ReviewEvalReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("parse JSON eval report: %v\noutput:\n%s", err, out)
	}
	if report.Passed != 1 || report.Results[0].Actual != "skip" {
		t.Fatalf("unexpected eval report: %+v", report)
	}
	data, err := os.ReadFile(filepath.Join(sessionsDir, "skip.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "status: reviewed") {
		t.Fatalf("eval with reviewer mutated session page:\n%s", string(data))
	}
}

func newForgeReviewOllamaServer(t *testing.T, wantModel, response string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, forgeReviewTagsResponse(wantModel, "sha256:reviewer"))
	})
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"model_info":{"gemma2.context_length":8192}}`)
	})
	mux.HandleFunc("/api/generate", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != wantModel {
			t.Errorf("model = %q, want %q", req.Model, wantModel)
		}
		if req.Stream {
			t.Errorf("stream = true, want false")
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"response": response})
	})
	return httptest.NewServer(mux)
}

func forgeReviewTagsResponse(model, digest string) string {
	body := map[string]any{
		"models": []map[string]any{
			{"name": model, "digest": digest},
		},
	}
	data, _ := json.Marshal(body)
	return string(data)
}

func writeForgeReviewEvalFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func writeForgeReviewEvalManifest(t *testing.T, path string, manifest llm.ReviewEvalManifest) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}
