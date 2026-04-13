package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// gcExecutor implements PhaseExecutor using Gas City's gc CLI for session management.
// It starts a gc session with the phase prompt via `gc session nudge`, monitors
// progress via `gc session peek`, and emits ao:phase events to the gc event bus.
type gcExecutor struct {
	cityPath     string        // path to city.toml directory; empty = auto-discover
	phaseTimeout time.Duration // max time per phase
	pollInterval time.Duration // how often to check session status
	execCommand  gcExecFn      // nil = exec.Command
	lookPath     gcLookFn      // nil = exec.LookPath
}

func (g *gcExecutor) Name() string { return "gc" }

func (g *gcExecutor) Execute(ctx context.Context, prompt, cwd, runID string, phaseNum int) error {
	cityPath := g.resolveCityPath(cwd)
	if cityPath == "" {
		return fmt.Errorf("gc executor: no city.toml found (walk up from %s)", cwd)
	}

	ready, reason := gcBridgeReady(cityPath, g.execCommand, g.lookPath)
	if !ready {
		return fmt.Errorf("gc executor: not ready: %s", reason)
	}

	_ = gcEmitPhaseEvent(cityPath, phaseNum, "started", runID, g.execCommand, g.lookPath)

	sessionAlias := fmt.Sprintf("rpi-%s-p%d", runID, phaseNum)
	if err := gcRunCommand(g.execCommand, cityPath, "session", "new", "--alias", sessionAlias, "--template", "worker"); err != nil {
		return fmt.Errorf("gc executor: create session %q: %w", sessionAlias, err)
	}
	if err := gcRunCommand(g.execCommand, cityPath, gcNudgeArgs(sessionAlias, prompt)...); err != nil {
		return fmt.Errorf("gc executor: nudge session %q: %w", sessionAlias, err)
	}

	return g.pollSessionCompletion(ctx, cityPath, sessionAlias, runID, phaseNum)
}

// resolveCityPath returns the city path from the executor config or discovers it.
func (g *gcExecutor) resolveCityPath(cwd string) string {
	if g.cityPath != "" {
		return g.cityPath
	}
	return gcBridgeCityPath(cwd)
}

// pollSessionCompletion blocks until the gc session finishes, is cancelled, or times out.
func (g *gcExecutor) pollSessionCompletion(ctx context.Context, cityPath, sessionAlias, runID string, phaseNum int) error {
	pollInterval := g.pollInterval
	if pollInterval == 0 {
		pollInterval = 10 * time.Second
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	timeout := g.phaseTimeout
	if timeout == 0 {
		timeout = 90 * time.Minute
	}
	deadline := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			_ = gcEmitPhaseEvent(cityPath, phaseNum, "cancelled", runID, g.execCommand, g.lookPath)
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("gc executor: phase %d timed out after %v", phaseNum, timeout)
		case <-ticker.C:
			done, err := g.checkSessionDone(cityPath, sessionAlias)
			if err != nil {
				continue // transient error, retry on next tick
			}
			if done {
				_ = gcEmitPhaseEvent(cityPath, phaseNum, "complete", runID, g.execCommand, g.lookPath)
				return nil
			}
		}
	}
}

// checkSessionDone returns true if the session is closed/completed or has disappeared.
func (g *gcExecutor) checkSessionDone(cityPath, sessionAlias string) (bool, error) {
	out, err := gcDefaultExec(g.execCommand)("gc", "--city", cityPath, "session", "list", "--json").Output()
	if err != nil {
		return false, fmt.Errorf("gc session list: %w", err)
	}
	sessions, err := parseGCSessions(out)
	if err != nil {
		return false, fmt.Errorf("parse sessions: %w", err)
	}
	for _, s := range sessions {
		if s.Alias == sessionAlias {
			return s.State == "closed" || s.State == "completed", nil
		}
	}
	// Session disappeared — likely controller crash or cleanup
	fmt.Printf("WARN: gc session %q not found in session list — treating as complete\n", sessionAlias)
	return true, nil
}

// gcRunCommand runs a gc CLI command with optional city path prefix.
func gcRunCommand(execCommand gcExecFn, cityPath string, args ...string) error {
	if cityPath != "" {
		// Check if --city is already in args
		hasCity := false
		for _, a := range args {
			if a == "--city" {
				hasCity = true
				break
			}
		}
		if !hasCity {
			args = append([]string{"--city", cityPath}, args...)
		}
	}
	cmd := gcDefaultExec(execCommand)("gc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// gcExecutorAvailable returns true if gc bridge is ready for use as a phase executor.
// This is used by selectExecutorFromCaps to determine if the gc backend should be offered.
func gcExecutorAvailable(cwd string, execCommand gcExecFn, lookPath gcLookFn) bool {
	cityPath := gcBridgeCityPath(cwd)
	if cityPath == "" {
		return false
	}
	ready, _ := gcBridgeReady(cityPath, execCommand, lookPath)
	return ready
}

// gcCityPathFromOpts extracts the city path from opts or discovers it from cwd.
func gcCityPathFromOpts(opts phasedEngineOptions) string {
	if p := strings.TrimSpace(opts.GCCityPath); p != "" {
		return p
	}
	return gcBridgeCityPath(opts.WorkingDir)
}
