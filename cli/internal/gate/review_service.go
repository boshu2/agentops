package gate

import (
	"context"
	"fmt"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

const defaultBulkApproveThreshold = 24 * time.Hour

type ReviewEntry struct {
	types.PoolEntry
	FilePath               string        `json:"file_path,omitempty"`
	Age                    time.Duration `json:"-"`
	AgeString              string        `json:"age,omitempty"`
	ApproachingAutoPromote bool          `json:"approaching_auto_promote,omitempty"`
	Urgency                string        `json:"-" yaml:"-"`
}

type ApproveRequest struct {
	CandidateID string
	Note        string
	Reviewer    string
}

type RejectRequest struct {
	CandidateID string
	Reason      string
	Reviewer    string
}

type BulkApproveRequest struct {
	OlderThan time.Duration
	Reviewer  string
	DryRun    bool
}

type ReviewPort interface {
	ListPending(context.Context) ([]ReviewEntry, error)
	Approve(context.Context, ApproveRequest) error
	Reject(context.Context, RejectRequest) error
	BulkApprove(context.Context, BulkApproveRequest) ([]string, error)
	Reviewer() string
}

type ReviewService struct {
	Port ReviewPort
}

type PendingRequest struct {
	DryRun bool
}

type PendingResult struct {
	DryRun  bool          `json:"dry_run" yaml:"dry_run"`
	Entries []ReviewEntry `json:"entries" yaml:"entries"`
}

func (service ReviewService) Pending(ctx context.Context, request PendingRequest) (PendingResult, error) {
	if request.DryRun {
		return PendingResult{DryRun: true}, nil
	}
	entries, err := service.Port.ListPending(ctx)
	if err != nil {
		return PendingResult{}, fmt.Errorf("gate pending: %w", err)
	}
	for index := range entries {
		entries[index].Urgency = reviewUrgency(entries[index])
	}
	return PendingResult{Entries: entries}, nil
}

func reviewUrgency(entry ReviewEntry) string {
	switch {
	case entry.ApproachingAutoPromote:
		return "HIGH (approaching 24h)"
	case entry.Age > 12*time.Hour:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

type ApproveInput struct {
	CandidateID string
	Note        string
	DryRun      bool
}

type ApproveResult struct {
	CandidateID string `json:"candidate_id" yaml:"candidate_id"`
	Note        string `json:"note,omitempty" yaml:"note,omitempty"`
	Reviewer    string `json:"reviewer,omitempty" yaml:"reviewer,omitempty"`
	DryRun      bool   `json:"dry_run" yaml:"dry_run"`
}

func (service ReviewService) Approve(ctx context.Context, input ApproveInput) (ApproveResult, error) {
	if input.CandidateID == "" {
		return ApproveResult{}, fmt.Errorf("gate approve: candidate ID is required")
	}
	result := ApproveResult{CandidateID: input.CandidateID, Note: input.Note, DryRun: input.DryRun}
	if input.DryRun {
		return result, nil
	}
	result.Reviewer = service.Port.Reviewer()
	if err := service.Port.Approve(ctx, ApproveRequest{CandidateID: input.CandidateID, Note: input.Note, Reviewer: result.Reviewer}); err != nil {
		return ApproveResult{}, fmt.Errorf("approve candidate: %w", err)
	}
	return result, nil
}

type RejectInput struct {
	CandidateID string
	Reason      string
	DryRun      bool
}

type RejectResult struct {
	CandidateID string `json:"candidate_id" yaml:"candidate_id"`
	Reason      string `json:"reason" yaml:"reason"`
	Reviewer    string `json:"reviewer,omitempty" yaml:"reviewer,omitempty"`
	DryRun      bool   `json:"dry_run" yaml:"dry_run"`
}

func (service ReviewService) Reject(ctx context.Context, input RejectInput) (RejectResult, error) {
	if input.CandidateID == "" {
		return RejectResult{}, fmt.Errorf("gate reject: candidate ID is required")
	}
	if input.Reason == "" {
		return RejectResult{}, fmt.Errorf("--reason is required for rejection")
	}
	result := RejectResult{CandidateID: input.CandidateID, Reason: input.Reason, DryRun: input.DryRun}
	if input.DryRun {
		return result, nil
	}
	result.Reviewer = service.Port.Reviewer()
	if err := service.Port.Reject(ctx, RejectRequest{CandidateID: input.CandidateID, Reason: input.Reason, Reviewer: result.Reviewer}); err != nil {
		return RejectResult{}, fmt.Errorf("reject candidate: %w", err)
	}
	return result, nil
}

type BulkApproveInput struct {
	OlderThan string
	Tier      string
	DryRun    bool
}

type BulkApproveResult struct {
	Approved  []string      `json:"approved" yaml:"approved"`
	Threshold time.Duration `json:"-" yaml:"-"`
	Tier      string        `json:"tier" yaml:"tier"`
	DryRun    bool          `json:"dry_run" yaml:"dry_run"`
}

func (service ReviewService) BulkApprove(ctx context.Context, input BulkApproveInput) (BulkApproveResult, error) {
	threshold := defaultBulkApproveThreshold
	if input.OlderThan != "" {
		parsed, err := time.ParseDuration(input.OlderThan)
		if err != nil {
			return BulkApproveResult{}, fmt.Errorf("invalid duration %q: %w", input.OlderThan, err)
		}
		threshold = parsed
	}
	tier := input.Tier
	if tier == "" {
		tier = "silver"
	}
	approved, err := service.Port.BulkApprove(ctx, BulkApproveRequest{OlderThan: threshold, Reviewer: service.Port.Reviewer(), DryRun: input.DryRun})
	if err != nil {
		return BulkApproveResult{}, fmt.Errorf("bulk approve: %w", err)
	}
	return BulkApproveResult{Approved: approved, Threshold: threshold, Tier: tier, DryRun: input.DryRun}, nil
}
