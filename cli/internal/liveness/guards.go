package liveness

// Separation-of-duties primitives layered on the role matrix (per the
// SageHarbor/RubyMoose review of ag-c3c7.1). These are the reusable building
// blocks the request-shaped Check (request.go) composes, and which ag-xdrw
// consumes directly. Pure logic only — no CLI, event schema, blast-radius,
// deadman, or CI surface.

// Disjoint reports whether an author and a judge are distinct, non-empty
// identities. A self-grade (author==judge) or a missing identity returns Denied
// — a hard integrity failure, not an escalatable capability gap.
//
// This is the reusable primitive ag-xdrw consumes: no work may be verified by
// its own author. It is identity-level (orthogonal to Authorize, which is
// role-level) so the two compose in Check.
func Disjoint(authorID, judgeID string) Decision {
	if authorID == "" || judgeID == "" || authorID == judgeID {
		return Denied
	}
	return Allowed
}

// VerifyAssignedRole reports whether a claimed role matches the role assigned to
// an actor by an external/trusted source. An empty assignment, an unknown
// assigned role, or a claim that does not match the assignment returns Denied:
// authority is sourced, not authored — an actor cannot self-grant a role. (The
// self-source check itself — RoleSource must not be the actor — lives in Check,
// which holds the agent identity.)
func VerifyAssignedRole(claimed, assigned Role) Decision {
	if assigned == "" || !IsRole(assigned) || claimed != assigned {
		return Denied
	}
	return Allowed
}

// MatrixConstitutional marks the role-capability matrix and these guards as a
// constitutional (protected) surface: they cannot be changed by the cluster's
// ordinary quorum. Editing roleCapabilities, the verbs, or these guards requires
// the constitutional bar (3-of-3 across >=2 model families + operator) per the
// self-evolution constitution. It is a declaration for the admission layer's
// protected-surface manifest, not a runtime toggle.
const MatrixConstitutional = true
