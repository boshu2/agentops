package liveness

import (
	"reflect"
	"testing"
)

func TestAuthorize(t *testing.T) {
	cases := []struct {
		name string
		role Role
		verb Verb
		want Decision
	}{
		// Each role's granted verbs.
		{"orchestrator routes", RoleOrchestrator, VerbRoute, Allowed},
		{"orchestrator votes", RoleOrchestrator, VerbVote, Allowed},
		{"orchestrator shepherds", RoleOrchestrator, VerbShepherd, Allowed},
		{"orchestrator synthesizes", RoleOrchestrator, VerbSynthesize, Allowed},
		{"worker edits", RoleWorker, VerbEdit, Allowed},
		{"verifier judges", RoleVerifier, VerbJudge, Allowed},
		{"scribe records", RoleScribe, VerbRecord, Allowed},
		{"heartbeat nudges", RoleHeartbeat, VerbNudge, Allowed},

		// The separation semantics — the reason the matrix exists.
		{"orchestrator CANNOT edit (control/data-plane separation; injection defense)", RoleOrchestrator, VerbEdit, NeedsAdmission},
		{"verifier CANNOT edit (no-self-grade: never edit what you judge)", RoleVerifier, VerbEdit, NeedsAdmission},
		{"verifier CANNOT vote (judges, does not decide)", RoleVerifier, VerbVote, NeedsAdmission},
		{"worker CANNOT vote (executes, does not decide)", RoleWorker, VerbVote, NeedsAdmission},
		{"scribe CANNOT vote (records, does not decide)", RoleScribe, VerbVote, NeedsAdmission},
		{"scribe CANNOT edit", RoleScribe, VerbEdit, NeedsAdmission},
		{"heartbeat CANNOT edit (nudge only)", RoleHeartbeat, VerbEdit, NeedsAdmission},
		{"heartbeat CANNOT vote (nudge only)", RoleHeartbeat, VerbVote, NeedsAdmission},

		// Unknown role / unknown verb default to deny-and-escalate.
		{"unknown role denied", Role("admin"), VerbVote, NeedsAdmission},
		{"empty role denied", Role(""), VerbNudge, NeedsAdmission},
		{"unknown verb denied", RoleOrchestrator, Verb("delete-the-gate"), NeedsAdmission},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Authorize(tc.role, tc.verb); got != tc.want {
				t.Fatalf("Authorize(%q, %q) = %q, want %q", tc.role, tc.verb, got, tc.want)
			}
		})
	}
}

func TestCan(t *testing.T) {
	if !Can(RoleWorker, VerbEdit) {
		t.Fatalf("Can(worker, edit) = false, want true")
	}
	if Can(RoleOrchestrator, VerbEdit) {
		t.Fatalf("Can(orchestrator, edit) = true, want false (orchestrators do not edit)")
	}
}

func TestCapabilities(t *testing.T) {
	got := Capabilities(RoleOrchestrator)
	want := []Verb{VerbRoute, VerbShepherd, VerbSynthesize, VerbVote} // sorted
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Capabilities(orchestrator) = %v, want %v", got, want)
	}
	if got := Capabilities(RoleVerifier); !reflect.DeepEqual(got, []Verb{VerbJudge}) {
		t.Fatalf("Capabilities(verifier) = %v, want [judge]", got)
	}
	if got := Capabilities(Role("admin")); got != nil {
		t.Fatalf("Capabilities(unknown) = %v, want nil", got)
	}
}

func TestRoles(t *testing.T) {
	got := Roles()
	want := []Role{RoleHeartbeat, RoleOrchestrator, RoleScribe, RoleVerifier, RoleWorker} // sorted
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Roles() = %v, want %v", got, want)
	}
}

func TestIsRole(t *testing.T) {
	if !IsRole(RoleHeartbeat) {
		t.Fatalf("IsRole(heartbeat) = false, want true")
	}
	if IsRole(Role("admin")) {
		t.Fatalf("IsRole(admin) = true, want false")
	}
}
