package beads

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
	"github.com/boshu2/agentops/cli/internal/trackerexec"
	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

type Executor struct {
	tracker *Tracker
}

func NewExecutor(tracker *Tracker) *Executor {
	return &Executor{tracker: tracker}
}

func (executor *Executor) Execute(ctx context.Context, args []string, streams beadsapp.ExecStreams) error {
	if executor == nil || executor.tracker == nil {
		return fmt.Errorf("beads tracker executor is not configured")
	}
	resolution, err := executor.tracker.resolve()
	if err != nil {
		return err
	}
	if resolution.Tracker == trackerresolve.BR && len(args) > 0 && args[0] == "children" {
		return executor.childrenBR(ctx, resolution, args[1:], streams)
	}
	if resolution.Tracker == trackerresolve.BD && len(args) > 0 && beadsapp.IsReadVerb(args[0]) && beadsapp.ArgsHaveJSONFlag(args) {
		return executor.bdReadJSON(ctx, resolution, args, streams)
	}
	return executor.stream(ctx, resolution, args, streams)
}

func (executor *Executor) stream(ctx context.Context, resolution trackerresolve.Resolution, args []string, streams beadsapp.ExecStreams) error {
	command := resolvedCommand(ctx, resolution, args, streams)
	return exitError(command.Run())
}

func (executor *Executor) childrenBR(ctx context.Context, resolution trackerresolve.Resolution, args []string, streams beadsapp.ExecStreams) error {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		return fmt.Errorf("ao beads exec children: an epic id is required")
	}
	epic := args[0]
	var stdout, stderr bytes.Buffer
	command := resolvedCommand(ctx, resolution, []string{"show", epic, "--json"}, beadsapp.ExecStreams{Stdout: &stdout, Stderr: &stderr})
	if err := command.Run(); err != nil {
		writeTrimmedLine(streams.Stderr, stderr.String())
		if converted := exitError(err); converted != err {
			return converted
		}
		return fmt.Errorf("br show %s --json: %w", epic, err)
	}
	children, err := beadsapp.BRChildren(stdout.Bytes())
	if err != nil {
		return fmt.Errorf("parse br show %s --json: %w", epic, err)
	}
	for _, child := range children {
		fmt.Fprintln(streams.Stdout, child)
	}
	return nil
}

func (executor *Executor) bdReadJSON(ctx context.Context, resolution trackerresolve.Resolution, args []string, streams beadsapp.ExecStreams) error {
	var stdout, stderr bytes.Buffer
	command := resolvedCommand(ctx, resolution, args, beadsapp.ExecStreams{Stdin: streams.Stdin, Stdout: &stdout, Stderr: &stderr})
	runErr := command.Run()
	writeTrimmedLine(streams.Stderr, stderr.String())
	if runErr != nil {
		return exitError(runErr)
	}
	canonical, err := beadsapp.CanonicalizeBDReadJSON(args[0], stdout.Bytes())
	if err != nil {
		_, _ = streams.Stdout.Write(stdout.Bytes())
		return nil
	}
	fmt.Fprintln(streams.Stdout, string(canonical))
	return nil
}

func resolvedCommand(ctx context.Context, resolution trackerresolve.Resolution, args []string, streams beadsapp.ExecStreams) *trackerexec.ResolvedCommand {
	return (trackerexec.Factory{}).Command(ctx, resolution, args, trackerexec.Streams{
		Stdin: streams.Stdin, Stdout: streams.Stdout, Stderr: streams.Stderr,
	})
}

func exitError(err error) error {
	if err == nil {
		return nil
	}
	var processExit *exec.ExitError
	if errors.As(err, &processExit) {
		return &beadsapp.ExitError{Code: processExit.ExitCode()}
	}
	return err
}

func writeTrimmedLine(writer interface{ Write([]byte) (int, error) }, value string) {
	if writer == nil {
		return
	}
	if message := strings.TrimSpace(value); message != "" {
		_, _ = fmt.Fprintln(writer, message)
	}
}

var _ beadsapp.TrackerExecutor = (*Executor)(nil)
