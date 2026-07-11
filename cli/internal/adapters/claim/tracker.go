// Package claim contains driven adapters for the claim command family.
package claim

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
	claimapp "github.com/boshu2/agentops/cli/internal/claim"
)

type ExitErrorFactory func(code int, message string) error

type Resolver interface {
	Resolve() (beadsapp.TrackerResolution, error)
}

type Tracker struct {
	resolver  Resolver
	exitError ExitErrorFactory
}

func NewTracker(resolver Resolver, exitError ExitErrorFactory) Tracker {
	return Tracker{resolver: resolver, exitError: exitError}
}

func (tracker Tracker) Claim(ctx context.Context, id string, streams claimapp.Streams) error {
	if tracker.resolver == nil || tracker.exitError == nil {
		return errors.New("claim tracker adapter is not configured")
	}
	resolution, err := tracker.resolver.Resolve()
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, resolution.Binary, "update", id, "--claim") // #nosec G204 -- tracker resolver constrains the binary to br|bd.
	command.Dir = resolution.WorkDir
	command.Env = append([]string(nil), resolution.ChildEnv...)
	output, runErr := command.CombinedOutput()
	if len(output) > 0 && streams.Stdout != nil {
		_, _ = streams.Stdout.Write(output)
	}
	if runErr == nil {
		return nil
	}
	code := 127
	var processExit *exec.ExitError
	if errors.As(runErr, &processExit) {
		code = processExit.ExitCode()
	}
	return tracker.exitError(code, strings.TrimSpace(runErr.Error()))
}

var _ claimapp.TrackerClaimer = Tracker{}
