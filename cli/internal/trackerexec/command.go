// Package trackerexec owns construction and exit mapping for subprocesses
// launched from a canonical trackerresolve.Resolution.
package trackerexec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

// Streams are the caller-owned process streams applied to a tracker command.
type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Factory constructs tracker commands from an already canonical resolution.
// It deliberately performs no tracker selection or ledger discovery.
type Factory struct{}

// Command applies the caller context, canonical process context, arguments,
// and streams to one tracker subprocess.
func (Factory) Command(
	ctx context.Context,
	resolution trackerresolve.Resolution,
	args []string,
	streams Streams,
) *ResolvedCommand {
	// A cobra command invoked outside Execute (direct RunE calls in tests or
	// embedding) carries a nil context; exec.CommandContext panics on nil.
	// Background preserves the no-cancellation semantics such callers had.
	if ctx == nil {
		ctx = context.Background()
	}
	command := exec.CommandContext(ctx, resolution.Binary, args...) // #nosec G204 -- trackerresolve constrains the binary to br|bd.
	command.Dir = resolution.WorkDir
	if resolution.ChildEnv != nil {
		childEnv := make([]string, len(resolution.ChildEnv))
		copy(childEnv, resolution.ChildEnv)
		resolution.ChildEnv = childEnv
	}
	command.Env = resolution.ChildEnv
	command.Stdin = streams.Stdin
	command.Stdout = streams.Stdout
	command.Stderr = streams.Stderr
	return &ResolvedCommand{command: command}
}

// ResolvedCommand executes a command produced by Factory while preserving a
// typed child exit for callers that map process status to CLI status.
type ResolvedCommand struct {
	command *exec.Cmd
}

func (command *ResolvedCommand) Run() error {
	return mapExit(command.command.Run())
}

func (command *ResolvedCommand) Output() ([]byte, error) {
	output, err := command.command.Output()
	return output, mapExit(err)
}

func (command *ResolvedCommand) CombinedOutput() ([]byte, error) {
	output, err := command.command.CombinedOutput()
	return output, mapExit(err)
}

func (command *ResolvedCommand) Start() error {
	return command.command.Start()
}

func (command *ResolvedCommand) Wait() error {
	return mapExit(command.command.Wait())
}

// ExitError is the stable tracker-process exit contract. Unwrap retains the
// original exec.ExitError for callers that need process-state detail.
type ExitError struct {
	Code  int
	Cause error
}

func (err *ExitError) Error() string {
	if err == nil {
		return ""
	}
	if err.Cause != nil {
		return err.Cause.Error()
	}
	return fmt.Sprintf("tracker process exited with code %d", err.Code)
}

func (err *ExitError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

func (err *ExitError) ExitCode() int {
	if err == nil {
		return 0
	}
	return err.Code
}

func mapExit(err error) error {
	if err == nil {
		return nil
	}
	var processExit *exec.ExitError
	if !errors.As(err, &processExit) {
		return err
	}
	return &ExitError{Code: processExit.ExitCode(), Cause: err}
}
