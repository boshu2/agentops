package ports

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ReviewDisposition string

const (
	ReviewConfirmed ReviewDisposition = "CONFIRMED"
	ReviewRefuted   ReviewDisposition = "REFUTED"
)

type ReviewFailureClass string

const (
	ReviewFailureTransport ReviewFailureClass = "transport"
	ReviewFailureSemantic  ReviewFailureClass = "semantic"
)

type ReviewRequestV1 struct {
	SchemaVersion            string
	SubjectID                string
	HeadSHA                  string
	AcceptanceContract       string
	AcceptanceContractSHA256 string
	DiffPath                 string
	DiffSHA256               string
	AuthorContextID          string
	AuthorFamily             string
	DiversityMode            string
	Nonce                    string
	EvidenceDir              string
	ReadOnly                 bool
}

func (r ReviewRequestV1) Validate() error {
	if r.SchemaVersion != "review-request.v1" {
		return fmt.Errorf("schema_version must be review-request.v1")
	}
	if strings.TrimSpace(r.SubjectID) == "" || len(strings.TrimSpace(r.HeadSHA)) < 7 {
		return fmt.Errorf("subject_id and a commit-bound head_sha are required")
	}
	if !r.ReadOnly {
		return fmt.Errorf("review request must be read-only")
	}
	if strings.TrimSpace(r.AuthorContextID) == "" || strings.TrimSpace(r.AuthorFamily) == "" {
		return fmt.Errorf("author context and family are required")
	}
	if r.DiversityMode != "fresh-context" && r.DiversityMode != "multi-model" {
		return fmt.Errorf("unsupported diversity mode %q", r.DiversityMode)
	}
	if strings.TrimSpace(r.Nonce) == "" || strings.TrimSpace(r.EvidenceDir) == "" {
		return fmt.Errorf("nonce and evidence_dir are required")
	}
	if err := verifyDigest(r.AcceptanceContract, r.AcceptanceContractSHA256); err != nil {
		return fmt.Errorf("acceptance contract: %w", err)
	}
	if err := verifyDigest(r.DiffPath, r.DiffSHA256); err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	return nil
}

func verifyDigest(path, expected string) error {
	if strings.TrimSpace(path) == "" || len(expected) != 64 {
		return fmt.Errorf("path and sha256 digest are required")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if fmt.Sprintf("%x", h.Sum(nil)) != strings.ToLower(expected) {
		return fmt.Errorf("content digest mismatch")
	}
	return nil
}

type ReviewFinding struct {
	Title    string
	Evidence string
}

type ReviewLaneResultV1 struct {
	SchemaVersion string
	LaneID        string
	Family        string
	ContextID     string
	Disposition   ReviewDisposition
	FailureClass  ReviewFailureClass
	FailureReason string
	Findings      []ReviewFinding
	EvidencePath  string
	Nonce         string
	ReadOnly      bool
}

func (r ReviewLaneResultV1) Validate() error {
	if r.SchemaVersion != "review-lane-result.v1" {
		return fmt.Errorf("schema_version must be review-lane-result.v1")
	}
	if strings.TrimSpace(r.LaneID) == "" || strings.TrimSpace(r.Family) == "" || strings.TrimSpace(r.ContextID) == "" || strings.TrimSpace(r.Nonce) == "" {
		return fmt.Errorf("lane, family, context, and nonce are required")
	}
	if r.FailureClass == ReviewFailureTransport {
		if r.Disposition != "" {
			return fmt.Errorf("transport failure cannot carry a semantic disposition")
		}
		if strings.TrimSpace(r.FailureReason) == "" {
			return fmt.Errorf("transport failure reason is required")
		}
		return nil
	}
	if r.Disposition != ReviewConfirmed && r.Disposition != ReviewRefuted {
		return fmt.Errorf("semantic disposition must be CONFIRMED or REFUTED")
	}
	if !r.ReadOnly {
		return fmt.Errorf("semantic reviewer result must attest read-only execution")
	}
	if strings.TrimSpace(r.EvidencePath) == "" {
		return fmt.Errorf("semantic reviewer evidence path is required")
	}
	if r.Disposition == ReviewRefuted && len(r.Findings) == 0 {
		return fmt.Errorf("REFUTED result requires a finding")
	}
	return nil
}

func (r ReviewLaneResultV1) ValidateAgainst(request ReviewRequestV1) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if err := r.Validate(); err != nil {
		return err
	}
	if r.Nonce != request.Nonce {
		return fmt.Errorf("review nonce mismatch")
	}
	if r.FailureClass == ReviewFailureTransport {
		return nil
	}
	if r.ContextID == request.AuthorContextID {
		return fmt.Errorf("author context cannot count as an independent reviewer")
	}
	root, err := filepath.EvalSymlinks(request.EvidenceDir)
	if err != nil {
		return fmt.Errorf("resolve evidence directory: %w", err)
	}
	evidence, err := filepath.EvalSymlinks(r.EvidencePath)
	if err != nil {
		return fmt.Errorf("resolve reviewer evidence: %w", err)
	}
	rel, err := filepath.Rel(root, evidence)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("reviewer evidence escapes evidence_dir")
	}
	info, err := os.Stat(evidence)
	if err != nil {
		return fmt.Errorf("reviewer evidence: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("reviewer evidence must be a nonempty regular file")
	}
	return nil
}

type ReviewLanePort interface {
	Run(context.Context, ReviewRequestV1) (ReviewLaneResultV1, error)
}
