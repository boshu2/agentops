// Package delivery provides the optional, caller-selected GC delivery reducer.
// It deliberately has no dependency on ao commands or AgentOps ports.
package delivery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

const (
	preparedFile    = "handoff-prepared.json"
	nonRoutableFile = "delivery.non-routable.json"
	publishedFile   = "delivery.published.json"
	committedFile   = "handoff-committed.json"
)

var ErrCrash = errors.New("delivery reducer crash cut")

func IsCrash(err error) bool { return errors.Is(err, ErrCrash) }

// AdmissionCertificate is the strict subset required to admit delivery. The
// caller must pass the exact certificate digest calculated from its bytes.
type AdmissionCertificate struct {
	SchemaVersion       string       `json:"schema_version"`
	SemanticBeadID      string       `json:"semantic_bead_id"`
	IntentDigest        string       `json:"intent_digest"`
	Verdict             string       `json:"verdict"`
	Candidate           Candidate    `json:"candidate"`
	Store               Store        `json:"store"`
	ChangedPathManifest string       `json:"changed_path_manifest"`
	VerdictDigest       string       `json:"verdict_digest"`
	EvidenceDigest      string       `json:"evidence_digest"`
	Attestations        Attestations `json:"attestations"`
	DeliveryGroupID     string       `json:"delivery_group_id"`
	PrefixSafety        string       `json:"prefix_safety"`
}

type Candidate struct {
	Commit        string `json:"commit"`
	Tree          string `json:"tree"`
	ContentDigest string `json:"content_digest"`
}

type Store struct {
	Identity string `json:"identity"`
	Digest   string `json:"digest"`
}
type Attestations struct {
	Author    Runtime `json:"author"`
	Validator Runtime `json:"validator"`
}
type Runtime struct {
	ContextID          string    `json:"context_id"`
	RequestedModel     string    `json:"requested_model"`
	RequestedReasoning string    `json:"requested_reasoning"`
	RequestedProvider  string    `json:"requested_provider"`
	ActualModel        string    `json:"actual_model"`
	ActualReasoning    string    `json:"actual_reasoning"`
	ActualProvider     string    `json:"actual_provider"`
	ActualEffort       string    `json:"actual_effort"`
	Fallback           *Fallback `json:"fallback"`
}
type Fallback struct {
	Allowed *bool           `json:"allowed"`
	Used    *bool           `json:"used"`
	Reason  json.RawMessage `json:"reason"`
}

type Target struct {
	DeliveryBeadID      string
	SemanticBeadID      string
	SemanticTerminalRef string
	RigID               string
	Repository          string
	Remote              string
	Epoch               int
	Mode                string
	Deadline            string
	PreparedAt          string
	CommittedAt         string
	BaseRef             string
	BaseOID             string
	ObservedAt          string // optional caller-attested observation time for deadline reduction
}

type Request struct {
	Root              string
	Certificate       AdmissionCertificate
	CertificateBytes  []byte
	CertificateDigest string
	Target            Target
	SubjectManifest   SubjectManifest
	SubjectBytes      []byte
	SubjectDigest     string
	NativeContext     NativeContext
	NativeBytes       []byte
	NativeDigest      string
}

type Terminal struct {
	BeadID            string
	Ref               string
	Verdict           string
	CertificateDigest string
}

type DeliveryState string

const (
	DeliveryStateQueued            DeliveryState = "queued"
	DeliveryStatePreparing         DeliveryState = "preparing"
	DeliveryStateBranchReady       DeliveryState = "branch_ready"
	DeliveryStatePRPrepared        DeliveryState = "pr_prepared"
	DeliveryStatePROpen            DeliveryState = "pr_open"
	DeliveryStateCIWait            DeliveryState = "ci_wait"
	DeliveryStateRebaseNeeded      DeliveryState = "rebase_needed"
	DeliveryStateMergeEligible     DeliveryState = "merge_eligible"
	DeliveryStateMergeArmed        DeliveryState = "merge_armed"
	DeliveryStateLanded            DeliveryState = "landed"
	DeliveryStateRepairWait        DeliveryState = "repair_wait"
	DeliveryStateManualReview      DeliveryState = "manual_review"
	DeliveryStateStalled           DeliveryState = "stalled"
	DeliveryStateFailed            DeliveryState = "delivery_failed"
	DeliveryStateSuccessorRequired DeliveryState = "successor_required"
	DeliveryStateCancelled         DeliveryState = "cancelled"
)

// DeliveryRecord is the sole lifecycle authority. Receipts are immutable
// evidence and crash fences; they never select the next transition.
type DeliveryRecord struct {
	SchemaVersion            string        `json:"schema_version"`
	Revision                 int           `json:"revision"`
	HandoffID                string        `json:"handoff_id"`
	Epoch                    DeliveryEpoch `json:"epoch"`
	PR                       PullRequest   `json:"pr,omitempty"`
	State                    DeliveryState `json:"state"`
	Current                  ReceiptRef    `json:"current_receipt"`
	Publication              string        `json:"publication"`
	ReadyAt                  string        `json:"ready_at"`
	Deadline                 string        `json:"deadline"`
	SemanticBead             string        `json:"semantic_bead_id"`
	TerminalRef              string        `json:"semantic_terminal_ref"`
	Certificate              string        `json:"admission_certificate_digest"`
	Committed                string        `json:"committed_handoff_digest"`
	Mode                     string        `json:"mode"`
	Rig                      string        `json:"rig_id"`
	Repository               string        `json:"repository"`
	Remote                   string        `json:"remote"`
	Candidate                string        `json:"candidate_oid"`
	Manifest                 string        `json:"subject_manifest_digest"`
	GateDigest               string        `json:"gate_digest,omitempty"`
	ArmID                    string        `json:"arm_id,omitempty"`
	AutoMergeEffectID        string        `json:"auto_merge_effect_id,omitempty"`
	AutoMergeAttempt         ReceiptRef    `json:"auto_merge_attempt,omitempty"`
	LandingDigest            string        `json:"landing_digest,omitempty"`
	LandedSHA                string        `json:"landed_sha,omitempty"`
	DeadlineOutcome          string        `json:"deadline_outcome,omitempty"`
	DeliveryOutcome          string        `json:"delivery_outcome,omitempty"`
	EpochSuccessorID         string        `json:"epoch_successor_id,omitempty"`
	RepairBeadID             string        `json:"repair_bead_id,omitempty"`
	SemanticSuccessorID      string        `json:"semantic_successor_id,omitempty"`
	PredecessorReceiptDigest string        `json:"predecessor_receipt_digest,omitempty"`
	Predecessor              string        `json:"predecessor,omitempty"`
}
type DeliveryEpoch struct {
	Number   int    `json:"number"`
	BaseRef  string `json:"base_ref"`
	BaseOID  string `json:"base_oid"`
	Branch   string `json:"branch"`
	Head     string `json:"head,omitempty"`
	Tree     string `json:"tree,omitempty"`
	LeaseOID string `json:"lease_oid,omitempty"`
}
type ReceiptRef struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}
type DeliveryBead struct {
	ID          string
	ExternalRef string
	Route       string
	Record      DeliveryRecord
}

type Branch struct{ Name, BaseRef, BaseOID, Head, Tree, Proof, LeaseOID string }

// ID/EffectID are deterministic local identities; NodeID/Number/URL are the
// observed immutable forge identity recovered by exact branch on cold replay.
type PullRequest struct {
	ID         string `json:"id"`
	EffectID   string `json:"effect_id"`
	Repository string `json:"repository"`
	BaseRef    string `json:"base_ref"`
	Branch     string `json:"branch"`
	NodeID     string `json:"node_id,omitempty"`
	Number     string `json:"number,omitempty"`
	URL        string `json:"url,omitempty"`
}

// HostedGate is the immutable, app-qualified protection observation for one
// exact PR head.  A check name by itself is not authority: the provider must
// include the hosted application identity that produced it.
type HostedGate struct {
	Repository       string        `json:"repository"`
	BaseRef          string        `json:"base_ref"`
	BaseOID          string        `json:"base_oid"`
	Head             string        `json:"head"`
	PRState          string        `json:"pr_state"`
	Draft            bool          `json:"draft"`
	MergeState       string        `json:"merge_state"`
	AutoMergeEnabled bool          `json:"auto_merge_enabled"`
	Strict           bool          `json:"strict"`
	ProtectionDigest string        `json:"protection_digest"`
	RequiredChecks   []HostedCheck `json:"required_checks"`
	Checks           []HostedCheck `json:"checks"`
}
type HostedCheck struct {
	AppID      string `json:"app_id"`
	Context    string `json:"context"`
	Status     string `json:"status,omitempty"`
	Conclusion string `json:"conclusion,omitempty"`
}

// MergeArm is the only auto-merge intent. It binds both the observed hosted
// protection and the exact target/head tuple, making a later stale merge
// attempt unlawful rather than merely undesirable.
type MergeArm struct {
	ID               string `json:"id"`
	EffectID         string `json:"effect_id"`
	PRID             string `json:"pr_id"`
	Repository       string `json:"repository"`
	NodeID           string `json:"node_id"`
	Number           string `json:"number"`
	Branch           string `json:"branch"`
	Head             string `json:"head"`
	BaseRef          string `json:"base_ref"`
	BaseOID          string `json:"base_oid"`
	ProtectionDigest string `json:"protection_digest"`
	GateDigest       string `json:"gate_digest"`
}
type MergeObservation struct {
	State  string `json:"state"` // absent, armed, landed, refused, unknown
	Reason string `json:"reason,omitempty"`
}
type Landing struct {
	PRID    string   `json:"pr_id"`
	Head    string   `json:"head"`
	SHA     string   `json:"sha"`
	Tree    string   `json:"tree"`
	Parents []string `json:"parents"`
}

// Providers makes all mutable boundaries explicit. Production wiring may use
// its own caller-selected adapters; this thin slice proves only fake boundaries.
type Providers interface {
	Terminal(context.Context, string) (Terminal, error)
	FindDelivery(context.Context, string) (DeliveryBead, bool, error)
	CreateDelivery(context.Context, DeliveryBead) (DeliveryBead, error)
	PublishRoute(context.Context, string) error
	RetireRoute(context.Context, string) error
	// StoreTransition is read-check-write-reread, not CAS. The selected Order
	// licenses exactly one rig-scoped writer; callers must still supply the
	// complete observed record to reject stale or conflicting transitions.
	StoreTransition(context.Context, DeliveryBead, DeliveryRecord) (DeliveryBead, error)
	BaseDescends(context.Context, string, string) (bool, error)
	ObserveBase(context.Context, string) (string, error)
	FindBranch(context.Context, string) (Branch, bool, error)
	PrepareBranch(context.Context, Branch) (Branch, error)
}

type PRIntent struct {
	SchemaVersion string `json:"schema_version"`
	HandoffID     string `json:"handoff_id"`
	Epoch         int    `json:"epoch"`
	Repository    string `json:"repository"`
	BaseRef       string `json:"base_ref"`
	BaseOID       string `json:"base_oid"`
	Branch        string `json:"branch"`
	ExpectedHead  string `json:"expected_head"`
	PRID          string `json:"pr_id"`
	EffectID      string `json:"effect_id"`
	NodeID        string `json:"known_node_id,omitempty"`
	Number        string `json:"known_number,omitempty"`
	URL           string `json:"known_url,omitempty"`
}
type PRObservation struct {
	State   string      `json:"state"`
	Draft   bool        `json:"draft"`
	BaseOID string      `json:"base_oid"`
	Head    string      `json:"head"`
	PR      PullRequest `json:"pr"`
}
type PRProviders interface {
	ObservePR(context.Context, PRIntent) (PRObservation, error)
	CreatePR(context.Context, PRIntent) (PRObservation, error)
}

// EpochPusher is intentionally separate from local composition.  The reducer
// writes an immutable epoch receipt before this one remote branch effect.
type EpochPusher interface {
	PushBranch(context.Context, Branch) error
}

type SubjectVerifier interface {
	VerifySubject(context.Context, Request) error
}

type Result struct {
	Status string `json:"status"`
	Effect string `json:"effect,omitempty"`
	Reason string `json:"reason,omitempty"`
}
type Crash func(string) error

func CrashAt(cut string) Crash {
	return func(point string) error {
		if cut == point {
			return ErrCrash
		}
		return nil
	}
}

func AllCrashCuts() []string {
	return []string{
		"before_initial_certificate", "after_initial_certificate", "before_initial_prepared", "after_initial_prepared",
		"before_terminal", "after_terminal", "before_successor_create", "after_successor_create",
		"before_initial_payload", "after_initial_payload", "before_initial_committed", "after_initial_committed",
		"before_publication", "after_publication", "before_branch_push", "after_branch_push",
		"before_pr_intent", "after_pr_intent", "before_pr_prepare", "after_pr_prepare",
		"before_pr_create", "after_pr_create", "before_pr_receipt", "after_pr_receipt", "before_pr_open", "after_pr_open",
		"before_gate", "after_gate", "before_auto_merge", "after_auto_merge",
		"before_merge_arm_intent", "after_merge_arm_intent", "before_merge_arm_transition", "after_merge_arm_transition",
	}
}

type Reducer struct {
	providers Providers
	crash     Crash
}

func NewReducer(providers Providers, crash Crash) *Reducer {
	return &Reducer{providers: providers, crash: crash}
}

func (r *Reducer) Step(ctx context.Context, request Request) (Result, error) {
	request, err := exactRequest(request)
	if err != nil {
		return Result{}, err
	}
	if verifier, ok := r.providers.(SubjectVerifier); ok {
		if err := verifier.VerifySubject(ctx, request); err != nil {
			return Result{}, err
		}
	}
	if request.Target.DeliveryBeadID == "" && request.Target.Epoch != 1 {
		return Result{}, errors.New("later delivery epochs require an explicit selected delivery bead")
	}
	if request.Target.DeliveryBeadID != "" {
		probe := request
		probe.Target.Epoch = 1 // handoff identity deliberately excludes epoch/base.
		prepared := makePrepared(probe)
		selected, found, findErr := r.providers.FindDelivery(ctx, request.Target.DeliveryBeadID)
		if findErr != nil || !found || selected.Record.HandoffID != prepared.HandoffID {
			return Result{}, errors.New("selected delivery bead does not match the semantic handoff")
		}
		request.Target.Epoch = selected.Record.Epoch.Number
		request.Target.BaseRef = selected.Record.Epoch.BaseRef
		request.Target.BaseOID = selected.Record.Epoch.BaseOID
		if makePrepared(request).DeliveryBeadID != request.Target.DeliveryBeadID {
			return Result{}, errors.New("selected delivery bead is not the deterministic epoch leaf")
		}
	}
	// An implicit invocation must select (or prove the absence of) its Beads
	// leaf before observing a mutable remote base.  Otherwise an observation
	// can be attributed to a leaf selected only after that observation.
	if request.Target.DeliveryBeadID == "" {
		probe := request
		probe.Target.Epoch = 1
		prepared := makePrepared(probe)
		selected, found, findErr := r.providers.FindDelivery(ctx, prepared.DeliveryBeadID)
		if findErr != nil {
			return Result{}, findErr
		}
		if found {
			if selected.Record.HandoffID != prepared.HandoffID || !matchesDeliveryRecord(selected, prepared, request) {
				return Result{}, errors.New("implicit delivery bead does not match the semantic handoff")
			}
			request.Target.Epoch = selected.Record.Epoch.Number
			request.Target.BaseRef = selected.Record.Epoch.BaseRef
			request.Target.BaseOID = selected.Record.Epoch.BaseOID
		}
	}
	// The caller's base OID is evidence input, never lifecycle authority. The
	// provider observes the current base only after selection.
	baseOID, err := r.providers.ObserveBase(ctx, request.Target.BaseRef)
	if err != nil || !isHex(baseOID, 40) {
		return Result{}, errors.New("delivery could not observe an exact current base")
	}
	return r.stepDelivery(ctx, request, baseOID)
}

func (r *Reducer) stepDelivery(ctx context.Context, request Request, observedBase string) (Result, error) {
	prepared := makePrepared(request)
	bead, found, err := r.providers.FindDelivery(ctx, prepared.DeliveryBeadID)
	if err != nil {
		return Result{}, err
	}
	if !found {
		// Initial admission binds the provider observation. Existing delivery
		// beads retain their immutable epoch base and compare it separately.
		request.Target.BaseOID = observedBase
		prepared = makePrepared(request)
		if deadlineExpired(request.Target) {
			return Result{Status: "stalled", Reason: "deadline_expired_before_delivery"}, nil
		}
		if result, done, err := r.ensureInitialPrepared(request, prepared); err != nil || done {
			return result, err
		}
		if err := r.requireTerminal(ctx, request); err != nil {
			return Result{}, err
		}
		if err := r.cut("before_successor_create"); err != nil {
			return Result{}, err
		}
		created, err := r.providers.CreateDelivery(ctx, DeliveryBead{ID: prepared.DeliveryBeadID, ExternalRef: prepared.ExternalRef, Record: initialDeliveryRecord(prepared, request)})
		if err != nil {
			return Result{}, err
		}
		if !matchesDeliveryRecord(created, prepared, request) || created.Route != "" {
			return Result{}, errors.New("delivery create returned conflicting delivery.v1 identity")
		}
		if err := r.cut("after_successor_create"); err != nil {
			return Result{}, err
		}
		return Result{Status: "successor_created", Effect: "beads.create"}, nil
	}
	if request.Target.DeliveryBeadID == "" {
		request.Target.Epoch = bead.Record.Epoch.Number
		request.Target.BaseRef = bead.Record.Epoch.BaseRef
		request.Target.BaseOID = bead.Record.Epoch.BaseOID
		prepared = makePrepared(request)
	}
	if !matchesDeliveryRecord(bead, prepared, request) {
		return Result{}, errors.New("delivery bead does not carry the exact delivery.v1 identity")
	}
	if err := validDeliveryRecord(bead.Record); err != nil {
		return Result{}, err
	}
	if err := validateCurrentReceipt(request.Root, bead.Record); err != nil {
		return Result{}, err
	}
	if err := validateRecordAutoMergeAttempt(request.Root, bead.Record); err != nil {
		return Result{}, err
	}
	if err := r.validateSuccessorActivation(ctx, request.Root, bead); err != nil {
		return Result{}, err
	}
	if err := r.requireTerminal(ctx, request); err != nil {
		return Result{}, err
	}
	state := markerStore{root: request.Root, prefix: receiptNamespace(prepared)}
	if deadlineExpired(request.Target) && !isHostedState(bead.Record.State) {
		return r.recordDeadline(ctx, state, bead, request.Target)
	}
	if request.Target.DeliveryBeadID != "" && bead.Record.Epoch.BaseOID != observedBase && canRebaseEpoch(bead.Record.State) {
		return r.observeBaseMove(ctx, state, bead, observedBase)
	}
	switch bead.Record.State {
	case DeliveryStateQueued:
		if bead.Record.Predecessor != "" {
			if bead.Record.Publication != "published" {
				return Result{}, errors.New("selected successor is not published")
			}
			predecessor, found, err := r.providers.FindDelivery(ctx, bead.Record.Predecessor)
			if err != nil || !found || predecessor.Record.HandoffID != bead.Record.HandoffID || predecessor.Record.EpochSuccessorID != bead.ID || predecessor.Record.Epoch.Number+1 != bead.Record.Epoch.Number {
				return Result{}, errors.New("selected successor lacks its exact predecessor link")
			}
			if predecessor.Route != "" {
				if predecessor.Route != "agentops.delivery" {
					return Result{}, errors.New("successor predecessor has an unexpected route")
				}
				if err := r.providers.RetireRoute(ctx, predecessor.ID); err != nil {
					return Result{}, err
				}
				return Result{Status: "predecessor_route_retired", Effect: "beads.retire_route"}, nil
			}
			if bead.Route == "" {
				if err := r.providers.PublishRoute(ctx, bead.ID); err != nil {
					return Result{}, err
				}
				return Result{Status: "successor_route_published", Effect: "beads.publish"}, nil
			}
			if bead.Route != "agentops.delivery" {
				return Result{}, errors.New("selected successor has an unexpected route")
			}
			return r.storeDeliveryTransition(ctx, bead, DeliveryStatePreparing, bead.Record.Current)
		}
		if result, done, err := r.ensureInitialCommitted(request, prepared); err != nil || done {
			return result, err
		}
		if bead.Route != "" {
			return Result{}, errors.New("delivery bead has unexpected route")
		}
		handoffState := markerStore{root: request.Root, prefix: filepath.Join("handoffs", prepared.HandoffID)}
		committedDigest := handoffState.digest("committed.json")
		if !isHex(committedDigest, 64) {
			return Result{}, errors.New("delivery committed handoff is absent")
		}
		epochState := markerStore{root: request.Root, prefix: receiptNamespace(prepared)}
		activation := struct {
			SchemaVersion string `json:"schema_version"`
			Committed     string `json:"committed_handoff_digest"`
		}{SchemaVersion: "delivery-activation.v1", Committed: committedDigest}
		if !epochState.exists("activation.json") {
			return Result{}, errors.New("delivery activation receipt is absent")
		}
		if err := epochState.matches("activation.json", activation); err != nil {
			return Result{}, err
		}
		want := bead.Record
		want.Publication, want.Committed, want.State, want.Current, want.Revision = "published", committedDigest, DeliveryStatePreparing, receiptRef("activation", epochState), bead.Record.Revision+1
		return r.storeDeliveryRecord(ctx, bead, want)
	case DeliveryStatePreparing:
		if bead.Route == "" {
			if bead.Record.Publication != "published" || bead.Record.Committed == "" {
				return Result{}, errors.New("route publication lacks committed delivery.v1 envelope")
			}
			if err := r.cut("before_publication"); err != nil {
				return Result{}, err
			}
			if err := r.providers.PublishRoute(ctx, bead.ID); err != nil {
				return Result{}, err
			}
			if err := r.cut("after_publication"); err != nil {
				return Result{}, err
			}
			return Result{Status: "route_published", Effect: "beads.publish"}, nil
		}
		if bead.Route != "agentops.delivery" {
			return Result{}, errors.New("delivery bead has unexpected route")
		}
		return r.composeEpoch(ctx, state, bead, prepared, request)
	case DeliveryStateBranchReady:
		return r.preparePR(ctx, state, bead, prepared, request)
	case DeliveryStatePRPrepared:
		return r.openPreparedPR(ctx, state, bead, prepared, request, observedBase)
	case DeliveryStatePROpen:
		return r.enterCIWait(ctx, bead)
	case DeliveryStateCIWait, DeliveryStateMergeEligible, DeliveryStateMergeArmed, DeliveryStateManualReview:
		return r.advanceHosted(ctx, state, bead, prepared, request, observedBase)
	case DeliveryStateLanded, DeliveryStateFailed, DeliveryStateCancelled, DeliveryStateStalled:
		return Result{Status: string(bead.Record.State)}, nil
	case DeliveryStateRebaseNeeded:
		return r.createSuccessorFromIntent(ctx, state, bead, request, observedBase)
	case DeliveryStateSuccessorRequired, DeliveryStateRepairWait:
		return Result{Status: string(bead.Record.State)}, nil
	default:
		return Result{}, errors.New("delivery.v1 contains an illegal state")
	}
}

func canRebaseEpoch(state DeliveryState) bool {
	switch state {
	case DeliveryStatePreparing, DeliveryStateBranchReady, DeliveryStatePRPrepared, DeliveryStatePROpen, DeliveryStateCIWait, DeliveryStateMergeEligible:
		return true
	default:
		return false
	}
}

type SuccessorIntent struct {
	SchemaVersion            string     `json:"schema_version"`
	HandoffID                string     `json:"handoff_id"`
	PredecessorID            string     `json:"predecessor_id"`
	PredecessorReceiptDigest string     `json:"predecessor_receipt_digest"`
	PredecessorRevision      int        `json:"predecessor_revision"`
	Epoch                    int        `json:"epoch"`
	ChildID                  string     `json:"child_id"`
	ExternalRef              string     `json:"external_ref"`
	SemanticBead             string     `json:"semantic_bead_id"`
	TerminalRef              string     `json:"semantic_terminal_ref"`
	ReadyAt                  string     `json:"ready_at"`
	Deadline                 string     `json:"deadline"`
	Mode                     string     `json:"mode"`
	Rig                      string     `json:"rig_id"`
	Repository               string     `json:"repository"`
	Remote                   string     `json:"remote"`
	BaseRef                  string     `json:"base_ref"`
	BaseOID                  string     `json:"base_oid"`
	Branch                   string     `json:"branch"`
	LeaseOID                 string     `json:"lease_oid,omitempty"`
	Candidate                string     `json:"candidate_oid"`
	Certificate              string     `json:"admission_certificate_digest"`
	Committed                string     `json:"committed_handoff_digest"`
	Manifest                 string     `json:"subject_manifest_digest"`
	AutoMergeEffectID        string     `json:"auto_merge_effect_id,omitempty"`
	AutoMergeAttempt         ReceiptRef `json:"auto_merge_attempt,omitempty"`
}
type SuccessorCreatedReceipt struct {
	SchemaVersion string          `json:"schema_version"`
	Intent        SuccessorIntent `json:"intent"`
	ChildDigest   string          `json:"child_digest"`
	ChildID       string          `json:"child_id"`
}

func (r *Reducer) createSuccessorFromIntent(ctx context.Context, state markerStore, predecessor DeliveryBead, request Request, observedBase string) (Result, error) {
	var move BaseMoveReceipt
	if found, err := state.read("base-move.json", &move); err != nil || !found || move.CurrentBaseOID != observedBase || move.PreviousBaseOID != predecessor.Record.Epoch.BaseOID {
		return Result{}, errors.New("successor has no exact current base-move receipt")
	}
	childRequest := request
	childRequest.Target.Epoch = predecessor.Record.Epoch.Number + 1
	childRequest.Target.BaseOID = move.CurrentBaseOID
	childPrepared := makePrepared(childRequest)
	effectID, attempt, err := successorAutoMergeFence(state, predecessor)
	if err != nil {
		return Result{}, err
	}
	intent := SuccessorIntent{SchemaVersion: "successor-intent.v1", HandoffID: predecessor.Record.HandoffID, PredecessorID: predecessor.ID, PredecessorReceiptDigest: predecessor.Record.Current.Digest, PredecessorRevision: predecessor.Record.Revision, Epoch: childRequest.Target.Epoch, ChildID: childPrepared.DeliveryBeadID, ExternalRef: childPrepared.ExternalRef, SemanticBead: predecessor.Record.SemanticBead, TerminalRef: predecessor.Record.TerminalRef, ReadyAt: predecessor.Record.ReadyAt, Deadline: predecessor.Record.Deadline, Mode: predecessor.Record.Mode, Rig: predecessor.Record.Rig, Repository: predecessor.Record.Repository, Remote: predecessor.Record.Remote, BaseRef: predecessor.Record.Epoch.BaseRef, BaseOID: move.CurrentBaseOID, Branch: predecessor.Record.Epoch.Branch, LeaseOID: predecessor.Record.Epoch.Head, Candidate: predecessor.Record.Candidate, Certificate: predecessor.Record.Certificate, Committed: predecessor.Record.Committed, Manifest: predecessor.Record.Manifest, AutoMergeEffectID: effectID, AutoMergeAttempt: attempt}
	const name = "successor-intent.json"
	if !state.exists(name) {
		if predecessor.Record.EpochSuccessorID == "" {
			want := predecessor.Record
			want.EpochSuccessorID, want.Revision = intent.ChildID, predecessor.Record.Revision+1
			return r.storeDeliveryRecord(ctx, predecessor, want)
		}
		if predecessor.Record.EpochSuccessorID != intent.ChildID {
			return Result{}, errors.New("successor link conflicts with deterministic child")
		}
		if err := state.writeImmutable(name, intent); err != nil {
			return Result{}, err
		}
		return Result{Status: "successor_intent"}, nil
	}
	var storedIntent SuccessorIntent
	if found, err := state.read(name, &storedIntent); err != nil || !found {
		return Result{}, errors.New("successor intent disappeared")
	}
	if storedIntent != intent || (predecessor.Record.EpochSuccessorID != "" && predecessor.Record.EpochSuccessorID != intent.ChildID) {
		return Result{}, errors.New("successor intent conflicts with selected predecessor")
	}
	child, found, err := r.providers.FindDelivery(ctx, intent.ChildID)
	if err != nil {
		return Result{}, err
	}
	if found {
		if !matchesSuccessorChild(child, childPrepared, childRequest, intent) {
			return Result{}, errors.New("successor child conflicts with intent")
		}
		return r.advanceCreatedSuccessor(ctx, state, predecessor, child, intent)
	}
	record := initialDeliveryRecord(childPrepared, childRequest)
	record.Predecessor, record.PredecessorReceiptDigest, record.Committed = intent.PredecessorID, intent.PredecessorReceiptDigest, intent.Committed
	record.Epoch.Branch, record.Epoch.LeaseOID, record.PR = predecessor.Record.Epoch.Branch, predecessor.Record.Epoch.Head, predecessor.Record.PR
	record.AutoMergeEffectID, record.AutoMergeAttempt = intent.AutoMergeEffectID, intent.AutoMergeAttempt
	if err := r.cut("before_successor_create"); err != nil {
		return Result{}, err
	}
	created, err := r.providers.CreateDelivery(ctx, DeliveryBead{ID: intent.ChildID, ExternalRef: intent.ExternalRef, Record: record})
	if err != nil {
		return Result{}, err
	}
	if !matchesSuccessorChild(created, childPrepared, childRequest, intent) {
		return Result{}, errors.New("successor create did not return intent identity")
	}
	if err := r.cut("after_successor_create"); err != nil {
		return Result{}, err
	}
	return Result{Status: "successor_created", Effect: "beads.create"}, nil
}

func successorAutoMergeFence(state markerStore, predecessor DeliveryBead) (string, ReceiptRef, error) {
	effectID, attempt := predecessor.Record.AutoMergeEffectID, predecessor.Record.AutoMergeAttempt
	if !state.exists("auto-merge-attempt.json") {
		return effectID, attempt, nil
	}
	var receipt AutoMergeAttemptReceipt
	if found, err := state.read("auto-merge-attempt.json", &receipt); err != nil || !found || receipt.SchemaVersion != "auto-merge-attempt-receipt.v1" || receipt.HandoffID != predecessor.Record.HandoffID || receipt.Epoch != predecessor.Record.Epoch.Number || receipt.EffectID == "" || receipt.EffectID != receipt.Arm.EffectID || receipt.Outcome != "prepared" {
		return "", ReceiptRef{}, errors.New("successor has a conflicting auto-merge attempt fence")
	}
	ref := receiptRef("auto-merge-attempt", state)
	if effectID != "" && effectID != receipt.EffectID || attempt != (ReceiptRef{}) && attempt != ref {
		return "", ReceiptRef{}, errors.New("successor auto-merge attempt disagrees with Beads")
	}
	return receipt.EffectID, ref, nil
}

func (r *Reducer) advanceCreatedSuccessor(ctx context.Context, state markerStore, predecessor, child DeliveryBead, intent SuccessorIntent) (Result, error) {
	creation := successorCreationRecord(child.Record)
	digest, err := valueDigest(creation)
	if err != nil {
		return Result{}, err
	}
	receipt := SuccessorCreatedReceipt{SchemaVersion: "successor-created-receipt.v1", Intent: intent, ChildDigest: digest, ChildID: child.ID}
	const name = "successor-created.json"
	if !state.exists(name) {
		if err := state.writeImmutable(name, receipt); err != nil {
			return Result{}, err
		}
		return Result{Status: "successor_created_receipted"}, nil
	}
	var storedReceipt SuccessorCreatedReceipt
	if found, err := state.read(name, &storedReceipt); err != nil || !found || storedReceipt.ChildID != child.ID || storedReceipt.Intent != intent || storedReceipt.ChildDigest != digest {
		return Result{}, errors.New("successor created receipt conflicts")
	}
	receipt = storedReceipt
	ref := receiptRef("successor-created", state)
	if predecessor.Record.EpochSuccessorID != child.ID {
		return Result{}, errors.New("predecessor successor link conflicts")
	}
	if child.Record.Publication == "pending" {
		childState := markerStore{root: state.root, prefix: receiptNamespaceFor(child.Record)}
		activation := struct {
			SchemaVersion string `json:"schema_version"`
			ChildDigest   string `json:"child_digest"`
			ReceiptDigest string `json:"successor_receipt_digest"`
		}{SchemaVersion: "successor-activation.v1", ChildDigest: digest, ReceiptDigest: ref.Digest}
		if !childState.exists("activation.json") {
			if err := childState.writeImmutable("activation.json", activation); err != nil {
				return Result{}, err
			}
			return Result{Status: "successor_activation_receipted"}, nil
		}
		if err := childState.matches("activation.json", activation); err != nil {
			return Result{}, err
		}
		want := child.Record
		want.Publication, want.Current, want.Revision = "published", receiptRef("activation", childState), child.Record.Revision+1
		return r.storeDeliveryRecord(ctx, child, want)
	}
	if err := validateCurrentReceipt(state.root, child.Record); err != nil {
		return Result{}, err
	}
	if err := validateSuccessorActivationWithPredecessor(state.root, child, predecessor); err != nil {
		return Result{}, err
	}
	// Retire first: publishing first exposes two selected leaves between
	// invocations.  The sole external mutation remains one route transition.
	if predecessor.Route != "" {
		if err := r.providers.RetireRoute(ctx, predecessor.ID); err != nil {
			return Result{}, err
		}
		return Result{Status: "predecessor_route_retired", Effect: "beads.retire_route"}, nil
	}
	if child.Route == "" {
		if err := r.cut("before_successor_publication"); err != nil {
			return Result{}, err
		}
		if err := r.providers.PublishRoute(ctx, child.ID); err != nil {
			return Result{}, err
		}
		if err := r.cut("after_successor_publication"); err != nil {
			return Result{}, err
		}
		return Result{Status: "successor_route_published", Effect: "beads.publish"}, nil
	}
	return Result{Status: "successor_ready"}, nil
}

func successorCreationRecord(record DeliveryRecord) DeliveryRecord {
	return DeliveryRecord{
		SchemaVersion: record.SchemaVersion, Revision: 1, HandoffID: record.HandoffID,
		Epoch: DeliveryEpoch{Number: record.Epoch.Number, BaseRef: record.Epoch.BaseRef, BaseOID: record.Epoch.BaseOID, Branch: record.Epoch.Branch, LeaseOID: record.Epoch.LeaseOID},
		PR:    record.PR, State: DeliveryStateQueued, Publication: "pending", ReadyAt: record.ReadyAt, Deadline: record.Deadline,
		SemanticBead: record.SemanticBead, TerminalRef: record.TerminalRef, Certificate: record.Certificate, Committed: record.Committed,
		Mode: record.Mode, Rig: record.Rig, Repository: record.Repository, Remote: record.Remote, Candidate: record.Candidate, Manifest: record.Manifest,
		AutoMergeEffectID: record.AutoMergeEffectID, AutoMergeAttempt: record.AutoMergeAttempt,
		PredecessorReceiptDigest: record.PredecessorReceiptDigest, Predecessor: record.Predecessor,
	}
}

type BaseMoveReceipt struct {
	SchemaVersion, HandoffID, PreviousBaseOID, CurrentBaseOID string
	Epoch                                                     int
}

type DeliveryOutcomeReceipt struct {
	SchemaVersion string        `json:"schema_version"`
	HandoffID     string        `json:"handoff_id"`
	Epoch         int           `json:"epoch"`
	State         DeliveryState `json:"state"`
	Reason        string        `json:"reason"`
}

// A selected leaf never has its epoch identity rewritten. A clean observed
// movement is receipted first and only the following invocation changes state.
func (r *Reducer) observeBaseMove(ctx context.Context, state markerStore, bead DeliveryBead, current string) (Result, error) {
	receipt := BaseMoveReceipt{SchemaVersion: "base-move-receipt.v1", HandoffID: bead.Record.HandoffID, Epoch: bead.Record.Epoch.Number, PreviousBaseOID: bead.Record.Epoch.BaseOID, CurrentBaseOID: current}
	const name = "base-move.json"
	if !state.exists(name) {
		descends, err := r.providers.BaseDescends(ctx, current, bead.Record.Epoch.BaseOID)
		if err != nil || !descends {
			return r.recordDeliveryOutcome(ctx, state, bead, DeliveryStateFailed, "non_descendant_base_move")
		}
		if err := state.writeImmutable(name, receipt); err != nil {
			return Result{}, err
		}
		return Result{Status: "base_move_observed"}, nil
	}
	if err := state.matches(name, receipt); err != nil {
		return Result{}, err
	}
	return r.storeDeliveryTransition(ctx, bead, DeliveryStateRebaseNeeded, receiptRef("base-move", state))
}

func (r *Reducer) recordDeliveryOutcome(ctx context.Context, state markerStore, bead DeliveryBead, next DeliveryState, reason string) (Result, error) {
	receipt := DeliveryOutcomeReceipt{SchemaVersion: "delivery-outcome-receipt.v1", HandoffID: bead.Record.HandoffID, Epoch: bead.Record.Epoch.Number, State: next, Reason: reason}
	const name = "delivery-outcome.json"
	if !state.exists(name) {
		if err := state.writeImmutable(name, receipt); err != nil {
			return Result{}, err
		}
		return Result{Status: "delivery_outcome_receipted", Reason: reason}, nil
	}
	if err := state.matches(name, receipt); err != nil {
		return Result{}, err
	}
	want := bead.Record
	want.State, want.Current, want.DeliveryOutcome, want.Revision = next, receiptRef("delivery-outcome", state), reason, bead.Record.Revision+1
	result, err := r.storeDeliveryRecord(ctx, bead, want)
	result.Reason = reason
	return result, err
}

func initialDeliveryRecord(prepared Prepared, request Request) DeliveryRecord {
	record := DeliveryRecord{SchemaVersion: "gc.delivery.v1", Revision: 1, HandoffID: prepared.HandoffID, State: DeliveryStateQueued, Publication: "pending", ReadyAt: request.Target.PreparedAt, Deadline: request.Target.Deadline, SemanticBead: request.Target.SemanticBeadID, TerminalRef: request.Target.SemanticTerminalRef, Certificate: request.CertificateDigest, Mode: request.Target.Mode, Rig: request.Target.RigID, Repository: request.Target.Repository, Remote: request.Target.Remote, Candidate: request.Certificate.Candidate.Commit, Manifest: request.Certificate.ChangedPathManifest, Epoch: DeliveryEpoch{Number: request.Target.Epoch, BaseRef: request.Target.BaseRef, BaseOID: request.Target.BaseOID, Branch: "gc/delivery/" + prepared.HandoffID[:20]}}
	if request.Target.Epoch > 1 {
		record.Predecessor = "delivery-" + prepared.HandoffID[:20] + fmt.Sprintf("-e%06d", request.Target.Epoch-1)
	}
	return record
}

func matchesDeliveryRecord(bead DeliveryBead, prepared Prepared, request Request) bool {
	record := bead.Record
	return bead.ID == prepared.DeliveryBeadID && bead.ExternalRef == prepared.ExternalRef && record.SchemaVersion == "gc.delivery.v1" && record.Revision > 0 && record.HandoffID == prepared.HandoffID && record.SemanticBead == request.Target.SemanticBeadID && record.TerminalRef == request.Target.SemanticTerminalRef && record.Certificate == request.CertificateDigest && record.Mode == request.Target.Mode && record.Rig == request.Target.RigID && record.Repository == request.Target.Repository && record.Remote == request.Target.Remote && record.Candidate == request.Certificate.Candidate.Commit && record.Manifest == request.Certificate.ChangedPathManifest && record.ReadyAt == request.Target.PreparedAt && record.Deadline == request.Target.Deadline && record.Epoch.Number == request.Target.Epoch && record.Epoch.BaseRef == request.Target.BaseRef && record.Epoch.BaseOID == request.Target.BaseOID && record.Epoch.Branch == "gc/delivery/"+prepared.HandoffID[:20]
}

func matchesSuccessorChild(child DeliveryBead, prepared Prepared, request Request, intent SuccessorIntent) bool {
	if child.ID != intent.ChildID || child.ExternalRef != intent.ExternalRef || !matchesDeliveryRecord(child, prepared, request) {
		return false
	}
	record := child.Record
	return record.Predecessor == intent.PredecessorID && record.PredecessorReceiptDigest == intent.PredecessorReceiptDigest && record.SemanticBead == intent.SemanticBead && record.TerminalRef == intent.TerminalRef && record.ReadyAt == intent.ReadyAt && record.Deadline == intent.Deadline && record.Mode == intent.Mode && record.Rig == intent.Rig && record.Repository == intent.Repository && record.Remote == intent.Remote && record.Candidate == intent.Candidate && record.Committed == intent.Committed && record.Certificate == intent.Certificate && record.Manifest == intent.Manifest && record.AutoMergeEffectID == intent.AutoMergeEffectID && record.AutoMergeAttempt == intent.AutoMergeAttempt && record.Epoch.Number == intent.Epoch && record.Epoch.BaseRef == intent.BaseRef && record.Epoch.BaseOID == intent.BaseOID && record.Epoch.Branch == intent.Branch && record.Epoch.LeaseOID == intent.LeaseOID
}

func receiptNamespace(prepared Prepared) string {
	return filepath.Join("handoffs", prepared.HandoffID, "epochs", fmt.Sprintf("%06d", prepared.Epoch))
}

func receiptNamespaceFor(record DeliveryRecord) string {
	return filepath.Join("handoffs", record.HandoffID, "epochs", fmt.Sprintf("%06d", record.Epoch.Number))
}

// Before atomic Beads creation, these exact bytes establish only admission.
// Once the delivery.v1 bead exists, it is the sole lifecycle selector.
func (r *Reducer) ensureInitialPrepared(request Request, prepared Prepared) (Result, bool, error) {
	state := markerStore{root: request.Root, prefix: filepath.Join("handoffs", prepared.HandoffID)}
	if state.exists("certificate.json") {
		if err := state.matchesBytes("certificate.json", request.CertificateBytes); err != nil {
			return Result{}, true, err
		}
		if !state.exists("prepared.json") {
			if err := state.writeImmutable("prepared.json", prepared); err != nil {
				return Result{}, true, err
			}
			return Result{Status: "prepared"}, true, nil
		}
		if err := state.matches("prepared.json", prepared); err != nil {
			return Result{}, true, err
		}
		return Result{}, false, nil
	}
	if err := r.cut("before_initial_certificate"); err != nil {
		return Result{}, true, err
	}
	if err := state.writeBytesImmutable("certificate.json", request.CertificateBytes); err != nil {
		return Result{}, true, err
	}
	if err := r.cut("after_initial_certificate"); err != nil {
		return Result{}, true, err
	}
	if err := r.cut("before_initial_prepared"); err != nil {
		return Result{}, true, err
	}
	if err := state.writeImmutable("prepared.json", prepared); err != nil {
		return Result{}, true, err
	}
	if err := r.cut("after_initial_prepared"); err != nil {
		return Result{}, true, err
	}
	return Result{Status: "prepared"}, true, nil
}

func (r *Reducer) ensureInitialCommitted(request Request, prepared Prepared) (Result, bool, error) {
	state := markerStore{root: request.Root, prefix: filepath.Join("handoffs", prepared.HandoffID)}
	payload := makeDelivery(prepared, "published")
	if !state.exists("payload.json") {
		if err := r.cut("before_initial_payload"); err != nil {
			return Result{}, true, err
		}
		if err := state.writeImmutable("payload.json", payload); err != nil {
			return Result{}, true, err
		}
		if err := r.cut("after_initial_payload"); err != nil {
			return Result{}, true, err
		}
		return Result{Status: "publication_prepared"}, true, nil
	}
	if err := state.matches("payload.json", payload); err != nil {
		return Result{}, true, err
	}
	committed := makeCommitted(prepared, state.digest("prepared.json"), state.digest("payload.json"), request.Target.CommittedAt)
	if !state.exists("committed.json") {
		if err := r.cut("before_initial_committed"); err != nil {
			return Result{}, true, err
		}
		if err := state.writeImmutable("committed.json", committed); err != nil {
			return Result{}, true, err
		}
		epochState := markerStore{root: request.Root, prefix: receiptNamespace(prepared)}
		activation := struct {
			SchemaVersion string `json:"schema_version"`
			Committed     string `json:"committed_handoff_digest"`
		}{SchemaVersion: "delivery-activation.v1", Committed: state.digest("committed.json")}
		if err := epochState.writeImmutable("activation.json", activation); err != nil {
			return Result{}, true, err
		}
		if err := r.cut("after_initial_committed"); err != nil {
			return Result{}, true, err
		}
		return Result{Status: "committed"}, true, nil
	}
	if err := state.matches("committed.json", committed); err != nil {
		return Result{}, false, err
	}
	epochState := markerStore{root: request.Root, prefix: receiptNamespace(prepared)}
	activation := struct {
		SchemaVersion string `json:"schema_version"`
		Committed     string `json:"committed_handoff_digest"`
	}{SchemaVersion: "delivery-activation.v1", Committed: state.digest("committed.json")}
	if !epochState.exists("activation.json") {
		if err := epochState.writeImmutable("activation.json", activation); err != nil {
			return Result{}, true, err
		}
		return Result{Status: "activation_receipted"}, true, nil
	}
	return Result{}, false, epochState.matches("activation.json", activation)
}

func receiptRef(phase string, state markerStore) ReceiptRef {
	path := filepath.Join(state.prefix, phase+".json")
	return ReceiptRef{Path: path, Digest: state.digest(phase + ".json")}
}

func (r *Reducer) storeDeliveryTransition(ctx context.Context, bead DeliveryBead, next DeliveryState, receipt ReceiptRef) (Result, error) {
	want := bead.Record
	want.State, want.Current, want.Revision = next, receipt, bead.Record.Revision+1
	return r.storeDeliveryRecord(ctx, bead, want)
}

func (r *Reducer) storeDeliveryRecord(ctx context.Context, bead DeliveryBead, want DeliveryRecord) (Result, error) {
	if err := validDeliveryRecord(want); err != nil {
		return Result{}, err
	}
	stored, err := r.providers.StoreTransition(ctx, bead, want)
	if err != nil {
		return Result{}, err
	}
	if stored.Record != want {
		return Result{}, errors.New("delivery transition was not reread exactly")
	}
	return Result{Status: string(want.State), Effect: "beads.transition"}, nil
}

func validDeliveryRecord(record DeliveryRecord) error {
	states := map[DeliveryState]bool{DeliveryStateQueued: true, DeliveryStatePreparing: true, DeliveryStateBranchReady: true, DeliveryStatePRPrepared: true, DeliveryStatePROpen: true, DeliveryStateCIWait: true, DeliveryStateRebaseNeeded: true, DeliveryStateMergeEligible: true, DeliveryStateMergeArmed: true, DeliveryStateLanded: true, DeliveryStateRepairWait: true, DeliveryStateManualReview: true, DeliveryStateStalled: true, DeliveryStateFailed: true, DeliveryStateSuccessorRequired: true, DeliveryStateCancelled: true}
	if record.SchemaVersion != "gc.delivery.v1" || record.Revision < 1 || !isHex(record.HandoffID, 64) || !states[record.State] || (record.Publication != "pending" && record.Publication != "published") || record.SemanticBead == "" || record.TerminalRef == "" || (record.Mode != "auto" && record.Mode != "manual") || !isHex(record.Certificate, 64) || !isHex(record.Candidate, 40) || !isHex(record.Manifest, 64) || record.Epoch.Number < 1 || !isHex(record.Epoch.BaseOID, 40) || record.Epoch.BaseRef == "" || record.Epoch.Branch != "gc/delivery/"+record.HandoffID[:20] || !isTime(record.ReadyAt) || !isTime(record.Deadline) {
		return errors.New("gc.delivery.v1 has incomplete typed identity")
	}
	if record.Publication == "published" && !isHex(record.Committed, 64) {
		return errors.New("published gc.delivery.v1 lacks committed handoff digest")
	}
	if record.Publication == "pending" && (record.State != DeliveryStateQueued || record.Current != (ReceiptRef{})) {
		return errors.New("pending gc.delivery.v1 is not an untouched queued creation envelope")
	}
	if record.Publication == "pending" && ((record.Epoch.Number == 1 && record.Committed != "") || (record.Epoch.Number > 1 && !isHex(record.Committed, 64))) {
		return errors.New("pending gc.delivery.v1 has the wrong committed-handoff binding")
	}
	if record.State != DeliveryStateQueued && record.Publication != "published" {
		return errors.New("active gc.delivery.v1 is not published")
	}
	if record.Publication == "published" || record.State != DeliveryStateQueued || record.Current != (ReceiptRef{}) {
		if !validCurrentReceipt(record) {
			return errors.New("gc.delivery.v1 lacks a state-confined current receipt")
		}
	}
	if record.Epoch.Head != "" && !isHex(record.Epoch.Head, 40) || record.Epoch.Tree != "" && !isHex(record.Epoch.Tree, 40) || record.Epoch.LeaseOID != "" && !isHex(record.Epoch.LeaseOID, 40) {
		return errors.New("gc.delivery.v1 has malformed epoch head, tree, or lease")
	}
	if !validStateFields(record) {
		return errors.New("gc.delivery.v1 violates state-dependent identity")
	}
	bytes, err := verdictcheck.CanonicalJSON(record)
	if err != nil || len(bytes) > 4096 {
		return errors.New("gc.delivery.v1 exceeds strict canonical envelope bound")
	}
	return nil
}

func validCurrentReceipt(record DeliveryRecord) bool {
	epoch, name, ok := deliveryReceiptLocation(record, record.Current)
	if !ok || epoch != record.Epoch.Number {
		return false
	}
	allowed := map[DeliveryState]map[string]bool{
		DeliveryStateQueued: {"activation.json": true}, DeliveryStatePreparing: {"activation.json": true},
		DeliveryStateBranchReady: {"branch.json": true}, DeliveryStatePRPrepared: {"pr-intent.json": true},
		DeliveryStatePROpen: {"pr-open.json": true}, DeliveryStateCIWait: {"pr-open.json": true, "hosted-gate.json": true},
		DeliveryStateRebaseNeeded: {"base-move.json": true, "successor-created.json": true}, DeliveryStateMergeEligible: {"hosted-gate.json": true},
		DeliveryStateMergeArmed: {"merge-arm.json": true}, DeliveryStateLanded: {"landing.json": true},
		DeliveryStateRepairWait: {"delivery-outcome.json": true, "auto-merge-attempt.json": true, "merge-refusal.json": true}, DeliveryStateManualReview: {"hosted-gate.json": true},
		DeliveryStateStalled: {"deadline.json": true, "delivery-outcome.json": true, "auto-merge-attempt.json": true, "merge-refusal.json": true},
		DeliveryStateFailed:  {"delivery-outcome.json": true}, DeliveryStateSuccessorRequired: {"delivery-outcome.json": true},
		DeliveryStateCancelled: {"delivery-outcome.json": true},
	}
	return allowed[record.State][name]
}

// validateCurrentReceipt binds the persisted lifecycle selector to its exact
// immutable evidence before a terminal return or another provider effect.  The
// record remains the selector; this finite switch only verifies the receipt
// admitted by that selector's state and name.
func validateCurrentReceipt(root string, record DeliveryRecord) error {
	if record.Current == (ReceiptRef{}) {
		return nil
	}
	epoch, name, ok := deliveryReceiptLocation(record, record.Current)
	if !ok || epoch != record.Epoch.Number || !validCurrentReceipt(record) {
		return errors.New("delivery.v1 current receipt has an illegal location")
	}
	bytes, err := os.ReadFile(filepath.Join(root, record.Current.Path))
	if err != nil {
		return fmt.Errorf("delivery.v1 current receipt is absent: %w", err)
	}
	sum := sha256.Sum256(bytes)
	if hex.EncodeToString(sum[:]) != record.Current.Digest {
		return errors.New("delivery.v1 current receipt digest conflicts")
	}
	decode := func(into any) error {
		if err := decodeStrict(bytes, into); err != nil {
			return fmt.Errorf("delivery.v1 current receipt is not strict JSON: %w", err)
		}
		return nil
	}
	matchBranch := func(receipt BranchReceipt) bool {
		digest, err := valueDigest(branchPublicIdentity(Branch{Name: record.Epoch.Branch, BaseRef: record.Epoch.BaseRef, BaseOID: record.Epoch.BaseOID, Head: record.Epoch.Head, LeaseOID: record.Epoch.LeaseOID}))
		return err == nil && receipt.SchemaVersion == "branch-receipt.v1" && (receipt.Outcome == "created" || receipt.Outcome == "adopted" || receipt.Outcome == "observed") && receipt.HandoffID == record.HandoffID && receipt.Epoch == record.Epoch.Number && receipt.RigID == record.Rig && receipt.Repository == record.Repository && receipt.Remote == record.Remote && receipt.Branch == record.Epoch.Branch && receipt.BaseRef == record.Epoch.BaseRef && receipt.BaseOID == record.Epoch.BaseOID && receipt.ExpectedHead == record.Epoch.Head && receipt.LeaseOID == record.Epoch.LeaseOID && receipt.ResponseDigest == digest
	}
	switch name {
	case "activation.json":
		if record.Predecessor == "" {
			var receipt struct {
				SchemaVersion string `json:"schema_version"`
				Committed     string `json:"committed_handoff_digest"`
			}
			if err := decode(&receipt); err != nil || receipt.SchemaVersion != "delivery-activation.v1" || receipt.Committed != record.Committed {
				return errors.New("delivery activation does not bind the published record")
			}
			return nil
		}
		var receipt struct {
			SchemaVersion string `json:"schema_version"`
			ChildDigest   string `json:"child_digest"`
			ReceiptDigest string `json:"successor_receipt_digest"`
		}
		if err := decode(&receipt); err != nil || receipt.SchemaVersion != "successor-activation.v1" {
			return errors.New("successor activation is invalid")
		}
		childDigest, err := valueDigest(successorCreationRecord(record))
		if err != nil || receipt.ChildDigest != childDigest || !isHex(receipt.ReceiptDigest, 64) {
			return errors.New("successor activation does not bind child creation")
		}
		createdPath := filepath.Join(receiptNamespaceFor(DeliveryRecord{HandoffID: record.HandoffID, Epoch: DeliveryEpoch{Number: record.Epoch.Number - 1}}), "successor-created.json")
		createdBytes, err := os.ReadFile(filepath.Join(root, createdPath))
		if err != nil {
			return errors.New("successor activation lacks predecessor creation receipt")
		}
		createdSum := sha256.Sum256(createdBytes)
		var created SuccessorCreatedReceipt
		if receipt.ReceiptDigest != hex.EncodeToString(createdSum[:]) || decodeStrict(createdBytes, &created) != nil || created.SchemaVersion != "successor-created-receipt.v1" || created.ChildID != "delivery-"+record.HandoffID[:20]+fmt.Sprintf("-e%06d", record.Epoch.Number) || created.ChildDigest != childDigest || created.Intent.HandoffID != record.HandoffID || created.Intent.PredecessorID != record.Predecessor || created.Intent.ChildID != created.ChildID || created.Intent.Epoch != record.Epoch.Number {
			return errors.New("successor activation conflicts with predecessor creation")
		}
		return nil
	case "branch.json":
		var receipt BranchReceipt
		if err := decode(&receipt); err != nil || !matchBranch(receipt) {
			return errors.New("branch receipt does not bind the delivery record")
		}
		return nil
	case "pr-intent.json":
		var receipt PRIntent
		if err := decode(&receipt); err != nil || receipt != expectedPRIntent(Prepared{HandoffID: record.HandoffID}, record.Repository, record.Epoch, record.PR) {
			return errors.New("PR intent does not bind the delivery record")
		}
		return nil
	case "pr-open.json":
		var receipt PROpenReceipt
		if err := decode(&receipt); err != nil {
			return err
		}
		observation := PRObservation{State: receipt.State, Draft: receipt.Draft, BaseOID: receipt.ObservedBaseOID, Head: receipt.ObservedHead, PR: PullRequest{ID: receipt.PRID, EffectID: receipt.EffectID, Repository: receipt.Repository, Branch: receipt.Branch, BaseRef: receipt.BaseRef, NodeID: receipt.NodeID, Number: receipt.Number, URL: receipt.URL}}
		intent := expectedPRIntent(Prepared{HandoffID: record.HandoffID}, record.Repository, record.Epoch, record.PR)
		firstEpochIntent := expectedPRIntent(Prepared{HandoffID: record.HandoffID}, record.Repository, record.Epoch, PullRequest{})
		intentDigest, digestErr := valueDigest(intent)
		firstEpochDigest, firstEpochErr := valueDigest(firstEpochIntent)
		responseDigest, responseErr := valueDigest(observation)
		intentMatches := receipt.IntentDigest == intentDigest && matchesPRObservation(observation, intent)
		if record.Epoch.Number == 1 {
			intentMatches = intentMatches || (receipt.IntentDigest == firstEpochDigest && matchesPRObservation(observation, firstEpochIntent))
		}
		if digestErr != nil || firstEpochErr != nil || responseErr != nil || receipt.SchemaVersion != "pr-open-receipt.v1" || !intentMatches || receipt.HandoffID != record.HandoffID || receipt.Epoch != record.Epoch.Number || receipt.RigID != record.Rig || receipt.Remote != record.Remote || receipt.Repository != record.Repository || receipt.PRID != record.PR.ID || receipt.Branch != record.PR.Branch || receipt.BaseRef != record.PR.BaseRef || receipt.BaseOID != record.Epoch.BaseOID || receipt.ExpectedHead != record.Epoch.Head || receipt.ObservedBaseOID != record.Epoch.BaseOID || receipt.ObservedHead != record.Epoch.Head || receipt.EffectID != record.PR.EffectID || receipt.NodeID != record.PR.NodeID || receipt.Number != record.PR.Number || receipt.URL != record.PR.URL || receipt.ResponseDigest != responseDigest {
			return errors.New("PR receipt does not bind the delivery record")
		}
		return nil
	case "hosted-gate.json":
		var receipt GateReceipt
		if err := decode(&receipt); err != nil {
			return err
		}
		digest, digestErr := hostedGateDigest(receipt.Gate)
		qualified, _, qualifyErr := qualifyHostedGate(receipt.Gate, DeliveryBead{Record: record})
		if digestErr != nil || qualifyErr != nil || !qualified || receipt.SchemaVersion != "hosted-gate-receipt.v1" || receipt.HandoffID != record.HandoffID || receipt.Epoch != record.Epoch.Number || receipt.PRID != record.PR.ID || receipt.Head != record.Epoch.Head || receipt.BaseRef != record.Epoch.BaseRef || receipt.BaseOID != record.Epoch.BaseOID || receipt.GateDigest != digest || record.GateDigest != digest || (receipt.Gate.AutoMergeEnabled && !hasOwnedAutoMergeAttempt(root, record)) {
			return errors.New("hosted gate receipt does not bind the delivery record")
		}
		return nil
	case "base-move.json":
		var receipt BaseMoveReceipt
		if err := decode(&receipt); err != nil || receipt.SchemaVersion != "base-move-receipt.v1" || receipt.HandoffID != record.HandoffID || receipt.Epoch != record.Epoch.Number || receipt.PreviousBaseOID != record.Epoch.BaseOID || !isHex(receipt.CurrentBaseOID, 40) || receipt.CurrentBaseOID == receipt.PreviousBaseOID {
			return errors.New("base-move receipt does not bind the delivery record")
		}
		return nil
	case "successor-created.json":
		var receipt SuccessorCreatedReceipt
		if err := decode(&receipt); err != nil || receipt.SchemaVersion != "successor-created-receipt.v1" || receipt.Intent.HandoffID != record.HandoffID || receipt.Intent.PredecessorID != "delivery-"+record.HandoffID[:20]+fmt.Sprintf("-e%06d", record.Epoch.Number) || receipt.Intent.Epoch != record.Epoch.Number+1 || receipt.ChildID != record.EpochSuccessorID || receipt.Intent.ChildID != record.EpochSuccessorID || !isHex(receipt.ChildDigest, 64) {
			return errors.New("successor creation receipt does not bind the delivery record")
		}
		return nil
	case "merge-arm.json":
		var receipt MergeArmReceipt
		if err := decode(&receipt); err != nil || receipt.SchemaVersion != "merge-arm-receipt.v1" || receipt.HandoffID != record.HandoffID || receipt.Epoch != record.Epoch.Number || receipt.Arm != expectedMergeArm(DeliveryBead{Record: record}, receipt.Arm.ProtectionDigest) || receipt.Arm.ID != record.ArmID || receipt.Arm.EffectID != record.AutoMergeEffectID {
			return errors.New("merge-arm receipt does not bind the delivery record")
		}
		return nil
	case "landing.json":
		var receipt LandingReceipt
		if err := decode(&receipt); err != nil {
			return err
		}
		landing := Landing{PRID: receipt.PRID, Head: receipt.Head, SHA: receipt.LandedSHA, Tree: receipt.Tree, Parents: receipt.Parents}
		digest, digestErr := valueDigest(landing)
		if digestErr != nil || receipt.SchemaVersion != "landing-receipt.v1" || receipt.HandoffID != record.HandoffID || receipt.Epoch != record.Epoch.Number || receipt.PRID != record.PR.ID || receipt.Head != record.Epoch.Head || receipt.Tree != record.Epoch.Tree || len(receipt.Parents) != 1 || receipt.Parents[0] != record.Epoch.BaseOID || !isHex(receipt.LandedSHA, 40) || record.LandingDigest != digest || record.LandedSHA != receipt.LandedSHA {
			return errors.New("landing receipt does not bind the canonical record")
		}
		return nil
	case "delivery-outcome.json":
		var receipt DeliveryOutcomeReceipt
		if err := decode(&receipt); err != nil || receipt.SchemaVersion != "delivery-outcome-receipt.v1" || receipt.HandoffID != record.HandoffID || receipt.Epoch != record.Epoch.Number || receipt.State != record.State || receipt.Reason == "" || receipt.Reason != record.DeliveryOutcome {
			return errors.New("delivery outcome receipt does not bind the delivery record")
		}
		return nil
	case "auto-merge-attempt.json":
		if record.AutoMergeAttempt != record.Current || validateAutoMergeAttemptRef(root, record, record.Current) != nil {
			return errors.New("auto-merge attempt receipt does not bind the delivery record")
		}
		return nil
	case "merge-refusal.json":
		var receipt MergeRefusalReceipt
		if err := decode(&receipt); err != nil || receipt.SchemaVersion != "merge-refusal-receipt.v1" || receipt.HandoffID != record.HandoffID || receipt.Epoch != record.Epoch.Number || (receipt.Observation.State != "refused" && receipt.Observation.State != "unknown") || receipt.Arm != expectedMergeArm(DeliveryBead{Record: record}, receipt.Arm.ProtectionDigest) {
			return errors.New("merge-refusal receipt does not bind the delivery record")
		}
		return nil
	case "deadline.json":
		var receipt DeadlineReceipt
		if err := decode(&receipt); err != nil || receipt.SchemaVersion != "delivery-deadline-receipt.v1" || receipt.HandoffID != record.HandoffID || receipt.Epoch != record.Epoch.Number || receipt.Deadline != record.Deadline || !isTime(receipt.ObservedAt) || receipt.Outcome != "expired" || record.DeadlineOutcome != receipt.Outcome {
			return errors.New("deadline receipt does not bind the delivery record")
		}
		return nil
	default:
		return errors.New("delivery.v1 current receipt name is unsupported")
	}
}

// validateAutoMergeAttemptRef validates the handoff-wide fuse independently
// of Current so a successor cannot inherit an unchecked prior-epoch attempt.
func validateAutoMergeAttemptRef(root string, record DeliveryRecord, ref ReceiptRef) error {
	epoch, name, ok := deliveryReceiptLocation(record, ref)
	if !ok || epoch > record.Epoch.Number || name != "auto-merge-attempt.json" {
		return errors.New("auto-merge attempt has an illegal receipt location")
	}
	bytes, err := os.ReadFile(filepath.Join(root, ref.Path))
	if err != nil {
		return fmt.Errorf("auto-merge attempt is absent: %w", err)
	}
	sum := sha256.Sum256(bytes)
	if hex.EncodeToString(sum[:]) != ref.Digest {
		return errors.New("auto-merge attempt digest conflicts")
	}
	var receipt AutoMergeAttemptReceipt
	if err := decodeStrict(bytes, &receipt); err != nil {
		return fmt.Errorf("auto-merge attempt is not strict JSON: %w", err)
	}
	effectID := identifier("agentops.gc.delivery.auto-merge-effect.v1", record.HandoffID, record.Repository, record.PR.NodeID, record.Epoch.BaseRef, record.Epoch.Branch)
	arm := receipt.Arm
	wantArmID := identifier("agentops.gc.delivery.merge-arm.v1", arm.EffectID, strconv.Itoa(receipt.Epoch), arm.BaseOID, arm.Head, arm.GateDigest, "SQUASH")
	if receipt.SchemaVersion != "auto-merge-attempt-receipt.v1" || receipt.HandoffID != record.HandoffID || receipt.Epoch != epoch || receipt.Outcome != "prepared" || receipt.EffectID != effectID || record.AutoMergeEffectID != effectID || arm.ID != wantArmID || arm.EffectID != receipt.EffectID || arm.PRID != record.PR.ID || arm.Repository != record.Repository || arm.NodeID != record.PR.NodeID || arm.Number != record.PR.Number || arm.Branch != record.Epoch.Branch || arm.BaseRef != record.Epoch.BaseRef || !isHex(arm.BaseOID, 40) || !isHex(arm.Head, 40) || !isHex(arm.ProtectionDigest, 64) || !isHex(arm.GateDigest, 64) {
		return errors.New("auto-merge attempt does not bind stable handoff identity")
	}
	return nil
}

func validateRecordAutoMergeAttempt(root string, record DeliveryRecord) error {
	if record.AutoMergeAttempt == (ReceiptRef{}) {
		return nil
	}
	return validateAutoMergeAttemptRef(root, record, record.AutoMergeAttempt)
}

func hasOwnedAutoMergeAttempt(root string, record DeliveryRecord) bool {
	if record.AutoMergeAttempt != (ReceiptRef{}) {
		return validateAutoMergeAttemptRef(root, record, record.AutoMergeAttempt) == nil
	}
	state := markerStore{root: root, prefix: receiptNamespaceFor(record)}
	if !state.exists("auto-merge-attempt.json") {
		return false
	}
	return validateAutoMergeAttemptRef(root, record, receiptRef("auto-merge-attempt", state)) == nil
}

func (r *Reducer) validateSuccessorActivation(ctx context.Context, root string, child DeliveryBead) error {
	if child.Record.Predecessor == "" || child.Record.Current == (ReceiptRef{}) {
		return nil
	}
	_, name, ok := deliveryReceiptLocation(child.Record, child.Record.Current)
	if !ok || name != "activation.json" {
		return nil
	}
	predecessor, found, err := r.providers.FindDelivery(ctx, child.Record.Predecessor)
	if err != nil || !found {
		return errors.New("successor activation lacks exact predecessor")
	}
	return validateSuccessorActivationWithPredecessor(root, child, predecessor)
}

func validateSuccessorActivationWithPredecessor(root string, child, predecessor DeliveryBead) error {
	if child.Record.Epoch.Number != predecessor.Record.Epoch.Number+1 || child.Record.Predecessor != predecessor.ID || predecessor.Record.HandoffID != child.Record.HandoffID {
		return errors.New("successor activation predecessor identity conflicts")
	}
	childState := markerStore{root: root, prefix: receiptNamespaceFor(child.Record)}
	var activation struct {
		SchemaVersion string `json:"schema_version"`
		ChildDigest   string `json:"child_digest"`
		ReceiptDigest string `json:"successor_receipt_digest"`
	}
	if found, err := childState.read("activation.json", &activation); err != nil || !found || activation.SchemaVersion != "successor-activation.v1" {
		return errors.New("successor activation is absent or invalid")
	}
	predecessorState := markerStore{root: root, prefix: receiptNamespaceFor(predecessor.Record)}
	var created SuccessorCreatedReceipt
	if found, err := predecessorState.read("successor-created.json", &created); err != nil || !found {
		return errors.New("successor activation lacks created receipt")
	}
	childDigest, err := valueDigest(successorCreationRecord(child.Record))
	if err != nil || activation.ChildDigest != childDigest || activation.ReceiptDigest != receiptRef("successor-created", predecessorState).Digest || created.SchemaVersion != "successor-created-receipt.v1" || created.ChildID != child.ID || created.ChildDigest != childDigest {
		return errors.New("successor activation conflicts with child creation")
	}
	var move BaseMoveReceipt
	if found, err := predecessorState.read("base-move.json", &move); err != nil || !found || move.SchemaVersion != "base-move-receipt.v1" || move.HandoffID != predecessor.Record.HandoffID || move.Epoch != predecessor.Record.Epoch.Number || move.PreviousBaseOID != predecessor.Record.Epoch.BaseOID || move.CurrentBaseOID != child.Record.Epoch.BaseOID {
		return errors.New("successor activation lacks exact base-move evidence")
	}
	want := SuccessorIntent{SchemaVersion: "successor-intent.v1", HandoffID: predecessor.Record.HandoffID, PredecessorID: predecessor.ID, PredecessorReceiptDigest: receiptRef("base-move", predecessorState).Digest, PredecessorRevision: predecessor.Record.Revision, Epoch: child.Record.Epoch.Number, ChildID: child.ID, ExternalRef: child.ExternalRef, SemanticBead: child.Record.SemanticBead, TerminalRef: child.Record.TerminalRef, ReadyAt: child.Record.ReadyAt, Deadline: child.Record.Deadline, Mode: child.Record.Mode, Rig: child.Record.Rig, Repository: child.Record.Repository, Remote: child.Record.Remote, BaseRef: child.Record.Epoch.BaseRef, BaseOID: child.Record.Epoch.BaseOID, Branch: child.Record.Epoch.Branch, LeaseOID: child.Record.Epoch.LeaseOID, Candidate: child.Record.Candidate, Certificate: child.Record.Certificate, Committed: child.Record.Committed, Manifest: child.Record.Manifest, AutoMergeEffectID: child.Record.AutoMergeEffectID, AutoMergeAttempt: child.Record.AutoMergeAttempt}
	if created.Intent != want || child.Record.PredecessorReceiptDigest != want.PredecessorReceiptDigest {
		return errors.New("successor created intent does not fully bind the child")
	}
	return nil
}

func deliveryReceiptLocation(record DeliveryRecord, ref ReceiptRef) (int, string, bool) {
	if !isHex(ref.Digest, 64) || ref.Path != filepath.Clean(ref.Path) {
		return 0, "", false
	}
	prefix := filepath.Join("handoffs", record.HandoffID, "epochs") + string(filepath.Separator)
	if !strings.HasPrefix(ref.Path, prefix) {
		return 0, "", false
	}
	relative := strings.TrimPrefix(ref.Path, prefix)
	parts := strings.Split(relative, string(filepath.Separator))
	if len(parts) != 2 || len(parts[0]) != 6 || parts[1] == "" {
		return 0, "", false
	}
	epoch, err := strconv.Atoi(parts[0])
	if err != nil || epoch < 1 || epoch > record.Epoch.Number || fmt.Sprintf("%06d", epoch) != parts[0] {
		return 0, "", false
	}
	return epoch, parts[1], true
}

func validPRShape(record DeliveryRecord) (empty, stable, actual bool) {
	pr := record.PR
	empty = pr == (PullRequest{})
	stable = !empty && pr.ID != "" && isHex(pr.EffectID, 64) && pr.Repository == record.Repository && pr.BaseRef == record.Epoch.BaseRef && pr.Branch == record.Epoch.Branch
	actual = stable && pr.NodeID != "" && pr.Number != "" && strings.HasPrefix(pr.URL, "https://")
	return empty, stable, actual
}

func validStateFields(record DeliveryRecord) bool {
	hasEpoch := isHex(record.Epoch.Head, 40) && isHex(record.Epoch.Tree, 40)
	if (record.Epoch.Head == "") != (record.Epoch.Tree == "") {
		return false
	}
	if record.Epoch.Number == 1 {
		if record.Predecessor != "" || record.PredecessorReceiptDigest != "" || record.Epoch.LeaseOID != "" {
			return false
		}
	} else if record.Predecessor == "" || !isHex(record.PredecessorReceiptDigest, 64) {
		return false
	}
	if record.EpochSuccessorID != "" && record.EpochSuccessorID != "delivery-"+record.HandoffID[:20]+fmt.Sprintf("-e%06d", record.Epoch.Number+1) {
		return false
	}
	emptyPR, stablePR, actualPR := validPRShape(record)
	if !emptyPR && !stablePR {
		return false
	}
	switch record.State {
	case DeliveryStateBranchReady:
		if !hasEpoch {
			return false
		}
	case DeliveryStatePRPrepared:
		if !hasEpoch || !stablePR {
			return false
		}
	case DeliveryStatePROpen, DeliveryStateCIWait, DeliveryStateMergeEligible, DeliveryStateMergeArmed, DeliveryStateManualReview, DeliveryStateLanded:
		if !hasEpoch || !actualPR {
			return false
		}
	}
	if record.State == DeliveryStateMergeEligible || record.State == DeliveryStateMergeArmed || record.State == DeliveryStateManualReview {
		if !isHex(record.GateDigest, 64) {
			return false
		}
	}
	if record.State == DeliveryStateManualReview && record.Mode != "manual" {
		return false
	}
	if record.ArmID != "" && !isHex(record.ArmID, 64) || record.AutoMergeEffectID != "" && !isHex(record.AutoMergeEffectID, 64) {
		return false
	}
	if record.State == DeliveryStateMergeArmed && (record.Mode != "auto" || !isHex(record.ArmID, 64) || !isHex(record.AutoMergeEffectID, 64)) {
		return false
	}
	if record.State == DeliveryStateLanded && (!isHex(record.LandingDigest, 64) || !isHex(record.LandedSHA, 40)) {
		return false
	}
	if record.AutoMergeAttempt != (ReceiptRef{}) {
		epoch, name, ok := deliveryReceiptLocation(record, record.AutoMergeAttempt)
		if !ok || epoch > record.Epoch.Number || name != "auto-merge-attempt.json" || !isHex(record.AutoMergeEffectID, 64) {
			return false
		}
	}
	return true
}

func (r *Reducer) composeEpoch(ctx context.Context, state markerStore, bead DeliveryBead, prepared Prepared, request Request) (Result, error) {
	const name = "epoch.json"
	var receipt EpochReceipt
	if found, err := state.read(name, &receipt); err != nil {
		return Result{}, err
	} else if found {
		if !state.exists("branch.json") {
			return r.pushEpoch(ctx, state, bead, prepared, request)
		}
		if ok, err := state.branchReceiptMatches("branch.json", prepared, request.Target, receipt.branch()); err != nil || !ok {
			if err == nil {
				err = errors.New("branch receipt conflicts with epoch")
			}
			return Result{}, err
		}
		want := bead.Record
		want.State, want.Current, want.Revision = DeliveryStateBranchReady, receiptRef("branch", state), bead.Record.Revision+1
		want.Epoch.Head, want.Epoch.Tree = receipt.EpochHead, receipt.EpochTree
		return r.storeDeliveryRecord(ctx, bead, want)
	}
	planned := expectedBranch(prepared, request)
	planned.LeaseOID = bead.Record.Epoch.LeaseOID
	branch, err := r.providers.PrepareBranch(ctx, planned)
	if err != nil {
		switch {
		case errors.Is(err, errTargetRegression):
			return Result{Status: "target_regression", Reason: "base_moved_during_epoch_composition"}, nil
		case errors.Is(err, errPathCollision):
			return r.recordDeliveryOutcome(ctx, state, bead, DeliveryStateRepairWait, "path_collision")
		case errors.Is(err, errZeroDiff):
			return r.recordDeliveryOutcome(ctx, state, bead, DeliveryStateSuccessorRequired, "zero_diff_requires_semantic_recheck")
		}
		return Result{}, err
	}
	if !matchesEpochComposition(branch, planned) {
		return Result{}, errors.New("epoch composition returned conflicting identity")
	}
	if err := state.writeImmutable(name, makeEpochReceipt(prepared, request, planned, branch)); err != nil {
		return Result{}, err
	}
	return Result{Status: "epoch_composed", Effect: "git.compose_epoch"}, nil
}

func (r *Reducer) pushEpoch(ctx context.Context, state markerStore, bead DeliveryBead, prepared Prepared, request Request) (Result, error) {
	var epoch EpochReceipt
	found, err := state.read("epoch.json", &epoch)
	planned := expectedBranch(prepared, request)
	planned.LeaseOID = bead.Record.Epoch.LeaseOID
	if err != nil || !found || !epoch.matchesPlan(prepared, request, planned) {
		return Result{}, errors.New("delivery epoch lacks exact composition receipt")
	}
	branch := epoch.branch()
	observed, exists, err := r.providers.FindBranch(ctx, branch.Name)
	if err != nil {
		return Result{}, err
	}
	if exists && observed.Head == branch.Head {
		receipt, err := makeBranchReceipt(prepared, request.Target, branch, "observed")
		if err != nil {
			return Result{}, err
		}
		if err := state.writeImmutable("branch.json", receipt); err != nil {
			return Result{}, err
		}
		return Result{Status: "branch_observed"}, nil
	}
	if exists && branch.LeaseOID == "" {
		return Result{}, errors.New("remote delivery branch head conflicts with epoch receipt")
	}
	if exists && observed.Head != branch.LeaseOID {
		return Result{}, errors.New("remote delivery branch does not match epoch lease")
	}
	pusher, ok := r.providers.(EpochPusher)
	if !ok {
		return Result{}, errors.New("delivery provider does not implement epoch push")
	}
	if err := r.cut("before_branch_push"); err != nil {
		return Result{}, err
	}
	if err := pusher.PushBranch(ctx, branch); err != nil {
		return Result{}, err
	}
	if err := r.cut("after_branch_push"); err != nil {
		return Result{}, err
	}
	return Result{Status: "branch_pushed", Effect: "git.push_force_with_lease"}, nil
}

func (r *Reducer) preparePR(ctx context.Context, state markerStore, bead DeliveryBead, prepared Prepared, request Request) (Result, error) {
	intent := expectedPRIntent(prepared, request.Target.Repository, bead.Record.Epoch, bead.Record.PR)
	if !state.exists("pr-intent.json") {
		if err := r.cut("before_pr_intent"); err != nil {
			return Result{}, err
		}
		if err := state.writeImmutable("pr-intent.json", intent); err != nil {
			return Result{}, err
		}
		if err := r.cut("after_pr_intent"); err != nil {
			return Result{}, err
		}
		return Result{Status: "pr_intent"}, nil
	}
	var stored PRIntent
	if found, err := state.read("pr-intent.json", &stored); err != nil || !found || stored != intent {
		return Result{}, errors.New("PR intent conflicts with exact branch epoch")
	}
	want := bead.Record
	want.PR = stablePRFromIntent(intent)
	want.State, want.Current, want.Revision = DeliveryStatePRPrepared, receiptRef("pr-intent", state), bead.Record.Revision+1
	if err := r.cut("before_pr_prepare"); err != nil {
		return Result{}, err
	}
	result, err := r.storeDeliveryRecord(ctx, bead, want)
	if err != nil {
		return Result{}, err
	}
	if err := r.cut("after_pr_prepare"); err != nil {
		return Result{}, err
	}
	return result, nil
}

func expectedPRIntent(prepared Prepared, repository string, epoch DeliveryEpoch, known PullRequest) PRIntent {
	effect := identifier("agentops.gc.delivery.pr-effect.v1", prepared.HandoffID, repository, epoch.BaseRef, epoch.Branch)
	return PRIntent{SchemaVersion: "pr-intent.v1", HandoffID: prepared.HandoffID, Epoch: epoch.Number, Repository: repository, PRID: "pr-" + effect[:20], Branch: epoch.Branch, BaseRef: epoch.BaseRef, BaseOID: epoch.BaseOID, ExpectedHead: epoch.Head, EffectID: effect, NodeID: known.NodeID, Number: known.Number, URL: known.URL}
}

func stablePRFromIntent(intent PRIntent) PullRequest {
	return PullRequest{ID: intent.PRID, EffectID: intent.EffectID, Repository: intent.Repository, BaseRef: intent.BaseRef, Branch: intent.Branch, NodeID: intent.NodeID, Number: intent.Number, URL: intent.URL}
}

func (r *Reducer) openPreparedPR(ctx context.Context, state markerStore, bead DeliveryBead, prepared Prepared, request Request, observedBase string) (Result, error) {
	intent := expectedPRIntent(prepared, request.Target.Repository, bead.Record.Epoch, bead.Record.PR)
	var storedIntent PRIntent
	if found, err := state.read("pr-intent.json", &storedIntent); err != nil || !found || storedIntent != intent {
		return Result{}, errors.New("prepared PR lacks exact intent")
	}
	if state.exists("pr-open.json") {
		var receipt PROpenReceipt
		if found, err := state.read("pr-open.json", &receipt); err != nil || !found {
			return Result{}, errors.New("PR receipt disappeared")
		}
		observation := PRObservation{State: receipt.State, Draft: receipt.Draft, BaseOID: receipt.ObservedBaseOID, Head: receipt.ObservedHead, PR: PullRequest{ID: receipt.PRID, Repository: receipt.Repository, Branch: receipt.Branch, BaseRef: receipt.BaseRef, EffectID: receipt.EffectID, NodeID: receipt.NodeID, Number: receipt.Number, URL: receipt.URL}}
		if !matchesPROpenReceipt(receipt, observation, prepared, request.Target, intent) || !matchesPRObservation(observation, intent) {
			return Result{}, errors.New("PR receipt conflicts with intent")
		}
		want := bead.Record
		want.PR, want.State, want.Current, want.Revision = observation.PR, DeliveryStatePROpen, receiptRef("pr-open", state), bead.Record.Revision+1
		if err := r.cut("before_pr_open"); err != nil {
			return Result{}, err
		}
		result, err := r.storeDeliveryRecord(ctx, bead, want)
		if err != nil {
			return Result{}, err
		}
		if err := r.cut("after_pr_open"); err != nil {
			return Result{}, err
		}
		return result, nil
	}
	providers, ok := r.providers.(PRProviders)
	if !ok {
		return Result{}, errors.New("delivery provider does not implement PR boundary")
	}
	observation, err := providers.ObservePR(ctx, intent)
	if err != nil {
		return Result{}, err
	}
	if observation.State == "open" {
		if !matchesPRObservation(observation, intent) {
			return Result{}, errors.New("observed PR conflicts with intent")
		}
		outcome := "already_applied"
		if intent.NodeID != "" {
			outcome = "updated"
		}
		return r.writePRReceipt(state, prepared, request.Target, intent, observation, outcome, false)
	}
	if observation.State != "absent" || bead.Record.PR.NodeID != "" || bead.Record.PR.Number != "" || bead.Record.PR.URL != "" {
		return Result{}, errors.New("known PR cannot be created again")
	}
	currentBase, err := r.providers.ObserveBase(ctx, bead.Record.Epoch.BaseRef)
	if err != nil || !isHex(currentBase, 40) {
		return Result{}, errors.New("PR create could not re-observe exact current base")
	}
	if currentBase != bead.Record.Epoch.BaseOID {
		return r.observeBaseMove(ctx, state, bead, currentBase)
	}
	if err := r.cut("before_pr_create"); err != nil {
		return Result{}, err
	}
	created, err := providers.CreatePR(ctx, intent)
	if err != nil {
		return Result{}, err
	}
	if !matchesPRObservation(created, intent) {
		return Result{}, errors.New("PR create returned a conflicting identity")
	}
	if err := r.cut("after_pr_create"); err != nil {
		return Result{}, err
	}
	return r.writePRReceipt(state, prepared, request.Target, intent, created, "applied", true)
}

func matchesPROpenReceipt(receipt PROpenReceipt, observation PRObservation, prepared Prepared, target Target, intent PRIntent) bool {
	intentDigest, err := valueDigest(intent)
	if err != nil {
		return false
	}
	responseDigest, err := valueDigest(observation)
	if err != nil {
		return false
	}
	return receipt.SchemaVersion == "pr-open-receipt.v1" && receipt.IntentDigest == intentDigest && receipt.HandoffID == prepared.HandoffID && receipt.Epoch == prepared.Epoch && receipt.RigID == target.RigID && receipt.Repository == target.Repository && receipt.Remote == target.Remote && receipt.PRID == intent.PRID && receipt.Branch == intent.Branch && receipt.BaseRef == intent.BaseRef && receipt.BaseOID == intent.BaseOID && receipt.ExpectedHead == intent.ExpectedHead && receipt.EffectID == intent.EffectID && (receipt.Outcome == "applied" || receipt.Outcome == "already_applied" || receipt.Outcome == "updated") && receipt.ResponseDigest == responseDigest
}

func matchesPRObservation(observation PRObservation, intent PRIntent) bool {
	actual := observation.PR
	return observation.State == "open" && !observation.Draft && observation.BaseOID == intent.BaseOID && observation.Head == intent.ExpectedHead && actual.ID == intent.PRID && actual.Repository == intent.Repository && actual.Branch == intent.Branch && actual.BaseRef == intent.BaseRef && actual.EffectID == intent.EffectID && actual.NodeID != "" && actual.Number != "" && actual.URL != "" && (intent.NodeID == "" || actual.NodeID == intent.NodeID) && (intent.Number == "" || actual.Number == intent.Number) && (intent.URL == "" || actual.URL == intent.URL)
}

func (r *Reducer) writePRReceipt(state markerStore, prepared Prepared, target Target, intent PRIntent, observation PRObservation, outcome string, effect bool) (Result, error) {
	digest, err := valueDigest(observation)
	if err != nil {
		return Result{}, err
	}
	actual := observation.PR
	intentDigest, err := valueDigest(intent)
	if err != nil {
		return Result{}, err
	}
	receipt := PROpenReceipt{SchemaVersion: "pr-open-receipt.v1", IntentDigest: intentDigest, HandoffID: prepared.HandoffID, Epoch: prepared.Epoch, RigID: target.RigID, Repository: intent.Repository, Remote: target.Remote, PRID: actual.ID, Branch: actual.Branch, BaseRef: actual.BaseRef, BaseOID: intent.BaseOID, ObservedBaseOID: observation.BaseOID, ExpectedHead: intent.ExpectedHead, ObservedHead: observation.Head, EffectID: actual.EffectID, NodeID: actual.NodeID, Number: actual.Number, URL: actual.URL, State: observation.State, Draft: observation.Draft, Outcome: outcome, ResponseDigest: digest}
	if err := r.cut("before_pr_receipt"); err != nil {
		return Result{}, err
	}
	if err := state.writeImmutable("pr-open.json", receipt); err != nil {
		return Result{}, err
	}
	if err := r.cut("after_pr_receipt"); err != nil {
		return Result{}, err
	}
	result := Result{Status: "pr_receipted"}
	if effect {
		result.Effect = "forge.pr"
	}
	return result, nil
}

func exactRequest(request Request) (Request, error) {
	certificate, err := decodeCertificate(request.CertificateBytes)
	if err != nil {
		return Request{}, err
	}
	if !reflect.DeepEqual(certificate, request.Certificate) {
		return Request{}, errors.New("certificate bytes and parsed certificate disagree")
	}
	request.Certificate = certificate
	return request, validateRequest(request)
}

func (r *Reducer) requireTerminal(ctx context.Context, request Request) error {
	if err := r.cut("before_terminal"); err != nil {
		return err
	}
	terminal, err := r.providers.Terminal(ctx, request.Target.SemanticBeadID)
	if err != nil {
		return err
	}
	if !matchesTerminal(terminal, request) {
		return errors.New("terminal semantic identity does not match PASS certificate")
	}
	return r.cut("after_terminal")
}

func matchesTerminal(terminal Terminal, request Request) bool {
	return terminal.BeadID == request.Target.SemanticBeadID && terminal.Ref == request.Target.SemanticTerminalRef && terminal.Verdict == "PASS" && terminal.CertificateDigest == request.CertificateDigest
}

func expectedBranch(prepared Prepared, request Request) Branch {
	proof := identifier("agentops.gc.epoch-proof.v1", prepared.HandoffID, request.Certificate.Candidate.Commit, request.Certificate.ChangedPathManifest, request.Target.BaseOID)
	return Branch{Name: "gc/delivery/" + prepared.HandoffID[:20], BaseRef: request.Target.BaseRef, BaseOID: request.Target.BaseOID, Head: request.Certificate.Candidate.Commit, Proof: proof}
}

// Head is intentionally produced by the isolated epoch composition, not by
// the semantic candidate.  The candidate is the input plan head; the returned
// branch head is a new one-parent commit on the observed target base.
func matchesEpochComposition(actual, planned Branch) bool {
	return actual.Name == planned.Name && actual.BaseRef == planned.BaseRef && actual.BaseOID == planned.BaseOID && actual.Proof == planned.Proof && isHex(actual.Head, 40) && isHex(actual.Tree, 40)
}

func (r *Reducer) cut(point string) error {
	if r.crash == nil {
		return nil
	}
	return r.crash(point)
}

func validateRequest(request Request) error {
	checks := []func(Request) error{
		validateTarget, validateCertificateDigest, validateCertificateIdentity,
		validateCertificateProvenance, validateDeliveryPolicy,
		validateExactInputs,
	}
	for _, check := range checks {
		if err := check(request); err != nil {
			return err
		}
	}
	return nil
}

func validateExactInputs(request Request) error {
	// Unit-only reducer tests may omit the native inputs. The production CLI
	// requires them before constructing a provider; the subject identity is the
	// executor canonical_manifest_digest, never a raw JSON-byte digest.
	if len(request.SubjectBytes) == 0 && len(request.NativeBytes) == 0 {
		return nil
	}
	if len(request.SubjectBytes) == 0 || len(request.NativeBytes) == 0 {
		return errors.New("subject manifest and native context must be supplied together")
	}
	manifest, err := DecodeExactSubjectManifest(request.SubjectBytes, request.SubjectDigest)
	if err != nil || !reflect.DeepEqual(manifest, request.SubjectManifest) || request.Certificate.ChangedPathManifest != manifest.CanonicalManifestDigest || request.SubjectDigest != manifest.CanonicalManifestDigest {
		return errors.New("subject manifest does not bind the admitted certificate")
	}
	native, err := DecodeExactNativeContext(request.NativeBytes, request.NativeDigest)
	if err != nil || !reflect.DeepEqual(native, request.NativeContext) || native.RigID != request.Target.RigID || native.Repository != request.Target.Repository || native.Remote != request.Target.Remote || native.BaseRef != request.Target.BaseRef {
		return errors.New("native context does not bind the delivery target")
	}
	return nil
}

func validateTarget(request Request) error {
	values := []string{request.Root, request.Target.SemanticBeadID, request.Target.SemanticTerminalRef, request.Target.RigID, request.Target.Repository, request.Target.Remote, request.Target.BaseRef}
	if hasEmpty(values) || !isHex(request.Target.BaseOID, 40) {
		return errors.New("delivery target identity is required")
	}
	return nil
}

func hasEmpty(values []string) bool {
	for _, value := range values {
		if value == "" {
			return true
		}
	}
	return false
}

func validateCertificateDigest(request Request) error {
	actual := sha256.Sum256(request.CertificateBytes)
	if len(request.CertificateBytes) == 0 || request.CertificateDigest != hex.EncodeToString(actual[:]) {
		return errors.New("certificate digest does not match exact bytes")
	}
	return nil
}

func validateCertificateIdentity(request Request) error {
	certificate := request.Certificate
	if certificate.SchemaVersion != "admission-certificate.v2" || certificate.Verdict != "PASS" || certificate.SemanticBeadID != request.Target.SemanticBeadID {
		return errors.New("certificate is not an exact PASS for target")
	}
	if !validCertificateDigests(certificate, request.CertificateDigest) {
		return errors.New("certificate identity digest is malformed")
	}
	return nil
}

func validCertificateDigests(certificate AdmissionCertificate, certificateDigest string) bool {
	values := []struct {
		value string
		width int
	}{
		{certificate.IntentDigest, 64}, {certificate.Candidate.Commit, 40}, {certificate.Candidate.Tree, 40},
		{certificate.Candidate.ContentDigest, 64}, {certificate.Store.Digest, 64}, {certificate.ChangedPathManifest, 64},
		{certificate.VerdictDigest, 64}, {certificate.EvidenceDigest, 64}, {certificateDigest, 64},
	}
	if certificate.Store.Identity == "" {
		return false
	}
	for _, value := range values {
		if !isHex(value.value, value.width) {
			return false
		}
	}
	return true
}

func validateCertificateProvenance(request Request) error {
	attestations := request.Certificate.Attestations
	if request.Certificate.DeliveryGroupID == "" || !validPrefixSafety(request.Certificate.PrefixSafety) || !validRuntime(attestations.Author, "author") || !validRuntime(attestations.Validator, "validator") || attestations.Author.ContextID == attestations.Validator.ContextID {
		return errors.New("certificate provenance is not an exact admitted profile")
	}
	return nil
}

func validPrefixSafety(value string) bool {
	return value == "safe" || value == "atomic_group" || value == "externally_gated"
}

func validateDeliveryPolicy(request Request) error {
	target := request.Target
	if target.Epoch < 1 || !validMode(target.Mode) || !isTime(target.Deadline) || !isTime(target.PreparedAt) || !isTime(target.CommittedAt) || (target.ObservedAt != "" && !isTime(target.ObservedAt)) {
		return errors.New("delivery epoch or mode is invalid")
	}
	return nil
}

func deadlineExpired(target Target) bool {
	if target.ObservedAt == "" {
		return false
	}
	deadline, deadlineErr := time.Parse(time.RFC3339, target.Deadline)
	observed, observedErr := time.Parse(time.RFC3339, target.ObservedAt)
	return deadlineErr == nil && observedErr == nil && !observed.Before(deadline)
}

func validMode(value string) bool { return value == "auto" || value == "manual" }

func decodeCertificate(value []byte) (AdmissionCertificate, error) {
	var certificate AdmissionCertificate
	if err := decodeStrict(value, &certificate); err != nil {
		return AdmissionCertificate{}, fmt.Errorf("invalid exact certificate bytes: %w", err)
	}
	return certificate, nil
}
func decodeStrict(value []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func isTime(value string) bool { _, err := time.Parse(time.RFC3339, value); return err == nil }

type runtimeProfile struct {
	requestedModel, requestedReasoning, requestedProvider      string
	actualModel, actualReasoning, actualProvider, actualEffort string
}

var runtimeProfiles = map[string][]runtimeProfile{
	"author": {
		{"terra", "high", "codex", "gpt-5.6-terra", "high", "codex", "high"},
		{"opus", "medium", "claude", "claude-opus-4-8", "medium", "claude", "medium"},
	},
	"validator": {{"sol", "high", "codex", "gpt-5.6-sol", "high", "codex", "high"}},
}

func validRuntime(runtime Runtime, role string) bool {
	if runtime.ContextID == "" || !validFallback(runtime.Fallback) {
		return false
	}
	for _, expected := range runtimeProfiles[role] {
		if profileOf(runtime) == expected {
			return true
		}
	}
	return false
}

func validFallback(fallback *Fallback) bool {
	return fallback != nil && fallback.Allowed != nil && fallback.Used != nil && !*fallback.Allowed && !*fallback.Used && bytes.Equal(fallback.Reason, []byte("null"))
}

func profileOf(runtime Runtime) runtimeProfile {
	return runtimeProfile{runtime.RequestedModel, runtime.RequestedReasoning, runtime.RequestedProvider, runtime.ActualModel, runtime.ActualReasoning, runtime.ActualProvider, runtime.ActualEffort}
}

func isHex(value string, width int) bool {
	if len(value) != width {
		return false
	}
	for i := range value {
		if (value[i] < '0' || value[i] > '9') && (value[i] < 'a' || value[i] > 'f') {
			return false
		}
	}
	return true
}
func identifier(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

type Prepared struct {
	SchemaVersion              string `json:"schema_version"`
	HandoffID                  string `json:"handoff_id"`
	SemanticBeadID             string `json:"semantic_bead_id"`
	SemanticTerminalRef        string `json:"semantic_terminal_ref"`
	AdmissionCertificateRef    string `json:"admission_certificate_ref"`
	AdmissionCertificateDigest string `json:"admission_certificate_digest"`
	DeliveryBeadID             string `json:"expected_delivery_bead_id"`
	ExternalRef                string `json:"expected_external_ref"`
	Epoch                      int    `json:"epoch"`
	Mode                       string `json:"mode"`
	State                      string `json:"state"`
	Deadline                   string `json:"deadline"`
	PreparedAt                 string `json:"prepared_at"`
}
type HandoffDeliveryArtifact struct {
	SchemaVersion              string  `json:"schema_version"`
	Kind                       string  `json:"kind"`
	HandoffID                  string  `json:"handoff_id"`
	SemanticBeadID             string  `json:"semantic_bead_id"`
	SemanticTerminalRef        string  `json:"semantic_terminal_ref"`
	AdmissionCertificateDigest string  `json:"admission_certificate_digest"`
	DeliveryBeadID             string  `json:"delivery_bead_id"`
	ExternalRef                string  `json:"external_ref"`
	Epoch                      int     `json:"epoch"`
	PredecessorReceiptDigest   *string `json:"predecessor_receipt_digest"`
	Mode                       string  `json:"mode"`
	State                      string  `json:"state"`
	Publication                string  `json:"publication"`
	Deadline                   string  `json:"deadline"`
	EffectGate                 any     `json:"effect_gate"`
	SuccessorBeadID            *string `json:"successor_bead_id"`
}
type Committed struct {
	SchemaVersion              string `json:"schema_version"`
	HandoffID                  string `json:"handoff_id"`
	PreparedDigest             string `json:"prepared_digest"`
	SemanticBeadID             string `json:"semantic_bead_id"`
	SemanticTerminalVerdict    string `json:"semantic_terminal_verdict"`
	SemanticTerminalRef        string `json:"semantic_terminal_ref"`
	AdmissionCertificateDigest string `json:"admission_certificate_digest"`
	DeliveryBeadID             string `json:"delivery_bead_id"`
	ExternalRef                string `json:"expected_external_ref"`
	Epoch                      int    `json:"epoch"`
	DeliveryPayloadRef         string `json:"delivery_payload_ref"`
	DeliveryPayloadDigest      string `json:"delivery_payload_digest"`
	Mode                       string `json:"mode"`
	State                      string `json:"state"`
	Deadline                   string `json:"deadline"`
	CommittedAt                string `json:"committed_at"`
}
type BranchReceipt struct {
	SchemaVersion  string `json:"schema_version"`
	HandoffID      string `json:"handoff_id"`
	Epoch          int    `json:"epoch"`
	RigID          string `json:"rig_id"`
	Repository     string `json:"repository"`
	Remote         string `json:"remote"`
	Branch         string `json:"branch"`
	BaseRef        string `json:"base_ref"`
	BaseOID        string `json:"base_oid"`
	ExpectedHead   string `json:"expected_head"`
	LeaseOID       string `json:"lease_oid,omitempty"`
	Outcome        string `json:"outcome"`
	ResponseDigest string `json:"response_digest"`
}

// EpochReceipt is the durable crash fence between deterministic local
// composition and the single remote force-with-lease branch effect.
type EpochReceipt struct {
	SchemaVersion         string `json:"schema_version"`
	HandoffID             string `json:"handoff_id"`
	Epoch                 int    `json:"epoch"`
	Repository            string `json:"repository"`
	Remote                string `json:"remote"`
	BaseRef               string `json:"base_ref"`
	ObservedBaseOID       string `json:"observed_base_oid"`
	Candidate             string `json:"candidate"`
	SubjectManifestDigest string `json:"subject_manifest_digest"`
	PathProof             string `json:"path_proof"`
	Branch                string `json:"branch"`
	LeaseOID              string `json:"lease_oid,omitempty"`
	EpochHead             string `json:"epoch_head"`
	EpochTree             string `json:"epoch_tree"`
}
type PROpenReceipt struct {
	SchemaVersion   string `json:"schema_version"`
	IntentDigest    string `json:"intent_digest"`
	HandoffID       string `json:"handoff_id"`
	Epoch           int    `json:"epoch"`
	RigID           string `json:"rig_id"`
	Repository      string `json:"repository"`
	Remote          string `json:"remote"`
	PRID            string `json:"pr_id"`
	Branch          string `json:"branch"`
	BaseRef         string `json:"base_ref"`
	BaseOID         string `json:"expected_base_oid"`
	ObservedBaseOID string `json:"observed_base_oid"`
	ExpectedHead    string `json:"expected_head"`
	ObservedHead    string `json:"observed_head"`
	EffectID        string `json:"effect_id"`
	NodeID          string `json:"node_id"`
	Number          string `json:"number"`
	URL             string `json:"url"`
	State           string `json:"state"`
	Draft           bool   `json:"draft"`
	Outcome         string `json:"outcome"`
	ResponseDigest  string `json:"response_digest"`
}
type GateReceipt struct {
	SchemaVersion string     `json:"schema_version"`
	HandoffID     string     `json:"handoff_id"`
	Epoch         int        `json:"epoch"`
	PRID          string     `json:"pr_id"`
	Head          string     `json:"head"`
	BaseRef       string     `json:"base_ref"`
	BaseOID       string     `json:"base_oid"`
	Gate          HostedGate `json:"gate"`
	GateDigest    string     `json:"gate_digest"`
}
type MergeRefusalReceipt struct {
	SchemaVersion string           `json:"schema_version"`
	HandoffID     string           `json:"handoff_id"`
	Epoch         int              `json:"epoch"`
	Arm           MergeArm         `json:"arm"`
	Observation   MergeObservation `json:"observation"`
}
type LandingReceipt struct {
	SchemaVersion string   `json:"schema_version"`
	HandoffID     string   `json:"handoff_id"`
	Epoch         int      `json:"epoch"`
	PRID          string   `json:"pr_id"`
	Head          string   `json:"head"`
	LandedSHA     string   `json:"landed_sha"`
	Tree          string   `json:"tree"`
	Parents       []string `json:"parents"`
}

func makePrepared(r Request) Prepared {
	// Moving main changes an epoch, not the delivery handoff identity.  The
	// candidate admission and semantic target remain the stable handoff.
	handoff := identifier("agentops.gc.delivery.handoff.v1", r.CertificateDigest, r.Target.SemanticBeadID, r.Target.SemanticTerminalRef, r.Target.RigID, r.Target.Repository, r.Target.Remote, r.Target.BaseRef, r.Target.Mode)
	return Prepared{SchemaVersion: "handoff-prepared.v1", HandoffID: handoff, SemanticBeadID: r.Target.SemanticBeadID, SemanticTerminalRef: r.Target.SemanticTerminalRef, AdmissionCertificateRef: "certificate:sha256:" + r.CertificateDigest, AdmissionCertificateDigest: r.CertificateDigest, DeliveryBeadID: "delivery-" + handoff[:20] + fmt.Sprintf("-e%06d", r.Target.Epoch), ExternalRef: "handoff:" + handoff + fmt.Sprintf(":epoch:%d", r.Target.Epoch), Epoch: r.Target.Epoch, Mode: r.Target.Mode, State: "queued", Deadline: r.Target.Deadline, PreparedAt: r.Target.PreparedAt}
}
func makeDelivery(p Prepared, publication string) HandoffDeliveryArtifact {
	return HandoffDeliveryArtifact{SchemaVersion: "delivery.v1", Kind: "delivery", HandoffID: p.HandoffID, SemanticBeadID: p.SemanticBeadID, SemanticTerminalRef: p.SemanticTerminalRef, AdmissionCertificateDigest: p.AdmissionCertificateDigest, DeliveryBeadID: p.DeliveryBeadID, ExternalRef: p.ExternalRef, Epoch: p.Epoch, Mode: p.Mode, State: p.State, Publication: publication, Deadline: p.Deadline}
}
func makeCommitted(p Prepared, preparedDigest, publishedDigest, committedAt string) Committed {
	return Committed{SchemaVersion: "handoff-committed.v1", HandoffID: p.HandoffID, PreparedDigest: preparedDigest, SemanticBeadID: p.SemanticBeadID, SemanticTerminalVerdict: "PASS", SemanticTerminalRef: p.SemanticTerminalRef, AdmissionCertificateDigest: p.AdmissionCertificateDigest, DeliveryBeadID: p.DeliveryBeadID, ExternalRef: p.ExternalRef, Epoch: p.Epoch, DeliveryPayloadRef: publishedFile, DeliveryPayloadDigest: publishedDigest, Mode: p.Mode, State: p.State, Deadline: p.Deadline, CommittedAt: committedAt}
}
func makeBranchReceipt(p Prepared, target Target, branch Branch, outcome string) (BranchReceipt, error) {
	digest, err := valueDigest(branchPublicIdentity(branch))
	if err != nil {
		return BranchReceipt{}, err
	}
	return BranchReceipt{SchemaVersion: "branch-receipt.v1", HandoffID: p.HandoffID, Epoch: p.Epoch, RigID: target.RigID, Repository: target.Repository, Remote: target.Remote, Branch: branch.Name, BaseRef: branch.BaseRef, BaseOID: branch.BaseOID, ExpectedHead: branch.Head, LeaseOID: branch.LeaseOID, Outcome: outcome, ResponseDigest: digest}, nil
}

func branchPublicIdentity(branch Branch) Branch {
	return Branch{Name: branch.Name, BaseRef: branch.BaseRef, BaseOID: branch.BaseOID, Head: branch.Head, LeaseOID: branch.LeaseOID}
}

func makeEpochReceipt(prepared Prepared, request Request, planned, actual Branch) EpochReceipt {
	return EpochReceipt{SchemaVersion: "epoch-receipt.v1", HandoffID: prepared.HandoffID, Epoch: prepared.Epoch, Repository: request.Target.Repository, Remote: request.Target.Remote, BaseRef: planned.BaseRef, ObservedBaseOID: planned.BaseOID, Candidate: planned.Head, SubjectManifestDigest: request.Certificate.ChangedPathManifest, PathProof: planned.Proof, Branch: actual.Name, LeaseOID: planned.LeaseOID, EpochHead: actual.Head, EpochTree: actual.Tree}
}

func (receipt EpochReceipt) branch() Branch {
	return Branch{Name: receipt.Branch, BaseRef: receipt.BaseRef, BaseOID: receipt.ObservedBaseOID, Head: receipt.EpochHead, Tree: receipt.EpochTree, Proof: receipt.PathProof, LeaseOID: receipt.LeaseOID}
}

func (receipt EpochReceipt) matchesPlan(prepared Prepared, request Request, planned Branch) bool {
	return receipt.SchemaVersion == "epoch-receipt.v1" && receipt.HandoffID == prepared.HandoffID && receipt.Epoch == prepared.Epoch && receipt.Repository == request.Target.Repository && receipt.Remote == request.Target.Remote && receipt.BaseRef == planned.BaseRef && receipt.ObservedBaseOID == planned.BaseOID && receipt.Candidate == planned.Head && receipt.SubjectManifestDigest == request.Certificate.ChangedPathManifest && receipt.PathProof == planned.Proof && receipt.Branch == planned.Name && receipt.LeaseOID == planned.LeaseOID && isHex(receipt.EpochHead, 40) && isHex(receipt.EpochTree, 40)
}

func valueDigest(value any) (string, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:]), nil
}

type markerStore struct{ root, prefix string }

func (s markerStore) path(name string) string {
	name = strings.TrimPrefix(name, "receipts/")
	if s.prefix != "" {
		name = filepath.Join(s.prefix, name)
	}
	return filepath.Join(s.root, name)
}
func (s markerStore) exists(name string) bool { _, err := os.Stat(s.path(name)); return err == nil }
func (s markerStore) read(name string, into any) (bool, error) {
	bytes, err := os.ReadFile(s.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := decodeStrict(bytes, into); err != nil {
		return false, fmt.Errorf("invalid immutable artifact %s: %w", name, err)
	}
	return true, nil
}
func (s markerStore) branchReceiptMatches(name string, prepared Prepared, target Target, branch Branch) (bool, error) {
	var receipt BranchReceipt
	found, err := s.read(name, &receipt)
	if err != nil || !found {
		return false, err
	}
	digest, err := valueDigest(branchPublicIdentity(branch))
	if err != nil {
		return false, err
	}
	return receipt.SchemaVersion == "branch-receipt.v1" && (receipt.Outcome == "created" || receipt.Outcome == "adopted" || receipt.Outcome == "observed") && receipt.HandoffID == prepared.HandoffID && receipt.Epoch == prepared.Epoch && receipt.RigID == target.RigID && receipt.Repository == target.Repository && receipt.Remote == target.Remote && receipt.Branch == branch.Name && receipt.BaseRef == branch.BaseRef && receipt.BaseOID == branch.BaseOID && receipt.ExpectedHead == branch.Head && receipt.LeaseOID == branch.LeaseOID && receipt.ResponseDigest == digest, nil
}
func (s markerStore) bytes(value any) ([]byte, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(bytes, '\n'), nil
}
func (s markerStore) writeImmutable(name string, value any) error {
	expected, err := s.bytes(value)
	if err != nil {
		return err
	}
	path := s.path(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err == nil {
		written, writeErr := file.Write(expected)
		syncErr := file.Sync()
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if written != len(expected) {
			return io.ErrShortWrite
		}
		if syncErr != nil {
			return syncErr
		}
		if closeErr != nil {
			return closeErr
		}
		parent, openErr := os.Open(filepath.Dir(path))
		if openErr != nil {
			return openErr
		}
		syncErr = parent.Sync()
		closeErr = parent.Close()
		if syncErr != nil {
			return syncErr
		}
		return closeErr
	}
	if !errors.Is(err, os.ErrExist) {
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(existing) != string(expected) {
		return fmt.Errorf("immutable artifact conflict: %s", name)
	}
	return nil
}
func (s markerStore) writeBytesImmutable(name string, value []byte) error {
	path := s.path(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return s.matchesBytes(name, value)
	}
	if err != nil {
		return err
	}
	written, writeErr := file.Write(value)
	syncErr, closeErr := file.Sync(), file.Close()
	if writeErr != nil {
		return writeErr
	}
	if written != len(value) {
		return io.ErrShortWrite
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
func (s markerStore) matchesBytes(name string, want []byte) error {
	got, err := os.ReadFile(s.path(name))
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return errors.New("immutable artifact identity conflict")
	}
	return nil
}
func (s markerStore) matches(name string, value any) error {
	expected, err := s.bytes(value)
	if err != nil {
		return err
	}
	actual, err := os.ReadFile(s.path(name))
	if err != nil {
		return err
	}
	if string(actual) != string(expected) {
		return fmt.Errorf("immutable artifact conflict: %s", name)
	}
	return nil
}
func (s markerStore) digest(name string) string {
	bytes, err := os.ReadFile(s.path(name))
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}
