package liveness

import "testing"

func TestAdmitInboundWorkMessagePeerDirectiveDowngradedToProposal(t *testing.T) {
	got := AdmitInboundWorkMessage(InboundWorkMessage{
		SenderID:      "sageharbor",
		SourceKind:    InboundSourcePeer,
		Authenticated: true,
		Intent:        InboundIntentDirective,
	})
	if got.Decision != NeedsAdmission || got.Action != AdmissionPropose || got.Intent != InboundIntentProposal {
		t.Fatalf("peer directive = %+v, want NeedsAdmission/propose/proposal", got)
	}
	if got.CanExecute() {
		t.Fatalf("peer directive must not execute reflexively: %+v", got)
	}
}

func TestAdmitInboundWorkMessageOtherHostInjectionCannotExecute(t *testing.T) {
	got := AdmitInboundWorkMessage(InboundWorkMessage{
		SenderID:      "relay-bushido",
		SourceKind:    InboundSourceOtherHost,
		Authenticated: true,
		Intent:        InboundIntentDirective,
	})
	if got.CanExecute() {
		t.Fatalf("relayed other-host work request executed reflexively: %+v", got)
	}
	if got.Decision != NeedsAdmission || got.Action != AdmissionPropose || got.Intent != InboundIntentProposal {
		t.Fatalf("relayed other-host directive = %+v, want admitted as proposal", got)
	}
}

func TestAdmitInboundWorkMessageTmuxInjectionCannotExecute(t *testing.T) {
	got := AdmitInboundWorkMessage(InboundWorkMessage{
		SenderID:      "ao-1",
		SourceKind:    InboundSourceTmux,
		Authenticated: false,
		Intent:        InboundIntentDirective,
	})
	if got.CanExecute() {
		t.Fatalf("tmux inbound text executed reflexively: %+v", got)
	}
	if got.Decision != NeedsAdmission || got.Action != AdmissionPropose || got.Intent != InboundIntentProposal {
		t.Fatalf("tmux directive = %+v, want admitted as proposal", got)
	}
}

func TestAdmitInboundWorkMessageAuthenticatedOperatorDirectiveExecutes(t *testing.T) {
	got := AdmitInboundWorkMessage(InboundWorkMessage{
		SenderID:      "operator",
		SourceKind:    InboundSourceOperator,
		Authenticated: true,
		Intent:        InboundIntentDirective,
	})
	if !got.CanExecute() {
		t.Fatalf("authenticated operator directive = %+v, want executable", got)
	}
}

func TestAdmitInboundWorkMessageOperatorMustAuthenticate(t *testing.T) {
	got := AdmitInboundWorkMessage(InboundWorkMessage{
		SenderID:      "operator",
		SourceKind:    InboundSourceOperator,
		Authenticated: false,
		Intent:        InboundIntentDirective,
	})
	if got.Decision != Denied || got.Action != AdmissionReject || got.CanExecute() {
		t.Fatalf("unauthenticated operator claim = %+v, want denied/reject/non-executable", got)
	}
}

func TestAdmitInboundWorkMessageQuorumDirectiveRequiresCrossModelACKs(t *testing.T) {
	req := SignificantActionRequest{
		ActorID:        "orchestrator-1",
		ActorContextID: "ctx-author",
		Action:         SignificantActionMergeMain,
		ACKs: []QuorumACK{
			{AgentID: "windycastle", ContextID: "ctx-judge-a", ModelFamily: "claude", Verdict: ACKVerdictApprove},
			{AgentID: "rubymoose", ContextID: "ctx-judge-b", ModelFamily: "openai", Verdict: ACKVerdictApprove},
		},
	}
	got := AdmitInboundWorkMessage(InboundWorkMessage{
		SenderID:                 "quorum-log",
		SourceKind:               InboundSourceQuorum,
		Authenticated:            true,
		Intent:                   InboundIntentDirective,
		SignificantAction:        SignificantActionMergeMain,
		SignificantActionRequest: req,
	})
	if !got.CanExecute() {
		t.Fatalf("cross-model quorum directive = %+v, want executable", got)
	}
}

func TestAdmitInboundWorkMessageIncompleteQuorumStaysProposal(t *testing.T) {
	got := AdmitInboundWorkMessage(InboundWorkMessage{
		SenderID:          "quorum-log",
		SourceKind:        InboundSourceQuorum,
		Authenticated:     true,
		Intent:            InboundIntentDirective,
		SignificantAction: SignificantActionMergeMain,
		SignificantActionRequest: SignificantActionRequest{
			ActorID: "orchestrator-1",
			Action:  SignificantActionMergeMain,
			ACKs: []QuorumACK{
				{AgentID: "windycastle", ModelFamily: "claude", Verdict: ACKVerdictApprove},
			},
		},
	})
	if got.Decision != NeedsAdmission || got.Action != AdmissionPropose || got.CanExecute() {
		t.Fatalf("incomplete quorum directive = %+v, want proposal/non-executable", got)
	}
}

func TestAdmitInboundWorkMessageQuorumSourceAloneDoesNotExecute(t *testing.T) {
	got := AdmitInboundWorkMessage(InboundWorkMessage{
		SenderID:      "quorum-log",
		SourceKind:    InboundSourceQuorum,
		Authenticated: true,
		Intent:        InboundIntentDirective,
	})
	if got.Decision != NeedsAdmission || got.Action != AdmissionPropose || got.CanExecute() {
		t.Fatalf("unratified quorum source = %+v, want proposal/non-executable", got)
	}
}

func TestAdmitInboundWorkMessageInfoRecordsButDoesNotExecute(t *testing.T) {
	got := AdmitInboundWorkMessage(InboundWorkMessage{
		SenderID:      "peer-1",
		SourceKind:    InboundSourcePeer,
		Authenticated: true,
		Intent:        InboundIntentInfo,
	})
	if got.Decision != Allowed || got.Action != AdmissionRecord || got.CanExecute() {
		t.Fatalf("peer info = %+v, want record-only", got)
	}
}

func TestAdmitInboundWorkMessageUnknownSourceRejected(t *testing.T) {
	got := AdmitInboundWorkMessage(InboundWorkMessage{
		SenderID:      "unknown",
		SourceKind:    InboundSourceKind("slack-paste"),
		Authenticated: true,
		Intent:        InboundIntentDirective,
	})
	if got.Decision != Denied || got.Action != AdmissionReject || got.CanExecute() {
		t.Fatalf("unknown source = %+v, want denied/reject/non-executable", got)
	}
}
