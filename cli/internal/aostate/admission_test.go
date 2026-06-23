package aostate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAdmissionCoreAdmitsReviewedCandidate(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	candidatePath, verdictPath, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, nil, nil)

	report, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, AdmissionRequest{
		CandidatePath: candidatePath,
		VerdictPath:   verdictPath,
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}, admissionTestOptions(tmp, true))
	if err != nil {
		t.Fatalf("AdmitFinding(valid): %v", err)
	}
	if report.Verdict != "ADMITTED" || !report.Wrote {
		t.Fatalf("report = %+v, want ADMITTED write", report)
	}
	acceptedPath := filepath.Join(tmp, ".ao", "accepted", "findings", "finding-age-membrane-valid.json")
	if _, err := os.Stat(acceptedPath); err != nil {
		t.Fatalf("accepted finding not written: %v", err)
	}
	ledgerPath := filepath.Join(tmp, ".ao", "admissions", "ledger.jsonl")
	ledger, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("ledger not written: %v", err)
	}
	if !strings.Contains(string(ledger), `"kind":"ao_state_admission_ledger_row"`) {
		t.Fatalf("ledger missing row: %s", ledger)
	}

	verify, err := VerifyRepo(context.Background(), tmp)
	if err != nil {
		t.Fatalf("VerifyRepo: %v", err)
	}
	if verify.Verdict != "PASS" {
		t.Fatalf("VerifyRepo verdict = %s failures=%v", verify.Verdict, verify.Failures)
	}
	if verify.AcceptedFindings != 1 || verify.LedgerRows != 1 {
		t.Fatalf("verify counts = %+v, want one accepted finding and one ledger row", verify)
	}
}

func TestAoStateAdmissionSchemasValidateFixtures(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	valid := []string{"valid-candidate.json", "valid-verdict.json"}
	for _, name := range valid {
		path := filepath.Join(tmp, "schemas", "fixtures", "ao-state", name)
		if err := ValidateStateFile(tmp, path); err != nil {
			t.Fatalf("ValidateStateFile(%s): %v", name, err)
		}
	}
	bad := []string{"bad-candidate-extra-field.json", "bad-verdict-missing-proof.json"}
	for _, name := range bad {
		path := filepath.Join(tmp, "schemas", "fixtures", "ao-state", name)
		if err := ValidateStateFile(tmp, path); err == nil {
			t.Fatalf("ValidateStateFile(%s) accepted invalid fixture", name)
		}
	}
}

func TestAdmissionCoreRejectsSelfReviewContext(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	_, _, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, nil, func(v *AdmissionVerdict) {
		v.ReviewerContext = "ctx-author-a"
	})
	_, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, AdmissionRequest{
		CandidatePath: ".ao/candidates/findings/finding-age-membrane-valid.json",
		VerdictPath:   ".ao/reviews/finding-age-membrane-valid.verdict.json",
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}, admissionTestOptions(tmp, true))
	if err == nil || !strings.Contains(err.Error(), "self-review") {
		t.Fatalf("error = %v, want self-review rejection", err)
	}
}

// Fail-closed property: a cancelled context must abort admission BEFORE anything
// is written — the membrane never admits when its caller has given up.
func TestAdmissionCoreFailsClosedOnCancelledContext(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	candidatePath, verdictPath, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE the call: a valid candidate+verdict must still be refused.

	report, err := AdmitFinding(ctx, candidateBytes, verdictBytes, AdmissionRequest{
		CandidatePath: candidatePath,
		VerdictPath:   verdictPath,
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}, admissionTestOptions(tmp, true))
	if err == nil {
		t.Fatal("AdmitFinding admitted under a cancelled context; the membrane must fail closed")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if report.Wrote {
		t.Fatal("report.Wrote is true under a cancelled context")
	}
	acceptedPath := filepath.Join(tmp, ".ao", "accepted", "findings", "finding-age-membrane-valid.json")
	if _, statErr := os.Stat(acceptedPath); statErr == nil {
		t.Fatal("a finding was written despite the cancelled context")
	}
}

// Fail-closed property: a syntactically-valid-JSON but schema-INVALID verdict
// must be refused (validateVerdictBytes), never admitted. Guards against a future
// change loosening verdict validation — the membrane's whole job is to reject
// unproven work.
func TestAdmissionCoreFailsClosedOnInvalidVerdictBytes(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	candidatePath, verdictPath, candidateBytes, _ := writeValidAdmissionFiles(t, tmp, nil, nil)

	report, err := AdmitFinding(context.Background(), candidateBytes, []byte(`{"kind":"not_a_verdict"}`), AdmissionRequest{
		CandidatePath: candidatePath,
		VerdictPath:   verdictPath,
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}, admissionTestOptions(tmp, true))
	if err == nil {
		t.Fatal("AdmitFinding accepted a schema-invalid verdict; the membrane must fail closed")
	}
	if report.Wrote {
		t.Fatal("report.Wrote is true for an invalid verdict")
	}
	acceptedPath := filepath.Join(tmp, ".ao", "accepted", "findings", "finding-age-membrane-valid.json")
	if _, statErr := os.Stat(acceptedPath); statErr == nil {
		t.Fatal("a finding was written despite an invalid verdict")
	}
}

func TestAdmissionCoreRejectsStaleDigest(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	_, _, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, nil, func(v *AdmissionVerdict) {
		v.CandidateDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	})
	_, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, AdmissionRequest{
		CandidatePath: ".ao/candidates/findings/finding-age-membrane-valid.json",
		VerdictPath:   ".ao/reviews/finding-age-membrane-valid.verdict.json",
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}, admissionTestOptions(tmp, true))
	if err == nil || !strings.Contains(err.Error(), "stale digest") {
		t.Fatalf("error = %v, want stale digest rejection", err)
	}
}

func TestAdmissionCoreRejectsPathEscape(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	_, _, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, nil, nil)
	_, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, AdmissionRequest{
		CandidatePath: ".ao/candidates/findings/finding-age-membrane-valid.json",
		VerdictPath:   ".ao/reviews/finding-age-membrane-valid.verdict.json",
		Destination:   ".ao/accepted/findings/../../outside.json",
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}, admissionTestOptions(tmp, true))
	if err == nil || !strings.Contains(err.Error(), "escapes accepted state") {
		t.Fatalf("error = %v, want path escape rejection", err)
	}
}

func TestAdmissionCoreRejectsDuplicateAdmit(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	_, _, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, nil, nil)
	req := AdmissionRequest{
		CandidatePath: ".ao/candidates/findings/finding-age-membrane-valid.json",
		VerdictPath:   ".ao/reviews/finding-age-membrane-valid.verdict.json",
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}
	if _, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, req, admissionTestOptions(tmp, true)); err != nil {
		t.Fatalf("first AdmitFinding: %v", err)
	}
	_, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, req, admissionTestOptions(tmp, true))
	if err == nil || !strings.Contains(err.Error(), "duplicate admit") {
		t.Fatalf("error = %v, want duplicate rejection", err)
	}
}

func TestAdmissionCoreRejectsLeak(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	_, _, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, func(c *FindingCandidate) {
		c.Body = "Never admit a raw secret like sk-1234567890abcdef into accepted state."
	}, nil)
	_, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, AdmissionRequest{
		CandidatePath: ".ao/candidates/findings/finding-age-membrane-valid.json",
		VerdictPath:   ".ao/reviews/finding-age-membrane-valid.verdict.json",
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}, admissionTestOptions(tmp, true))
	if err == nil || !strings.Contains(err.Error(), "leak detected") {
		t.Fatalf("error = %v, want leak rejection", err)
	}
}

func TestAdmissionCoreRawAgentsPromptBundleReject(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	_, _, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, func(c *FindingCandidate) {
		c.Source.Path = ".agents/prompts/raw-review-bundle.md"
	}, nil)
	_, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, AdmissionRequest{
		CandidatePath: ".ao/candidates/findings/finding-age-membrane-valid.json",
		VerdictPath:   ".ao/reviews/finding-age-membrane-valid.verdict.json",
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}, admissionTestOptions(tmp, true))
	if err == nil {
		t.Fatal("AdmitFinding accepted raw .agents prompt bundle")
	}
	acceptedPath := filepath.Join(tmp, ".ao", "accepted", "findings", "finding-age-membrane-valid.json")
	if _, statErr := os.Stat(acceptedPath); !os.IsNotExist(statErr) {
		t.Fatalf("accepted path stat = %v, want no accepted state", statErr)
	}
}

func TestAdmissionCorePreservesRedactionSourceDigest(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	_, _, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, nil, nil)
	req := AdmissionRequest{
		CandidatePath: ".ao/candidates/findings/finding-age-membrane-valid.json",
		VerdictPath:   ".ao/reviews/finding-age-membrane-valid.verdict.json",
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}
	if _, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, req, admissionTestOptions(tmp, true)); err != nil {
		t.Fatalf("AdmitFinding: %v", err)
	}
	accepted := readAcceptedFinding(t, tmp)
	if accepted.Candidate.Source.SourceDigest != "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("source_digest = %s, want redaction source digest preserved", accepted.Candidate.Source.SourceDigest)
	}
}

func TestAdmissionCoreRejectsTempWriteCrash(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	_, _, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, nil, nil)
	mustWriteFile(t, tmp, acceptedDirRel, []byte("not a directory"))
	_, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, AdmissionRequest{
		CandidatePath: ".ao/candidates/findings/finding-age-membrane-valid.json",
		VerdictPath:   ".ao/reviews/finding-age-membrane-valid.verdict.json",
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}, admissionTestOptions(tmp, true))
	if err == nil || !strings.Contains(err.Error(), "accepted destination") {
		t.Fatalf("error = %v, want accepted temp-write failure", err)
	}
	if _, statErr := os.Stat(filepath.Join(tmp, ledgerRel)); !os.IsNotExist(statErr) {
		t.Fatalf("ledger stat = %v, want no ledger write", statErr)
	}
}

func TestAdmissionCoreRejectsLedgerAppendFailure(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	_, _, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, nil, nil)
	if err := os.MkdirAll(filepath.Join(tmp, filepath.FromSlash(ledgerRel)), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, AdmissionRequest{
		CandidatePath: ".ao/candidates/findings/finding-age-membrane-valid.json",
		VerdictPath:   ".ao/reviews/finding-age-membrane-valid.verdict.json",
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}, admissionTestOptions(tmp, true))
	if err == nil || !strings.Contains(err.Error(), "prepare admission ledger") {
		t.Fatalf("error = %v, want ledger append failure", err)
	}
	acceptedPath := filepath.Join(tmp, ".ao", "accepted", "findings", "finding-age-membrane-valid.json")
	if _, statErr := os.Stat(acceptedPath); !os.IsNotExist(statErr) {
		t.Fatalf("accepted path stat = %v, want no accepted state", statErr)
	}
}

func TestAdmissionCoreRecordsSinglePaneDowngrade(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	_, _, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, nil, nil)
	req := AdmissionRequest{
		CandidatePath: ".ao/candidates/findings/finding-age-membrane-valid.json",
		VerdictPath:   ".ao/reviews/finding-age-membrane-valid.verdict.json",
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}
	if _, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, req, admissionTestOptions(tmp, true)); err != nil {
		t.Fatalf("AdmitFinding: %v", err)
	}
	accepted := readAcceptedFinding(t, tmp)
	if accepted.Execution.PaneMode != "single-pane" {
		t.Fatalf("pane_mode = %q, want single-pane", accepted.Execution.PaneMode)
	}
	if accepted.Execution.SinglePaneDowngradeReason == "" {
		t.Fatal("single-pane admission did not record downgrade reason")
	}
}

func TestAdmissionCoreCarriesWorktreeMetadata(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	_, _, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, nil, nil)
	req := AdmissionRequest{
		CandidatePath: ".ao/candidates/findings/finding-age-membrane-valid.json",
		VerdictPath:   ".ao/reviews/finding-age-membrane-valid.verdict.json",
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
		ExecutionContext: ExecutionContext{
			WorktreePath:             tmp,
			Branch:                   "feature/ao-state",
			HeadSHA:                  "0123456789abcdef",
			ReservationID:            "reservation-123",
			CanonicalBeadStateSource: "_beads/issues.jsonl",
			PaneMode:                 "multi-pane",
		},
	}
	if _, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, req, admissionTestOptions(tmp, true)); err != nil {
		t.Fatalf("AdmitFinding: %v", err)
	}
	accepted := readAcceptedFinding(t, tmp)
	if accepted.Execution.BeadID != "age-membrane-memory-arch-tz2s.2.8.2" {
		t.Fatalf("bead_id = %q, want candidate bead id", accepted.Execution.BeadID)
	}
	if accepted.Execution.WorktreePath != tmp || accepted.Execution.Branch != "feature/ao-state" || accepted.Execution.PaneMode != "multi-pane" {
		t.Fatalf("execution context = %+v, want supplied worktree metadata", accepted.Execution)
	}
}

func TestVerifyRepoRejectsProofRefMissing(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	_, _, candidateBytes, verdictBytes := writeValidAdmissionFiles(t, tmp, nil, nil)
	req := AdmissionRequest{
		CandidatePath: ".ao/candidates/findings/finding-age-membrane-valid.json",
		VerdictPath:   ".ao/reviews/finding-age-membrane-valid.verdict.json",
		OperatorID:    "codex:operator",
		Reason:        "unit test admission",
	}
	if _, err := AdmitFinding(context.Background(), candidateBytes, verdictBytes, req, admissionTestOptions(tmp, true)); err != nil {
		t.Fatalf("AdmitFinding: %v", err)
	}
	var row LedgerRow
	ledgerBytes, err := os.ReadFile(filepath.Join(tmp, filepath.FromSlash(ledgerRel)))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(ledgerBytes))), &row); err != nil {
		t.Fatal(err)
	}
	row.ProofRef = ""
	mutated, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	mutated = append(mutated, '\n')
	if err := os.WriteFile(filepath.Join(tmp, filepath.FromSlash(ledgerRel)), mutated, 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := VerifyRepo(context.Background(), tmp)
	if err != nil {
		t.Fatalf("VerifyRepo: %v", err)
	}
	if report.Verdict != "FAIL" {
		t.Fatalf("VerifyRepo verdict = %s, want FAIL", report.Verdict)
	}
	if !strings.Contains(strings.Join(report.Failures, "\n"), "proof_ref") {
		t.Fatalf("failures = %v, want missing proof_ref failure", report.Failures)
	}
}

func TestVerifyRepoRejectsCandidateAsAuthorityProjection(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	mustWriteFile(t, tmp, filepath.Join(".ao", "projections", "manifest.json"), []byte(`{
  "schema_version": 1,
  "kind": "ao_state_projection_manifest",
  "generated_at": "2026-06-20T00:00:00Z",
  "entries": [
    {
      "id": "finding-age-membrane-valid",
      "authority_path": ".ao/candidates/findings/finding-age-membrane-valid.json",
      "digest": "sha256:1111111111111111111111111111111111111111111111111111111111111111"
    }
  ]
}
`))
	report, err := VerifyRepo(context.Background(), tmp)
	if err != nil {
		t.Fatalf("VerifyRepo: %v", err)
	}
	if report.Verdict != "FAIL" {
		t.Fatalf("VerifyRepo verdict = %s, want FAIL", report.Verdict)
	}
	if !strings.Contains(strings.Join(report.Failures, "\n"), "candidates") {
		t.Fatalf("failures = %v, want candidate authority failure", report.Failures)
	}
}

func TestVerifyRepoRejectsLedgerMalformedAdmission(t *testing.T) {
	tmp := copyAOStateSchemasAndFixtures(t)
	mustWriteFile(t, tmp, ledgerRel, []byte(`{"schema_version":1,"kind":"ao_state_admission_ledger_row","finding_id":"finding-age-membrane-valid","candidate_digest":"sha256:1111111111111111111111111111111111111111111111111111111111111111","verdict_digest":"sha256:2222222222222222222222222222222222222222222222222222222222222222","accepted_path":".ao/candidates/findings/finding-age-membrane-valid.json","admitted_at":"2026-06-20T00:00:00Z","admitted_by":"codex:operator","reviewer_id":"codex:validator-b","reviewer_context_id":"ctx-reviewer-b","reviewer_family":"codex","evidence_ref":".agents/evidence/ao-state-plane/s2-admission-core.log","proof_ref":"docs/evidence/age-membrane-memory-arch-tz2s.2.8-pawl.md","trust_tier":"fresh-context"}`+"\n"))
	report, err := VerifyRepo(context.Background(), tmp)
	if err != nil {
		t.Fatalf("VerifyRepo: %v", err)
	}
	if report.Verdict != "FAIL" {
		t.Fatalf("VerifyRepo verdict = %s, want FAIL", report.Verdict)
	}
	if !strings.Contains(strings.Join(report.Failures, "\n"), "ledger row") {
		t.Fatalf("failures = %v, want malformed ledger failure", report.Failures)
	}
}

func writeValidAdmissionFiles(
	t *testing.T,
	root string,
	mutateCandidate func(*FindingCandidate),
	mutateVerdict func(*AdmissionVerdict),
) (string, string, []byte, []byte) {
	t.Helper()
	var candidate FindingCandidate
	if err := json.Unmarshal(readAOFixture(t, root, "valid-candidate.json"), &candidate); err != nil {
		t.Fatal(err)
	}
	if mutateCandidate != nil {
		mutateCandidate(&candidate)
	}
	candidateBytes, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	candidateBytes = append(candidateBytes, '\n')
	candidateDigest, err := CanonicalDigest(candidateBytes)
	if err != nil {
		t.Fatal(err)
	}
	var verdict AdmissionVerdict
	if err := json.Unmarshal(readAOFixture(t, root, "valid-verdict.json"), &verdict); err != nil {
		t.Fatal(err)
	}
	verdict.CandidateDigest = candidateDigest
	if mutateVerdict != nil {
		mutateVerdict(&verdict)
	}
	verdictBytes, err := json.MarshalIndent(verdict, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	verdictBytes = append(verdictBytes, '\n')
	candidatePath := filepath.ToSlash(filepath.Join(candidateDirRel, candidate.ID+".json"))
	verdictPath := ".ao/reviews/" + candidate.ID + ".verdict.json"
	mustWriteFile(t, root, candidatePath, candidateBytes)
	mustWriteFile(t, root, verdictPath, verdictBytes)
	return candidatePath, verdictPath, candidateBytes, verdictBytes
}

func admissionTestOptions(root string, write bool) AdmissionOptions {
	return AdmissionOptions{
		Root:   root,
		Now:    time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
		MaxAge: 30 * 24 * time.Hour,
		Write:  write,
	}
}

func readAOFixture(t *testing.T, root, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "schemas", "fixtures", "ao-state", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readAcceptedFinding(t *testing.T, root string) AcceptedFinding {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".ao", "accepted", "findings", "finding-age-membrane-valid.json"))
	if err != nil {
		t.Fatal(err)
	}
	var accepted AcceptedFinding
	if err := json.Unmarshal(data, &accepted); err != nil {
		t.Fatal(err)
	}
	return accepted
}

func copyAOStateSchemasAndFixtures(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	tmp := t.TempDir()
	for _, rel := range []string{
		candidateSchemaRel,
		verdictSchemaRel,
		acceptedSchemaRel,
		ledgerSchemaRel,
		manifestSchemaRel,
	} {
		copyFile(t, root, tmp, rel)
	}
	fixtures := filepath.Join(root, "schemas", "fixtures", "ao-state")
	entries, err := os.ReadDir(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		copyFile(t, root, tmp, filepath.Join("schemas", "fixtures", "ao-state", entry.Name()))
	}
	return tmp
}

func mustWriteFile(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	dest := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, fromRoot, toRoot, rel string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fromRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(toRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "schemas")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root not found")
		}
		dir = parent
	}
}
