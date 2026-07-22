package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNativeHostedGateBindsStrictProtectionAppAndLatestExactHead(t *testing.T) {
	if args := nativeRESTArgs("endpoint", "projection"); !hasArg(args, "Accept: application/vnd.github+json") || !hasArg(args, "X-GitHub-Api-Version: 2026-03-10") {
		t.Fatalf("REST provenance headers = %#v", args)
	}
	pr, view := nativeHostedIdentityForTest()
	appID := int64(15368)
	protection := nativeProtection{Strict: true}
	protection.Checks = append(protection.Checks, struct {
		Context string `json:"context"`
		AppID   *int64 `json:"app_id"`
	}{Context: "summary", AppID: &appID})
	var runs nativeCheckRuns
	runs.TotalCount = 1
	run := struct {
		Name       string `json:"name"`
		HeadSHA    string `json:"head_sha"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		App        struct {
			ID int64 `json:"id"`
		} `json:"app"`
	}{Name: "summary", HeadSHA: view.HeadRefOID, Status: "completed", Conclusion: "success"}
	run.App.ID = appID
	runs.CheckRuns = append(runs.CheckRuns, run)
	provider := &NativeProviders{ghRun: func(_ context.Context, args ...string) ([]byte, error) {
		switch {
		case len(args) > 1 && args[0] == "pr" && args[1] == "view":
			return json.Marshal(view)
		case hasArgContaining(args, "/protection/required_status_checks"):
			if flagValue(args, "--jq") != "{strict: .strict, contexts: .contexts, checks: .checks}" {
				t.Fatalf("protection projection argv = %#v", args)
			}
			return json.Marshal(protection)
		case hasArgContaining(args, "/check-runs?filter=latest&per_page=100"):
			if !strings.Contains(flagValue(args, "--jq"), "app: {id: .app.id}") {
				t.Fatalf("check projection argv = %#v", args)
			}
			return json.Marshal(runs)
		default:
			return nil, errors.New("unexpected gh command")
		}
	}}
	gate, err := provider.HostedGate(context.Background(), pr)
	if err != nil {
		t.Fatal(err)
	}
	if gate.Repository != pr.Repository || gate.BaseRef != pr.BaseRef || gate.BaseOID != view.BaseRefOID || gate.Head != view.HeadRefOID || !gate.Strict || !isHex(gate.ProtectionDigest, 64) || len(gate.RequiredChecks) != 1 || len(gate.Checks) != 1 || gate.RequiredChecks[0].AppID != "15368" || gate.Checks[0].Status != "COMPLETED" || gate.Checks[0].Conclusion != "SUCCESS" {
		t.Fatalf("native hosted gate = %#v", gate)
	}
	bead := DeliveryBead{Record: DeliveryRecord{Repository: pr.Repository, Epoch: DeliveryEpoch{BaseRef: pr.BaseRef, BaseOID: view.BaseRefOID, Head: view.HeadRefOID}}}
	if qualified, reason, err := qualifyHostedGate(gate, bead); err != nil || !qualified || reason != "" {
		t.Fatalf("qualified gate = %t, %q, %v", qualified, reason, err)
	}
}

func TestNativeHostedGatePreservesContextOnlyProtectionAsUnqualified(t *testing.T) {
	pr, view := nativeHostedIdentityForTest()
	protection := nativeProtection{Strict: true, Contexts: []string{"legacy"}}
	provider := &NativeProviders{ghRun: func(_ context.Context, args ...string) ([]byte, error) {
		switch {
		case len(args) > 1 && args[0] == "pr" && args[1] == "view":
			return json.Marshal(view)
		case hasArgContaining(args, "/protection/required_status_checks"):
			return json.Marshal(protection)
		case hasArgContaining(args, "/check-runs?filter=latest&per_page=100"):
			return json.Marshal(nativeCheckRuns{})
		default:
			return nil, errors.New("unexpected gh command")
		}
	}}
	gate, err := provider.HostedGate(context.Background(), pr)
	if err != nil || len(gate.RequiredChecks) != 1 || gate.RequiredChecks[0].AppID != "0" {
		t.Fatalf("context-only gate = %#v, %v", gate, err)
	}
	bead := DeliveryBead{Record: DeliveryRecord{Repository: pr.Repository, Epoch: DeliveryEpoch{BaseRef: pr.BaseRef, BaseOID: view.BaseRefOID, Head: view.HeadRefOID}}}
	if _, _, err := qualifyHostedGate(gate, bead); err == nil {
		t.Fatal("context-only protection acquired fabricated app authority")
	}
}

func TestNativeArmMergeUsesOneExactGraphQLMutationAndColdObserve(t *testing.T) {
	pr, view := nativeHostedIdentityForTest()
	arm := nativeMergeArmForTest(pr, view)
	mutationCount := 0
	provider := &NativeProviders{ghRun: func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) > 1 && args[0] == "pr" && args[1] == "view" {
			return json.Marshal(view)
		}
		if len(args) > 2 && reflect.DeepEqual(args[:3], []string{"api", "graphql", "-f"}) {
			mutationCount++
			if !strings.Contains(args[3], "enablePullRequestAutoMerge") || !strings.Contains(args[3], "mergeMethod: SQUASH") {
				t.Fatalf("GraphQL mutation = %q", args[3])
			}
			if flagValue(args, "expectedHeadOid") != "" {
				t.Fatalf("GraphQL variables were not passed with -F pairs: %#v", args)
			}
			if !hasArg(args, "expectedHeadOid="+arm.Head) || !hasArg(args, "pullRequestId="+arm.NodeID) || !hasArg(args, "clientMutationId="+arm.EffectID) {
				t.Fatalf("GraphQL exact variables = %#v", args)
			}
			view.AutoMergeRequest = &nativeAutoMergeRequest{MergeMethod: "SQUASH"}
			response := struct {
				Data struct {
					Enable struct {
						ClientMutationID string         `json:"clientMutationId"`
						PullRequest      nativeHostedPR `json:"pullRequest"`
					} `json:"enablePullRequestAutoMerge"`
				} `json:"data"`
			}{}
			response.Data.Enable.ClientMutationID, response.Data.Enable.PullRequest = arm.EffectID, view
			return json.Marshal(response)
		}
		return nil, errors.New("unexpected gh command")
	}}
	armed, err := provider.ArmMerge(context.Background(), arm)
	if err != nil || armed != arm || mutationCount != 1 {
		t.Fatalf("ArmMerge = %#v, %v, mutations=%d", armed, err, mutationCount)
	}
	armed, err = provider.ArmMerge(context.Background(), arm)
	if err != nil || armed != arm || mutationCount != 1 {
		t.Fatalf("cold ArmMerge = %#v, %v, mutations=%d", armed, err, mutationCount)
	}
}

func TestNativeLandingReadsExactMergedHeadCommitTreeAndParents(t *testing.T) {
	pr, view := nativeHostedIdentityForTest()
	view.State = "MERGED"
	view.MergeCommit = &nativeMergeCommit{OID: strings.Repeat("d", 40)}
	commit := struct {
		SHA  string `json:"sha"`
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
		Parents []struct {
			SHA string `json:"sha"`
		} `json:"parents"`
	}{SHA: view.MergeCommit.OID}
	commit.Tree.SHA = strings.Repeat("e", 40)
	commit.Parents = append(commit.Parents, struct {
		SHA string `json:"sha"`
	}{SHA: view.BaseRefOID})
	provider := &NativeProviders{ghRun: func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) > 1 && args[0] == "pr" && args[1] == "view" {
			return json.Marshal(view)
		}
		if hasArgContaining(args, "/git/commits/"+view.MergeCommit.OID) {
			if !strings.Contains(flagValue(args, "--jq"), "parents") {
				t.Fatalf("landing projection argv = %#v", args)
			}
			return json.Marshal(commit)
		}
		return nil, errors.New("unexpected gh command")
	}}
	landing, found, err := provider.Landing(context.Background(), pr)
	if err != nil || !found || landing.PRID != pr.ID || landing.Head != view.HeadRefOID || landing.SHA != view.MergeCommit.OID || landing.Tree != commit.Tree.SHA || !reflect.DeepEqual(landing.Parents, []string{view.BaseRefOID}) {
		t.Fatalf("Landing = %#v, found=%t, err=%v", landing, found, err)
	}
}

func nativeHostedIdentityForTest() (PullRequest, nativeHostedPR) {
	pr := PullRequest{ID: "pr-stable", EffectID: strings.Repeat("a", 64), Repository: "boshu2/agentops", BaseRef: "main", Branch: "gc/delivery/handoff", NodeID: "PR_node", Number: "17", URL: "https://example.invalid/pull/17"}
	view := nativeHostedPR{NodeID: pr.NodeID, Number: 17, State: "OPEN", MergeState: "CLEAN", BaseRefName: pr.BaseRef, BaseRefOID: strings.Repeat("b", 40), HeadRefName: pr.Branch, HeadRefOID: strings.Repeat("c", 40)}
	return pr, view
}

func nativeMergeArmForTest(pr PullRequest, view nativeHostedPR) MergeArm {
	return MergeArm{ID: strings.Repeat("1", 64), EffectID: strings.Repeat("2", 64), PRID: pr.ID, Repository: pr.Repository, NodeID: pr.NodeID, Number: pr.Number, Branch: pr.Branch, Head: view.HeadRefOID, BaseRef: view.BaseRefName, BaseOID: view.BaseRefOID, ProtectionDigest: strings.Repeat("3", 64), GateDigest: strings.Repeat("4", 64)}
}

func hasArgContaining(args []string, fragment string) bool {
	for _, arg := range args {
		if strings.Contains(arg, fragment) {
			return true
		}
	}
	return false
}

func hasArg(args []string, exact string) bool {
	for _, arg := range args {
		if arg == exact {
			return true
		}
	}
	return false
}
