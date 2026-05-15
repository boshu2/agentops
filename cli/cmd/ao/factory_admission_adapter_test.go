// practices: [hexagonal-architecture, tdd]
package main

import (
	"context"
	"errors"
	"testing"

	"github.com/boshu2/agentops/cli/internal/daemon"
)

// fakeDaemonEvidenceProvider drives the production adapter without
// standing up the real LocalFactoryAdmissionEvidenceProvider (which
// would need a real repo + gh CLI).
type fakeDaemonEvidenceProvider struct {
	repoState  daemon.FactoryRepoState
	repoErr    error
	prMatrix   daemon.FactoryPRBlockerMatrix
	prErr      error
	ciBaseline daemon.FactoryCIBaselineEvidence
	ciErr      error
}

func (f fakeDaemonEvidenceProvider) RepoState(ctx context.Context) (daemon.FactoryRepoState, error) {
	return f.repoState, f.repoErr
}
func (f fakeDaemonEvidenceProvider) OpenPRBlockers(ctx context.Context, touched []string) (daemon.FactoryPRBlockerMatrix, error) {
	return f.prMatrix, f.prErr
}
func (f fakeDaemonEvidenceProvider) MainCIBaseline(ctx context.Context) (daemon.FactoryCIBaselineEvidence, error) {
	return f.ciBaseline, f.ciErr
}

func TestProductionFactoryAdmission_RepoStateTranslation(t *testing.T) {
	a := newProductionFactoryAdmission(fakeDaemonEvidenceProvider{
		repoState: daemon.FactoryRepoState{HeadSHA: "abc", Dirty: true, TrackedAgents: []string{"x/y"}},
	})
	got, err := a.ProbeRepoState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.HeadSHA != "abc" || !got.Dirty || len(got.TrackedAgents) != 1 || got.TrackedAgents[0] != "x/y" {
		t.Fatalf("translation wrong: got %+v", got)
	}
}

func TestProductionFactoryAdmission_OpenPRBlockersReduceStructToString(t *testing.T) {
	a := newProductionFactoryAdmission(fakeDaemonEvidenceProvider{
		prMatrix: daemon.FactoryPRBlockerMatrix{
			Known: true,
			Blockers: []daemon.FactoryOpenPRBlocker{
				{PRNumber: 42, HeadRef: "feature-foo"},
				{PRNumber: 99, HeadRef: "bug-fix"},
			},
		},
	})
	got, _ := a.ProbeOpenPRBlockers(context.Background(), []string{"file.go"})
	if !got.Known {
		t.Fatal("Known not propagated")
	}
	if len(got.Blockers) != 2 {
		t.Fatalf("len = %d, want 2", len(got.Blockers))
	}
	if got.Blockers[0] != "PR#42 feature-foo" {
		t.Fatalf("Blockers[0] = %q", got.Blockers[0])
	}
}

func TestProductionFactoryAdmission_MainCIGreenTranslated(t *testing.T) {
	a := newProductionFactoryAdmission(fakeDaemonEvidenceProvider{
		ciBaseline: daemon.FactoryCIBaselineEvidence{
			Known:    true,
			Baseline: daemon.FactoryMainCIBaseline{Status: daemon.FactoryCIStatusGreen},
		},
	})
	got, _ := a.ProbeMainCIBaseline(context.Background())
	if !got.Known {
		t.Fatal("Known not propagated")
	}
	if !got.Green {
		t.Fatal("Green should be true when daemon Status=green")
	}
}

func TestProductionFactoryAdmission_MainCIRedTranslatesToNotGreen(t *testing.T) {
	a := newProductionFactoryAdmission(fakeDaemonEvidenceProvider{
		ciBaseline: daemon.FactoryCIBaselineEvidence{
			Known:    true,
			Baseline: daemon.FactoryMainCIBaseline{Status: daemon.FactoryCIStatusRed},
		},
	})
	got, _ := a.ProbeMainCIBaseline(context.Background())
	if !got.Known {
		t.Fatal("Known should pass through")
	}
	if got.Green {
		t.Fatal("Green should be false when daemon Status=red")
	}
}

func TestProductionFactoryAdmission_NilProviderErrorsClearly(t *testing.T) {
	a := newProductionFactoryAdmission(nil)
	if _, err := a.ProbeRepoState(context.Background()); err == nil {
		t.Fatal("nil provider on RepoState should error")
	}
	if _, err := a.ProbeOpenPRBlockers(context.Background(), nil); err == nil {
		t.Fatal("nil provider on OpenPRBlockers should error")
	}
	if _, err := a.ProbeMainCIBaseline(context.Background()); err == nil {
		t.Fatal("nil provider on MainCIBaseline should error")
	}
}

func TestProductionFactoryAdmission_ErrorsWrapped(t *testing.T) {
	a := newProductionFactoryAdmission(fakeDaemonEvidenceProvider{
		repoErr: errors.New("repo bad"),
	})
	_, err := a.ProbeRepoState(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if msg := err.Error(); msg == "" || msg == "repo bad" {
		t.Fatalf("error not wrapped with context: %q", msg)
	}
}

func TestProductionFactoryAdmission_HonorsContextCancellation(t *testing.T) {
	a := newProductionFactoryAdmission(fakeDaemonEvidenceProvider{})
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
