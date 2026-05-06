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
	generatedAt, expiresAt, err := parseFactoryAdmissionWindow(work)
	if err != nil {
		return FactoryAdmissionDecision{}, err
	}

	reasons := evaluateFactoryAdmissionWindow(now, generatedAt, expiresAt)

	collected, err := e.collectFactoryAdmissionEvidence(ctx, work)
	if err != nil {
		return FactoryAdmissionDecision{}, err
	}
	reasons = append(reasons, collected.reasons...)
	reasons = append(reasons, evaluateFactoryAdmissionAggregate(work, collected.blockers, collected.ciBaseline)...)

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
			BaseSHA:            factoryDecisionBaseSHA(work.BaseSHA, collected.repo.HeadSHA),
			OpenPRBlockerCount: len(collected.blockers),
			MainCIStatus:       collected.ciBaseline.Status,
			Stale:              now.After(expiresAt),
		},
	}
	if err := decision.Validate(); err != nil {
		return FactoryAdmissionDecision{}, fmt.Errorf("admission decision: %w", err)
	}
	return decision, nil
}

func parseFactoryAdmissionWindow(work FactoryWorkOrder) (time.Time, time.Time, error) {
	generatedAt, err := parseFactoryAdmissionTime("generated_at", work.GeneratedAt)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	expiresAt, err := parseFactoryAdmissionTime("expires_at", work.ExpiresAt)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return generatedAt, expiresAt, nil
}

func evaluateFactoryAdmissionWindow(now, generatedAt, expiresAt time.Time) []string {
	reasons := []string{}
	if now.After(expiresAt) {
		reasons = append(reasons, FactoryAdmissionReasonExpiredWorkOrder)
	}
	if generatedAt.After(now) {
		reasons = append(reasons, FactoryAdmissionReasonFutureGeneratedAt)
	}
	return reasons
}

// factoryAdmissionEvidenceBundle gathers the three evidence-provider results
// and the reasons collected while consulting them. Pulling this off the
// evaluate() caller keeps that function under the cli/internal/ CC ceiling
// and makes the per-source error handling testable in isolation.
type factoryAdmissionEvidenceBundle struct {
	reasons    []string
	blockers   []FactoryOpenPRBlocker
	ciBaseline FactoryMainCIBaseline
	repo       FactoryRepoState
}

func (e FactoryAdmissionEvaluator) collectFactoryAdmissionEvidence(ctx context.Context, work FactoryWorkOrder) (factoryAdmissionEvidenceBundle, error) {
	bundle := factoryAdmissionEvidenceBundle{
		blockers:   append([]FactoryOpenPRBlocker{}, work.OpenPRBlockers...),
		ciBaseline: work.MainCIBaseline,
	}
	if e.Evidence == nil {
		bundle.reasons = append(bundle.reasons, FactoryAdmissionReasonEvidenceProviderMissing)
		return bundle, nil
	}

	if err := e.collectFactoryRepoEvidence(ctx, work, &bundle); err != nil {
		return factoryAdmissionEvidenceBundle{}, err
	}
	if err := e.collectFactoryBlockerEvidence(ctx, work, &bundle); err != nil {
		return factoryAdmissionEvidenceBundle{}, err
	}
	if err := e.collectFactoryCIEvidence(ctx, work, &bundle); err != nil {
		return factoryAdmissionEvidenceBundle{}, err
	}
	return bundle, nil
}

func (e FactoryAdmissionEvaluator) collectFactoryRepoEvidence(ctx context.Context, work FactoryWorkOrder, bundle *factoryAdmissionEvidenceBundle) error {
	collectedRepo, err := e.Evidence.RepoState(ctx)
	if err != nil {
		if isFactoryAdmissionContextError(err) {
			return err
		}
		bundle.reasons = append(bundle.reasons, FactoryAdmissionReasonRepoStateUnknown)
		return nil
	}
	bundle.repo = collectedRepo
	bundle.reasons = append(bundle.reasons, evaluateFactoryRepoState(work.BaseSHA, collectedRepo)...)
	return nil
}

func (e FactoryAdmissionEvaluator) collectFactoryBlockerEvidence(ctx context.Context, work FactoryWorkOrder, bundle *factoryAdmissionEvidenceBundle) error {
	matrix, err := e.Evidence.OpenPRBlockers(ctx, work.AllowedFiles)
	if err != nil {
		if isFactoryAdmissionContextError(err) {
			return err
		}
		if factoryAdmissionBlocksUnknownEvidence(work) {
			bundle.reasons = append(bundle.reasons, FactoryAdmissionReasonOpenPREvidenceUnknown)
		}
		return nil
	}
	if !matrix.Known {
		if factoryAdmissionBlocksUnknownEvidence(work) {
			bundle.reasons = append(bundle.reasons, FactoryAdmissionReasonOpenPREvidenceUnknown)
		}
		return nil
	}
	bundle.blockers = append(bundle.blockers, matrix.Blockers...)
	return nil
}

func (e FactoryAdmissionEvaluator) collectFactoryCIEvidence(ctx context.Context, work FactoryWorkOrder, bundle *factoryAdmissionEvidenceBundle) error {
	ci, err := e.Evidence.MainCIBaseline(ctx)
	if err != nil {
		if isFactoryAdmissionContextError(err) {
			return err
		}
		if factoryAdmissionBlocksUnknownEvidence(work) {
			bundle.reasons = append(bundle.reasons, FactoryAdmissionReasonMainCIUnknown)
		}
		return nil
	}
	if !ci.Known {
		if factoryAdmissionBlocksUnknownEvidence(work) {
			bundle.reasons = append(bundle.reasons, FactoryAdmissionReasonMainCIUnknown)
		}
		return nil
	}
	bundle.ciBaseline = ci.Baseline
	return nil
}

func evaluateFactoryAdmissionAggregate(work FactoryWorkOrder, blockers []FactoryOpenPRBlocker, ciBaseline FactoryMainCIBaseline) []string {
	reasons := []string{}
	if len(blockers) > 0 {
		reasons = append(reasons, FactoryAdmissionReasonOpenPROverlap)
	}
	switch {
	case ciBaseline.Status == FactoryCIStatusRed:
		reasons = append(reasons, FactoryAdmissionReasonMainCIRed)
	case ciBaseline.Status == FactoryCIStatusUnknown && factoryAdmissionBlocksUnknownEvidence(work):
		reasons = append(reasons, FactoryAdmissionReasonMainCIUnknown)
	}
	return reasons
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
