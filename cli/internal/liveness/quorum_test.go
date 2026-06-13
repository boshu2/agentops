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
		ActorID:        "sageharbor",
		ActorContextID: "ctx-author",
		Action:         SignificantActionMergeMain,
		ACKs: []QuorumACK{
			{AgentID: "windycastle", ContextID: "ctx-judge-a", ModelFamily: "claude", Verdict: ACKVerdictApprove},
			{AgentID: "rubymoose", ContextID: "ctx-judge-b", ModelFamily: "openai", Verdict: ACKVerdictApprove},
		},
	})
	if got != Allowed {
		t.Fatalf("two distinct-context ACKs = %q, want %q", got, Allowed)
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
	// The author's own CONTEXT is excluded (not the author's model): the actor's
	// ACK carries the author context and must not count, leaving 1 distinct
	// non-author context, below the floor.
	got := CheckSignificantAction(SignificantActionRequest{
		ActorID:        "sageharbor",
		ActorContextID: "ctx-author",
		Action:         SignificantActionP0Bead,
		ACKs: []QuorumACK{
			{AgentID: "sageharbor", ContextID: "ctx-author", ModelFamily: "openai", Verdict: ACKVerdictApprove},
			{AgentID: "windycastle", ContextID: "ctx-judge-a", ModelFamily: "claude", Verdict: ACKVerdictApprove},
		},
	})
	if got != NeedsAdmission {
		t.Fatalf("actor self-context ACK counted: got %q, want %q", got, NeedsAdmission)
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
