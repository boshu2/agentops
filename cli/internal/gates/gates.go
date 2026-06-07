// Package gates is the BC2 Validation gate orchestrator: a declarative,
// decentralized registry of checks that the `ao gate check` command runs in
// fast (cockpit/pre-push) or full (CI/refinery) mode.
//
// Each check registers itself via init() from its own file (see checks/), so
// the registry has no central monolith file — adding a check is one new file
// plus one barrel import. The orchestrator filters checks by tier and
// changed-file routing, runs them via the GateRunnerPort (or a native Run
// func), and composes a report.
//
// Design: ag-qidx GA1. See .agents/plans/2026-06-07-ao-gate-architecture.md.
package gates

import (
	"context"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// Tier is a bitmask of the run modes a check participates in. A check declares
// its own tiers; the run modes (--fast, --full) are filters over the single
// registry, so a "fast list" and "full list" can never drift apart.
type Tier uint8

const (
	// Fast is the cockpit/pre-push subset — it must stay seconds, or
	// developers route around the gate with --no-verify.
	Fast Tier = 1 << iota
	// Full is the exhaustive CI/refinery set.
	Full
)

// Has reports whether t includes mode.
func (t Tier) Has(mode Tier) bool { return t&mode != 0 }

// RunContext carries the read-only inputs a check needs to run. It is computed
// once per run and shared (read-only) across all selected checks.
type RunContext struct {
	// RepoRoot is the absolute path to the repository root.
	RepoRoot string
	// ChangedFiles is the routed change set (repo-relative paths).
	ChangedFiles []string
	// Mode is the active run mode (Fast or Full).
	Mode Tier
}

// CheckFunc is a native-Go check implementation. It returns a GateVerdict; a
// non-nil error means the check could not be evaluated at all (distinct from a
// FAIL verdict, which is a normal negative result).
type CheckFunc func(ctx context.Context, rc RunContext) (ports.GateVerdict, error)

// Check is one registered gate. It is flat data plus an optional Run func.
// Special behaviour (environment, wrappers) lives in fields, never as bespoke
// orchestrator branches — that keeps the registry declarative (ag-qidx G2: the
// win of the Go rewrite is structure, not fewer lines).
type Check struct {
	// ID is the unique check name (e.g. "go.vet").
	ID string
	// Tiers is the set of modes this check runs in.
	Tiers Tier
	// Match holds path globs that make this check "affected" in fast mode.
	// Empty means always-run: the check is selected regardless of which files
	// changed (full mode ignores Match entirely and runs everything).
	Match []string
	// Blocking determines failure semantics: a FAIL on a blocking check fails
	// the run (exit 1); a FAIL on a non-blocking check is advisory.
	Blocking bool
	// Backing is the scripts/check-<Backing>.sh script run via the
	// GateRunnerPort. Exactly one of Backing or Run must be set.
	Backing string
	// Run is a native-Go implementation. Exactly one of Backing or Run must be
	// set.
	Run CheckFunc
}

// AlwaysRun reports whether the check is selected regardless of changed files
// (i.e. it has no path globs to route on).
func (c Check) AlwaysRun() bool { return len(c.Match) == 0 }
