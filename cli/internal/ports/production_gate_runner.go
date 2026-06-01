// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/quality"
)

// ProductionGateRunner satisfies GateRunnerPort by invoking
// scripts/check-<name>.sh subprocesses and translating exit code +
// stdout into a GateVerdict.
//
// Mapping rules (documented per port contract):
//   - exit 0 → PASS (Reason: "exit 0")
//   - exit 2 → WARN (some scripts use 2 for advisory mode)
//   - exit 75 → SKIP (sysexits EX_TEMPFAIL — convention used by
//     gates that decide structural-availability skip)
//   - any other non-zero → FAIL
//   - script does not exist → UNKNOWN (the adapter chooses the
//     OPTIMISTIC interpretation: typo OR newly-introduced gate)
//   - empty req.Name → UNKNOWN (port contract requirement)
//
// LogTail is the last 4096 bytes of combined stdout+stderr (port
// contract cap).
type ProductionGateRunner struct {
	repoRoot string
}

func NewProductionGateRunner(repoRoot string) *ProductionGateRunner {
	return &ProductionGateRunner{repoRoot: repoRoot}
}

// Run invokes scripts/check-<name>.sh and returns a GateVerdict.
func (g *ProductionGateRunner) Run(ctx context.Context, req GateRunRequest) (GateVerdict, error) {
	if err := ctx.Err(); err != nil {
		return GateVerdict{}, err
	}
	if req.Name == "" {
		return GateVerdict{
			Status: GateStatusUnknown,
			Reason: "empty GateName",
		}, nil
	}
	if g.repoRoot == "" {
		return GateVerdict{}, fmt.Errorf("ProductionGateRunner: repoRoot required")
	}
	scriptPath := filepath.Join(g.repoRoot, "scripts", "check-"+string(req.Name)+".sh")
	if !quality.FileExists(scriptPath) {
		return GateVerdict{
			Status: GateStatusUnknown,
			Reason: fmt.Sprintf("no script for gate %q at %s", req.Name, scriptPath),
		}, nil
	}

	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	cmd.Dir = g.repoRoot
	if len(req.Env) > 0 {
		envList := make([]string, 0, len(req.Env))
		for k, v := range req.Env {
			envList = append(envList, k+"="+v)
		}
		cmd.Env = append(cmd.Environ(), envList...)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()

	exitCode := 0
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if runErr != nil {
		// Non-exit error (e.g. binary missing, context cancelled mid-run)
		return GateVerdict{
			Status:  GateStatusUnknown,
			Reason:  fmt.Sprintf("subprocess error: %v", runErr),
			LogTail: tailBytes(out.Bytes(), 4096),
		}, nil
	}

	verdict := exitCodeToVerdict(exitCode)
	verdict.LogTail = tailBytes(out.Bytes(), 4096)
	return verdict, nil
}

// exitCodeToVerdict maps subprocess exit codes to GateVerdict per the
// adapter's documented rules.
func exitCodeToVerdict(code int) GateVerdict {
	switch code {
	case 0:
		return GateVerdict{Status: GateStatusPass, Reason: "exit 0"}
	case 2:
		return GateVerdict{Status: GateStatusWarn, Reason: "exit 2 (advisory)"}
	case 75:
		return GateVerdict{Status: GateStatusSkip, Reason: "exit 75 (structural skip)"}
	}
	return GateVerdict{Status: GateStatusFail, Reason: fmt.Sprintf("exit %d", code)}
}

// tailBytes returns at most n trailing bytes of b.
func tailBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return strings.TrimSpace(string(b[len(b)-n:]))
}

// Compile-time assertion: ProductionGateRunner satisfies the port.
var _ GateRunnerPort = (*ProductionGateRunner)(nil)
