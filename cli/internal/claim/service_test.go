package claim

import (
	"context"
	"errors"
	"testing"

	"github.com/boshu2/agentops/cli/internal/claimproof"
	"github.com/boshu2/agentops/cli/internal/ports"
)

type fakeTracker struct{ id string }

func (tracker *fakeTracker) Claim(_ context.Context, id string, _ Streams) error {
	tracker.id = id
	return nil
}

type fakeBinder struct {
	binding ports.EvidenceBinding
	list    []ports.EvidenceBinding
	err     error
}

func (binder *fakeBinder) Bind(_ context.Context, binding ports.EvidenceBinding) error {
	binder.binding = binding
	return binder.err
}
func (binder *fakeBinder) List(context.Context) ([]ports.EvidenceBinding, error) {
	return binder.list, binder.err
}

type fakeChecker struct{ changed bool }

func (checker *fakeChecker) Check(_ context.Context, base string, changed bool) (claimproof.Report, error) {
	checker.changed = changed
	return claimproof.Report{Summary: claimproof.Summary{Base: base}}, nil
}

func TestServiceDelegatesClaimAndNormalizesBinding(t *testing.T) {
	tracker, binder, checker := &fakeTracker{}, &fakeBinder{}, &fakeChecker{}
	service := NewService(tracker, binder, checker)
	if err := service.Claim(context.Background(), "age-1", Streams{}); err != nil {
		t.Fatal(err)
	}
	if tracker.id != "age-1" {
		t.Fatalf("claimed %q", tracker.id)
	}
	request := BindRequest{Claim: "AOP-X", Path: "p.md", Level: "pg2", Anchors: []string{"L1"}}
	if err := service.Bind(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if binder.binding.Level != ports.EvidenceLevelPG2 || binder.binding.Claim != "AOP-X" {
		t.Fatalf("binding = %+v", binder.binding)
	}
	if _, err := service.Check(context.Background(), "base", true); err != nil || !checker.changed {
		t.Fatalf("check err=%v changed=%t", err, checker.changed)
	}
}

func TestServiceRejectsInvalidRequestsBeforeEffects(t *testing.T) {
	binder := &fakeBinder{err: errors.New("must not run")}
	service := NewService(&fakeTracker{}, binder, &fakeChecker{})
	for _, request := range []BindRequest{
		{Path: "p", Level: "PG1"},
		{Claim: "A", Level: "PG1"},
		{Claim: "A", Path: "p", Level: "PG9"},
	} {
		if err := service.Bind(context.Background(), request); err == nil {
			t.Fatalf("Bind(%+v) succeeded", request)
		}
	}
	if binder.binding.Claim != "" {
		t.Fatalf("binder ran: %+v", binder.binding)
	}
	if _, err := service.Check(context.Background(), "base", false); err == nil {
		t.Fatal("Check without --changed succeeded")
	}
}
