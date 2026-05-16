// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
	"fmt"
)

// InMemorySafetyPolicy is a SafetyPolicyPort backed by fixed policy
// decisions. Unknown policies fail closed with BLOCK.
type InMemorySafetyPolicy struct {
	decisions map[SafetyPolicyName]SafetyDecision
}

func NewInMemorySafetyPolicy(decisions map[SafetyPolicyName]SafetyDecision) *InMemorySafetyPolicy {
	if decisions == nil {
		decisions = map[SafetyPolicyName]SafetyDecision{}
	}
	copied := make(map[SafetyPolicyName]SafetyDecision, len(decisions))
	for key, decision := range decisions {
		copied[key] = decision
	}
	return &InMemorySafetyPolicy{decisions: copied}
}

func (p *InMemorySafetyPolicy) Evaluate(ctx context.Context, req SafetyPolicyRequest) (SafetyDecision, error) {
	if err := ctx.Err(); err != nil {
		return SafetyDecision{}, err
	}
	if req.Policy == "" {
		return SafetyDecision{}, errors.New("ports: SafetyPolicyPort.Evaluate policy required")
	}
	if decision, ok := p.decisions[req.Policy]; ok {
		if decision.Reason == "" {
			decision.Reason = fmt.Sprintf("policy %q matched configured decision", req.Policy)
		}
		return decision, nil
	}
	return SafetyDecision{
		Status: SafetyDecisionBlock,
		Reason: fmt.Sprintf("unknown safety policy %q", req.Policy),
	}, nil
}

var _ SafetyPolicyPort = (*InMemorySafetyPolicy)(nil)
