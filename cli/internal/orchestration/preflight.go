// practices: [design-by-contract, safe-degradation]
package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/worktree"
)

// PreflightOptions configures a preflight run.
type PreflightOptions struct {
	RepoRoot string
	Profile  string
	RunID    string
	Runner   CommandRunner
}

// RunPreflight executes admission checks for an orchestration profile.
func RunPreflight(ctx context.Context, opts PreflightOptions) (InstrumentResult, error) {
	if opts.Runner == nil {
		return InstrumentResult{}, fmt.Errorf("preflight: runner is nil")
	}
	if opts.RunID == "" {
		opts.RunID = NewRunID()
	}
	profiles, err := LoadProfilesContract(opts.RepoRoot)
	if err != nil {
		return InstrumentResult{}, err
	}
	profile, err := profiles.ProfileByID(opts.Profile)
	if err != nil {
		return InstrumentResult{}, err
	}
	toolsContract, err := LoadToolsContract(opts.RepoRoot)
	if err != nil {
		return InstrumentResult{}, err
	}

	var checks []CheckStatus
	coordDegraded := false

	toolReports, err := ProbeTools(ctx, toolsContract, opts.Runner)
	if err != nil {
		return InstrumentResult{}, err
	}
	required := ToolsForProfile(toolsContract, opts.Profile, toolReports)
	for _, r := range required {
		spec := findToolSpec(toolsContract, r.ID)
		cs := CheckStatus{ID: "tool:" + r.ID, Status: VerdictStatusPass}
		if !r.Available {
			if spec != nil && spec.Optional {
				cs.Status = VerdictStatusWarn
				cs.Detail = "optional tool unavailable"
			} else {
				cs.Status = VerdictStatusFail
				cs.Detail = "required tool unavailable"
			}
		}
		checks = append(checks, cs)
	}

	if floor, ok := toolsContract.VersionFloors["atm"]; ok && floor.MinVersion != "" {
		atmReport := findToolReport(required, "atm")
		cs := CheckStatus{ID: "atm_version_floor", Status: VerdictStatusPass}
		if atmReport == nil || !atmReport.Available {
			cs.Status = VerdictStatusFail
			cs.Detail = "atm not available for version check"
		} else if !VersionMeetsFloor(atmReport.Version, floor.MinVersion) {
			cs.Status = VerdictStatusFail
			cs.Detail = fmt.Sprintf("atm version %q below floor %q", atmReport.Version, floor.MinVersion)
		}
		checks = append(checks, cs)
	}

	amCS := CheckStatus{ID: "am_health", Status: VerdictStatusPass}
	amCtx, amCancel := context.WithTimeout(ctx, 4*time.Second)
	defer amCancel()
	if out, err := opts.Runner.Run(amCtx, "am", "robot", "health"); err != nil {
		amCS.Status = VerdictStatusWarn
		amCS.Detail = "agent mail unavailable"
		coordDegraded = true
	} else if !amHealthOK(out) {
		amCS.Status = VerdictStatusWarn
		amCS.Detail = "agent mail health not ok"
		coordDegraded = true
	}
	checks = append(checks, amCS)

	sessCS := CheckStatus{ID: "session_collision", Status: VerdictStatusPass}
	listCtx, listCancel := context.WithTimeout(ctx, 4*time.Second)
	defer listCancel()
	if out, err := opts.Runner.Run(listCtx, "atm", "list"); err != nil {
		sessCS.Status = VerdictStatusWarn
		sessCS.Detail = "atm list unavailable"
	} else if sessionListed(out, opts.Profile) {
		sessCS.Status = VerdictStatusWarn
		sessCS.Detail = "existing agentops session may collide"
	}
	checks = append(checks, sessCS)

	// Concurrency safety: the two shared-state hazards that bit fan-out runs.
	// NTM has no notion of either — this is the AgentOps admission gate's job.
	// WARN-tier: preflight READS the hazard; the pre-push hook + worktree guard
	// ENFORCE it (fail-closed) in their own lanes.
	checks = append(checks, concurrencyChecks(opts.RepoRoot)...)

	verdict := AggregateVerdictFromChecks(checks)
	return InstrumentResult{
		SchemaVersion:        InstrumentSchemaVersionV1,
		Command:              InstrumentCommandPreflight,
		Profile:              opts.Profile,
		RunID:                opts.RunID,
		Verdict:              verdict,
		CoordinationDegraded: coordDegraded,
		Checks:               checks,
		Panes:                profile.Panes,
	}, nil
}

// concurrencyChecks surfaces the two shared-state hazards that bite fan-out runs
// (proven live 2026-06-18): mutating work in the bare canonical checkout instead
// of an isolated worktree (a concurrent session's `git reset` wipes uncommitted
// work), and a concurrent serialized push already holding the lock on main. Both
// are deterministic local checks; the substrate (NTM) has no notion of either.
func concurrencyChecks(repoRoot string) []CheckStatus {
	var checks []CheckStatus

	// Worktree isolation: a linked git worktree carries a .git FILE (a gitdir
	// pointer); the shared canonical checkout carries a .git DIRECTORY. Spawning
	// from the canonical checkout means N sessions share one mutable working tree.
	wt := CheckStatus{ID: "worktree_isolation", Status: VerdictStatusPass}
	if isolated, err := worktree.IsIsolated(repoRoot); err != nil {
		wt.Status = VerdictStatusWarn
		wt.Detail = "cannot determine worktree isolation"
	} else if !isolated {
		wt.Status = VerdictStatusWarn
		wt.Detail = "spawning from the shared canonical checkout — use an isolated worktree so a concurrent reset cannot wipe uncommitted work"
	}
	checks = append(checks, wt)

	// Push lock: push-serial.sh holds ${TMPDIR:-/tmp}/agentops-push.lock for the
	// duration of a serialized push. Its presence means main is being written now.
	pl := CheckStatus{ID: "push_lock", Status: VerdictStatusPass}
	tmp := os.Getenv("TMPDIR")
	if tmp == "" {
		tmp = "/tmp"
	}
	if _, err := os.Stat(filepath.Join(tmp, "agentops-push.lock")); err == nil {
		pl.Status = VerdictStatusWarn
		pl.Detail = "a serialized push to main is in progress (push-serial lock held) — main is being written"
	}
	checks = append(checks, pl)

	return checks
}

func findToolSpec(c ToolsContract, id string) *ToolSpec {
	for i := range c.Tools {
		if c.Tools[i].ID == id {
			return &c.Tools[i]
		}
	}
	return nil
}

func findToolReport(reports []ToolReport, id string) *ToolReport {
	for i := range reports {
		if reports[i].ID == id {
			return &reports[i]
		}
	}
	return nil
}

func amHealthOK(out []byte) bool {
	var env struct {
		OK     bool   `json:"ok"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		return len(strings.TrimSpace(string(out))) > 0
	}
	if env.OK {
		return true
	}
	return strings.EqualFold(env.Status, "ok") || strings.EqualFold(env.Status, "healthy")
}

func sessionListed(listOut []byte, profile string) bool {
	// Conservative: any agentops-- line is a potential collision hint.
	return strings.Contains(string(listOut), "agentops--")
}
