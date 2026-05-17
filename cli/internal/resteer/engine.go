package resteer

import (
	"sort"

	"github.com/boshu2/agentops/cli/internal/verdictledger"
)

// SkipReason identifies why a directive produced no re-steer recommendation.
// It is part of the engine's explainability surface (ADR-0006 §Consequences:
// "the re-steer output should explain why a directive was skipped").
type SkipReason string

const (
	// SkipHealthy means the directive has no qualifying failure streak.
	SkipHealthy SkipReason = "healthy"
	// SkipInsufficientEvidence means iteration count < minimum_evidence_count.
	SkipInsufficientEvidence SkipReason = "insufficient_evidence"
	// SkipCooldown means a re-steer event is recorded within the cooldown
	// window (ADR-0006 I-5: cooldown overrides streak).
	SkipCooldown SkipReason = "cooldown"
)

// Recommendation is the typed output of the F5.2 engine for one directive: a
// PROPOSED re-steer mutation, never an applied one. F5.3 turns this into an
// actual GOALS.md mutation after human confirmation.
type Recommendation struct {
	// DirectiveID is the stable GOALS.md directive ID (pattern ^d-...).
	DirectiveID string `json:"directive_id"`
	// MutationType is the proposed mutation: one of the Mutation* constants.
	// The default is MutationPriorityBump; MutationSteerFlip appears only when
	// the policy dual opt-in (ADR-0006 I-3) is satisfied.
	MutationType string `json:"mutation_type"`
	// FailureStreak is the consecutive-fail count that triggered the proposal.
	FailureStreak int `json:"failure_streak"`
	// IterationCount is the total evidence count for the directive.
	IterationCount int `json:"iteration_count"`
	// PriorityBump is the bounded number of positions to move the directive up
	// (1..max_priority_bump). It is 0 for non-priority_bump mutations.
	PriorityBump int `json:"priority_bump,omitempty"`
	// Rationale is a human-readable one-line explanation of the proposal.
	Rationale string `json:"rationale"`
}

// Skip records a directive the engine examined but did not recommend, with the
// reason why. It feeds explainable re-steer output.
type Skip struct {
	DirectiveID   string     `json:"directive_id"`
	Reason        SkipReason `json:"reason"`
	FailureStreak int        `json:"failure_streak"`
}

// Result is the full engine output for one re-steer evaluation: the
// recommendations to surface and the directives skipped (with reasons).
type Result struct {
	Recommendations []Recommendation `json:"recommendations"`
	Skipped         []Skip           `json:"skipped"`
}

// Evaluate runs the F5.2 policy engine over the verdict ledger for the given
// set of directive IDs and returns the recommendations plus skip reasons.
//
// For each directive a recommendation is produced only when ALL gates pass:
//
//   - IterationCount >= policy.MinimumEvidenceCount   (ADR-0006 I-4)
//   - FailureStreak  >= policy.FailureStreakLength    (ADR-0006 §FAILURE STREAK; I-1 via the >=2 schema floor)
//   - NOT InCooldown(directive, policy.CooldownIterations) (ADR-0006 I-5)
//
// A healthy directive (streak below threshold) yields no recommendation. The
// cooldown gate takes precedence over the streak gate. directiveIDs should be
// the stable GOALS.md directive IDs; ledger entries for IDs not in the list
// are ignored.
func Evaluate(ledger *verdictledger.Ledger, policy Policy, directiveIDs []string) Result {
	res := Result{}
	for _, id := range directiveIDs {
		rec, skip := evaluateDirective(ledger, policy, id)
		if rec != nil {
			res.Recommendations = append(res.Recommendations, *rec)
			continue
		}
		res.Skipped = append(res.Skipped, *skip)
	}
	sort.Slice(res.Recommendations, func(i, j int) bool {
		return res.Recommendations[i].DirectiveID < res.Recommendations[j].DirectiveID
	})
	sort.Slice(res.Skipped, func(i, j int) bool {
		return res.Skipped[i].DirectiveID < res.Skipped[j].DirectiveID
	})
	return res
}

// evaluateDirective applies the policy gates to one directive. It returns
// either a non-nil Recommendation (and nil Skip) or a non-nil Skip (and nil
// Recommendation) — never both, never neither.
func evaluateDirective(ledger *verdictledger.Ledger, policy Policy, directiveID string) (*Recommendation, *Skip) {
	iterations := ledger.IterationCount(directiveID)
	streak := ledger.FailureStreak(directiveID)

	if iterations < policy.MinimumEvidenceCount {
		return nil, &Skip{DirectiveID: directiveID, Reason: SkipInsufficientEvidence, FailureStreak: streak}
	}
	if streak < policy.FailureStreakLength {
		return nil, &Skip{DirectiveID: directiveID, Reason: SkipHealthy, FailureStreak: streak}
	}
	if ledger.InCooldown(directiveID, policy.CooldownIterations) {
		return nil, &Skip{DirectiveID: directiveID, Reason: SkipCooldown, FailureStreak: streak}
	}
	return buildRecommendation(policy, directiveID, streak, iterations), nil
}

// buildRecommendation constructs the Recommendation for an eligible directive.
//
// The default mutation is a bounded priority bump (ADR-0006: priority_bump is
// the safe default; max_priority_bump caps the move). A steer_flip is proposed
// ONLY when policy.SteerFlipPermitted() — i.e. both allow_steer_flip:true AND
// "steer_flip" in allowed_mutation_types (ADR-0006 I-3). Even with the dual
// opt-in the flip is offered only when priority_bump is itself not permitted,
// so a policy that allows both still gets the safer priority bump.
func buildRecommendation(policy Policy, directiveID string, streak, iterations int) *Recommendation {
	if policy.allowsMutation(MutationPriorityBump) {
		return &Recommendation{
			DirectiveID:    directiveID,
			MutationType:   MutationPriorityBump,
			FailureStreak:  streak,
			IterationCount: iterations,
			PriorityBump:   priorityBumpFor(policy, streak),
			Rationale: rationalef("%d consecutive scenario failures over %d iterations; "+
				"propose priority bump (bounded by max_priority_bump=%d) for operator review",
				streak, iterations, policy.MaxPriorityBump),
		}
	}
	if policy.SteerFlipPermitted() {
		return &Recommendation{
			DirectiveID:    directiveID,
			MutationType:   MutationSteerFlip,
			FailureStreak:  streak,
			IterationCount: iterations,
			Rationale: rationalef("%d consecutive scenario failures over %d iterations; "+
				"steer_flip permitted by dual opt-in (allow_steer_flip + allowed_mutation_types)",
				streak, iterations),
		}
	}
	// No priority_bump and no permitted steer_flip: fall back to the first
	// allowed setpoint mutation, or priority_bump as a last resort if the
	// allowed list is empty.
	mutationType := firstAllowedSetpoint(policy)
	return &Recommendation{
		DirectiveID:    directiveID,
		MutationType:   mutationType,
		FailureStreak:  streak,
		IterationCount: iterations,
		Rationale: rationalef("%d consecutive scenario failures over %d iterations; "+
			"propose %s for operator review", streak, iterations, mutationType),
	}
}

// priorityBumpFor returns the bounded number of priority positions to propose:
// it scales gently with streak length but never exceeds max_priority_bump.
func priorityBumpFor(policy Policy, streak int) int {
	bump := streak - policy.FailureStreakLength + 1
	if bump < 1 {
		bump = 1
	}
	if bump > policy.MaxPriorityBump {
		bump = policy.MaxPriorityBump
	}
	return bump
}

// firstAllowedSetpoint returns the first setpoint mutation the policy permits,
// preferring loosen over tighten. It falls back to priority_bump when the
// allowed list contains no setpoint mutation (an unusual but valid policy).
func firstAllowedSetpoint(policy Policy) string {
	if policy.allowsMutation(MutationSetpointLoosen) {
		return MutationSetpointLoosen
	}
	if policy.allowsMutation(MutationSetpointTighten) {
		return MutationSetpointTighten
	}
	return MutationPriorityBump
}
