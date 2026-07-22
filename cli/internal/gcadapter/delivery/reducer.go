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
	"strings"
	"time"
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
}

type Request struct {
	Root              string
	Certificate       AdmissionCertificate
	CertificateBytes  []byte
	CertificateDigest string
	Target            Target
}

type Terminal struct {
	BeadID            string
	Ref               string
	Verdict           string
	CertificateDigest string
}

type DeliveryBead struct {
	ID          string
	ExternalRef string
	Route       string
}

type Branch struct{ Name, BaseRef, BaseOID, Head string }
type PullRequest struct{ ID, Branch, BaseRef, BaseOID, Head, EffectID string }

// Providers makes all mutable boundaries explicit. Production wiring may use
// its own caller-selected adapters; this thin slice proves only fake boundaries.
type Providers interface {
	Terminal(context.Context, string) (Terminal, error)
	FindDelivery(context.Context, string) (DeliveryBead, bool, error)
	CreateDelivery(context.Context, DeliveryBead) (DeliveryBead, error)
	PublishDelivery(context.Context, string) error
	FindBranch(context.Context, string) (Branch, bool, error)
	PrepareBranch(context.Context, Branch) (Branch, error)
	FindPR(context.Context, string) (PullRequest, bool, error)
	CreatePR(context.Context, PullRequest) (PullRequest, error)
}

type Result struct {
	Status string `json:"status"`
	Effect string `json:"effect,omitempty"`
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
		"before_prepared", "after_prepared", "before_terminal", "after_terminal",
		"before_successor_create", "after_successor_create", "before_publication", "after_publication",
		"before_committed", "after_committed", "before_branch", "after_branch", "before_pr", "after_pr",
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
	state := markerStore{root: request.Root}
	prepared := makePrepared(request)
	if result, done, err := r.ensurePrepared(state, prepared); err != nil || done {
		return result, err
	}
	if err := r.requireTerminal(ctx, request); err != nil {
		return Result{}, err
	}
	bead, result, done, err := r.ensureSuccessor(ctx, prepared)
	if err != nil || done {
		return result, err
	}
	if result, done, err := ensureNonRoutable(state, prepared, bead); err != nil || done {
		return result, err
	}
	if result, done, err := ensurePublicationPrepared(state, prepared); err != nil || done {
		return result, err
	}
	if result, done, err := r.ensureCommitted(state, prepared, request); err != nil || done {
		return result, err
	}
	if result, done, err := r.publish(ctx, prepared, bead); err != nil || done {
		return result, err
	}
	branch, result, done, err := r.ensureBranch(ctx, state, prepared, request)
	if err != nil || done {
		return result, err
	}
	return r.ensurePR(ctx, state, prepared, request, branch)
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

func (r *Reducer) ensurePrepared(state markerStore, prepared Prepared) (Result, bool, error) {
	found, err := state.read(preparedFile, &Prepared{})
	if err != nil {
		return Result{}, true, err
	}
	if found {
		return Result{}, false, state.matches(preparedFile, prepared)
	}
	if err := r.cut("before_prepared"); err != nil {
		return Result{}, true, err
	}
	if err := state.writeImmutable(preparedFile, prepared); err != nil {
		return Result{}, true, err
	}
	if err := r.cut("after_prepared"); err != nil {
		return Result{}, true, err
	}
	return Result{Status: "prepared"}, true, nil
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

func (r *Reducer) ensureSuccessor(ctx context.Context, prepared Prepared) (DeliveryBead, Result, bool, error) {
	bead, found, err := r.providers.FindDelivery(ctx, prepared.DeliveryBeadID)
	if err != nil {
		return DeliveryBead{}, Result{}, true, err
	}
	if found {
		if !matchesDelivery(bead, prepared) {
			return DeliveryBead{}, Result{}, true, errors.New("conflicting delivery bead identity")
		}
		return bead, Result{}, false, nil
	}
	if err := r.cut("before_successor_create"); err != nil {
		return DeliveryBead{}, Result{}, true, err
	}
	created, err := r.providers.CreateDelivery(ctx, DeliveryBead{ID: prepared.DeliveryBeadID, ExternalRef: prepared.ExternalRef})
	if err != nil {
		return DeliveryBead{}, Result{}, true, err
	}
	if created.ID != prepared.DeliveryBeadID || created.ExternalRef != prepared.ExternalRef || created.Route != "" {
		return DeliveryBead{}, Result{}, true, errors.New("delivery create returned a conflicting identity")
	}
	if err := r.cut("after_successor_create"); err != nil {
		return DeliveryBead{}, Result{}, true, err
	}
	return DeliveryBead{}, Result{Status: "successor_created", Effect: "beads.create"}, true, nil
}

func matchesDelivery(bead DeliveryBead, prepared Prepared) bool {
	return bead.ID == prepared.DeliveryBeadID && bead.ExternalRef == prepared.ExternalRef && (bead.Route == "" || bead.Route == "agentops.delivery")
}

func ensureNonRoutable(state markerStore, prepared Prepared, bead DeliveryBead) (Result, bool, error) {
	artifact := makeDelivery(prepared, "non_routable")
	if state.exists(nonRoutableFile) {
		return Result{}, false, state.matches(nonRoutableFile, artifact)
	}
	if bead.Route != "" {
		return Result{}, true, errors.New("expected non-routable successor")
	}
	if err := state.writeImmutable(nonRoutableFile, artifact); err != nil {
		return Result{}, true, err
	}
	return Result{Status: "non_routable_recorded"}, true, nil
}

func ensurePublicationPrepared(state markerStore, prepared Prepared) (Result, bool, error) {
	artifact := makeDelivery(prepared, "published")
	if state.exists(publishedFile) {
		return Result{}, false, state.matches(publishedFile, artifact)
	}
	if err := state.writeImmutable(publishedFile, artifact); err != nil {
		return Result{}, true, err
	}
	return Result{Status: "publication_prepared"}, true, nil
}

func (r *Reducer) ensureCommitted(state markerStore, prepared Prepared, request Request) (Result, bool, error) {
	committed := makeCommitted(prepared, state.digest(preparedFile), state.digest(publishedFile), request.Target.CommittedAt)
	if state.exists(committedFile) {
		return Result{}, false, state.matches(committedFile, committed)
	}
	if err := r.cut("before_committed"); err != nil {
		return Result{}, true, err
	}
	if err := state.writeImmutable(committedFile, committed); err != nil {
		return Result{}, true, err
	}
	if err := r.cut("after_committed"); err != nil {
		return Result{}, true, err
	}
	return Result{Status: "committed"}, true, nil
}

func (r *Reducer) publish(ctx context.Context, prepared Prepared, bead DeliveryBead) (Result, bool, error) {
	if bead.Route == "agentops.delivery" {
		return Result{}, false, nil
	}
	if bead.Route != "" {
		return Result{}, true, errors.New("published delivery has an unexpected route")
	}
	if err := r.cut("before_publication"); err != nil {
		return Result{}, true, err
	}
	if err := r.providers.PublishDelivery(ctx, prepared.DeliveryBeadID); err != nil {
		return Result{}, true, err
	}
	if err := r.cut("after_publication"); err != nil {
		return Result{}, true, err
	}
	return Result{Status: "published", Effect: "beads.publish"}, true, nil
}

func (r *Reducer) ensureBranch(ctx context.Context, state markerStore, prepared Prepared, request Request) (Branch, Result, bool, error) {
	branch := expectedBranch(prepared, request)
	stored, found, err := r.providers.FindBranch(ctx, branch.Name)
	if err != nil {
		return Branch{}, Result{}, true, err
	}
	if !found {
		return r.createBranch(ctx, state, prepared, request.Target, branch)
	}
	if stored != branch {
		return Branch{}, Result{}, true, errors.New("conflicting branch identity")
	}
	result, done, err := receiptBranch(state, prepared, request.Target, stored, "adopted")
	return stored, result, done, err
}

func expectedBranch(prepared Prepared, request Request) Branch {
	return Branch{Name: "delivery/" + prepared.HandoffID[:20], BaseRef: request.Target.BaseRef, BaseOID: request.Target.BaseOID, Head: request.Certificate.Candidate.Commit}
}

func (r *Reducer) createBranch(ctx context.Context, state markerStore, prepared Prepared, target Target, branch Branch) (Branch, Result, bool, error) {
	if err := r.cut("before_branch"); err != nil {
		return Branch{}, Result{}, true, err
	}
	created, err := r.providers.PrepareBranch(ctx, branch)
	if err != nil {
		return Branch{}, Result{}, true, err
	}
	if created != branch {
		return Branch{}, Result{}, true, errors.New("branch prepare returned a conflicting identity")
	}
	if err := r.cut("after_branch"); err != nil {
		return Branch{}, Result{}, true, err
	}
	if _, _, err := receiptBranch(state, prepared, target, created, "created"); err != nil {
		return Branch{}, Result{}, true, err
	}
	return Branch{}, Result{Status: "branch_prepared", Effect: "git.branch"}, true, nil
}

func receiptBranch(state markerStore, prepared Prepared, target Target, branch Branch, outcome string) (Result, bool, error) {
	if state.exists("receipts/branch.json") {
		ok, err := state.branchReceiptMatches("receipts/branch.json", prepared, target, branch)
		if err != nil || !ok {
			if err == nil {
				err = errors.New("branch receipt identity conflict")
			}
			return Result{}, true, err
		}
		return Result{}, false, nil
	}
	receipt, err := makeBranchReceipt(prepared, target, branch, outcome)
	if err != nil {
		return Result{}, true, err
	}
	if err := state.writeImmutable("receipts/branch.json", receipt); err != nil {
		return Result{}, true, err
	}
	return Result{Status: "branch_receipted"}, true, nil
}

func (r *Reducer) ensurePR(ctx context.Context, state markerStore, prepared Prepared, request Request, branch Branch) (Result, error) {
	pr := expectedPR(prepared, branch)
	stored, found, err := r.providers.FindPR(ctx, pr.ID)
	if err != nil {
		return Result{}, err
	}
	if !found {
		return r.createPR(ctx, state, prepared, request.Target, pr)
	}
	if stored != pr {
		return Result{}, errors.New("conflicting PR identity")
	}
	if result, done, err := receiptPR(state, prepared, request.Target, stored, "adopted"); done {
		return result, err
	}
	return Result{Status: "converged"}, nil
}

func expectedPR(prepared Prepared, branch Branch) PullRequest {
	effectID := identifier(prepared.HandoffID, "pr")
	return PullRequest{ID: "pr-" + effectID[:20], Branch: branch.Name, BaseRef: branch.BaseRef, BaseOID: branch.BaseOID, Head: branch.Head, EffectID: effectID}
}

func (r *Reducer) createPR(ctx context.Context, state markerStore, prepared Prepared, target Target, pr PullRequest) (Result, error) {
	if err := r.cut("before_pr"); err != nil {
		return Result{}, err
	}
	created, err := r.providers.CreatePR(ctx, pr)
	if err != nil {
		return Result{}, err
	}
	if created != pr {
		return Result{}, errors.New("PR create returned a conflicting identity")
	}
	if err := r.cut("after_pr"); err != nil {
		return Result{}, err
	}
	if _, _, err := receiptPR(state, prepared, target, created, "created"); err != nil {
		return Result{}, err
	}
	return Result{Status: "pr_opened", Effect: "forge.pr"}, nil
}

func receiptPR(state markerStore, prepared Prepared, target Target, pr PullRequest, outcome string) (Result, bool, error) {
	if state.exists("receipts/pr-open.json") {
		ok, err := state.prReceiptMatches("receipts/pr-open.json", prepared, target, pr)
		if err != nil || !ok {
			if err == nil {
				err = errors.New("PR receipt identity conflict")
			}
			return Result{}, true, err
		}
		return Result{}, false, nil
	}
	receipt, err := makePROpenReceipt(prepared, target, pr, outcome)
	if err != nil {
		return Result{}, true, err
	}
	if err := state.writeImmutable("receipts/pr-open.json", receipt); err != nil {
		return Result{}, true, err
	}
	return Result{Status: "pr_receipted"}, true, nil
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
	}
	for _, check := range checks {
		if err := check(request); err != nil {
			return err
		}
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
	if target.Epoch < 1 || !validMode(target.Mode) || !isTime(target.Deadline) || !isTime(target.PreparedAt) || !isTime(target.CommittedAt) {
		return errors.New("delivery epoch or mode is invalid")
	}
	return nil
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
type DeliveryArtifact struct {
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
	Outcome        string `json:"outcome"`
	ResponseDigest string `json:"response_digest"`
}
type PROpenReceipt struct {
	SchemaVersion  string `json:"schema_version"`
	HandoffID      string `json:"handoff_id"`
	Epoch          int    `json:"epoch"`
	RigID          string `json:"rig_id"`
	Repository     string `json:"repository"`
	Remote         string `json:"remote"`
	PRID           string `json:"pr_id"`
	Branch         string `json:"branch"`
	BaseRef        string `json:"base_ref"`
	BaseOID        string `json:"base_oid"`
	ExpectedHead   string `json:"expected_head"`
	EffectID       string `json:"effect_id"`
	Outcome        string `json:"outcome"`
	ResponseDigest string `json:"response_digest"`
}

func makePrepared(r Request) Prepared {
	handoff := identifier("agentops.gc.delivery.handoff.v1", r.CertificateDigest, r.Target.SemanticBeadID, r.Target.SemanticTerminalRef, r.Target.RigID, r.Target.Repository, r.Target.Remote, r.Target.BaseRef, r.Target.BaseOID, r.Target.Mode, fmt.Sprint(r.Target.Epoch))
	return Prepared{SchemaVersion: "handoff-prepared.v1", HandoffID: handoff, SemanticBeadID: r.Target.SemanticBeadID, SemanticTerminalRef: r.Target.SemanticTerminalRef, AdmissionCertificateRef: "certificate:sha256:" + r.CertificateDigest, AdmissionCertificateDigest: r.CertificateDigest, DeliveryBeadID: "delivery-" + handoff[:20] + fmt.Sprintf("-e%06d", r.Target.Epoch), ExternalRef: "handoff:" + handoff + fmt.Sprintf(":epoch:%d", r.Target.Epoch), Epoch: r.Target.Epoch, Mode: r.Target.Mode, State: "queued", Deadline: r.Target.Deadline, PreparedAt: r.Target.PreparedAt}
}
func makeDelivery(p Prepared, publication string) DeliveryArtifact {
	return DeliveryArtifact{SchemaVersion: "delivery.v1", Kind: "delivery", HandoffID: p.HandoffID, SemanticBeadID: p.SemanticBeadID, SemanticTerminalRef: p.SemanticTerminalRef, AdmissionCertificateDigest: p.AdmissionCertificateDigest, DeliveryBeadID: p.DeliveryBeadID, ExternalRef: p.ExternalRef, Epoch: p.Epoch, Mode: p.Mode, State: p.State, Publication: publication, Deadline: p.Deadline}
}
func makeCommitted(p Prepared, preparedDigest, publishedDigest, committedAt string) Committed {
	return Committed{SchemaVersion: "handoff-committed.v1", HandoffID: p.HandoffID, PreparedDigest: preparedDigest, SemanticBeadID: p.SemanticBeadID, SemanticTerminalVerdict: "PASS", SemanticTerminalRef: p.SemanticTerminalRef, AdmissionCertificateDigest: p.AdmissionCertificateDigest, DeliveryBeadID: p.DeliveryBeadID, ExternalRef: p.ExternalRef, Epoch: p.Epoch, DeliveryPayloadRef: publishedFile, DeliveryPayloadDigest: publishedDigest, Mode: p.Mode, State: p.State, Deadline: p.Deadline, CommittedAt: committedAt}
}
func makeBranchReceipt(p Prepared, target Target, branch Branch, outcome string) (BranchReceipt, error) {
	digest, err := valueDigest(branch)
	if err != nil {
		return BranchReceipt{}, err
	}
	return BranchReceipt{SchemaVersion: "branch-receipt.v1", HandoffID: p.HandoffID, Epoch: p.Epoch, RigID: target.RigID, Repository: target.Repository, Remote: target.Remote, Branch: branch.Name, BaseRef: branch.BaseRef, BaseOID: branch.BaseOID, ExpectedHead: branch.Head, Outcome: outcome, ResponseDigest: digest}, nil
}
func makePROpenReceipt(p Prepared, target Target, pr PullRequest, outcome string) (PROpenReceipt, error) {
	digest, err := valueDigest(pr)
	if err != nil {
		return PROpenReceipt{}, err
	}
	return PROpenReceipt{SchemaVersion: "pr-open-receipt.v1", HandoffID: p.HandoffID, Epoch: p.Epoch, RigID: target.RigID, Repository: target.Repository, Remote: target.Remote, PRID: pr.ID, Branch: pr.Branch, BaseRef: pr.BaseRef, BaseOID: pr.BaseOID, ExpectedHead: pr.Head, EffectID: pr.EffectID, Outcome: outcome, ResponseDigest: digest}, nil
}
func valueDigest(value any) (string, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:]), nil
}

type markerStore struct{ root string }

func (s markerStore) path(name string) string { return filepath.Join(s.root, name) }
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
	digest, err := valueDigest(branch)
	if err != nil {
		return false, err
	}
	return receipt.SchemaVersion == "branch-receipt.v1" && (receipt.Outcome == "created" || receipt.Outcome == "adopted") && receipt.HandoffID == prepared.HandoffID && receipt.Epoch == prepared.Epoch && receipt.RigID == target.RigID && receipt.Repository == target.Repository && receipt.Remote == target.Remote && receipt.Branch == branch.Name && receipt.BaseRef == branch.BaseRef && receipt.BaseOID == branch.BaseOID && receipt.ExpectedHead == branch.Head && receipt.ResponseDigest == digest, nil
}
func (s markerStore) prReceiptMatches(name string, prepared Prepared, target Target, pr PullRequest) (bool, error) {
	var receipt PROpenReceipt
	found, err := s.read(name, &receipt)
	if err != nil || !found {
		return false, err
	}
	digest, err := valueDigest(pr)
	if err != nil {
		return false, err
	}
	return receipt.SchemaVersion == "pr-open-receipt.v1" && (receipt.Outcome == "created" || receipt.Outcome == "adopted") && receipt.HandoffID == prepared.HandoffID && receipt.Epoch == prepared.Epoch && receipt.RigID == target.RigID && receipt.Repository == target.Repository && receipt.Remote == target.Remote && receipt.PRID == pr.ID && receipt.Branch == pr.Branch && receipt.BaseRef == pr.BaseRef && receipt.BaseOID == pr.BaseOID && receipt.ExpectedHead == pr.Head && receipt.EffectID == pr.EffectID && receipt.ResponseDigest == digest, nil
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
