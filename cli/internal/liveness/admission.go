package liveness

// InboundSourceKind identifies where an inbound work message entered the
// control plane. Transport names are intentionally coarse: the admission rule is
// about authority class, not a particular substrate implementation.
type InboundSourceKind string

const (
	InboundSourceOperator  InboundSourceKind = "operator"
	InboundSourceQuorum    InboundSourceKind = "quorum"
	InboundSourcePeer      InboundSourceKind = "peer"
	InboundSourceOtherHost InboundSourceKind = "other-host"
	InboundSourceTmux      InboundSourceKind = "tmux"
	InboundSourceUnknown   InboundSourceKind = "unknown"
)

// InboundIntent is the claimed intent of a message after local classification.
type InboundIntent string

const (
	InboundIntentUnknown   InboundIntent = "unknown"
	InboundIntentDirective InboundIntent = "directive"
	InboundIntentProposal  InboundIntent = "proposal"
	InboundIntentInfo      InboundIntent = "info"
)

// AdmissionAction is the side effect class a caller may take after admission.
type AdmissionAction string

const (
	AdmissionExecute AdmissionAction = "execute"
	AdmissionPropose AdmissionAction = "propose"
	AdmissionRecord  AdmissionAction = "record"
	AdmissionReject  AdmissionAction = "reject"
)

// InboundWorkMessage is the substrate-neutral input to the inbound half of the
// membrane. Authenticated means the transport verified the claimed sender; it is
// not by itself authority to execute.
type InboundWorkMessage struct {
	SenderID      string
	SourceKind    InboundSourceKind
	Authenticated bool
	Intent        InboundIntent

	// QuorumRatified means the caller verified that this directive came from a
	// ratified quorum record. For significant actions, prefer supplying the
	// ACK-bearing SignificantActionRequest below so this package can verify the
	// cross-model threshold directly.
	QuorumRatified bool

	// SignificantAction is optional. When a quorum-sourced directive names one,
	// SignificantActionRequest must also be populated so the quorum primitive can
	// verify cross-model ACKs before execution.
	SignificantAction        SignificantAction
	SignificantActionRequest SignificantActionRequest
}

// InboundAdmission is the complete inbound admission verdict. Intent records
// the post-classification intent, so callers can show when a relayed directive
// was downgraded to a proposal.
type InboundAdmission struct {
	Decision Decision
	Action   AdmissionAction
	Intent   InboundIntent
	Reason   string
}

// CanExecute reports whether the admission verdict permits immediate execution.
func (a InboundAdmission) CanExecute() bool {
	return a.Decision == Allowed && a.Action == AdmissionExecute
}

// AdmitInboundWorkMessage classifies and gates an inbound work message. The
// invariant is intentionally narrow and mechanical: only authenticated operator
// directives or verified quorum directives may execute. Peer, other-host, and
// tmux messages are proposals even when authenticated; they are never executed
// reflexively.
func AdmitInboundWorkMessage(msg InboundWorkMessage) InboundAdmission {
	source := normalizeInboundSource(msg.SourceKind)
	intent := normalizeInboundIntent(msg.Intent)

	switch source {
	case InboundSourceOperator:
		if !msg.Authenticated || msg.SenderID == "" {
			return inboundDenied(intent, "operator source must be authenticated")
		}
		if intent != InboundIntentDirective {
			return inboundNeedsAdmission(intent, AdmissionPropose, "operator non-directive remains non-executable")
		}
		return InboundAdmission{Decision: Allowed, Action: AdmissionExecute, Intent: intent, Reason: "authenticated operator directive"}

	case InboundSourceQuorum:
		if !msg.Authenticated || msg.SenderID == "" {
			return inboundDenied(intent, "quorum source must be authenticated")
		}
		if intent != InboundIntentDirective {
			return inboundNeedsAdmission(intent, AdmissionPropose, "quorum non-directive remains non-executable")
		}
		action := msg.SignificantAction
		req := msg.SignificantActionRequest
		if action == "" {
			action = req.Action
		}
		if action == "" {
			if msg.QuorumRatified {
				return InboundAdmission{Decision: Allowed, Action: AdmissionExecute, Intent: intent, Reason: "ratified quorum directive"}
			}
			return inboundNeedsAdmission(intent, AdmissionPropose, "quorum directive lacks ratification")
		}
		if !IsSignificantAction(action) {
			if msg.QuorumRatified {
				return InboundAdmission{Decision: Allowed, Action: AdmissionExecute, Intent: intent, Reason: "ratified quorum directive"}
			}
			return inboundNeedsAdmission(intent, AdmissionPropose, "non-significant quorum directive lacks ratification")
		}
		if req.Action == "" {
			req.Action = action
		}
		if CheckSignificantAction(req) == Allowed {
			return InboundAdmission{Decision: Allowed, Action: AdmissionExecute, Intent: intent, Reason: "cross-model quorum directive"}
		}
		return inboundNeedsAdmission(intent, AdmissionPropose, "quorum directive lacks required cross-model ACKs")

	case InboundSourcePeer, InboundSourceOtherHost, InboundSourceTmux:
		if intent == InboundIntentInfo {
			return InboundAdmission{Decision: Allowed, Action: AdmissionRecord, Intent: intent, Reason: "informational inbound message"}
		}
		return inboundNeedsAdmission(InboundIntentProposal, AdmissionPropose, "untrusted work message admitted only as proposal")

	default:
		return inboundDenied(intent, "unknown inbound source")
	}
}

func normalizeInboundSource(source InboundSourceKind) InboundSourceKind {
	if source == "" {
		return InboundSourceUnknown
	}
	switch source {
	case InboundSourceOperator, InboundSourceQuorum, InboundSourcePeer, InboundSourceOtherHost, InboundSourceTmux:
		return source
	default:
		return InboundSourceUnknown
	}
}

func normalizeInboundIntent(intent InboundIntent) InboundIntent {
	if intent == "" {
		return InboundIntentUnknown
	}
	switch intent {
	case InboundIntentDirective, InboundIntentProposal, InboundIntentInfo:
		return intent
	default:
		return InboundIntentUnknown
	}
}

func inboundNeedsAdmission(intent InboundIntent, action AdmissionAction, reason string) InboundAdmission {
	return InboundAdmission{Decision: NeedsAdmission, Action: action, Intent: intent, Reason: reason}
}

func inboundDenied(intent InboundIntent, reason string) InboundAdmission {
	return InboundAdmission{Decision: Denied, Action: AdmissionReject, Intent: intent, Reason: reason}
}
