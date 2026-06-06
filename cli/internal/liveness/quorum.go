package liveness

// SignificantAction is a control-plane action that must not be performed by one
// orchestrator acting alone.
type SignificantAction string

const (
	SignificantActionMergeMain          SignificantAction = "merge-main"
	SignificantActionDelete             SignificantAction = "delete"
	SignificantActionArchitectureChange SignificantAction = "architecture-change"
	SignificantActionProductNorthStar   SignificantAction = "product-north-star"
	SignificantActionP0Bead             SignificantAction = "p0-bead"
	SignificantActionDocCanonization    SignificantAction = "doc-canonization"
)

// ACKVerdict records whether a quorum participant approved a significant action.
type ACKVerdict string

const (
	ACKVerdictApprove ACKVerdict = "ACK"
	ACKVerdictBlock   ACKVerdict = "BLOCK"
)

// QuorumACK is one recorded cross-model ACK from the consensus log.
type QuorumACK struct {
	AgentID     string
	ModelFamily string
	Verdict     ACKVerdict
}

// SignificantActionRequest is the orchestration-tier gate input. ActorID is the
// orchestrator attempting the action; ACKs are the already-recorded quorum
// approvals, for example from Agent Mail or bd.
type SignificantActionRequest struct {
	ActorID string
	Action  SignificantAction
	ACKs    []QuorumACK
}

// IsSignificantAction reports whether action needs cross-model quorum.
func IsSignificantAction(action SignificantAction) bool {
	switch action {
	case SignificantActionMergeMain,
		SignificantActionDelete,
		SignificantActionArchitectureChange,
		SignificantActionProductNorthStar,
		SignificantActionP0Bead,
		SignificantActionDocCanonization:
		return true
	default:
		return false
	}
}

// CheckSignificantAction enforces the orchestration-tier no-self-grade rule. A
// non-significant action is allowed. A significant action needs at least two
// distinct non-actor ACKs spanning at least two model families; otherwise it must
// be routed to admission instead of executed.
func CheckSignificantAction(req SignificantActionRequest) Decision {
	if !IsSignificantAction(req.Action) {
		return Allowed
	}
	if req.ActorID == "" {
		return Denied
	}
	agents := map[string]struct{}{}
	families := map[string]struct{}{}
	for _, ack := range req.ACKs {
		if ack.Verdict != ACKVerdictApprove {
			continue
		}
		if ack.AgentID == "" || ack.ModelFamily == "" || ack.AgentID == req.ActorID {
			continue
		}
		agents[ack.AgentID] = struct{}{}
		families[ack.ModelFamily] = struct{}{}
	}
	if len(agents) >= 2 && len(families) >= 2 {
		return Allowed
	}
	return NeedsAdmission
}
