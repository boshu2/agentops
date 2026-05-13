// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"testing"
)

func TestInMemoryFactoryAdmission_DefaultsAreZeroValue(t *testing.T) {
	a := NewInMemoryFactoryAdmission()
	repo, err := a.ProbeRepoState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if repo.HeadSHA != "" || repo.Dirty || len(repo.TrackedAgents) != 0 {
		t.Fatalf("default RepoState should be zero, got %+v", repo)
	}
	pr, _ := a.ProbeOpenPRBlockers(context.Background(), nil)
	if pr.Known {
		t.Fatal("default PR evidence should be Known=false")
	}
	ci, _ := a.ProbeMainCIBaseline(context.Background())
	if ci.Known {
		t.Fatal("default CI evidence should be Known=false")
	}
}

func TestInMemoryFactoryAdmission_RepoStateSeeded(t *testing.T) {
	a := NewInMemoryFactoryAdmission()
	a.RepoStateValue = FactoryRepoEvidence{HeadSHA: "abc123", Dirty: true, TrackedAgents: []string{"a/b"}}
	got, _ := a.ProbeRepoState(context.Background())
	if got.HeadSHA != "abc123" {
		t.Fatalf("HeadSHA = %q, want abc123", got.HeadSHA)
	}
	if !got.Dirty {
		t.Fatal("Dirty should be true")
	}
	if len(got.TrackedAgents) != 1 || got.TrackedAgents[0] != "a/b" {
		t.Fatalf("TrackedAgents = %v", got.TrackedAgents)
	}
}

func TestInMemoryFactoryAdmission_OpenPRBlockersUsesFunc(t *testing.T) {
	a := NewInMemoryFactoryAdmission()
	a.OpenPRBlockersFunc = func(touched []string) (FactoryPREvidence, error) {
		return FactoryPREvidence{Known: true, Blockers: append([]string{"PR#42"}, touched...)}, nil
	}
	got, _ := a.ProbeOpenPRBlockers(context.Background(), []string{"file.go"})
	if !got.Known {
		t.Fatal("Known should be true (func returned it)")
	}
	if len(got.Blockers) != 2 || got.Blockers[0] != "PR#42" || got.Blockers[1] != "file.go" {
		t.Fatalf("Blockers = %v, want [PR#42 file.go]", got.Blockers)
	}
}

func TestInMemoryFactoryAdmission_MainCIBaselineSeeded(t *testing.T) {
	a := NewInMemoryFactoryAdmission()
	a.MainCIBaselineVal = FactoryCIEvidence{Known: true, Green: false}
	got, _ := a.ProbeMainCIBaseline(context.Background())
	if !got.Known {
		t.Fatal("Known should be true")
	}
	if got.Green {
		t.Fatal("Green should be false")
	}
}

func TestInMemoryFactoryAdmission_ErrorsPropagate(t *testing.T) {
	a := NewInMemoryFactoryAdmission()
	a.RepoStateErr = errors.New("repo unavailable")
	_, err := a.ProbeRepoState(context.Background())
	if err == nil || err.Error() != "repo unavailable" {
		t.Fatalf("error not propagated: %v", err)
	}
	a.MainCIBaselineErr = errors.New("ci api down")
	_, err = a.ProbeMainCIBaseline(context.Background())
	if err == nil || err.Error() != "ci api down" {
		t.Fatalf("CI error not propagated: %v", err)
	}
}

func TestInMemoryFactoryAdmission_HonorsContextCancellation(t *testing.T) {
	a := NewInMemoryFactoryAdmission()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, name := range []string{"RepoState", "OpenPRBlockers", "MainCIBaseline"} {
		var err error
		switch name {
		case "RepoState":
			_, err = a.ProbeRepoState(ctx)
		case "OpenPRBlockers":
			_, err = a.ProbeOpenPRBlockers(ctx, nil)
		case "MainCIBaseline":
			_, err = a.ProbeMainCIBaseline(ctx)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%s: error = %v, want context.Canceled", name, err)
		}
	}
}
