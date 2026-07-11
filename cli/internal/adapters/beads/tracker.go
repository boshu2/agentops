// Package beads implements the beads command family's driven effects.
package beads

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

type Tracker struct {
	workingDirectory func() (string, error)
	environment      func() []string
	lookPath         trackerresolve.LookPath
}

func NewTracker() *Tracker {
	return NewTrackerWith(os.Getwd, os.Environ, exec.LookPath)
}

func NewTrackerWith(workingDirectory func() (string, error), environment func() []string, lookPath trackerresolve.LookPath) *Tracker {
	return &Tracker{workingDirectory: workingDirectory, environment: environment, lookPath: lookPath}
}

func (tracker *Tracker) Resolve() (beadsapp.TrackerResolution, error) {
	cwd, env, err := tracker.context()
	if err != nil {
		return beadsapp.TrackerResolution{}, err
	}
	resolved, err := trackerresolve.ResolveWithLookPath(cwd, env, tracker.lookPath)
	if err != nil {
		return beadsapp.TrackerResolution{}, err
	}
	return beadsapp.TrackerResolution{
		Tracker:   resolved.Tracker,
		Binary:    resolved.Binary,
		LedgerDir: resolved.LedgerDir,
		Source:    resolved.Source,
		WorkDir:   resolved.WorkDir,
		ChildEnv:  append([]string(nil), resolved.ChildEnv...),
	}, nil
}

func (tracker *Tracker) BRLedger() (beadsapp.LedgerResolution, error) {
	cwd, env, err := tracker.context()
	if err != nil {
		return beadsapp.LedgerResolution{}, err
	}
	resolved := trackerresolve.ResolveLedger(cwd, env, trackerresolve.BR)
	return beadsapp.LedgerResolution{Path: resolved.Path, Source: resolved.Source}, nil
}

func (tracker *Tracker) BeadsDirOverride() bool {
	_, ok := trackerresolve.BeadsDirValue(tracker.environment())
	return ok
}

func (tracker *Tracker) InspectLedger(path string) beadsapp.LedgerSnapshot {
	info, err := os.Stat(path)
	if err != nil {
		return beadsapp.LedgerSnapshot{}
	}
	snapshot := beadsapp.LedgerSnapshot{Exists: true, Directory: info.IsDir()}
	if !info.IsDir() {
		return snapshot
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return snapshot
	}
	snapshot.Readable = true
	for _, entry := range entries {
		snapshot.Entries = append(snapshot.Entries, entry.Name())
	}
	sort.Strings(snapshot.Entries)
	return snapshot
}

func (tracker *Tracker) Available() bool {
	_, err := tracker.Resolve()
	return err == nil
}

func (tracker *Tracker) Output(ctx context.Context, args ...string) ([]byte, error) {
	resolved, err := tracker.Resolve()
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, resolved.Binary, args...) // #nosec G204 -- trackerresolve constrains the binary to br|bd.
	command.Dir = resolved.WorkDir
	command.Env = append([]string(nil), resolved.ChildEnv...)
	return command.Output()
}

func (tracker *Tracker) ListInProgress(ctx context.Context) ([]byte, error) {
	output, err := tracker.Output(ctx, "list", "--status", "in_progress", "--json", "--limit", "500")
	if err == nil {
		return output, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil, fmt.Errorf("br list exited %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
	}
	return nil, err
}

func (tracker *Tracker) context() (string, []string, error) {
	if tracker == nil || tracker.workingDirectory == nil || tracker.environment == nil || tracker.lookPath == nil {
		return "", nil, fmt.Errorf("beads tracker adapter is not configured")
	}
	cwd, err := tracker.workingDirectory()
	if err != nil {
		return "", nil, err
	}
	return cwd, tracker.environment(), nil
}

var _ beadsapp.TrackerResolver = (*Tracker)(nil)
var _ beadsapp.TrackerClient = (*Tracker)(nil)
var _ beadsapp.LedgerInspector = (*Tracker)(nil)
var _ beadsapp.StaleSource = (*Tracker)(nil)
