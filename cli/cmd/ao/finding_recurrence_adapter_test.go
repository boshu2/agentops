package main

import (
	"context"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestProductionFindingRecurrenceReducer_UsesDistinctObjectives(t *testing.T) {
	observations := []ports.FindingObservation{
		{ID: "a-1", ClassKey: "v1:docs/stale", ObjectiveID: "objective-a", EvidenceRef: "a/1"},
		{ID: "a-2", ClassKey: "v1:docs/stale", ObjectiveID: "objective-a", EvidenceRef: "a/2"},
		{ID: "a-3", ClassKey: "v1:docs/stale", ObjectiveID: "objective-a", EvidenceRef: "a/3"},
		{ID: "a-4", ClassKey: "v1:docs/stale", ObjectiveID: "objective-a", EvidenceRef: "a/4"},
		{ID: "b-1", ClassKey: "v1:docs/stale", ObjectiveID: "objective-b", EvidenceRef: "b/1"},
	}

	got, err := newProductionFindingRecurrenceReducer().Reduce(context.Background(), observations)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RecurrenceCount != 2 || len(got[0].Evidence) != 2 {
		t.Fatalf("want one candidate citing two distinct objectives, got %+v", got)
	}
}

func TestProductionFindingRecurrenceReducer_OneCatchCreatesNoPolicy(t *testing.T) {
	got, err := newProductionFindingRecurrenceReducer().Reduce(context.Background(), []ports.FindingObservation{
		{ID: "one", ClassKey: "v1:docs/one-off", ObjectiveID: "objective-a", EvidenceRef: "a/1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("one finding must create no producer policy, got %+v", got)
	}
}
