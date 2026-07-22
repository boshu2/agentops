package delivery_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
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
			fake.SetPublishGuard(func() error { _, err := os.Stat(filepath.Join(trialRoot, "delivery.published.json")); return err })
			before := fake.MutationCount()
			_, err := delivery.NewReducer(fake, delivery.CrashAt(cut)).Step(context.Background(), trial)
			if got := fake.MutationCount() - before; got > 1 {
				t.Fatalf("crash invocation performed %d provider mutations", got)
			}
			if err != nil && !delivery.IsCrash(err) {
				t.Fatalf("crash step: %v", err)
			}
			for i := 0; i < 32; i++ {
				before := fake.MutationCount()
				result, runErr := delivery.NewReducer(fake, nil).Step(context.Background(), trial)
				if got := fake.MutationCount() - before; got > 1 {
					t.Fatalf("replay %d performed %d provider mutations", i, got)
				}
				if runErr != nil {
					t.Fatalf("replay %d: %v", i, runErr)
				}
				if result.Status == "converged" {
					break
				}
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
			if _, err := os.Stat(filepath.Join(trialRoot, "receipts", "pr-open.json")); err != nil {
				t.Fatalf("immutable PR receipt: %v", err)
			}
			for _, artifact := range []string{"handoff-prepared.json", "delivery.non-routable.json", "delivery.published.json", "handoff-committed.json", "receipts/branch.json", "receipts/pr-open.json"} {
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
		"rig": func(target *delivery.Target) { target.RigID = "rig-b" }, "repository": func(target *delivery.Target) { target.Repository = "other/repo" }, "remote": func(target *delivery.Target) { target.Remote = "upstream" }, "base_ref": func(target *delivery.Target) { target.BaseRef = "release" }, "base_oid": func(target *delivery.Target) { target.BaseOID = strings.Repeat("e", 40) }, "mode": func(target *delivery.Target) { target.Mode = "manual" }, "epoch": func(target *delivery.Target) { target.Epoch = 2 },
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
			if handoffID(t, left.Root) == handoffID(t, right.Root) {
				t.Fatalf("%s did not change handoff identity", name)
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
	bytes, err := os.ReadFile(filepath.Join(request.Root, "handoff-prepared.json"))
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
		if result.Status == "published" {
			reachedPublished = true
			break
		}
	}
	if !reachedPublished {
		t.Fatal("did not reach committed route publication before branch collision")
	}
	handoff := handoffID(t, request.Root)
	branchName := "delivery/" + handoff[:20]
	fixture.PutBranch(delivery.Branch{Name: branchName, BaseRef: "evil", BaseOID: strings.Repeat("d", 40), Head: strings.Repeat("a", 40)})
	if result, err := delivery.NewReducer(fixture, nil).Step(context.Background(), request); err == nil {
		t.Fatalf("branch collision was accepted at status %s for %s", result.Status, branchName)
	}

	request.Root = t.TempDir()
	fixture = delivery.NewFakeProviders(terminal)
	for i := 0; i < 16; i++ {
		result, runErr := delivery.NewReducer(fixture, nil).Step(context.Background(), request)
		if runErr != nil {
			t.Fatal(runErr)
		}
		if result.Status == "branch_prepared" {
			break
		}
	}
	if _, err := delivery.NewReducer(fixture, delivery.CrashAt("before_pr")).Step(context.Background(), request); !delivery.IsCrash(err) {
		t.Fatalf("before_pr cut = %v", err)
	}
	handoff = handoffID(t, request.Root)
	prID := "pr-" + identifierForTest(handoff, "pr")[:20]
	fixture.PutPR(delivery.PullRequest{ID: prID, Branch: "wrong", BaseRef: "main", BaseOID: strings.Repeat("d", 40), Head: strings.Repeat("a", 40), EffectID: identifierForTest(handoff, "pr")})
	if _, err := delivery.NewReducer(fixture, nil).Step(context.Background(), request); err == nil {
		t.Fatal("PR collision was accepted")
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
	for i := 0; i < 16; i++ {
		providers, openErr := delivery.OpenFixtureProviders(fixture, terminal)
		if openErr != nil {
			t.Fatal(openErr)
		}
		result, stepErr := delivery.NewReducer(providers, nil).Step(context.Background(), request)
		if stepErr != nil {
			t.Fatal(stepErr)
		}
		if result.Status == "converged" {
			return
		}
	}
	t.Fatal("fresh fixture providers did not converge")
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
		"handoff-prepared.json", "delivery.non-routable.json", "delivery.published.json", "handoff-committed.json",
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
			markerPath := filepath.Join(root, marker)
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
		name, status, path string
	}{
		{"branch", "branch_prepared", "receipts/branch.json"},
		{"pr", "pr_opened", "receipts/pr-open.json"},
	} {
		for _, schema := range []string{"missing", "wrong"} {
			t.Run(phase.name+"_"+schema, func(t *testing.T) {
				request := requestForTest(t, t.TempDir())
				fake := delivery.NewFakeProviders(terminalFor(request))
				reachStatus(t, fake, request, phase.status)
				mutateReceiptSchema(t, filepath.Join(request.Root, phase.path), schema)
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

func TestDeliveryBinaryColdProcessFixtureReplayEmitsLowercaseResult(t *testing.T) {
	certificate := certificateFor("semantic-42")
	bytes, err := json.Marshal(certificate)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	certificatePath := filepath.Join(root, "certificate.json")
	if err := os.WriteFile(certificatePath, bytes, 0o600); err != nil {
		t.Fatal(err)
	}
	cliRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "agentops-gc-delivery")
	build := exec.Command("go", "build", "-o", binary, "./cmd/agentops-gc-delivery")
	build.Dir = cliRoot
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build binary: %v: %s", buildErr, output)
	}
	fixture, evidence := filepath.Join(root, "fixture.json"), filepath.Join(root, "evidence")
	status := ""
	for i := 0; i < 16; i++ {
		command := exec.Command(binary, "step", "--root", evidence, "--certificate", certificatePath, "--semantic-bead", "semantic-42", "--terminal-ref", "beads:semantic-42#terminal", "--rig", "rig-a", "--repository", "boshu2/agentops", "--remote", "origin", "--epoch", "1", "--mode", "auto", "--deadline", "2026-07-22T00:00:00Z", "--prepared-at", "2026-07-21T00:00:00Z", "--committed-at", "2026-07-21T00:00:01Z", "--base-ref", "main", "--base-oid", strings.Repeat("d", 40), "--fake-terminal-ref", "beads:semantic-42#terminal", "--fixture-state", fixture)
		command.Dir = cliRoot
		output, runErr := command.Output()
		if runErr != nil {
			t.Fatalf("run %d: %v", i, runErr)
		}
		var result map[string]any
		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatal(err)
		}
		if _, bad := result["Status"]; bad {
			t.Fatal("binary emitted Go-cased result")
		}
		status, _ = result["status"].(string)
		if status == "converged" {
			break
		}
	}
	if status != "converged" {
		t.Fatalf("final status = %q", status)
	}
	var state struct {
		Deliveries map[string]any `json:"deliveries"`
		Branches   map[string]any `json:"branches"`
		PRs        map[string]any `json:"prs"`
	}
	stateBytes, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Deliveries) != 1 || len(state.Branches) != 1 || len(state.PRs) != 1 {
		t.Fatalf("fixture final state deliveries=%d branches=%d prs=%d", len(state.Deliveries), len(state.Branches), len(state.PRs))
	}
}

func handoffID(t *testing.T, root string) string {
	t.Helper()
	bytes, err := os.ReadFile(filepath.Join(root, "handoff-prepared.json"))
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
