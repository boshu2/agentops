package liveness

import "testing"

// Hardening (refuter P2): a single agent must not forge ">=2 distinct contexts"
// via whitespace/case/unicode variants of one context id.
func TestCheckSignificantActionContextIDCanonicalizationCollapsesVariants(t *testing.T) {
	res := CheckSignificantActionDetailed(SignificantActionRequest{
		ActorID:        "codex",
		ActorContextID: "ctx-author",
		Action:         SignificantActionMergeMain,
		ACKs: []QuorumACK{
			{AgentID: "j1", ContextID: "ctx-judge", ModelFamily: "claude", Verdict: ACKVerdictApprove},
			// Same context, only differing by trailing NBSP + case: must collapse.
			{AgentID: "j2", ContextID: "CTX-JUDGE ", ModelFamily: "openai", Verdict: ACKVerdictApprove},
		},
	})
	if res.DistinctNonAuthorContexts != 1 {
		t.Fatalf("DistinctNonAuthorContexts = %d, want 1 (whitespace/case/unicode variants collapse)", res.DistinctNonAuthorContexts)
	}
	if res.Decision != NeedsAdmission {
		t.Fatalf("Decision = %q, want %q (one real context cannot clear the floor)", res.Decision, NeedsAdmission)
	}
}

// Hardening (refuter P2 positive case): two genuinely distinct contexts still pass.
func TestCheckSignificantActionDistinctCanonicalContextsStillPass(t *testing.T) {
	res := CheckSignificantActionDetailed(SignificantActionRequest{
		ActorID:        "codex",
		ActorContextID: "ctx-author",
		Action:         SignificantActionMergeMain,
		ACKs: []QuorumACK{
			{AgentID: "j1", ContextID: " ctx-a ", ModelFamily: "claude", Verdict: ACKVerdictApprove},
			{AgentID: "j2", ContextID: "ctx-b", ModelFamily: "openai", Verdict: ACKVerdictApprove},
		},
	})
	if res.Decision != Allowed || res.DistinctNonAuthorContexts != 2 {
		t.Fatalf("two distinct contexts (one padded): Decision=%q n=%d, want Allowed/2", res.Decision, res.DistinctNonAuthorContexts)
	}
}

// Hardening (refuter P3): with ActorContextID empty but ActorID set, the author's
// own ACK must still be excluded by AgentID — it cannot count toward its floor.
func TestCheckSignificantActionAuthorExcludedByAgentIDWhenContextEmpty(t *testing.T) {
	res := CheckSignificantActionDetailed(SignificantActionRequest{
		ActorID:        "worker-1",
		ActorContextID: "", // unknown author context
		Action:         SignificantActionMergeMain,
		ACKs: []QuorumACK{
			// Author's own ACK with a non-empty context — must NOT count (AgentID match).
			{AgentID: "worker-1", ContextID: "ctx-self", ModelFamily: "claude", Verdict: ACKVerdictApprove},
			{AgentID: "rev-a", ContextID: "ctx-a", ModelFamily: "openai", Verdict: ACKVerdictApprove},
		},
	})
	if res.DistinctNonAuthorContexts != 1 {
		t.Fatalf("DistinctNonAuthorContexts = %d, want 1 (author ACK excluded by AgentID)", res.DistinctNonAuthorContexts)
	}
	if res.Decision != NeedsAdmission {
		t.Fatalf("Decision = %q, want %q (only one non-author context)", res.Decision, NeedsAdmission)
	}
}

// CanonicalizeContextID unit coverage.
func TestCanonicalizeContextID(t *testing.T) {
	cases := map[string]string{
		"ctx-1":        "ctx-1",
		"  ctx-1  ":    "ctx-1",
		"CTX-1":        "ctx-1",
		"ctx-1 ":  "ctx-1",
		"ctx 1\tx":     "ctx 1 x",
		"":             "",
		"   ":          "",
	}
	for in, want := range cases {
		if got := CanonicalizeContextID(in); got != want {
			t.Fatalf("CanonicalizeContextID(%q) = %q, want %q", in, got, want)
		}
	}
}
