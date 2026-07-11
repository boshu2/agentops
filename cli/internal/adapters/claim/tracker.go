// Package claim contains driven adapters for the claim command family.
package claim

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	claimapp "github.com/boshu2/agentops/cli/internal/claim"
	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

type ExitErrorFactory func(code int, message string) error

type Tracker struct {
	workingDirectory func() (string, error)
	environment      func() []string
	lookPath         trackerresolve.LookPath
	exitError        ExitErrorFactory
}

func NewTracker(exitError ExitErrorFactory) Tracker {
	return NewTrackerWith(os.Getwd, os.Environ, exec.LookPath, exitError)
}

func NewTrackerWith(
	workingDirectory func() (string, error),
	environment func() []string,
	lookPath trackerresolve.LookPath,
	exitError ExitErrorFactory,
) Tracker {
	return Tracker{
		workingDirectory: workingDirectory, environment: environment,
		lookPath: lookPath, exitError: exitError,
	}
}

func (tracker Tracker) Claim(ctx context.Context, id string, streams claimapp.Streams) error {
	if tracker.workingDirectory == nil || tracker.environment == nil || tracker.lookPath == nil || tracker.exitError == nil {
		return errors.New("claim tracker adapter is not configured")
	}
	cwd, err := tracker.workingDirectory()
	if err != nil {
		return tracker.exitError(127, strings.TrimSpace(err.Error()))
	}
	environment := append([]string(nil), tracker.environment()...)
	resolution, err := trackerresolve.ResolveWithLookPath(cwd, environment, tracker.lookPath)
	if err != nil {
		return tracker.exitError(127, strings.TrimSpace(err.Error()))
	}
	command := exec.CommandContext(ctx, resolution.Binary, "update", id, "--claim") // #nosec G204 -- tracker resolver constrains the binary to br|bd.
	command.Dir = cwd
	if resolution.Tracker == trackerresolve.BR {
		if _, present := trackerresolve.BeadsDirValue(environment); !present {
			environment = append(environment, "BEADS_DIR="+resolution.LedgerDir)
		}
	}
	command.Env = environment
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
