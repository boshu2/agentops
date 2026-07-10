package ports

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeHashedFixture(t *testing.T, dir, name, contents string) (string, string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(contents))
	return path, fmt.Sprintf("%x", sum)
}

func TestReviewRequestV1RejectsMutableOrSelfReview(t *testing.T) {
	dir := t.TempDir()
	contract, contractSHA := writeHashedFixture(t, dir, "contract.txt", "Given x\n")
	diff, diffSHA := writeHashedFixture(t, dir, "diff.patch", "+ change\n")
	base := ReviewRequestV1{
		SchemaVersion: "review-request.v1",
		SubjectID:     "age-1", HeadSHA: "deadbeef", AcceptanceContract: contract,
		AcceptanceContractSHA256: contractSHA,
		DiffPath:                 diff, DiffSHA256: diffSHA, AuthorContextID: "author-1", AuthorFamily: "gpt",
		DiversityMode: "fresh-context", Nonce: "n1", EvidenceDir: dir,
		ReadOnly: true,
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	bad := base
	bad.ReadOnly = false
	if err := bad.Validate(); err == nil {
		t.Fatal("mutable reviewer request should fail")
	}
	bad = base
	bad.Nonce = ""
	if err := bad.Validate(); err == nil {
		t.Fatal("nonce-less reviewer request should fail")
	}
	bad = base
	bad.DiffSHA256 = ""
	if err := bad.Validate(); err == nil {
		t.Fatal("mutable path without a content digest should fail")
	}
	if err := os.WriteFile(diff, []byte("+ changed after binding\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := base.Validate(); err == nil {
		t.Fatal("content changed after digest binding should fail")
	}
	if err := os.WriteFile(diff, []byte("+ change\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("restored bound request: %v", err)
	}

	evidence, _ := writeHashedFixture(t, dir, "self-review.txt", "reviewed files: 1\nx.go:1\n")
	result := ReviewLaneResultV1{SchemaVersion: "review-lane-result.v1", LaneID: "lane-1", Family: "gpt", ContextID: "author-1", Disposition: ReviewConfirmed, EvidencePath: evidence, Nonce: "n1", ReadOnly: true}
	if err := result.ValidateAgainst(base); err == nil {
		t.Fatal("same-author context must not count as independent review")
	}
}

func TestReviewLaneResultV1SeparatesTransportFromSemanticFailure(t *testing.T) {
	evidenceDir := t.TempDir()
	contract, contractSHA := writeHashedFixture(t, evidenceDir, "contract.txt", "Given x\n")
	diff, diffSHA := writeHashedFixture(t, evidenceDir, "diff.patch", "+ change\n")
	evidencePath := filepath.Join(evidenceDir, "review-1.txt")
	if err := os.WriteFile(evidencePath, []byte("reviewed files: 3\nx.go:1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := ReviewRequestV1{
		SchemaVersion: "review-request.v1", SubjectID: "age-1", HeadSHA: "deadbeef",
		AcceptanceContract: contract, AcceptanceContractSHA256: contractSHA,
		DiffPath: diff, DiffSHA256: diffSHA, AuthorContextID: "author-1",
		AuthorFamily: "claude", DiversityMode: "fresh-context", Nonce: "n1", EvidenceDir: evidenceDir, ReadOnly: true,
	}
	transport := ReviewLaneResultV1{
		SchemaVersion: "review-lane-result.v1", LaneID: "ntm:session:1",
		Family: "gpt", ContextID: "review-1", FailureClass: ReviewFailureTransport,
		FailureReason: "provider unreachable", Nonce: "n1",
	}
	if err := transport.Validate(); err != nil {
		t.Fatalf("transport result: %v", err)
	}
	if transport.Disposition != "" {
		t.Fatalf("transport loss fabricated semantic disposition %q", transport.Disposition)
	}

	semantic := ReviewLaneResultV1{
		SchemaVersion: "review-lane-result.v1", LaneID: "ntm:session:1",
		Family: "gpt", ContextID: "review-1", Disposition: ReviewRefuted,
		FailureClass: ReviewFailureSemantic, Findings: []ReviewFinding{{Title: "bug", Evidence: "x.go:1"}},
		EvidencePath: evidencePath, Nonce: "n1", ReadOnly: true,
	}
	if err := semantic.ValidateAgainst(request); err != nil {
		t.Fatalf("semantic result: %v", err)
	}
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "escape.txt")
	if err := os.WriteFile(outsidePath, []byte("reviewed files: 2\ny.go:1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	semantic.EvidencePath = outsidePath
	if err := semantic.ValidateAgainst(request); err == nil {
		t.Fatal("evidence outside the request directory must fail")
	}
	linkPath := filepath.Join(evidenceDir, "linked-escape.txt")
	if err := os.Symlink(outsidePath, linkPath); err != nil {
		t.Fatal(err)
	}
	semantic.EvidencePath = linkPath
	if err := semantic.ValidateAgainst(request); err == nil {
		t.Fatal("symlink evidence escaping the request directory must fail")
	}
	semantic.EvidencePath = filepath.Join(evidenceDir, "missing.txt")
	if err := semantic.ValidateAgainst(request); err == nil {
		t.Fatal("missing evidence must fail")
	}
}
