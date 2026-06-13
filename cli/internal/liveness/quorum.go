package liveness

import "strings"

// CanonicalizeContextID normalizes a context identifier before it is compared or
// de-duplicated, so a single agent cannot forge "distinct" contexts via trivial
// string variation (leading/trailing or internal whitespace, case, or
// whitespace-class differences such as NBSP/tab). Two ids that differ only by
// these collapse to one context. This is the load-bearing canonicalization: the
// independence floor counts DISTINCT contexts, so the distinctness test must not
// be fooled by cosmetic variants.
func CanonicalizeContextID(id string) string {
	// strings.Fields splits on any unicode whitespace (incl. NBSP, tabs,
	// newlines) and drops empties, so re-joining with a single space collapses
	// all whitespace-class and run differences to one canonical form.
	return strings.ToLower(strings.Join(strings.Fields(id), " "))
}

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

// QuorumACK is one recorded ACK from the consensus log. ContextID is the
// session-identity axis: two distinct ContextIDs are two effectively-independent
// judges even when they share a model. An ACK with an empty ContextID predates
// the context axis and is NOT counted toward the floor (see
// CheckSignificantActionDetailed) — the context, not the model, is what makes a
// judge independent.
type QuorumACK struct {
	AgentID     string
	ModelFamily string
	ContextID   string
	Verdict     ACKVerdict
}

// SignificantActionRequest is the orchestration-tier gate input. ActorID is the
// orchestrator attempting the action; ActorContextID is the author's CONTEXT —
// the axis that is excluded from the floor (never the author's model). ACKs are
// the already-recorded quorum approvals, for example from Agent Mail or bd.
// RequireCrossFamily is an OPTIONAL strengthener: off by default (the context
// floor IS the floor); on = additionally require >=2 distinct model families
// among the counted contexts.
type SignificantActionRequest struct {
	ActorID            string
	ActorContextID     string
	Action             SignificantAction
	RequireCrossFamily bool
	ACKs               []QuorumACK
}

// CheckSignificantActionResult is the detailed verdict of the context-quorum
// gate. DistinctNonAuthorContexts is the count of distinct approving contexts
// after excluding the author context and de-duplicating shared contexts.
// UnmetReason is populated only when Decision != Allowed.
type CheckSignificantActionResult struct {
	Decision                  Decision
	DistinctNonAuthorContexts int
	UnmetReason               string
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

// CheckSignificantAction enforces the orchestration-tier no-self-grade rule and
// is a thin wrapper over CheckSignificantActionDetailed (kept for existing
// callers that only need the Decision).
func CheckSignificantAction(req SignificantActionRequest) Decision {
	return CheckSignificantActionDetailed(req).Decision
}

// CheckSignificantActionDetailed is the context-quorum gate. The independence
// axis is FRESH CONTEXT, not model family: a significant action needs at least
// two distinct NON-AUTHOR contexts that APPROVE. The author's CONTEXT is
// excluded (never the author's model — the producing model may judge its own
// work from a fresh context). model_family is demoted to an OPTIONAL
// RequireCrossFamily strengthener.
//
// Counting rules for an ACK to contribute one distinct context:
//   - Verdict must be ACKVerdictApprove (BLOCK never counts).
//   - ACK.AgentID must not equal req.ActorID (belt-and-suspenders author
//     exclusion that holds even when ActorContextID is unknown).
//   - ContextID (after canonicalization) must be non-empty. A legacy ACK with an
//     empty ContextID predates the context axis and is NOT counted (fail-closed:
//     an unidentifiable context cannot be proven independent).
//   - Canonicalized ContextID must differ from the canonicalized
//     req.ActorContextID (the author's context is excluded).
//
// ContextIDs are canonicalized (CanonicalizeContextID) before dedup so a single
// agent cannot forge "distinct" contexts via whitespace/case/unicode variants.
// Distinct contexts are de-duplicated, so two ACKs sharing one context are one
// judge. Author MODEL is never a basis for exclusion — only the context/actor.
func CheckSignificantActionDetailed(req SignificantActionRequest) CheckSignificantActionResult {
	if !IsSignificantAction(req.Action) {
		return CheckSignificantActionResult{Decision: Allowed}
	}
	// Cannot exclude the author's context from the floor if we do not know it.
	if req.ActorID == "" && req.ActorContextID == "" {
		return CheckSignificantActionResult{
			Decision:    Denied,
			UnmetReason: "cannot identify author context",
		}
	}

	authorContext := CanonicalizeContextID(req.ActorContextID)
	contexts := map[string]struct{}{}
	families := map[string]struct{}{}
	for _, ack := range req.ACKs {
		if ack.Verdict != ACKVerdictApprove {
			continue
		}
		// Belt-and-suspenders author exclusion: drop an ACK whose AgentID is the
		// actor even when ActorContextID is unknown — otherwise an author with an
		// empty ActorContextID could count its own ACK toward the floor.
		if req.ActorID != "" && ack.AgentID == req.ActorID {
			continue
		}
		ctx := CanonicalizeContextID(ack.ContextID)
		// Empty (or empty-after-canonicalization) context cannot be proven
		// independent; the author's own context is excluded.
		if ctx == "" || ctx == authorContext {
			continue
		}
		contexts[ctx] = struct{}{}
		if ack.ModelFamily != "" {
			families[ack.ModelFamily] = struct{}{}
		}
	}
	n := len(contexts)

	if n < 2 {
		return CheckSignificantActionResult{
			Decision:                  NeedsAdmission,
			DistinctNonAuthorContexts: n,
			UnmetReason:               "context floor: need >=2 distinct non-author contexts",
		}
	}
	if req.RequireCrossFamily && len(families) < 2 {
		return CheckSignificantActionResult{
			Decision:                  NeedsAdmission,
			DistinctNonAuthorContexts: n,
			UnmetReason:               "cross-family strengthener: need >=2 model families",
		}
	}
	return CheckSignificantActionResult{
		Decision:                  Allowed,
		DistinctNonAuthorContexts: n,
	}
}
