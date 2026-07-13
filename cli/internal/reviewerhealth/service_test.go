package reviewerhealth

import (
	"context"
	"testing"
	"time"
)

type fakeProbe map[string]ProbeResult

func (probe fakeProbe) Check(_ context.Context, reviewer Reviewer, _ time.Duration) ProbeResult {
	return probe[reviewer.Name]
}

func TestServiceChecksCatalogInOrderAndReportsLiveReviewers(t *testing.T) {
	catalog := []Reviewer{{Name: "codex", InstallCommand: "install codex"}, {Name: "agy", InstallCommand: "install agy"}}
	service := NewService(catalog, fakeProbe{
		"codex": {Status: "pass", Detail: "reachable", Live: true},
		"agy":   {Status: "warn", Detail: "missing"},
	})
	checks, live := service.Check(context.Background(), time.Second)
	if len(checks) != 2 || checks[0].Name != "Reviewer: codex" || checks[1].Name != "Reviewer: agy" {
		t.Fatalf("checks = %+v", checks)
	}
	if len(live) != 1 || live[0] != "codex" {
		t.Fatalf("live = %v", live)
	}
}

func TestDefaultCatalogPreservesInstallGuidance(t *testing.T) {
	catalog := DefaultCatalog()
	if len(catalog) != 2 || catalog[0].Name != "codex" || catalog[1].Name != "agy" {
		t.Fatalf("catalog = %+v", catalog)
	}
	for _, reviewer := range catalog {
		if reviewer.InstallCommand == "" {
			t.Fatalf("missing install command: %+v", reviewer)
		}
	}
}
