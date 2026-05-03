package eval

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"time"

	rpilib "github.com/boshu2/agentops/cli/internal/rpi"
)

const (
	claudeExecutable = "claude"
	codexExecutable  = "codex"
)

// LiveRuntimeOptions configures an optional live runtime adapter execution.
// Callers must set Enabled=true before any external Claude/Codex process is run.
type LiveRuntimeOptions struct {
	Suite          *Suite
	SuitePath      string
	SuiteData      []byte
	RunID          string
	Runtime        Runtime
	RuntimeCommand string
	OutputPath     string
	WorkDir        string
	Env            []string
	IsolationRoot  string
	Model          string
	Profile        string
	Enabled        bool
	Now            func() time.Time
	LookPath       func(string) (string, error)
	VersionRunner  RuntimeVersionRunner
	Runner         RuntimeRunner
	// OverrideDisableHooks forces the run to behave as if Suite.Environment.DisableHooks
	// were true, without mutating the loaded suite. Used by the baseline A/B runner to
	// toggle skill-on vs skill-off across two runs over the same suite.
	OverrideDisableHooks bool
}

// RuntimeCommand describes one Claude or Codex process invocation.
type RuntimeCommand struct {
	Executable     string
	Args           []string
	Env            []string
	Dir            string
	TimeoutSeconds int
	Attempt        int
}

// RuntimeExecutionResult captures the score and artifacts returned by a live
// runtime adapter invocation.
type RuntimeExecutionResult struct {
	Status          Status
	Verdict         Verdict
	AggregateScore  float64
	DimensionScores map[Dimension]float64
	CaseResults     []CaseResult
	Artifacts       []Artifact
	TranscriptPath  string
	ScorecardPath   string
	Version         string
	Model           string
	Profile         string
	Diagnostics     []string
}

// RuntimeRunner executes a live runtime prompt command.
type RuntimeRunner func(context.Context, RuntimeCommand) (RuntimeExecutionResult, error)

// RuntimeVersionRunner probes a live runtime executable for version metadata.
type RuntimeVersionRunner func(context.Context, RuntimeCommand) (string, error)

type liveRuntimeAdapter interface {
	DefaultCommand() string
	VersionArgs() []string
	DirectArgs(command, prompt string) []string
}

type staticLiveRuntimeAdapter struct {
	defaultCommand string
	versionArgs    []string
}

func (a staticLiveRuntimeAdapter) DefaultCommand() string {
	return a.defaultCommand
}

func (a staticLiveRuntimeAdapter) VersionArgs() []string {
	return append([]string{}, a.versionArgs...)
}

func (a staticLiveRuntimeAdapter) DirectArgs(command, prompt string) []string {
	return rpilib.RuntimeDirectCommandArgs(command, prompt)
}

// RunLiveRuntime executes an optional Claude or Codex adapter run and returns a
// normal eval run record. It skips instead of invoking external processes unless
// opts.Enabled is true.
func RunLiveRuntime(ctx context.Context, opts LiveRuntimeOptions) (*RunRecord, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Suite == nil {
		return nil, fmt.Errorf("suite is required")
	}
	suite := *opts.Suite
	runtimeName := opts.Runtime
	if runtimeName == "" {
		var err error
		runtimeName, err = inferLiveRuntime(suite)
		if err != nil {
			return nil, err
		}
	}
	adapter, err := liveRuntimeAdapterFor(runtimeName)
	if err != nil {
		return nil, err
	}

	workDir := opts.WorkDir
	if workDir == "" {
		workDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}
	now := opts.Now
	if now == nil {
		now = defaultNow
	}
	started := now().UTC()
	record := newLiveRuntimeRecord(opts, suite, runtimeName, workDir, started)
	model, profile := runtimeMetadataFromCommand(opts.RuntimeCommand)
	record.Runtime.Model = firstNonEmpty(opts.Model, model)
	record.Runtime.Profile = firstNonEmpty(opts.Profile, profile)

	if !opts.Enabled {
		markRuntimeSkipped(record, suite, "live runtime disabled; set LiveRuntimeOptions.Enabled to true")
		return finishLiveRuntimeRun(opts, record, now)
	}

	command := strings.TrimSpace(opts.RuntimeCommand)
	if command == "" {
		command = adapter.DefaultCommand()
	}
	executable, _ := rpilib.SplitRuntimeCommand(command)
	if executable == "" {
		markRuntimeSkipped(record, suite, "runtime command is empty")
		return finishLiveRuntimeRun(opts, record, now)
	}
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	executablePath, err := lookPath(executable)
	if err != nil {
		reason := fmt.Sprintf("%s executable not found: %v", executable, err)
		markRuntimeSkipped(record, suite, reason)
		return finishLiveRuntimeRun(opts, record, now)
	}

	env, hostNotes, err := liveRuntimeEnv(opts, suite)
	if err != nil {
		return nil, err
	}
	record.Environment.HostNotes = append(record.Environment.HostNotes, hostNotes...)

	probeRuntimeVersion(ctx, opts, adapter, record, executablePath, env, workDir)

	runner := opts.Runner
	if runner == nil {
		runner = defaultRuntimeRunner
	}
	result, attempts, runErr := runLiveRuntimeWithAttempts(ctx, runner, RuntimeCommand{
		Executable:     executablePath,
		Args:           adapter.DirectArgs(command, liveRuntimePrompt(opts, suite)),
		Env:            env,
		Dir:            workDir,
		TimeoutSeconds: record.Runtime.TimeoutSeconds,
	}, record.Runtime.Attempts)
	record.Runtime.Attempts = attempts
	if runErr != nil {
		markRuntimeError(record, suite, runErr.Error())
		return finishLiveRuntimeRun(opts, record, now)
	}
	applyRuntimeExecutionResult(record, suite, result)
	return finishLiveRuntimeRun(opts, record, now)
}

func probeRuntimeVersion(ctx context.Context, opts LiveRuntimeOptions, adapter liveRuntimeAdapter, record *RunRecord, executablePath string, env []string, workDir string) {
	versionRunner := opts.VersionRunner
	if versionRunner == nil {
		versionRunner = defaultRuntimeVersionRunner
	}
	versionCtx := ctx
	versionCancel := func() {}
	if record.Runtime.TimeoutSeconds > 0 {
		versionCtx, versionCancel = context.WithTimeout(ctx, time.Duration(record.Runtime.TimeoutSeconds)*time.Second)
	}
	version, err := versionRunner(versionCtx, RuntimeCommand{
		Executable:     executablePath,
		Args:           adapter.VersionArgs(),
		Env:            env,
		Dir:            workDir,
		TimeoutSeconds: record.Runtime.TimeoutSeconds,
	})
	versionCancel()
	if err != nil {
		if errors.Is(versionCtx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("runtime version probe timed out after %ds", record.Runtime.TimeoutSeconds)
		}
		record.Environment.HostNotes = append(record.Environment.HostNotes, "runtime version probe failed: "+err.Error())
		return
	}
	record.Runtime.Version = strings.TrimSpace(version)
}

func liveRuntimeAdapterFor(runtimeName Runtime) (liveRuntimeAdapter, error) {
	switch runtimeName {
	case RuntimeClaude:
		return staticLiveRuntimeAdapter{
			defaultCommand: claudeExecutable,
			versionArgs:    []string{"--version"},
		}, nil
	case RuntimeCodex:
		return staticLiveRuntimeAdapter{
			defaultCommand: codexExecutable,
			versionArgs:    []string{"--version"},
		}, nil
	default:
		return nil, fmt.Errorf("runtime %q is not a live adapter runtime", runtimeName)
	}
}

func inferLiveRuntime(suite Suite) (Runtime, error) {
	seen := map[Runtime]struct{}{}
	for _, runtimeName := range suite.Allowed {
		if runtimeName == RuntimeClaude || runtimeName == RuntimeCodex {
			seen[runtimeName] = struct{}{}
		}
	}
	for _, evalCase := range suite.Cases {
		if evalCase.Runtime == RuntimeClaude || evalCase.Runtime == RuntimeCodex {
			seen[evalCase.Runtime] = struct{}{}
		}
	}
	if len(seen) == 1 {
		for runtimeName := range seen {
			return runtimeName, nil
		}
	}
	if len(seen) == 0 {
		return "", fmt.Errorf("live runtime is required")
	}
	return "", fmt.Errorf("live runtime is ambiguous")
}

func newLiveRuntimeRecord(opts LiveRuntimeOptions, suite Suite, runtimeName Runtime, workDir string, started time.Time) *RunRecord {
	runID := strings.TrimSpace(opts.RunID)
	if runID == "" {
		runID = defaultRunID(suite.ID, started)
	}
	suitePath := strings.TrimSpace(opts.SuitePath)
	if suitePath == "" {
		suitePath = suite.ID
	}
	return &RunRecord{
		SchemaVersion: 1,
		RunID:         runID,
		Suite: SuiteRef{
			ID:         suite.ID,
			Path:       suitePath,
			Visibility: suite.Visibility,
			Tier:       suite.Tier,
			SHA256:     optionalSHA256(opts.SuiteData),
		},
		StartedAt: started,
		Status:    StatusInconclusive,
		Verdict:   VerdictInconclusive,
		Git:       collectGitRecord(workDir),
		Runtime: RuntimeRecord{
			Name:           runtimeName,
			Live:           true,
			Attempts:       effectiveAttempts(suite),
			TimeoutSeconds: effectiveTimeout(suite),
		},
		Environment:     liveEnvironmentRecord(opts, suite),
		Baseline:        &BaselineRecord{Mode: BaselineModeNone},
		CaseResults:     inconclusiveCaseResults(suite),
		AggregateScore:  0,
		DimensionScores: zeroDimensionScores(suite),
	}
}

// effectiveDisableHooks returns true when either the suite declares
// DisableHooks or the runtime caller has set OverrideDisableHooks.
// Used by liveEnvironmentRecord, liveRuntimeEnv, and liveRuntimePrompt
// so the LID baseline-A/B runner can toggle skill loading without
// mutating the loaded suite.
func effectiveDisableHooks(opts LiveRuntimeOptions, suite Suite) bool {
	return suite.Environment.DisableHooks || opts.OverrideDisableHooks
}

func liveEnvironmentRecord(opts LiveRuntimeOptions, suite Suite) EnvironmentRecord {
	return EnvironmentRecord{
		ScrubbedEnvPrefixes: liveScrubPrefixes(suite),
		IsolatedHome:        suite.Environment.IsolateHome,
		IsolatedCodexHome:   suite.Environment.IsolateCodexHome,
		NetworkAccess:       networkAccess(suite),
		HooksDisabled:       effectiveDisableHooks(opts, suite),
	}
}

func liveRuntimeEnv(opts LiveRuntimeOptions, suite Suite) ([]string, []string, error) {
	base := opts.Env
	if base == nil {
		base = os.Environ()
	}
	env := scrubbedEnv(base, liveScrubPrefixes(suite))
	var notes []string
	if suite.Environment.IsolateHome || suite.Environment.IsolateCodexHome {
		root := opts.IsolationRoot
		if root == "" {
			var err error
			root, err = os.MkdirTemp("", "agentops-eval-runtime-*")
			if err != nil {
				return nil, nil, fmt.Errorf("create runtime isolation root: %w", err)
			}
		}
		if suite.Environment.IsolateHome {
			home := filepath.Join(root, "home")
			if err := os.MkdirAll(home, 0o700); err != nil {
				return nil, nil, fmt.Errorf("create isolated home: %w", err)
			}
			env = setEnvValue(env, "HOME", home)
			if goruntime.GOOS == "windows" {
				env = setEnvValue(env, "USERPROFILE", home)
			}
			notes = append(notes, "HOME isolated")
		}
		if suite.Environment.IsolateCodexHome {
			codexHome := filepath.Join(root, "codex-home")
			if err := os.MkdirAll(codexHome, 0o700); err != nil {
				return nil, nil, fmt.Errorf("create isolated Codex home: %w", err)
			}
			env = setEnvValue(env, "CODEX_HOME", codexHome)
			notes = append(notes, "CODEX_HOME isolated")
		}
	}
	notes = append(notes, fmt.Sprintf("scrubbed env prefixes: %s", strings.Join(liveScrubPrefixes(suite), ",")))
	if effectiveDisableHooks(opts, suite) {
		env = append(env, "AGENTOPS_HOOKS_DISABLED=1")
		notes = append(notes, "hooks disabled (AGENTOPS_HOOKS_DISABLED=1)")
	}
	return env, notes, nil
}

func liveScrubPrefixes(suite Suite) []string {
	prefixes := append(scrubPrefixes(suite), "AGENTOPS_RPI_RUNTIME", "CLAUDECODE", "CLAUDE_CODE_")
	sort.Strings(prefixes)
	out := prefixes[:0]
	for _, prefix := range prefixes {
		if prefix == "" {
			continue
		}
		if len(out) == 0 || out[len(out)-1] != prefix {
			out = append(out, prefix)
		}
	}
	return append([]string{}, out...)
}

func setEnvValue(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		out = append(out, entry)
	}
	return append(out, prefix+value)
}

func runLiveRuntimeWithAttempts(ctx context.Context, runner RuntimeRunner, command RuntimeCommand, maxAttempts int) (RuntimeExecutionResult, int, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attemptCtx := ctx
		cancel := func() {}
		if command.TimeoutSeconds > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, time.Duration(command.TimeoutSeconds)*time.Second)
		}
		command.Attempt = attempt
		result, err := runner(attemptCtx, command)
		cancel()
		if err == nil {
			return result, attempt, nil
		}
		lastErr = err
		if errors.Is(attemptCtx.Err(), context.DeadlineExceeded) {
			lastErr = fmt.Errorf("runtime timed out after %ds", command.TimeoutSeconds)
		}
	}
	return RuntimeExecutionResult{}, maxAttempts, lastErr
}

func defaultRuntimeVersionRunner(ctx context.Context, command RuntimeCommand) (string, error) {
	cmd := exec.CommandContext(ctx, command.Executable, command.Args...)
	cmd.Dir = command.Dir
	cmd.Env = command.Env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("probe runtime version: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func defaultRuntimeRunner(ctx context.Context, command RuntimeCommand) (RuntimeExecutionResult, error) {
	cmd := exec.CommandContext(ctx, command.Executable, command.Args...)
	cmd.Dir = command.Dir
	cmd.Env = command.Env
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := RuntimeExecutionResult{
		Status:      StatusInconclusive,
		Verdict:     VerdictInconclusive,
		Diagnostics: compactStrings([]string{stdout.String(), stderr.String()}),
	}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return result, fmt.Errorf("%s exited with code %d: %w", command.Executable, exitErr.ExitCode(), err)
	}
	return result, fmt.Errorf("run %s: %w", command.Executable, err)
}

func applyRuntimeExecutionResult(record *RunRecord, suite Suite, result RuntimeExecutionResult) {
	status := result.Status
	if status == "" {
		status = StatusInconclusive
	}
	verdict := result.Verdict
	if verdict == "" {
		verdict = runtimeVerdict(status)
	}
	record.Status = status
	record.Verdict = verdict
	if result.Version != "" {
		record.Runtime.Version = result.Version
	}
	if result.Model != "" {
		record.Runtime.Model = result.Model
	}
	if result.Profile != "" {
		record.Runtime.Profile = result.Profile
	}
	if len(result.DimensionScores) > 0 {
		record.DimensionScores = result.DimensionScores
	} else {
		record.DimensionScores = zeroDimensionScores(suite)
	}
	record.AggregateScore = roundScore(result.AggregateScore)
	if len(result.CaseResults) > 0 {
		record.CaseResults = result.CaseResults
	} else {
		record.CaseResults = runtimeCaseResults(suite, status, record.AggregateScore, record.DimensionScores, result.Diagnostics, "")
	}
	record.Artifacts = append(record.Artifacts, runtimeArtifacts(result)...)
}

func markRuntimeSkipped(record *RunRecord, suite Suite, reason string) {
	record.Status = StatusSkipped
	record.Verdict = VerdictInconclusive
	record.Runtime.Attempts = 1
	record.Runtime.SkippedReason = reason
	record.CaseResults = runtimeCaseResults(suite, StatusSkipped, 0, zeroDimensionScores(suite), []string{reason}, reason)
	record.AggregateScore = 0
	record.DimensionScores = zeroDimensionScores(suite)
}

func markRuntimeError(record *RunRecord, suite Suite, reason string) {
	record.Status = StatusError
	record.Verdict = VerdictFail
	record.CaseResults = runtimeCaseResults(suite, StatusError, 0, zeroDimensionScores(suite), []string{reason}, reason)
	record.AggregateScore = 0
	record.DimensionScores = zeroDimensionScores(suite)
}

func finishLiveRuntimeRun(opts LiveRuntimeOptions, record *RunRecord, now func() time.Time) (*RunRecord, error) {
	completed := now().UTC()
	record.CompletedAt = &completed
	if opts.OutputPath != "" {
		record.Artifacts = append(record.Artifacts, Artifact{
			Path:    opts.OutputPath,
			Purpose: "evaluation run record",
			Kind:    "run_json",
		})
		if writeErr := WriteRun(opts.OutputPath, record); writeErr != nil {
			return nil, writeErr
		}
	}
	return record, nil
}

func runtimeCaseResults(suite Suite, status Status, score float64, dims map[Dimension]float64, diagnostics []string, failure string) []CaseResult {
	if len(suite.Cases) == 0 {
		return []CaseResult{{
			ID:              "runtime",
			Status:          status,
			Score:           roundScore(score),
			DimensionScores: cloneDimensionScores(dims),
			FailureMessage:  failure,
			Diagnostics:     compactStrings(diagnostics),
		}}
	}
	results := make([]CaseResult, 0, len(suite.Cases))
	for _, evalCase := range suite.Cases {
		results = append(results, CaseResult{
			ID:              evalCase.ID,
			Status:          status,
			Score:           roundScore(score),
			DimensionScores: caseDimensions(suite, evalCase, score),
			Critical:        evalCase.Critical,
			FailureMessage:  failure,
			Diagnostics:     compactStrings(diagnostics),
		})
	}
	return results
}

func inconclusiveCaseResults(suite Suite) []CaseResult {
	return runtimeCaseResults(suite, StatusInconclusive, 0, zeroDimensionScores(suite), nil, "")
}

func zeroDimensionScores(suite Suite) map[Dimension]float64 {
	dims := make(map[Dimension]float64, len(suite.Scoring.Dimensions))
	for _, scoringDim := range suite.Scoring.Dimensions {
		dims[scoringDim.Name] = 0
	}
	if len(dims) == 0 {
		dims[DimensionRuntimeCompatibility] = 0
	}
	return dims
}

func cloneDimensionScores(dims map[Dimension]float64) map[Dimension]float64 {
	out := make(map[Dimension]float64, len(dims))
	for dim, score := range dims {
		out[dim] = score
	}
	return out
}

func runtimeArtifacts(result RuntimeExecutionResult) []Artifact {
	var artifacts []Artifact
	if strings.TrimSpace(result.TranscriptPath) != "" {
		artifacts = append(artifacts, Artifact{
			Path:    result.TranscriptPath,
			Purpose: "runtime transcript",
			Kind:    "transcript",
		})
	}
	if strings.TrimSpace(result.ScorecardPath) != "" {
		artifacts = append(artifacts, Artifact{
			Path:    result.ScorecardPath,
			Purpose: "runtime scorecard",
			Kind:    "scorecard",
		})
	}
	artifacts = append(artifacts, result.Artifacts...)
	return artifacts
}

func runtimeVerdict(status Status) Verdict {
	switch status {
	case StatusPass:
		return VerdictPass
	case StatusFail, StatusError:
		return VerdictFail
	default:
		return VerdictInconclusive
	}
}

func effectiveAttempts(suite Suite) int {
	if suite.Environment.MaxAttempts > 0 {
		return suite.Environment.MaxAttempts
	}
	return 1
}

func liveRuntimePrompt(opts LiveRuntimeOptions, suite Suite) string {
	var parts []string
	for _, evalCase := range suite.Cases {
		if prompt := strings.TrimSpace(inputString(evalCase.Inputs, "prompt")); prompt != "" {
			parts = append(parts, prompt)
		}
	}
	var prompt string
	switch {
	case len(parts) > 0:
		prompt = strings.Join(parts, "\n\n")
	case strings.TrimSpace(suite.Description) != "":
		prompt = strings.TrimSpace(suite.Description)
	default:
		prompt = strings.TrimSpace(suite.Name)
	}
	if effectiveDisableHooks(opts, suite) {
		prompt += "\n\nConstraint: Do NOT load additional skills or plugins. Work only with base agent capabilities."
	}
	return prompt
}

func runtimeMetadataFromCommand(command string) (string, string) {
	_, args := rpilib.SplitRuntimeCommand(command)
	var model string
	var profile string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--model" && i+1 < len(args):
			model = args[i+1]
			i++
		case strings.HasPrefix(arg, "--model="):
			model = strings.TrimPrefix(arg, "--model=")
		case arg == "--profile" && i+1 < len(args):
			profile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--profile="):
			profile = strings.TrimPrefix(arg, "--profile=")
		}
	}
	return model, profile
}

func optionalSHA256(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return sha256Hex(data)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
