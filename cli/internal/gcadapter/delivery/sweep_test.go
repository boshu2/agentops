package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

type sweepTestSnapshot []ReadyDelivery

func (s sweepTestSnapshot) ReadyDeliveries(context.Context, int) ([]ReadyDelivery, error) {
	return append([]ReadyDelivery(nil), s...), nil
}

type sweepTestProviders struct {
	beads     map[string]DeliveryBead
	terminals map[string]Terminal
	calls     []string
	mutations int
}

func (p *sweepTestProviders) Terminal(_ context.Context, id string) (Terminal, error) {
	p.calls = append(p.calls, id)
	terminal, ok := p.terminals[id]
	if !ok {
		return Terminal{}, errors.New("missing terminal")
	}
	return terminal, nil
}
func (p *sweepTestProviders) FindDelivery(_ context.Context, id string) (DeliveryBead, bool, error) {
	value, ok := p.beads[id]
	return value, ok, nil
}
func (p *sweepTestProviders) CreateDelivery(_ context.Context, value DeliveryBead) (DeliveryBead, error) {
	p.mutations++
	p.beads[value.ID] = value
	return value, nil
}
func (p *sweepTestProviders) PublishRoute(context.Context, string) error { p.mutations++; return nil }
func (p *sweepTestProviders) RetireRoute(context.Context, string) error  { p.mutations++; return nil }
func (p *sweepTestProviders) StoreTransition(_ context.Context, observed DeliveryBead, next DeliveryRecord) (DeliveryBead, error) {
	p.mutations++
	observed.Record = next
	p.beads[observed.ID] = observed
	return observed, nil
}
func (p *sweepTestProviders) BaseDescends(context.Context, string, string) (bool, error) {
	return true, nil
}
func (p *sweepTestProviders) ObserveBase(_ context.Context, _ string) (string, error) {
	return strings.Repeat("d", 40), nil
}
func (p *sweepTestProviders) FindBranch(context.Context, string) (Branch, bool, error) {
	return Branch{}, false, nil
}
func (p *sweepTestProviders) PrepareBranch(context.Context, Branch) (Branch, error) {
	return Branch{}, errors.New("unexpected step after preflight")
}

func TestSweepPreflightUsesEachHandoffRequest(t *testing.T) {
	root := t.TempDir()
	first, firstBead := writeSweepRequest(t, root, "semantic-one", "2026-07-21T00:00:00Z")
	second, secondBead := writeSweepRequest(t, root, "semantic-two", "2026-07-21T00:00:01Z")
	provider := &sweepTestProviders{beads: map[string]DeliveryBead{first.ID: firstBead, second.ID: secondBead}, terminals: map[string]Terminal{
		"semantic-one": terminalForSweep(firstBead), "semantic-two": terminalForSweep(secondBead),
	}}
	results, err := SweepReady(context.Background(), provider, sweepTestSnapshot{second, first}, SweepController{Root: root, RigID: "rig-a", Repository: "boshu2/agentops", Remote: "origin"})
	if err != nil || len(results) != 2 {
		t.Fatalf("sweep = %#v, %v", results, err)
	}
	if len(provider.calls) < 2 || strings.Join(provider.calls[:2], ",") != "semantic-one,semantic-two" {
		t.Fatalf("terminal preflight calls = %q; requests were cross-bound", strings.Join(provider.calls, ","))
	}
}

func TestSweepPreflightFailurePrecedesEveryMutation(t *testing.T) {
	root := t.TempDir()
	first, firstBead := writeSweepRequest(t, root, "semantic-one", "2026-07-21T00:00:00Z")
	broken, brokenBead := writeSweepRequest(t, root, "semantic-two", "2026-07-21T00:00:01Z")
	broken.RequestDigest = strings.Repeat("0", 64)
	provider := &sweepTestProviders{beads: map[string]DeliveryBead{first.ID: firstBead, broken.ID: brokenBead}, terminals: map[string]Terminal{
		"semantic-one": terminalForSweep(firstBead), "semantic-two": terminalForSweep(brokenBead),
	}}
	if _, err := SweepReady(context.Background(), provider, sweepTestSnapshot{first, broken}, SweepController{Root: root, RigID: "rig-a", Repository: "boshu2/agentops", Remote: "origin"}); err == nil {
		t.Fatal("sweep accepted a request with the wrong digest")
	}
	if provider.mutations != 0 {
		t.Fatalf("preflight failure performed %d mutations", provider.mutations)
	}
}

func writeSweepRequest(t *testing.T, root, semantic, readyAt string) (ReadyDelivery, DeliveryBead) {
	t.Helper()
	manifest := SubjectManifest{SchemaVersion: "subject-manifest.v1", DeclaredRoots: []string{"."}, Entries: []ManifestEntry{}}
	identity, err := verdictcheck.CanonicalJSON(map[string]any{"schema_version": manifest.SchemaVersion, "declared_roots": manifest.DeclaredRoots, "exclusions": manifest.Exclusions, "entries": manifest.Entries})
	if err != nil {
		t.Fatal(err)
	}
	manifestSum := sha256.Sum256(identity)
	manifest.CanonicalManifestDigest = hex.EncodeToString(manifestSum[:])
	subjectBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	native := NativeContext{SchemaVersion: "gc-delivery-native-context.v1", RigID: "rig-a", Repository: "boshu2/agentops", RepositoryDir: "/repo", WorktreeRoot: "/worktrees", BeadsDir: "/beads", Remote: "origin", BaseRef: "main", SuccessorCapability: strings.Repeat("a", 64), ToolchainLock: strings.Repeat("b", 64), ToolchainReceipt: "/receipt", ToolchainReceiptSum: strings.Repeat("c", 64), BeadsRepresentation: "B-successor-delivery-bead", Executables: map[string]ExecutableBinding{"gc": {Path: "/gc", Digest: strings.Repeat("1", 64)}, "bd": {Path: "/bd", Digest: strings.Repeat("2", 64)}, "git": {Path: "/git", Digest: strings.Repeat("3", 64)}, "gh": {Path: "/gh", Digest: strings.Repeat("4", 64)}, "bash": {Path: "/bash", Digest: strings.Repeat("5", 64)}, "agentops-gc-delivery": {Path: "/delivery", Digest: strings.Repeat("6", 64)}}, CheckOnlyGateArgv: [][]string{{"/bash", "check"}}}
	nativeBytes, err := json.Marshal(native)
	if err != nil {
		t.Fatal(err)
	}
	certificate := AdmissionCertificate{SchemaVersion: "admission-certificate.v2", SemanticBeadID: semantic, IntentDigest: strings.Repeat("a", 64), Verdict: "PASS", Candidate: Candidate{Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), ContentDigest: strings.Repeat("c", 64)}, Store: Store{Identity: "beads", Digest: strings.Repeat("d", 64)}, ChangedPathManifest: manifest.CanonicalManifestDigest, VerdictDigest: strings.Repeat("f", 64), EvidenceDigest: strings.Repeat("0", 64), Attestations: sweepAttestations(), DeliveryGroupID: "group", PrefixSafety: "safe"}
	certificateBytes, err := json.Marshal(certificate)
	if err != nil {
		t.Fatal(err)
	}
	certificateDigest := digestSweep(certificateBytes)
	request := Request{Root: root, Certificate: certificate, CertificateBytes: certificateBytes, CertificateDigest: certificateDigest, SubjectManifest: manifest, SubjectBytes: subjectBytes, SubjectDigest: manifest.CanonicalManifestDigest, NativeContext: native, NativeBytes: nativeBytes, NativeDigest: digestSweep(nativeBytes), Target: Target{SemanticBeadID: semantic, SemanticTerminalRef: "beads:" + semantic + "#terminal", RigID: "rig-a", Repository: "boshu2/agentops", Remote: "origin", Epoch: 1, Mode: "auto", Deadline: "2026-07-22T00:00:00Z", PreparedAt: readyAt, CommittedAt: "2026-07-21T00:00:02Z", BaseRef: "main", BaseOID: strings.Repeat("d", 40)}}
	prepared := makePrepared(request)
	bead := DeliveryBead{ID: prepared.DeliveryBeadID, ExternalRef: prepared.ExternalRef, Record: initialDeliveryRecord(prepared, request)}
	prefix := filepath.Join("handoffs", prepared.HandoffID)
	writeSweepBytes(t, root, filepath.Join(prefix, "certificate.json"), certificateBytes)
	writeSweepBytes(t, root, filepath.Join(prefix, "subject-manifest.json"), subjectBytes)
	writeSweepBytes(t, root, filepath.Join(prefix, "native-context.json"), nativeBytes)
	wire := deliveryRequestFor(bead.Record, request)
	requestBytes, err := verdictcheck.CanonicalJSON(wire)
	if err != nil {
		t.Fatal(err)
	}
	path := requestRefPath(bead.Record)
	writeSweepBytes(t, root, path, requestBytes)
	return ReadyDelivery{ID: bead.ID, ReadyAt: readyAt, RequestPath: path, RequestDigest: digestSweep(requestBytes)}, bead
}

func sweepAttestations() Attestations {
	allowed, used := false, false
	noFallback := &Fallback{Allowed: &allowed, Used: &used, Reason: json.RawMessage("null")}
	return Attestations{Author: Runtime{ContextID: "author", RequestedModel: "terra", RequestedReasoning: "high", RequestedProvider: "codex", ActualModel: "gpt-5.6-terra", ActualReasoning: "high", ActualProvider: "codex", ActualEffort: "high", Fallback: noFallback}, Validator: Runtime{ContextID: "validator", RequestedModel: "sol", RequestedReasoning: "high", RequestedProvider: "codex", ActualModel: "gpt-5.6-sol", ActualReasoning: "high", ActualProvider: "codex", ActualEffort: "high", Fallback: noFallback}}
}

func writeSweepBytes(t *testing.T, root, ref string, value []byte) {
	t.Helper()
	path, err := evidencePath(root, ref)
	if err != nil {
		t.Fatal(err)
	}
	state := markerStore{root: root, prefix: filepath.Dir(ref)}
	if err := state.writeBytesImmutable(filepath.Base(path), value); err != nil {
		t.Fatal(err)
	}
}

func digestSweep(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func terminalForSweep(bead DeliveryBead) Terminal {
	return Terminal{BeadID: bead.Record.SemanticBead, Ref: bead.Record.TerminalRef, Verdict: "PASS", CertificateDigest: bead.Record.Certificate}
}
