package main

import (
	"encoding/json"
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
