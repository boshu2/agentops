package liveness

import "testing"

// valid returns a baseline well-formed request that Check would Allow, which
// each case mutates to exercise one decision path.
func valid() AuthorizationRequest {
	return AuthorizationRequest{
		AgentID:     "agent-1",
		Role:        RoleWorker,
		RoleSource:  "lease",
		Verb:        VerbEdit,
		Surface:     "worktree/foo",
		ModelFamily: "claude",
	}
}

func TestCheck(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*AuthorizationRequest)
		want   Decision
	}{
		// Allowed.
		{"well-formed worker edit", func(r *AuthorizationRequest) {}, Allowed},
		{"orchestrator vote", func(r *AuthorizationRequest) { r.Role = RoleOrchestrator; r.Verb = VerbVote }, Allowed},
		{"verifier judges another's artifact", func(r *AuthorizationRequest) {
			r.Role = RoleVerifier
			r.Verb = VerbJudge
			r.ArtifactAuthorID = "agent-2"
		}, Allowed},

		// Denied — identity / source integrity.
		{"missing agent identity", func(r *AuthorizationRequest) { r.AgentID = "" }, Denied},
		{"missing role source", func(r *AuthorizationRequest) { r.RoleSource = "" }, Denied},
		{"arbitrary role-source string is not a grant", func(r *AuthorizationRequest) { r.RoleSource = "i-grant-myself" }, Denied},
		{"self-asserted role (source == actor)", func(r *AuthorizationRequest) { r.RoleSource = r.AgentID }, Denied},
		{"unknown role", func(r *AuthorizationRequest) { r.Role = Role("admin") }, Denied},

		// Denied — judge integrity.
		{"verifier judging its OWN artifact (self-grade)", func(r *AuthorizationRequest) {
			r.Role = RoleVerifier
			r.Verb = VerbJudge
			r.ArtifactAuthorID = r.AgentID
		}, Denied},
		{"judge with no named artifact author", func(r *AuthorizationRequest) {
			r.Role = RoleVerifier
			r.Verb = VerbJudge
			r.ArtifactAuthorID = ""
		}, Denied},

		// NeedsAdmission — protected-surface edit (escalate, don't execute).
		{"worker editing the protected role-matrix", func(r *AuthorizationRequest) { r.Surface = "role-matrix" }, NeedsAdmission},
		{"worker editing kernel-policy", func(r *AuthorizationRequest) { r.Surface = "kernel-policy" }, NeedsAdmission},

		// NeedsAdmission — known role out of capability.
		{"orchestrator attempting an edit", func(r *AuthorizationRequest) { r.Role = RoleOrchestrator; r.Verb = VerbEdit }, NeedsAdmission},
		{"verifier attempting a vote", func(r *AuthorizationRequest) { r.Role = RoleVerifier; r.Verb = VerbVote }, NeedsAdmission},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := valid()
			tc.mutate(&req)
			if got := Check(req); got != tc.want {
				t.Fatalf("Check(%+v) = %q, want %q", req, got, tc.want)
			}
		})
	}
}

func TestIsProtectedSurface(t *testing.T) {
	if !IsProtectedSurface("role-matrix") {
		t.Fatalf("IsProtectedSurface(role-matrix) = false, want true")
	}
	if IsProtectedSurface("worktree/foo") {
		t.Fatalf("IsProtectedSurface(worktree/foo) = true, want false")
	}
}

func TestIsTrustedRoleSource(t *testing.T) {
	for _, s := range []string{"lease", "registration", "operator", "quorum"} {
		if !IsTrustedRoleSource(s) {
			t.Fatalf("IsTrustedRoleSource(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "self-asserted", "pulse", "orchestrator-assignment", "agent-1", "random"} {
		if IsTrustedRoleSource(s) {
			t.Fatalf("IsTrustedRoleSource(%q) = true, want false", s)
		}
	}
}

// TestCheckValidSourcesAllowed confirms every allowlisted source passes Check on
// an otherwise-well-formed request (the reviewers' happy-path bar).
func TestCheckValidSourcesAllowed(t *testing.T) {
	for _, src := range []string{"lease", "registration", "operator", "quorum"} {
		req := valid()
		req.RoleSource = src
		if got := Check(req); got != Allowed {
			t.Fatalf("Check with RoleSource=%q = %q, want Allowed", src, got)
		}
	}
}
