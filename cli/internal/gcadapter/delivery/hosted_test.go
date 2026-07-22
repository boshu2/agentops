package delivery_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gcadapter/delivery"
)

type armedReplayProviders struct{ *delivery.FakeProviders }

func (armedReplayProviders) ObserveMerge(context.Context, delivery.MergeArm) (delivery.MergeObservation, error) {
	return delivery.MergeObservation{State: "armed"}, nil
}

func TestHostedAutoFlowWritesGateArmFuseAndExactLandingOnce(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	fake := delivery.NewFakeProviders(terminalFor(request))
	seen := map[string]bool{}
	for attempt := 0; attempt < 40; attempt++ {
		before := fake.MutationCount()
		result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
		if err != nil {
			t.Fatalf("step %d: %v", attempt, err)
		}
		if delta := fake.MutationCount() - before; delta > 1 {
			t.Fatalf("step %d performed %d provider mutations", attempt, delta)
		}
		seen[result.Status] = true
		if result.Status == "landed" {
			break
		}
	}
	for _, status := range []string{"pr_open", "ci_wait", "gate_receipted", "merge_eligible", "merge_arm_prepared", "merge_armed", "auto_merge_sent", "landing_receipted", "landed"} {
		if !seen[status] {
			t.Fatalf("flow omitted %q: %#v", status, seen)
		}
	}
	if fake.AutoMergeCount() != 1 || fake.PRCreateCount() != 1 {
		t.Fatalf("effects: PR=%d auto-merge=%d", fake.PRCreateCount(), fake.AutoMergeCount())
	}
	leaf, found := fake.Delivery(1)
	if !found || leaf.Record.State != delivery.DeliveryStateLanded || leaf.Record.GateDigest == "" || leaf.Record.ArmID == "" || leaf.Record.AutoMergeEffectID == "" || leaf.Record.AutoMergeAttempt.Path == "" || leaf.Record.LandingDigest == "" || leaf.Record.LandedSHA == "" {
		t.Fatalf("landed delivery record = %#v", leaf.Record)
	}
	epoch := filepath.Join(request.Root, "handoffs", leaf.Record.HandoffID, "epochs", "000001")
	for _, path := range []string{
		filepath.Join(epoch, "hosted-gate.json"), filepath.Join(epoch, "merge-arm.json"),
		filepath.Join(epoch, "auto-merge-result.json"), filepath.Join(epoch, "landing.json"),
		filepath.Join(epoch, "auto-merge-attempt.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing bounded delivery evidence %s: %v", path, err)
		}
	}
}

func TestHostedManualModeNeverMutatesMergeAndStillAcceptsExactExternalLanding(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	request.Target.Mode = "manual"
	fake := delivery.NewFakeProviders(terminalFor(request))
	leaf := reachBeadState(t, fake, request, delivery.DeliveryStateManualReview)
	if fake.AutoMergeCount() != 0 {
		t.Fatalf("manual mode sent %d merge effects", fake.AutoMergeCount())
	}
	landing := delivery.Landing{PRID: leaf.Record.PR.ID, Head: leaf.Record.Epoch.Head, SHA: strings.Repeat("c", 40), Tree: leaf.Record.Epoch.Tree, Parents: []string{leaf.Record.Epoch.BaseOID}}
	fake.SetLanding(landing)
	result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "landing_receipted" {
		t.Fatalf("manual landing receipt = %#v, %v", result, err)
	}
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "landed" || fake.AutoMergeCount() != 0 {
		t.Fatalf("manual landed = %#v, %v", result, err)
	}
}

func TestHostedCrashAfterEffectReconcilesLandingWithoutResend(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	fake := delivery.NewFakeProviders(terminalFor(request))
	reachBeadState(t, fake, request, delivery.DeliveryStateMergeArmed)
	result, err := delivery.NewReducer(fake, delivery.CrashAt("after_auto_merge")).Step(context.Background(), request)
	if !delivery.IsCrash(err) || result.Status != "" || fake.AutoMergeCount() != 1 {
		t.Fatalf("after-effect crash = %#v, %v, merges=%d", result, err, fake.AutoMergeCount())
	}
	for attempt := 0; attempt < 4; attempt++ {
		result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status == "landed" {
			break
		}
	}
	if result.Status != "landed" || fake.AutoMergeCount() != 1 {
		t.Fatalf("crash replay = %#v, merges=%d", result, fake.AutoMergeCount())
	}
}

func TestHostedCrashAfterFuseBeforeEffectStopsWithoutGuessingOrResend(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	fake := delivery.NewFakeProviders(terminalFor(request))
	leaf := reachBeadState(t, fake, request, delivery.DeliveryStateMergeArmed)
	_, err := delivery.NewReducer(fake, delivery.CrashAt("before_auto_merge")).Step(context.Background(), request)
	if !delivery.IsCrash(err) || fake.AutoMergeCount() != 0 {
		t.Fatalf("pre-effect crash = %v, merges=%d", err, fake.AutoMergeCount())
	}
	if _, err := os.Stat(filepath.Join(request.Root, "handoffs", leaf.Record.HandoffID, "epochs", "000001", "auto-merge-attempt.json")); err != nil {
		t.Fatalf("attempt fuse was not durable: %v", err)
	}
	result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "stalled" || result.Reason != "auto_merge_attempt_ambiguous" || fake.AutoMergeCount() != 0 {
		t.Fatalf("bounded ambiguity stop = %#v, %v, merges=%d", result, err, fake.AutoMergeCount())
	}
}

func TestHostedRejectsExternallyEnabledAutoMergeWithoutLocalFence(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	fake := delivery.NewFakeProviders(terminalFor(request))
	leaf := reachBeadState(t, fake, request, delivery.DeliveryStateCIWait)
	required := delivery.HostedCheck{AppID: "1234", Context: "required / test"}
	passed := required
	passed.Status, passed.Conclusion = "COMPLETED", "SUCCESS"
	fake.SetHostedGate(delivery.HostedGate{Repository: request.Target.Repository, BaseRef: leaf.Record.Epoch.BaseRef, BaseOID: leaf.Record.Epoch.BaseOID, Head: leaf.Record.Epoch.Head, PRState: "OPEN", MergeState: "CLEAN", AutoMergeEnabled: true, Strict: true, ProtectionDigest: strings.Repeat("b", 64), RequiredChecks: []delivery.HostedCheck{required}, Checks: []delivery.HostedCheck{passed}})

	before := fake.MutationCount()
	result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "delivery_outcome_receipted" || result.Reason != "existing_auto_merge" || fake.MutationCount() != before {
		t.Fatalf("external auto-merge was eligible: %#v, %v, mutations=%d", result, err, fake.MutationCount()-before)
	}
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "repair_wait" || result.Reason != "existing_auto_merge" || fake.AutoMergeCount() != 0 {
		t.Fatalf("external auto-merge did not stop: %#v, %v, sends=%d", result, err, fake.AutoMergeCount())
	}
}

func TestHostedReplaysOwnedAutoMergeFenceWithoutResend(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	fake := delivery.NewFakeProviders(terminalFor(request))
	leaf := reachBeadState(t, fake, request, delivery.DeliveryStateMergeArmed)
	if _, err := delivery.NewReducer(fake, delivery.CrashAt("before_auto_merge")).Step(context.Background(), request); !delivery.IsCrash(err) {
		t.Fatalf("fence crash = %v", err)
	}
	required := delivery.HostedCheck{AppID: "1234", Context: "required / test"}
	passed := required
	passed.Status, passed.Conclusion = "COMPLETED", "SUCCESS"
	fake.SetHostedGate(delivery.HostedGate{Repository: request.Target.Repository, BaseRef: leaf.Record.Epoch.BaseRef, BaseOID: leaf.Record.Epoch.BaseOID, Head: leaf.Record.Epoch.Head, PRState: "OPEN", MergeState: "CLEAN", AutoMergeEnabled: true, Strict: true, ProtectionDigest: strings.Repeat("b", 64), RequiredChecks: []delivery.HostedCheck{required}, Checks: []delivery.HostedCheck{passed}})
	result, err := delivery.NewReducer(armedReplayProviders{fake}, nil).Step(context.Background(), request)
	if err != nil || result.Status != "merge_armed" || fake.AutoMergeCount() != 0 {
		t.Fatalf("owned-fence replay = %#v, %v, sends=%d", result, err, fake.AutoMergeCount())
	}
}

func TestHostedCurrentLandingReceiptRebindsCanonicalRecord(t *testing.T) {
	for name, mutate := range map[string]func(*delivery.DeliveryRecord){
		"handoff":        func(record *delivery.DeliveryRecord) { record.HandoffID = strings.Repeat("f", 64) },
		"epoch":          func(record *delivery.DeliveryRecord) { record.Epoch.Number = 2 },
		"pr":             func(record *delivery.DeliveryRecord) { record.PR.ID = "pr-other" },
		"head":           func(record *delivery.DeliveryRecord) { record.Epoch.Head = strings.Repeat("d", 40) },
		"tree":           func(record *delivery.DeliveryRecord) { record.Epoch.Tree = strings.Repeat("e", 40) },
		"landing_digest": func(record *delivery.DeliveryRecord) { record.LandingDigest = strings.Repeat("d", 64) },
		"landed_sha":     func(record *delivery.DeliveryRecord) { record.LandedSHA = strings.Repeat("d", 40) },
	} {
		t.Run(name, func(t *testing.T) {
			request := requestForTest(t, t.TempDir())
			request.Target.Mode = "manual"
			fake := delivery.NewFakeProviders(terminalFor(request))
			leaf := reachBeadState(t, fake, request, delivery.DeliveryStateManualReview)
			fake.SetLanding(delivery.Landing{PRID: leaf.Record.PR.ID, Head: leaf.Record.Epoch.Head, SHA: strings.Repeat("c", 40), Tree: leaf.Record.Epoch.Tree, Parents: []string{leaf.Record.Epoch.BaseOID}})
			for _, status := range []string{"landing_receipted", "landed"} {
				result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
				if err != nil || result.Status != status {
					t.Fatalf("reach %s = %#v, %v", status, result, err)
				}
			}
			current, _ := fake.Delivery(1)
			mutate(&current.Record)
			fake.PutDelivery(current)
			before := fake.MutationCount()
			if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err == nil {
				t.Fatalf("tampered landing record returned %#v", result)
			}
			if fake.MutationCount() != before {
				t.Fatal("tampered landing record mutated provider")
			}
		})
	}
}

func TestHostedAttemptFuseAndEffectIdentitySurviveMovingEpoch(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	fake := delivery.NewFakeProviders(terminalFor(request))
	predecessor := reachBeadState(t, fake, request, delivery.DeliveryStateMergeArmed)
	effectID, armID := predecessor.Record.AutoMergeEffectID, predecessor.Record.ArmID
	selected := request
	selected.Target.DeliveryBeadID = predecessor.ID
	if _, err := delivery.NewReducer(fake, delivery.CrashAt("before_auto_merge")).Step(context.Background(), selected); !delivery.IsCrash(err) {
		t.Fatalf("attempt fuse crash = %v", err)
	}
	if fake.AutoMergeCount() != 0 {
		t.Fatalf("pre-effect crash sent %d mutations", fake.AutoMergeCount())
	}

	child := createReadySuccessor(t, fake, request, predecessor, strings.Repeat("c", 40))
	if child.Record.AutoMergeEffectID != effectID || child.Record.AutoMergeAttempt.Path == "" || !strings.Contains(child.Record.AutoMergeAttempt.Path, "/epochs/000001/auto-merge-attempt.json") {
		t.Fatalf("successor lost handoff-wide fuse: parent=%#v child=%#v", predecessor.Record, child.Record)
	}
	childSelected := request
	childSelected.Target.DeliveryBeadID = child.ID
	child = reachSelectedState(t, fake, childSelected, 2, delivery.DeliveryStateMergeArmed)
	if child.Record.AutoMergeEffectID != effectID || child.Record.ArmID == armID {
		t.Fatalf("successor effect/arm identity = effect %q arm %q; want stable effect %q and new arm", child.Record.AutoMergeEffectID, child.Record.ArmID, effectID)
	}
	required := delivery.HostedCheck{AppID: "1234", Context: "required / test"}
	passed := required
	passed.Status, passed.Conclusion = "COMPLETED", "SUCCESS"
	fake.SetHostedGate(delivery.HostedGate{Repository: request.Target.Repository, BaseRef: child.Record.Epoch.BaseRef, BaseOID: child.Record.Epoch.BaseOID, Head: child.Record.Epoch.Head, PRState: "OPEN", MergeState: "CLEAN", AutoMergeEnabled: true, Strict: true, ProtectionDigest: strings.Repeat("b", 64), RequiredChecks: []delivery.HostedCheck{required}, Checks: []delivery.HostedCheck{passed}})
	result, err := delivery.NewReducer(fake, nil).Step(context.Background(), childSelected)
	if err != nil || result.Status != "merge_armed" || result.Reason != "auto_merge_attempt_already_recorded" || fake.AutoMergeCount() != 0 {
		t.Fatalf("successor fuse replay = %#v, %v, merges=%d", result, err, fake.AutoMergeCount())
	}
}

func TestHostedGateRequiresStrictAppQualifiedExactSuccessfulSet(t *testing.T) {
	for _, scenario := range []string{"non_strict", "zero_app", "duplicate", "unexpected", "pending", "wrong_head"} {
		t.Run(scenario, func(t *testing.T) {
			request := requestForTest(t, t.TempDir())
			request.Target.Mode = "manual"
			fake := delivery.NewFakeProviders(terminalFor(request))
			leaf := reachBeadState(t, fake, request, delivery.DeliveryStateCIWait)
			required := delivery.HostedCheck{AppID: "1234", Context: "required / test"}
			passed := required
			passed.Status, passed.Conclusion = "COMPLETED", "SUCCESS"
			gate := delivery.HostedGate{Repository: request.Target.Repository, BaseRef: leaf.Record.Epoch.BaseRef, BaseOID: leaf.Record.Epoch.BaseOID, Head: leaf.Record.Epoch.Head, PRState: "OPEN", MergeState: "CLEAN", Strict: true, ProtectionDigest: strings.Repeat("b", 64), RequiredChecks: []delivery.HostedCheck{required}, Checks: []delivery.HostedCheck{passed}}
			switch scenario {
			case "non_strict":
				gate.Strict = false
			case "zero_app":
				gate.RequiredChecks[0].AppID, gate.Checks[0].AppID = "0", "0"
			case "duplicate":
				gate.Checks = append(gate.Checks, gate.Checks[0])
			case "unexpected":
				gate.Checks = append(gate.Checks, delivery.HostedCheck{AppID: "999", Context: "other", Status: "COMPLETED", Conclusion: "SUCCESS"})
			case "pending":
				gate.Checks = nil
			case "wrong_head":
				gate.Head = strings.Repeat("e", 40)
			}
			fake.SetHostedGate(gate)
			before := fake.MutationCount()
			result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
			if scenario == "pending" {
				if err != nil || result.Status != "ci_wait" || result.Reason != "required_checks_pending" {
					t.Fatalf("pending gate = %#v, %v", result, err)
				}
			} else {
				if err != nil || result.Status != "delivery_outcome_receipted" || result.Reason != "protection_ambiguity" {
					t.Fatalf("%s gate outcome = %#v, %v", scenario, result, err)
				}
				result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
				if err != nil || result.Status != "repair_wait" || result.Reason != "protection_ambiguity" {
					t.Fatalf("%s gate hold = %#v, %v", scenario, result, err)
				}
			}
			wantMutations := before
			if scenario != "pending" {
				wantMutations++
			}
			if fake.MutationCount() != wantMutations || fake.AutoMergeCount() != 0 {
				t.Fatalf("%s gate mutated delivery", scenario)
			}
		})
	}
}

func TestHostedRejectsNonExactLandingWithoutTransition(t *testing.T) {
	for _, scenario := range []string{"head", "tree", "parent", "sha"} {
		t.Run(scenario, func(t *testing.T) {
			request := requestForTest(t, t.TempDir())
			request.Target.Mode = "manual"
			fake := delivery.NewFakeProviders(terminalFor(request))
			leaf := reachBeadState(t, fake, request, delivery.DeliveryStateManualReview)
			landing := delivery.Landing{PRID: leaf.Record.PR.ID, Head: leaf.Record.Epoch.Head, SHA: strings.Repeat("c", 40), Tree: leaf.Record.Epoch.Tree, Parents: []string{leaf.Record.Epoch.BaseOID}}
			switch scenario {
			case "head":
				landing.Head = strings.Repeat("e", 40)
			case "tree":
				landing.Tree = strings.Repeat("e", 40)
			case "parent":
				landing.Parents = []string{strings.Repeat("e", 40)}
			case "sha":
				landing.SHA = "not-an-oid"
			}
			fake.SetLanding(landing)
			before := fake.MutationCount()
			if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err == nil {
				t.Fatalf("%s landing advanced as %#v", scenario, result)
			}
			if fake.MutationCount() != before {
				t.Fatalf("%s landing mutated Beads", scenario)
			}
		})
	}
}

func TestHostedDeadlineIsDurableButExactLandingWinsTheSameObservation(t *testing.T) {
	t.Run("expired_wait", func(t *testing.T) {
		request := requestForTest(t, t.TempDir())
		fake := delivery.NewFakeProviders(terminalFor(request))
		reachBeadState(t, fake, request, delivery.DeliveryStateCIWait)
		request.Target.ObservedAt = "2026-07-23T00:00:00Z"
		before := fake.MutationCount()
		result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
		if err != nil || result.Status != "deadline_observed" || fake.MutationCount() != before {
			t.Fatalf("deadline receipt = %#v, %v", result, err)
		}
		result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
		if err != nil || result.Status != "stalled" || result.Reason != "deadline_expired" || fake.MutationCount() != before+1 || fake.AutoMergeCount() != 0 {
			t.Fatalf("deadline transition = %#v, %v", result, err)
		}
	})
	t.Run("landing_first", func(t *testing.T) {
		request := requestForTest(t, t.TempDir())
		request.Target.Mode = "manual"
		fake := delivery.NewFakeProviders(terminalFor(request))
		leaf := reachBeadState(t, fake, request, delivery.DeliveryStateManualReview)
		fake.SetLanding(delivery.Landing{PRID: leaf.Record.PR.ID, Head: leaf.Record.Epoch.Head, SHA: strings.Repeat("c", 40), Tree: leaf.Record.Epoch.Tree, Parents: []string{leaf.Record.Epoch.BaseOID}})
		request.Target.ObservedAt = "2026-07-23T00:00:00Z"
		result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
		if err != nil || result.Status != "landing_receipted" {
			t.Fatalf("expired landing receipt = %#v, %v", result, err)
		}
		result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
		if err != nil || result.Status != "landed" {
			t.Fatalf("expired exact landing = %#v, %v", result, err)
		}
	})
}

func reachBeadState(t *testing.T, fake *delivery.FakeProviders, request delivery.Request, want delivery.DeliveryState) delivery.DeliveryBead {
	t.Helper()
	for attempt := 0; attempt < 40; attempt++ {
		if leaf, found := fake.Delivery(1); found && leaf.Record.State == want {
			return leaf
		}
		if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err != nil {
			t.Fatalf("reach %s at step %d: %v", want, attempt, err)
		}
	}
	t.Fatalf("did not reach Beads state %s", want)
	return delivery.DeliveryBead{}
}
