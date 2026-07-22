package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

// ReadySnapshotter is deliberately narrow.  A sweep is one bounded snapshot,
// not a scheduler: it receives at most eight already-ready Beads leaves and
// performs at most one reducer transition for each snapshot member.
type ReadySnapshotter interface {
	ReadyDeliveries(context.Context, int) ([]ReadyDelivery, error)
}

type SweepResult struct {
	DeliveryBeadID string `json:"delivery_bead_id"`
	Status         string `json:"status"`
	Effect         string `json:"effect,omitempty"`
}

// SweepController is deliberately limited to rig-owned configuration.  The
// selected delivery Bead owns every delivery-specific fact through its exact
// request reference; a sweep must never receive a reusable delivery Request.
type SweepController struct {
	Root       string
	RigID      string
	Repository string
	Remote     string
}

// deliveryRequestRef is the small immutable Beads metadata envelope.  The
// referenced request is canonical JSON beneath the controller-owned evidence
// root and is content-addressed before it is used.
type deliveryRequestRef struct {
	SchemaVersion string `json:"schema_version"`
	Path          string `json:"path"`
	Digest        string `json:"digest"`
}

// deliveryRequest is the per-delivery handoff input.  It deliberately carries
// every fact the reducer needs so a snapshot member cannot borrow facts from a
// different member of the same sweep.
type deliveryRequest struct {
	SchemaVersion       string `json:"schema_version"`
	CertificateRef      string `json:"admission_certificate_ref"`
	CertificateDigest   string `json:"admission_certificate_digest"`
	SubjectRef          string `json:"subject_manifest_ref"`
	SubjectDigest       string `json:"subject_manifest_digest"`
	NativeRef           string `json:"native_context_ref"`
	NativeDigest        string `json:"native_context_digest"`
	SemanticBeadID      string `json:"semantic_bead_id"`
	SemanticTerminalRef string `json:"semantic_terminal_ref"`
	RigID               string `json:"rig_id"`
	Repository          string `json:"repository"`
	Remote              string `json:"remote"`
	Epoch               int    `json:"epoch"`
	Mode                string `json:"mode"`
	Deadline            string `json:"deadline"`
	PreparedAt          string `json:"prepared_at"`
	CommittedAt         string `json:"committed_at"`
	BaseRef             string `json:"base_ref"`
	BaseOID             string `json:"base_oid"`
}

func requestRefPath(record DeliveryRecord) string {
	return filepath.Join("handoffs", record.HandoffID, "epochs", fmt.Sprintf("%06d", record.Epoch.Number), "delivery-request.json")
}

func evidencePath(root, ref string) (string, error) {
	if root == "" || !filepath.IsAbs(root) || !safeRelativePath(ref, false) {
		return "", errors.New("delivery request reference is outside the evidence root")
	}
	path := filepath.Join(root, ref)
	rel, err := filepath.Rel(root, path)
	if err != nil || !safeRelativePath(rel, false) {
		return "", errors.New("delivery request reference escapes the evidence root")
	}
	return path, nil
}

func requestFromReference(root string, item ReadyDelivery) (Request, error) {
	wire, err := readExactDeliveryRequest(root, item)
	if err != nil {
		return Request{}, err
	}
	certificatePath, err := evidencePath(root, wire.CertificateRef)
	if err != nil {
		return Request{}, err
	}
	certificateBytes, err := os.ReadFile(certificatePath)
	if err != nil {
		return Request{}, err
	}
	certificate, err := decodeCertificate(certificateBytes)
	if err != nil {
		return Request{}, err
	}
	certificateSum := sha256.Sum256(certificateBytes)
	if wire.CertificateDigest != hex.EncodeToString(certificateSum[:]) {
		return Request{}, errors.New("delivery request certificate digest does not match exact bytes")
	}
	subjectPath, err := evidencePath(root, wire.SubjectRef)
	if err != nil {
		return Request{}, err
	}
	subject, subjectBytes, err := ReadExactSubjectManifest(subjectPath, wire.SubjectDigest)
	if err != nil {
		return Request{}, err
	}
	nativePath, err := evidencePath(root, wire.NativeRef)
	if err != nil {
		return Request{}, err
	}
	native, nativeBytes, err := ReadExactNativeContext(nativePath, wire.NativeDigest)
	if err != nil {
		return Request{}, err
	}
	request := Request{Root: root, Certificate: certificate, CertificateBytes: certificateBytes, CertificateDigest: wire.CertificateDigest, Target: Target{DeliveryBeadID: item.ID, SemanticBeadID: wire.SemanticBeadID, SemanticTerminalRef: wire.SemanticTerminalRef, RigID: wire.RigID, Repository: wire.Repository, Remote: wire.Remote, Epoch: wire.Epoch, Mode: wire.Mode, Deadline: wire.Deadline, PreparedAt: wire.PreparedAt, CommittedAt: wire.CommittedAt, BaseRef: wire.BaseRef, BaseOID: wire.BaseOID}, SubjectManifest: subject, SubjectBytes: subjectBytes, SubjectDigest: wire.SubjectDigest, NativeContext: native, NativeBytes: nativeBytes, NativeDigest: wire.NativeDigest}
	return exactRequest(request)
}

func readExactDeliveryRequest(root string, item ReadyDelivery) (deliveryRequest, error) {
	path, err := evidencePath(root, item.RequestPath)
	if err != nil {
		return deliveryRequest{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return deliveryRequest{}, err
	}
	sum := sha256.Sum256(raw)
	if item.RequestDigest != hex.EncodeToString(sum[:]) {
		return deliveryRequest{}, errors.New("delivery request digest does not match exact bytes")
	}
	var wire deliveryRequest
	if err := decodeStrict(raw, &wire); err != nil {
		return deliveryRequest{}, err
	}
	canonical, err := jsonCanonical(wire)
	if err != nil || string(canonical) != string(raw) || wire.SchemaVersion != "delivery-request.v1" {
		return deliveryRequest{}, errors.New("delivery request is not canonical delivery-request.v1")
	}
	return wire, nil
}

// jsonCanonical is kept here so both request emission and loading use the
// exact same canonical byte contract without a second JSON implementation.
func jsonCanonical(value any) ([]byte, error) { return verdictcheck.CanonicalJSON(value) }

type sweepFailureRecorder interface {
	RecordSweepFailure(context.Context, string, string, error) error
}

// SweepReady advances the immutable initial snapshot only.  Work made ready
// by an earlier Step is intentionally invisible until a later cooldown.
func SweepReady(ctx context.Context, providers Providers, snapshotter ReadySnapshotter, controller SweepController) ([]SweepResult, error) {
	if err := validateSweepController(controller); err != nil {
		return nil, err
	}
	ready, err := snapshotter.ReadyDeliveries(ctx, 8)
	if err != nil {
		return nil, err
	}
	sortReadyDeliveries(ready)
	requests, err := preflightSweepRequests(ctx, providers, controller, ready)
	if err != nil {
		return nil, err
	}
	return advanceSweepSnapshot(ctx, providers, ready, requests)
}

func validateSweepController(controller SweepController) error {
	if controller.Root == "" || !filepath.IsAbs(controller.Root) || controller.RigID == "" || controller.Repository == "" || controller.Remote == "" {
		return errors.New("delivery sweep requires controller-owned root, rig, repository, and remote")
	}
	return nil
}

func sortReadyDeliveries(ready []ReadyDelivery) {
	sort.SliceStable(ready, func(i, j int) bool {
		if ready[i].ReadyAt != ready[j].ReadyAt {
			return ready[i].ReadyAt < ready[j].ReadyAt
		}
		return ready[i].ID < ready[j].ID
	})
}

func preflightSweepRequests(ctx context.Context, providers Providers, controller SweepController, ready []ReadyDelivery) ([]Request, error) {
	requests := make([]Request, len(ready))
	// Validate the complete immutable snapshot before any reducer step can make
	// one Beads/Git/forge mutation.
	for index, item := range ready {
		request, err := preflightSweepRequest(ctx, providers, controller, item)
		if err != nil {
			return nil, recordSweepPreflightFailure(ctx, providers, controller.Root, item.ID, err)
		}
		requests[index] = request
	}
	return requests, nil
}

func preflightSweepRequest(ctx context.Context, providers Providers, controller SweepController, item ReadyDelivery) (Request, error) {
	if item.ID == "" || item.RequestPath == "" || !isHex(item.RequestDigest, 64) {
		return Request{}, errors.New("delivery sweep snapshot contains an invalid request reference")
	}
	request, err := requestFromReference(controller.Root, item)
	if err != nil {
		return Request{}, err
	}
	if request.Target.RigID != controller.RigID || request.Target.Repository != controller.Repository || request.Target.Remote != controller.Remote {
		return Request{}, errors.New("delivery request is outside controller-owned rig or repository")
	}
	bead, found, err := providers.FindDelivery(ctx, item.ID)
	if err != nil || !found || !matchesDeliveryRecord(bead, makePrepared(request), request) {
		return Request{}, errors.New("delivery request does not exactly bind the selected bead")
	}
	terminal, err := providers.Terminal(ctx, request.Target.SemanticBeadID)
	if err != nil || !matchesTerminal(terminal, request) {
		return Request{}, errors.New("delivery request terminal does not exactly bind PASS evidence")
	}
	return request, nil
}

func recordSweepPreflightFailure(ctx context.Context, providers Providers, root, beadID string, failure error) error {
	recorder, ok := providers.(sweepFailureRecorder)
	if !ok {
		return failure
	}
	if err := recorder.RecordSweepFailure(ctx, root, beadID, failure); err != nil {
		return err
	}
	return failure
}

func advanceSweepSnapshot(ctx context.Context, providers Providers, ready []ReadyDelivery, requests []Request) ([]SweepResult, error) {
	results := make([]SweepResult, 0, len(ready))
	for index, item := range ready {
		result, err := NewReducer(providers, nil).Step(ctx, requests[index])
		if err != nil {
			return results, err // fail closed; do not advance later snapshot members
		}
		results = append(results, SweepResult{DeliveryBeadID: item.ID, Status: result.Status, Effect: result.Effect})
	}
	return results, nil
}
