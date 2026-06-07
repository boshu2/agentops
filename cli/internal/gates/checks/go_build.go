package checks

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// go.build is the native-Go reference check (the Phase B target shape): instead
// of shelling to a script, it runs `go build ./...` directly and maps the result
// to a verdict.
func init() {
	gates.Register(gates.Check{
		ID:       "go.build",
		Tiers:    gates.Fast | gates.Full,
		Match:    []string{"cli/**", "go.mod", "go.sum"},
		Blocking: true,
		Run:      runGoBuild,
	})
}

func runGoBuild(ctx context.Context, rc gates.RunContext) (ports.GateVerdict, error) {
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = filepath.Join(rc.RepoRoot, "cli")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return ports.GateVerdict{
			Status:  ports.GateStatusFail,
			Reason:  "go build ./... failed",
			LogTail: tail(out.String(), 4096),
		}, nil
	}
	return ports.GateVerdict{Status: ports.GateStatusPass, Reason: "go build ./... ok"}, nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
