// Package taxonomy defines the knowledge classification system and scoring rubric.
//
// # Knowledge Types
//
// The system classifies extracted knowledge into five types:
//   - Decision: Architectural choice with rationale (base score: 0.8)
//   - Solution: Working fix for a problem (base score: 0.9)
//   - Learning: Insight gained from experience (base score: 0.7)
//   - Failure: What didn't work and why (base score: 0.75)
//   - Reference: Pointer to useful resource (base score: 0.5)
//
// # Quality Tiers
//
// Candidates are assigned to tiers based on their composite score:
//   - Gold (0.85-1.0): Highest quality, auto-promoted
//   - Silver (0.70-0.84): High quality, auto-promoted
//   - Bronze (0.50-0.69): Acceptable, 5% sampled for human review
//   - Discard (<0.50): Below threshold, not stored
//
// # Scoring Rubric
//
// The composite score is calculated from five dimensions:
//   - Specificity (30%): Named entities, concrete values, code snippets
//   - Actionability (25%): Imperative verbs, clear steps, before/after
//   - Novelty (20%): Uniqueness vs. common knowledge
//   - Context (15%): Quality of surrounding context
//   - Confidence (10%): Assertion strength
package taxonomy

import "github.com/boshu2/agentops/cli/internal/domain"

// KnowledgeTypeInfo describes a knowledge type and its base scoring.
type KnowledgeTypeInfo struct {
	// Type is the knowledge type constant.
	Type domain.KnowledgeType

	// Description explains what this type represents.
	Description string

	// BaseScore is the starting score for this type (0.0-1.0).
	BaseScore float64
}

// KnowledgeTypes maps type identifiers to their info.
var KnowledgeTypes = map[domain.KnowledgeType]KnowledgeTypeInfo{
	domain.KnowledgeTypeDecision: {
		Type:        domain.KnowledgeTypeDecision,
		Description: "Architectural choice with rationale",
		BaseScore:   0.8,
	},
	domain.KnowledgeTypeSolution: {
		Type:        domain.KnowledgeTypeSolution,
		Description: "Working fix for a problem",
		BaseScore:   0.9,
	},
	domain.KnowledgeTypeLearning: {
		Type:        domain.KnowledgeTypeLearning,
		Description: "Insight gained from experience",
		BaseScore:   0.7,
	},
	domain.KnowledgeTypeFailure: {
		Type:        domain.KnowledgeTypeFailure,
		Description: "What didn't work and why",
		BaseScore:   0.75,
	},
	domain.KnowledgeTypeReference: {
		Type:        domain.KnowledgeTypeReference,
		Description: "Pointer to useful resource",
		BaseScore:   0.5,
	},
}

// TierConfig defines the configuration for a quality tier.
type TierConfig struct {
	// Tier is the tier constant.
	Tier domain.Tier

	// MinScore is the minimum score to qualify for this tier.
	MinScore float64

	// MaxScore is the maximum score for this tier (exclusive upper bound for next tier).
	MaxScore float64

	// Confidence is the confidence value to use when storing.
	Confidence float64

	// HumanGateRequired indicates if human review is required.
	HumanGateRequired bool

	// HumanGateSampleRate is the percentage of entries to sample for review (0.0-1.0).
	HumanGateSampleRate float64
}

// DefaultTierConfigs provides the default tier thresholds.
// These can be overridden via configuration.
var DefaultTierConfigs = map[domain.Tier]TierConfig{
	domain.TierGold: {
		Tier:                domain.TierGold,
		MinScore:            0.85,
		MaxScore:            1.01, // Exclusive upper bound
		Confidence:          0.95,
		HumanGateRequired:   false,
		HumanGateSampleRate: 0.0,
	},
	domain.TierSilver: {
		Tier:                domain.TierSilver,
		MinScore:            0.70,
		MaxScore:            0.85,
		Confidence:          0.80,
		HumanGateRequired:   false,
		HumanGateSampleRate: 0.0,
	},
	domain.TierBronze: {
		Tier:                domain.TierBronze,
		MinScore:            0.50,
		MaxScore:            0.70,
		Confidence:          0.60,
		HumanGateRequired:   true,
		HumanGateSampleRate: 0.05, // 5% sample
	},
	domain.TierDiscard: {
		Tier:                domain.TierDiscard,
		MinScore:            0.0,
		MaxScore:            0.50,
		Confidence:          0.0, // Not stored
		HumanGateRequired:   false,
		HumanGateSampleRate: 0.0,
	},
}

// RubricWeights defines the weights for each scoring dimension.
// All weights must sum to 1.0.
type RubricWeights struct {
	// Specificity measures named entities, concrete values, code snippets.
	// Weight: 0.30 (30%)
	Specificity float64

	// Actionability measures imperative verbs, clear steps, before/after.
	// Weight: 0.25 (25%)
	Actionability float64

	// Novelty measures uniqueness vs. common knowledge.
	// Weight: 0.20 (20%)
	Novelty float64

	// Context measures quality of surrounding context.
	// Weight: 0.15 (15%)
	Context float64

	// Confidence measures assertion strength.
	// Weight: 0.10 (10%)
	Confidence float64
}

// DefaultRubricWeights provides the standard scoring weights.
var DefaultRubricWeights = RubricWeights{
	Specificity:   0.30,
	Actionability: 0.25,
	Novelty:       0.20,
	Context:       0.15,
	Confidence:    0.10,
}

// ValidateWeights checks that rubric weights sum to 1.0.
func (w RubricWeights) ValidateWeights() bool {
	sum := w.Specificity + w.Actionability + w.Novelty + w.Context + w.Confidence
	// Allow small floating point variance
	return sum >= 0.99 && sum <= 1.01
}

// TierOrder provides the tier ordering from highest to lowest quality.
var TierOrder = []domain.Tier{
	domain.TierGold,
	domain.TierSilver,
	domain.TierBronze,
	domain.TierDiscard,
}

// AssignTier returns the appropriate tier for a given score.
func AssignTier(score float64, configs map[domain.Tier]TierConfig) domain.Tier {
	for _, tier := range TierOrder {
		config, ok := configs[tier]
		if !ok {
			continue
		}
		if score >= config.MinScore && score < config.MaxScore {
			return tier
		}
	}
	return domain.TierDiscard
}

// GetBaseScore returns the base score for a knowledge type.
func GetBaseScore(kt domain.KnowledgeType) float64 {
	if info, ok := KnowledgeTypes[kt]; ok {
		return info.BaseScore
	}
	return 0.5 // Default for unknown types
}

// GetConfidence returns the confidence level for a tier.
func GetConfidence(tier domain.Tier, configs map[domain.Tier]TierConfig) float64 {
	if config, ok := configs[tier]; ok {
		return config.Confidence
	}
	return 0.0
}

// RequiresHumanGate checks if a tier requires human review.
func RequiresHumanGate(tier domain.Tier, configs map[domain.Tier]TierConfig) bool {
	if config, ok := configs[tier]; ok {
		return config.HumanGateRequired
	}
	return false
}
