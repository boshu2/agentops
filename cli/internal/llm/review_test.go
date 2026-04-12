package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const draftPage = `---
type: session
session_id: test-review
status: draft
tier: 1
confidence: 0.85
---

# Session notes

### Implement feature

Summary of the work done.

**Entities:**
- [[file:foo.go]]

**Assistant:** The assistant implemented the feature.
`

const alreadyReviewed = `---
type: session
session_id: test-already
status: reviewed
tier: 1
confidence: 0.85
---

# Session notes

### Fix bug

Summary.

**Entities:**
- [[file:bar.go]]

**Assistant:** Fixed.
`

const lowConfidence = `---
type: session
session_id: test-low
status: draft
tier: 1
confidence: 0.01
---

# Session notes

### Chunk 0 — SKIP
`

func TestReviewDraftSessions_PromotesDraft(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s1.md"), []byte(draftPage), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ReviewDraftSessions(ReviewOptions{SessionsDir: dir})
	if err != nil {
		t.Fatalf("ReviewDraftSessions: %v", err)
	}
	if result.Reviewed != 1 {
		t.Errorf("Reviewed: want 1, got %d", result.Reviewed)
	}
	// Verify the file was rewritten with status:reviewed.
	b, _ := os.ReadFile(filepath.Join(dir, "s1.md"))
	if !strings.Contains(string(b), "status: reviewed") {
		t.Errorf("page not promoted:\n%s", string(b))
	}
	if !strings.Contains(string(b), "reviewed_by: ao-forge-tier2-structural") {
		t.Errorf("missing reviewed_by:\n%s", string(b))
	}
	if !strings.Contains(string(b), "reviewed_at:") {
		t.Errorf("missing reviewed_at:\n%s", string(b))
	}
}

func TestReviewDraftSessions_SkipsAlreadyReviewed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s1.md"), []byte(alreadyReviewed), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ReviewDraftSessions(ReviewOptions{SessionsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if result.Reviewed != 0 {
		t.Errorf("should skip already reviewed, got Reviewed=%d", result.Reviewed)
	}
}

func TestReviewDraftSessions_SkipsLowConfidence(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s1.md"), []byte(lowConfidence), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ReviewDraftSessions(ReviewOptions{SessionsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if result.Reviewed != 0 {
		t.Errorf("should skip low confidence, got Reviewed=%d", result.Reviewed)
	}
	// Original content should be unchanged.
	b, _ := os.ReadFile(filepath.Join(dir, "s1.md"))
	if !strings.Contains(string(b), "status: draft") {
		t.Errorf("low confidence page was modified")
	}
}

func TestReviewDraftSessions_DryRun(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s1.md"), []byte(draftPage), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ReviewDraftSessions(ReviewOptions{SessionsDir: dir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Reviewed != 1 {
		t.Errorf("DryRun should count as reviewed, got %d", result.Reviewed)
	}
	// File should be unchanged in dry-run mode.
	b, _ := os.ReadFile(filepath.Join(dir, "s1.md"))
	if strings.Contains(string(b), "status: reviewed") {
		t.Errorf("dry run modified the file")
	}
}

func TestReviewDraftSessions_EmptyDir(t *testing.T) {
	result, err := ReviewDraftSessions(ReviewOptions{SessionsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if result.Reviewed != 0 || result.Skipped != 0 {
		t.Errorf("empty dir: Reviewed=%d Skipped=%d", result.Reviewed, result.Skipped)
	}
}

func TestReviewDraftSessions_MissingDir(t *testing.T) {
	result, err := ReviewDraftSessions(ReviewOptions{SessionsDir: "/nonexistent/path"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Reviewed != 0 {
		t.Errorf("missing dir should return empty result")
	}
}

func TestEvaluateReviewDraftSessions_ManifestDecisions(t *testing.T) {
	dir := t.TempDir()
	writeReviewTestFile(t, dir, "promote.md", draftPage)
	writeReviewTestFile(t, dir, "skip-low-confidence.md", lowConfidence)

	manifestPath := filepath.Join(t.TempDir(), "review-eval.json")
	writeReviewEvalManifest(t, manifestPath, ReviewEvalManifest{
		ID: "fixture-review-eval",
		Cases: []ReviewEvalCase{
			{
				ID:       "promote-draft",
				Path:     "promote.md",
				Expected: "promote",
				Reason:   "complete draft with confidence above threshold",
			},
			{
				ID:       "skip-low-confidence",
				Path:     "skip-low-confidence.md",
				Expected: "skip",
				Reason:   "low-confidence summary should remain draft",
			},
		},
	})

	report, err := EvaluateReviewDraftSessions(ReviewEvalOptions{
		SessionsDir:  dir,
		ManifestPath: manifestPath,
	})
	if err != nil {
		t.Fatalf("EvaluateReviewDraftSessions: %v", err)
	}
	if report.Cases != 2 || report.Passed != 2 || report.Failed != 0 {
		t.Fatalf("unexpected report summary: %+v", report)
	}
	if report.Accuracy != 1 {
		t.Fatalf("accuracy = %.2f, want 1", report.Accuracy)
	}
	if report.Results[0].Path != "promote.md" || report.Results[0].Actual != "promote" {
		t.Fatalf("promote result = %+v", report.Results[0])
	}

	b, err := os.ReadFile(filepath.Join(dir, "promote.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "status: reviewed") {
		t.Fatalf("eval mutated promoted fixture:\n%s", string(b))
	}
}

func TestEvaluateReviewDraftSessions_RecordsMissingCaseAsFailure(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), "review-eval.json")
	writeReviewEvalManifest(t, manifestPath, ReviewEvalManifest{
		ID: "missing-page-eval",
		Cases: []ReviewEvalCase{
			{ID: "missing", Path: "missing.md", Expected: "promote"},
		},
	})

	report, err := EvaluateReviewDraftSessions(ReviewEvalOptions{
		SessionsDir:  t.TempDir(),
		ManifestPath: manifestPath,
	})
	if err != nil {
		t.Fatalf("EvaluateReviewDraftSessions: %v", err)
	}
	if report.Passed != 0 || report.Failed != 1 || report.Errors != 1 {
		t.Fatalf("unexpected missing-page report: %+v", report)
	}
	if report.Results[0].ErrorMessage == "" {
		t.Fatalf("missing-page result should include error: %+v", report.Results[0])
	}
}

func TestLoadReviewEvalManifest_RequiresExpectedDecision(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), "review-eval.json")
	writeReviewEvalManifest(t, manifestPath, ReviewEvalManifest{
		ID: "invalid-review-eval",
		Cases: []ReviewEvalCase{
			{ID: "missing-expected", Path: "page.md"},
		},
	})

	if _, err := LoadReviewEvalManifest(manifestPath); err == nil {
		t.Fatal("LoadReviewEvalManifest succeeded, want missing expected decision error")
	}
}

func writeReviewTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func writeReviewEvalManifest(t *testing.T, path string, manifest ReviewEvalManifest) {
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
