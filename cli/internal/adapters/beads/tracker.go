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
	"github.com/boshu2/agentops/cli/internal/trackerexec"
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
	resolved, err := tracker.resolve()
	if err != nil {
		return beadsapp.TrackerResolution{}, err
	}
	return appResolution(resolved), nil
}

func (tracker *Tracker) resolve() (trackerresolve.Resolution, error) {
	cwd, env, err := tracker.context()
	if err != nil {
		return trackerresolve.Resolution{}, err
	}
	return trackerresolve.ResolveWithLookPath(cwd, env, tracker.lookPath)
}

func appResolution(resolved trackerresolve.Resolution) beadsapp.TrackerResolution {
	return beadsapp.TrackerResolution{
		Tracker:   resolved.Tracker,
		Binary:    resolved.Binary,
		LedgerDir: resolved.LedgerDir,
		Source:    resolved.Source,
		WorkDir:   resolved.WorkDir,
		ChildEnv:  append([]string(nil), resolved.ChildEnv...),
	}
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
	resolved, err := tracker.resolve()
	if err != nil {
		return nil, err
	}
	return (trackerexec.Factory{}).Command(ctx, resolved, args, trackerexec.Streams{}).Output()
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

func (tracker *Tracker) Show(ctx context.Context, beadID string) (beadsapp.StaleBeadRecord, error) {
	output, err := tracker.Output(ctx, "show", beadID, "--json")
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return beadsapp.StaleBeadRecord{}, fmt.Errorf("br show %s exited %d: %s", beadID, exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return beadsapp.StaleBeadRecord{}, err
	}
	return beadsapp.ParseShownBead(output, beadID)
}

func (tracker *Tracker) Claim(ctx context.Context, beadID, agent string) error {
	resolved, err := tracker.resolve()
	if err != nil {
		return err
	}
	args := []string{"update", beadID, "--claim"}
	if agent != "" {
		args = append(args, "--actor", agent)
	}
	output, err := (trackerexec.Factory{}).Command(ctx, resolved, args, trackerexec.Streams{}).CombinedOutput()
	if err != nil {
		return fmt.Errorf("br update --claim failed: %w: %s", err, string(output))
	}
	return nil
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
