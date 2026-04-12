package llm

import (
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
