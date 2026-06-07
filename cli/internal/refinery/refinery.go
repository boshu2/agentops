// Package refinery is the bushido continuous-validation backstop (ag-qidx P2).
// It watches main, runs the full gate on each new commit, and on a DETERMINISTIC
// blocking failure raises a poison-main beacon + files a fix-bead + alerts —
// it NEVER blind-reverts (the repo's 18-30% flake rate would make auto-revert
// fight developers). It is a backstop, not a gatekeeper: if it is down, merges
// still succeed; it resumes from its state file on restart.
package refinery

import (
	"context"
	"fmt"
	"time"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// State is the durable refinery state (persisted as .refinery-state JSON).
type State struct {
	// LastCheckedSHA is the most recent main HEAD the refinery has evaluated.
	LastCheckedSHA string `json:"last_checked_sha"`
	// Poison lists the currently-poisoned commits (deterministic failures not
	// yet fixed forward).
	Poison []PoisonEntry `json:"poison"`
}

// PoisonEntry records a deterministic failure on main.
type PoisonEntry struct {
	SHA     string   `json:"sha"`
	Checks  []string `json:"checks"`
	FixBead string   `json:"fix_bead,omitempty"`
}

// ---- ports (injected; production adapters in adapters.go, fakes in tests) ----

// CommitSource reports the current main HEAD.
type CommitSource interface {
	MainHead(ctx context.Context) (string, error)
}

// GateChecker runs the full gate and returns the report.
type GateChecker interface {
	CheckFull(ctx context.Context) (*gates.Report, error)
}

// Rerunner re-runs a single check (for flaky-vs-deterministic classification).
type Rerunner interface {
	Rerun(ctx context.Context, checkID string) (ports.GateVerdict, error)
}

// BeadFiler files a blocking fix-bead and returns its ID.
type BeadFiler interface {
	FileFixBead(ctx context.Context, sha string, checks []string) (string, error)
}

// Beacon marks/clears a poisoned main commit so pushers can see it.
type Beacon interface {
	Set(ctx context.Context, sha string, checks []string) error
	Clear(ctx context.Context, sha string) error
}

// StateStore loads and persists refinery State.
type StateStore interface {
	Load() (State, error)
	Save(State) error
}

// Refinery is the backstop engine.
type Refinery struct {
	Commits CommitSource
	Gate    GateChecker
	Rerun   Rerunner
	Beads   BeadFiler
	Beacon  Beacon
	Store   StateStore
	// RerunN is how many times a failing check is re-run to classify it as
	// deterministic (fails every time) vs flaky. Default 3 if zero.
	RerunN int
	// Log receives human-readable progress (optional).
	Log func(string)
}

// Result summarizes one RunOnce tick.
type Result struct {
	SHA           string
	Skipped       bool     // HEAD unchanged since last check
	Green         bool     // no blocking failures
	Failing       []string // all blocking-failed check IDs
	Deterministic []string // the subset that reproduced (escalated)
	FixBead       string
}

func (r *Refinery) logf(format string, a ...any) {
	if r.Log != nil {
		r.Log(fmt.Sprintf(format, a...))
	}
}

func (r *Refinery) rerunCount() int {
	if r.RerunN > 0 {
		return r.RerunN
	}
	return 3
}

// RunOnce evaluates the current main HEAD if it has advanced. On a deterministic
// blocking failure it sets a beacon and files a fix-bead; on green it clears any
// beacon. It NEVER reverts.
func (r *Refinery) RunOnce(ctx context.Context) (Result, error) {
	st, err := r.Store.Load()
	if err != nil {
		return Result{}, fmt.Errorf("refinery: load state: %w", err)
	}
	head, err := r.Commits.MainHead(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("refinery: main head: %w", err)
	}
	if head == "" {
		return Result{}, fmt.Errorf("refinery: empty main HEAD")
	}
	if head == st.LastCheckedSHA {
		return Result{SHA: head, Skipped: true}, nil
	}

	r.logf("refinery: evaluating %s", head)
	report, err := r.Gate.CheckFull(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("refinery: gate check: %w", err)
	}

	failing := blockingFailures(report)
	if len(failing) == 0 {
		if err := r.Beacon.Clear(ctx, head); err != nil {
			r.logf("refinery: beacon clear failed: %v", err)
		}
		st.LastCheckedSHA = head
		st.Poison = nil
		if err := r.Store.Save(st); err != nil {
			return Result{}, fmt.Errorf("refinery: save state: %w", err)
		}
		r.logf("refinery: %s GREEN", head)
		return Result{SHA: head, Green: true}, nil
	}

	// Classify: only escalate failures that reproduce deterministically.
	var deterministic []string
	for _, id := range failing {
		if r.isDeterministic(ctx, id) {
			deterministic = append(deterministic, id)
		} else {
			r.logf("refinery: %s failed on %s but did not reproduce — treating as flaky, NOT escalating", head, id)
		}
	}

	res := Result{SHA: head, Failing: failing, Deterministic: deterministic}
	if len(deterministic) > 0 {
		bead, ferr := r.Beads.FileFixBead(ctx, head, deterministic)
		if ferr != nil {
			r.logf("refinery: file fix-bead failed: %v", ferr)
		}
		res.FixBead = bead
		if err := r.Beacon.Set(ctx, head, deterministic); err != nil {
			r.logf("refinery: beacon set failed: %v", err)
		}
		st.Poison = append(st.Poison, PoisonEntry{SHA: head, Checks: deterministic, FixBead: bead})
		r.logf("refinery: %s POISONED by %v — fix-bead %s filed (no revert)", head, deterministic, bead)
	}

	st.LastCheckedSHA = head
	if err := r.Store.Save(st); err != nil {
		return Result{}, fmt.Errorf("refinery: save state: %w", err)
	}
	return res, nil
}

// isDeterministic re-runs a failing check RerunN times; it is deterministic only
// if it FAILS every time (any pass => flaky, do not escalate).
func (r *Refinery) isDeterministic(ctx context.Context, checkID string) bool {
	for i := 0; i < r.rerunCount(); i++ {
		v, err := r.Rerun.Rerun(ctx, checkID)
		if err != nil {
			// Could not re-run -> conservatively treat as NOT deterministic
			// (don't escalate on inability to reproduce).
			return false
		}
		if v.Status != ports.GateStatusFail {
			return false
		}
	}
	return true
}

// Loop runs RunOnce every interval until ctx is cancelled. It is a BACKSTOP: a
// RunOnce error (transient git/gate/network failure, bushido hiccup) is logged
// and the loop continues — the daemon never dies on one bad tick, and resumes
// from its state file across restarts.
func (r *Refinery) Loop(ctx context.Context, interval time.Duration) error {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if _, err := r.RunOnce(ctx); err != nil {
			r.logf("refinery: tick error (continuing): %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

// blockingFailures returns the IDs of blocking checks that FAILed.
func blockingFailures(report *gates.Report) []string {
	var out []string
	for _, res := range report.Results {
		if res.Check.Blocking && res.Verdict.Status == ports.GateStatusFail {
			out = append(out, res.Check.ID)
		}
	}
	return out
}
