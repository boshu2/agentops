// practices: [design-by-contract, safe-degradation]
package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
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
		OK     bool `json:"ok"`
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
