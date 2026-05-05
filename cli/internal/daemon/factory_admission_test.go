package daemon

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

func TestFactoryAdmissionEvaluatorAllowsCleanWorkOrder(t *testing.T) {
	evaluator := validFactoryAdmissionEvaluator()
	spec := NewFactoryAdmissionJobSpec("factory-run-1", validFactoryWorkOrder())

	decision, err := evaluator.EvaluateAdmission(context.Background(), spec)
	if err != nil {
		t.Fatalf("EvaluateAdmission: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("Allowed = false reasons=%v, want allowed", decision.Reasons)
	}
	if decision.Evidence.OpenPRBlockerCount != 0 || decision.Evidence.MainCIStatus != FactoryCIStatusGreen {
		t.Fatalf("evidence = %#v, want clean green evidence", decision.Evidence)
	}
}

func TestFactoryAdmissionEvaluatorBlocksUnsafeEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*FactoryWorkOrder, *fakeFactoryAdmissionEvidence)
		want   string
	}{
		{
			name: "expired",
			mutate: func(work *FactoryWorkOrder, _ *fakeFactoryAdmissionEvidence) {
				work.ExpiresAt = "2026-05-04T23:34:00Z"
			},
			want: FactoryAdmissionReasonExpiredWorkOrder,
		},
		{
			name: "base sha mismatch",
			mutate: func(_ *FactoryWorkOrder, evidence *fakeFactoryAdmissionEvidence) {
				evidence.repo.HeadSHA = "999999999999"
			},
			want: FactoryAdmissionReasonBaseSHAMismatch,
		},
		{
			name: "dirty worktree",
			mutate: func(_ *FactoryWorkOrder, evidence *fakeFactoryAdmissionEvidence) {
				evidence.repo.Dirty = true
			},
			want: FactoryAdmissionReasonDirtyWorktree,
		},
		{
			name: "tracked agents",
			mutate: func(_ *FactoryWorkOrder, evidence *fakeFactoryAdmissionEvidence) {
				evidence.repo.TrackedAgents = []string{".agents/rpi/next-work.jsonl"}
			},
			want: FactoryAdmissionReasonTrackedAgents,
		},
		{
			name: "open pr overlap",
			mutate: func(_ *FactoryWorkOrder, evidence *fakeFactoryAdmissionEvidence) {
				evidence.pr.Blockers = []FactoryOpenPRBlocker{{
					PRNumber: 123,
					HeadRef:  "nightly/example",
					Files:    []string{"cli/internal/daemon/factory_admission.go"},
				}}
			},
			want: FactoryAdmissionReasonOpenPROverlap,
		},
		{
			name: "open pr evidence unknown",
			mutate: func(_ *FactoryWorkOrder, evidence *fakeFactoryAdmissionEvidence) {
				evidence.pr.Known = false
			},
			want: FactoryAdmissionReasonOpenPREvidenceUnknown,
		},
		{
			name: "main ci red",
			mutate: func(_ *FactoryWorkOrder, evidence *fakeFactoryAdmissionEvidence) {
				evidence.ci.Baseline.Status = FactoryCIStatusRed
				evidence.ci.Baseline.FailedJobs = []string{"go-build"}
			},
			want: FactoryAdmissionReasonMainCIRed,
		},
		{
			name: "main ci unknown",
			mutate: func(_ *FactoryWorkOrder, evidence *fakeFactoryAdmissionEvidence) {
				evidence.ci.Known = false
			},
			want: FactoryAdmissionReasonMainCIUnknown,
		},
		{
			name: "provider missing",
			mutate: func(_ *FactoryWorkOrder, evidence *fakeFactoryAdmissionEvidence) {
				*evidence = fakeFactoryAdmissionEvidence{}
			},
			want: FactoryAdmissionReasonEvidenceProviderMissing,
		},
		{
			name: "provider error",
			mutate: func(_ *FactoryWorkOrder, evidence *fakeFactoryAdmissionEvidence) {
				evidence.repoErr = errors.New("git unavailable")
			},
			want: FactoryAdmissionReasonRepoStateUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			work := validFactoryWorkOrder()
			evidence := validFactoryAdmissionEvidence()
			tc.mutate(&work, &evidence)
			evaluator := FactoryAdmissionEvaluator{
				Clock: fixedFactoryAdmissionClock,
			}
			if tc.want != FactoryAdmissionReasonEvidenceProviderMissing {
				evaluator.Evidence = evidence
			}
			spec := NewFactoryAdmissionJobSpec("factory-run-1", work)
			decision, err := evaluator.EvaluateAdmission(context.Background(), spec)
			if err != nil {
				t.Fatalf("EvaluateAdmission: %v", err)
			}
			if decision.Allowed {
				t.Fatalf("Allowed = true, want blocked with %q", tc.want)
			}
			if !slices.Contains(decision.Reasons, tc.want) {
				t.Fatalf("reasons = %v, want %q", decision.Reasons, tc.want)
			}
		})
	}
}

func TestFactoryAdmissionEvaluatorAllowsUnknownEvidenceOnlyForNonMutating(t *testing.T) {
	work := validFactoryWorkOrder()
	work.LandingPolicy = FactoryLandingPolicyOff
	work.UnknownEvidencePolicy = FactoryUnknownEvidenceAllowNonMutating
	evidence := validFactoryAdmissionEvidence()
	evidence.pr.Known = false
	evidence.ci.Known = false

	decision, err := FactoryAdmissionEvaluator{
		Clock:    fixedFactoryAdmissionClock,
		Evidence: evidence,
	}.EvaluateLocalPilot(context.Background(), NewFactoryLocalPilotJobSpec("factory-run-1", work))
	if err != nil {
		t.Fatalf("EvaluateLocalPilot: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("Allowed = false reasons=%v, want allowed non-mutating unknown evidence", decision.Reasons)
	}
}

func TestFactoryAdmissionEvaluatorReturnsContextError(t *testing.T) {
	evidence := validFactoryAdmissionEvidence()
	evidence.repoErr = context.DeadlineExceeded
	_, err := FactoryAdmissionEvaluator{
		Clock:    fixedFactoryAdmissionClock,
		Evidence: evidence,
	}.EvaluateAdmission(context.Background(), NewFactoryAdmissionJobSpec("factory-run-1", validFactoryWorkOrder()))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline", err)
	}
}

func validFactoryAdmissionEvaluator() FactoryAdmissionEvaluator {
	return FactoryAdmissionEvaluator{
		Clock:    fixedFactoryAdmissionClock,
		Evidence: validFactoryAdmissionEvidence(),
	}
}

func validFactoryAdmissionEvidence() fakeFactoryAdmissionEvidence {
	return fakeFactoryAdmissionEvidence{
		repo: FactoryRepoState{
			HeadSHA: "abcdef1234567890",
		},
		pr: FactoryPRBlockerMatrix{
			Known: true,
		},
		ci: FactoryCIBaselineEvidence{
			Known: true,
			Baseline: FactoryMainCIBaseline{
				Status:    FactoryCIStatusGreen,
				RunID:     "123",
				CheckedAt: "2026-05-04T23:29:00Z",
			},
		},
	}
}

func fixedFactoryAdmissionClock() time.Time {
	return time.Date(2026, 5, 4, 23, 35, 0, 0, time.UTC)
}

type fakeFactoryAdmissionEvidence struct {
	repo    FactoryRepoState
	repoErr error
	pr      FactoryPRBlockerMatrix
	prErr   error
	ci      FactoryCIBaselineEvidence
	ciErr   error
}

func (f fakeFactoryAdmissionEvidence) RepoState(context.Context) (FactoryRepoState, error) {
	return f.repo, f.repoErr
}

func (f fakeFactoryAdmissionEvidence) OpenPRBlockers(context.Context, []string) (FactoryPRBlockerMatrix, error) {
	return f.pr, f.prErr
}

func (f fakeFactoryAdmissionEvidence) MainCIBaseline(context.Context) (FactoryCIBaselineEvidence, error) {
	return f.ci, f.ciErr
}
