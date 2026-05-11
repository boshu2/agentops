// practices: [agile-manifesto, dora-metrics]
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	rpilib "github.com/boshu2/agentops/cli/internal/rpi"
)

// PhaseExecutor abstracts the backend used to run a single phase session.
//
// Selection policy (deterministic, logged):
//
//	stream  — stream-json execution with live parsing and fallback semantics.
//	direct  — plain prompt execution without stream parsing.
//
// The chosen backend is recorded in phasedState.Backend, emitted to stdout, and
// appended to the orchestration log so every run has a traceable selection record.
type PhaseExecutor interface {
	// Name returns the backend identifier ("direct" or "stream").
	Name() string
	// Execute runs the phase prompt and blocks until the session completes.
	// The context carries cancellation signals (e.g. SIGINT/SIGTERM) from the
	// orchestrator so that long-running phase sessions can be interrupted.
	Execute(ctx context.Context, prompt, cwd, runID string, phaseNum int) error
}

type directExecutor struct {
	runtimeCommand string
	phaseTimeout   time.Duration
	stdoutWriter   io.Writer // defaults to os.Stdout; set to io.Discard when dashboard active
}

func (d *directExecutor) Name() string { return "direct" }

func (d *directExecutor) effectiveStdoutWriter() io.Writer {
	if d.stdoutWriter != nil {
		return d.stdoutWriter
	}
	return os.Stdout
}

func (d *directExecutor) Execute(ctx context.Context, prompt, cwd, runID string, phaseNum int) error {
	if _, err := appendRPIC2Event(cwd, rpiC2EventInput{
		RunID: runID, Phase: phaseNum, Backend: "direct", Source: "runtime_direct",
		Type:    "phase.direct.started",
		Message: fmt.Sprintf("phase %d direct session started", phaseNum),
	}); err != nil {
		VerbosePrintf("Warning: could not append direct start event: %v\n", err)
	}
	execErr := spawnRuntimeDirectWithWriter(d.runtimeCommand, prompt, cwd, phaseNum, d.phaseTimeout, d.effectiveStdoutWriter())
	evType, evMsg := "phase.direct.completed", fmt.Sprintf("phase %d direct session completed", phaseNum)
	if execErr != nil {
		evType, evMsg = "phase.direct.failed", execErr.Error()
	}
	if _, err := appendRPIC2Event(cwd, rpiC2EventInput{
		RunID: runID, Phase: phaseNum, Backend: "direct", Source: "runtime_direct",
		Type: evType, Message: evMsg,
	}); err != nil {
		VerbosePrintf("Warning: could not append direct completion event: %v\n", err)
	}
	return execErr
}

type streamExecutor struct {
	runtimeCommand       string
	statusPath           string
	allPhases            []PhaseProgress
	phaseTimeout         time.Duration
	stallTimeout         time.Duration
	streamStartupTimeout time.Duration
	stallCheckInterval   time.Duration
	stdoutWriter         io.Writer // defaults to os.Stdout; set to io.Discard when dashboard active
}

func (s *streamExecutor) Name() string { return "stream" }

func (s *streamExecutor) effectiveStdoutWriter() io.Writer {
	if s.stdoutWriter != nil {
		return s.stdoutWriter
	}
	return os.Stdout
}

func (s *streamExecutor) Execute(ctx context.Context, prompt, cwd, runID string, phaseNum int) error {
	err := spawnRuntimePhaseWithStream(s.runtimeCommand, prompt, cwd, runID, phaseNum, s.statusPath, s.allPhases, s.phaseTimeout, s.stallTimeout, s.streamStartupTimeout, s.stallCheckInterval, s.effectiveStdoutWriter())
	if err == nil {
		return nil
	}
	if !shouldFallbackToDirect(err) {
		return err
	}
	fmt.Printf("Stream backend degraded for phase %d; falling back to direct execution (%v)\n", phaseNum, err)
	if _, evErr := appendRPIC2Event(cwd, rpiC2EventInput{
		RunID: runID, Phase: phaseNum, Backend: "direct", Source: "stream_fallback",
		Type:    "phase.direct.started",
		Message: fmt.Sprintf("phase %d: stream degraded, switched to direct", phaseNum),
	}); evErr != nil {
		VerbosePrintf("Warning: could not append fallback start event: %v\n", evErr)
	}
	directErr := spawnRuntimeDirectWithWriter(s.runtimeCommand, prompt, cwd, phaseNum, s.phaseTimeout, s.effectiveStdoutWriter())
	evType, evMsg := "phase.direct.completed", fmt.Sprintf("phase %d direct fallback completed", phaseNum)
	if directErr != nil {
		evType, evMsg = "phase.direct.failed", directErr.Error()
	}
	if _, evErr := appendRPIC2Event(cwd, rpiC2EventInput{
		RunID: runID, Phase: phaseNum, Backend: "direct", Source: "stream_fallback",
		Type: evType, Message: evMsg,
	}); evErr != nil {
		VerbosePrintf("Warning: could not append fallback completion event: %v\n", evErr)
	}
	if directErr != nil {
		return fmt.Errorf("stream execution failed: %w; direct fallback failed: %v", err, directErr)
	}
	return nil
}

func shouldFallbackToDirect(err error) bool {
	return rpilib.ShouldFallbackToDirect(err, string(failReasonStall))
}

func runtimeBinaryName(command string) string {
	return rpilib.RuntimeBinaryName(command)
}

func splitRuntimeCommand(command string) (string, []string) {
	return rpilib.SplitRuntimeCommand(command)
}

func runtimeDirectCommandArgs(command, prompt string) []string {
	return rpilib.RuntimeDirectCommandArgs(command, prompt)
}

func runtimeStreamCommandArgs(command, prompt string) ([]string, error) {
	return rpilib.RuntimeStreamCommandArgs(command, prompt)
}

func formatRuntimePromptInvocation(command, prompt string) string {
	return rpilib.FormatRuntimePromptInvocation(command, prompt)
}

// backendCapabilities probes the runtime environment for executor prerequisites.
// All fields are populated by probeBackendCapabilities.
type backendCapabilities struct {
	// LiveStatusEnabled is true when --live-status flag is set.
	LiveStatusEnabled bool
	// RuntimeMode is one of auto|direct|stream.
	RuntimeMode string
}

// probeBackendCapabilities detects available backends in the current environment.
// It is a pure function (no side effects) to keep selectExecutor testable.
func probeBackendCapabilities(liveStatus bool, runtimeMode string) backendCapabilities {
	caps := rpilib.ProbeBackendCapabilities(liveStatus, runtimeMode, normalizeRuntimeMode)
	return backendCapabilities{
		LiveStatusEnabled: caps.LiveStatusEnabled,
		RuntimeMode:       caps.RuntimeMode,
	}
}

// selectExecutorFromCaps resolves an executor from pre-probed capabilities.
// This is the deterministic core of backend selection — testable without env mutation.
// opts provides timeout/interval values for the executors; use defaultPhasedEngineOptions()
// when calling from tests that do not have a full opts available.
//
// Selection order (first match wins):
//  1. runtime=stream — always stream
//  2. runtime=direct — always direct
//  3. runtime=gc     — Gas City session management
//  4. runtime=auto   — stream when live-status enabled, otherwise direct
func selectExecutorFromCaps(caps backendCapabilities, statusPath string, allPhases []PhaseProgress, opts phasedEngineOptions) (PhaseExecutor, string) {
	stdWriter := opts.StdoutWriter
	if stdWriter == nil {
		stdWriter = os.Stdout
	}

	switch caps.RuntimeMode {
	case "stream":
		return &streamExecutor{
			runtimeCommand:       opts.RuntimeCommand,
			statusPath:           statusPath,
			allPhases:            allPhases,
			phaseTimeout:         opts.PhaseTimeout,
			stallTimeout:         opts.StallTimeout,
			streamStartupTimeout: opts.StreamStartupTimeout,
			stallCheckInterval:   opts.StallCheckInterval,
			stdoutWriter:         stdWriter,
		}, "runtime=stream"
	case "direct":
		return &directExecutor{
			runtimeCommand: opts.RuntimeCommand,
			phaseTimeout:   opts.PhaseTimeout,
			stdoutWriter:   stdWriter,
		}, "runtime=direct"
	case "tmux":
		return &tmuxExecutor{
			tmuxCommand:    effectiveTmuxCommand(opts.TmuxCommand),
			runtimeCommand: opts.RuntimeCommand,
			phaseTimeout:   opts.PhaseTimeout,
			pollInterval:   5 * time.Second,
			workerCount:    opts.TmuxWorkers,
		}, "runtime=tmux"
	case "gc":
		return &gcExecutor{
			cityPath:     gcCityPathFromOpts(opts),
			phaseTimeout: opts.PhaseTimeout,
			pollInterval: 10 * time.Second,
			execCommand:  opts.ExecCommand,
			lookPath:     opts.LookPath,
			apiClient:    opts.GasCityClient,
			apiCityName:  opts.GCCityName,
		}, gcExecutorSelectionReason("runtime=gc", opts.GasCityClient, "")
	default: // auto — prefer gc when available, fall back to stream
		gcReady, gcReason := gcExecutorAvailability(opts.WorkingDir, opts.ExecCommand, opts.LookPath)
		if gcReady {
			return &gcExecutor{
				cityPath:     gcCityPathFromOpts(opts),
				phaseTimeout: opts.PhaseTimeout,
				pollInterval: 10 * time.Second,
				execCommand:  opts.ExecCommand,
				lookPath:     opts.LookPath,
				apiClient:    opts.GasCityClient,
				apiCityName:  opts.GCCityName,
			}, gcExecutorSelectionReason("runtime=auto", opts.GasCityClient, "")
		}
		return &streamExecutor{
			runtimeCommand:       opts.RuntimeCommand,
			statusPath:           statusPath,
			allPhases:            allPhases,
			phaseTimeout:         opts.PhaseTimeout,
			stallTimeout:         opts.StallTimeout,
			streamStartupTimeout: opts.StreamStartupTimeout,
			stallCheckInterval:   opts.StallCheckInterval,
			stdoutWriter:         stdWriter,
		}, fmt.Sprintf("runtime=auto backend=stream gc-degraded=%q", gcReason)
	}
}

// selectExecutor resolves the executor backend based on flags and environment.
// The selection policy, chosen backend, and reason are logged to logPath for
// observability. Pass an empty logPath to skip log writing (e.g., in tests).
//
// Selection order: runtime override (stream/direct) > auto (live-status=>stream, else direct).
func selectExecutor(statusPath string, allPhases []PhaseProgress) PhaseExecutor {
	return selectExecutorWithLog(statusPath, allPhases, "", "", false, defaultPhasedEngineOptions())
}

// selectExecutorWithLog is the log-aware variant used by runRPIPhasedWithOpts.
// logPath and runID are used to append the selection record to the orchestration log.
// liveStatus must be provided explicitly so the function does not read package globals.
// opts provides timeout/interval values embedded into the returned executor.
func selectExecutorWithLog(statusPath string, allPhases []PhaseProgress, logPath, runID string, liveStatus bool, opts phasedEngineOptions) PhaseExecutor {
	caps := probeBackendCapabilities(liveStatus, opts.RuntimeMode)
	executor, reason := selectExecutorFromCaps(caps, statusPath, allPhases, opts)
	displayBackend := executor.Name()
	if gcExec, ok := executor.(*gcExecutor); ok {
		displayBackend = gcExec.backendMode()
	}
	msg := fmt.Sprintf("backend=%s reason=%q", displayBackend, reason)
	fmt.Printf("Executor backend: %s (%s)\n", displayBackend, reason)
	if logPath != "" {
		logPhaseTransition(logPath, runID, "backend-selection", msg)
	}
	return executor
}

// Exit codes for phased orchestration.
const (
	ExitGateFail  = 10 // Council gate returned FAIL
	ExitUserAbort = 20 // User cancelled the session
	ExitCLIError  = 30 // Runtime CLI error (not found, config issue)
)

// spawnClaudePhase exists for compatibility with legacy direct-spawn tests.
// Production code should use a PhaseExecutor (selectExecutorFromCaps) instead.
func spawnClaudePhase(prompt, cwd, _ string, phaseNum int) error {
	return spawnClaudeDirectGlobal(prompt, cwd, phaseNum)
}

// spawnRuntimeDirectWithWriter runs <runtimeCommand> -p directly, routing stdout to stdoutWriter.
// phaseTimeout controls the maximum runtime; pass 0 to disable the timeout.
func spawnRuntimeDirectWithWriter(runtimeCommand, prompt, cwd string, phaseNum int, phaseTimeout time.Duration, stdoutWriter io.Writer) error {
	command := effectiveRuntimeCommand(runtimeCommand)
	executable, _ := splitRuntimeCommand(command)
	if executable == "" {
		return fmt.Errorf("runtime command is empty")
	}
	args := runtimeDirectCommandArgs(command, prompt)

	ctx := context.Background()
	cancel := func() {}
	if phaseTimeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), phaseTimeout)
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = cwd
	cmd.Stdout = stdoutWriter
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = cleanEnvNoClaude()
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("phase %d timed out after %s (set --phase-timeout to increase)", phaseNum, phaseTimeout)
	}
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("%s exited with code %d: %w", command, exitErr.ExitCode(), err)
	}
	return fmt.Errorf("%s execution failed: %w", command, err)
}

// spawnRuntimeDirectImpl is the backward-compatible wrapper using os.Stdout.
func spawnRuntimeDirectImpl(runtimeCommand, prompt, cwd string, phaseNum int, phaseTimeout time.Duration) error {
	return spawnRuntimeDirectWithWriter(runtimeCommand, prompt, cwd, phaseNum, phaseTimeout, os.Stdout)
}

// spawnClaudeDirectImpl is the legacy wrapper pinned to the default runtime.
func spawnClaudeDirectImpl(prompt, cwd string, phaseNum int, phaseTimeout time.Duration) error {
	return spawnRuntimeDirectImpl("claude", prompt, cwd, phaseNum, phaseTimeout)
}

// spawnRuntimePhaseWithStream spawns a runtime session using --output-format stream-json
// and feeds stdout through ParseStreamEvents for live progress tracking.
// An onUpdate callback calls WriteLiveStatus after every parsed event so that
// external watchers (e.g. ao status) can tail the status file.
// Stderr is passed through to os.Stderr for real-time error visibility.
// phaseTimeout, stallTimeout, streamStartupTimeout, and checkInterval are passed
// explicitly so the function does not read package-level globals.
// streamWatchdogState holds the shared atomic state used by watchdog goroutines
// and the stream event callback to coordinate stall/startup detection.
type streamWatchdogState struct {
	eventCount       atomic.Int64
	lastActivityUnix atomic.Int64
}

func spawnRuntimePhaseWithStream(runtimeCommand, prompt, cwd, runID string, phaseNum int, statusPath string, allPhases []PhaseProgress, phaseTimeout, stallTimeout, streamStartupTimeout, checkInterval time.Duration, stdoutWriter io.Writer) error {
	command := effectiveRuntimeCommand(runtimeCommand)
	executable, _ := splitRuntimeCommand(command)
	if executable == "" {
		return fmt.Errorf("runtime command is empty")
	}
	args, argsErr := runtimeStreamCommandArgs(command, prompt)
	if argsErr != nil {
		return argsErr
	}

	ctx, cancel := buildStreamPhaseContext(context.Background(), phaseTimeout)
	defer cancel()

	effectiveCheckInterval := normalizeCheckInterval(checkInterval)
	startedAt := time.Now()

	watchdog := &streamWatchdogState{}
	watchdog.lastActivityUnix.Store(startedAt.UnixNano())

	stallCtx, stallCancel := context.WithCancelCause(ctx)
	defer stallCancel(nil)

	startStreamWatchdogs(stallCtx, stallCancel, watchdog, startedAt, effectiveCheckInterval, stallTimeout, streamStartupTimeout)

	cmd := exec.CommandContext(stallCtx, executable, args...)
	cmd.Dir = cwd
	cmd.Stderr = os.Stderr
	cmd.Env = cleanEnvNoClaude()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", command, err)
	}
	if _, err := appendRPIC2Event(cwd, rpiC2EventInput{
		RunID:   runID,
		Phase:   phaseNum,
		Backend: "stream",
		Source:  "runtime_stream",
		Type:    "phase.stream.started",
		Message: fmt.Sprintf("phase %d stream session started", phaseNum),
	}); err != nil {
		VerbosePrintf("Warning: could not append stream start event: %v\n", err)
	}
	go closeStreamPipeOnCancel(stallCtx, stdout)

	onUpdate := buildStreamUpdateCallback(watchdog, allPhases, phaseNum, statusPath)
	onEvent := func(ev StreamEvent) {
		if _, err := appendRPIC2Event(cwd, mapStreamEventToRPIC2(runID, phaseNum, ev)); err != nil {
			VerbosePrintf("Warning: could not append stream event: %v\n", err)
		}
	}
	tee := io.TeeReader(stdout, stdoutWriter)
	_, parseErr := ParseStreamEventsWithHandler(tee, onEvent, onUpdate)
	waitErr := cmd.Wait()

	resultErr := classifyStreamResult(ctx, stallCtx, command, phaseNum, phaseTimeout, waitErr, parseErr, watchdog.eventCount.Load())
	resultType := "phase.stream.completed"
	resultMessage := fmt.Sprintf("phase %d stream session completed", phaseNum)
	if resultErr != nil {
		resultType = "phase.stream.failed"
		resultMessage = resultErr.Error()
	}
	if _, err := appendRPIC2Event(cwd, rpiC2EventInput{
		RunID:   runID,
		Phase:   phaseNum,
		Backend: "stream",
		Source:  "runtime_stream",
		Type:    resultType,
		Message: resultMessage,
	}); err != nil {
		VerbosePrintf("Warning: could not append stream completion event: %v\n", err)
	}
	return resultErr
}

func closeStreamPipeOnCancel(ctx context.Context, stdout io.ReadCloser) {
	<-ctx.Done()
	if err := stdout.Close(); err != nil {
		VerbosePrintf("Warning: error closing stdout pipe: %v\n", err)
	}
}

// buildStreamPhaseContext creates a context with optional timeout for a stream phase.
// It derives from parent so that orchestrator-level cancellation propagates.
func buildStreamPhaseContext(parent context.Context, phaseTimeout time.Duration) (context.Context, context.CancelFunc) {
	return rpilib.BuildStreamPhaseContext(parent, phaseTimeout)
}

// normalizeCheckInterval returns checkInterval or a 1s default.
func normalizeCheckInterval(checkInterval time.Duration) time.Duration {
	return rpilib.NormalizeCheckInterval(checkInterval)
}

// startStreamWatchdogs launches startup and stall watchdog goroutines.
func startStreamWatchdogs(stallCtx context.Context, stallCancel context.CancelCauseFunc, watchdog *streamWatchdogState, startedAt time.Time, checkInterval, stallTimeout, startupTimeout time.Duration) {
	if startupTimeout > 0 {
		startupCheckInterval := checkInterval
		if startupCheckInterval > 5*time.Second {
			startupCheckInterval = 5 * time.Second
		}
		go runStartupWatchdog(stallCtx, stallCancel, &watchdog.eventCount, startedAt, startupCheckInterval, startupTimeout)
	}
	if stallTimeout > 0 {
		go runStallWatchdog(stallCtx, stallCancel, &watchdog.lastActivityUnix, checkInterval, stallTimeout)
	}
}

// runStartupWatchdog cancels the context if no stream events arrive within the timeout.
func runStartupWatchdog(ctx context.Context, cancel context.CancelCauseFunc, eventCount *atomic.Int64, startedAt time.Time, checkInterval, startupTimeout time.Duration) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if eventCount.Load() > 0 {
				return
			}
			if time.Since(startedAt) > startupTimeout {
				cancel(fmt.Errorf("stream startup timeout: no events received after %s", startupTimeout))
				return
			}
		}
	}
}

// runStallWatchdog cancels the context if no stream activity occurs within the timeout.
func runStallWatchdog(ctx context.Context, cancel context.CancelCauseFunc, lastActivityUnix *atomic.Int64, checkInterval, stallTimeout time.Duration) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			last := time.Unix(0, lastActivityUnix.Load())
			if time.Since(last) > stallTimeout {
				cancel(fmt.Errorf("stall detected: no stream activity for %s", stallTimeout))
				return
			}
		}
	}
}

// buildStreamUpdateCallback creates the onUpdate callback for ParseStreamEvents
// that records activity and merges progress into allPhases.
func buildStreamUpdateCallback(watchdog *streamWatchdogState, allPhases []PhaseProgress, phaseNum int, statusPath string) func(PhaseProgress) {
	phaseIdx := phaseNum - 1
	return func(p PhaseProgress) {
		watchdog.eventCount.Add(1)
		watchdog.lastActivityUnix.Store(time.Now().UnixNano())

		if phaseIdx >= 0 && phaseIdx < len(allPhases) {
			mergePhaseProgress(&allPhases[phaseIdx], p)
		}
		if writeErr := WriteLiveStatus(statusPath, allPhases, phaseIdx); writeErr != nil {
			VerbosePrintf("Warning: could not write live status: %v\n", writeErr)
		}
	}
}

// mergePhaseProgress updates dst with non-zero fields from src.
func mergePhaseProgress(dst *PhaseProgress, src PhaseProgress) {
	if src.Name != "" {
		dst.Name = src.Name
	}
	if src.CurrentAction != "" {
		dst.CurrentAction = src.CurrentAction
	}
	if src.RetryCount != 0 {
		dst.RetryCount = src.RetryCount
	}
	if src.LastError != "" {
		dst.LastError = src.LastError
	}
}

// classifyStreamResult examines the context, wait error, and parse error to
// produce the appropriate error for a completed stream-json phase.
func classifyStreamResult(ctx, stallCtx context.Context, command string, phaseNum int, phaseTimeout time.Duration, waitErr, parseErr error, eventCount int64) error {
	return rpilib.ClassifyStreamResult(ctx, stallCtx, command, phaseNum, phaseTimeout, waitErr, parseErr, eventCount,
		string(failReasonTimeout), string(failReasonStall), string(failReasonExit), string(failReasonUnknown))
}

// spawnClaudePhaseWithStream is the legacy wrapper pinned to the default runtime.
func spawnClaudePhaseWithStream(prompt, cwd, runID string, phaseNum int, statusPath string, allPhases []PhaseProgress, phaseTimeout, stallTimeout, streamStartupTimeout, checkInterval time.Duration) error {
	return spawnRuntimePhaseWithStream("claude", prompt, cwd, runID, phaseNum, statusPath, allPhases, phaseTimeout, stallTimeout, streamStartupTimeout, checkInterval, os.Stdout)
}

func updateLivePhaseStatus(statusPath string, allPhases []PhaseProgress, phaseNum int, action string, retries int, lastErr string) {
	phaseIdx := phaseNum - 1
	if phaseIdx < 0 || phaseIdx >= len(allPhases) {
		return
	}

	p := allPhases[phaseIdx]
	if action != "" {
		p.CurrentAction = summarizeStatusAction(action)
	}
	p.RetryCount = retries
	p.LastError = summarizeStatusAction(lastErr)
	p.LastUpdate = time.Now()
	allPhases[phaseIdx] = p

	if err := WriteLiveStatus(statusPath, allPhases, phaseIdx); err != nil {
		VerbosePrintf("Warning: could not write live status: %v\n", err)
	}
}

// buildAllPhases constructs a []PhaseProgress with Name fields populated
// from the global phases slice, used as the initial state for live status tracking.
func buildAllPhases(phaseDefs []phase) []PhaseProgress {
	defs := make([]rpilib.PhaseNameDef, len(phaseDefs))
	for i, p := range phaseDefs {
		defs[i] = rpilib.PhaseNameDef{Name: p.Name}
	}
	updates := rpilib.BuildAllPhaseProgress(defs)
	all := make([]PhaseProgress, len(updates))
	for i, u := range updates {
		all[i] = PhaseProgress{Name: u.Name, CurrentAction: u.CurrentAction}
	}
	return all
}

// cleanEnvNoClaude builds a clean env without CLAUDECODE to avoid nesting guard.
func cleanEnvNoClaude() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDECODE=") || strings.HasPrefix(e, "CLAUDE_CODE_") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// --- Runtime process helpers ---

// defaultLookPath returns exec.LookPath if fn is nil.
func defaultLookPath(fn gcLookFn) gcLookFn {
	if fn != nil {
		return fn
	}
	return exec.LookPath
}

// spawnClaudeDirectGlobal is the package-level wrapper for spawnClaudeDirectImpl that
// reads phasedPhaseTimeout from the global (for spawnClaudePhase fallback paths).
func spawnClaudeDirectGlobal(prompt, cwd string, phaseNum int) error {
	return spawnClaudeDirectImpl(prompt, cwd, phaseNum, phasedPhaseTimeout)
}

// stallCheckInterval controls how frequently the stall watchdog goroutine fires in
// spawnClaudePhaseWithStream. Overridable in tests to exercise stall detection quickly.
var stallCheckInterval = 30 * time.Second
