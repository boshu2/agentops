package main

import (
	"context"
	"os"
	"os/exec"

	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

const (
	trackerBR = trackerresolve.BR
	trackerBD = trackerresolve.BD

	trackerSourceEnv    = trackerresolve.SourceEnv
	trackerSourceConfig = trackerresolve.SourceConfig
	trackerSourceLedger = trackerresolve.SourceLedger
	trackerSourceBinary = trackerresolve.SourceBinary

	beadsDirSourceEnv       = trackerresolve.LedgerSourceEnv
	beadsDirSourceGitCommon = trackerresolve.LedgerSourceGitCommon
	beadsDirSourceRepoRoot  = trackerresolve.LedgerSourceRepoRoot
	beadsDirSourceCWD       = trackerresolve.LedgerSourceCWD
)

// These aliases keep the package-main migration mechanical without retaining
// type ownership here. The canonical types live in trackerresolve.
type (
	trackerResolution  = trackerresolve.Resolution
	beadsDirResolution = trackerresolve.LedgerResolution
)

// Tests replace trackerLookPath; every resolution still executes in the one
// internal owner with the injected binary lookup.
var trackerLookPath = exec.LookPath

var resolveTracker = func(cwd string, env []string) (trackerResolution, error) {
	return trackerresolve.ResolveWithLookPath(cwd, env, trackerLookPath)
}

var resolveBeadsDir = func(cwd string, env []string) beadsDirResolution {
	return trackerresolve.ResolveLedger(cwd, env, trackerresolve.BR)
}

func beadsTrackerCommandContext(ctx context.Context, args ...string) *exec.Cmd {
	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		cwd = "."
	}
	return beadsTrackerCommandContextInDir(ctx, cwd, args...)
}

func beadsTrackerCommandContextInDir(ctx context.Context, cwd string, args ...string) *exec.Cmd {
	resolution, err := resolveTracker(cwd, os.Environ())
	if err != nil {
		command := exec.CommandContext(ctx, "__agentops_tracker_resolution_failed__", args...)
		command.Dir = cwd
		return command
	}
	command := exec.CommandContext(ctx, resolution.Binary, args...) // #nosec G204 -- binary is constrained to the selected br|bd tracker.
	command.Dir = resolution.WorkDir
	command.Env = append([]string(nil), resolution.ChildEnv...)
	return command
}

func beadsTrackerEnvForDir(cwd string) []string {
	ledger := trackerresolve.ResolveLedger(cwd, os.Environ(), trackerresolve.BR)
	return trackerresolve.ChildEnvironment(os.Environ(), trackerresolve.BR, ledger.Path)
}

func beadsEnvValue(env []string) (string, bool) {
	return trackerresolve.BeadsDirValue(env)
}

func repoRootForBeads(cwd string) (string, error) {
	return trackerresolve.RepoRoot(cwd), nil
}
