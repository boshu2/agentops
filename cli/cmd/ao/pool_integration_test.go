// practices: [microservices, sre]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/types"
)

// createTestPoolCandidate writes a pool candidate JSON file into the correct
// pool directory (.agents/pool/<status>/).  The pool package constant
// pool.PoolDir == ".agents/pool" is the source of truth.
func createTestPoolCandidate(t *testing.T, dir string, id string, tier types.Tier, utility float64) {
	t.Helper()
	createTestPoolCandidateWithStatus(t, dir, id, tier, utility, types.PoolStatusPending)
}

// createTestPoolCandidateWithStatus writes a candidate into the given status directory.
func createTestPoolCandidateWithStatus(t *testing.T, dir string, id string, tier types.Tier, utility float64, status types.PoolStatus) {
	t.Helper()

	entry := types.PoolEntry{
		Candidate: types.Candidate{
			ID:      id,
			Type:    "learning",
			Content: "Test learning content for " + id,
			Tier:    tier,
			Source: types.Source{
				SessionID:      "test-session-001",
				TranscriptPath: "transcripts/test.md",
				MessageIndex:   1,
			},
			Utility:    utility,
			Confidence: 0.8,
			Maturity:   "validated",
		},
		ScoringResult: types.Scoring{
			RawScore:       utility,
			TierAssignment: tier,
			Rubric: types.RubricScores{
				Specificity:   0.7,
				Actionability: 0.8,
				Novelty:       0.6,
				Context:       0.7,
				Confidence:    0.8,
			},
			ScoredAt: time.Now().Add(-47 * time.Hour),
		},
		Status:  status,
		AddedAt: time.Now().Add(-48 * time.Hour),
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	// Use the canonical pool directory from the pool package.
	statusDir := statusToDir(status)
	targetDir := filepath.Join(dir, pool.PoolDir, statusDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(targetDir, id+".json"), string(data))
}

// statusToDir maps a PoolStatus to its directory name inside .agents/pool/.
func statusToDir(s types.PoolStatus) string {
	switch s {
	case types.PoolStatusPending:
		return pool.PendingDir
	case types.PoolStatusStaged:
		return pool.StagedDir
	case types.PoolStatusRejected:
		return pool.RejectedDir
	default:
		return pool.PendingDir
	}
}

// ensurePoolDirs creates all pool status directories.
func ensurePoolDirs(t *testing.T, dir string) {
	t.Helper()
	for _, sub := range []string{pool.PendingDir, pool.StagedDir, pool.RejectedDir, pool.ValidatedDir} {
		if err := os.MkdirAll(filepath.Join(dir, pool.PoolDir, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
}

// resetPoolFlags clears global pool flag state that leaks between subtests
// because cobra binds flags to package-level vars.
func resetPoolFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		poolTier = ""
		poolStatus = ""
		poolLimit = 50
		poolOffset = 0
		poolReason = ""
		poolThreshold = defaultAutoPromoteThreshold
		poolDoPromote = false
		poolGold = true
		poolWide = false
	})
}

// --- Empty pool ---

func TestPoolList_Integration_EmptyPool(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	ensurePoolDirs(t, dir)
	resetPoolFlags(t)

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"pool", "list"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("pool list failed: %v", err)
	}

	if !strings.Contains(out, "No pool entries") {
		t.Errorf("expected 'No pool entries' for empty pool, got:\n%s", out)
	}
}

// --- List with candidates ---

func TestPoolList_Integration_WithCandidates(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	ensurePoolDirs(t, dir)
	resetPoolFlags(t)

	ids := make([]string, 3)
	for i := 0; i < 3; i++ {
		ids[i] = fmt.Sprintf("pend-2026-01-15-test%03d", i)
		createTestPoolCandidate(t, dir, ids[i], types.TierSilver, 0.7+float64(i)*0.05)
	}

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"pool", "list"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("pool list failed: %v", err)
	}

	// Table must contain headers
	if !strings.Contains(out, "ID") || !strings.Contains(out, "TIER") {
		t.Errorf("expected table headers (ID, TIER) in output, got:\n%s", out)
	}

	// Table truncates IDs (default max 12 chars → "pend-2026...").
	// Verify the common prefix appears at least once.
	if !strings.Contains(out, "pend-2026") {
		t.Errorf("expected truncated candidate IDs in output, got:\n%s", out)
	}

	// All 3 candidates should produce 3 data rows with the tier label
	if strings.Count(out, "silver") < 3 {
		t.Errorf("expected 3 silver entries in output, got:\n%s", out)
	}
}

// --- List with tier filter ---

func TestPoolList_Integration_TierFilter(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	ensurePoolDirs(t, dir)
	resetPoolFlags(t)

	createTestPoolCandidate(t, dir, "pend-gold-001", types.TierGold, 0.95)
	createTestPoolCandidate(t, dir, "pend-bronze-001", types.TierBronze, 0.55)

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"pool", "list", "--tier=gold"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("pool list --tier=gold failed: %v", err)
	}

	// Table truncates IDs; check for "gold" tier label and absence of "bronze"
	if !strings.Contains(out, "gold") {
		t.Errorf("expected gold tier in filtered output, got:\n%s", out)
	}
	if strings.Contains(out, "bronze") {
		t.Errorf("bronze candidate should be filtered out, got:\n%s", out)
	}
}

// --- Stage (actual, not dry-run) ---

func TestPoolStage_Integration(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	ensurePoolDirs(t, dir)
	resetPoolFlags(t)

	candidateID := "pend-2026-01-15-stage001"
	createTestPoolCandidate(t, dir, candidateID, types.TierSilver, 0.85)

	// Verify candidate exists in pending before staging
	pendingFile := filepath.Join(dir, pool.PoolDir, pool.PendingDir, candidateID+".json")
	if _, err := os.Stat(pendingFile); err != nil {
		t.Fatalf("candidate file should exist in pending before stage: %v", err)
	}

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"pool", "stage", candidateID})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("pool stage failed: %v", err)
	}

	if !strings.Contains(out, "Staged") || !strings.Contains(out, candidateID) {
		t.Errorf("expected 'Staged: <id>' output, got:\n%s", out)
	}

	// File should now be in staged, not pending
	stagedFile := filepath.Join(dir, pool.PoolDir, pool.StagedDir, candidateID+".json")
	if _, err := os.Stat(stagedFile); err != nil {
		t.Errorf("candidate should exist in staged dir after staging: %v", err)
	}
	if _, err := os.Stat(pendingFile); err == nil {
		t.Errorf("candidate should no longer exist in pending dir after staging")
	}
}

// --- Promote (requires staged candidate) ---

func TestPoolPromote_Integration(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	ensurePoolDirs(t, dir)
	resetPoolFlags(t)

	candidateID := "staged-2026-01-15-promote001"
	// Create directly as staged so promote can work
	createTestPoolCandidateWithStatus(t, dir, candidateID, types.TierGold, 0.95, types.PoolStatusStaged)

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"pool", "promote", candidateID})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("pool promote failed: %v", err)
	}

	if !strings.Contains(out, "Promoted") || !strings.Contains(out, candidateID) {
		t.Errorf("expected 'Promoted: <id>' output, got:\n%s", out)
	}

	if !strings.Contains(out, "Artifact:") {
		t.Errorf("expected 'Artifact:' path in output, got:\n%s", out)
	}

	// Staged file should be removed after promotion
	stagedFile := filepath.Join(dir, pool.PoolDir, pool.StagedDir, candidateID+".json")
	if _, err := os.Stat(stagedFile); err == nil {
		t.Errorf("staged file should be removed after promotion")
	}

	// Artifact should exist in learnings dir
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		t.Fatalf("expected learnings dir to exist after promotion: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one artifact in learnings dir after promotion")
	}
}

// --- Reject (actual, not dry-run) ---

func TestPoolReject_Integration(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	ensurePoolDirs(t, dir)
	resetPoolFlags(t)

	candidateID := "pend-2026-01-15-reject001"
	createTestPoolCandidate(t, dir, candidateID, types.TierBronze, 0.3)

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"pool", "reject", candidateID, "--reason", "Too vague for production use"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("pool reject failed: %v", err)
	}

	if !strings.Contains(out, "Rejected") || !strings.Contains(out, candidateID) {
		t.Errorf("expected 'Rejected: <id>' output, got:\n%s", out)
	}

	// File should now be in rejected, not pending
	rejectedFile := filepath.Join(dir, pool.PoolDir, pool.RejectedDir, candidateID+".json")
	if _, err := os.Stat(rejectedFile); err != nil {
		t.Errorf("candidate should exist in rejected dir: %v", err)
	}
	pendingFile := filepath.Join(dir, pool.PoolDir, pool.PendingDir, candidateID+".json")
	if _, err := os.Stat(pendingFile); err == nil {
		t.Errorf("candidate should no longer exist in pending dir after rejection")
	}
}

// --- Promote rejects non-staged candidates ---

func TestPoolPromote_Integration_RejectsNonStaged(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	ensurePoolDirs(t, dir)
	resetPoolFlags(t)

	candidateID := "pend-2026-01-15-notstaged"
	createTestPoolCandidate(t, dir, candidateID, types.TierGold, 0.95)

	_, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"pool", "promote", candidateID})
		return rootCmd.Execute()
	})

	if err == nil {
		t.Fatal("expected error when promoting a pending (non-staged) candidate, got nil")
	}

	if !strings.Contains(err.Error(), "must be staged") {
		t.Errorf("expected 'must be staged' in error, got: %v", err)
	}
}

// --- Stage-then-promote lifecycle ---

func TestPoolStageAndPromote_Integration_Lifecycle(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	ensurePoolDirs(t, dir)
	resetPoolFlags(t)

	candidateID := "pend-2026-01-15-lifecycle"
	createTestPoolCandidate(t, dir, candidateID, types.TierSilver, 0.80)

	// Stage
	_, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"pool", "stage", candidateID})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("stage failed: %v", err)
	}

	// Promote
	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"pool", "promote", candidateID})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("promote after stage failed: %v", err)
	}

	if !strings.Contains(out, "Promoted") {
		t.Errorf("expected 'Promoted' in output, got:\n%s", out)
	}

	// Verify candidate is gone from pool dirs
	for _, sub := range []string{pool.PendingDir, pool.StagedDir} {
		f := filepath.Join(dir, pool.PoolDir, sub, candidateID+".json")
		if _, statErr := os.Stat(f); statErr == nil {
			t.Errorf("candidate should not exist in %s after full lifecycle", sub)
		}
	}
}
