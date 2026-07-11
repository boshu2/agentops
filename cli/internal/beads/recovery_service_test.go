package beads

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeRecoveryPorts struct {
	shown     []StaleBeadRecord
	showIndex int
	calls     []string
	appendErr error
}

func (fake *fakeRecoveryPorts) ListInProgress(context.Context) ([]byte, error) {
	return []byte("[]"), nil
}
func (fake *fakeRecoveryPorts) Show(_ context.Context, id string) (StaleBeadRecord, error) {
	fake.calls = append(fake.calls, "show:"+id)
	record := fake.shown[fake.showIndex]
	fake.showIndex++
	return record, nil
}
func (fake *fakeRecoveryPorts) Claim(_ context.Context, id, agent string) error {
	fake.calls = append(fake.calls, "claim:"+id+":"+agent)
	return nil
}
func (fake *fakeRecoveryPorts) Now() time.Time { return time.Date(2026, 7, 11, 18, 0, 0, 0, time.UTC) }
func (fake *fakeRecoveryPorts) Actor() string  { return "codex" }
func (fake *fakeRecoveryPorts) ResolveRepoPath(path string) (string, error) {
	fake.calls = append(fake.calls, "resolve:"+path)
	return "/repo/" + path, nil
}
func (fake *fakeRecoveryPorts) AppendEvent(path string, _ any) error {
	fake.calls = append(fake.calls, "append:"+path)
	return fake.appendErr
}

func TestRecoveryServiceReportsLedgerFailureAfterClaim(t *testing.T) {
	ports := &fakeRecoveryPorts{
		shown: []StaleBeadRecord{
			{ID: "age-x", Status: "in_progress", Assignee: "old", UpdatedAt: "2026-07-11T10:00:00Z"},
			{ID: "age-x", Status: "in_progress", Assignee: "codex", UpdatedAt: "2026-07-11T18:00:00Z"},
		},
		appendErr: errors.New("disk full"),
	}
	service := RecoveryService{Claims: ports, Runtime: ports}
	_, err := service.Resume(context.Background(), "age-x", ResumeOptions{Ledger: "ledger.jsonl"})
	if err == nil || !strings.Contains(err.Error(), "claim already transferred") {
		t.Fatalf("error = %v", err)
	}
	want := []string{"show:age-x", "claim:age-x:codex", "show:age-x", "resolve:ledger.jsonl", "append:/repo/ledger.jsonl"}
	if !reflect.DeepEqual(ports.calls, want) {
		t.Fatalf("calls = %v, want %v", ports.calls, want)
	}
}
