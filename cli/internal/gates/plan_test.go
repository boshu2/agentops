package gates

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// panicRunner fails the test if any check is executed. Plan() must select
// without ever routing a check through the runner, so a Plan run that touches
// this runner is a bug.
type panicRunner struct{ t *testing.T }

func (r panicRunner) Run(context.Context, ports.GateRunRequest) (ports.GateVerdict, error) {
	r.t.Fatalf("plan executed a check via the runner; dry-run must run nothing")
	return ports.GateVerdict{}, nil
}

func planOrch(t *testing.T, files ChangedFilesPort) *Orchestrator {
	t.Helper()
	return NewOrchestrator(sampleRegistry(t), panicRunner{t}, files, "/repo")
}

func planIDs(checks []PlanCheck) map[string]PlanCheck {
	m := map[string]PlanCheck{}
	for _, c := range checks {
		m[c.Name] = c
	}
	return m
}

func TestOrchestrator_PlanFullSelectsEveryTierMatchedWithoutExecuting(t *testing.T) {
	o := planOrch(t, fakeFiles{})
	plan, err := o.Plan(context.Background(), RunOptions{Mode: Full})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	sel := planIDs(plan.Selected)
	for _, id := range []string{"go", "docs", "always", "fullonly"} {
		if _, ok := sel[id]; !ok {
			t.Errorf("full plan should select %q; selected=%v", id, keys(sel))
		}
	}
	if len(plan.Selected) != 4 {
		t.Fatalf("full plan selected %d checks, want 4", len(plan.Selected))
	}
	if len(plan.Skipped) != 0 {
		t.Fatalf("full plan skipped %d checks, want 0", len(plan.Skipped))
	}
	// Selection facts are surfaced, not verdicts.
	if got := sel["go"]; !got.Blocking || got.Tier != "fast,full" || got.Reason == "" {
		t.Fatalf("go plan entry = %+v, want blocking fast,full with a reason", got)
	}
	if got := sel["docs"]; got.Blocking {
		t.Fatalf("docs is advisory; plan marked it blocking: %+v", got)
	}
}

func TestOrchestrator_PlanFastRoutesOnChangedFilesWithoutExecuting(t *testing.T) {
	t.Run("no changed files selects only always-run", func(t *testing.T) {
		o := planOrch(t, fakeFiles{})
		plan, err := o.Plan(context.Background(), RunOptions{Mode: Fast, Scope: ScopeHead})
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		sel := planIDs(plan.Selected)
		if len(sel) != 1 {
			t.Fatalf("fast plan (no changes) selected %v, want only always", keys(sel))
		}
		if _, ok := sel["always"]; !ok {
			t.Fatalf("fast plan (no changes) should select always; got %v", keys(sel))
		}
		skip := planIDs(plan.Skipped)
		for _, id := range []string{"go", "docs", "fullonly"} {
			if _, ok := skip[id]; !ok {
				t.Errorf("fast plan (no changes) should skip %q; skipped=%v", id, keys(skip))
			}
		}
		if r := skip["fullonly"].Reason; !strings.Contains(r, "tier") {
			t.Errorf("fullonly skip reason = %q, want a tier-mismatch reason", r)
		}
	})

	t.Run("changed cli file additionally selects the routed check", func(t *testing.T) {
		o := planOrch(t, fakeFiles{files: []string{"cli/main.go"}})
		plan, err := o.Plan(context.Background(), RunOptions{Mode: Fast, Scope: ScopeHead})
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		sel := planIDs(plan.Selected)
		for _, id := range []string{"always", "go"} {
			if _, ok := sel[id]; !ok {
				t.Errorf("fast plan (cli change) should select %q; selected=%v", id, keys(sel))
			}
		}
		if _, ok := sel["docs"]; ok {
			t.Errorf("fast plan (cli change) must not select docs; selected=%v", keys(sel))
		}
		if plan.ChangedCount != 1 {
			t.Fatalf("plan ChangedCount = %d, want 1", plan.ChangedCount)
		}
		if r := sel["go"].Reason; !strings.Contains(r, "cli/main.go") {
			t.Errorf("go selection reason = %q, want it to cite the matched file", r)
		}
	})
}

func TestPlan_JSONIsAPlanShapedContract(t *testing.T) {
	o := planOrch(t, fakeFiles{files: []string{"cli/main.go"}})
	plan, err := o.Plan(context.Background(), RunOptions{Mode: Fast, Scope: ScopeHead})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	raw, err := plan.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var decoded struct {
		Plan struct {
			DryRun        bool   `json:"dry_run"`
			Mode          string `json:"mode"`
			Scope         string `json:"scope"`
			SelectedCount int    `json:"selected_count"`
			SkippedCount  int    `json:"skipped_count"`
		} `json:"plan"`
		Selected []struct {
			Name     string `json:"name"`
			Tier     string `json:"tier"`
			Blocking bool   `json:"blocking"`
			Reason   string `json:"reason"`
		} `json:"selected"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal plan JSON: %v\n%s", err, raw)
	}
	if !decoded.Plan.DryRun {
		t.Errorf("plan JSON dry_run = false, want true (must be distinguishable from a run report)")
	}
	if decoded.Plan.Mode != "fast" || decoded.Plan.Scope != "head" {
		t.Errorf("plan JSON header = %+v, want fast/head", decoded.Plan)
	}
	if decoded.Plan.SelectedCount != len(decoded.Selected) {
		t.Errorf("selected_count %d != len(selected) %d", decoded.Plan.SelectedCount, len(decoded.Selected))
	}
	byName := map[string]bool{}
	for _, c := range decoded.Selected {
		byName[c.Name] = true
	}
	if !byName["go"] || !byName["always"] {
		t.Errorf("plan JSON selected = %v, want go and always", decoded.Selected)
	}
}

func TestPlan_HumanRendersWouldRunAndSkip(t *testing.T) {
	plan := &Plan{
		Mode:  Fast,
		Scope: ScopeHead,
		Selected: []PlanCheck{
			{Name: "go", Tier: "fast,full", Blocking: true, Reason: "selected: changed file \"cli/main.go\" matched \"cli/**\"", WorkflowBacking: "bash scripts/go", ArtifactPath: "scripts/go", RepairHint: "bash scripts/go"},
		},
		Skipped: []PlanCheck{
			{Name: "docs", Tier: "fast,full", Blocking: false, Reason: "skipped: no changed file matched match globs docs/**", WorkflowBacking: "bash scripts/docs", ArtifactPath: "scripts/docs", RepairHint: "bash scripts/docs"},
		},
		ForeignRepo: false,
	}
	var b strings.Builder
	plan.Human(&b)
	out := b.String()
	for _, want := range []string{
		"dry-run — no checks executed",
		"would run:",
		"RUN   go",
		"blocking",
		"would skip:",
		"SKIP  docs",
		"advisory",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human plan missing %q\n---\n%s", want, out)
		}
	}
	// Dry-run output must never assert a verdict; no PASS/FAIL marks belong here.
	for _, forbidden := range []string{"PASS", "FAIL"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("human plan must not carry a verdict mark %q\n---\n%s", forbidden, out)
		}
	}
}

func TestPlan_HumanForeignRepoSuppressesSkipDetail(t *testing.T) {
	plan := &Plan{
		Mode:        Fast,
		Scope:       ScopeHead,
		Selected:    []PlanCheck{{Name: "always", Tier: "fast,full", Blocking: true, Reason: "selected: check has no match globs and always runs"}},
		Skipped:     []PlanCheck{{Name: "go", Tier: "fast,full", Blocking: true, Reason: "skipped: no changed file matched match globs cli/**"}},
		ForeignRepo: true,
	}
	var b strings.Builder
	plan.Human(&b)
	out := b.String()
	if strings.Contains(out, "would skip:") || strings.Contains(out, "SKIP  go") {
		t.Errorf("foreign-repo plan must aggregate skips, not name internal scripts\n---\n%s", out)
	}
	if !strings.Contains(out, "would skip 1 checks") {
		t.Errorf("foreign-repo plan should carry an aggregate skip line\n---\n%s", out)
	}
}

func keys(m map[string]PlanCheck) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
