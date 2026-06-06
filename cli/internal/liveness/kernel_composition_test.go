package liveness

import (
	"errors"
	"testing"
	"time"
)

// TestKernelComposition_FullAdmissionFlow exercises the liveness files together
// — inbound admission (AdmitInboundWorkMessage) + roles (Authorize) + guards
// (Disjoint) + request (Check) + quorum (CheckSignificantAction) + work_lease
// (LeaseTracker) — as one end-to-end admission lifecycle. The pieces were built
// across separate PRs by separate contexts (#733 roles, #737 quorum/guards-via-
// binder, #739 work_lease); this L2 test is the mechanical proof that they
// compose into a single coherent control plane on one vocabulary, not parallel
// ones. If a future change breaks the composition (e.g. Check's VerbJudge stops
// routing through Disjoint, or the quorum drops family-diversity), this test
// fails where the per-file unit tests would not.
func TestKernelComposition_FullAdmissionFlow(t *testing.T) {
	base := time.Date(2026, 6, 5, 4, 0, 0, 0, time.UTC)
	clk := base
	leases := NewLeaseTracker(func() time.Time { return clk })

	// 0) INBOUND ADMISSION (admission.go): a relayed work command from another
	//    host is not a directive, even if the transport authenticated the sender.
	//    It is downgraded to a proposal and cannot execute reflexively.
	injection := AdmitInboundWorkMessage(InboundWorkMessage{
		SenderID:      "relay-mac",
		SourceKind:    InboundSourceOtherHost,
		Authenticated: true,
		Intent:        InboundIntentDirective,
	})
	if injection.CanExecute() || injection.Decision != NeedsAdmission || injection.Action != AdmissionPropose || injection.Intent != InboundIntentProposal {
		t.Fatalf("other-host inbound directive = %+v, want proposal/non-executable", injection)
	}

	// 1) ROLE-MATRIX (roles.go): separation of duties. A worker edits; a worker
	//    may not self-vote; an orchestrator routes but may not edit (injection
	//    defense). Out-of-capability escalates, never allows.
	if got := Authorize(RoleWorker, VerbEdit); got != Allowed {
		t.Fatalf("worker edit: want Allowed, got %s", got)
	}
	if got := Authorize(RoleWorker, VerbVote); got != NeedsAdmission {
		t.Fatalf("worker self-vote: want NeedsAdmission, got %s", got)
	}
	if got := Authorize(RoleOrchestrator, VerbEdit); got != NeedsAdmission {
		t.Fatalf("orchestrator edit (injection defense): want NeedsAdmission, got %s", got)
	}

	// 2) EXTERNAL ROLE SOURCE (request.go + guards.go): a role is authority only
	//    if externally sourced. The same worker edit passes when leased, and is
	//    Denied the instant the role is self-asserted (source == actor).
	worker := AuthorizationRequest{
		AgentID: "worker-1", Role: RoleWorker, RoleSource: "lease",
		Verb: VerbEdit, ModelFamily: "claude",
	}
	if got := Check(worker); got != Allowed {
		t.Fatalf("externally-leased worker edit: want Allowed, got %s", got)
	}
	selfAsserted := worker
	selfAsserted.RoleSource = "worker-1" // source == actor → self-asserted authority
	if got := Check(selfAsserted); got != Denied {
		t.Fatalf("self-asserted role: want Denied, got %s", got)
	}

	// 3) WORK-LEASE (work_lease.go): the worker takes a lease on its bead, keyed
	//    by the externally-sourced identity that just passed Check.
	lease, err := leases.Create("ag-bead", "feat/ag-bead", worker.AgentID, worker.ModelFamily, "build", "ev-1", time.Hour)
	if err != nil {
		t.Fatalf("lease create: %v", err)
	}
	if lease.Status(clk) != LeaseActive {
		t.Fatalf("fresh lease: want active, got %s", lease.Status(clk))
	}

	// 4) AUTHOR != JUDGE (guards.go Disjoint, exercised through request.go Check
	//    on VerbJudge): the worker cannot judge its own artifact; a distinct
	//    verifier can. The high-level Check and the low-level Disjoint must agree.
	selfJudge := AuthorizationRequest{
		AgentID: "worker-1", Role: RoleVerifier, RoleSource: "operator",
		Verb: VerbJudge, ArtifactAuthorID: "worker-1", ModelFamily: "claude",
	}
	if got := Check(selfJudge); got != Denied {
		t.Fatalf("self-judge (author==judge): want Denied, got %s", got)
	}
	crossJudge := selfJudge
	crossJudge.AgentID = "verifier-2"
	if got := Check(crossJudge); got != Allowed {
		t.Fatalf("distinct verifier judging worker's artifact: want Allowed, got %s", got)
	}
	if Disjoint("worker-1", "worker-1") != Denied || Disjoint("worker-1", "verifier-2") != Allowed {
		t.Fatal("Disjoint primitive disagrees with Check VerbJudge — vocabularies diverged")
	}

	// 5) CROSS-MODEL QUORUM (quorum.go): merging the worker's result to main is a
	//    significant action. Solo → NeedsAdmission; the actor's own ACK does not
	//    count; two SAME-family ACKs do not clear (collusion guard); two distinct
	//    families clear.
	merge := SignificantActionRequest{ActorID: "worker-1", Action: SignificantActionMergeMain}
	if got := CheckSignificantAction(merge); got != NeedsAdmission {
		t.Fatalf("solo merge-main: want NeedsAdmission, got %s", got)
	}
	selfAck := merge
	selfAck.ACKs = []QuorumACK{{AgentID: "worker-1", ModelFamily: "claude", Verdict: ACKVerdictApprove}}
	if got := CheckSignificantAction(selfAck); got != NeedsAdmission {
		t.Fatalf("actor self-ACK must not count: want NeedsAdmission, got %s", got)
	}
	sameFamily := merge
	sameFamily.ACKs = []QuorumACK{
		{AgentID: "rev-a", ModelFamily: "claude", Verdict: ACKVerdictApprove},
		{AgentID: "rev-b", ModelFamily: "claude", Verdict: ACKVerdictApprove},
	}
	if got := CheckSignificantAction(sameFamily); got != NeedsAdmission {
		t.Fatalf("two same-family ACKs (collusion guard): want NeedsAdmission, got %s", got)
	}
	crossFamily := merge
	crossFamily.ACKs = []QuorumACK{
		{AgentID: "rev-a", ModelFamily: "claude", Verdict: ACKVerdictApprove},
		{AgentID: "rev-b", ModelFamily: "codex", Verdict: ACKVerdictApprove},
	}
	if got := CheckSignificantAction(crossFamily); got != Allowed {
		t.Fatalf("two cross-family ACKs: want Allowed, got %s", got)
	}
	quorumInbound := AdmitInboundWorkMessage(InboundWorkMessage{
		SenderID:                 "quorum-log",
		SourceKind:               InboundSourceQuorum,
		Authenticated:            true,
		Intent:                   InboundIntentDirective,
		SignificantAction:        SignificantActionMergeMain,
		SignificantActionRequest: crossFamily,
	})
	if !quorumInbound.CanExecute() {
		t.Fatalf("cross-family quorum inbound directive = %+v, want executable", quorumInbound)
	}

	// 6) HARD EXPIRY + TAKEOVER (work_lease.go): the worker goes dark, its lease
	//    expires, late renewal is rejected (expiry is a hard state), the deadman
	//    sweep surfaces it, and a distinct owner takes over via Create. Mechanical
	//    handoff — no scheduler luck.
	clk = base.Add(2 * time.Hour)
	if _, err := leases.Renew("ag-bead", "ev-2", time.Hour); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("late renewal of expired lease: want ErrLeaseExpired, got %v", err)
	}
	expired := leases.Expired()
	if len(expired) != 1 || expired[0].BeadID != "ag-bead" {
		t.Fatalf("deadman sweep: want [ag-bead] expired, got %+v", expired)
	}
	if _, err := leases.Create("ag-bead", "feat/ag-bead-2", "worker-3", "codex", "takeover", "ev-3", time.Hour); err != nil {
		t.Fatalf("quorum takeover Create over expired lease: %v", err)
	}
}
