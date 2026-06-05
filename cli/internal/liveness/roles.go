// Package liveness implements the liveness-kernel's mechanical control-plane
// primitives.
//
// roles.go is the role-capability matrix: each role's allowed verbs, with every
// out-of-capability action resolving to NeedsAdmission rather than executing.
// This mechanizes the control/data-plane separation — an orchestrator cannot
// perform a worker's edit, a verifier cannot edit what it judges, a heartbeat
// can only nudge — so role drift and self-grading are blocked by construction
// rather than by discipline. The default is deny-and-escalate: anything not
// explicitly granted is routed to the admission controller, never executed.
package liveness

import "sort"

// Role is a control-plane actor class.
type Role string

// The defined control-plane roles.
const (
	// RoleOrchestrator routes, votes, shepherds, and synthesizes. It does NOT
	// perform worker edits — that separation is the prompt-injection defense.
	RoleOrchestrator Role = "orchestrator"
	// RoleWorker edits within its claimed worktree.
	RoleWorker Role = "worker"
	// RoleVerifier judges; it never edits what it judges (no-self-grade).
	RoleVerifier Role = "verifier"
	// RoleScribe records evidence; it never decides.
	RoleScribe Role = "scribe"
	// RoleHeartbeat only nudges; it never acts on the work itself.
	RoleHeartbeat Role = "heartbeat"
)

// Verb is an action a role may attempt.
type Verb string

// The control-plane verbs.
const (
	VerbRoute      Verb = "route"
	VerbVote       Verb = "vote"
	VerbShepherd   Verb = "shepherd"
	VerbSynthesize Verb = "synthesize"
	VerbEdit       Verb = "edit"
	VerbJudge      Verb = "judge"
	VerbRecord     Verb = "record"
	VerbNudge      Verb = "nudge"
)

// Decision is the outcome of an authorization check.
type Decision string

const (
	// Allowed means the role holds the verb in the capability matrix.
	Allowed Decision = "allowed"
	// NeedsAdmission means the action is outside the role's capability and must
	// be routed to the admission controller rather than executed.
	NeedsAdmission Decision = "needs-admission"
)

// roleCapabilities is the authoritative role->verb matrix. A verb absent for a
// role, or an unknown role, yields NeedsAdmission.
var roleCapabilities = map[Role]map[Verb]struct{}{
	RoleOrchestrator: {VerbRoute: {}, VerbVote: {}, VerbShepherd: {}, VerbSynthesize: {}},
	RoleWorker:       {VerbEdit: {}},
	RoleVerifier:     {VerbJudge: {}},
	RoleScribe:       {VerbRecord: {}},
	RoleHeartbeat:    {VerbNudge: {}},
}

// Authorize reports whether role may perform verb. Anything not explicitly
// granted resolves to NeedsAdmission — deny-and-escalate, never allow-by-default.
func Authorize(role Role, verb Verb) Decision {
	if verbs, ok := roleCapabilities[role]; ok {
		if _, granted := verbs[verb]; granted {
			return Allowed
		}
	}
	return NeedsAdmission
}

// Can is a boolean convenience over Authorize.
func Can(role Role, verb Verb) bool {
	return Authorize(role, verb) == Allowed
}

// IsRole reports whether r is a defined role.
func IsRole(r Role) bool {
	_, ok := roleCapabilities[r]
	return ok
}

// Capabilities returns the verbs granted to role in sorted order, or nil for an
// unknown role.
func Capabilities(role Role) []Verb {
	verbs, ok := roleCapabilities[role]
	if !ok {
		return nil
	}
	out := make([]Verb, 0, len(verbs))
	for v := range verbs {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Roles returns all defined roles in sorted order.
func Roles() []Role {
	out := make([]Role, 0, len(roleCapabilities))
	for r := range roleCapabilities {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
