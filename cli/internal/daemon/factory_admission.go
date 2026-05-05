package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	FactoryAdmissionReasonEvidenceProviderMissing = "evidence_provider_missing"
	FactoryAdmissionReasonExpiredWorkOrder        = "expired_work_order"
	FactoryAdmissionReasonFutureGeneratedAt       = "future_generated_at"
	FactoryAdmissionReasonRepoStateUnknown        = "repo_state_unknown"
	FactoryAdmissionReasonRepoHeadUnknown         = "repo_head_unknown"
	FactoryAdmissionReasonBaseSHAMismatch         = "base_sha_mismatch"
	FactoryAdmissionReasonDirtyWorktree           = "dirty_worktree"
	FactoryAdmissionReasonTrackedAgents           = "tracked_agents"
	FactoryAdmissionReasonOpenPREvidenceUnknown   = "open_pr_evidence_unknown"
	FactoryAdmissionReasonOpenPROverlap           = "open_pr_overlap"
	FactoryAdmissionReasonMainCIUnknown           = "main_ci_unknown"
	FactoryAdmissionReasonMainCIRed               = "main_ci_red"
	FactoryAdmissionReasonRPIHandoffUnavailable   = "rpi_handoff_unavailable"
)

type FactoryAdmissionEvaluator struct {
	Clock    func() time.Time
	Evidence FactoryAdmissionEvidenceProvider
}

type FactoryAdmissionEvidenceProvider interface {
	RepoState(context.Context) (FactoryRepoState, error)
	OpenPRBlockers(context.Context, []string) (FactoryPRBlockerMatrix, error)
	MainCIBaseline(context.Context) (FactoryCIBaselineEvidence, error)
}

type FactoryRepoState struct {
	HeadSHA       string
	Dirty         bool
	TrackedAgents []string
}

type FactoryPRBlockerMatrix struct {
	Known    bool
	Blockers []FactoryOpenPRBlocker
}

type FactoryCIBaselineEvidence struct {
	Known    bool
	Baseline FactoryMainCIBaseline
}

func (e FactoryAdmissionEvaluator) EvaluateAdmission(ctx context.Context, spec FactoryAdmissionJobSpec) (FactoryAdmissionDecision, error) {
	if err := spec.Validate(); err != nil {
		return FactoryAdmissionDecision{}, err
	}
	return e.evaluate(ctx, spec.RunID, spec.WorkOrder)
}

func (e FactoryAdmissionEvaluator) EvaluateLocalPilot(ctx context.Context, spec FactoryLocalPilotJobSpec) (FactoryAdmissionDecision, error) {
	if err := spec.Validate(); err != nil {
		return FactoryAdmissionDecision{}, err
	}
	return e.evaluate(ctx, spec.RunID, spec.WorkOrder)
}

func (e FactoryAdmissionEvaluator) evaluate(ctx context.Context, runID string, work FactoryWorkOrder) (FactoryAdmissionDecision, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	now := e.now()
	generatedAt, err := parseFactoryAdmissionTime("generated_at", work.GeneratedAt)
	if err != nil {
		return FactoryAdmissionDecision{}, err
	}
	expiresAt, err := parseFactoryAdmissionTime("expires_at", work.ExpiresAt)
	if err != nil {
		return FactoryAdmissionDecision{}, err
	}

	reasons := []string{}
	if now.After(expiresAt) {
		reasons = append(reasons, FactoryAdmissionReasonExpiredWorkOrder)
	}
	if generatedAt.After(now) {
		reasons = append(reasons, FactoryAdmissionReasonFutureGeneratedAt)
	}

	repo := FactoryRepoState{}
	blockers := append([]FactoryOpenPRBlocker{}, work.OpenPRBlockers...)
	ciBaseline := work.MainCIBaseline
	if e.Evidence == nil {
		reasons = append(reasons, FactoryAdmissionReasonEvidenceProviderMissing)
	} else {
		collectedRepo, err := e.Evidence.RepoState(ctx)
		if err != nil {
			if isFactoryAdmissionContextError(err) {
				return FactoryAdmissionDecision{}, err
			}
			reasons = append(reasons, FactoryAdmissionReasonRepoStateUnknown)
		} else {
			repo = collectedRepo
			reasons = append(reasons, evaluateFactoryRepoState(work.BaseSHA, collectedRepo)...)
		}

		matrix, err := e.Evidence.OpenPRBlockers(ctx, work.AllowedFiles)
		if err != nil {
			if isFactoryAdmissionContextError(err) {
				return FactoryAdmissionDecision{}, err
			}
			if factoryAdmissionBlocksUnknownEvidence(work) {
				reasons = append(reasons, FactoryAdmissionReasonOpenPREvidenceUnknown)
			}
		} else if !matrix.Known {
			if factoryAdmissionBlocksUnknownEvidence(work) {
				reasons = append(reasons, FactoryAdmissionReasonOpenPREvidenceUnknown)
			}
		} else {
			blockers = append(blockers, matrix.Blockers...)
		}

		ci, err := e.Evidence.MainCIBaseline(ctx)
		if err != nil {
			if isFactoryAdmissionContextError(err) {
				return FactoryAdmissionDecision{}, err
			}
			if factoryAdmissionBlocksUnknownEvidence(work) {
				reasons = append(reasons, FactoryAdmissionReasonMainCIUnknown)
			}
		} else if !ci.Known {
			if factoryAdmissionBlocksUnknownEvidence(work) {
				reasons = append(reasons, FactoryAdmissionReasonMainCIUnknown)
			}
		} else {
			ciBaseline = ci.Baseline
		}
	}

	if len(blockers) > 0 {
		reasons = append(reasons, FactoryAdmissionReasonOpenPROverlap)
	}
	if ciBaseline.Status == FactoryCIStatusRed {
		reasons = append(reasons, FactoryAdmissionReasonMainCIRed)
	} else if ciBaseline.Status == FactoryCIStatusUnknown && factoryAdmissionBlocksUnknownEvidence(work) {
		reasons = append(reasons, FactoryAdmissionReasonMainCIUnknown)
	}

	reasons = uniqueNonEmptyFactoryAdmissionReasons(reasons)
	decision := FactoryAdmissionDecision{
		SchemaVersion: FactoryAdmissionJobSpecSchemaVersion,
		WorkOrderID:   work.WorkOrderID,
		RunID:         runID,
		EvaluatedAt:   now.UTC().Format(time.RFC3339Nano),
		Allowed:       len(reasons) == 0,
		Reasons:       reasons,
		LandingPolicy: work.LandingPolicy,
		DigestPolicy:  work.DigestPolicy,
		Evidence: FactoryDecisionEvidence{
			BaseSHA:            factoryDecisionBaseSHA(work.BaseSHA, repo.HeadSHA),
			OpenPRBlockerCount: len(blockers),
			MainCIStatus:       ciBaseline.Status,
			Stale:              now.After(expiresAt),
		},
	}
	if err := decision.Validate(); err != nil {
		return FactoryAdmissionDecision{}, fmt.Errorf("admission decision: %w", err)
	}
	return decision, nil
}

func (e FactoryAdmissionEvaluator) now() time.Time {
	if e.Clock != nil {
		return e.Clock().UTC()
	}
	return time.Now().UTC()
}

func evaluateFactoryRepoState(baseSHA string, repo FactoryRepoState) []string {
	reasons := []string{}
	if strings.TrimSpace(repo.HeadSHA) == "" {
		reasons = append(reasons, FactoryAdmissionReasonRepoHeadUnknown)
	} else if !factorySHAMatches(baseSHA, repo.HeadSHA) {
		reasons = append(reasons, FactoryAdmissionReasonBaseSHAMismatch)
	}
	if repo.Dirty {
		reasons = append(reasons, FactoryAdmissionReasonDirtyWorktree)
	}
	if len(repo.TrackedAgents) > 0 {
		reasons = append(reasons, FactoryAdmissionReasonTrackedAgents)
	}
	return reasons
}

func factorySHAMatches(want, got string) bool {
	want = strings.TrimSpace(strings.ToLower(want))
	got = strings.TrimSpace(strings.ToLower(got))
	if want == "" || got == "" {
		return false
	}
	return want == got || strings.HasPrefix(want, got) || strings.HasPrefix(got, want)
}

func factoryDecisionBaseSHA(workOrderSHA, repoSHA string) string {
	if strings.TrimSpace(repoSHA) != "" {
		return strings.TrimSpace(repoSHA)
	}
	return strings.TrimSpace(workOrderSHA)
}

func factoryAdmissionBlocksUnknownEvidence(work FactoryWorkOrder) bool {
	if work.UnknownEvidencePolicy != FactoryUnknownEvidenceAllowNonMutating {
		return true
	}
	return work.LandingPolicy != FactoryLandingPolicyOff
}

func uniqueNonEmptyFactoryAdmissionReasons(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, reason := range in {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			continue
		}
		if _, ok := seen[reason]; ok {
			continue
		}
		seen[reason] = struct{}{}
		out = append(out, reason)
	}
	return out
}

func isFactoryAdmissionContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
