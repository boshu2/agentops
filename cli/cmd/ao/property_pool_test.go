// practices: [property-based-testing, design-by-contract]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/types"
)

// rapidTempDir creates a temporary directory and registers cleanup on the rapid.T.
func rapidTempDir(t *rapid.T) string {
	dir, err := os.MkdirTemp("", "pool-prop-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// genCandidateID generates a valid pool candidate ID (alphanumeric + hyphens).
func genCandidateID() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		prefix := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "prefix")
		suffix := rapid.StringMatching(`[a-z0-9]{3,8}`).Draw(t, "suffix")
		return prefix + "-" + suffix
	})
}

// genTier picks a random valid tier.
func genTier() *rapid.Generator[types.Tier] {
	return rapid.Custom(func(t *rapid.T) types.Tier {
		tiers := []types.Tier{types.TierGold, types.TierSilver, types.TierBronze, types.TierDiscard}
		idx := rapid.IntRange(0, len(tiers)-1).Draw(t, "tier-idx")
		return tiers[idx]
	})
}

// genCandidate generates a minimal valid Candidate suitable for pool.Add.
func genCandidate() *rapid.Generator[types.Candidate] {
	return rapid.Custom(func(t *rapid.T) types.Candidate {
		id := genCandidateID().Draw(t, "id")
		tier := genTier().Draw(t, "tier")
		rawScore := rapid.Float64Range(0.0, 1.0).Draw(t, "raw-score")
		content := rapid.StringMatching(`[A-Za-z0-9 ]{10,80}`).Draw(t, "content")

		return types.Candidate{
			ID:           id,
			Type:         types.KnowledgeTypeLearning,
			Content:      content,
			RawScore:     rawScore,
			Tier:         tier,
			ExtractedAt:  time.Now(),
			IsCurrent:    true,
			ExpiryStatus: types.ExpiryStatusActive,
			Utility:      types.InitialUtility,
			Maturity:     types.MaturityProvisional,
		}
	})
}

// genScoring generates a minimal valid Scoring struct.
func genScoring() *rapid.Generator[types.Scoring] {
	return rapid.Custom(func(t *rapid.T) types.Scoring {
		raw := rapid.Float64Range(0.0, 1.0).Draw(t, "raw-score")
		tier := genTier().Draw(t, "tier")
		return types.Scoring{
			RawScore:       raw,
			TierAssignment: tier,
			Rubric: types.RubricScores{
				Specificity:   rapid.Float64Range(0.0, 1.0).Draw(t, "specificity"),
				Actionability: rapid.Float64Range(0.0, 1.0).Draw(t, "actionability"),
				Novelty:       rapid.Float64Range(0.0, 1.0).Draw(t, "novelty"),
				Context:       rapid.Float64Range(0.0, 1.0).Draw(t, "context"),
				Confidence:    rapid.Float64Range(0.0, 1.0).Draw(t, "confidence"),
			},
			GateRequired: rapid.Bool().Draw(t, "gate-required"),
			ScoredAt:     time.Now(),
		}
	})
}

// TestPropertyPoolIngest_Idempotent verifies that adding the same candidate
// twice to a pool produces an idempotent result: the pool state after the
// second add is identical to the state after the first.
func TestPropertyPoolIngest_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	rapid.Check(t, func(t *rapid.T) {
		tmp := rapidTempDir(t)
		p := pool.NewPool(tmp)

		cand := genCandidate().Draw(t, "candidate")
		scoring := genScoring().Draw(t, "scoring")

		// First add: should succeed.
		err1 := p.Add(cand, scoring)
		if err1 != nil {
			t.Fatalf("first Add failed: %v", err1)
		}

		// Read pool state after first add.
		entry1, err := p.Get(cand.ID)
		if err != nil {
			t.Fatalf("Get after first Add failed: %v", err)
		}

		// Second add of the exact same candidate: the pool stores by filename
		// (candidate.ID + ".json"), so writing again overwrites the same file.
		// This should succeed and produce the same ID in the pool.
		err2 := p.Add(cand, scoring)
		if err2 != nil {
			t.Fatalf("second Add failed: %v", err2)
		}

		// Read pool state after second add.
		entry2, err := p.Get(cand.ID)
		if err != nil {
			t.Fatalf("Get after second Add failed: %v", err)
		}

		// Core idempotency: the candidate data is identical.
		if entry1.Candidate.ID != entry2.Candidate.ID {
			t.Fatalf("ID changed: %q vs %q", entry1.Candidate.ID, entry2.Candidate.ID)
		}
		if entry1.Candidate.Content != entry2.Candidate.Content {
			t.Fatalf("Content changed after re-add")
		}
		if entry1.Candidate.Tier != entry2.Candidate.Tier {
			t.Fatalf("Tier changed: %q vs %q", entry1.Candidate.Tier, entry2.Candidate.Tier)
		}
		if entry1.Status != entry2.Status {
			t.Fatalf("Status changed: %q vs %q", entry1.Status, entry2.Status)
		}

		// Verify only one file exists for this candidate (not duplicated).
		pendingDir := filepath.Join(tmp, pool.PoolDir, pool.PendingDir)
		files, err := os.ReadDir(pendingDir)
		if err != nil {
			t.Fatalf("read pending dir: %v", err)
		}
		count := 0
		expectedFilename := cand.ID + ".json"
		for _, f := range files {
			if f.Name() == expectedFilename {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("expected exactly 1 file for ID %q, got %d", cand.ID, count)
		}
	})
}

// TestPropertyPoolIngest_MonotonicGrowth verifies that each successful Add
// call increases the total pool item count by exactly one (monotonic growth).
func TestPropertyPoolIngest_MonotonicGrowth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	rapid.Check(t, func(t *rapid.T) {
		tmp := rapidTempDir(t)
		p := pool.NewPool(tmp)

		// Generate a batch of candidates with unique IDs.
		n := rapid.IntRange(1, 10).Draw(t, "batch-size")
		ids := make(map[string]bool)

		prevCount := 0

		for i := 0; i < n; i++ {
			// Generate unique candidates by ensuring no duplicate IDs.
			var cand types.Candidate
			for {
				cand = genCandidate().Draw(t, fmt.Sprintf("candidate-%d", i))
				if !ids[cand.ID] {
					ids[cand.ID] = true
					break
				}
			}
			scoring := genScoring().Draw(t, fmt.Sprintf("scoring-%d", i))

			err := p.Add(cand, scoring)
			if err != nil {
				t.Fatalf("Add #%d failed: %v", i, err)
			}

			// Count files in pending directory.
			pendingDir := filepath.Join(tmp, pool.PoolDir, pool.PendingDir)
			files, err := os.ReadDir(pendingDir)
			if err != nil {
				t.Fatalf("read pending dir after Add #%d: %v", i, err)
			}
			jsonCount := 0
			for _, f := range files {
				if !f.IsDir() && filepath.Ext(f.Name()) == ".json" {
					jsonCount++
				}
			}

			// Monotonic: count must be strictly greater than previous.
			if jsonCount <= prevCount {
				t.Fatalf("monotonic growth violated: after Add #%d, count=%d <= prev=%d",
					i, jsonCount, prevCount)
			}
			// Exactly one more than previous.
			if jsonCount != prevCount+1 {
				t.Fatalf("expected count=%d after Add #%d, got %d", prevCount+1, i, jsonCount)
			}

			prevCount = jsonCount
		}
	})
}

// TestPropertyPoolIngest_GetRoundTrip verifies that any candidate added to the
// pool can be retrieved with the same core data.
func TestPropertyPoolIngest_GetRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	rapid.Check(t, func(t *rapid.T) {
		tmp := rapidTempDir(t)
		p := pool.NewPool(tmp)

		cand := genCandidate().Draw(t, "candidate")
		scoring := genScoring().Draw(t, "scoring")

		if err := p.Add(cand, scoring); err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		entry, err := p.Get(cand.ID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		// Verify the round-trip preserves core fields.
		if entry.Candidate.ID != cand.ID {
			t.Fatalf("ID mismatch: got %q, want %q", entry.Candidate.ID, cand.ID)
		}
		if entry.Candidate.Content != cand.Content {
			t.Fatalf("Content mismatch")
		}
		if entry.Candidate.Type != cand.Type {
			t.Fatalf("Type mismatch: got %q, want %q", entry.Candidate.Type, cand.Type)
		}
		if entry.Candidate.Tier != cand.Tier {
			t.Fatalf("Tier mismatch: got %q, want %q", entry.Candidate.Tier, cand.Tier)
		}
		if entry.Status != types.PoolStatusPending {
			t.Fatalf("Status mismatch: got %q, want %q", entry.Status, types.PoolStatusPending)
		}
	})
}
