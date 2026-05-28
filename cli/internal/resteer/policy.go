// Package resteer is the F5.2 re-steer policy engine. It reads the F5.1 verdict
// ledger (cli/internal/verdictledger) and the F5.9 re-steer policy
// (schemas/re-steer-policy.v1.schema.json), and for each GOALS.md directive
// decides whether a chronic scenario-failure streak warrants a re-steer
// RECOMMENDATION.
//
// This package never mutates GOALS.md. Its output is a typed Recommendation
// describing a proposed directive mutation and the evidence behind it. F5.3
// (`ao goals steer --auto`) owns turning a Recommendation into an applied
// mutation, gated by human confirmation per ADR-0006 I-2.
//
// Every safety invariant in ADR-0006 is honored here:
//
//   - I-1 single fresh failure never triggers a mutation — the policy's
//     failure_streak_length is >= 2 (schema-enforced) and the engine requires
//     FailureStreak >= failure_streak_length.
//   - I-3 Steer-direction flips require dual opt-in — a steer_flip
//     recommendation is produced ONLY when the policy has both
//     allow_steer_flip:true AND "steer_flip" in allowed_mutation_types.
//   - I-4 minimum evidence — a directive must have >= minimum_evidence_count
//     iteration records before any recommendation.
//   - I-5 cooldown is enforced regardless of streak.
package resteer

import (
	"encoding/json"
	"fmt"
	"os"
)

// Mutation type identifiers. These mirror the re-steer-policy.v1 enum and the
// verdictledger.Mutation* constants.
const (
	MutationPriorityBump    = "priority_bump"
	MutationSetpointTighten = "setpoint_tighten"
	MutationSetpointLoosen  = "setpoint_loosen"
	MutationSteerFlip       = "steer_flip"
)

// DefaultPolicyRelPath is the canonical (tracked) location of the re-steer
// policy, relative to the project root.
const DefaultPolicyRelPath = "docs/re-steer-policy.json"

// Policy is the parsed, validated re-steer policy. Field names and JSON tags
// match schemas/re-steer-policy.v1.schema.json exactly.
type Policy struct {
	MinimumEvidenceCount int      `json:"minimum_evidence_count"`
	FailureStreakLength  int      `json:"failure_streak_length"`
	CooldownIterations   int      `json:"cooldown_iterations"`
	AllowedMutationTypes []string `json:"allowed_mutation_types"`
	MaxPriorityBump      int      `json:"max_priority_bump"`
	AutoApply            bool     `json:"auto_apply"`
	AllowSteerFlip       bool     `json:"allow_steer_flip"`
}

// DefaultPolicy returns the built-in safe defaults from ADR-0006 §Default
// Policy. It is used when no policy file is present. The defaults are
// recommendation-only (AutoApply:false) and forbid Steer flips
// (AllowSteerFlip:false, steer_flip absent from AllowedMutationTypes).
func DefaultPolicy() Policy {
	return Policy{
		MinimumEvidenceCount: 5,
		FailureStreakLength:  3,
		CooldownIterations:   5,
		AllowedMutationTypes: []string{MutationPriorityBump, MutationSetpointTighten, MutationSetpointLoosen},
		MaxPriorityBump:      3,
		AutoApply:            false,
		AllowSteerFlip:       false,
	}
}

// LoadPolicy reads and validates the re-steer policy at path. A missing file
// is not an error: the ADR-0006 safe DefaultPolicy is returned instead. A
// malformed or schema-invalid file is a hard error so a bad policy never
// silently degrades to permissive behavior.
func LoadPolicy(path string) (Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultPolicy(), nil
		}
		return Policy{}, fmt.Errorf("read re-steer policy %s: %w", path, err)
	}
	var p Policy
	if err := json.Unmarshal(data, &p); err != nil {
		return Policy{}, fmt.Errorf("parse re-steer policy %s: %w", path, err)
	}
	if defect := p.validate(); defect != "" {
		return Policy{}, fmt.Errorf("invalid re-steer policy %s: %s", path, defect)
	}
	return p, nil
}

// allowsMutation reports whether mutationType appears in the policy's
// allowed_mutation_types list.
func (p Policy) allowsMutation(mutationType string) bool {
	for _, m := range p.AllowedMutationTypes {
		if m == mutationType {
			return true
		}
	}
	return false
}

// SteerFlipPermitted reports whether the policy permits a Steer-direction flip.
// Per ADR-0006 I-3 this requires BOTH allow_steer_flip:true AND "steer_flip"
// present in allowed_mutation_types. Either alone is insufficient — the dual
// opt-in is what makes the invariant resistant to copy-paste misconfiguration.
func (p Policy) SteerFlipPermitted() bool {
	return p.AllowSteerFlip && p.allowsMutation(MutationSteerFlip)
}

// validate returns a non-empty string describing the first defect in the
// policy, or "" if it satisfies the re-steer-policy.v1 contract. It enforces
// the schema constraints that matter for engine safety, notably ADR-0006 I-1
// (failure_streak_length minimum 2).
func (p Policy) validate() string {
	if p.MinimumEvidenceCount < 1 {
		return "minimum_evidence_count must be >= 1"
	}
	if p.FailureStreakLength < 2 {
		return "failure_streak_length must be >= 2 (ADR-0006 I-1)"
	}
	if p.CooldownIterations < 1 {
		return "cooldown_iterations must be >= 1"
	}
	if p.MaxPriorityBump < 1 {
		return "max_priority_bump must be >= 1"
	}
	for _, m := range p.AllowedMutationTypes {
		if !validMutationType(m) {
			return "invalid allowed_mutation_types entry: " + m
		}
	}
	return ""
}

// Equal reports whether two policies have identical field values, including a
// member-wise comparison of allowed_mutation_types. Policy contains a slice so
// it is not comparable with ==; this helper is the canonical equality check.
func (p Policy) Equal(other Policy) bool {
	if p.MinimumEvidenceCount != other.MinimumEvidenceCount ||
		p.FailureStreakLength != other.FailureStreakLength ||
		p.CooldownIterations != other.CooldownIterations ||
		p.MaxPriorityBump != other.MaxPriorityBump ||
		p.AutoApply != other.AutoApply ||
		p.AllowSteerFlip != other.AllowSteerFlip {
		return false
	}
	if len(p.AllowedMutationTypes) != len(other.AllowedMutationTypes) {
		return false
	}
	for i := range p.AllowedMutationTypes {
		if p.AllowedMutationTypes[i] != other.AllowedMutationTypes[i] {
			return false
		}
	}
	return true
}

// validMutationType reports whether m is a recognized re-steer mutation type.
func validMutationType(m string) bool {
	switch m {
	case MutationPriorityBump, MutationSetpointTighten, MutationSetpointLoosen, MutationSteerFlip:
		return true
	default:
		return false
	}
}
