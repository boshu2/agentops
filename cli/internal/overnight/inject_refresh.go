package overnight

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/search"
)

// InjectRefreshResult describes the outcome of a single inject-cache
// refresh. The stage is best-effort: missing inject subsystems degrade
// gracefully without failing the iteration.
type InjectRefreshResult struct {
	// Attempted is true when RefreshInjectCache actually tried to
	// rebuild the cache (as opposed to exiting early because of a
	// cancelled context).
	Attempted bool

	// Succeeded is true when the chosen refresh method completed
	// without error.
	Succeeded bool

	// Method records the path the refresh took:
	//   "in-process" — search.BuildIndex + search.SaveIndex driven by
	//                  this package directly (preferred).
	//   "subprocess" — the `ao store rebuild` fallback, used only when
	//                  the in-process path errored and the `ao` binary
	//                  is resolvable on PATH.
	//   "skipped"    — neither path was viable (no corpus, missing
	//                  binary, unresolvable PATH, etc.); this is an
	//                  honest degradation, not a failure.
	Method string

	// Duration is the wall-clock time the refresh took end-to-end.
	Duration time.Duration

	// Degraded lists human-readable notes for partial failures,
	// fallbacks, and skips. The outer ReduceResult bubbles these up so
	// the morning report can show why the cache may be stale.
	Degraded []string

	// ErrorMessage captures the last hard error string, empty when
	// Succeeded is true.
	ErrorMessage string
}

// refreshInjectCacheFn is the package-level override point used by
// tests. RunReduce calls this indirect rather than RefreshInjectCache
// directly so test fakes can short-circuit the stage without spawning
// exec.Command. Mirrors the pattern used by Wave 3 Worker 14's recovery
// tests.
var refreshInjectCacheFn = RefreshInjectCache

// defaultInjectRefreshTimeout is the fallback deadline used when the
// caller-supplied context has no deadline of its own. A minute is more
// than enough for BuildIndex on any reasonable .agents/ tree and keeps
// a runaway subprocess from stalling REDUCE.
const defaultInjectRefreshTimeout = 60 * time.Second

// RefreshInjectCache rebuilds the inject cache so the morning day-time
// session sees the freshly-compounded corpus.
//
// This closes PRODUCT.md Gap #1's loop framing ("harvest → forge →
// INJECT → report") — without it, the MEASURE stage's inject-visibility
// probe reports on a stale cache.
//
// Strategy:
//
//  1. Prefer an in-process rebuild via search.BuildIndex +
//     search.SaveIndex against <cwd>/.agents/. These are the same
//     primitives the `ao store rebuild` command invokes, so the on-disk
//     artifact is identical.
//  2. On in-process error, fall back to `ao store rebuild` as a
//     subprocess with a deadline derived from ctx (or
//     defaultInjectRefreshTimeout when ctx has none).
//  3. If the `ao` binary is unresolvable via exec.LookPath, return a
//     degraded result with Method="skipped" and a clear note. A stale
//     cache is less bad than a hard iteration failure.
//
// RefreshInjectCache never mutates source code, never touches git, and
// never spawns swarm agents.
func RefreshInjectCache(ctx context.Context, cwd string, log io.Writer) (*InjectRefreshResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if log == nil {
		log = io.Discard
	}
	started := time.Now()
	result := &InjectRefreshResult{
		Attempted: true,
	}

	if cwd == "" {
		result.Duration = stageDurationSince(started)
		result.Method = "skipped"
		result.ErrorMessage = "inject-refresh: empty cwd"
		result.Degraded = append(result.Degraded,
			"inject-refresh: empty cwd, skipped")
		return result, fmt.Errorf("overnight: RefreshInjectCache requires a non-empty cwd")
	}

	agentsDir := filepath.Join(cwd, ".agents")
	if _, err := os.Stat(agentsDir); err != nil {
		result.Duration = stageDurationSince(started)
		result.Method = "skipped"
		result.ErrorMessage = fmt.Sprintf("inject-refresh: .agents/ unavailable: %v", err)
		result.Degraded = append(result.Degraded,
			"inject-refresh: .agents/ unavailable, skipped")
		fmt.Fprintf(log, "overnight/inject-refresh: skipped (%v)\n", err)
		if errors.Is(err, os.ErrNotExist) {
			// No corpus is not a hard failure — REDUCE still has
			// meaningful work upstream. Bubble up a soft skip.
			return result, nil
		}
		return result, fmt.Errorf("overnight: stat .agents/: %w", err)
	}

	fmt.Fprintln(log, "overnight/inject-refresh: start (in-process)")

	if err := refreshInProcess(agentsDir); err == nil {
		result.Method = "in-process"
		result.Succeeded = true
		result.Duration = stageDurationSince(started)
		fmt.Fprintf(log, "overnight/inject-refresh: done in %s (in-process)\n",
			result.Duration)
		return result, nil
	} else {
		result.Degraded = append(result.Degraded,
			fmt.Sprintf("inject-refresh: in-process failed: %v", err))
		fmt.Fprintf(log, "overnight/inject-refresh: in-process failed: %v\n", err)
	}

	// Fallback: subprocess. Honour the caller's deadline; fall back to
	// defaultInjectRefreshTimeout when the context has none.
	subCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		subCtx, cancel = context.WithTimeout(ctx, defaultInjectRefreshTimeout)
		defer cancel()
	}

	// Route the lookup AND the command through the package-level shim
	// vars (exec_shim.go). Boundary tests intercept these to enforce
	// the anti-goals mechanically — any regression that adds a git or
	// rpi subprocess call here would be caught by the shim, not left
	// to slip through a direct exec.* call.
	aoBin, lookErr := ExecLookPath("ao")
	if lookErr != nil {
		result.Method = "skipped"
		result.Duration = stageDurationSince(started)
		result.ErrorMessage = fmt.Sprintf("inject-refresh: ao binary not on PATH: %v", lookErr)
		result.Degraded = append(result.Degraded,
			"inject-refresh: ao binary not on PATH, skipped subprocess fallback")
		fmt.Fprintf(log, "overnight/inject-refresh: skipped subprocess (%v)\n", lookErr)
		// Honest degradation — not a hard error.
		return result, nil
	}

	cmd := ExecCommandContext(subCtx, aoBin, "store", "rebuild")
	cmd.Dir = cwd
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Run(); err != nil {
		result.Method = "subprocess"
		result.Duration = stageDurationSince(started)
		result.ErrorMessage = fmt.Sprintf("inject-refresh: subprocess failed: %v", err)
		result.Degraded = append(result.Degraded,
			fmt.Sprintf("inject-refresh: subprocess failed: %v", err))
		fmt.Fprintf(log, "overnight/inject-refresh: subprocess failed: %v\n", err)
		return result, fmt.Errorf("overnight: inject-refresh subprocess: %w", err)
	}

	result.Method = "subprocess"
	result.Succeeded = true
	result.Duration = stageDurationSince(started)
	fmt.Fprintf(log, "overnight/inject-refresh: done in %s (subprocess)\n",
		result.Duration)
	return result, nil
}

// refreshInProcess rebuilds the inject cache by scanning agentsDir with
// search.BuildIndex and writing the result via search.SaveIndex. These
// are the same primitives driven by the `ao store rebuild` subcommand,
// so the on-disk artifact is identical.
func refreshInProcess(agentsDir string) error {
	idx, err := search.BuildIndex(agentsDir)
	if err != nil {
		return fmt.Errorf("build index: %w", err)
	}
	idxPath := filepath.Join(agentsDir, "index.jsonl")
	if err := search.SaveIndex(idx, idxPath); err != nil {
		return fmt.Errorf("save index: %w", err)
	}
	return nil
}
