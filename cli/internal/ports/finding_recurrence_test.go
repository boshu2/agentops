package ports

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindingRecurrenceReducer_DistinctObjectivesCreateOneAdvisoryProducerCandidate(t *testing.T) {
	observations := loadFindingRecurrenceFixture(t)

	got, err := NewInMemoryFindingRecurrenceReducer().Reduce(context.Background(), observations)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("candidates = %d, want 1", len(got))
	}
	candidate := got[0]
	if !candidate.Advisory {
		t.Fatal("recurring judgment finding must remain advisory")
	}
	if candidate.RecurrenceCount != 2 {
		t.Fatalf("recurrence_count = %d, want 2 distinct objectives (not 5 review events)", candidate.RecurrenceCount)
	}
	wantObjectives := []string{"objective-a", "objective-b"}
	if len(candidate.Evidence) != len(wantObjectives) {
		t.Fatalf("evidence refs = %d, want one per distinct objective", len(candidate.Evidence))
	}
	for i, want := range wantObjectives {
		if candidate.Evidence[i].ObjectiveID != want {
			t.Fatalf("evidence[%d].objective_id = %q, want %q", i, candidate.Evidence[i].ObjectiveID, want)
		}
	}
}

func loadFindingRecurrenceFixture(t *testing.T) []FindingObservation {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve fixture caller")
	}
	path := filepath.Join(filepath.Dir(here), "..", "..", "..", "tests", "fixtures", "learning-recurrence", "distinct-objectives.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var observations []FindingObservation
	if err := json.Unmarshal(raw, &observations); err != nil {
		t.Fatal(err)
	}
	return observations
}
