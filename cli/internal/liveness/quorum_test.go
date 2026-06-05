package liveness

import "testing"

func TestCheckSignificantActionSoloCommitNeedsAdmission(t *testing.T) {
	got := CheckSignificantAction(SignificantActionRequest{
		ActorID: "sageharbor",
		Action:  SignificantActionMergeMain,
		ACKs:    nil,
	})
	if got != NeedsAdmission {
		t.Fatalf("solo significant action = %q, want %q", got, NeedsAdmission)
	}
}

func TestCheckSignificantActionTwoCrossModelACKsAllowed(t *testing.T) {
	got := CheckSignificantAction(SignificantActionRequest{
		ActorID: "sageharbor",
		Action:  SignificantActionMergeMain,
		ACKs: []QuorumACK{
			{AgentID: "windycastle", ModelFamily: "claude", Verdict: ACKVerdictApprove},
			{AgentID: "rubymoose", ModelFamily: "openai", Verdict: ACKVerdictApprove},
		},
	})
	if got != Allowed {
		t.Fatalf("two cross-model ACKs = %q, want %q", got, Allowed)
	}
}

func TestCheckSignificantActionSameFamilyNeedsAdmission(t *testing.T) {
	got := CheckSignificantAction(SignificantActionRequest{
		ActorID: "sageharbor",
		Action:  SignificantActionDelete,
		ACKs: []QuorumACK{
			{AgentID: "windycastle", ModelFamily: "claude", Verdict: ACKVerdictApprove},
			{AgentID: "rubymoose", ModelFamily: "claude", Verdict: ACKVerdictApprove},
		},
	})
	if got != NeedsAdmission {
		t.Fatalf("same-family ACKs = %q, want %q", got, NeedsAdmission)
	}
}

func TestCheckSignificantActionActorACKDoesNotCount(t *testing.T) {
	got := CheckSignificantAction(SignificantActionRequest{
		ActorID: "sageharbor",
		Action:  SignificantActionP0Bead,
		ACKs: []QuorumACK{
			{AgentID: "sageharbor", ModelFamily: "openai", Verdict: ACKVerdictApprove},
			{AgentID: "windycastle", ModelFamily: "claude", Verdict: ACKVerdictApprove},
		},
	})
	if got != NeedsAdmission {
		t.Fatalf("actor self-ACK counted: got %q, want %q", got, NeedsAdmission)
	}
}

func TestCheckSignificantActionNonSignificantAllowed(t *testing.T) {
	got := CheckSignificantAction(SignificantActionRequest{
		ActorID: "sageharbor",
		Action:  SignificantAction("nudge"),
	})
	if got != Allowed {
		t.Fatalf("non-significant action = %q, want %q", got, Allowed)
	}
}

func TestCheckSignificantActionMissingActorDenied(t *testing.T) {
	got := CheckSignificantAction(SignificantActionRequest{
		Action: SignificantActionArchitectureChange,
		ACKs: []QuorumACK{
			{AgentID: "windycastle", ModelFamily: "claude", Verdict: ACKVerdictApprove},
			{AgentID: "rubymoose", ModelFamily: "openai", Verdict: ACKVerdictApprove},
		},
	})
	if got != Denied {
		t.Fatalf("missing actor = %q, want %q", got, Denied)
	}
}
