package gates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// PlanCheck is one check the orchestrator would run (or skip) for an
// invocation, projected WITHOUT executing the check or its script runner. It
// carries only static registry facts plus the routing reason — never a verdict,
// duration, or captured output, because no check runs to produce them.
type PlanCheck struct {
	// Name is the check ID (e.g. "go.vet").
	Name string
	// Tier renders the check's tier membership ("fast", "full", "fast,full").
	Tier string
	// Blocking is the failure semantics: a blocking check would fail the run on
	// FAIL, an advisory one would not.
	Blocking bool
	// Reason is the selection reason (for Selected) or the skip reason (for
	// Skipped) the orchestrator computed for this invocation.
	Reason string
	// WorkflowBacking is the command-shaped backing visible in CI/workflows.
	WorkflowBacking string
	// ArtifactPath is the local artifact that implements the check.
	ArtifactPath string
	// RepairHint is the operator-facing rerun/repair hint.
	RepairHint string
}

func newPlanCheck(c Check, reason string) PlanCheck {
	return PlanCheck{
		Name:            c.ID,
		Tier:            tierString(c.Tiers),
		Blocking:        c.Blocking,
		Reason:          reason,
		WorkflowBacking: c.WorkflowBacking(),
		ArtifactPath:    c.ArtifactPath(),
		RepairHint:      c.EffectiveRepairHint(),
	}
}

// Plan is the dry-run projection of a gate check invocation: the checks that
// would run and the checks that would be skipped (each with its routing
// reason), computed WITHOUT executing any check. Selection itself still reads
// repository state (git changed-file resolution, repo detection) — read-only
// probes, never a check body. It is the honest form of "show
// what would happen" — the same tier filter and changed-file routing the real
// run uses, minus every process/filesystem effect the checks themselves carry.
type Plan struct {
	Mode         Tier
	Scope        Scope
	ChangedCount int
	Selected     []PlanCheck
	Skipped      []PlanCheck
	// ForeignRepo is true when the run root is not the agentops repository. Human
	// suppresses the per-check skip wall there (it names agentops-internal
	// backing scripts that do not exist in the user's repo); JSON always carries
	// every row.
	ForeignRepo bool
}

// Plan returns the dry-run projection of a run: which checks would run and which
// would be skipped (with reasons), computed WITHOUT executing any check or its
// script runner. It reuses the exact selection path the real run uses
// (selectCheckPlans: tier filter, then Fast-mode changed-file routing with the
// full-run invalidation escape hatch), so a plan can never drift from what an
// actual run would select. It is the engine behind `ao gate check --dry-run`.
func (o *Orchestrator) Plan(ctx context.Context, opts RunOptions) (*Plan, error) {
	selected, skipped, changed, err := o.selectCheckPlans(ctx, opts)
	if err != nil {
		return nil, err
	}
	plan := &Plan{
		Mode:         opts.Mode,
		Scope:        opts.Scope,
		ChangedCount: len(changed),
		Selected:     make([]PlanCheck, 0, len(selected)),
		Skipped:      make([]PlanCheck, 0, len(skipped)),
		ForeignRepo:  !IsAgentOpsRepo(o.repoRoot),
	}
	for _, sel := range selected {
		plan.Selected = append(plan.Selected, newPlanCheck(sel.Check, sel.Reason))
	}
	for _, skip := range skipped {
		plan.Skipped = append(plan.Skipped, newPlanCheck(skip.Check, skip.Reason))
	}
	return plan, nil
}

// ---- JSON wire format (a plan-shaped subset of the run report contract) ----

type jsonPlan struct {
	Plan     jsonPlanRun     `json:"plan"`
	Selected []jsonPlanCheck `json:"selected"`
	Skipped  []jsonPlanCheck `json:"skipped,omitempty"`
}

type jsonPlanRun struct {
	// DryRun is always true; it lets a JSON consumer tell a plan from a run
	// report (which has no such field) unambiguously.
	DryRun            bool   `json:"dry_run"`
	Mode              string `json:"mode"`
	Scope             string `json:"scope"`
	ChangedFilesCount int    `json:"changed_files_count"`
	SelectedCount     int    `json:"selected_count"`
	SkippedCount      int    `json:"skipped_count"`
}

type jsonPlanCheck struct {
	Name            string `json:"name"`
	Tier            string `json:"tier"`
	Blocking        bool   `json:"blocking"`
	Reason          string `json:"reason"`
	WorkflowBacking string `json:"workflow_backing"`
	ArtifactPath    string `json:"artifact_path"`
	RepairHint      string `json:"repair_hint"`
}

// JSON renders the plan as the wire contract: a `plan` header plus `selected`
// and `skipped` arrays. It mirrors the run report's shape minus the fields that
// only exist after execution (status, log_tail, duration).
func (p *Plan) JSON() ([]byte, error) {
	jp := jsonPlan{
		Plan: jsonPlanRun{
			DryRun:            true,
			Mode:              modeString(p.Mode),
			Scope:             string(p.Scope),
			ChangedFilesCount: p.ChangedCount,
			SelectedCount:     len(p.Selected),
			SkippedCount:      len(p.Skipped),
		},
		Selected: jsonPlanChecks(p.Selected),
	}
	if len(p.Skipped) > 0 {
		jp.Skipped = jsonPlanChecks(p.Skipped)
	}
	out, err := json.MarshalIndent(jp, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("gates: marshal plan: %w", err)
	}
	return out, nil
}

func jsonPlanChecks(checks []PlanCheck) []jsonPlanCheck {
	// PlanCheck and jsonPlanCheck are field-identical (same names, types, order);
	// only the json tags differ, so a direct conversion is exact.
	out := make([]jsonPlanCheck, 0, len(checks))
	for _, c := range checks {
		out = append(out, jsonPlanCheck(c))
	}
	return out
}

// Human writes a concise human-readable plan to w. It never claims a verdict:
// every line describes what WOULD happen, and the header states plainly that no
// checks executed.
func (p *Plan) Human(w io.Writer) {
	fmt.Fprintf(w, "%s/%s: would run %d checks, skip %d (dry-run — no checks executed)\n",
		modeString(p.Mode), p.Scope, len(p.Selected), len(p.Skipped))
	if len(p.Selected) > 0 {
		fmt.Fprintln(w, "\nwould run:")
		for _, c := range p.Selected {
			fmt.Fprintf(w, "RUN   %s%s\n", c.Name, humanPlanDetails(c))
		}
	}
	// In a foreign repo the per-check skip detail names agentops-internal backing
	// scripts that do not exist in the user's repository — noise the aggregate
	// line below already covers. JSON keeps every row.
	if len(p.Skipped) > 0 && !p.ForeignRepo {
		fmt.Fprintln(w, "\nwould skip:")
		for _, c := range p.Skipped {
			fmt.Fprintf(w, "SKIP  %s%s\n", c.Name, humanPlanDetails(c))
		}
	}
	if len(p.Skipped) > 0 && p.ForeignRepo {
		fmt.Fprintf(w, "\nwould skip %d checks (routing/tier)\n", len(p.Skipped))
	}
}

func humanPlanDetails(c PlanCheck) string {
	parts := []string{
		"tier: " + c.Tier,
		blockingLabel(c.Blocking),
	}
	if c.Reason != "" {
		parts = append(parts, c.Reason)
	}
	parts = append(parts,
		"backing: "+c.WorkflowBacking,
		"artifact: "+c.ArtifactPath,
		"repair: "+c.RepairHint,
	)
	return " | " + strings.Join(parts, " | ")
}

func blockingLabel(blocking bool) string {
	if blocking {
		return "blocking"
	}
	return "advisory"
}
