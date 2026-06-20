package checks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/boshu2/agentops/cli/internal/aostate"
	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

func init() {
	gates.Register(gates.Check{
		ID:         "state.verify",
		Tiers:      gates.Fast | gates.Full,
		Blocking:   true,
		Run:        runStateVerifyGate,
		RepairHint: "ao state verify",
	})
}

func runStateVerifyGate(ctx context.Context, rc gates.RunContext) (ports.GateVerdict, error) {
	report, err := aostate.VerifyRepo(ctx, rc.RepoRoot)
	if err != nil {
		return ports.GateVerdict{Status: ports.GateStatusUnknown, Reason: fmt.Sprintf("state verify could not run: %v", err)}, err
	}
	if report.Verdict == "PASS" {
		return ports.GateVerdict{
			Status: ports.GateStatusPass,
			Reason: fmt.Sprintf("ao state verify ok (%d schema(s), %d valid fixture(s), %d bad fixture(s) rejected, %d accepted finding(s), %d ledger row(s))",
				report.Schemas, report.GoodFixtures, report.BadFixturesRejected, report.AcceptedFindings, report.LedgerRows),
		}, nil
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		raw = []byte(fmt.Sprintf("marshal state verify report: %v", err))
	}
	return ports.GateVerdict{
		Status:  ports.GateStatusFail,
		Reason:  fmt.Sprintf("ao state verify failed with %d failure(s)", len(report.Failures)),
		LogTail: string(raw),
	}, nil
}
