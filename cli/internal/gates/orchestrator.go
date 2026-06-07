package gates

import (
	"context"
	"fmt"
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
	Check    Check
	Verdict  ports.GateVerdict
	Duration time.Duration
	// Err is set only when the check could not be evaluated at all (distinct
	// from a FAIL verdict).
	Err error
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
	selected, changed, err := o.selectChecks(ctx, opts)
	if err != nil {
		return nil, err
	}
	rc := RunContext{RepoRoot: o.repoRoot, ChangedFiles: changed, Mode: opts.Mode}

	results := make([]CheckResult, 0, len(selected))
	for _, c := range selected {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		start := o.now()
		verdict, runErr := o.runOne(ctx, c, rc)
		results = append(results, CheckResult{
			Check:    c,
			Verdict:  verdict,
			Duration: o.now().Sub(start),
			Err:      runErr,
		})
		if opts.FailFast && isBlockingFail(c, verdict) {
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
	}, nil
}

// selectChecks applies the tier filter, then (Fast mode only) changed-file
// routing with the full-run invalidation escape hatch.
func (o *Orchestrator) selectChecks(ctx context.Context, opts RunOptions) ([]Check, []string, error) {
	var tierMatched []Check
	for _, c := range o.reg.All() {
		if c.Tiers.Has(opts.Mode) {
			tierMatched = append(tierMatched, c)
		}
	}
	if opts.Mode == Full {
		return tierMatched, nil, nil
	}

	changed, err := o.files.Changed(ctx, opts.Scope)
	if err != nil {
		return nil, nil, fmt.Errorf("gates: detect changed files: %w", err)
	}
	if invalidatesAll(changed) {
		return tierMatched, changed, nil
	}
	selected := make([]Check, 0, len(tierMatched))
	for _, c := range tierMatched {
		if c.affected(changed) {
			selected = append(selected, c)
		}
	}
	return selected, changed, nil
}

// runOne dispatches to the native Run func or the GateRunnerPort (Backing).
func (o *Orchestrator) runOne(ctx context.Context, c Check, rc RunContext) (ports.GateVerdict, error) {
	if c.Run != nil {
		return c.Run(ctx, rc)
	}
	return o.runner.Run(ctx, ports.GateRunRequest{Name: ports.GateName(c.Backing)})
}

// isBlockingFail reports whether a verdict should fail the run: only a FAIL on a
// blocking check. A non-blocking FAIL (and any WARN/SKIP) is advisory.
func isBlockingFail(c Check, v ports.GateVerdict) bool {
	return c.Blocking && v.Status == ports.GateStatusFail
}
