package gate

import (
	"context"

	gateapp "github.com/boshu2/agentops/cli/internal/gate"
	"github.com/boshu2/agentops/cli/internal/pool"
)

type ReviewPool struct {
	pool     *pool.Pool
	reviewer func() string
}

func NewReviewPool(root string, reviewer func() string) *ReviewPool {
	return &ReviewPool{pool: pool.NewPool(root), reviewer: reviewer}
}

func (adapter *ReviewPool) ListPending(ctx context.Context) ([]gateapp.ReviewEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := adapter.pool.ListPendingReview()
	if err != nil {
		return nil, err
	}
	var result []gateapp.ReviewEntry
	if entries != nil {
		result = make([]gateapp.ReviewEntry, 0, len(entries))
	}
	for _, entry := range entries {
		result = append(result, gateapp.ReviewEntry{
			PoolEntry:              entry.PoolEntry,
			FilePath:               entry.FilePath,
			Age:                    entry.Age,
			AgeString:              entry.AgeString,
			ApproachingAutoPromote: entry.ApproachingAutoPromote,
		})
	}
	return result, nil
}

func (adapter *ReviewPool) Approve(ctx context.Context, request gateapp.ApproveRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return adapter.pool.Approve(request.CandidateID, request.Note, request.Reviewer)
}

func (adapter *ReviewPool) Reject(ctx context.Context, request gateapp.RejectRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return adapter.pool.Reject(request.CandidateID, request.Reason, request.Reviewer)
}

func (adapter *ReviewPool) BulkApprove(ctx context.Context, request gateapp.BulkApproveRequest) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return adapter.pool.BulkApprove(request.OlderThan, request.Reviewer, request.DryRun)
}

func (adapter *ReviewPool) Reviewer() string {
	if adapter.reviewer == nil {
		return "unknown"
	}
	return adapter.reviewer()
}

var _ gateapp.ReviewPort = (*ReviewPool)(nil)
