package delivery_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gcadapter/delivery"
)

func TestReducerKillAnywhereConvergesToOneBeadAndOnePROutsideAO(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	certificate := certificateFor("semantic-42")
	certificateBytes, err := json.Marshal(certificate)
	if err != nil {
		t.Fatal(err)
	}
	certificateDigest := digest(certificateBytes)
	request := delivery.Request{
		Root:              root,
		Certificate:       certificate,
		CertificateBytes:  certificateBytes,
		CertificateDigest: certificateDigest,
		Target:            delivery.Target{SemanticBeadID: "semantic-42", SemanticTerminalRef: "beads:semantic-42#terminal", RigID: "rig-a", Repository: "boshu2/agentops", Remote: "origin", Epoch: 1, Mode: "auto", Deadline: "2026-07-22T00:00:00Z", PreparedAt: "2026-07-21T00:00:00Z", CommittedAt: "2026-07-21T00:00:01Z", BaseRef: "main", BaseOID: strings.Repeat("d", 40)},
	}
	providers := delivery.NewFakeProviders(delivery.Terminal{BeadID: "semantic-42", Ref: "beads:semantic-42#terminal", Verdict: "PASS", CertificateDigest: certificateDigest})
	cuts := delivery.AllCrashCuts()
	for _, cut := range cuts {
		t.Run(cut, func(t *testing.T) {
			trialRoot := filepath.Join(root, cut)
			if err := os.MkdirAll(trialRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			trial := request
			trial.Root = trialRoot
			fake := providers.Clone()
			crashed, terminal := false, false
			for i := 0; i < 48; i++ {
				before := fake.MutationCount()
				var crash delivery.Crash
				if !crashed {
					crash = delivery.CrashAt(cut)
				}
				result, runErr := delivery.NewReducer(fake, crash).Step(context.Background(), trial)
				if got := fake.MutationCount() - before; got > 1 {
					t.Fatalf("step %d performed %d provider mutations", i, got)
				}
				if delivery.IsCrash(runErr) {
					crashed = true
					continue
				}
				if runErr != nil {
					t.Fatalf("step %d: %v", i, runErr)
				}
				if result.Status == "landed" || (cut == "before_auto_merge" && result.Status == "stalled") {
					terminal = true
					break
				}
			}
			if !crashed || !terminal {
				t.Fatalf("cut %q: crashed=%t terminal=%t", cut, crashed, terminal)
			}
			if got := fake.DeliveryCount(); got != 1 {
				t.Fatalf("delivery beads = %d, want 1", got)
			}
			if got := fake.BranchCount(); got != 1 {
				t.Fatalf("branches = %d, want 1", got)
			}
			if got := fake.PRCount(); got != 1 {
				t.Fatalf("prs = %d, want 1", got)
			}
			if !fake.OnlyDeliveryWasInitiallyNonRoutable() {
				t.Fatal("delivery bead was not non-routable before publication")
			}
			handoff := handoffID(t, trialRoot)
			epoch := filepath.Join("handoffs", handoff, "epochs", "000001")
			for _, artifact := range []string{filepath.Join("handoffs", handoff, "prepared.json"), filepath.Join("handoffs", handoff, "payload.json"), filepath.Join("handoffs", handoff, "committed.json"), filepath.Join(epoch, "epoch.json"), filepath.Join(epoch, "branch.json"), filepath.Join(epoch, "pr-intent.json"), filepath.Join(epoch, "pr-open.json"), filepath.Join(epoch, "hosted-gate.json"), filepath.Join(epoch, "merge-arm.json")} {
				bytes, readErr := os.ReadFile(filepath.Join(trialRoot, artifact))
				if readErr != nil {
					t.Fatalf("read %s: %v", artifact, readErr)
				}
				var object map[string]any
				if unmarshalErr := json.Unmarshal(bytes, &object); unmarshalErr != nil {
					t.Fatalf("decode %s: %v", artifact, unmarshalErr)
				}
				if _, ok := object["schema_version"]; !ok {
					t.Fatalf("%s has no schema_version", artifact)
				}
				for key := range object {
					if key != strings.ToLower(key) {
						t.Fatalf("%s emitted non-schema field %q", artifact, key)
					}
				}
			}
		})
	}

	for _, forbidden := range []string{"cli/cmd/ao", "cli/internal/ports"} {
		if _, err := os.Stat(filepath.Join("..", "..", "..", "..", forbidden)); err == nil {
			// The reducer package must not import either forbidden surface.
			bytes, readErr := os.ReadFile("reducer.go")
			if readErr != nil {
				t.Fatal(readErr)
			}
			if strings.Contains(string(bytes), forbidden) {
				t.Fatalf("reducer imports forbidden surface %q", forbidden)
			}
		}
	}
}

func TestImplicitStepSelectsBeadsLeafBeforeObservingMutableBase(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	fake := delivery.NewFakeProviders(terminalFor(request))

	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	calls := fake.Calls()
	if len(calls) < 2 || calls[0] != "find_delivery" || calls[1] != "observe_base" {
		t.Fatalf("provider calls = %v, want Beads selection before base observation", calls)
	}
}

func TestReducerRejectsCertificateTargetAndRouteMutations(t *testing.T) {
	certificate := certificateFor("semantic-42")
	certificateBytes, err := json.Marshal(certificate)
	if err != nil {
		t.Fatal(err)
	}
	base := delivery.Request{Root: t.TempDir(), Certificate: certificate, CertificateBytes: certificateBytes, CertificateDigest: digest(certificateBytes), Target: delivery.Target{SemanticBeadID: "semantic-42", SemanticTerminalRef: "beads:semantic-42#terminal", RigID: "rig-a", Repository: "boshu2/agentops", Remote: "origin", Epoch: 1, Mode: "auto", Deadline: "2026-07-22T00:00:00Z", PreparedAt: "2026-07-21T00:00:00Z", CommittedAt: "2026-07-21T00:00:01Z", BaseRef: "main", BaseOID: strings.Repeat("d", 40)}}
	terminal := delivery.Terminal{BeadID: "semantic-42", Ref: "beads:semantic-42#terminal", Verdict: "PASS", CertificateDigest: base.CertificateDigest}
	for name, mutate := range map[string]func(*delivery.Request){
		"bytes_extra_field": func(r *delivery.Request) {
			r.CertificateBytes = append(append([]byte{}, r.CertificateBytes[:len(r.CertificateBytes)-1]...), []byte(`,"extra":true}`)...)
			r.CertificateDigest = digest(r.CertificateBytes)
		},
		"bytes_struct_mismatch": func(r *delivery.Request) { r.Certificate.SemanticBeadID = "other" },
		"same_author_validator_context": func(r *delivery.Request) {
			r.Certificate.Attestations.Validator.ContextID = r.Certificate.Attestations.Author.ContextID
			bytes, _ := json.Marshal(r.Certificate)
			r.CertificateBytes = bytes
			r.CertificateDigest = digest(bytes)
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := base
			request.Root = t.TempDir()
			mutate(&request)
			if _, got := delivery.NewReducer(delivery.NewFakeProviders(terminal), nil).Step(context.Background(), request); got == nil {
				t.Fatal("mutation was accepted")
			}
		})
	}
	for name, mutate := range map[string]func(*delivery.Target){
		"rig": func(target *delivery.Target) { target.RigID = "rig-b" }, "repository": func(target *delivery.Target) { target.Repository = "other/repo" }, "remote": func(target *delivery.Target) { target.Remote = "upstream" }, "base_ref": func(target *delivery.Target) { target.BaseRef = "release" }, "base_oid": func(target *delivery.Target) { target.BaseOID = strings.Repeat("e", 40) }, "mode": func(target *delivery.Target) { target.Mode = "manual" },
	} {
		t.Run("handoff_binds_"+name, func(t *testing.T) {
			left := base
			left.Root = t.TempDir()
			right := base
			right.Root = t.TempDir()
			mutate(&right.Target)
			leftFake, rightFake := delivery.NewFakeProviders(terminal), delivery.NewFakeProviders(terminal)
			if _, err := delivery.NewReducer(leftFake, nil).Step(context.Background(), left); err != nil {
				t.Fatal(err)
			}
			if _, err := delivery.NewReducer(rightFake, nil).Step(context.Background(), right); err != nil {
				t.Fatal(err)
			}
			equal := handoffID(t, left.Root) == handoffID(t, right.Root)
			if (name == "base_oid") != equal {
				t.Fatalf("unexpected moving-main handoff identity behavior for %s", name)
			}
		})
	}
	request := base
	request.Root = t.TempDir()
	fake := delivery.NewFakeProviders(terminal)
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	var prepared struct {
		DeliveryBeadID string `json:"expected_delivery_bead_id"`
		ExternalRef    string `json:"expected_external_ref"`
	}
	bytes, err := os.ReadFile(filepath.Join(request.Root, "handoffs", handoffID(t, request.Root), "prepared.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(bytes, &prepared); err != nil {
		t.Fatal(err)
	}
	fake.PutDelivery(delivery.DeliveryBead{ID: prepared.DeliveryBeadID, ExternalRef: prepared.ExternalRef, Route: "wrong.pool"})
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err == nil {
		t.Fatal("mismatched delivery route was accepted")
	}
	for i := 0; i < 12; i++ {
		result, runErr := delivery.NewReducer(fake, nil).Step(context.Background(), request)
		if runErr != nil {
			break
		}
		if result.Status == "committed" {
			break
		}
	}
	// A fresh fixture keeps the later identity-collision checks independent of
	// the deliberately bad route above.
	fixture := delivery.NewFakeProviders(terminal)
	reachedPublished := false
	for i := 0; i < 12; i++ {
		result, runErr := delivery.NewReducer(fixture, nil).Step(context.Background(), request)
		if runErr != nil {
			t.Fatal(runErr)
		}
		if result.Status == "route_published" {
			reachedPublished = true
			break
		}
	}
	if !reachedPublished {
		t.Fatal("did not reach committed route publication before branch collision")
	}
	handoff := handoffID(t, request.Root)
	branchName := "gc/delivery/" + handoff[:20]
	fixture.PutBranch(delivery.Branch{Name: branchName, BaseRef: "evil", BaseOID: strings.Repeat("d", 40), Head: strings.Repeat("a", 40)})
	if result, err := delivery.NewReducer(fixture, nil).Step(context.Background(), request); err != nil || result.Status != "epoch_composed" {
		t.Fatalf("branch collision compose = %#v, %v", result, err)
	}
	if result, err := delivery.NewReducer(fixture, nil).Step(context.Background(), request); err == nil {
		t.Fatalf("branch collision was accepted at status %s for %s", result.Status, branchName)
	}

}

func TestOfflineFixtureColdReplayConverges(t *testing.T) {
	certificate := certificateFor("semantic-42")
	bytes, err := json.Marshal(certificate)
	if err != nil {
		t.Fatal(err)
	}
	request := delivery.Request{Root: filepath.Join(t.TempDir(), "evidence"), Certificate: certificate, CertificateBytes: bytes, CertificateDigest: digest(bytes), Target: delivery.Target{SemanticBeadID: "semantic-42", SemanticTerminalRef: "beads:semantic-42#terminal", RigID: "rig-a", Repository: "boshu2/agentops", Remote: "origin", Epoch: 1, Mode: "auto", Deadline: "2026-07-22T00:00:00Z", PreparedAt: "2026-07-21T00:00:00Z", CommittedAt: "2026-07-21T00:00:01Z", BaseRef: "main", BaseOID: strings.Repeat("d", 40)}}
	fixture := filepath.Join(t.TempDir(), "offline-fixture.json")
	terminal := delivery.Terminal{BeadID: request.Target.SemanticBeadID, Ref: request.Target.SemanticTerminalRef, Verdict: "PASS", CertificateDigest: request.CertificateDigest}
	for i := 0; i < 40; i++ {
		providers, openErr := delivery.OpenFixtureProviders(fixture, terminal)
		if openErr != nil {
			t.Fatal(openErr)
		}
		result, stepErr := delivery.NewReducer(providers, nil).Step(context.Background(), request)
		if stepErr != nil {
			t.Fatal(stepErr)
		}
		if result.Status == "landed" {
			return
		}
	}
	t.Fatal("fresh fixture providers did not converge")
}

// A manual delivery never arms or performs a merge.  Once the exact PR head
// has a nonempty app-qualified hosted gate, the one-step reducer exposes the
// operator-visible manual-review state and exits without another mutation.
func TestReducerManualDeliveryStopsAtManualReview(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	request.Target.Mode = "manual"
	terminal := terminalFor(request)
	providers := delivery.NewFakeProviders(terminal)

	for i := 0; i < 20; i++ {
		result, err := delivery.NewReducer(providers, nil).Step(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status == "manual_review" {
			if result.Effect != "beads.transition" {
				t.Fatalf("manual review performed an effect: %#v", result)
			}
			if providers.MergeCount() != 0 {
				t.Fatal("manual delivery attempted a merge")
			}
			return
		}
	}
	t.Fatal("manual delivery did not reach manual_review")
}

func TestReducerAutoDeliveryArmsMergesAndVerifiesExactLanding(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	providers := delivery.NewFakeProviders(terminalFor(request))
	seen := map[string]int{}
	for i := 0; i < 24; i++ {
		before := providers.MutationCount()
		result, err := delivery.NewReducer(providers, nil).Step(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if got := providers.MutationCount() - before; got > 1 {
			t.Fatalf("step %d performed %d external mutations", i, got)
		}
		seen[result.Status]++
		if result.Status == "landed" {
			if providers.MergeCount() != 1 {
				t.Fatalf("merges = %d, want exactly one", providers.MergeCount())
			}
			return
		}
	}
	t.Fatalf("did not land; states: %#v", seen)
}

func TestReducerDeadlineStallsBeforeAnyEffect(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	request.Target.ObservedAt = "2026-07-23T00:00:00Z"
	providers := delivery.NewFakeProviders(terminalFor(request))
	result, err := delivery.NewReducer(providers, nil).Step(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "stalled" || result.Reason != "deadline_expired_before_delivery" {
		t.Fatalf("result = %#v, want structured stalled deadline", result)
	}
	if providers.MutationCount() != 0 {
		t.Fatal("expired delivery performed an effect")
	}
}

func TestReducerRejectsMissingFallbackFieldsBeforePreparedMarker(t *testing.T) {
	for _, role := range []string{"author", "validator"} {
		for _, field := range []string{"fallback", "allowed", "used", "reason"} {
			t.Run(role+"_"+field, func(t *testing.T) {
				bytes := missingFallbackField(t, certificateFor("semantic-42"), role, field)
				var certificate delivery.AdmissionCertificate
				if err := json.Unmarshal(bytes, &certificate); err != nil {
					t.Fatal(err)
				}
				root := t.TempDir()
				request := delivery.Request{Root: root, Certificate: certificate, CertificateBytes: bytes, CertificateDigest: digest(bytes), Target: delivery.Target{SemanticBeadID: "semantic-42", SemanticTerminalRef: "beads:semantic-42#terminal", RigID: "rig-a", Repository: "boshu2/agentops", Remote: "origin", Epoch: 1, Mode: "auto", Deadline: "2026-07-22T00:00:00Z", PreparedAt: "2026-07-21T00:00:00Z", CommittedAt: "2026-07-21T00:00:01Z", BaseRef: "main", BaseOID: strings.Repeat("d", 40)}}
				terminal := delivery.Terminal{BeadID: "semantic-42", Ref: "beads:semantic-42#terminal", Verdict: "PASS", CertificateDigest: request.CertificateDigest}
				if _, err := delivery.NewReducer(delivery.NewFakeProviders(terminal), nil).Step(context.Background(), request); err == nil {
					t.Fatal("missing required fallback field was accepted")
				}
				if _, err := os.Stat(filepath.Join(root, "handoff-prepared.json")); !os.IsNotExist(err) {
					t.Fatalf("prepared marker exists after rejected fallback: %v", err)
				}
			})
		}
	}
}

func missingFallbackField(t *testing.T, certificate delivery.AdmissionCertificate, role, field string) []byte {
	t.Helper()
	bytes, err := json.Marshal(certificate)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(bytes, &document); err != nil {
		t.Fatal(err)
	}
	runtime := document["attestations"].(map[string]any)[role].(map[string]any)
	if field == "fallback" {
		delete(runtime, field)
	} else {
		delete(runtime["fallback"].(map[string]any), field)
	}
	bytes, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}

func TestReducerRejectsCorruptedImmutableMarkers(t *testing.T) {
	for _, marker := range []string{
		"certificate.json", "prepared.json", "payload.json", "committed.json",
	} {
		t.Run(marker, func(t *testing.T) {
			certificate := certificateFor("semantic-42")
			certificateBytes, err := json.Marshal(certificate)
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			request := delivery.Request{Root: root, Certificate: certificate, CertificateBytes: certificateBytes, CertificateDigest: digest(certificateBytes), Target: delivery.Target{SemanticBeadID: "semantic-42", SemanticTerminalRef: "beads:semantic-42#terminal", RigID: "rig-a", Repository: "boshu2/agentops", Remote: "origin", Epoch: 1, Mode: "auto", Deadline: "2026-07-22T00:00:00Z", PreparedAt: "2026-07-21T00:00:00Z", CommittedAt: "2026-07-21T00:00:01Z", BaseRef: "main", BaseOID: strings.Repeat("d", 40)}}
			terminal := delivery.Terminal{BeadID: "semantic-42", Ref: "beads:semantic-42#terminal", Verdict: "PASS", CertificateDigest: request.CertificateDigest}
			fake := delivery.NewFakeProviders(terminal)
			if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			markerPath := filepath.Join(root, "handoffs", handoffID(t, root), marker)
			for i := 0; i < 16; i++ {
				if _, err := os.Stat(markerPath); err == nil {
					break
				}
				if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := os.Stat(markerPath); err != nil {
				t.Fatalf("did not reach marker %s: %v", marker, err)
			}
			if err := os.WriteFile(markerPath, []byte(`{"corrupted":true}`+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err == nil {
				t.Fatalf("corrupted %s advanced as %q", marker, result.Status)
			}
		})
	}
}

func TestReducerRejectsUppercaseIdentifiersBeforePreparedMarker(t *testing.T) {
	for name, mutate := range map[string]func(*delivery.AdmissionCertificate, *delivery.Target){
		"certificate_intent_digest": func(certificate *delivery.AdmissionCertificate, _ *delivery.Target) {
			certificate.IntentDigest = strings.ToUpper(certificate.IntentDigest)
		},
		"target_base_oid": func(_ *delivery.AdmissionCertificate, target *delivery.Target) {
			target.BaseOID = strings.ToUpper(target.BaseOID)
		},
	} {
		t.Run(name, func(t *testing.T) {
			certificate := certificateFor("semantic-42")
			target := targetForTest()
			mutate(&certificate, &target)
			bytes, err := json.Marshal(certificate)
			if err != nil {
				t.Fatal(err)
			}
			request := delivery.Request{Root: t.TempDir(), Certificate: certificate, CertificateBytes: bytes, CertificateDigest: digest(bytes), Target: target}
			if _, err := delivery.NewReducer(delivery.NewFakeProviders(terminalFor(request)), nil).Step(context.Background(), request); err == nil {
				t.Fatal("uppercase identifier was accepted")
			}
			if _, err := os.Stat(filepath.Join(request.Root, "handoff-prepared.json")); !os.IsNotExist(err) {
				t.Fatalf("prepared marker exists after rejected identifier: %v", err)
			}
		})
	}
}

func TestReducerRejectsReceiptSchemaVersionMutation(t *testing.T) {
	for _, phase := range []struct {
		name, status string
	}{
		{"branch", "branch_observed"},
		{"pr", "pr_receipted"},
	} {
		for _, schema := range []string{"missing", "wrong"} {
			t.Run(phase.name+"_"+schema, func(t *testing.T) {
				request := requestForTest(t, t.TempDir())
				fake := delivery.NewFakeProviders(terminalFor(request))
				reachStatus(t, fake, request, phase.status)
				leaf, found := fake.Delivery(1)
				if !found {
					t.Fatal("delivery leaf missing")
				}
				path := filepath.Join(request.Root, "handoffs", leaf.Record.HandoffID, "epochs", fmt.Sprintf("%06d", leaf.Record.Epoch.Number), phase.name+".json")
				if phase.name == "pr" {
					path = filepath.Join(filepath.Dir(path), "pr-open.json")
				}
				mutateReceiptSchema(t, path, schema)
				before := fake.MutationCount()
				if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err == nil {
					t.Fatalf("corrupted %s receipt advanced as %q", phase.name, result.Status)
				}
				if got := fake.MutationCount(); got != before {
					t.Fatalf("corrupted %s receipt mutated provider: before=%d after=%d", phase.name, before, got)
				}
			})
		}
	}
}

func TestReducerCreatesDeterministicEpochSuccessorOnlyAfterCleanBaseMovement(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	fake := delivery.NewFakeProviders(terminalFor(request))
	for i := 0; i < 6; i++ {
		if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
	first, ok := fake.Delivery(1)
	if !ok || first.Record.State != delivery.DeliveryStatePreparing || first.Route != "agentops.delivery" {
		t.Fatalf("predecessor not routed preparing: %#v", first)
	}
	first.Record.Epoch.Head, first.Record.Epoch.Tree = strings.Repeat("a", 40), strings.Repeat("b", 40)
	first.Record.PR = delivery.PullRequest{ID: "pr-stable", EffectID: strings.Repeat("9", 64), Repository: request.Target.Repository, BaseRef: request.Target.BaseRef, Branch: first.Record.Epoch.Branch, NodeID: "PR_node", Number: "42", URL: "https://example.invalid/pr/42"}
	fake.PutDelivery(first)
	selected := request
	selected.Target.DeliveryBeadID = first.ID
	newBase := strings.Repeat("c", 40)
	fake.SetObservedBase(newBase)
	fake.AllowBaseDescendant(newBase, request.Target.BaseOID)
	before := fake.MutationCount()
	if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil || result.Status != "base_move_observed" || fake.MutationCount() != before {
		t.Fatalf("base move = %#v, %v", result, err)
	}
	before = fake.MutationCount()
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil || fake.MutationCount() != before+1 {
		t.Fatalf("rebase transition: %v", err)
	}
	first, _ = fake.Delivery(1)
	before = fake.MutationCount()
	if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil || result.Status != "rebase_needed" || fake.MutationCount() != before+1 {
		t.Fatalf("successor link = %#v, %v", result, err)
	}
	before = fake.MutationCount()
	if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil || result.Status != "successor_intent" || fake.MutationCount() != before {
		t.Fatalf("intent = %#v, %v", result, err)
	}
	before = fake.MutationCount()
	if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil || result.Effect != "beads.create" || fake.MutationCount() != before+1 {
		t.Fatalf("create = %#v, %v", result, err)
	}
	child, ok := fake.Delivery(2)
	if !ok {
		t.Fatal("missing child")
	}
	if child.Route != "" || child.Record.Publication != "pending" || child.Record.Revision != 1 || child.Record.Predecessor != first.ID || child.Record.PredecessorReceiptDigest != first.Record.Current.Digest || child.Record.Epoch.BaseOID != newBase || child.Record.Epoch.Branch != first.Record.Epoch.Branch || child.Record.Epoch.LeaseOID != first.Record.Epoch.Head || child.Record.PR.NodeID != "PR_node" {
		t.Fatalf("child identity = %#v", child)
	}
	before = fake.MutationCount()
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil || fake.MutationCount() != before {
		t.Fatalf("created receipt: %v", err)
	}
	before = fake.MutationCount()
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil || fake.MutationCount() != before {
		t.Fatalf("child activation: %v", err)
	}
	first, _ = fake.Delivery(1)
	if first.Record.EpochSuccessorID != child.ID {
		t.Fatalf("predecessor link=%q", first.Record.EpochSuccessorID)
	}
	before = fake.MutationCount()
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil || fake.MutationCount() != before+1 {
		t.Fatalf("child publication: %v", err)
	}
	before = fake.MutationCount()
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil || fake.MutationCount() != before+1 {
		t.Fatalf("predecessor retire: %v", err)
	}
	before = fake.MutationCount()
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil || fake.MutationCount() != before+1 {
		t.Fatalf("child route: %v", err)
	}
	child, _ = fake.Delivery(2)
	if child.Route != "agentops.delivery" || child.Record.Publication != "published" || child.Record.Committed != first.Record.Committed {
		t.Fatalf("published child=%#v", child)
	}
	before = fake.MutationCount()
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil || fake.MutationCount() != before {
		t.Fatalf("settled replay: %v", err)
	}
	childSelected := request
	childSelected.Target.DeliveryBeadID = child.ID
	before = fake.MutationCount()
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), childSelected); err != nil || fake.MutationCount() != before+1 {
		t.Fatalf("child preparing: %v", err)
	}
	bad := request
	bad.Target.Epoch = 2
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), bad); err == nil {
		t.Fatal("caller epoch two accepted")
	}
}

func TestInitialDeliveryUsesNamespacedEvidenceAndExplicitLeafSelection(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	fake := delivery.NewFakeProviders(terminalFor(request))
	result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "prepared" {
		t.Fatalf("prepared = %#v, %v", result, err)
	}
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Effect != "beads.create" {
		t.Fatalf("atomic create = %#v, %v", result, err)
	}
	leaf, ok := fake.Delivery(1)
	if !ok {
		t.Fatal("missing initial leaf")
	}
	root := filepath.Join(request.Root, "handoffs", leaf.Record.HandoffID)
	for _, name := range []string{"certificate.json", "prepared.json"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "committed.json")); err != nil {
		t.Fatalf("committed evidence: %v", err)
	}
	badEpoch := request
	badEpoch.Target.Epoch = 2
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), badEpoch); err == nil {
		t.Fatal("caller selected epoch 2 without a bead")
	}
	badLeaf := request
	badLeaf.Target.DeliveryBeadID = "delivery-not-this-leaf"
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), badLeaf); err == nil {
		t.Fatal("mismatched selected leaf was accepted")
	}
	selected := request
	selected.Target.DeliveryBeadID = leaf.ID
	selected.Target.BaseOID = strings.Repeat("e", 40)
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil {
		t.Fatalf("selected leaf rewrote immutable base: %v", err)
	}
	leaf, _ = fake.Delivery(1)
	if leaf.Record.Epoch.BaseOID != strings.Repeat("d", 40) {
		t.Fatalf("selected leaf base rewritten to %s", leaf.Record.Epoch.BaseOID)
	}
}

func TestReducerRejectsMalformedOrOversizedCanonicalEnvelopeBeforeEffect(t *testing.T) {
	for name, mutate := range map[string]func(*delivery.DeliveryRecord){
		"zero_revision": func(record *delivery.DeliveryRecord) { record.Revision = 0 },
		"oversized":     func(record *delivery.DeliveryRecord) { record.RepairBeadID = strings.Repeat("x", 5000) },
		"current_path_escape": func(record *delivery.DeliveryRecord) {
			record.State, record.Current = delivery.DeliveryStatePreparing, delivery.ReceiptRef{Path: "handoffs/" + record.HandoffID + "/epochs/000001/../activation.json", Digest: strings.Repeat("a", 64)}
		},
		"current_wrong_namespace": func(record *delivery.DeliveryRecord) {
			record.State, record.Current = delivery.DeliveryStatePreparing, delivery.ReceiptRef{Path: "handoffs/" + strings.Repeat("b", 64) + "/epochs/000001/activation.json", Digest: strings.Repeat("a", 64)}
		},
		"landed_without_exact_evidence": func(record *delivery.DeliveryRecord) {
			record.State, record.Publication, record.Committed = delivery.DeliveryStateLanded, "published", strings.Repeat("b", 64)
			record.Current = delivery.ReceiptRef{Path: "handoffs/" + record.HandoffID + "/epochs/000001/landing.json", Digest: strings.Repeat("a", 64)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := requestForTest(t, t.TempDir())
			fake := delivery.NewFakeProviders(terminalFor(request))
			for _, want := range []string{"prepared", "successor_created"} {
				result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
				if err != nil || result.Status != want {
					t.Fatalf("reach %s = %#v, %v", want, result, err)
				}
			}
			leaf, found := fake.Delivery(1)
			if !found {
				t.Fatal("delivery leaf missing")
			}
			mutate(&leaf.Record)
			fake.PutDelivery(leaf)
			before := fake.MutationCount()
			if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err == nil {
				t.Fatalf("malformed envelope advanced as %#v", result)
			}
			if fake.MutationCount() != before {
				t.Fatal("malformed envelope performed a provider mutation")
			}
		})
	}
}

func TestInitialPreparedReceiptRepairsAfterCertificateOnlyCrash(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	fake := delivery.NewFakeProviders(terminalFor(request))
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	leaf := ""
	// Prepared evidence is written before the first Beads mutation. Simulate a
	// process death after certificate durability but before the second receipt.
	for _, path := range []string{filepath.Join(request.Root, "handoffs")} {
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 1 {
			t.Fatalf("handoff root: %v", err)
		}
		leaf = filepath.Join(path, entries[0].Name())
	}
	if err := os.Remove(filepath.Join(leaf, "prepared.json")); err != nil {
		t.Fatal(err)
	}
	result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "prepared" {
		t.Fatalf("repair = %#v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(leaf, "prepared.json")); err != nil {
		t.Fatal(err)
	}
	if fake.MutationCount() != 0 {
		t.Fatal("local receipt repair created a Beads mutation")
	}
}

func TestReducerEpochComposePushObserveThenBranchReady(t *testing.T) {
	request := requestForTest(t, t.TempDir())
	fake := delivery.NewFakeProviders(terminalFor(request))
	for i := 0; i < 6; i++ {
		if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
	before := fake.MutationCount()
	result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "epoch_composed" || fake.MutationCount() != before {
		t.Fatalf("compose=%#v %v", result, err)
	}
	before = fake.MutationCount()
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Effect != "git.push_force_with_lease" || fake.MutationCount() != before+1 {
		t.Fatalf("push=%#v %v", result, err)
	}
	before = fake.MutationCount()
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "branch_observed" || fake.MutationCount() != before {
		t.Fatalf("observe=%#v %v", result, err)
	}
	before = fake.MutationCount()
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "branch_ready" || fake.MutationCount() != before+1 {
		t.Fatalf("ready=%#v %v", result, err)
	}
	before = fake.MutationCount()
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "pr_intent" || fake.MutationCount() != before {
		t.Fatalf("next bounded seam=%#v %v", result, err)
	}
}

func TestReducerPersistsTypedEpochCompositionStopOutcomes(t *testing.T) {
	for _, scenario := range []struct {
		reason string
		state  delivery.DeliveryState
	}{
		{"path_collision", delivery.DeliveryStateRepairWait},
		{"zero_diff", delivery.DeliveryStateSuccessorRequired},
	} {
		t.Run(scenario.reason, func(t *testing.T) {
			request, fake, _ := preparingLeaf(t)
			fake.SetPrepareBranchFailure(scenario.reason)
			before := fake.MutationCount()
			result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
			if err != nil || result.Status != "delivery_outcome_receipted" || result.Reason == "" || fake.MutationCount() != before {
				t.Fatalf("outcome receipt = %#v, %v", result, err)
			}
			result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
			if err != nil || result.Status != string(scenario.state) || result.Reason == "" || fake.MutationCount() != before+1 {
				t.Fatalf("typed transition = %#v, %v", result, err)
			}
			leaf, _ := fake.Delivery(1)
			if leaf.Record.State != scenario.state || leaf.Record.DeliveryOutcome != result.Reason || leaf.Record.Current.Path == "" {
				t.Fatalf("typed delivery outcome = %#v", leaf.Record)
			}
		})
	}
}

func TestReducerTargetRegressionWaitsForNextObservedBaseWithoutMutation(t *testing.T) {
	request, fake, _ := preparingLeaf(t)
	fake.SetPrepareBranchFailure("target_regression")
	before := fake.MutationCount()
	result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "target_regression" || result.Reason != "base_moved_during_epoch_composition" || fake.MutationCount() != before {
		t.Fatalf("target regression = %#v, %v", result, err)
	}
}

func TestReducerPersistsNonDescendantBaseFailure(t *testing.T) {
	request, fake, leaf := preparingLeaf(t)
	selected := request
	selected.Target.DeliveryBeadID = leaf.ID
	fake.SetObservedBase(strings.Repeat("c", 40))
	before := fake.MutationCount()
	result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected)
	if err != nil || result.Status != "delivery_outcome_receipted" || result.Reason != "non_descendant_base_move" || fake.MutationCount() != before {
		t.Fatalf("non-descendant receipt = %#v, %v", result, err)
	}
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), selected)
	if err != nil || result.Status != "delivery_failed" || result.Reason != "non_descendant_base_move" || fake.MutationCount() != before+1 {
		t.Fatalf("non-descendant transition = %#v, %v", result, err)
	}
}

func TestReducerPRIntentPrepareCreateReceiptThenOpen(t *testing.T) {
	request, fake, _ := branchReadyLeaf(t)
	before := fake.MutationCount()
	result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "pr_intent" || fake.MutationCount() != before {
		t.Fatalf("pr intent = %#v, %v", result, err)
	}
	leaf, _ := fake.Delivery(1)
	intentPath := filepath.Join(request.Root, "handoffs", leaf.Record.HandoffID, "epochs", fmt.Sprintf("%06d", leaf.Record.Epoch.Number), "pr-intent.json")
	var intentWire map[string]any
	readReceipt(t, intentPath, &intentWire)
	for _, key := range []string{"schema_version", "handoff_id", "epoch", "repository", "base_ref", "base_oid", "branch", "expected_head", "pr_id", "effect_id"} {
		if _, ok := intentWire[key]; !ok {
			t.Fatalf("PR intent lacks snake_case key %q: %#v", key, intentWire)
		}
	}
	before = fake.MutationCount()
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "pr_prepared" || fake.MutationCount() != before+1 {
		t.Fatalf("pr prepared = %#v, %v", result, err)
	}
	leaf, _ = fake.Delivery(1)
	if leaf.Record.PR.ID == "" || leaf.Record.PR.EffectID == "" || leaf.Record.PR.Repository != request.Target.Repository || leaf.Record.PR.Branch != leaf.Record.Epoch.Branch || leaf.Record.PR.NodeID != "" {
		t.Fatalf("prepared stable PR identity = %#v", leaf.Record.PR)
	}
	before = fake.MutationCount()
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "pr_receipted" || result.Effect != "forge.pr" || fake.MutationCount() != before+1 || fake.PRCreateCount() != 1 {
		t.Fatalf("pr create/receipt = %#v, %v", result, err)
	}
	before = fake.MutationCount()
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "pr_open" || fake.MutationCount() != before+1 {
		t.Fatalf("pr open = %#v, %v", result, err)
	}
	leaf, _ = fake.Delivery(1)
	prWire, marshalErr := json.Marshal(leaf.Record.PR)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	var stableWire map[string]any
	if err := json.Unmarshal(prWire, &stableWire); err != nil {
		t.Fatal(err)
	}
	if _, ok := stableWire["base_oid"]; ok {
		t.Fatalf("stable PR leaked epoch base: %s", prWire)
	}
	if _, ok := stableWire["head"]; ok {
		t.Fatalf("stable PR leaked epoch head: %s", prWire)
	}
	before = fake.MutationCount()
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "ci_wait" || fake.MutationCount() != before+1 || fake.PRCreateCount() != 1 {
		t.Fatalf("hosted handoff = %#v, %v", result, err)
	}
}

func TestReducerPRCrashCutsColdReplayCreateAtMostOnce(t *testing.T) {
	for _, cut := range []string{
		"before_pr_intent", "after_pr_intent",
		"before_pr_prepare", "after_pr_prepare",
		"before_pr_create", "after_pr_create",
		"before_pr_receipt", "after_pr_receipt",
		"before_pr_open", "after_pr_open",
	} {
		t.Run(cut, func(t *testing.T) {
			request := requestForTest(t, t.TempDir())
			fixture := filepath.Join(t.TempDir(), "provider.json")
			crashed := false
			converged := false
			for attempt := 0; attempt < 32; attempt++ {
				fake, err := delivery.OpenFixtureProviders(fixture, terminalFor(request))
				if err != nil {
					t.Fatal(err)
				}
				if leaf, found := fake.Delivery(1); found && leaf.Record.State == delivery.DeliveryStatePROpen {
					converged = true
					break
				}
				var crash delivery.Crash
				if !crashed {
					crash = delivery.CrashAt(cut)
				}
				result, err := delivery.NewReducer(fake, crash).Step(context.Background(), request)
				if delivery.IsCrash(err) {
					crashed = true
					continue
				}
				if err != nil {
					t.Fatalf("attempt %d: %v", attempt, err)
				}
				if result.Status == "pr_open" {
					converged = true
					break
				}
			}
			if !crashed {
				t.Fatalf("crash cut %q was never reached", cut)
			}
			if !converged {
				t.Fatal("cold replay did not converge to pr_open")
			}
			fake, err := delivery.OpenFixtureProviders(fixture, terminalFor(request))
			if err != nil {
				t.Fatal(err)
			}
			if got := fake.PRCreateCount(); got != 1 {
				t.Fatalf("PR creates = %d, want exactly 1", got)
			}
			if got := fake.PRCount(); got != 1 {
				t.Fatalf("PRs = %d, want exactly 1", got)
			}
		})
	}
}

func TestReducerAdoptsPreexistingExactPRWithoutCreate(t *testing.T) {
	request, fake, leaf := branchReadyLeaf(t)
	result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "pr_intent" {
		t.Fatalf("prepare PR intent = %#v, %v", result, err)
	}
	intentPath := filepath.Join(request.Root, "handoffs", leaf.Record.HandoffID, "epochs", fmt.Sprintf("%06d", leaf.Record.Epoch.Number), "pr-intent.json")
	var intent delivery.PRIntent
	readReceipt(t, intentPath, &intent)
	fake.PutPR(delivery.PullRequest{
		ID: intent.PRID, EffectID: intent.EffectID, Repository: intent.Repository,
		BaseRef: intent.BaseRef, Branch: intent.Branch, NodeID: "PR_existing",
		Number: "17", URL: "https://example.invalid/pull/17",
	})
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "pr_prepared" {
		t.Fatalf("prepare existing PR = %#v, %v", result, err)
	}
	before := fake.MutationCount()
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "pr_receipted" || result.Effect != "" {
		t.Fatalf("adopt existing PR = %#v, %v", result, err)
	}
	if fake.MutationCount() != before || fake.PRCreateCount() != 0 {
		t.Fatalf("existing PR adoption mutated provider: mutations=%d creates=%d", fake.MutationCount()-before, fake.PRCreateCount())
	}
	var receipt delivery.PROpenReceipt
	readReceipt(t, filepath.Join(filepath.Dir(intentPath), "pr-open.json"), &receipt)
	if receipt.Outcome != "already_applied" || receipt.IntentDigest == "" || receipt.ResponseDigest == "" || receipt.ObservedBaseOID != intent.BaseOID || receipt.ObservedHead != intent.ExpectedHead || receipt.NodeID != "PR_existing" {
		t.Fatalf("existing PR receipt = %#v", receipt)
	}
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "pr_open" || fake.PRCreateCount() != 0 {
		t.Fatalf("open adopted PR = %#v, %v", result, err)
	}
}

func TestReducerBaseMovementBeforePRCreateNeverCreatesPR(t *testing.T) {
	for _, phase := range []string{"branch_ready", "pr_prepared"} {
		t.Run(phase, func(t *testing.T) {
			request, fake, leaf := branchReadyLeaf(t)
			if phase == "pr_prepared" {
				for _, want := range []string{"pr_intent", "pr_prepared"} {
					result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
					if err != nil || result.Status != want {
						t.Fatalf("reach %s = %#v, %v", want, result, err)
					}
				}
				leaf, _ = fake.Delivery(1)
			}
			selected := request
			selected.Target.DeliveryBeadID = leaf.ID
			newBase := strings.Repeat("c", 40)
			fake.SetObservedBase(newBase)
			fake.AllowBaseDescendant(newBase, leaf.Record.Epoch.BaseOID)
			result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected)
			if err != nil || result.Status != "base_move_observed" {
				t.Fatalf("observe base movement = %#v, %v", result, err)
			}
			result, err = delivery.NewReducer(fake, nil).Step(context.Background(), selected)
			if err != nil || result.Status != "rebase_needed" {
				t.Fatalf("record rebase = %#v, %v", result, err)
			}
			if fake.PRCreateCount() != 0 || fake.PRCount() != 0 {
				t.Fatalf("moving base created PR: creates=%d prs=%d", fake.PRCreateCount(), fake.PRCount())
			}
		})
	}
}

func TestReducerKnownMissingClosedConflictingOrAmbiguousPRNeverCreatesSecond(t *testing.T) {
	for _, scenario := range []string{"known_missing", "closed", "conflicting", "ambiguous"} {
		t.Run(scenario, func(t *testing.T) {
			request, fake, leaf := branchReadyLeaf(t)
			if scenario == "known_missing" {
				leaf.Record.PR = delivery.PullRequest{ID: "pr-known", EffectID: strings.Repeat("8", 64), Repository: request.Target.Repository, BaseRef: request.Target.BaseRef, Branch: leaf.Record.Epoch.Branch, NodeID: "PR_known", Number: "9", URL: "https://example.invalid/pull/9"}
				fake.PutDelivery(leaf)
			}
			for _, want := range []string{"pr_intent", "pr_prepared"} {
				result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
				if err != nil || result.Status != want {
					t.Fatalf("reach %s = %#v, %v", want, result, err)
				}
			}
			leaf, _ = fake.Delivery(1)
			switch scenario {
			case "closed":
				fake.SetPRObservation(delivery.PRObservation{State: "closed"})
			case "ambiguous":
				fake.SetPRObservation(delivery.PRObservation{State: "ambiguous"})
			case "conflicting":
				fake.SetPRObservation(delivery.PRObservation{
					State: "open", BaseOID: leaf.Record.Epoch.BaseOID, Head: leaf.Record.Epoch.Head,
					PR: delivery.PullRequest{ID: "other-pr", EffectID: strings.Repeat("7", 64), Repository: request.Target.Repository, BaseRef: leaf.Record.Epoch.BaseRef, Branch: leaf.Record.Epoch.Branch, NodeID: "PR_other", Number: "99", URL: "https://example.invalid/pull/99"},
				})
			}
			if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err == nil {
				t.Fatalf("%s observation advanced as %#v", scenario, result)
			}
			if fake.PRCreateCount() != 0 {
				t.Fatalf("%s observation created %d PRs", scenario, fake.PRCreateCount())
			}
		})
	}
}

func TestReducerEpochSuccessorUpdatesSameActualPRWithoutSecondCreate(t *testing.T) {
	request, fake, leaf := branchReadyLeaf(t)
	for _, want := range []string{"pr_intent", "pr_prepared", "pr_receipted", "pr_open"} {
		result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
		if err != nil || result.Status != want {
			t.Fatalf("reach epoch-one %s = %#v, %v", want, result, err)
		}
	}
	if fake.PRCreateCount() != 1 {
		t.Fatalf("epoch one PR creates = %d", fake.PRCreateCount())
	}
	leaf, _ = fake.Delivery(1)
	selected := request
	selected.Target.DeliveryBeadID = leaf.ID
	newBase := strings.Repeat("c", 40)
	fake.SetObservedBase(newBase)
	fake.AllowBaseDescendant(newBase, leaf.Record.Epoch.BaseOID)
	childReady := false
	for attempt := 0; attempt < 16; attempt++ {
		if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil {
			t.Fatalf("successor step %d: %v", attempt, err)
		}
		child, found := fake.Delivery(2)
		predecessor, _ := fake.Delivery(1)
		if found && child.Route == "agentops.delivery" && child.Record.Publication == "published" && predecessor.Route == "" {
			childReady = true
			break
		}
	}
	if !childReady {
		t.Fatal("epoch-two successor was not published and selected")
	}
	child, _ := fake.Delivery(2)
	if child.Record.PR.NodeID == "" || child.Record.PR.ID != leaf.Record.PR.ID || child.Record.PR.EffectID != leaf.Record.PR.EffectID {
		t.Fatalf("successor did not retain stable actual PR: parent=%#v child=%#v", leaf.Record.PR, child.Record.PR)
	}
	childSelected := request
	childSelected.Target.DeliveryBeadID = child.ID
	converged := false
	for attempt := 0; attempt < 16; attempt++ {
		result, err := delivery.NewReducer(fake, nil).Step(context.Background(), childSelected)
		if err != nil {
			t.Fatalf("epoch-two step %d: %v", attempt, err)
		}
		if result.Status == "pr_open" {
			converged = true
			break
		}
	}
	if !converged {
		t.Fatal("epoch two did not reopen the stable PR")
	}
	if fake.PRCreateCount() != 1 || fake.PRCount() != 1 {
		t.Fatalf("successor duplicated PR: creates=%d prs=%d", fake.PRCreateCount(), fake.PRCount())
	}
}

func TestReducerSuccessorCreationAndPublicationCrashCutsColdReplayOneChild(t *testing.T) {
	for _, cut := range []string{"before_successor_create", "after_successor_create", "before_successor_publication", "after_successor_publication"} {
		t.Run(cut, func(t *testing.T) {
			request, fake, predecessor := epochOnePROpenLeaf(t)
			selected := request
			selected.Target.DeliveryBeadID = predecessor.ID
			newBase := strings.Repeat("c", 40)
			fake.SetObservedBase(newBase)
			fake.AllowBaseDescendant(newBase, predecessor.Record.Epoch.BaseOID)
			for _, want := range []string{"base_move_observed", "rebase_needed", "rebase_needed", "successor_intent"} {
				result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected)
				if err != nil || result.Status != want {
					t.Fatalf("reach %s = %#v, %v", want, result, err)
				}
			}
			crashed, ready := false, false
			for attempt := 0; attempt < 20; attempt++ {
				var crash delivery.Crash
				if !crashed {
					crash = delivery.CrashAt(cut)
				}
				result, err := delivery.NewReducer(fake, crash).Step(context.Background(), selected)
				if delivery.IsCrash(err) {
					crashed = true
					continue
				}
				if err != nil {
					t.Fatalf("replay %d: %v", attempt, err)
				}
				parent, _ := fake.Delivery(1)
				child, found := fake.Delivery(2)
				if found && parent.Route != "" && child.Route != "" {
					t.Fatalf("cut=%s replay=%d exposed two selected routes: parent=%q child=%q", cut, attempt, parent.Route, child.Route)
				}
				if result.Status == "successor_ready" {
					ready = true
					break
				}
			}
			if !crashed || !ready || fake.DeliveryCount() != 2 {
				t.Fatalf("cut=%s crashed=%t ready=%t deliveries=%d", cut, crashed, ready, fake.DeliveryCount())
			}
			child, found := fake.Delivery(2)
			parent, _ := fake.Delivery(1)
			if !found || child.Route != "agentops.delivery" || parent.Route != "" || child.Record.Predecessor != parent.ID || child.Record.PR.ID != parent.Record.PR.ID || fake.PRCreateCount() != 1 {
				t.Fatalf("crash-replayed successor: parent=%#v child=%#v creates=%d", parent, child, fake.PRCreateCount())
			}
		})
	}
}

func TestReducerRejectsEveryTamperedSuccessorIntentField(t *testing.T) {
	mutations := map[string]func(map[string]any){
		"schema_version":               func(value map[string]any) { value["schema_version"] = "wrong.v1" },
		"handoff_id":                   func(value map[string]any) { value["handoff_id"] = strings.Repeat("1", 64) },
		"predecessor_id":               func(value map[string]any) { value["predecessor_id"] = "delivery-other-e000001" },
		"child_id":                     func(value map[string]any) { value["child_id"] = "delivery-other-e000002" },
		"external_ref":                 func(value map[string]any) { value["external_ref"] = "handoff:other:epoch:2" },
		"epoch":                        func(value map[string]any) { value["epoch"] = float64(3) },
		"branch":                       func(value map[string]any) { value["branch"] = "gc/delivery/ffffffffffffffffffff" },
		"lease_oid":                    func(value map[string]any) { value["lease_oid"] = strings.Repeat("e", 40) },
		"semantic_bead_id":             func(value map[string]any) { value["semantic_bead_id"] = "semantic-other" },
		"semantic_terminal_ref":        func(value map[string]any) { value["semantic_terminal_ref"] = "beads:other#terminal" },
		"admission_certificate_digest": func(value map[string]any) { value["admission_certificate_digest"] = strings.Repeat("e", 64) },
		"committed_handoff_digest":     func(value map[string]any) { value["committed_handoff_digest"] = strings.Repeat("e", 64) },
		"subject_manifest_digest":      func(value map[string]any) { value["subject_manifest_digest"] = strings.Repeat("1", 64) },
		"predecessor_revision":         func(value map[string]any) { value["predecessor_revision"] = float64(999) },
		"predecessor_receipt_digest":   func(value map[string]any) { value["predecessor_receipt_digest"] = strings.Repeat("e", 64) },
		"candidate_oid":                func(value map[string]any) { value["candidate_oid"] = strings.Repeat("e", 40) },
		"repository":                   func(value map[string]any) { value["repository"] = "other/repository" },
		"ready_at":                     func(value map[string]any) { value["ready_at"] = "2026-07-21T01:00:00Z" },
		"deadline":                     func(value map[string]any) { value["deadline"] = "2026-07-23T00:00:00Z" },
		"mode":                         func(value map[string]any) { value["mode"] = "manual" },
		"rig_id":                       func(value map[string]any) { value["rig_id"] = "rig-other" },
		"remote":                       func(value map[string]any) { value["remote"] = "upstream" },
		"base_ref":                     func(value map[string]any) { value["base_ref"] = "trunk" },
		"base_oid":                     func(value map[string]any) { value["base_oid"] = strings.Repeat("e", 40) },
		"auto_merge_effect_id":         func(value map[string]any) { value["auto_merge_effect_id"] = strings.Repeat("1", 64) },
		"auto_merge_attempt": func(value map[string]any) {
			value["auto_merge_attempt"] = map[string]any{"path": "handoffs/other/epochs/000001/auto-merge-attempt.json", "digest": strings.Repeat("1", 64)}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			request, fake, predecessor := epochOnePROpenLeaf(t)
			selected := request
			selected.Target.DeliveryBeadID = predecessor.ID
			newBase := strings.Repeat("c", 40)
			fake.SetObservedBase(newBase)
			fake.AllowBaseDescendant(newBase, predecessor.Record.Epoch.BaseOID)
			for _, want := range []string{"base_move_observed", "rebase_needed", "rebase_needed", "successor_intent"} {
				result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected)
				if err != nil || result.Status != want {
					t.Fatalf("reach %s = %#v, %v", want, result, err)
				}
			}
			intentPath := filepath.Join(request.Root, "handoffs", predecessor.Record.HandoffID, "epochs", "000001", "successor-intent.json")
			mutateReceipt(t, intentPath, mutate)
			before := fake.MutationCount()
			if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err == nil {
				t.Fatalf("tampered intent advanced as %#v", result)
			}
			if fake.MutationCount() != before || fake.DeliveryCount() != 1 {
				t.Fatal("tampered intent created or mutated a successor")
			}
		})
	}
}

func TestReducerRejectsTamperedSuccessorCreatedChildDigest(t *testing.T) {
	request, fake, predecessor := epochOnePROpenLeaf(t)
	selected := request
	selected.Target.DeliveryBeadID = predecessor.ID
	newBase := strings.Repeat("c", 40)
	fake.SetObservedBase(newBase)
	fake.AllowBaseDescendant(newBase, predecessor.Record.Epoch.BaseOID)
	for _, want := range []string{"base_move_observed", "rebase_needed", "rebase_needed", "successor_intent", "successor_created", "successor_created_receipted"} {
		result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected)
		if err != nil || result.Status != want {
			t.Fatalf("reach %s = %#v, %v", want, result, err)
		}
	}
	receiptPath := filepath.Join(request.Root, "handoffs", predecessor.Record.HandoffID, "epochs", "000001", "successor-created.json")
	mutateReceipt(t, receiptPath, func(value map[string]any) { value["child_digest"] = strings.Repeat("e", 64) })
	before := fake.MutationCount()
	if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err == nil {
		t.Fatalf("tampered child digest advanced as %#v", result)
	}
	if fake.MutationCount() != before {
		t.Fatal("tampered child digest mutated Beads")
	}
}

func TestReducerRejectsTamperedPublishedSuccessorActivationBeforeRouteRetirement(t *testing.T) {
	request, fake, predecessor := epochOnePROpenLeaf(t)
	selected := request
	selected.Target.DeliveryBeadID = predecessor.ID
	newBase := strings.Repeat("c", 40)
	fake.SetObservedBase(newBase)
	fake.AllowBaseDescendant(newBase, predecessor.Record.Epoch.BaseOID)
	var child delivery.DeliveryBead
	for attempt := 0; attempt < 16; attempt++ {
		if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil {
			t.Fatalf("reach published successor step %d: %v", attempt, err)
		}
		candidate, found := fake.Delivery(2)
		parent, _ := fake.Delivery(1)
		if found && candidate.Record.Publication == "published" && candidate.Record.Current.Path != "" && parent.Route == "agentops.delivery" {
			child = candidate
			break
		}
	}
	if child.ID == "" {
		t.Fatal("did not reach the published child before route retirement")
	}
	mutateReceipt(t, filepath.Join(request.Root, child.Record.Current.Path), func(receipt map[string]any) { receipt["child_digest"] = strings.Repeat("e", 64) })
	before := fake.MutationCount()
	if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err == nil {
		t.Fatalf("tampered activation retired or published a route as %#v", result)
	}
	parent, _ := fake.Delivery(1)
	child, _ = fake.Delivery(2)
	if fake.MutationCount() != before || parent.Route != "agentops.delivery" || child.Route != "" {
		t.Fatalf("tampered activation changed routes: parent=%q child=%q mutations=%d", parent.Route, child.Route, fake.MutationCount()-before)
	}
}

func TestReducerRejectsMissingPublishedSuccessorActivationBeforeRouteRetirement(t *testing.T) {
	request, fake, predecessor := epochOnePROpenLeaf(t)
	selected := request
	selected.Target.DeliveryBeadID = predecessor.ID
	newBase := strings.Repeat("c", 40)
	fake.SetObservedBase(newBase)
	fake.AllowBaseDescendant(newBase, predecessor.Record.Epoch.BaseOID)
	var child delivery.DeliveryBead
	for attempt := 0; attempt < 16; attempt++ {
		if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil {
			t.Fatalf("reach published successor step %d: %v", attempt, err)
		}
		candidate, found := fake.Delivery(2)
		parent, _ := fake.Delivery(1)
		if found && candidate.Record.Publication == "published" && candidate.Record.Current.Path != "" && parent.Route == "agentops.delivery" {
			child = candidate
			break
		}
	}
	if child.ID == "" {
		t.Fatal("did not reach the published child before route retirement")
	}
	if err := os.Remove(filepath.Join(request.Root, child.Record.Current.Path)); err != nil {
		t.Fatal(err)
	}
	before := fake.MutationCount()
	if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err == nil {
		t.Fatalf("missing activation retired or published a route as %#v", result)
	}
	parent, _ := fake.Delivery(1)
	child, _ = fake.Delivery(2)
	if fake.MutationCount() != before || parent.Route != "agentops.delivery" || child.Route != "" {
		t.Fatalf("missing activation changed routes: parent=%q child=%q mutations=%d", parent.Route, child.Route, fake.MutationCount()-before)
	}
}

func TestReducerSuccessorActivationRebindsEveryCreatedIntentGroup(t *testing.T) {
	mutations := map[string]func(map[string]any){
		"predecessor_revision": func(intent map[string]any) { intent["predecessor_revision"] = float64(999) },
		"predecessor_receipt":  func(intent map[string]any) { intent["predecessor_receipt_digest"] = strings.Repeat("e", 64) },
		"external_ref":         func(intent map[string]any) { intent["external_ref"] = "handoff:other:epoch:2" },
		"semantic_terminal": func(intent map[string]any) {
			intent["semantic_bead_id"], intent["semantic_terminal_ref"] = "other", "beads:other#terminal"
		},
		"time_mode_rig": func(intent map[string]any) {
			intent["ready_at"], intent["deadline"], intent["mode"], intent["rig_id"] = "2026-07-21T01:00:00Z", "2026-07-23T00:00:00Z", "manual", "other-rig"
		},
		"repository_remote": func(intent map[string]any) { intent["repository"], intent["remote"] = "other/repo", "upstream" },
		"base_branch_lease": func(intent map[string]any) {
			intent["base_ref"], intent["base_oid"], intent["branch"], intent["lease_oid"] = "trunk", strings.Repeat("e", 40), "gc/delivery/ffffffffffffffffffff", strings.Repeat("e", 40)
		},
		"candidate_certificate_manifest": func(intent map[string]any) {
			intent["candidate_oid"], intent["admission_certificate_digest"], intent["subject_manifest_digest"] = strings.Repeat("e", 40), strings.Repeat("e", 64), strings.Repeat("d", 64)
		},
		"committed":         func(intent map[string]any) { intent["committed_handoff_digest"] = strings.Repeat("e", 64) },
		"auto_merge_effect": func(intent map[string]any) { intent["auto_merge_effect_id"] = strings.Repeat("e", 64) },
		"auto_merge_attempt": func(intent map[string]any) {
			intent["auto_merge_attempt"] = map[string]any{"path": "handoffs/" + strings.Repeat("a", 64) + "/epochs/000001/auto-merge-attempt.json", "digest": strings.Repeat("e", 64)}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			request, fake, predecessor := epochOnePROpenLeaf(t)
			selected := request
			selected.Target.DeliveryBeadID = predecessor.ID
			newBase := strings.Repeat("c", 40)
			fake.SetObservedBase(newBase)
			fake.AllowBaseDescendant(newBase, predecessor.Record.Epoch.BaseOID)
			var child delivery.DeliveryBead
			for attempt := 0; attempt < 16; attempt++ {
				if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil {
					t.Fatalf("reach published successor step %d: %v", attempt, err)
				}
				candidate, found := fake.Delivery(2)
				parent, _ := fake.Delivery(1)
				if found && candidate.Record.Publication == "published" && parent.Route == "agentops.delivery" {
					child = candidate
					break
				}
			}
			if child.ID == "" {
				t.Fatal("did not reach published successor")
			}
			createdPath := filepath.Join(request.Root, "handoffs", predecessor.Record.HandoffID, "epochs", "000001", "successor-created.json")
			mutateReceipt(t, createdPath, func(receipt map[string]any) { mutate(receipt["intent"].(map[string]any)) })
			createdBytes, err := os.ReadFile(createdPath)
			if err != nil {
				t.Fatal(err)
			}
			activationPath := filepath.Join(request.Root, child.Record.Current.Path)
			mutateReceipt(t, activationPath, func(receipt map[string]any) { receipt["successor_receipt_digest"] = digest(createdBytes) })
			activationBytes, err := os.ReadFile(activationPath)
			if err != nil {
				t.Fatal(err)
			}
			child.Record.Current.Digest = digest(activationBytes)
			fake.PutDelivery(child)
			before := fake.MutationCount()
			if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err == nil {
				t.Fatalf("coherently tampered %s intent advanced as %#v", name, result)
			}
			parent, _ := fake.Delivery(1)
			if fake.MutationCount() != before || parent.Route != "agentops.delivery" {
				t.Fatalf("coherently tampered %s intent changed routes", name)
			}
		})
	}
}

func TestReducerThreeEpochChainHasOneActiveLeafStableBranchPRAndExactLeases(t *testing.T) {
	request, fake, epochOne := epochOnePROpenLeaf(t)
	epochTwo := createReadySuccessor(t, fake, request, epochOne, strings.Repeat("c", 40))
	epochTwoSelected := request
	epochTwoSelected.Target.DeliveryBeadID = epochTwo.ID
	epochTwo = reachSelectedState(t, fake, epochTwoSelected, 2, delivery.DeliveryStatePROpen)
	epochThree := createReadySuccessor(t, fake, request, epochTwo, strings.Repeat("e", 40))

	if fake.DeliveryCount() != 3 || fake.BranchCount() != 1 || fake.PRCount() != 1 || fake.PRCreateCount() != 1 {
		t.Fatalf("three-epoch cardinality: deliveries=%d branches=%d prs=%d creates=%d", fake.DeliveryCount(), fake.BranchCount(), fake.PRCount(), fake.PRCreateCount())
	}
	one, _ := fake.Delivery(1)
	two, _ := fake.Delivery(2)
	three, _ := fake.Delivery(3)
	if one.Route != "" || two.Route != "" || three.Route != "agentops.delivery" {
		t.Fatalf("active routes: e1=%q e2=%q e3=%q", one.Route, two.Route, three.Route)
	}
	if two.Record.Predecessor != one.ID || three.Record.Predecessor != two.ID || one.Record.EpochSuccessorID != two.ID || two.Record.EpochSuccessorID != three.ID {
		t.Fatalf("predecessor chain: e1=%#v e2=%#v e3=%#v", one.Record, two.Record, three.Record)
	}
	if two.Record.Epoch.LeaseOID != one.Record.Epoch.Head || three.Record.Epoch.LeaseOID != two.Record.Epoch.Head || one.Record.Epoch.Branch != two.Record.Epoch.Branch || two.Record.Epoch.Branch != three.Record.Epoch.Branch || one.Record.PR.ID != two.Record.PR.ID || two.Record.PR.ID != three.Record.PR.ID {
		t.Fatalf("stable delivery identity: e1=%#v e2=%#v e3=%#v", one.Record, two.Record, three.Record)
	}
	if epochThree.ID != three.ID {
		t.Fatalf("returned epoch three = %s, stored %s", epochThree.ID, three.ID)
	}
}

func epochOnePROpenLeaf(t *testing.T) (delivery.Request, *delivery.FakeProviders, delivery.DeliveryBead) {
	t.Helper()
	request, fake, _ := branchReadyLeaf(t)
	return request, fake, reachSelectedState(t, fake, request, 1, delivery.DeliveryStatePROpen)
}

func createReadySuccessor(t *testing.T, fake *delivery.FakeProviders, request delivery.Request, predecessor delivery.DeliveryBead, newBase string) delivery.DeliveryBead {
	t.Helper()
	selected := request
	selected.Target.DeliveryBeadID = predecessor.ID
	fake.SetObservedBase(newBase)
	fake.AllowBaseDescendant(newBase, predecessor.Record.Epoch.BaseOID)
	for attempt := 0; attempt < 16; attempt++ {
		if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil {
			t.Fatalf("create epoch %d successor step %d: %v", predecessor.Record.Epoch.Number+1, attempt, err)
		}
		child, found := fake.Delivery(predecessor.Record.Epoch.Number + 1)
		parent, _ := fake.Delivery(predecessor.Record.Epoch.Number)
		if found && child.Route == "agentops.delivery" && child.Record.Publication == "published" && parent.Route == "" {
			return child
		}
	}
	t.Fatalf("epoch %d successor did not become the active leaf", predecessor.Record.Epoch.Number+1)
	return delivery.DeliveryBead{}
}

func reachSelectedState(t *testing.T, fake *delivery.FakeProviders, request delivery.Request, epoch int, state delivery.DeliveryState) delivery.DeliveryBead {
	t.Helper()
	for attempt := 0; attempt < 20; attempt++ {
		if leaf, found := fake.Delivery(epoch); found && leaf.Record.State == state {
			return leaf
		}
		if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err != nil {
			t.Fatalf("reach epoch %d %s at step %d: %v", epoch, state, attempt, err)
		}
	}
	t.Fatalf("epoch %d did not reach %s", epoch, state)
	return delivery.DeliveryBead{}
}

func branchReadyLeaf(t *testing.T) (delivery.Request, *delivery.FakeProviders, delivery.DeliveryBead) {
	t.Helper()
	request, fake, _ := preparingLeaf(t)
	for _, want := range []string{"epoch_composed", "branch_pushed", "branch_observed", "branch_ready"} {
		result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
		if err != nil || result.Status != want {
			t.Fatalf("reach branch_ready %q = %#v, %v", want, result, err)
		}
	}
	leaf, found := fake.Delivery(1)
	if !found || leaf.Record.State != delivery.DeliveryStateBranchReady {
		t.Fatalf("branch-ready leaf = %#v", leaf)
	}
	return request, fake, leaf
}

func TestReducerEpochTwoPushesOnceWithExactPredecessorLease(t *testing.T) {
	request, fake, child := epochTwoPreparingLeaf(t)
	lease := child.Record.Epoch.LeaseOID
	if lease == "" {
		t.Fatal("epoch two has no predecessor lease")
	}

	result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "epoch_composed" {
		t.Fatalf("compose epoch two = %#v, %v", result, err)
	}
	var epoch delivery.EpochReceipt
	readReceipt(t, epochReceiptPath(request.Root, child), &epoch)
	if epoch.LeaseOID != lease {
		t.Fatalf("epoch receipt lease = %q, want %q", epoch.LeaseOID, lease)
	}

	fake.PutBranch(delivery.Branch{Name: child.Record.Epoch.Branch, BaseRef: child.Record.Epoch.BaseRef, BaseOID: child.Record.Epoch.BaseOID, Head: lease})
	beforePushes := fake.PushCount()
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "branch_pushed" || result.Effect != "git.push_force_with_lease" {
		t.Fatalf("push epoch two = %#v, %v", result, err)
	}
	if got := fake.PushCount(); got != beforePushes+1 {
		t.Fatalf("push count = %d, want %d", got, beforePushes+1)
	}
	pushed, found, err := fake.FindBranch(context.Background(), child.Record.Epoch.Branch)
	if err != nil || !found || pushed.LeaseOID != lease {
		t.Fatalf("pushed branch = %#v, found=%t, err=%v", pushed, found, err)
	}
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "branch_observed" {
		t.Fatalf("observe epoch two branch = %#v, %v", result, err)
	}
	var branchReceipt delivery.BranchReceipt
	readReceipt(t, branchReceiptPath(request.Root, child), &branchReceipt)
	if branchReceipt.Outcome != "observed" || branchReceipt.LeaseOID != lease || branchReceipt.ExpectedHead != pushed.Head {
		t.Fatalf("branch receipt = %#v", branchReceipt)
	}
	result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "branch_ready" {
		t.Fatalf("advance epoch two branch = %#v, %v", result, err)
	}
	if got := fake.PushCount(); got != beforePushes+1 {
		t.Fatalf("replay pushed epoch two again: %d", got)
	}
}

func TestReducerEpochTwoRejectsConflictingRemoteHeadWithoutPush(t *testing.T) {
	request, fake, child := epochTwoPreparingLeaf(t)
	composeEpoch(t, fake, request)
	fake.PutBranch(delivery.Branch{Name: child.Record.Epoch.Branch, BaseRef: child.Record.Epoch.BaseRef, BaseOID: child.Record.Epoch.BaseOID, Head: strings.Repeat("e", 40)})
	beforeMutations, beforePushes := fake.MutationCount(), fake.PushCount()
	if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err == nil {
		t.Fatalf("conflicting remote head advanced as %#v", result)
	}
	if got := fake.MutationCount(); got != beforeMutations {
		t.Fatalf("conflicting remote head mutated providers: got %d want %d", got, beforeMutations)
	}
	if got := fake.PushCount(); got != beforePushes {
		t.Fatalf("conflicting remote head pushed: got %d want %d", got, beforePushes)
	}
}

func TestReducerRejectsCorruptEpochBranchReceiptWithoutProviderMutation(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"schema":        func(receipt map[string]any) { receipt["schema_version"] = "wrong.v1" },
		"expected_head": func(receipt map[string]any) { receipt["expected_head"] = strings.Repeat("e", 40) },
		"lease_oid":     func(receipt map[string]any) { receipt["lease_oid"] = strings.Repeat("e", 40) },
	} {
		t.Run(name, func(t *testing.T) {
			request, fake, child := epochTwoPreparingLeaf(t)
			composeEpoch(t, fake, request)
			fake.PutBranch(delivery.Branch{Name: child.Record.Epoch.Branch, BaseRef: child.Record.Epoch.BaseRef, BaseOID: child.Record.Epoch.BaseOID, Head: child.Record.Epoch.LeaseOID})
			if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err != nil || result.Status != "branch_pushed" {
				t.Fatalf("push = %#v, %v", result, err)
			}
			if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err != nil || result.Status != "branch_observed" {
				t.Fatalf("observe = %#v, %v", result, err)
			}
			mutateReceipt(t, branchReceiptPath(request.Root, child), mutate)
			beforeMutations, beforePushes := fake.MutationCount(), fake.PushCount()
			if result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err == nil {
				t.Fatalf("corrupt branch receipt advanced as %#v", result)
			}
			if got := fake.MutationCount(); got != beforeMutations {
				t.Fatalf("corrupt branch receipt mutated providers: got %d want %d", got, beforeMutations)
			}
			if got := fake.PushCount(); got != beforePushes {
				t.Fatalf("corrupt branch receipt pushed: got %d want %d", got, beforePushes)
			}
		})
	}
}

func TestReducerEpochPushCrashCutsReplayWithoutDuplicatePush(t *testing.T) {
	for _, cut := range []string{"before_branch_push", "after_branch_push"} {
		t.Run(cut, func(t *testing.T) {
			request, fake, _ := preparingLeaf(t)
			composeEpoch(t, fake, request)
			beforePushes := fake.PushCount()
			if _, err := delivery.NewReducer(fake, delivery.CrashAt(cut)).Step(context.Background(), request); !delivery.IsCrash(err) {
				t.Fatalf("crash cut %q = %v", cut, err)
			}
			wantAfterCrash := beforePushes
			if cut == "after_branch_push" {
				wantAfterCrash++
			}
			if got := fake.PushCount(); got != wantAfterCrash {
				t.Fatalf("pushes after %s = %d, want %d", cut, got, wantAfterCrash)
			}
			result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
			wantReplayStatus := "branch_observed"
			if cut == "before_branch_push" {
				wantReplayStatus = "branch_pushed"
			}
			if err != nil || result.Status != wantReplayStatus {
				t.Fatalf("replay after %s = %#v, %v", cut, result, err)
			}
			if cut == "before_branch_push" {
				result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
				if err != nil || result.Status != "branch_observed" {
					t.Fatalf("replay observation = %#v, %v", result, err)
				}
			}
			result, err = delivery.NewReducer(fake, nil).Step(context.Background(), request)
			if err != nil || result.Status != "branch_ready" {
				t.Fatalf("replay branch ready = %#v, %v", result, err)
			}
			if got := fake.PushCount(); got != beforePushes+1 {
				t.Fatalf("replay duplicate push after %s: got %d want %d", cut, got, beforePushes+1)
			}
		})
	}
}

func preparingLeaf(t *testing.T) (delivery.Request, *delivery.FakeProviders, delivery.DeliveryBead) {
	t.Helper()
	request := requestForTest(t, t.TempDir())
	fake := delivery.NewFakeProviders(terminalFor(request))
	for i := 0; i < 6; i++ {
		if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
	leaf, found := fake.Delivery(1)
	if !found || leaf.Record.State != delivery.DeliveryStatePreparing || leaf.Route != "agentops.delivery" {
		t.Fatalf("initial preparing leaf = %#v", leaf)
	}
	return request, fake, leaf
}

func epochTwoPreparingLeaf(t *testing.T) (delivery.Request, *delivery.FakeProviders, delivery.DeliveryBead) {
	t.Helper()
	request, fake, predecessor := preparingLeaf(t)
	predecessor.Record.Epoch.Head, predecessor.Record.Epoch.Tree = strings.Repeat("a", 40), strings.Repeat("b", 40)
	predecessor.Record.PR = delivery.PullRequest{ID: "pr-stable", EffectID: strings.Repeat("9", 64), Repository: request.Target.Repository, BaseRef: request.Target.BaseRef, Branch: predecessor.Record.Epoch.Branch, NodeID: "PR_node", Number: "42", URL: "https://example.invalid/pr/42"}
	fake.PutDelivery(predecessor)
	selected := request
	selected.Target.DeliveryBeadID = predecessor.ID
	newBase := strings.Repeat("c", 40)
	fake.SetObservedBase(newBase)
	fake.AllowBaseDescendant(newBase, request.Target.BaseOID)
	for i := 0; i < 12; i++ {
		if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), selected); err != nil {
			t.Fatalf("prepare successor step %d: %v", i, err)
		}
	}
	child, found := fake.Delivery(2)
	if !found || child.Record.Predecessor != predecessor.ID || child.Route != "agentops.delivery" || child.Record.Epoch.LeaseOID != predecessor.Record.Epoch.Head {
		t.Fatalf("epoch two child = %#v", child)
	}
	childSelected := request
	childSelected.Target.DeliveryBeadID = child.ID
	if _, err := delivery.NewReducer(fake, nil).Step(context.Background(), childSelected); err != nil {
		t.Fatalf("transition child preparing: %v", err)
	}
	child, _ = fake.Delivery(2)
	if child.Record.State != delivery.DeliveryStatePreparing {
		t.Fatalf("child state = %q, want preparing", child.Record.State)
	}
	return childSelected, fake, child
}

func composeEpoch(t *testing.T, fake *delivery.FakeProviders, request delivery.Request) {
	t.Helper()
	result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
	if err != nil || result.Status != "epoch_composed" {
		t.Fatalf("compose epoch = %#v, %v", result, err)
	}
}

func epochReceiptPath(root string, leaf delivery.DeliveryBead) string {
	return filepath.Join(root, "handoffs", leaf.Record.HandoffID, "epochs", fmt.Sprintf("%06d", leaf.Record.Epoch.Number), "epoch.json")
}

func branchReceiptPath(root string, leaf delivery.DeliveryBead) string {
	return filepath.Join(root, "handoffs", leaf.Record.HandoffID, "epochs", fmt.Sprintf("%06d", leaf.Record.Epoch.Number), "branch.json")
}

func readReceipt(t *testing.T, path string, into any) {
	t.Helper()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(bytes, into); err != nil {
		t.Fatal(err)
	}
}

func mutateReceipt(t *testing.T, path string, mutate func(map[string]any)) {
	t.Helper()
	var receipt map[string]any
	readReceipt(t, path, &receipt)
	mutate(receipt)
	bytes, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(bytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mutateReceiptSchema(t *testing.T, path, schema string) {
	t.Helper()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var receipt map[string]any
	if err := json.Unmarshal(bytes, &receipt); err != nil {
		t.Fatal(err)
	}
	if schema == "missing" {
		delete(receipt, "schema_version")
	} else {
		receipt["schema_version"] = "wrong.v1"
	}
	bytes, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(bytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func reachStatus(t *testing.T, fake *delivery.FakeProviders, request delivery.Request, want string) {
	t.Helper()
	for i := 0; i < 16; i++ {
		result, err := delivery.NewReducer(fake, nil).Step(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status == want {
			return
		}
	}
	t.Fatalf("did not reach status %q", want)
}

func handoffID(t *testing.T, root string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "handoffs", "*", "prepared.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("prepared handoff matches = %#v, %v", matches, err)
	}
	bytes, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		HandoffID string `json:"handoff_id"`
	}
	if err := json.Unmarshal(bytes, &artifact); err != nil {
		t.Fatal(err)
	}
	return artifact.HandoffID
}
func identifierForTest(parts ...string) string { return digest([]byte(strings.Join(parts, "\x00"))) }

func digest(value []byte) string { sum := sha256.Sum256(value); return hex.EncodeToString(sum[:]) }

func requestForTest(t *testing.T, root string) delivery.Request {
	t.Helper()
	certificate := certificateFor("semantic-42")
	bytes, err := json.Marshal(certificate)
	if err != nil {
		t.Fatal(err)
	}
	return delivery.Request{Root: root, Certificate: certificate, CertificateBytes: bytes, CertificateDigest: digest(bytes), Target: targetForTest()}
}

func targetForTest() delivery.Target {
	return delivery.Target{SemanticBeadID: "semantic-42", SemanticTerminalRef: "beads:semantic-42#terminal", RigID: "rig-a", Repository: "boshu2/agentops", Remote: "origin", Epoch: 1, Mode: "auto", Deadline: "2026-07-22T00:00:00Z", PreparedAt: "2026-07-21T00:00:00Z", CommittedAt: "2026-07-21T00:00:01Z", BaseRef: "main", BaseOID: strings.Repeat("d", 40)}
}

func terminalFor(request delivery.Request) delivery.Terminal {
	return delivery.Terminal{BeadID: request.Target.SemanticBeadID, Ref: request.Target.SemanticTerminalRef, Verdict: "PASS", CertificateDigest: request.CertificateDigest}
}

func certificateFor(beadID string) delivery.AdmissionCertificate {
	noFallback := noFallbackProfile()
	return delivery.AdmissionCertificate{SchemaVersion: "admission-certificate.v2", SemanticBeadID: beadID, IntentDigest: strings.Repeat("a", 64), Verdict: "PASS", Candidate: delivery.Candidate{Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), ContentDigest: strings.Repeat("c", 64)}, Store: delivery.Store{Identity: "beads", Digest: strings.Repeat("d", 64)}, ChangedPathManifest: strings.Repeat("e", 64), VerdictDigest: strings.Repeat("f", 64), EvidenceDigest: strings.Repeat("0", 64), Attestations: delivery.Attestations{Author: delivery.Runtime{ContextID: "author", RequestedModel: "terra", RequestedReasoning: "high", RequestedProvider: "codex", ActualModel: "gpt-5.6-terra", ActualReasoning: "high", ActualProvider: "codex", ActualEffort: "high", Fallback: noFallback}, Validator: delivery.Runtime{ContextID: "validator", RequestedModel: "sol", RequestedReasoning: "high", RequestedProvider: "codex", ActualModel: "gpt-5.6-sol", ActualReasoning: "high", ActualProvider: "codex", ActualEffort: "high", Fallback: noFallback}}, DeliveryGroupID: "group", PrefixSafety: "safe"}
}

func noFallbackProfile() *delivery.Fallback {
	allowed, used := false, false
	return &delivery.Fallback{Allowed: &allowed, Used: &used, Reason: json.RawMessage("null")}
}
