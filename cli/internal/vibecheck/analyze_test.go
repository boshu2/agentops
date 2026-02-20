package vibecheck

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestAnalyze tests the Analyze orchestrator with a temporary git repo.
func TestAnalyze(t *testing.T) {
	// Create a temporary directory for our test git repo
	tmpDir, err := os.MkdirTemp("", "vibecheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repository
	if err := initGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Create some test commits with varied characteristics
	testTime := time.Now().Add(-24 * time.Hour) // 24 hours ago

	if err := createTestCommit(tmpDir, "file1.txt", "Initial commit", testTime); err != nil {
		t.Fatalf("failed to create commit 1: %v", err)
	}

	testTime = testTime.Add(6 * time.Hour)
	if err := createTestCommit(tmpDir, "file2.txt", "Add feature", testTime); err != nil {
		t.Fatalf("failed to create commit 2: %v", err)
	}

	testTime = testTime.Add(6 * time.Hour)
	if err := createTestCommit(tmpDir, "file1.txt", "Fix bug", testTime); err != nil {
		t.Fatalf("failed to create commit 3: %v", err)
	}

	// Run Analyze with a since time 48 hours ago
	opts := AnalyzeOptions{
		RepoPath: tmpDir,
		Since:    time.Now().Add(-48 * time.Hour),
	}

	result, err := Analyze(opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Verify result structure
	if result == nil {
		t.Fatal("result is nil")
		return
	}

	// Score should be between 0 and 100
	if result.Score < 0 || result.Score > 100 {
		t.Errorf("score out of range: %f", result.Score)
	}

	// Grade should be one of A, B, C, D, F
	validGrades := map[string]bool{"A": true, "B": true, "C": true, "D": true, "F": true}
	if !validGrades[result.Grade] {
		t.Errorf("invalid grade: %s", result.Grade)
	}

	// Should have events from the commits
	if len(result.Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(result.Events))
	}

	// Should have metrics
	expectedMetrics := map[string]bool{
		"velocity": true,
		"rework":   true,
		"trust":    true,
		"spirals":  true,
		"flow":     true,
	}
	for metricName := range expectedMetrics {
		if _, ok := result.Metrics[metricName]; !ok {
			t.Errorf("missing metric: %s", metricName)
		}
	}

	// Findings slice should be present and not nil
	if result.Findings == nil {
		t.Error("Findings is nil, should be a slice (possibly empty)")
	}
}

// TestAnalyzeEmptyRepo tests Analyze on a repo with no commits in the timeframe.
func TestAnalyzeEmptyRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vibecheck-empty-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := initGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Create a commit far in the past
	oldTime := time.Now().Add(-30 * 24 * time.Hour)
	if err := createTestCommit(tmpDir, "file.txt", "Old commit", oldTime); err != nil {
		t.Fatalf("failed to create old commit: %v", err)
	}

	// Analyze with a recent since time (should get no commits)
	opts := AnalyzeOptions{
		RepoPath: tmpDir,
		Since:    time.Now().Add(-1 * time.Hour),
	}

	result, err := Analyze(opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should have no events
	if len(result.Events) != 0 {
		t.Errorf("expected 0 events, got %d", len(result.Events))
	}

	// Score should be valid (metrics are still computed even with no events)
	if result.Score < 0 || result.Score > 100 {
		t.Errorf("score out of range: %f", result.Score)
	}

	// Grade should be valid
	validGrades := map[string]bool{"A": true, "B": true, "C": true, "D": true, "F": true}
	if !validGrades[result.Grade] {
		t.Errorf("invalid grade: %s", result.Grade)
	}
}

// TestAnalyzeMissingRepoPath tests that Analyze fails with empty RepoPath.
func TestAnalyzeMissingRepoPath(t *testing.T) {
	opts := AnalyzeOptions{
		RepoPath: "",
		Since:    time.Now(),
	}

	result, err := Analyze(opts)
	if err == nil {
		t.Fatal("expected error for missing RepoPath, got nil")
	}
	if result != nil {
		t.Fatal("expected nil result for error case")
	}
}

// TestAnalyzeInvalidRepo tests that Analyze fails with invalid repo path.
func TestAnalyzeInvalidRepo(t *testing.T) {
	opts := AnalyzeOptions{
		RepoPath: "/nonexistent/repo/path",
		Since:    time.Now(),
	}

	result, err := Analyze(opts)
	if err == nil {
		t.Fatal("expected error for invalid repo, got nil")
	}
	if result != nil {
		t.Fatal("expected nil result for error case")
	}
}

// Helper: initGitRepo initializes a git repository in the given directory.
func initGitRepo(repoPath string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}

	// Set git config for this repo (required for commits)
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config email failed: %w", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config name failed: %w", err)
	}

	return nil
}

// Helper: createTestCommit creates a commit in the given repo.
func createTestCommit(repoPath, filename, message string, timestamp time.Time) error {
	// Create/update file
	filePath := filepath.Join(repoPath, filename)
	content := fmt.Sprintf("Content for %s at %s\n", filename, timestamp.String())
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Stage file
	cmd := exec.Command("git", "add", filename)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// Commit with specific timestamp
	env := os.Environ()
	env = append(env, fmt.Sprintf("GIT_AUTHOR_DATE=%s", timestamp.Format(time.RFC3339)))
	env = append(env, fmt.Sprintf("GIT_COMMITTER_DATE=%s", timestamp.Format(time.RFC3339)))

	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoPath
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	return nil
}
