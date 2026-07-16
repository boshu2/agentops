// Package reviewerhealth contains process adapters for reviewer reachability.
package reviewerhealth

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	reviewerapp "github.com/boshu2/agentops/cli/internal/reviewerhealth"
)

type Probe struct {
	LookPath func(string) (string, error)
	Run      func(context.Context, string, ...string) error
	Now      func() time.Time
}

func SystemProbe() Probe {
	return Probe{
		LookPath: exec.LookPath,
		Run: func(ctx context.Context, name string, args ...string) error {
			return exec.CommandContext(ctx, name, args...).Run()
		},
		Now: time.Now,
	}
}

func (probe Probe) Check(ctx context.Context, reviewer reviewerapp.Reviewer, timeout time.Duration) reviewerapp.ProbeResult {
	if _, err := probe.LookPath(reviewer.Name); err != nil {
		return reviewerapp.ProbeResult{
			Status: "warn", Detail: fmt.Sprintf("not found on PATH — install: %s", reviewer.InstallCommand),
		}
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := probe.Now()
	err := probe.Run(checkCtx, reviewer.Name, "--version")
	elapsed := probe.Now().Sub(start).Round(time.Millisecond)
	switch {
	case errors.Is(checkCtx.Err(), context.DeadlineExceeded):
		return reviewerapp.ProbeResult{
			Status: "fail",
			Detail: fmt.Sprintf("unreachable: '%s --version' timed out after %s — check for a hung or unauthenticated binary: run '%s --version' manually", reviewer.Name, timeout, reviewer.Name),
		}
	case err != nil:
		return reviewerapp.ProbeResult{
			Status: "fail", Detail: fmt.Sprintf("'%s --version' failed (%v) — reinstall: %s", reviewer.Name, err, reviewer.InstallCommand),
		}
	default:
		return reviewerapp.ProbeResult{
			Status: "pass", Detail: fmt.Sprintf("reachable ('--version' answered in %s)", elapsed), Live: true,
		}
	}
}
