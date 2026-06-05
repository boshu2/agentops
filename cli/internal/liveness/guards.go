package liveness

// This file adds the separation-of-duties guards layered on top of the role
// matrix (per the RubyMoose checklist on ag-c3c7.1): author/judge disjointness
// (the reusable ag-xdrw primitive), external role binding (roles are assigned,
// never self-asserted — the same computed-not-authored invariant as reach), and
// the protected-surface marker. Pure logic only; no CLI, event schema,
// blast-radius, deadman, or CI surface lives here.

// Disjoint reports whether an author and a judge are distinct, non-empty
// identities. It returns NeedsAdmission when authorID == judgeID (a self-grade)
// or when either id is empty, and Allowed otherwise.
//
// This is the reusable primitive ag-xdrw consumes: no work may be verified by
// its own author. It is identity-level (orthogonal to Authorize, which is
// role-level) so the two compose — see AuthorizeReview.
func Disjoint(authorID, judgeID string) Decision {
	if authorID == "" || judgeID == "" || authorID == judgeID {
		return NeedsAdmission
	}
	return Allowed
}

// AuthorizeReview composes the verifier capability with author/judge
// disjointness: a judge may review an author's work only if it holds the
// verifier role AND is a distinct identity from the author. Any failure of
// either condition is NeedsAdmission.
func AuthorizeReview(judgeRole Role, authorID, judgeID string) Decision {
	if Authorize(judgeRole, VerbJudge) != Allowed {
		return NeedsAdmission
	}
	return Disjoint(authorID, judgeID)
}

// VerifyAssignedRole reports whether a claimed role matches the role assigned to
// an actor by an external/trusted source. An actor cannot self-assert authority
// it was not assigned: a mismatch, an empty assignment, or an unknown assigned
// role is NeedsAdmission. This is the role-level form of the same invariant that
// governs reach=always — high-authority labels are sourced, not authored — so
// the role passed to Authorize must come from VerifyAssignedRole, never from the
// actor's own claim.
func VerifyAssignedRole(claimed, assigned Role) Decision {
	if assigned == "" || !IsRole(assigned) || claimed != assigned {
		return NeedsAdmission
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
