// Package claim contains driven adapters for the claim command family.
package claim

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	claimapp "github.com/boshu2/agentops/cli/internal/claim"
	"github.com/boshu2/agentops/cli/internal/trackerexec"
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
	command := (trackerexec.Factory{}).Command(
		ctx,
		resolution,
		[]string{"update", id, "--claim"},
		trackerexec.Streams{Stdin: streams.Stdin, Stdout: streams.Stdout, Stderr: streams.Stderr},
	)
	runErr := command.Run()
	if runErr == nil {
		return nil
	}
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		return runErr
	}
	var processExit *trackerexec.ExitError
	if errors.As(runErr, &processExit) {
		return tracker.exitError(processExit.ExitCode(), strings.TrimSpace(runErr.Error()))
	}
	return tracker.exitError(127, strings.TrimSpace(runErr.Error()))
}

var _ claimapp.TrackerClaimer = Tracker{}
