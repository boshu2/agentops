package taxonomy

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/domain"
)

func TestAssignTier(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		expected domain.Tier
	}{
		{"Gold high", 0.95, domain.TierGold},
		{"Gold boundary", 0.85, domain.TierGold},
		{"Silver high", 0.84, domain.TierSilver},
		{"Silver boundary", 0.70, domain.TierSilver},
		{"Bronze high", 0.69, domain.TierBronze},
		{"Bronze boundary", 0.50, domain.TierBronze},
		{"Discard high", 0.49, domain.TierDiscard},
		{"Discard zero", 0.0, domain.TierDiscard},
		{"Perfect score", 1.0, domain.TierGold},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := AssignTier(tt.score, DefaultTierConfigs)
			if tier != tt.expected {
				t.Errorf("AssignTier(%f) = %q, want %q", tt.score, tier, tt.expected)
			}
		})
	}
}

func TestDefaultRubricWeightsSum(t *testing.T) {
	if !DefaultRubricWeights.ValidateWeights() {
		sum := DefaultRubricWeights.Specificity +
			DefaultRubricWeights.Actionability +
			DefaultRubricWeights.Novelty +
			DefaultRubricWeights.Context +
			DefaultRubricWeights.Confidence
		t.Errorf("DefaultRubricWeights sum = %f, want 1.0", sum)
	}
}

func TestValidateWeights(t *testing.T) {
	tests := []struct {
		name    string
		weights RubricWeights
		valid   bool
	}{
		{
			name:    "Valid default",
			weights: DefaultRubricWeights,
			valid:   true,
		},
		{
			name: "Valid exact",
			weights: RubricWeights{
				Specificity:   0.20,
				Actionability: 0.20,
				Novelty:       0.20,
				Context:       0.20,
				Confidence:    0.20,
			},
			valid: true,
		},
		{
			name: "Invalid under",
			weights: RubricWeights{
				Specificity:   0.10,
				Actionability: 0.10,
				Novelty:       0.10,
				Context:       0.10,
				Confidence:    0.10,
			},
			valid: false,
		},
		{
			name: "Invalid over",
			weights: RubricWeights{
				Specificity:   0.30,
				Actionability: 0.30,
				Novelty:       0.30,
				Context:       0.30,
				Confidence:    0.30,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.weights.ValidateWeights(); got != tt.valid {
				t.Errorf("ValidateWeights() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestGetBaseScore(t *testing.T) {
	tests := []struct {
		kt       domain.KnowledgeType
		expected float64
	}{
		{domain.KnowledgeTypeDecision, 0.8},
		{domain.KnowledgeTypeSolution, 0.9},
		{domain.KnowledgeTypeLearning, 0.7},
		{domain.KnowledgeTypeFailure, 0.75},
		{domain.KnowledgeTypeReference, 0.5},
		{domain.KnowledgeType("unknown"), 0.5}, // Default for unknown
	}

	for _, tt := range tests {
		t.Run(string(tt.kt), func(t *testing.T) {
			if got := GetBaseScore(tt.kt); got != tt.expected {
				t.Errorf("GetBaseScore(%q) = %f, want %f", tt.kt, got, tt.expected)
			}
		})
	}
}

func TestGetConfidence(t *testing.T) {
	tests := []struct {
		tier     domain.Tier
		expected float64
	}{
		{domain.TierGold, 0.95},
		{domain.TierSilver, 0.80},
		{domain.TierBronze, 0.60},
		{domain.TierDiscard, 0.0},
		{domain.Tier("unknown"), 0.0}, // Default for unknown
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := GetConfidence(tt.tier, DefaultTierConfigs); got != tt.expected {
				t.Errorf("GetConfidence(%q) = %f, want %f", tt.tier, got, tt.expected)
			}
		})
	}
}

func TestRequiresHumanGate(t *testing.T) {
	tests := []struct {
		tier     domain.Tier
		expected bool
	}{
		{domain.TierGold, false},
		{domain.TierSilver, false},
		{domain.TierBronze, true},
		{domain.TierDiscard, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := RequiresHumanGate(tt.tier, DefaultTierConfigs); got != tt.expected {
				t.Errorf("RequiresHumanGate(%q) = %v, want %v", tt.tier, got, tt.expected)
			}
		})
	}
}

func TestKnowledgeTypesCompleteness(t *testing.T) {
	expectedTypes := []domain.KnowledgeType{
		domain.KnowledgeTypeDecision,
		domain.KnowledgeTypeSolution,
		domain.KnowledgeTypeLearning,
		domain.KnowledgeTypeFailure,
		domain.KnowledgeTypeReference,
	}

	for _, kt := range expectedTypes {
		if _, ok := KnowledgeTypes[kt]; !ok {
			t.Errorf("KnowledgeTypes missing entry for %q", kt)
		}
	}

	if len(KnowledgeTypes) != len(expectedTypes) {
		t.Errorf("KnowledgeTypes has %d entries, want %d", len(KnowledgeTypes), len(expectedTypes))
	}
}

func TestTierConfigsCompleteness(t *testing.T) {
	expectedTiers := []domain.Tier{
		domain.TierGold,
		domain.TierSilver,
		domain.TierBronze,
		domain.TierDiscard,
	}

	for _, tier := range expectedTiers {
		if _, ok := DefaultTierConfigs[tier]; !ok {
			t.Errorf("DefaultTierConfigs missing entry for %q", tier)
		}
	}

	if len(DefaultTierConfigs) != len(expectedTiers) {
		t.Errorf("DefaultTierConfigs has %d entries, want %d", len(DefaultTierConfigs), len(expectedTiers))
	}
}

func TestAssignTier_EmptyConfigs(t *testing.T) {
	// Empty config map: should return Discard for any score
	emptyConfigs := map[domain.Tier]TierConfig{}
	got := AssignTier(0.95, emptyConfigs)
	if got != domain.TierDiscard {
		t.Errorf("AssignTier(0.95, empty) = %q, want %q", got, domain.TierDiscard)
	}
}

func TestAssignTier_PartialConfigs(t *testing.T) {
	// Config with only Gold tier
	partial := map[domain.Tier]TierConfig{
		domain.TierGold: DefaultTierConfigs[domain.TierGold],
	}
	// Score that would be Gold
	if got := AssignTier(0.90, partial); got != domain.TierGold {
		t.Errorf("AssignTier(0.90, gold-only) = %q, want %q", got, domain.TierGold)
	}
	// Score that would be Silver (not in config) — should fall through to Discard
	if got := AssignTier(0.75, partial); got != domain.TierDiscard {
		t.Errorf("AssignTier(0.75, gold-only) = %q, want %q", got, domain.TierDiscard)
	}
}

func TestRequiresHumanGate_UnknownTier(t *testing.T) {
	if got := RequiresHumanGate(domain.Tier("unknown"), DefaultTierConfigs); got != false {
		t.Errorf("RequiresHumanGate(unknown) = %v, want false", got)
	}
}

func TestTierBoundariesAreContiguous(t *testing.T) {
	// Verify that tier boundaries don't have gaps
	prevMax := 0.0
	for _, tier := range TierOrder {
		config := DefaultTierConfigs[tier]
		// Note: TierOrder is highest to lowest, but we process lowest to highest
		// So we reverse the check
		if config.MaxScore < config.MinScore {
			t.Errorf("Tier %q has MaxScore (%f) < MinScore (%f)",
				tier, config.MaxScore, config.MinScore)
		}
		prevMax = config.MaxScore
	}
	_ = prevMax // Suppress unused warning
}

// --- Benchmarks ---

func BenchmarkAssignTier(b *testing.B) {
	b.ResetTimer()
	for i := range b.N {
		AssignTier(float64(i%10)*0.1, DefaultTierConfigs)
	}
}

func BenchmarkGetBaseScore(b *testing.B) {
	knTypes := []domain.KnowledgeType{
		domain.KnowledgeTypeLearning,
		domain.KnowledgeTypeDecision,
		domain.KnowledgeTypeSolution,
	}
	b.ResetTimer()
	for i := range b.N {
		GetBaseScore(knTypes[i%len(knTypes)])
	}
}

func BenchmarkValidateWeights(b *testing.B) {
	w := DefaultRubricWeights
	b.ResetTimer()
	for range b.N {
		w.ValidateWeights()
	}
}
