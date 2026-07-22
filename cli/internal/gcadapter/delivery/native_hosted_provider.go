package delivery

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const githubRESTAPIVersion = "2026-03-10"

type nativeAutoMergeRequest struct {
	MergeMethod string `json:"mergeMethod"`
}

type nativeMergeCommit struct {
	OID string `json:"oid"`
}

type nativeHostedPR struct {
	NodeID           string                  `json:"id"`
	Number           int                     `json:"number"`
	State            string                  `json:"state"`
	Draft            bool                    `json:"isDraft"`
	MergeState       string                  `json:"mergeStateStatus"`
	BaseRefName      string                  `json:"baseRefName"`
	BaseRefOID       string                  `json:"baseRefOid"`
	HeadRefName      string                  `json:"headRefName"`
	HeadRefOID       string                  `json:"headRefOid"`
	AutoMergeRequest *nativeAutoMergeRequest `json:"autoMergeRequest"`
	MergeCommit      *nativeMergeCommit      `json:"mergeCommit"`
}

type nativeProtection struct {
	Strict   bool     `json:"strict"`
	Contexts []string `json:"contexts"`
	Checks   []struct {
		Context string `json:"context"`
		AppID   *int64 `json:"app_id"`
	} `json:"checks"`
}

type nativeCheckRuns struct {
	TotalCount int `json:"total_count"`
	CheckRuns  []struct {
		Name       string `json:"name"`
		HeadSHA    string `json:"head_sha"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		App        struct {
			ID int64 `json:"id"`
		} `json:"app"`
	} `json:"check_runs"`
}

func (p *NativeProviders) HostedGate(ctx context.Context, pr PullRequest) (HostedGate, error) {
	view, err := p.hostedPRView(ctx, pr)
	if err != nil {
		return HostedGate{}, err
	}
	protectionOutput, err := p.gh(ctx, nativeRESTArgs("repos/"+pr.Repository+"/branches/"+url.PathEscape(pr.BaseRef)+"/protection/required_status_checks", "{strict: .strict, contexts: .contexts, checks: .checks}")...)
	if err != nil {
		return HostedGate{}, fmt.Errorf("observe branch protection: %w", err)
	}
	var protection nativeProtection
	if err := decodeStrict(protectionOutput, &protection); err != nil {
		return HostedGate{}, errors.New("branch protection lacks strict required status checks")
	}
	required := make([]HostedCheck, 0, len(protection.Checks))
	for _, check := range protection.Checks {
		appID := int64(0)
		if check.AppID != nil {
			appID = *check.AppID
		}
		required = append(required, HostedCheck{AppID: strconv.FormatInt(appID, 10), Context: check.Context})
	}
	// Context-only requirements have no immutable app authority. Preserve them
	// as app_id=0 so the deterministic gate rejects rather than silently
	// attributing them to whichever integration reported most recently.
	if len(required) == 0 {
		for _, contextName := range protection.Contexts {
			required = append(required, HostedCheck{AppID: "0", Context: contextName})
		}
	}
	sortHostedChecks(required)
	protectionDigest, err := valueDigest(struct {
		Strict bool          `json:"strict"`
		Checks []HostedCheck `json:"checks"`
	}{Strict: protection.Strict, Checks: required})
	if err != nil {
		return HostedGate{}, err
	}
	checkOutput, err := p.gh(ctx, nativeRESTArgs("repos/"+pr.Repository+"/commits/"+view.HeadRefOID+"/check-runs?filter=latest&per_page=100", "{total_count: .total_count, check_runs: [.check_runs[] | {name: .name, head_sha: .head_sha, status: .status, conclusion: .conclusion, app: {id: .app.id}}]}")...)
	if err != nil {
		return HostedGate{}, fmt.Errorf("observe required check runs: %w", err)
	}
	var runs nativeCheckRuns
	if err := decodeStrict(checkOutput, &runs); err != nil || runs.TotalCount > len(runs.CheckRuns) || len(runs.CheckRuns) > 100 {
		return HostedGate{}, errors.New("required check run observation is incomplete or truncated")
	}
	observed := make([]HostedCheck, 0, len(required))
	for _, want := range required {
		matches := 0
		for _, run := range runs.CheckRuns {
			if run.HeadSHA != view.HeadRefOID {
				return HostedGate{}, errors.New("check run does not bind exact PR head")
			}
			if strconv.FormatInt(run.App.ID, 10) == want.AppID && run.Name == want.Context {
				matches++
				observed = append(observed, HostedCheck{AppID: want.AppID, Context: want.Context, Status: strings.ToUpper(run.Status), Conclusion: strings.ToUpper(run.Conclusion)})
			}
		}
		if matches > 1 {
			return HostedGate{}, errors.New("latest check run identity is ambiguous")
		}
	}
	sortHostedChecks(observed)
	return HostedGate{
		Repository: pr.Repository, BaseRef: view.BaseRefName, BaseOID: view.BaseRefOID,
		Head: view.HeadRefOID, PRState: strings.ToUpper(view.State), Draft: view.Draft,
		MergeState: strings.ToUpper(view.MergeState), AutoMergeEnabled: view.AutoMergeRequest != nil,
		Strict: protection.Strict, ProtectionDigest: protectionDigest,
		RequiredChecks: required, Checks: observed,
	}, nil
}

func (p *NativeProviders) ObserveMerge(ctx context.Context, arm MergeArm) (MergeObservation, error) {
	if err := validateMergeArm(arm); err != nil {
		return MergeObservation{}, err
	}
	view, err := p.hostedArmView(ctx, arm)
	if err != nil {
		return MergeObservation{}, err
	}
	if view.HeadRefOID != arm.Head || view.BaseRefName != arm.BaseRef {
		return MergeObservation{State: "refused", Reason: "pr_head_or_base_changed"}, nil
	}
	if strings.EqualFold(view.State, "MERGED") {
		return MergeObservation{State: "landed"}, nil
	}
	if !strings.EqualFold(view.State, "OPEN") || view.Draft {
		return MergeObservation{State: "refused", Reason: "pr_not_open"}, nil
	}
	if view.BaseRefOID != arm.BaseOID {
		return MergeObservation{State: "refused", Reason: "base_oid_changed"}, nil
	}
	if view.AutoMergeRequest == nil {
		return MergeObservation{State: "absent"}, nil
	}
	if strings.ToUpper(view.AutoMergeRequest.MergeMethod) != "SQUASH" {
		return MergeObservation{State: "refused", Reason: "wrong_merge_method"}, nil
	}
	return MergeObservation{State: "armed"}, nil
}

func (p *NativeProviders) ArmMerge(ctx context.Context, arm MergeArm) (MergeArm, error) {
	if err := validateMergeArm(arm); err != nil {
		return MergeArm{}, err
	}
	observation, err := p.ObserveMerge(ctx, arm)
	if err != nil {
		return MergeArm{}, err
	}
	if observation.State == "armed" {
		return arm, nil
	}
	if observation.State != "absent" {
		return MergeArm{}, errors.New("auto-merge create found conflicting hosted state")
	}
	const query = `mutation EnableAgentOpsAutoMerge($pullRequestId: ID!, $expectedHeadOid: GitObjectID!, $clientMutationId: String!) {
  enablePullRequestAutoMerge(input: {pullRequestId: $pullRequestId, expectedHeadOid: $expectedHeadOid, mergeMethod: SQUASH, clientMutationId: $clientMutationId}) {
    clientMutationId
    pullRequest { id state isDraft baseRefName baseRefOid headRefOid autoMergeRequest { mergeMethod } }
  }
}`
	output, err := p.gh(ctx, "api", "graphql", "-f", "query="+query, "-F", "pullRequestId="+arm.NodeID, "-F", "expectedHeadOid="+arm.Head, "-F", "clientMutationId="+arm.EffectID)
	if err != nil {
		return MergeArm{}, fmt.Errorf("enable native PR auto-merge: %w", err)
	}
	var response struct {
		Data struct {
			Enable struct {
				ClientMutationID string         `json:"clientMutationId"`
				PullRequest      nativeHostedPR `json:"pullRequest"`
			} `json:"enablePullRequestAutoMerge"`
		} `json:"data"`
	}
	if err := decodeStrict(output, &response); err != nil {
		return MergeArm{}, errors.New("auto-merge mutation returned invalid strict JSON")
	}
	view := response.Data.Enable.PullRequest
	if response.Data.Enable.ClientMutationID != arm.EffectID || view.NodeID != arm.NodeID || !strings.EqualFold(view.State, "OPEN") || view.Draft || view.BaseRefName != arm.BaseRef || view.BaseRefOID != arm.BaseOID || view.HeadRefOID != arm.Head || view.AutoMergeRequest == nil || strings.ToUpper(view.AutoMergeRequest.MergeMethod) != "SQUASH" {
		return MergeArm{}, errors.New("auto-merge mutation returned conflicting exact identity")
	}
	return arm, nil
}

func (p *NativeProviders) Landing(ctx context.Context, pr PullRequest) (Landing, bool, error) {
	view, err := p.hostedPRView(ctx, pr)
	if err != nil {
		return Landing{}, false, err
	}
	if !strings.EqualFold(view.State, "MERGED") {
		return Landing{}, false, nil
	}
	if view.MergeCommit == nil || !isHex(strings.ToLower(view.MergeCommit.OID), 40) {
		return Landing{}, false, errors.New("merged PR lacks exact merge commit")
	}
	output, err := p.gh(ctx, nativeRESTArgs("repos/"+pr.Repository+"/git/commits/"+view.MergeCommit.OID, "{sha: .sha, tree: {sha: .tree.sha}, parents: [.parents[] | {sha: .sha}]}")...)
	if err != nil {
		return Landing{}, false, fmt.Errorf("observe landed commit: %w", err)
	}
	var commit struct {
		SHA  string `json:"sha"`
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
		Parents []struct {
			SHA string `json:"sha"`
		} `json:"parents"`
	}
	if err := decodeStrict(output, &commit); err != nil || commit.SHA != view.MergeCommit.OID || !isHex(strings.ToLower(commit.Tree.SHA), 40) {
		return Landing{}, false, errors.New("landed commit response has conflicting identity")
	}
	parents := make([]string, len(commit.Parents))
	for index := range commit.Parents {
		if !isHex(strings.ToLower(commit.Parents[index].SHA), 40) {
			return Landing{}, false, errors.New("landed commit parent is invalid")
		}
		parents[index] = commit.Parents[index].SHA
	}
	return Landing{PRID: pr.ID, Head: view.HeadRefOID, SHA: commit.SHA, Tree: commit.Tree.SHA, Parents: parents}, true, nil
}

func (p *NativeProviders) hostedPRView(ctx context.Context, pr PullRequest) (nativeHostedPR, error) {
	if pr.Repository == "" || pr.NodeID == "" || pr.Number == "" || pr.BaseRef == "" || pr.Branch == "" {
		return nativeHostedPR{}, errors.New("hosted PR lacks stable actual identity")
	}
	number, err := strconv.Atoi(pr.Number)
	if err != nil || number < 1 {
		return nativeHostedPR{}, errors.New("hosted PR number is invalid")
	}
	output, err := p.gh(ctx, "pr", "view", pr.Number, "--repo", pr.Repository, "--json", "id,number,state,isDraft,mergeStateStatus,baseRefName,baseRefOid,headRefName,headRefOid,autoMergeRequest,mergeCommit")
	if err != nil {
		return nativeHostedPR{}, fmt.Errorf("observe hosted PR: %w", err)
	}
	var view nativeHostedPR
	if err := decodeStrict(output, &view); err != nil || view.NodeID != pr.NodeID || view.Number != number || view.BaseRefName != pr.BaseRef || view.HeadRefName != pr.Branch {
		return nativeHostedPR{}, errors.New("hosted PR view conflicts with stable actual identity")
	}
	if !isHex(strings.ToLower(view.HeadRefOID), 40) || !isHex(strings.ToLower(view.BaseRefOID), 40) {
		return nativeHostedPR{}, errors.New("hosted PR view lacks exact base/head OIDs")
	}
	return view, nil
}

func (p *NativeProviders) hostedArmView(ctx context.Context, arm MergeArm) (nativeHostedPR, error) {
	return p.hostedPRView(ctx, PullRequest{ID: arm.PRID, Repository: arm.Repository, BaseRef: arm.BaseRef, Branch: arm.Branch, NodeID: arm.NodeID, Number: arm.Number})
}

func validateMergeArm(arm MergeArm) error {
	if !isHex(arm.ID, 64) || !isHex(arm.EffectID, 64) || arm.PRID == "" || arm.Repository == "" || arm.NodeID == "" || arm.Number == "" || !strings.HasPrefix(arm.Branch, "gc/delivery/") || !isHex(arm.Head, 40) || arm.BaseRef == "" || !isHex(arm.BaseOID, 40) || !isHex(arm.ProtectionDigest, 64) || !isHex(arm.GateDigest, 64) {
		return errors.New("merge arm has incomplete exact identity")
	}
	return nil
}

func sortHostedChecks(checks []HostedCheck) {
	sort.Slice(checks, func(left, right int) bool {
		leftID, _ := strconv.ParseInt(checks[left].AppID, 10, 64)
		rightID, _ := strconv.ParseInt(checks[right].AppID, 10, 64)
		if leftID != rightID {
			return leftID < rightID
		}
		return checks[left].Context < checks[right].Context
	})
}

func nativeRESTArgs(endpoint, projection string) []string {
	return []string{"api", "--method", "GET", "-H", "Accept: application/vnd.github+json", "-H", "X-GitHub-Api-Version: " + githubRESTAPIVersion, endpoint, "--jq", projection}
}
