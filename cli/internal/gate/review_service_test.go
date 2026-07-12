package gate

import (
	"context"
	"errors"
	"testing"
	"time"
)

type reviewPortSpy struct {
	entries      []ReviewEntry
	listErr      error
	listCalls    int
	approveCalls []ApproveRequest
	rejectCalls  []RejectRequest
	bulkCalls    []BulkApproveRequest
	reviewer     string
}

func (port *reviewPortSpy) ListPending(context.Context) ([]ReviewEntry, error) {
	port.listCalls++
	return port.entries, port.listErr
}

func (port *reviewPortSpy) Approve(_ context.Context, request ApproveRequest) error {
	port.approveCalls = append(port.approveCalls, request)
	return nil
}

func (port *reviewPortSpy) Reject(_ context.Context, request RejectRequest) error {
	port.rejectCalls = append(port.rejectCalls, request)
	return nil
}

func (port *reviewPortSpy) BulkApprove(_ context.Context, request BulkApproveRequest) ([]string, error) {
	port.bulkCalls = append(port.bulkCalls, request)
	return []string{"cand-1"}, nil
}

func (port *reviewPortSpy) Reviewer() string { return port.reviewer }

func TestReviewServicePendingComputesUrgency(t *testing.T) {
	port := &reviewPortSpy{entries: []ReviewEntry{
		{ApproachingAutoPromote: true},
		{Age: 13 * time.Hour},
		{Age: time.Hour},
	}}
	result, err := (ReviewService{Port: port}).Pending(context.Background(), PendingRequest{})
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	want := []string{"HIGH (approaching 24h)", "MEDIUM", "LOW"}
	for index := range want {
		if result.Entries[index].Urgency != want[index] {
			t.Errorf("entry %d urgency = %q, want %q", index, result.Entries[index].Urgency, want[index])
		}
	}
}

func TestReviewServicePendingDryRunDoesNotReadStore(t *testing.T) {
	port := &reviewPortSpy{listErr: errors.New("must not be called")}
	result, err := (ReviewService{Port: port}).Pending(context.Background(), PendingRequest{DryRun: true})
	if err != nil || !result.DryRun || port.listCalls != 0 {
		t.Fatalf("result=%+v err=%v listCalls=%d", result, err, port.listCalls)
	}
}

func TestReviewServiceApproveDryRunDoesNotResolveReviewerOrMutate(t *testing.T) {
	port := &reviewPortSpy{reviewer: "reviewer"}
	result, err := (ReviewService{Port: port}).Approve(context.Background(), ApproveInput{CandidateID: "cand-1", Note: "ok", DryRun: true})
	if err != nil || !result.DryRun || len(port.approveCalls) != 0 {
		t.Fatalf("result=%+v err=%v calls=%v", result, err, port.approveCalls)
	}
}

func TestReviewServiceRejectRequiresReason(t *testing.T) {
	port := &reviewPortSpy{}
	_, err := (ReviewService{Port: port}).Reject(context.Background(), RejectInput{CandidateID: "cand-1"})
	if err == nil || len(port.rejectCalls) != 0 {
		t.Fatalf("err=%v calls=%v", err, port.rejectCalls)
	}
}

func TestReviewServiceBulkApproveParsesThresholdAndPreservesDryRun(t *testing.T) {
	port := &reviewPortSpy{reviewer: "reviewer"}
	result, err := (ReviewService{Port: port}).BulkApprove(context.Background(), BulkApproveInput{OlderThan: "12h", Tier: "gold", DryRun: true})
	if err != nil {
		t.Fatalf("BulkApprove: %v", err)
	}
	if len(port.bulkCalls) != 1 {
		t.Fatalf("bulk calls = %v", port.bulkCalls)
	}
	call := port.bulkCalls[0]
	if call.OlderThan != 12*time.Hour || call.Reviewer != "reviewer" || !call.DryRun {
		t.Errorf("call = %+v", call)
	}
	if result.Tier != "gold" || len(result.Approved) != 1 {
		t.Errorf("result = %+v", result)
	}
}

func TestReviewServiceBulkApproveRejectsInvalidDurationBeforeCallingPort(t *testing.T) {
	port := &reviewPortSpy{}
	_, err := (ReviewService{Port: port}).BulkApprove(context.Background(), BulkApproveInput{OlderThan: "later"})
	if err == nil || len(port.bulkCalls) != 0 {
		t.Fatalf("err=%v calls=%v", err, port.bulkCalls)
	}
}
