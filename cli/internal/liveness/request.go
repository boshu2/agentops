package liveness

// AuthorizationRequest is the request-shaped input to the liveness/admission
// authorization check. It carries the externally-sourced role (never
// self-asserted), the acting identity and model family, the verb, the target
// surface, and — for review actions — the author of the artifact being judged.
// It is the central primitive the admission kernel and ag-xdrw consume.
type AuthorizationRequest struct {
	AgentID          string // the acting identity
	Role             Role   // the role being exercised; must be externally sourced
	RoleSource       string // who/what assigned the role; must be a trusted source, not the actor
	Verb             Verb   // the action attempted
	Surface          string // the target surface (for protected-surface checks)
	ModelFamily      string // the acting model family (for the cross-model quorum layer)
	ArtifactAuthorID string // for VerbJudge: author of the artifact under review (required for judge)
}

// trustedRoleSources is the allowlist of authorities that may grant a role. An
// arbitrary or unknown source string is not a grant — authority is sourced from
// a recognized authority, never asserted by free text. (self-asserted, pulse,
// unknown, and empty all fail this check.)
var trustedRoleSources = map[string]struct{}{
	"lease":        {}, // role granted by a work-lease
	"registration": {}, // role from agent registration
	"operator":     {}, // operator-root assignment
	"quorum":       {}, // quorum-ratified assignment
}

// IsTrustedRoleSource reports whether s is an allowlisted role-granting authority.
func IsTrustedRoleSource(s string) bool {
	_, ok := trustedRoleSources[s]
	return ok
}

// Check is the central authorization decision over an AuthorizationRequest.
// Precedence — first failure wins:
//
//  1. Identity/source integrity -> Denied: missing agent identity; a role source
//     that is not an allowlisted authority (arbitrary strings do NOT grant); a
//     self-asserted role (RoleSource == AgentID); or an unknown role. Authority
//     is sourced from a recognized authority, not authored by the actor.
//  2. Protected-surface edit -> NeedsAdmission: editing a constitutional surface
//     (role-matrix, kernel policy) escalates regardless of role.
//  3. Capability -> NeedsAdmission: a known, validly-sourced role acting outside
//     its verbs escalates to the admission controller.
//  4. Judge integrity -> Denied: a judge action with no named artifact author,
//     or a verifier judging its own artifact (author == judge). No-self-grade,
//     and a judgment must name what it judges.
//  5. Otherwise -> Allowed.
func Check(req AuthorizationRequest) Decision {
	// 1. Identity present, role granted by a TRUSTED source, not self-asserted, known.
	if req.AgentID == "" || !IsTrustedRoleSource(req.RoleSource) || req.RoleSource == req.AgentID || !IsRole(req.Role) {
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
	// 4. A judge must name an artifact author and must not be that author.
	if req.Verb == VerbJudge {
		if req.ArtifactAuthorID == "" || Disjoint(req.ArtifactAuthorID, req.AgentID) != Allowed {
			return Denied
		}
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
