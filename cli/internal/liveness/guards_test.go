package liveness

import "testing"

func TestDisjoint(t *testing.T) {
	cases := []struct {
		name          string
		author, judge string
		want          Decision
	}{
		{"distinct identities", "alice", "bob", Allowed},
		{"same identity is a self-grade", "alice", "alice", NeedsAdmission},
		{"empty author", "", "bob", NeedsAdmission},
		{"empty judge", "alice", "", NeedsAdmission},
		{"both empty", "", "", NeedsAdmission},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Disjoint(tc.author, tc.judge); got != tc.want {
				t.Fatalf("Disjoint(%q,%q) = %q, want %q", tc.author, tc.judge, got, tc.want)
			}
		})
	}
}

func TestAuthorizeReview(t *testing.T) {
	cases := []struct {
		name          string
		role          Role
		author, judge string
		want          Decision
	}{
		{"verifier, distinct -> allowed", RoleVerifier, "alice", "bob", Allowed},
		{"verifier, same identity -> self-grade blocked", RoleVerifier, "alice", "alice", NeedsAdmission},
		{"orchestrator cannot review (lacks judge verb)", RoleOrchestrator, "alice", "bob", NeedsAdmission},
		{"worker cannot review", RoleWorker, "alice", "bob", NeedsAdmission},
		{"unknown role cannot review", Role("admin"), "alice", "bob", NeedsAdmission},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AuthorizeReview(tc.role, tc.author, tc.judge); got != tc.want {
				t.Fatalf("AuthorizeReview(%q,%q,%q) = %q, want %q", tc.role, tc.author, tc.judge, got, tc.want)
			}
		})
	}
}

func TestVerifyAssignedRole(t *testing.T) {
	cases := []struct {
		name              string
		claimed, assigned Role
		want              Decision
	}{
		{"claimed matches assigned", RoleWorker, RoleWorker, Allowed},
		{"self-asserted role not assigned (escalation)", RoleOrchestrator, RoleWorker, NeedsAdmission},
		{"empty assignment cannot be self-claimed", RoleVerifier, Role(""), NeedsAdmission},
		{"unknown assigned role denied", Role("admin"), Role("admin"), NeedsAdmission},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := VerifyAssignedRole(tc.claimed, tc.assigned); got != tc.want {
				t.Fatalf("VerifyAssignedRole(%q,%q) = %q, want %q", tc.claimed, tc.assigned, got, tc.want)
			}
		})
	}
}

func TestMatrixConstitutional(t *testing.T) {
	if MatrixConstitutional != true {
		t.Fatalf("MatrixConstitutional = %v, want true (the matrix must be a protected surface)", MatrixConstitutional)
	}
}
