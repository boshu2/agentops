package liveness

// AuthorizationRequest is the request-shaped input to the liveness/admission
// authorization check. It carries the externally-sourced role (never
// self-asserted), the acting identity and model family, the verb, the target
// surface, and — for review actions — the author of the artifact being judged.
// It is the central primitive the admission kernel and ag-xdrw consume.
type AuthorizationRequest struct {
	AgentID          string // the acting identity
	Role             Role   // the role being exercised; must be externally sourced
	RoleSource       string // who/what assigned the role; MUST NOT equal AgentID
	Verb             Verb   // the action attempted
	Surface          string // the target surface (for protected-surface checks)
	ModelFamily      string // the acting model family (for the cross-model quorum layer)
	ArtifactAuthorID string // for VerbJudge: author of the artifact under review (optional)
}

// Check is the central authorization decision over an AuthorizationRequest.
// Precedence — first failure wins:
//
//  1. Identity/source integrity -> Denied: missing agent identity, missing or
//     self-asserted role source (RoleSource == AgentID), or an unknown role.
//     Authority is sourced, not authored — a role the actor grants itself is
//     rejected hard.
//  2. Protected-surface edit -> NeedsAdmission: editing a constitutional surface
//     (role-matrix, kernel policy) escalates regardless of role — the gate
//     cannot be self-edited by ordinary capability.
//  3. Capability -> NeedsAdmission: a known, validly-sourced role acting outside
//     its verbs escalates to the admission controller.
//  4. Self-grade -> Denied: a verifier judging its own artifact is rejected hard
//     (no-self-grade / author != judge).
//  5. Otherwise -> Allowed.
func Check(req AuthorizationRequest) Decision {
	// 1. Identity present, role externally sourced (not self-asserted), and known.
	if req.AgentID == "" || req.RoleSource == "" || req.RoleSource == req.AgentID || !IsRole(req.Role) {
		return Denied
	}
	// 2. Editing a constitutional surface escalates regardless of role.
	if req.Verb == VerbEdit && IsProtectedSurface(req.Surface) {
		return NeedsAdmission
	}
	// 3. Role must hold the verb.
	if Authorize(req.Role, req.Verb) != Allowed {
		return NeedsAdmission
	}
	// 4. A verifier may not judge its own artifact.
	if req.Verb == VerbJudge && req.ArtifactAuthorID != "" && Disjoint(req.ArtifactAuthorID, req.AgentID) != Allowed {
		return Denied
	}
	return Allowed
}

// protectedSurfaces are the constitutional surfaces whose edits must escalate to
// the constitutional bar (3-of-3 / >=2 model families / operator) rather than
// execute. The matrix governs its own protection (see MatrixConstitutional).
var protectedSurfaces = map[string]struct{}{
	"role-matrix":   {},
	"kernel-policy": {},
}

// IsProtectedSurface reports whether s is a constitutional/protected surface.
func IsProtectedSurface(s string) bool {
	_, ok := protectedSurfaces[s]
	return ok
}
