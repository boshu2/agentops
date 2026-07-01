// Package aostate owns deterministic admission and verification for the .ao
// state plane. LLMs may author candidates and verdicts; only this package
// admits accepted state.
package aostate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	CandidateKind = "ao_state_finding_candidate"
	VerdictKind   = "ao_state_admission_verdict"
	AcceptedKind  = "ao_state_accepted_finding"
	LedgerKind    = "ao_state_admission_ledger_row"
	ManifestKind  = "ao_state_projection_manifest"

	candidateSchemaRel = "schemas/ao-state-finding-candidate.v1.schema.json"
	verdictSchemaRel   = "schemas/ao-state-admission-verdict.v1.schema.json"
	acceptedSchemaRel  = "schemas/ao-state-accepted-finding.v1.schema.json"
	ledgerSchemaRel    = "schemas/ao-state-admission-ledger.v1.schema.json"
	manifestSchemaRel  = "schemas/ao-state-projection-manifest.v1.schema.json"

	candidateDirRel = ".ao/candidates/findings"
	acceptedDirRel  = ".ao/accepted/findings"
	ledgerRel       = ".ao/admissions/ledger.jsonl"
	projectionsRel  = ".ao/projections"

	defaultMaxAge = 30 * 24 * time.Hour
)

// FindingCandidate mirrors schemas/ao-state-finding-candidate.v1.schema.json.
type FindingCandidate struct {
	SchemaVersion int             `json:"schema_version"`
	Kind          string          `json:"kind"`
	ID            string          `json:"id"`
	Title         string          `json:"title"`
	Status        string          `json:"status"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
	Source        CandidateSource `json:"source"`
	Severity      string          `json:"severity"`
	Category      string          `json:"category"`
	Summary       string          `json:"summary"`
	Evidence      []string        `json:"evidence"`
	Body          string          `json:"body"`
}

type CandidateSource struct {
	BeadID          string `json:"bead_id"`
	RunID           string `json:"run_id"`
	AuthorID        string `json:"author_id"`
	AuthorContextID string `json:"author_context_id"`
	AuthorFamily    string `json:"author_family"`
	Repo            string `json:"repo"`
	Path            string `json:"path"`
	SourceDigest    string `json:"source_digest"`
}

// AdmissionVerdict mirrors schemas/ao-state-admission-verdict.v1.schema.json.
type AdmissionVerdict struct {
	SchemaVersion   int    `json:"schema_version"`
	Kind            string `json:"kind"`
	CandidateID     string `json:"candidate_id"`
	CandidateDigest string `json:"candidate_digest"`
	ReviewerID      string `json:"reviewer_id"`
	ReviewerContext string `json:"reviewer_context_id"`
	ReviewerFamily  string `json:"reviewer_family"`
	ReviewedAt      string `json:"reviewed_at"`
	Verdict         string `json:"verdict"`
	HeadSHA         string `json:"head_sha"`
	EvidenceRef     string `json:"evidence_ref"`
	ProofRef        string `json:"proof_ref"`
	Summary         string `json:"summary"`
}

type AcceptedFinding struct {
	SchemaVersion   int              `json:"schema_version"`
	Kind            string           `json:"kind"`
	ID              string           `json:"id"`
	CandidateDigest string           `json:"candidate_digest"`
	VerdictDigest   string           `json:"verdict_digest"`
	AdmittedAt      string           `json:"admitted_at"`
	AdmittedBy      string           `json:"admitted_by"`
	CandidatePath   string           `json:"candidate_path"`
	VerdictPath     string           `json:"verdict_path"`
	TrustTier       string           `json:"trust_tier"`
	Candidate       FindingCandidate `json:"candidate"`
	Admission       AdmissionMeta    `json:"admission"`
	Execution       ExecutionContext `json:"execution_context"`
}

type AdmissionMeta struct {
	ReviewerID      string `json:"reviewer_id"`
	ReviewerContext string `json:"reviewer_context_id"`
	ReviewerFamily  string `json:"reviewer_family"`
	EvidenceRef     string `json:"evidence_ref"`
	ProofRef        string `json:"proof_ref"`
	Reason          string `json:"reason"`
}

type ExecutionContext struct {
	BeadID                    string `json:"bead_id"`
	WorktreePath              string `json:"worktree_path"`
	Branch                    string `json:"branch"`
	HeadSHA                   string `json:"head_sha"`
	ReservationID             string `json:"reservation_id,omitempty"`
	CanonicalBeadStateSource  string `json:"canonical_bead_state_source"`
	PaneMode                  string `json:"pane_mode"`
	SinglePaneDowngradeReason string `json:"single_pane_downgrade_reason,omitempty"`
}

type LedgerRow struct {
	SchemaVersion   int              `json:"schema_version"`
	Kind            string           `json:"kind"`
	FindingID       string           `json:"finding_id"`
	CandidateDigest string           `json:"candidate_digest"`
	VerdictDigest   string           `json:"verdict_digest"`
	AcceptedPath    string           `json:"accepted_path"`
	AdmittedAt      string           `json:"admitted_at"`
	AdmittedBy      string           `json:"admitted_by"`
	ReviewerID      string           `json:"reviewer_id"`
	ReviewerContext string           `json:"reviewer_context_id"`
	ReviewerFamily  string           `json:"reviewer_family"`
	EvidenceRef     string           `json:"evidence_ref"`
	ProofRef        string           `json:"proof_ref"`
	TrustTier       string           `json:"trust_tier"`
	Execution       ExecutionContext `json:"execution_context"`
}

type ProjectionManifest struct {
	SchemaVersion int               `json:"schema_version"`
	Kind          string            `json:"kind"`
	GeneratedAt   string            `json:"generated_at"`
	Entries       []ProjectionEntry `json:"entries"`
}

type ProjectionEntry struct {
	ID            string `json:"id"`
	AuthorityPath string `json:"authority_path"`
	Digest        string `json:"digest"`
}

type AdmissionRequest struct {
	CandidatePath    string
	VerdictPath      string
	Destination      string
	OperatorID       string
	Reason           string
	TrustTier        string
	ExecutionContext ExecutionContext
}

type AdmissionOptions struct {
	Root   string
	Now    time.Time
	MaxAge time.Duration
	Write  bool
}

type AdmissionReport struct {
	Verdict         string   `json:"verdict"`
	FindingID       string   `json:"finding_id"`
	CandidateDigest string   `json:"candidate_digest"`
	VerdictDigest   string   `json:"verdict_digest"`
	Destination     string   `json:"destination"`
	LedgerPath      string   `json:"ledger_path"`
	Reasons         []string `json:"reasons"`
	Wrote           bool     `json:"wrote"`
}

type VerifyReport struct {
	Verdict             string   `json:"verdict"`
	Schemas             int      `json:"schemas"`
	GoodFixtures        int      `json:"good_fixtures"`
	BadFixturesRejected int      `json:"bad_fixtures_rejected"`
	AcceptedFindings    int      `json:"accepted_findings"`
	LedgerRows          int      `json:"ledger_rows"`
	ProjectionManifests int      `json:"projection_manifests"`
	Failures            []string `json:"failures"`
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)sk-[a-z0-9]{12,}`),
	regexp.MustCompile(`(?i)aws_secret_access_key`),
	regexp.MustCompile(`(?i)(password|token|secret)\s*[:=]\s*["']?[^"'\s]{8,}`),
	regexp.MustCompile(`-----BEGIN (?:OPENSSH |RSA |EC |DSA )?PRIVATE KEY-----`),
}

// CanonicalDigest returns the sha256 digest of normalized JSON bytes.
func CanonicalDigest(data []byte) (string, error) {
	var value any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return "", fmt.Errorf("decode canonical json: %w", err)
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal canonical json: %w", err)
	}
	sum := sha256.Sum256(normalized)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func canonicalDigestOfValue(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return CanonicalDigest(data)
}

// CompileSchemaFile compiles a repo JSON schema from disk and preloads sibling
// .ao state schemas so local $ref links resolve.
func CompileSchemaFile(root, rel string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	for _, schemaRel := range []string{
		candidateSchemaRel,
		verdictSchemaRel,
		acceptedSchemaRel,
		ledgerSchemaRel,
		manifestSchemaRel,
	} {
		path := filepath.Join(root, filepath.FromSlash(schemaRel))
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read schema %s: %w", path, err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("parse schema %s: %w", path, err)
		}
		if err := compiler.AddResource(filepath.ToSlash(path), doc); err != nil {
			return nil, fmt.Errorf("add schema resource %s: %w", path, err)
		}
		idURL := "https://agentops.dev/schemas/" + filepath.Base(schemaRel)
		if err := compiler.AddResource(idURL, doc); err != nil {
			return nil, fmt.Errorf("add schema resource %s: %w", idURL, err)
		}
	}
	target := filepath.ToSlash(filepath.Join(root, filepath.FromSlash(rel)))
	schema, err := compiler.Compile(target)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", target, err)
	}
	return schema, nil
}

func ValidateJSON(schema *jsonschema.Schema, data []byte) error {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse json: %w", err)
	}
	if err := schema.Validate(inst); err != nil {
		return err
	}
	return nil
}

func ValidateStateFile(root, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var header struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	schemaRel, err := schemaRelForKind(header.Kind)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	schema, err := CompileSchemaFile(root, schemaRel)
	if err != nil {
		return err
	}
	if err := ValidateJSON(schema, data); err != nil {
		return fmt.Errorf("%s violates %s: %w", path, filepath.Base(schemaRel), err)
	}
	return nil
}

func ValidateCandidateFile(root, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read candidate: %w", err)
	}
	return validateCandidateBytes(root, data)
}

// AdmitFinding validates a candidate + independent verdict and optionally
// writes .ao/accepted plus .ao/admissions/ledger.jsonl.
func AdmitFinding(ctx context.Context, candidateBytes, verdictBytes []byte, req AdmissionRequest, opts AdmissionOptions) (AdmissionReport, error) {
	if err := ctx.Err(); err != nil {
		return AdmissionReport{}, err
	}
	root, now, maxAge := normalizeOptions(opts)
	req = normalizeAdmissionRequest(req)

	candidatePath, err := resolveCandidatePath(root, req.CandidatePath)
	if err != nil {
		return AdmissionReport{}, err
	}
	verdictPath, err := resolveRepoRelPath(root, req.VerdictPath, "verdict path")
	if err != nil {
		return AdmissionReport{}, err
	}
	if err := validateCandidateBytes(root, candidateBytes); err != nil {
		return AdmissionReport{}, err
	}
	if err := validateVerdictBytes(root, verdictBytes); err != nil {
		return AdmissionReport{}, err
	}

	var candidate FindingCandidate
	if err := json.Unmarshal(candidateBytes, &candidate); err != nil {
		return AdmissionReport{}, fmt.Errorf("decode candidate: %w", err)
	}
	var verdict AdmissionVerdict
	if err := json.Unmarshal(verdictBytes, &verdict); err != nil {
		return AdmissionReport{}, fmt.Errorf("decode verdict: %w", err)
	}
	candidateDigest, err := CanonicalDigest(candidateBytes)
	if err != nil {
		return AdmissionReport{}, err
	}
	verdictDigest, err := CanonicalDigest(verdictBytes)
	if err != nil {
		return AdmissionReport{}, err
	}
	if strings.TrimSpace(req.Destination) == "" {
		req.Destination = defaultDestinationForFinding(candidate.ID)
	}
	destRel, destAbs, err := resolveAcceptedDestination(root, req.Destination, candidate.ID)
	if err != nil {
		return AdmissionReport{}, err
	}
	if err := checkAdmission(candidateBytes, verdictBytes, candidate, verdict, candidateDigest, now, maxAge); err != nil {
		return AdmissionReport{}, err
	}
	execution := normalizeExecutionContext(root, candidate, verdict, req.ExecutionContext)
	if err := validateExecutionContext(execution); err != nil {
		return AdmissionReport{}, err
	}

	admittedAt := now.UTC().Format(time.RFC3339)
	accepted := AcceptedFinding{
		SchemaVersion:   1,
		Kind:            AcceptedKind,
		ID:              candidate.ID,
		CandidateDigest: candidateDigest,
		VerdictDigest:   verdictDigest,
		AdmittedAt:      admittedAt,
		AdmittedBy:      req.OperatorID,
		CandidatePath:   candidatePath,
		VerdictPath:     verdictPath,
		TrustTier:       req.TrustTier,
		Candidate:       candidate,
		Admission: AdmissionMeta{
			ReviewerID:      verdict.ReviewerID,
			ReviewerContext: verdict.ReviewerContext,
			ReviewerFamily:  verdict.ReviewerFamily,
			EvidenceRef:     verdict.EvidenceRef,
			ProofRef:        verdict.ProofRef,
			Reason:          req.Reason,
		},
		Execution: execution,
	}
	acceptedBytes, err := json.MarshalIndent(accepted, "", "  ")
	if err != nil {
		return AdmissionReport{}, fmt.Errorf("marshal accepted finding: %w", err)
	}
	acceptedBytes = append(acceptedBytes, '\n')
	if err := validateAcceptedBytes(root, acceptedBytes); err != nil {
		return AdmissionReport{}, err
	}

	row := LedgerRow{
		SchemaVersion:   1,
		Kind:            LedgerKind,
		FindingID:       candidate.ID,
		CandidateDigest: candidateDigest,
		VerdictDigest:   verdictDigest,
		AcceptedPath:    destRel,
		AdmittedAt:      admittedAt,
		AdmittedBy:      req.OperatorID,
		ReviewerID:      verdict.ReviewerID,
		ReviewerContext: verdict.ReviewerContext,
		ReviewerFamily:  verdict.ReviewerFamily,
		EvidenceRef:     verdict.EvidenceRef,
		ProofRef:        verdict.ProofRef,
		TrustTier:       req.TrustTier,
		Execution:       execution,
	}
	rowBytes, err := json.Marshal(row)
	if err != nil {
		return AdmissionReport{}, fmt.Errorf("marshal ledger row: %w", err)
	}
	if err := validateLedgerRowBytes(root, rowBytes); err != nil {
		return AdmissionReport{}, err
	}

	report := AdmissionReport{
		Verdict:         "ADMITTED",
		FindingID:       candidate.ID,
		CandidateDigest: candidateDigest,
		VerdictDigest:   verdictDigest,
		Destination:     destRel,
		LedgerPath:      ledgerRel,
		Reasons: []string{
			"candidate schema valid",
			"verdict schema valid",
			"candidate digest bound",
			"independent reviewer context",
			"fresh enough",
			"leak scan clean",
			"path confined",
			"execution context recorded",
		},
	}
	if !opts.Write {
		return report, nil
	}
	if err := writeAdmissionArtifacts(root, destAbs, destRel, acceptedBytes, rowBytes, candidate.ID, candidateDigest); err != nil {
		return AdmissionReport{}, err
	}
	report.Wrote = true
	return report, nil
}

// writeAdmissionArtifacts durably lands the accepted finding at destAbs and
// appends its ledger row, atomically and duplicate-guarded. It is the write
// phase extracted verbatim from AdmitFinding (behavior-identical): the caller
// invokes it only when opts.Write is set, and marks report.Wrote on success.
func writeAdmissionArtifacts(root, destAbs, destRel string, acceptedBytes, rowBytes []byte, findingID, candidateDigest string) error {
	if _, err := os.Stat(destAbs); err == nil {
		return fmt.Errorf("duplicate admit rejected: %s already exists", destRel)
	} else if !os.IsNotExist(err) {
		// In this branch err is necessarily non-nil (the `err == nil` case is
		// handled above), so a bare `!os.IsNotExist` is the real stat-error test.
		return fmt.Errorf("stat accepted destination: %w", err)
	}
	ledgerAbs := filepath.Join(root, filepath.FromSlash(ledgerRel))
	if err := ensureNoLedgerDuplicate(root, ledgerAbs, findingID, candidateDigest); err != nil {
		return fmt.Errorf("prepare admission ledger: %w", err)
	}
	acceptedTmp, cleanupAccepted, err := prepareAtomicFile(destAbs, acceptedBytes, 0o644)
	if err != nil {
		return fmt.Errorf("prepare accepted finding: %w", err)
	}
	defer cleanupAccepted()
	ledgerTmp, cleanupLedger, err := prepareLedgerAppend(ledgerAbs, rowBytes)
	if err != nil {
		return fmt.Errorf("prepare admission ledger: %w", err)
	}
	defer cleanupLedger()
	if err := os.Rename(acceptedTmp, destAbs); err != nil {
		return fmt.Errorf("write accepted finding: %w", err)
	}
	if err := os.Rename(ledgerTmp, ledgerAbs); err != nil {
		_ = os.Remove(destAbs)
		return fmt.Errorf("write admission ledger: %w", err)
	}
	return nil
}

func normalizeAdmissionRequest(req AdmissionRequest) AdmissionRequest {
	if strings.TrimSpace(req.Reason) == "" {
		req.Reason = "ao state admit"
	}
	if strings.TrimSpace(req.TrustTier) == "" {
		req.TrustTier = "fresh-context"
	}
	return req
}

func normalizeExecutionContext(root string, candidate FindingCandidate, verdict AdmissionVerdict, execution ExecutionContext) ExecutionContext {
	if strings.TrimSpace(execution.BeadID) == "" {
		execution.BeadID = candidate.Source.BeadID
	}
	if strings.TrimSpace(execution.WorktreePath) == "" {
		execution.WorktreePath = root
	}
	if strings.TrimSpace(execution.Branch) == "" {
		execution.Branch = "unknown"
	}
	if strings.TrimSpace(execution.HeadSHA) == "" {
		execution.HeadSHA = verdict.HeadSHA
	}
	if strings.TrimSpace(execution.HeadSHA) == "" {
		execution.HeadSHA = "unknown"
	}
	if strings.TrimSpace(execution.CanonicalBeadStateSource) == "" {
		execution.CanonicalBeadStateSource = "_beads/issues.jsonl"
	}
	if strings.TrimSpace(execution.PaneMode) == "" {
		execution.PaneMode = "single-pane"
	}
	if execution.PaneMode == "single-pane" && strings.TrimSpace(execution.SinglePaneDowngradeReason) == "" {
		execution.SinglePaneDowngradeReason = "no reservation metadata supplied; admission ran in single-pane mode"
	}
	return execution
}

func validateExecutionContext(execution ExecutionContext) error {
	required := map[string]string{
		"bead_id":                     execution.BeadID,
		"worktree_path":               execution.WorktreePath,
		"branch":                      execution.Branch,
		"head_sha":                    execution.HeadSHA,
		"canonical_bead_state_source": execution.CanonicalBeadStateSource,
		"pane_mode":                   execution.PaneMode,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("execution_context.%s is required", field)
		}
	}
	switch execution.PaneMode {
	case "single-pane":
		if strings.TrimSpace(execution.SinglePaneDowngradeReason) == "" {
			return fmt.Errorf("single-pane admission requires execution_context.single_pane_downgrade_reason")
		}
	case "multi-pane":
		if strings.TrimSpace(execution.ReservationID) == "" {
			return fmt.Errorf("multi-pane admission requires execution_context.reservation_id")
		}
	default:
		return fmt.Errorf("execution_context.pane_mode must be single-pane or multi-pane, got %q", execution.PaneMode)
	}
	return nil
}

func normalizeOptions(opts AdmissionOptions) (string, time.Time, time.Duration) {
	root := opts.Root
	if root == "" {
		root, _ = os.Getwd()
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	maxAge := opts.MaxAge
	if maxAge <= 0 {
		maxAge = defaultMaxAge
	}
	return root, now, maxAge
}

func validateCandidateBytes(root string, data []byte) error {
	schema, err := CompileSchemaFile(root, candidateSchemaRel)
	if err != nil {
		return err
	}
	if err := ValidateJSON(schema, data); err != nil {
		return fmt.Errorf("candidate violates %s: %w", candidateSchemaRel, err)
	}
	return nil
}

func validateVerdictBytes(root string, data []byte) error {
	schema, err := CompileSchemaFile(root, verdictSchemaRel)
	if err != nil {
		return err
	}
	if err := ValidateJSON(schema, data); err != nil {
		return fmt.Errorf("verdict violates %s: %w", verdictSchemaRel, err)
	}
	return nil
}

func validateAcceptedBytes(root string, data []byte) error {
	schema, err := CompileSchemaFile(root, acceptedSchemaRel)
	if err != nil {
		return err
	}
	if err := ValidateJSON(schema, data); err != nil {
		return fmt.Errorf("accepted finding violates %s: %w", acceptedSchemaRel, err)
	}
	return nil
}

func validateLedgerRowBytes(root string, data []byte) error {
	schema, err := CompileSchemaFile(root, ledgerSchemaRel)
	if err != nil {
		return err
	}
	if err := ValidateJSON(schema, data); err != nil {
		return fmt.Errorf("ledger row violates %s: %w", ledgerSchemaRel, err)
	}
	return nil
}

func checkAdmission(candidateRaw, verdictRaw []byte, candidate FindingCandidate, verdict AdmissionVerdict, candidateDigest string, now time.Time, maxAge time.Duration) error {
	if candidate.Kind != CandidateKind {
		return fmt.Errorf("candidate kind must be %q", CandidateKind)
	}
	if verdict.Kind != VerdictKind {
		return fmt.Errorf("verdict kind must be %q", VerdictKind)
	}
	if verdict.CandidateID != candidate.ID {
		return fmt.Errorf("verdict candidate_id %q does not match candidate id %q", verdict.CandidateID, candidate.ID)
	}
	if verdict.CandidateDigest != candidateDigest {
		return fmt.Errorf("stale digest rejected: verdict candidate_digest %q does not match %q", verdict.CandidateDigest, candidateDigest)
	}
	if normalizeID(candidate.Source.AuthorID) == normalizeID(verdict.ReviewerID) {
		return fmt.Errorf("self-review rejected: author_id and reviewer_id are both %q", candidate.Source.AuthorID)
	}
	if normalizeID(candidate.Source.AuthorContextID) == normalizeID(verdict.ReviewerContext) {
		return fmt.Errorf("self-review rejected: author_context_id and reviewer_context_id are both %q", candidate.Source.AuthorContextID)
	}
	if verdict.Verdict != "CONFIRMED" {
		return fmt.Errorf("review verdict must be CONFIRMED, got %q", verdict.Verdict)
	}
	if err := rejectStale(candidate, verdict, now, maxAge); err != nil {
		return err
	}
	if err := rejectUnsafeAgentsSource(candidate.Source.Path); err != nil {
		return err
	}
	if err := rejectLeaks(candidateRaw); err != nil {
		return err
	}
	if err := rejectLeaks(verdictRaw); err != nil {
		return err
	}
	return nil
}

func normalizeID(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func rejectStale(candidate FindingCandidate, verdict AdmissionVerdict, now time.Time, maxAge time.Duration) error {
	updated, err := time.Parse(time.RFC3339, candidate.UpdatedAt)
	if err != nil {
		return fmt.Errorf("updated_at invalid: %w", err)
	}
	reviewed, err := time.Parse(time.RFC3339, verdict.ReviewedAt)
	if err != nil {
		return fmt.Errorf("reviewed_at invalid: %w", err)
	}
	latest := updated
	if reviewed.After(latest) {
		latest = reviewed
	}
	if now.Sub(latest) > maxAge {
		return fmt.Errorf("stale finding rejected: latest timestamp %s is older than %s", latest.Format(time.RFC3339), maxAge)
	}
	if latest.After(now.Add(5 * time.Minute)) {
		return fmt.Errorf("stale finding rejected: latest timestamp %s is in the future", latest.Format(time.RFC3339))
	}
	return nil
}

func rejectUnsafeAgentsSource(path string) error {
	clean := filepath.ToSlash(filepath.Clean(path))
	if strings.HasPrefix(clean, ".agents/") &&
		!strings.HasPrefix(clean, ".agents/findings/") {
		return fmt.Errorf("raw .agents prompt bundle rejected: %s", path)
	}
	return nil
}

func rejectLeaks(raw []byte) error {
	text := string(raw)
	for _, pattern := range secretPatterns {
		if pattern.MatchString(text) {
			return fmt.Errorf("leak detected by state admission secret scan")
		}
	}
	return nil
}

func defaultDestinationForFinding(findingID string) string {
	return filepath.ToSlash(filepath.Join(acceptedDirRel, findingID+".json"))
}

func resolveCandidatePath(root, path string) (string, error) {
	rel, err := resolveRepoRelPath(root, path, "candidate path")
	if err != nil {
		return "", err
	}
	requiredPrefix := candidateDirRel + "/"
	if !strings.HasPrefix(rel, requiredPrefix) {
		return "", fmt.Errorf("candidate path must be under %s: %q", candidateDirRel, path)
	}
	return rel, nil
}

func resolveAcceptedDestination(root, dest, findingID string) (string, string, error) {
	if strings.TrimSpace(dest) == "" {
		dest = defaultDestinationForFinding(findingID)
	}
	if filepath.IsAbs(dest) {
		return "", "", fmt.Errorf("destination path escapes accepted state directory: absolute path %q", dest)
	}
	clean := filepath.ToSlash(filepath.Clean(dest))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", "", fmt.Errorf("destination path escapes accepted state directory: %q", dest)
	}
	requiredPrefix := acceptedDirRel + "/"
	if !strings.HasPrefix(clean, requiredPrefix) {
		return "", "", fmt.Errorf("destination path escapes accepted state directory: %q", dest)
	}
	if filepath.Base(clean) == "" || !strings.HasSuffix(clean, ".json") {
		return "", "", fmt.Errorf("destination must be a .json file under %s", acceptedDirRel)
	}
	abs := filepath.Join(root, filepath.FromSlash(clean))
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", "", fmt.Errorf("resolve destination: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", "", fmt.Errorf("destination path escapes repo root: %q", dest)
	}
	return clean, abs, nil
}

func resolveRepoRelPath(root, path, label string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	var abs string
	if filepath.IsAbs(path) {
		abs = filepath.Clean(path)
	} else {
		abs = filepath.Join(root, filepath.FromSlash(filepath.ToSlash(filepath.Clean(path))))
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", label, err)
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") {
		return "", fmt.Errorf("%s escapes repo root: %q", label, path)
	}
	return rel, nil
}

func prepareAtomicFile(path string, data []byte, mode os.FileMode) (string, func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-ao-state-*")
	if err != nil {
		return "", nil, err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		cleanup()
		return "", nil, err
	}
	return tmpName, cleanup, nil
}

func prepareLedgerAppend(path string, row []byte) (string, func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", nil, err
	}
	var existing []byte
	if data, err := os.ReadFile(path); err == nil {
		existing = data
	} else if !os.IsNotExist(err) {
		return "", nil, err
	}
	var next bytes.Buffer
	if len(existing) > 0 {
		next.Write(bytes.TrimRight(existing, "\n"))
		next.WriteByte('\n')
	}
	next.Write(row)
	next.WriteByte('\n')
	return prepareAtomicFile(path, next.Bytes(), 0o644)
}

func ensureNoLedgerDuplicate(root, ledgerAbs, findingID, candidateDigest string) error {
	rows, err := readLedgerRows(root, ledgerAbs)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.FindingID == findingID {
			return fmt.Errorf("duplicate admit rejected: ledger already contains finding %s", findingID)
		}
		if row.CandidateDigest == candidateDigest {
			return fmt.Errorf("duplicate admit rejected: ledger already contains candidate digest %s", candidateDigest)
		}
	}
	return nil
}

func readLedgerRows(root, ledgerAbs string) ([]LedgerRow, error) {
	data, err := os.ReadFile(ledgerAbs)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read admission ledger: %w", err)
	}
	var rows []LedgerRow
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if err := validateLedgerRowBytes(root, []byte(line)); err != nil {
			return nil, fmt.Errorf("ledger row %d invalid: %w", i+1, err)
		}
		var row LedgerRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("decode ledger row %d: %w", i+1, err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// VerifyRepo validates the .ao schemas, fixtures, accepted state, admission
// ledger, and projection authority references. Candidate files are deliberately
// not authority and are not walked unless explicitly validated by command.
func VerifyRepo(ctx context.Context, root string) (VerifyReport, error) {
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return VerifyReport{}, err
		}
	}
	report := VerifyReport{Verdict: "PASS"}
	addFailure := func(format string, args ...any) {
		report.Failures = append(report.Failures, fmt.Sprintf(format, args...))
		report.Verdict = "FAIL"
	}

	for _, schemaRel := range []string{
		candidateSchemaRel,
		verdictSchemaRel,
		acceptedSchemaRel,
		ledgerSchemaRel,
		manifestSchemaRel,
	} {
		if _, err := CompileSchemaFile(root, schemaRel); err != nil {
			addFailure("%v", err)
		} else {
			report.Schemas++
		}
	}

	validateKnownFixtures(root, &report, addFailure)
	validateAcceptedStore(ctx, root, &report, addFailure)
	validateAdmissionLedger(root, &report, addFailure)
	validateProjectionManifests(ctx, root, &report, addFailure)
	sort.Strings(report.Failures)
	return report, nil
}

func validateKnownFixtures(root string, report *VerifyReport, addFailure func(string, ...any)) {
	fixtures := filepath.Join(root, "schemas", "fixtures", "ao-state")
	entries, err := os.ReadDir(fixtures)
	if err != nil {
		addFailure("read ao-state fixtures: %v", err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()
		path := filepath.Join(fixtures, name)
		err := ValidateStateFile(root, path)
		if strings.HasPrefix(name, "bad-") {
			if err == nil {
				addFailure("bad fixture %s was accepted", name)
				continue
			}
			report.BadFixturesRejected++
			continue
		}
		if err != nil {
			addFailure("valid fixture %s rejected: %v", name, err)
			continue
		}
		report.GoodFixtures++
	}
}

func validateAcceptedStore(ctx context.Context, root string, report *VerifyReport, addFailure func(string, ...any)) {
	stateDir := filepath.Join(root, filepath.FromSlash(acceptedDirRel))
	entries, err := os.ReadDir(stateDir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		addFailure("read accepted state store: %v", err)
		return
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			addFailure("state verify canceled: %v", err)
			return
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(stateDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			addFailure("read accepted finding %s: %v", entry.Name(), err)
			continue
		}
		if err := validateAcceptedBytes(root, data); err != nil {
			addFailure("%v", err)
			continue
		}
		var accepted AcceptedFinding
		if err := json.Unmarshal(data, &accepted); err != nil {
			addFailure("decode accepted finding %s: %v", entry.Name(), err)
			continue
		}
		if err := validateAcceptedInvariant(accepted); err != nil {
			addFailure("accepted finding %s failed invariant: %v", entry.Name(), err)
			continue
		}
		report.AcceptedFindings++
	}
}

func validateAcceptedInvariant(accepted AcceptedFinding) error {
	if strings.HasPrefix(filepath.ToSlash(accepted.CandidatePath), acceptedDirRel+"/") {
		return errors.New("candidate_path cannot point at accepted state")
	}
	digest, err := canonicalDigestOfValue(accepted.Candidate)
	if err != nil {
		return err
	}
	if digest != accepted.CandidateDigest {
		return fmt.Errorf("candidate digest mismatch: accepted=%s actual=%s", accepted.CandidateDigest, digest)
	}
	return nil
}

func validateAdmissionLedger(root string, report *VerifyReport, addFailure func(string, ...any)) {
	ledgerAbs := filepath.Join(root, filepath.FromSlash(ledgerRel))
	rows, err := readLedgerRows(root, ledgerAbs)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		addFailure("%v", err)
		return
	}
	seenFindings := make(map[string]bool)
	seenDigests := make(map[string]bool)
	for _, row := range rows {
		if strings.HasPrefix(filepath.ToSlash(row.AcceptedPath), candidateDirRel+"/") {
			addFailure("ledger row for %s uses candidate as authority: %s", row.FindingID, row.AcceptedPath)
			continue
		}
		if seenFindings[row.FindingID] {
			addFailure("duplicate ledger finding id: %s", row.FindingID)
			continue
		}
		if seenDigests[row.CandidateDigest] {
			addFailure("duplicate ledger candidate digest: %s", row.CandidateDigest)
			continue
		}
		seenFindings[row.FindingID] = true
		seenDigests[row.CandidateDigest] = true
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(row.AcceptedPath))); err != nil {
			addFailure("ledger accepted_path missing for %s: %v", row.FindingID, err)
			continue
		}
		report.LedgerRows++
	}
}

func validateProjectionManifests(ctx context.Context, root string, report *VerifyReport, addFailure func(string, ...any)) {
	dir := filepath.Join(root, filepath.FromSlash(projectionsRel))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		addFailure("read state projections: %v", err)
		return
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			addFailure("state projection verify canceled: %v", err)
			return
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			addFailure("read projection manifest %s: %v", entry.Name(), err)
			continue
		}
		schema, err := CompileSchemaFile(root, manifestSchemaRel)
		if err != nil {
			addFailure("%v", err)
			continue
		}
		if err := ValidateJSON(schema, data); err != nil {
			addFailure("%s violates %s: %v", path, filepath.Base(manifestSchemaRel), err)
			continue
		}
		var manifest ProjectionManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			addFailure("decode projection manifest %s: %v", entry.Name(), err)
			continue
		}
		for _, projectionEntry := range manifest.Entries {
			if strings.HasPrefix(filepath.ToSlash(projectionEntry.AuthorityPath), candidateDirRel+"/") {
				addFailure("projection manifest %s uses candidate as authority for %s: %s", entry.Name(), projectionEntry.ID, projectionEntry.AuthorityPath)
			}
		}
		report.ProjectionManifests++
	}
}

func schemaRelForKind(kind string) (string, error) {
	switch kind {
	case CandidateKind:
		return candidateSchemaRel, nil
	case VerdictKind:
		return verdictSchemaRel, nil
	case AcceptedKind:
		return acceptedSchemaRel, nil
	case LedgerKind:
		return ledgerSchemaRel, nil
	case ManifestKind:
		return manifestSchemaRel, nil
	default:
		return "", fmt.Errorf("unknown ao state kind %q", kind)
	}
}
