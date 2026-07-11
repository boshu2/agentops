package close

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeRuntime struct{ snapshot Snapshot }

func (runtime fakeRuntime) Snapshot() (Snapshot, error) { return runtime.snapshot, nil }

type fakeTracker struct {
	resolution Resolution
	closed     bool
	statusErr  error
	syncErr    error
	events     *[]string
}

func (tracker *fakeTracker) Resolve(context.Context, Snapshot) (Resolution, error) {
	*tracker.events = append(*tracker.events, "resolve")
	return tracker.resolution, nil
}

func (tracker *fakeTracker) Status(context.Context, Resolution, string) (bool, error) {
	*tracker.events = append(*tracker.events, "tracker-status")
	return tracker.closed, tracker.statusErr
}

func (tracker *fakeTracker) Close(context.Context, Resolution, string, string) error {
	*tracker.events = append(*tracker.events, "close")
	tracker.closed = true
	return nil
}

func (tracker *fakeTracker) Sync(context.Context, Resolution) error {
	*tracker.events = append(*tracker.events, "sync")
	return tracker.syncErr
}

type fakeRepository struct {
	closed          bool
	ledgerStatuses  []bool
	preflightErr    error
	commitLedgerErr error
	commitPublicErr error
	events          *[]string
}

func (repository *fakeRepository) Preflight(context.Context, Snapshot, Resolution, string, []string) error {
	*repository.events = append(*repository.events, "preflight")
	return repository.preflightErr
}

func (repository *fakeRepository) LedgerStatus(context.Context, Resolution, string) (bool, error) {
	*repository.events = append(*repository.events, "ledger-status")
	if len(repository.ledgerStatuses) > 0 {
		closed := repository.ledgerStatuses[0]
		repository.ledgerStatuses = repository.ledgerStatuses[1:]
		return closed, nil
	}
	return repository.closed, nil
}

func (repository *fakeRepository) CommitLedger(context.Context, Snapshot, Resolution, string) (string, error) {
	*repository.events = append(*repository.events, "commit-ledger")
	return "ledger-head", repository.commitLedgerErr
}

func (repository *fakeRepository) CommitPublic(context.Context, Snapshot, Resolution, string, []string) (string, error) {
	*repository.events = append(*repository.events, "commit-public")
	return "public-head", repository.commitPublicErr
}

func TestServiceBRCloseOrdersMonotonicDurabilityPhases(t *testing.T) {
	events := []string{}
	tracker := &fakeTracker{resolution: Resolution{Backend: BackendBR}, events: &events}
	repository := &fakeRepository{ledgerStatuses: []bool{false, true}, events: &events}
	service := NewService(fakeRuntime{snapshot: Snapshot{WorkDir: "/repo"}}, tracker, repository)
	result, err := service.Execute(context.Background(), Request{ID: "age-1", Message: "done", Evidence: "proof", Mode: ModeEnsure})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"resolve", "preflight", "ledger-status", "close", "sync", "ledger-status", "commit-ledger", "commit-public"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	if result.Ref != "ledger-head" || result.AlreadyClosed {
		t.Fatalf("result = %+v", result)
	}
}

func TestServiceBRAlreadyClosedStillEnsuresPersistence(t *testing.T) {
	events := []string{}
	tracker := &fakeTracker{resolution: Resolution{Backend: BackendBR}, events: &events}
	repository := &fakeRepository{closed: true, events: &events}
	service := NewService(fakeRuntime{}, tracker, repository)
	result, err := service.Execute(context.Background(), Request{ID: "age-1", Mode: ModeEnsure})
	if err != nil {
		t.Fatal(err)
	}
	if !result.AlreadyClosed {
		t.Fatalf("result = %+v, want already closed", result)
	}
	for _, event := range events {
		if event == "close" {
			t.Fatalf("already-closed ensure repeated tracker close: %v", events)
		}
	}
	want := []string{"resolve", "preflight", "ledger-status", "sync", "ledger-status", "commit-ledger", "commit-public"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestServicePersistenceFailureIsStableAndNeverRollsBack(t *testing.T) {
	events := []string{}
	tracker := &fakeTracker{resolution: Resolution{Backend: BackendBR}, events: &events}
	repository := &fakeRepository{closed: true, commitPublicErr: errors.New("disk full"), events: &events}
	service := NewService(fakeRuntime{}, tracker, repository)
	_, err := service.Execute(context.Background(), Request{ID: "age-1", Mode: ModeEnsure})
	var failure *Failure
	if !errors.As(err, &failure) || failure.Code != ExitPersistence {
		t.Fatalf("error = %#v, want persistence failure", err)
	}
	for _, event := range events {
		if event == "reopen" || event == "close" {
			t.Fatalf("failure moved tracker state backward: %v", events)
		}
	}
}

func TestServiceBDUnknownStatusFailsClosed(t *testing.T) {
	events := []string{}
	tracker := &fakeTracker{
		resolution: Resolution{Backend: BackendBD}, statusErr: errors.New("bd unavailable"), events: &events,
	}
	repository := &fakeRepository{events: &events}
	service := NewService(fakeRuntime{}, tracker, repository)
	_, err := service.Execute(context.Background(), Request{ID: "agentops-1", Mode: ModeEnsure})
	var failure *Failure
	if !errors.As(err, &failure) || failure.Code != ExitTracker {
		t.Fatalf("error = %#v, want tracker failure", err)
	}
	if !reflect.DeepEqual(events, []string{"resolve", "preflight", "tracker-status"}) {
		t.Fatalf("events = %v; unknown status must not be treated as open", events)
	}
}
