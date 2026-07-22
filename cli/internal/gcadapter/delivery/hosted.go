package delivery

import (
	"context"
	"errors"
	"strconv"
)

type HostedDeliveryProviders interface {
	HostedGate(context.Context, PullRequest) (HostedGate, error)
	ObserveMerge(context.Context, MergeArm) (MergeObservation, error)
	ArmMerge(context.Context, MergeArm) (MergeArm, error)
	Landing(context.Context, PullRequest) (Landing, bool, error)
}

type DeadlineReceipt struct {
	SchemaVersion string `json:"schema_version"`
	HandoffID     string `json:"handoff_id"`
	Epoch         int    `json:"epoch"`
	Deadline      string `json:"deadline"`
	ObservedAt    string `json:"observed_at"`
	Outcome       string `json:"outcome"`
}

type MergeArmReceipt struct {
	SchemaVersion string   `json:"schema_version"`
	HandoffID     string   `json:"handoff_id"`
	Epoch         int      `json:"epoch"`
	Arm           MergeArm `json:"arm"`
}

type AutoMergeAttemptReceipt struct {
	SchemaVersion string   `json:"schema_version"`
	HandoffID     string   `json:"handoff_id"`
	Epoch         int      `json:"epoch"`
	EffectID      string   `json:"effect_id"`
	Arm           MergeArm `json:"arm"`
	Outcome       string   `json:"outcome"`
}

type AutoMergeResultReceipt struct {
	SchemaVersion string `json:"schema_version"`
	HandoffID     string `json:"handoff_id"`
	Epoch         int    `json:"epoch"`
	EffectID      string `json:"effect_id"`
	ArmID         string `json:"arm_id"`
	Outcome       string `json:"outcome"`
}

func isHostedState(state DeliveryState) bool {
	switch state {
	case DeliveryStatePROpen, DeliveryStateCIWait, DeliveryStateMergeEligible, DeliveryStateMergeArmed, DeliveryStateManualReview:
		return true
	default:
		return false
	}
}

func (r *Reducer) enterCIWait(ctx context.Context, bead DeliveryBead) (Result, error) {
	return r.storeDeliveryTransition(ctx, bead, DeliveryStateCIWait, bead.Record.Current)
}

func (r *Reducer) recordDeadline(ctx context.Context, state markerStore, bead DeliveryBead, target Target) (Result, error) {
	receipt := DeadlineReceipt{SchemaVersion: "delivery-deadline-receipt.v1", HandoffID: bead.Record.HandoffID, Epoch: bead.Record.Epoch.Number, Deadline: bead.Record.Deadline, ObservedAt: target.ObservedAt, Outcome: "expired"}
	const name = "deadline.json"
	if !state.exists(name) {
		if err := state.writeImmutable(name, receipt); err != nil {
			return Result{}, err
		}
		return Result{Status: "deadline_observed", Reason: "deadline_expired"}, nil
	}
	if err := state.matches(name, receipt); err != nil {
		return Result{}, err
	}
	want := bead.Record
	want.State, want.Current, want.DeadlineOutcome, want.Revision = DeliveryStateStalled, receiptRef("deadline", state), receipt.Outcome, bead.Record.Revision+1
	result, err := r.storeDeliveryRecord(ctx, bead, want)
	result.Reason = "deadline_expired"
	return result, err
}

func (r *Reducer) advanceHosted(ctx context.Context, state markerStore, bead DeliveryBead, prepared Prepared, request Request, observedBase string) (Result, error) {
	providers, ok := r.providers.(HostedDeliveryProviders)
	if !ok {
		return Result{}, errors.New("delivery provider does not implement hosted delivery boundary")
	}
	landing, found, err := providers.Landing(ctx, bead.Record.PR)
	if err != nil {
		return Result{}, err
	}
	if found {
		return r.acceptLanding(ctx, state, bead, landing, observedBase)
	}
	if deadlineExpired(request.Target) {
		return r.recordDeadline(ctx, state, bead, request.Target)
	}
	if bead.Record.Epoch.BaseOID != observedBase {
		return r.observeBaseMove(ctx, state, bead, observedBase)
	}
	switch bead.Record.State {
	case DeliveryStateCIWait:
		return r.advanceCIWait(ctx, state, bead, prepared, providers)
	case DeliveryStateMergeEligible:
		return r.prepareMergeArm(ctx, state, bead, prepared, providers)
	case DeliveryStateMergeArmed:
		return r.advanceMergeArm(ctx, state, bead, prepared, providers)
	case DeliveryStateManualReview:
		return Result{Status: "manual_review"}, nil
	default:
		return Result{}, errors.New("hosted reducer received an illegal state")
	}
}

func (r *Reducer) acceptLanding(ctx context.Context, state markerStore, bead DeliveryBead, landing Landing, observedBase string) (Result, error) {
	if landing.PRID != bead.Record.PR.ID || landing.Head != bead.Record.Epoch.Head || landing.Tree != bead.Record.Epoch.Tree || !isHex(landing.SHA, 40) || len(landing.Parents) != 1 || landing.Parents[0] != bead.Record.Epoch.BaseOID {
		return Result{}, errors.New("landing does not match exact squash epoch identity")
	}
	if observedBase != landing.SHA {
		contains, err := r.providers.BaseDescends(ctx, observedBase, landing.SHA)
		if err != nil || !contains {
			return Result{}, errors.New("current base does not contain exact landing")
		}
	}
	receipt := LandingReceipt{SchemaVersion: "landing-receipt.v1", HandoffID: bead.Record.HandoffID, Epoch: bead.Record.Epoch.Number, PRID: landing.PRID, Head: landing.Head, LandedSHA: landing.SHA, Tree: landing.Tree, Parents: landing.Parents}
	const name = "landing.json"
	if !state.exists(name) {
		if err := state.writeImmutable(name, receipt); err != nil {
			return Result{}, err
		}
		return Result{Status: "landing_receipted"}, nil
	}
	if err := state.matches(name, receipt); err != nil {
		return Result{}, err
	}
	digest, err := valueDigest(landing)
	if err != nil {
		return Result{}, err
	}
	want := bead.Record
	want.State, want.Current, want.LandingDigest, want.LandedSHA, want.Revision = DeliveryStateLanded, receiptRef("landing", state), digest, landing.SHA, bead.Record.Revision+1
	if state.exists("auto-merge-attempt.json") {
		want.AutoMergeAttempt = receiptRef("auto-merge-attempt", state)
	}
	return r.storeDeliveryRecord(ctx, bead, want)
}

func (r *Reducer) advanceCIWait(ctx context.Context, state markerStore, bead DeliveryBead, prepared Prepared, providers HostedDeliveryProviders) (Result, error) {
	gate, err := providers.HostedGate(ctx, bead.Record.PR)
	if err != nil {
		return Result{}, err
	}
	qualified, reason, err := qualifyHostedGate(gate, bead)
	if err != nil {
		return r.recordDeliveryOutcome(ctx, state, bead, DeliveryStateRepairWait, "protection_ambiguity")
	}
	if !qualified {
		return Result{Status: "ci_wait", Reason: reason}, nil
	}
	if gate.AutoMergeEnabled && !hasOwnedAutoMergeAttempt(state.root, bead.Record) {
		return r.recordDeliveryOutcome(ctx, state, bead, DeliveryStateRepairWait, "existing_auto_merge")
	}
	digest, err := hostedGateDigest(gate)
	if err != nil {
		return Result{}, err
	}
	receipt := GateReceipt{SchemaVersion: "hosted-gate-receipt.v1", HandoffID: prepared.HandoffID, Epoch: prepared.Epoch, PRID: bead.Record.PR.ID, Head: bead.Record.Epoch.Head, BaseRef: bead.Record.Epoch.BaseRef, BaseOID: bead.Record.Epoch.BaseOID, Gate: gate, GateDigest: digest}
	const name = "hosted-gate.json"
	if !state.exists(name) {
		if err := r.cut("before_gate"); err != nil {
			return Result{}, err
		}
		if err := state.writeImmutable(name, receipt); err != nil {
			return Result{}, err
		}
		if err := r.cut("after_gate"); err != nil {
			return Result{}, err
		}
		return Result{Status: "gate_receipted"}, nil
	}
	if err := state.matches(name, receipt); err != nil {
		return Result{}, errors.New("hosted gate changed after qualification")
	}
	want := bead.Record
	want.GateDigest, want.Current, want.Revision = digest, receiptRef("hosted-gate", state), bead.Record.Revision+1
	if bead.Record.Mode == "manual" {
		want.State = DeliveryStateManualReview
	} else {
		want.State = DeliveryStateMergeEligible
	}
	return r.storeDeliveryRecord(ctx, bead, want)
}

func (r *Reducer) prepareMergeArm(ctx context.Context, state markerStore, bead DeliveryBead, prepared Prepared, providers HostedDeliveryProviders) (Result, error) {
	gate, err := providers.HostedGate(ctx, bead.Record.PR)
	if err != nil {
		return Result{}, err
	}
	qualified, reason, err := qualifyHostedGate(gate, bead)
	if err != nil {
		return r.recordDeliveryOutcome(ctx, state, bead, DeliveryStateRepairWait, "protection_ambiguity")
	}
	if !qualified {
		return Result{Status: "merge_eligible", Reason: reason}, nil
	}
	if gate.AutoMergeEnabled && !hasOwnedAutoMergeAttempt(state.root, bead.Record) {
		return r.recordDeliveryOutcome(ctx, state, bead, DeliveryStateRepairWait, "existing_auto_merge")
	}
	digest, err := hostedGateDigest(gate)
	if err != nil || digest != bead.Record.GateDigest {
		return r.recordDeliveryOutcome(ctx, state, bead, DeliveryStateRepairWait, "protection_changed")
	}
	arm := expectedMergeArm(bead, gate.ProtectionDigest)
	receipt := MergeArmReceipt{SchemaVersion: "merge-arm-receipt.v1", HandoffID: prepared.HandoffID, Epoch: prepared.Epoch, Arm: arm}
	const name = "merge-arm.json"
	if !state.exists(name) {
		if err := r.cut("before_merge_arm_intent"); err != nil {
			return Result{}, err
		}
		if err := state.writeImmutable(name, receipt); err != nil {
			return Result{}, err
		}
		if err := r.cut("after_merge_arm_intent"); err != nil {
			return Result{}, err
		}
		return Result{Status: "merge_arm_prepared"}, nil
	}
	if err := state.matches(name, receipt); err != nil {
		return Result{}, err
	}
	want := bead.Record
	want.State, want.Current, want.ArmID, want.AutoMergeEffectID, want.Revision = DeliveryStateMergeArmed, receiptRef("merge-arm", state), arm.ID, autoMergeEffectID(arm), bead.Record.Revision+1
	if err := r.cut("before_merge_arm_transition"); err != nil {
		return Result{}, err
	}
	result, err := r.storeDeliveryRecord(ctx, bead, want)
	if err != nil {
		return Result{}, err
	}
	if err := r.cut("after_merge_arm_transition"); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (r *Reducer) advanceMergeArm(ctx context.Context, state markerStore, bead DeliveryBead, prepared Prepared, providers HostedDeliveryProviders) (Result, error) {
	var gateReceipt GateReceipt
	if found, err := state.read("hosted-gate.json", &gateReceipt); err != nil || !found || gateReceipt.GateDigest != bead.Record.GateDigest {
		return Result{}, errors.New("merge_armed lacks exact hosted gate receipt")
	}
	arm := expectedMergeArm(bead, gateReceipt.Gate.ProtectionDigest)
	var receipt MergeArmReceipt
	if found, err := state.read("merge-arm.json", &receipt); err != nil || !found || receipt.Arm != arm || bead.Record.ArmID != arm.ID || bead.Record.AutoMergeEffectID != autoMergeEffectID(arm) {
		return Result{}, errors.New("merge_armed lacks exact immutable arm intent")
	}
	gate, err := providers.HostedGate(ctx, bead.Record.PR)
	if err != nil {
		return Result{}, err
	}
	qualified, reason, err := qualifyHostedGate(gate, bead)
	if err != nil {
		return r.recordDeliveryOutcome(ctx, state, bead, DeliveryStateRepairWait, "protection_ambiguity")
	}
	if !qualified {
		return Result{Status: "merge_armed", Reason: reason}, nil
	}
	if gate.AutoMergeEnabled && !hasOwnedAutoMergeAttempt(state.root, bead.Record) {
		return r.recordDeliveryOutcome(ctx, state, bead, DeliveryStateRepairWait, "existing_auto_merge")
	}
	if digest, err := hostedGateDigest(gate); err != nil || digest != bead.Record.GateDigest {
		return r.recordDeliveryOutcome(ctx, state, bead, DeliveryStateRepairWait, "protection_changed")
	}
	observation, err := providers.ObserveMerge(ctx, arm)
	if err != nil {
		return Result{}, err
	}
	switch observation.State {
	case "armed":
		if bead.Record.AutoMergeAttempt.Path == "" && state.exists("auto-merge-attempt.json") {
			want := bead.Record
			want.AutoMergeAttempt, want.Revision = receiptRef("auto-merge-attempt", state), bead.Record.Revision+1
			return r.storeDeliveryRecord(ctx, bead, want)
		}
		return Result{Status: "merge_armed"}, nil
	case "landed":
		return Result{Status: "merge_armed", Reason: "landing_evidence_pending"}, nil
	case "refused", "unknown":
		return r.recordMergeRefusal(ctx, state, bead, arm, observation)
	case "absent":
	default:
		return Result{}, errors.New("merge observation has unsupported state")
	}
	// A prior epoch's attempt is a handoff-wide fuse. Its receipt remains
	// authoritative across successor epochs, so an absent observation may wait
	// for landing or deadline but can never issue another mutation.
	if bead.Record.AutoMergeAttempt != (ReceiptRef{}) {
		return Result{Status: "merge_armed", Reason: "auto_merge_attempt_already_recorded"}, nil
	}
	effectID := autoMergeEffectID(arm)
	attempt := AutoMergeAttemptReceipt{SchemaVersion: "auto-merge-attempt-receipt.v1", HandoffID: prepared.HandoffID, Epoch: prepared.Epoch, EffectID: effectID, Arm: arm, Outcome: "prepared"}
	const attemptName = "auto-merge-attempt.json"
	if state.exists(attemptName) {
		if err := state.matches(attemptName, attempt); err != nil {
			return r.stallOnFuse(ctx, bead, state, "auto_merge_attempt_fuse_conflict")
		}
		return r.stallOnFuse(ctx, bead, state, "auto_merge_attempt_ambiguous")
	}
	if err := state.writeImmutable(attemptName, attempt); err != nil {
		return Result{}, err
	}
	if err := r.cut("before_auto_merge"); err != nil {
		return Result{}, err
	}
	armed, err := providers.ArmMerge(ctx, arm)
	if err != nil {
		return Result{}, err
	}
	if armed != arm {
		return Result{}, errors.New("auto-merge effect returned conflicting arm identity")
	}
	if err := r.cut("after_auto_merge"); err != nil {
		return Result{}, err
	}
	result := AutoMergeResultReceipt{SchemaVersion: "auto-merge-result-receipt.v1", HandoffID: prepared.HandoffID, Epoch: prepared.Epoch, EffectID: effectID, ArmID: arm.ID, Outcome: "accepted"}
	if err := state.writeImmutable("auto-merge-result.json", result); err != nil {
		return Result{}, err
	}
	return Result{Status: "auto_merge_sent", Effect: "forge.enable_auto_merge"}, nil
}

// Auto-merge observation is not protected-check authority. It is deliberately
// excluded from the qualified gate digest so the marker-first replay can
// observe the forge's armed state without treating its own effect as a gate
// mutation.
func hostedGateDigest(gate HostedGate) (string, error) {
	gate.AutoMergeEnabled = false
	return valueDigest(gate)
}

func (r *Reducer) recordMergeRefusal(ctx context.Context, state markerStore, bead DeliveryBead, arm MergeArm, observation MergeObservation) (Result, error) {
	receipt := MergeRefusalReceipt{SchemaVersion: "merge-refusal-receipt.v1", HandoffID: bead.Record.HandoffID, Epoch: bead.Record.Epoch.Number, Arm: arm, Observation: observation}
	const name = "merge-refusal.json"
	if !state.exists(name) {
		if err := state.writeImmutable(name, receipt); err != nil {
			return Result{}, err
		}
		return Result{Status: "merge_refusal_receipted", Reason: observation.State}, nil
	}
	if err := state.matches(name, receipt); err != nil {
		return Result{}, err
	}
	want := DeliveryStateRepairWait
	if observation.State == "unknown" {
		want = DeliveryStateStalled
	}
	return r.storeDeliveryTransition(ctx, bead, want, receiptRef("merge-refusal", state))
}

func (r *Reducer) stallOnFuse(ctx context.Context, bead DeliveryBead, global markerStore, reason string) (Result, error) {
	want := bead.Record
	want.State, want.Current, want.AutoMergeAttempt, want.Revision = DeliveryStateStalled, receiptRef("auto-merge-attempt", global), receiptRef("auto-merge-attempt", global), bead.Record.Revision+1
	result, err := r.storeDeliveryRecord(ctx, bead, want)
	result.Reason = reason
	return result, err
}

func expectedMergeArm(bead DeliveryBead, protectionDigest string) MergeArm {
	effectID := identifier("agentops.gc.delivery.auto-merge-effect.v1", bead.Record.HandoffID, bead.Record.Repository, bead.Record.PR.NodeID, bead.Record.Epoch.BaseRef, bead.Record.Epoch.Branch)
	id := identifier("agentops.gc.delivery.merge-arm.v1", effectID, strconv.Itoa(bead.Record.Epoch.Number), bead.Record.Epoch.BaseOID, bead.Record.Epoch.Head, bead.Record.GateDigest, "SQUASH")
	return MergeArm{ID: id, EffectID: effectID, PRID: bead.Record.PR.ID, Repository: bead.Record.Repository, NodeID: bead.Record.PR.NodeID, Number: bead.Record.PR.Number, Branch: bead.Record.Epoch.Branch, Head: bead.Record.Epoch.Head, BaseRef: bead.Record.Epoch.BaseRef, BaseOID: bead.Record.Epoch.BaseOID, ProtectionDigest: protectionDigest, GateDigest: bead.Record.GateDigest}
}

func autoMergeEffectID(arm MergeArm) string {
	return arm.EffectID
}

func qualifyHostedGate(gate HostedGate, bead DeliveryBead) (bool, string, error) {
	if gate.Repository != bead.Record.Repository || gate.BaseRef != bead.Record.Epoch.BaseRef || gate.BaseOID != bead.Record.Epoch.BaseOID || gate.Head != bead.Record.Epoch.Head {
		return false, "", errors.New("hosted gate does not bind exact repository/base/head")
	}
	if gate.PRState != "OPEN" || gate.Draft {
		return false, "", errors.New("hosted gate PR is not open and non-draft")
	}
	if !gate.Strict {
		return false, "", errors.New("hosted gate branch protection is not strict")
	}
	if !isHex(gate.ProtectionDigest, 64) || len(gate.RequiredChecks) == 0 {
		return false, "", errors.New("hosted gate lacks exact protected-check authority")
	}
	required, err := hostedCheckSet(gate.RequiredChecks, false)
	if err != nil {
		return false, "", err
	}
	observed, err := hostedCheckSet(gate.Checks, true)
	if err != nil {
		return false, "", err
	}
	for identity := range observed {
		if _, ok := required[identity]; !ok {
			return false, "", errors.New("hosted gate observed an unconfigured check identity")
		}
	}
	if len(observed) != len(required) {
		return false, "required_checks_pending", nil
	}
	if gate.MergeState != "CLEAN" {
		return false, "merge_state_" + gate.MergeState, nil
	}
	return true, "", nil
}

func hostedCheckSet(checks []HostedCheck, requireSuccess bool) (map[string]struct{}, error) {
	set := make(map[string]struct{}, len(checks))
	for _, check := range checks {
		appID, err := strconv.ParseInt(check.AppID, 10, 64)
		if err != nil || appID < 1 || check.Context == "" {
			return nil, errors.New("hosted check lacks positive app identity and context")
		}
		identity := check.AppID + "\x00" + check.Context
		if _, exists := set[identity]; exists {
			return nil, errors.New("hosted gate contains duplicate check identity")
		}
		if requireSuccess && (check.Status != "COMPLETED" || check.Conclusion != "SUCCESS") {
			return nil, errors.New("hosted gate included a non-successful required check")
		}
		set[identity] = struct{}{}
	}
	return set, nil
}
