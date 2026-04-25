package eval

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const defaultTimeoutSeconds = 30

func RunSuite(opts RunOptions) (*RunRecord, error) {
	if strings.TrimSpace(opts.SuitePath) == "" {
		return nil, fmt.Errorf("suite path is required")
	}
	suite, suiteData, err := LoadSuite(opts.SuitePath)
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
	runID := strings.TrimSpace(opts.RunID)
	if runID == "" {
		runID = defaultRunID(suite.ID, started)
	}
	runtimeName := opts.Runtime
	if runtimeName == "" {
		runtimeName = inferRuntime(*suite)
	}
	if !validDeterministicRuntime(runtimeName) {
		return nil, fmt.Errorf("runtime %q is out of deterministic scope", runtimeName)
	}

	suiteDir := filepath.Dir(opts.SuitePath)
	caseResults := make([]CaseResult, 0, len(suite.Cases))
	for _, evalCase := range suite.Cases {
		caseResults = append(caseResults, runCase(*suite, suiteDir, evalCase))
	}
	aggregate, dimensions := scoreRun(*suite, caseResults)
	status := runStatus(*suite, caseResults, aggregate, dimensions)
	verdict := VerdictPass
	if status != StatusPass {
		verdict = VerdictFail
	}
	completed := now().UTC()
	record := &RunRecord{
		SchemaVersion: 1,
		RunID:         runID,
		Suite: SuiteRef{
			ID:         suite.ID,
			Path:       opts.SuitePath,
			Visibility: suite.Visibility,
			Tier:       suite.Tier,
			SHA256:     sha256Hex(suiteData),
		},
		StartedAt:       started,
		CompletedAt:     &completed,
		Status:          status,
		Verdict:         verdict,
		Git:             collectGitRecord(workDir),
		Runtime:         runtimeRecord(runtimeName, *suite),
		Environment:     environmentRecord(*suite),
		Baseline:        &BaselineRecord{Mode: BaselineModeNone},
		CaseResults:     caseResults,
		AggregateScore:  aggregate,
		DimensionScores: dimensions,
	}
	if opts.BaselinePath != "" {
		baseline, err := LoadRun(opts.BaselinePath)
		if err != nil {
			return nil, err
		}
		record, err = CompareRuns(record, baseline, compareOptionsFromSuite(*suite))
		if err != nil {
			return nil, err
		}
		record.Baseline.BaselinePath = opts.BaselinePath
	}
	if opts.OutputPath != "" {
		record.Artifacts = append(record.Artifacts, Artifact{
			Path:    opts.OutputPath,
			Purpose: "evaluation run record",
			Kind:    "run_json",
		})
		if err := WriteRun(opts.OutputPath, record); err != nil {
			return nil, err
		}
	}
	return record, nil
}

func runCase(suite Suite, suiteDir string, evalCase Case) CaseResult {
	start := time.Now()
	ctx := caseContext{
		suite:    &suite,
		suiteDir: suiteDir,
		evalCase: evalCase,
		exitCode: 0,
	}
	if evalCase.Kind == "command" {
		output, err := executeCaseCommand(suite, suiteDir, evalCase)
		ctx.stdout = output.stdout
		ctx.stderr = output.stderr
		ctx.exitCode = output.exitCode
		if err != nil && output.infrastructureError {
			return CaseResult{
				ID:              evalCase.ID,
				Status:          StatusError,
				Score:           0,
				DimensionScores: caseDimensions(suite, evalCase, 0),
				DurationMS:      time.Since(start).Milliseconds(),
				Critical:        evalCase.Critical,
				FailureMessage:  err.Error(),
				Diagnostics:     []string{err.Error()},
			}
		}
	}

	requiredTotal := 0
	requiredPassed := 0
	var diagnostics []string
	var failures []string
	for _, expectation := range evalCase.Expectations {
		result := evaluateExpectation(expectation, ctx)
		required := expectationRequired(expectation)
		if required {
			requiredTotal++
			if result.passed {
				requiredPassed++
			} else {
				failures = append(failures, result.message)
			}
		}
		if !result.passed || result.message != "" {
			diagnostics = append(diagnostics, result.message)
		}
	}
	score := 1.0
	if requiredTotal > 0 {
		score = roundScore(float64(requiredPassed) / float64(requiredTotal))
	}
	status := StatusPass
	if requiredPassed != requiredTotal {
		status = StatusFail
	}
	return CaseResult{
		ID:              evalCase.ID,
		Status:          status,
		Score:           score,
		DimensionScores: caseDimensions(suite, evalCase, score),
		DurationMS:      time.Since(start).Milliseconds(),
		Critical:        evalCase.Critical,
		FailureMessage:  strings.Join(failures, "; "),
		Diagnostics:     compactStrings(diagnostics),
	}
}

type commandOutput struct {
	stdout              string
	stderr              string
	exitCode            int
	infrastructureError bool
}

func executeCaseCommand(suite Suite, suiteDir string, evalCase Case) (commandOutput, error) {
	spec, err := commandSpecFromInputs(evalCase.Inputs)
	if err != nil {
		return commandOutput{exitCode: -1, infrastructureError: true}, err
	}
	timeout := evalCase.TimeoutSeconds
	if timeout == 0 {
		timeout = suite.Environment.TimeoutSeconds
	}
	if timeout == 0 {
		timeout = defaultTimeoutSeconds
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	name := spec.name
	args := spec.args
	if spec.shell != "" {
		name, args = shellCommand(spec.shell)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if spec.cwd != "" {
		cmd.Dir = resolvePath(suiteDir, spec.cwd)
	} else {
		cmd.Dir = suiteDir
	}
	cmd.Env = applyCommandEnv(scrubbedEnv(os.Environ(), scrubPrefixes(suite)), spec.env)
	if spec.stdin != "" {
		cmd.Stdin = strings.NewReader(spec.stdin)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	output := commandOutput{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: 0,
	}
	if ctx.Err() == context.DeadlineExceeded {
		output.exitCode = -1
		output.infrastructureError = true
		return output, fmt.Errorf("command timed out after %ds", timeout)
	}
	if err == nil {
		return output, nil
	}
	var exitErr *exec.ExitError
	if ok := errors.As(err, &exitErr); ok {
		output.exitCode = exitErr.ExitCode()
		return output, nil
	}
	output.exitCode = -1
	output.infrastructureError = true
	return output, fmt.Errorf("run command %q: %w", name, err)
}

type commandSpec struct {
	name  string
	args  []string
	shell string
	cwd   string
	stdin string
	env   map[string]string
}

func commandSpecFromInputs(inputs map[string]any) (commandSpec, error) {
	if len(inputs) == 0 {
		return commandSpec{}, fmt.Errorf("command case requires inputs.argv, inputs.command, or inputs.shell")
	}
	spec := commandSpec{
		cwd:   inputString(inputs, "cwd"),
		stdin: inputString(inputs, "stdin"),
		env:   inputStringMap(inputs, "env"),
		shell: inputString(inputs, "shell"),
	}
	if spec.shell != "" {
		return spec, nil
	}
	argv := inputStringSlice(inputs, "argv")
	if len(argv) > 0 {
		spec.name = argv[0]
		spec.args = argv[1:]
		return spec, nil
	}
	spec.name = inputString(inputs, "command")
	spec.args = inputStringSlice(inputs, "args")
	if spec.name == "" {
		return commandSpec{}, fmt.Errorf("command case requires non-empty command")
	}
	return spec, nil
}

func shellCommand(script string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/C", script}
	}
	return "sh", []string{"-c", script}
}

func scoreRun(suite Suite, results []CaseResult) (float64, map[Dimension]float64) {
	dimensions := make(map[Dimension]float64, len(suite.Scoring.Dimensions))
	for _, scoringDim := range suite.Scoring.Dimensions {
		var sum float64
		var count int
		for _, result := range results {
			if score, ok := result.DimensionScores[scoringDim.Name]; ok {
				sum += score
				count++
			}
		}
		if count == 0 {
			dimensions[scoringDim.Name] = 0
		} else {
			dimensions[scoringDim.Name] = roundScore(sum / float64(count))
		}
	}
	var weightedSum float64
	var weightTotal float64
	for _, scoringDim := range suite.Scoring.Dimensions {
		weightedSum += dimensions[scoringDim.Name] * scoringDim.Weight
		weightTotal += scoringDim.Weight
	}
	if weightTotal == 0 {
		return 0, dimensions
	}
	return roundScore(weightedSum / weightTotal), dimensions
}

func runStatus(suite Suite, results []CaseResult, aggregate float64, dimensions map[Dimension]float64) Status {
	hasError := false
	hasCriticalFailure := false
	for _, result := range results {
		if result.Status == StatusError {
			hasError = true
		}
		if result.Critical && result.Status != StatusPass {
			hasCriticalFailure = true
		}
	}
	if hasError {
		return StatusError
	}
	if hasCriticalFailure || aggregate < suite.Scoring.AggregateThreshold {
		return StatusFail
	}
	for _, scoringDim := range suite.Scoring.Dimensions {
		if dimensions[scoringDim.Name] < scoringDim.Threshold {
			return StatusFail
		}
	}
	return StatusPass
}

func caseDimensions(suite Suite, evalCase Case, score float64) map[Dimension]float64 {
	dims := evalCase.Dimensions
	if len(dims) == 0 {
		dims = make([]Dimension, 0, len(suite.Scoring.Dimensions))
		for _, scoringDim := range suite.Scoring.Dimensions {
			dims = append(dims, scoringDim.Name)
		}
	}
	if len(dims) == 0 {
		dims = []Dimension{DimensionCorrectness}
	}
	out := make(map[Dimension]float64, len(dims))
	for _, dim := range dims {
		out[dim] = roundScore(score)
	}
	return out
}

func runtimeRecord(name Runtime, suite Suite) RuntimeRecord {
	return RuntimeRecord{
		Name:           name,
		Live:           false,
		Attempts:       1,
		TimeoutSeconds: effectiveTimeout(suite),
	}
}

func environmentRecord(suite Suite) EnvironmentRecord {
	return EnvironmentRecord{
		ScrubbedEnvPrefixes: scrubPrefixes(suite),
		IsolatedHome:        suite.Environment.IsolateHome,
		IsolatedCodexHome:   suite.Environment.IsolateCodexHome,
		NetworkAccess:       networkAccess(suite),
	}
}

func collectGitRecord(workDir string) GitRecord {
	record := GitRecord{
		CandidateRef: "unknown",
		CandidateSHA: "0000000",
		Dirty:        false,
	}
	if ref := gitOutput(workDir, "rev-parse", "--abbrev-ref", "HEAD"); ref != "" {
		record.CandidateRef = ref
	}
	if sha := gitOutput(workDir, "rev-parse", "--short=12", "HEAD"); sha != "" {
		record.CandidateSHA = sha
	}
	status := gitOutput(workDir, "status", "--porcelain")
	if status == "" {
		return record
	}
	for _, line := range strings.Split(status, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		record.Dirty = true
		if len(line) > 3 {
			record.DirtyPaths = append(record.DirtyPaths, strings.TrimSpace(line[3:]))
		} else {
			record.DirtyPaths = append(record.DirtyPaths, line)
		}
	}
	sort.Strings(record.DirtyPaths)
	return record
}

func gitOutput(workDir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func inferRuntime(suite Suite) Runtime {
	for _, evalCase := range suite.Cases {
		if evalCase.Runtime == RuntimeShell || evalCase.Kind == "command" {
			return RuntimeShell
		}
		if evalCase.Runtime == RuntimeMock {
			return RuntimeMock
		}
	}
	return RuntimeStatic
}

func effectiveTimeout(suite Suite) int {
	if suite.Environment.TimeoutSeconds > 0 {
		return suite.Environment.TimeoutSeconds
	}
	return defaultTimeoutSeconds
}

func scrubPrefixes(suite Suite) []string {
	prefixes := append([]string{}, suite.Environment.ScrubEnvPrefixes...)
	if len(prefixes) == 0 {
		prefixes = append(prefixes, "AGENTOPS_RPI_RUNTIME")
	}
	sort.Strings(prefixes)
	return prefixes
}

func networkAccess(suite Suite) NetworkAccess {
	if suite.Environment.OfflineRequired || suite.Environment.Network == "forbidden" || suite.Tier == TierDeterministic {
		return NetworkDisabled
	}
	if suite.Environment.Network == "allowed" || suite.Environment.Network == "required" {
		return NetworkEnabled
	}
	return NetworkUnknown
}

func scrubbedEnv(env []string, prefixes []string) []string {
	var out []string
	for _, entry := range env {
		skip := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(entry, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, entry)
		}
	}
	return out
}

func applyCommandEnv(env []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return env
	}
	filtered := make([]string, 0, len(env)+len(overrides))
	for _, entry := range env {
		key := entry
		if idx := strings.IndexByte(entry, '='); idx >= 0 {
			key = entry[:idx]
		}
		if _, ok := overrides[key]; ok {
			continue
		}
		filtered = append(filtered, entry)
	}
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		filtered = append(filtered, key+"="+overrides[key])
	}
	return filtered
}

func inputString(inputs map[string]any, key string) string {
	if raw, ok := inputs[key]; ok {
		if value, ok := raw.(string); ok {
			return value
		}
	}
	return ""
}

func inputStringSlice(inputs map[string]any, key string) []string {
	raw, ok := inputs[key]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case []string:
		return append([]string(nil), value...)
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func inputStringMap(inputs map[string]any, key string) map[string]string {
	raw, ok := inputs[key]
	if !ok {
		return nil
	}
	out := map[string]string{}
	switch value := raw.(type) {
	case map[string]string:
		for k, v := range value {
			out[k] = v
		}
	case map[string]any:
		for k, v := range value {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

func resolvePath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}

func defaultRunID(suiteID string, started time.Time) string {
	return sanitizeRunID(fmt.Sprintf("eval-%s-%s", started.Format("20060102T150405Z"), suiteID))
}

func sanitizeRunID(value string) string {
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == ':' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-_.:")
	if out == "" {
		return "eval-run"
	}
	return out
}

func roundScore(value float64) float64 {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	return math.Round(value*10000) / 10000
}

func roundDelta(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func compactStrings(values []string) []string {
	out := values[:0]
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func compareOptionsFromSuite(suite Suite) CompareOptions {
	opts := CompareOptions{}
	if suite.BaselinePolicy.MaxAggregateRegression != nil {
		opts.MaxAggregateRegression = *suite.BaselinePolicy.MaxAggregateRegression
	}
	if suite.BaselinePolicy.MaxDimensionRegression != nil {
		opts.MaxDimensionRegression = *suite.BaselinePolicy.MaxDimensionRegression
	}
	return opts
}
