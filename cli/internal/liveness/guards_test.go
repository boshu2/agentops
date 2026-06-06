package liveness

import "testing"

func TestDisjoint(t *testing.T) {
	cases := []struct {
		name          string
		author, judge string
		want          Decision
	}{
		{"distinct identities", "alice", "bob", Allowed},
		{"same identity is a self-grade", "alice", "alice", Denied},
		{"empty author", "", "bob", Denied},
		{"empty judge", "alice", "", Denied},
		{"both empty", "", "", Denied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Disjoint(tc.author, tc.judge); got != tc.want {
				t.Fatalf("Disjoint(%q,%q) = %q, want %q", tc.author, tc.judge, got, tc.want)
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
		{"self-asserted role not assigned (escalation) -> denied", RoleOrchestrator, RoleWorker, Denied},
		{"empty assignment cannot be claimed -> denied", RoleVerifier, Role(""), Denied},
		{"unknown assigned role -> denied", Role("admin"), Role("admin"), Denied},
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
