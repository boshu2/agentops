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

// FLIP (S1.10): under the retired family-floor this asserted NeedsAdmission for
// two same-family ACKs. The independence axis is now FRESH CONTEXT, not model
// family, so two same-family ACKs that carry distinct non-author contexts PASS.
// (The staged acceptance file owns the canonically-named version; this in-package
// test keeps a distinct name to coexist with it.)
func TestCheckSignificantActionSameFamilyDistinctContextsAllowed(t *testing.T) {
	got := CheckSignificantAction(SignificantActionRequest{
		ActorID:        "sageharbor",
		ActorContextID: "ctx-author",
		Action:         SignificantActionDelete,
		ACKs: []QuorumACK{
			{AgentID: "windycastle", ContextID: "ctx-judge-1", ModelFamily: "claude", Verdict: ACKVerdictApprove},
			{AgentID: "rubymoose", ContextID: "ctx-judge-2", ModelFamily: "claude", Verdict: ACKVerdictApprove},
		},
	})
	if got != Allowed {
		t.Fatalf("same-family distinct-context ACKs = %q, want %q", got, Allowed)
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
