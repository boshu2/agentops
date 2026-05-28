// practices: [property-based-testing, design-by-contract]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

// maturityOrder maps each maturity level to a numeric rank for comparison.
// Higher rank = more mature (except anti-pattern which is a lateral state).
var maturityOrder = map[types.Maturity]int{
	types.MaturityProvisional: 0,
	types.MaturityCandidate:   1,
	types.MaturityEstablished: 2,
	types.MaturityAntiPattern: -1, // lateral, not in the linear progression
}

// allNormalMaturities are the three maturity levels in the normal progression.
var allNormalMaturities = []types.Maturity{
	types.MaturityProvisional,
	types.MaturityCandidate,
	types.MaturityEstablished,
}

// writeLearningJSONL creates a JSONL learning file in a temp directory
// and registers cleanup. Works with both *testing.T and *rapid.T.
func writeLearningJSONL(t *rapid.T, data map[string]any) string {
	dir, err := os.MkdirTemp("", "maturity-prop-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	path := filepath.Join(dir, "test-learning.jsonl")
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal learning data: %v", err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatalf("write learning file: %v", err)
	}
	return path
}

// writeTestLearningJSONL creates a JSONL learning file using *testing.T (for non-rapid tests).
func writeTestLearningJSONL(t *testing.T, data map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "learning.jsonl")
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal learning data: %v", err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatalf("write learning file: %v", err)
	}
	return path
}

// TestPropertyMaturity_MonotonicPromotionTransitions verifies that when
// utility and feedback are sufficient for promotion, the maturity level
// only increases along the provisional -> candidate -> established path.
func TestPropertyMaturity_MonotonicPromotionTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	rapid.Check(t, func(t *rapid.T) {
		// Pick a starting maturity (provisional or candidate).
		startIdx := rapid.IntRange(0, 1).Draw(t, "start-maturity-idx")
		startMaturity := allNormalMaturities[startIdx]

		// Generate conditions sufficient for promotion.
		utility := rapid.Float64Range(types.MaturityPromotionThreshold, 1.0).Draw(t, "utility")
		rewardCount := rapid.IntRange(5, 20).Draw(t, "reward-count")
		helpfulCount := rapid.IntRange(3, rewardCount).Draw(t, "helpful-count")
		harmfulCount := rapid.IntRange(0, helpfulCount-1).Draw(t, "harmful-count")

		data := map[string]any{
			"id":            "test-learning",
			"maturity":      string(startMaturity),
			"utility":       utility,
			"confidence":    0.7,
			"reward_count":  rewardCount,
			"helpful_count": helpfulCount,
			"harmful_count": harmfulCount,
		}

		path := writeLearningJSONL(t, data)
		result, err := ratchet.CheckMaturityTransition(path)
		if err != nil {
			t.Fatalf("CheckMaturityTransition failed: %v", err)
		}

		// If a transition occurred, the new maturity must be strictly higher
		// on the normal progression (monotonic increase).
		if result.Transitioned {
			oldRank := maturityOrder[result.OldMaturity]
			newRank := maturityOrder[result.NewMaturity]
			// Anti-pattern transitions are not expected here since utility is high.
			if newRank < 0 {
				t.Fatalf("unexpected anti-pattern transition with utility=%.2f", utility)
			}
			if newRank <= oldRank {
				t.Fatalf("non-monotonic promotion: %s (rank %d) -> %s (rank %d)",
					result.OldMaturity, oldRank, result.NewMaturity, newRank)
			}
		}
	})
}

// TestPropertyMaturity_AntiPatternDetection verifies that low utility combined
// with sufficient harmful feedback always triggers the anti-pattern transition,
// regardless of the starting maturity level.
func TestPropertyMaturity_AntiPatternDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	rapid.Check(t, func(t *rapid.T) {
		// Pick any starting maturity.
		allMaturities := []types.Maturity{
			types.MaturityProvisional,
			types.MaturityCandidate,
			types.MaturityEstablished,
		}
		startIdx := rapid.IntRange(0, len(allMaturities)-1).Draw(t, "start-maturity-idx")
		startMaturity := allMaturities[startIdx]

		// Generate conditions that should trigger anti-pattern.
		utility := rapid.Float64Range(0.0, types.MaturityAntiPatternThreshold).Draw(t, "utility")
		harmfulCount := rapid.IntRange(types.MinFeedbackForAntiPattern, 20).Draw(t, "harmful-count")

		data := map[string]any{
			"id":            "test-learning",
			"maturity":      string(startMaturity),
			"utility":       utility,
			"confidence":    0.3,
			"reward_count":  harmfulCount,
			"helpful_count": 0,
			"harmful_count": harmfulCount,
		}

		path := writeLearningJSONL(t, data)
		result, err := ratchet.CheckMaturityTransition(path)
		if err != nil {
			t.Fatalf("CheckMaturityTransition failed: %v", err)
		}

		// Must transition to anti-pattern (or already be anti-pattern).
		if result.NewMaturity != types.MaturityAntiPattern {
			t.Fatalf("expected anti-pattern for utility=%.2f, harmful=%d, start=%s; got %s",
				utility, harmfulCount, startMaturity, result.NewMaturity)
		}
	})
}

// TestPropertyMaturity_DemotionOnLowUtility verifies that established and
// candidate learnings get demoted when utility drops below thresholds.
func TestPropertyMaturity_DemotionOnLowUtility(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	rapid.Check(t, func(t *rapid.T) {
		// Test established -> candidate demotion (utility < 0.5).
		// Ensure utility is above anti-pattern threshold to avoid anti-pattern path.
		utility := rapid.Float64Range(types.MaturityAntiPatternThreshold+0.01, 0.49).Draw(t, "utility")
		// Ensure harmful count < MinFeedbackForAntiPattern to avoid anti-pattern path.
		harmfulCount := rapid.IntRange(0, types.MinFeedbackForAntiPattern-1).Draw(t, "harmful-count")

		data := map[string]any{
			"id":            "test-learning",
			"maturity":      string(types.MaturityEstablished),
			"utility":       utility,
			"confidence":    0.5,
			"reward_count":  5,
			"helpful_count": 3,
			"harmful_count": harmfulCount,
		}

		path := writeLearningJSONL(t, data)
		result, err := ratchet.CheckMaturityTransition(path)
		if err != nil {
			t.Fatalf("CheckMaturityTransition failed: %v", err)
		}

		if !result.Transitioned {
			t.Fatalf("expected demotion for established with utility=%.2f, got no transition", utility)
		}
		if result.NewMaturity != types.MaturityCandidate {
			t.Fatalf("expected demotion to candidate, got %s", result.NewMaturity)
		}
	})
}

// TestPropertyMaturity_CandidateDemotionOnLowUtility verifies that candidate
// learnings get demoted to provisional when utility drops below demotion threshold.
func TestPropertyMaturity_CandidateDemotionOnLowUtility(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	rapid.Check(t, func(t *rapid.T) {
		// Utility below demotion threshold but above anti-pattern threshold.
		utility := rapid.Float64Range(types.MaturityAntiPatternThreshold+0.01, types.MaturityDemotionThreshold-0.01).Draw(t, "utility")
		// Ensure harmful count < MinFeedbackForAntiPattern to avoid anti-pattern path.
		harmfulCount := rapid.IntRange(0, types.MinFeedbackForAntiPattern-1).Draw(t, "harmful-count")

		data := map[string]any{
			"id":            "test-learning",
			"maturity":      string(types.MaturityCandidate),
			"utility":       utility,
			"confidence":    0.5,
			"reward_count":  5,
			"helpful_count": 3,
			"harmful_count": harmfulCount,
		}

		path := writeLearningJSONL(t, data)
		result, err := ratchet.CheckMaturityTransition(path)
		if err != nil {
			t.Fatalf("CheckMaturityTransition failed: %v", err)
		}

		if !result.Transitioned {
			t.Fatalf("expected demotion for candidate with utility=%.2f, got no transition", utility)
		}
		if result.NewMaturity != types.MaturityProvisional {
			t.Fatalf("expected demotion to provisional, got %s", result.NewMaturity)
		}
	})
}

// TestPropertyMaturity_AllLevelsReachable verifies that all defined maturity
// levels can be reached from the initial provisional state through valid
// transitions.
func TestPropertyMaturity_AllLevelsReachable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	// Candidate reachable from provisional.
	t.Run("provisional_to_candidate", func(t *testing.T) {
		data := map[string]any{
			"id":            "test-learning",
			"maturity":      string(types.MaturityProvisional),
			"utility":       0.7,
			"confidence":    0.6,
			"reward_count":  5,
			"helpful_count": 4,
			"harmful_count": 0,
		}
		path := writeTestLearningJSONL(t, data)

		result, err := ratchet.CheckMaturityTransition(path)
		if err != nil {
			t.Fatalf("CheckMaturityTransition: %v", err)
		}
		if !result.Transitioned || result.NewMaturity != types.MaturityCandidate {
			t.Fatalf("expected transition to candidate, got transitioned=%v new=%s",
				result.Transitioned, result.NewMaturity)
		}
	})

	// Established reachable from candidate.
	t.Run("candidate_to_established", func(t *testing.T) {
		data := map[string]any{
			"id":            "test-learning",
			"maturity":      string(types.MaturityCandidate),
			"utility":       0.8,
			"confidence":    0.7,
			"reward_count":  10,
			"helpful_count": 8,
			"harmful_count": 1,
		}
		path := writeTestLearningJSONL(t, data)

		result, err := ratchet.CheckMaturityTransition(path)
		if err != nil {
			t.Fatalf("CheckMaturityTransition: %v", err)
		}
		if !result.Transitioned || result.NewMaturity != types.MaturityEstablished {
			t.Fatalf("expected transition to established, got transitioned=%v new=%s",
				result.Transitioned, result.NewMaturity)
		}
	})

	// Anti-pattern reachable from any state.
	t.Run("any_to_anti_pattern", func(t *testing.T) {
		for _, start := range allNormalMaturities {
			t.Run(string(start), func(t *testing.T) {
				data := map[string]any{
					"id":            "test-learning",
					"maturity":      string(start),
					"utility":       0.1,
					"confidence":    0.3,
					"reward_count":  5,
					"helpful_count": 0,
					"harmful_count": 5,
				}
				path := writeTestLearningJSONL(t, data)

				result, err := ratchet.CheckMaturityTransition(path)
				if err != nil {
					t.Fatalf("CheckMaturityTransition: %v", err)
				}
				if !result.Transitioned || result.NewMaturity != types.MaturityAntiPattern {
					t.Fatalf("expected transition to anti-pattern from %s, got transitioned=%v new=%s",
						start, result.Transitioned, result.NewMaturity)
				}
			})
		}
	})

	// Rehabilitation: anti-pattern back to provisional.
	t.Run("anti_pattern_to_provisional", func(t *testing.T) {
		data := map[string]any{
			"id":            "test-learning",
			"maturity":      string(types.MaturityAntiPattern),
			"utility":       0.7,
			"confidence":    0.6,
			"reward_count":  10,
			"helpful_count": 8,
			"harmful_count": 3,
		}
		path := writeTestLearningJSONL(t, data)

		result, err := ratchet.CheckMaturityTransition(path)
		if err != nil {
			t.Fatalf("CheckMaturityTransition: %v", err)
		}
		if !result.Transitioned || result.NewMaturity != types.MaturityProvisional {
			t.Fatalf("expected rehabilitation to provisional, got transitioned=%v new=%s",
				result.Transitioned, result.NewMaturity)
		}
	})
}

// TestPropertyMaturity_StableWhenNoTransition verifies that when conditions
// do not meet any transition criteria, the maturity level stays the same.
func TestPropertyMaturity_StableWhenNoTransition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	rapid.Check(t, func(t *rapid.T) {
		// Provisional with low feedback: should stay provisional.
		utility := rapid.Float64Range(types.MaturityDemotionThreshold, types.MaturityPromotionThreshold-0.01).Draw(t, "utility")
		rewardCount := rapid.IntRange(0, types.MinFeedbackForPromotion-1).Draw(t, "reward-count")
		harmfulCount := rapid.IntRange(0, types.MinFeedbackForAntiPattern-1).Draw(t, "harmful-count")

		data := map[string]any{
			"id":            "test-learning",
			"maturity":      string(types.MaturityProvisional),
			"utility":       utility,
			"confidence":    0.5,
			"reward_count":  rewardCount,
			"helpful_count": rewardCount,
			"harmful_count": harmfulCount,
		}

		path := writeLearningJSONL(t, data)
		result, err := ratchet.CheckMaturityTransition(path)
		if err != nil {
			t.Fatalf("CheckMaturityTransition failed: %v", err)
		}

		if result.Transitioned {
			t.Fatalf("expected no transition for provisional with utility=%.2f, rewards=%d; got %s -> %s",
				utility, rewardCount, result.OldMaturity, result.NewMaturity)
		}
		if result.OldMaturity != result.NewMaturity {
			t.Fatalf("maturity changed without transition flag: %s -> %s",
				result.OldMaturity, result.NewMaturity)
		}
	})
}
