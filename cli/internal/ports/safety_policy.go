// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// SafetyPolicyName identifies a deterministic safety policy. Examples
// include "git.destructive", "worker.git", "scope.edit", and
// "holdout.read".
type SafetyPolicyName string

// SafetyDecisionStatus is the outcome of evaluating a policy against a
// proposed operation.
type SafetyDecisionStatus string

const (
	SafetyDecisionAllow SafetyDecisionStatus = "ALLOW"
	SafetyDecisionWarn  SafetyDecisionStatus = "WARN"
	SafetyDecisionBlock SafetyDecisionStatus = "BLOCK"
)

// SafetyPolicyRequest describes a proposed runtime operation. Subject
// is the file, command, tool, or resource being touched. Metadata is an
// open bag for adapter-specific facts such as worker role or scope id.
type SafetyPolicyRequest struct {
	Policy    SafetyPolicyName
	Actor     string
	Operation string
	Subject   string
	Metadata  map[string]string
}

// SafetyDecision is the typed safety result. Reason must be
// human-readable because blocked decisions are shown to operators.
type SafetyDecision struct {
	Status SafetyDecisionStatus
	Reason string
}

// SafetyPolicyPort is the Evidence and Trust safety surface for
// hookless runtime operations. It absorbs deterministic blocking hooks
// such as destructive git guards, worker git authority, edit scope
// guards, and holdout-isolation gates.
//
// Contract:
//
//   - Evaluate rejects an empty Policy.
//   - A successful decision MUST include a non-empty Reason.
//   - Unknown policies should fail closed unless an adapter documents a
//     narrower policy.
//   - Context cancellation MUST be honored on a best-effort basis.
type SafetyPolicyPort interface {
	Evaluate(ctx context.Context, req SafetyPolicyRequest) (SafetyDecision, error)
}
