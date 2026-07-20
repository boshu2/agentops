package gates

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// RunOptions configures a single orchestrator run.
type RunOptions struct {
	// Mode is Fast (cockpit, changed-file routed) or Full (CI/refinery, all).
	Mode Tier
	// Scope selects the diff used for routing in Fast mode (ignored in Full).
	Scope Scope
	// FailFast stops after the first blocking FAIL (the local cockpit default);
	// CI/refinery want collect-all, so they leave this false.
	FailFast bool
}

// CheckResult is the outcome of running one check.
type CheckResult struct {
	Check          Check
	SelectedReason string
	Verdict        ports.GateVerdict
	Duration       time.Duration
	// Err is set only when the check could not be evaluated at all (distinct
	// from a FAIL verdict).
	Err error
}

// SkippedCheck records a registered check that did not execute in this run.
type SkippedCheck struct {
	Check  Check
	Reason string
}

type checkSelection struct {
	Check  Check
	Reason string
}

// Orchestrator selects and runs checks against the registry. Phase A runs them
// serially (shared-state races between check scripts — e.g. two regenerating a
// generated surface); bounded-pool parallelism is deferred (ag-qidx GA8).
type Orchestrator struct {
	reg      *Registry
	runner   ports.GateRunnerPort
	files    ChangedFilesPort
	repoRoot string
	now      func() time.Time
}

// NewOrchestrator wires the registry, the gate runner (for Backing checks), and
// the changed-files port (for Fast-mode routing).
func NewOrchestrator(reg *Registry, runner ports.GateRunnerPort, files ChangedFilesPort, repoRoot string) *Orchestrator {
	return &Orchestrator{reg: reg, runner: runner, files: files, repoRoot: repoRoot, now: time.Now}
}

// Run selects the checks for opts and runs them serially, returning a Report.
func (o *Orchestrator) Run(ctx context.Context, opts RunOptions) (*Report, error) {
	started := o.now()
	selected, skipped, changed, err := o.selectCheckPlans(ctx, opts)
	if err != nil {
		return nil, err
	}
	rc := RunContext{RepoRoot: o.repoRoot, ChangedFiles: changed, Mode: opts.Mode}

	results := make([]CheckResult, 0, len(selected))
	for i, plan := range selected {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		start := o.now()
		verdict, runErr := o.runOne(ctx, plan.Check, rc)
		results = append(results, CheckResult{
			Check:          plan.Check,
			SelectedReason: plan.Reason,
			Verdict:        verdict,
			Duration:       o.now().Sub(start),
			Err:            runErr,
		})
		if opts.FailFast && (isBlockingFail(plan.Check, verdict) || (plan.Check.Blocking && runErr != nil)) {
			for _, pending := range selected[i+1:] {
				skipped = append(skipped, SkippedCheck{
					Check:  pending.Check,
					Reason: fmt.Sprintf("skipped: fail-fast stopped after blocking failure in %s", plan.Check.ID),
				})
			}
			break
		}
	}

	return &Report{
		Mode:         opts.Mode,
		Scope:        opts.Scope,
		ChangedCount: len(changed),
		StartedAt:    started,
		Elapsed:      o.now().Sub(started),
		Results:      results,
		Skipped:      skipped,
		ForeignRepo:  !IsAgentOpsRepo(o.repoRoot),
	}, nil
}

// Select returns the checks that would run for opts (and the routed change set)
// WITHOUT executing them. It is the dry-run/introspection surface and the basis
// of the predicate-parity guard (ag-qidx GA7): a check must route on the change
// class it guards, never a different one (the #634 silent-drop class).
func (o *Orchestrator) Select(ctx context.Context, opts RunOptions) ([]Check, []string, error) {
	return o.selectChecks(ctx, opts)
}

// selectChecks applies the tier filter, then (Fast mode only) changed-file
// routing with the full-run invalidation escape hatch.
func (o *Orchestrator) selectChecks(ctx context.Context, opts RunOptions) ([]Check, []string, error) {
	selected, _, changed, err := o.selectCheckPlans(ctx, opts)
	if err != nil {
		return nil, nil, err
	}
	checks := make([]Check, 0, len(selected))
	for _, plan := range selected {
		checks = append(checks, plan.Check)
	}
	return checks, changed, nil
}

// selectCheckPlans applies the tier filter, then (Fast mode only) changed-file
// routing with the full-run invalidation escape hatch. It keeps explicit
// reasons for both selected and skipped checks so report consumers can explain
// route decisions without reconstructing the registry.
func (o *Orchestrator) selectCheckPlans(ctx context.Context, opts RunOptions) ([]checkSelection, []SkippedCheck, []string, error) {
	var tierMatched []Check
	var skipped []SkippedCheck
	for _, c := range o.reg.All() {
		if c.Tiers.Has(opts.Mode) {
			tierMatched = append(tierMatched, c)
			continue
		}
		skipped = append(skipped, SkippedCheck{
			Check:  c,
			Reason: fmt.Sprintf("skipped: check tiers %s do not include active mode %s", tierString(c.Tiers), modeString(opts.Mode)),
		})
	}
	if opts.Mode == Full {
		selected := make([]checkSelection, 0, len(tierMatched))
		for _, c := range tierMatched {
			selected = append(selected, checkSelection{
				Check:  c,
				Reason: fmt.Sprintf("selected: full mode runs every check that includes tier %s", modeString(Full)),
			})
		}
		return selected, skipped, nil, nil
	}

	changed, err := o.files.Changed(ctx, opts.Scope)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("gates: detect changed files: %w", err)
	}
	if invalidatingFile := firstInvalidatingFile(changed); invalidatingFile != "" {
		selected := make([]checkSelection, 0, len(tierMatched))
		for _, c := range tierMatched {
			selected = append(selected, checkSelection{
				Check:  c,
				Reason: fmt.Sprintf("selected: changed file %q invalidates fast routing, so all fast checks run", invalidatingFile),
			})
		}
		return selected, skipped, changed, nil
	}
	selected := make([]checkSelection, 0, len(tierMatched))
	for _, c := range tierMatched {
		reason, ok := fastSelectionReason(c, changed)
		if ok {
			selected = append(selected, checkSelection{Check: c, Reason: reason})
			continue
		}
		skipped = append(skipped, SkippedCheck{Check: c, Reason: fastSkipReason(c, changed)})
	}
	return selected, skipped, changed, nil
}

// runOne dispatches to the native Run func or the GateRunnerPort (Backing).
func (o *Orchestrator) runOne(ctx context.Context, c Check, rc RunContext) (ports.GateVerdict, error) {
	if c.Run != nil {
		return c.Run(ctx, rc)
	}
	return o.runner.Run(ctx, ports.GateRunRequest{Name: ports.GateName(c.Backing), Args: c.Args, Env: c.Env})
}

// isBlockingFail reports whether a verdict should fail the run. It is
// fail-closed for blocking checks: a blocking check clears the run ONLY when it
// definitively PASSed or SKIPped (a first-class not-applicable verdict). WARN,
// FAIL, UNKNOWN, and any empty/unrecognized status all fail the run. Advisory
// behavior belongs on the registered check as Blocking=false; accepting WARN
// from a blocking check would let interpreter or usage failures degrade release
// authority. "No verdict = not done" applies to the gate's own output.
// A non-blocking check (of any status) is always advisory.
func isBlockingFail(c Check, v ports.GateVerdict) bool {
	if !c.Blocking {
		return false
	}
	switch v.Status {
	case ports.GateStatusPass, ports.GateStatusSkip:
		return false
	default:
		return true
	}
}

func fastSelectionReason(c Check, changed []string) (string, bool) {
	if c.AlwaysRun() {
		return "selected: check has no match globs and always runs", true
	}
	file, pattern, ok := c.affectedReason(changed)
	if ok {
		return fmt.Sprintf("selected: changed file %q matched %q", file, pattern), true
	}
	return "", false
}

func fastSkipReason(c Check, changed []string) string {
	match := strings.Join(c.Match, ", ")
	if len(changed) == 0 {
		return fmt.Sprintf("skipped: no changed files; match globs require %s", match)
	}
	return fmt.Sprintf("skipped: no changed file matched match globs %s", match)
}
