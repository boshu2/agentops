package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type caseContext struct {
	suite    *Suite
	suiteDir string
	evalCase Case
	stdout   string
	stderr   string
	exitCode int
}

type expectationResult struct {
	passed  bool
	message string
}

func evaluateExpectation(exp Expectation, ctx caseContext) expectationResult {
	switch exp.Type {
	case "exit_code":
		return evaluateExitCode(exp, ctx)
	case "stdout_contains":
		return evaluateStringContains("stdout", ctx.stdout, stringExpectationValue(exp))
	case "stderr_contains":
		return evaluateStringContains("stderr", ctx.stderr, stringExpectationValue(exp))
	case "stdout_contains_auto_detect":
		return evaluateStdoutContainsAutoDetect(exp, ctx)
	case "file_exists":
		return evaluateFileExists(exp, ctx)
	case "file_absent":
		return evaluateFileAbsent(exp, ctx)
	case "artifact_contains":
		return evaluateArtifactContains(exp, ctx)
	case "json_path":
		return evaluateJSONPath(exp, ctx)
	case "schema_valid":
		if err := validateSchemaTarget(exp, ctx); err != nil {
			return failf("schema_valid %s: %v", exp.Target, err)
		}
		return passf("schema_valid %s matched", exp.Target)
	case "score_at_least":
		return evaluateScoreAtLeast(exp, ctx)
	case "manual_review":
		return failf("manual_review is not supported by deterministic evals")
	default:
		return failf("unsupported expectation type %q", exp.Type)
	}
}

func evaluateExitCode(exp Expectation, ctx caseContext) expectationResult {
	want, ok := intValue(exp.Value)
	if !ok {
		want = 0
	}
	if ctx.exitCode == want {
		return passf("exit_code matched %d", want)
	}
	return failf("exit_code = %d, want %d", ctx.exitCode, want)
}

func evaluateStringContains(name, haystack, needle string) expectationResult {
	if strings.Contains(haystack, needle) {
		return passf("%s contains %q", name, needle)
	}
	return failf("%s does not contain %q", name, needle)
}

func evaluateFileExists(exp Expectation, ctx caseContext) expectationResult {
	path := resolvePath(ctx.suiteDir, exp.Target)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return passf("file exists: %s", exp.Target)
	}
	return failf("file does not exist: %s", exp.Target)
}

func evaluateFileAbsent(exp Expectation, ctx caseContext) expectationResult {
	path := resolvePath(ctx.suiteDir, exp.Target)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return passf("file absent: %s", exp.Target)
	}
	return failf("file exists: %s", exp.Target)
}

func evaluateArtifactContains(exp Expectation, ctx caseContext) expectationResult {
	path := resolvePath(ctx.suiteDir, exp.Target)
	data, err := os.ReadFile(path)
	if err != nil {
		return failf("read artifact %s: %v", exp.Target, err)
	}
	return evaluateStringContains("artifact "+exp.Target, string(data), stringExpectationValue(exp))
}

func evaluateJSONPath(exp Expectation, ctx caseContext) expectationResult {
	actual, ok, err := jsonPathValue(exp.Target, ctx)
	if err != nil {
		return failf("json_path %s: %v", exp.Target, err)
	}
	if !ok {
		return failf("json_path %s not found", exp.Target)
	}
	if exp.Value == nil {
		return passf("json_path %s exists", exp.Target)
	}
	if jsonValuesEqual(actual, exp.Value) {
		return passf("json_path %s matched", exp.Target)
	}
	return failf("json_path %s = %v, want %v", exp.Target, actual, exp.Value)
}

func evaluateScoreAtLeast(exp Expectation, ctx caseContext) expectationResult {
	actual, ok, err := jsonPathValue(exp.Target, ctx)
	if err != nil {
		return failf("score_at_least %s: %v", exp.Target, err)
	}
	if !ok {
		return failf("score_at_least target %s not found", exp.Target)
	}
	actualScore, ok := floatValue(actual)
	if !ok {
		return failf("score_at_least target %s is not numeric", exp.Target)
	}
	threshold := scoreThreshold(exp)
	if actualScore >= threshold {
		return passf("score %.4f >= %.4f", actualScore, threshold)
	}
	return failf("score %.4f < %.4f", actualScore, threshold)
}

func scoreThreshold(exp Expectation) float64 {
	if exp.Threshold != nil {
		return *exp.Threshold
	}
	if exp.Value != nil {
		if parsed, ok := floatValue(exp.Value); ok {
			return parsed
		}
	}
	return 1.0
}

func expectationRequired(exp Expectation) bool {
	if exp.Required == nil {
		return true
	}
	return *exp.Required
}

func passf(format string, args ...any) expectationResult {
	return expectationResult{passed: true, message: fmt.Sprintf(format, args...)}
}

func failf(format string, args ...any) expectationResult {
	return expectationResult{passed: false, message: fmt.Sprintf(format, args...)}
}

func stringExpectationValue(exp Expectation) string {
	if exp.Value != nil {
		if value, ok := exp.Value.(string); ok {
			return value
		}
		return fmt.Sprint(exp.Value)
	}
	return exp.Target
}

func intValue(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		i, err := strconv.Atoi(v)
		return i, err == nil
	default:
		return 0, false
	}
}

func floatValue(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func jsonPathValue(target string, ctx caseContext) (any, bool, error) {
	source, path := splitJSONTarget(target)
	var data []byte
	switch source {
	case "stdout":
		data = []byte(ctx.stdout)
	case "stderr":
		data = []byte(ctx.stderr)
	default:
		filePath := resolvePath(ctx.suiteDir, source)
		raw, err := os.ReadFile(filePath)
		if err != nil {
			return nil, false, err
		}
		data = raw
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, false, err
	}
	if path == "" || path == "$" {
		return root, true, nil
	}
	return walkJSONPath(root, path)
}

func splitJSONTarget(target string) (string, string) {
	target = strings.TrimSpace(target)
	for _, source := range []string{"stdout", "stderr"} {
		if target == source {
			return source, ""
		}
		if strings.HasPrefix(target, source+".") {
			return source, strings.TrimPrefix(target, source+".")
		}
		if strings.HasPrefix(target, source+":") {
			return source, strings.TrimPrefix(target, source+":")
		}
	}
	if idx := strings.Index(target, ":"); idx > 0 && !looksLikeWindowsDrive(target, idx) {
		return target[:idx], target[idx+1:]
	}
	return target, ""
}

func looksLikeWindowsDrive(path string, colon int) bool {
	return colon == 1 && len(path) > 2 && filepath.VolumeName(path[:2]) != ""
}

func walkJSONPath(root any, path string) (any, bool, error) {
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return root, true, nil
	}
	current := root
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}
		value, ok, err := walkJSONSegment(current, part)
		if err != nil || !ok {
			return nil, ok, err
		}
		current = value
	}
	return current, true, nil
}

func walkJSONSegment(current any, segment string) (any, bool, error) {
	name := segment
	indexes := []int{}
	for {
		open := strings.Index(name, "[")
		if open < 0 {
			break
		}
		closeIdx := strings.Index(name[open:], "]")
		if closeIdx < 0 {
			return nil, false, fmt.Errorf("invalid array path segment %q", segment)
		}
		idxText := name[open+1 : open+closeIdx]
		idx, err := strconv.Atoi(idxText)
		if err != nil {
			return nil, false, fmt.Errorf("invalid array index %q", idxText)
		}
		indexes = append(indexes, idx)
		name = name[:open] + name[open+closeIdx+1:]
	}
	value := current
	if name != "" {
		obj, ok := value.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		value, ok = obj[name]
		if !ok {
			return nil, false, nil
		}
	}
	for _, idx := range indexes {
		arr, ok := value.([]any)
		if !ok || idx < 0 || idx >= len(arr) {
			return nil, false, nil
		}
		value = arr[idx]
	}
	return value, true, nil
}

func jsonValuesEqual(actual, expected any) bool {
	if reflect.DeepEqual(actual, expected) {
		return true
	}
	actualFloat, actualOK := floatValue(actual)
	expectedFloat, expectedOK := floatValue(expected)
	if actualOK && expectedOK {
		return actualFloat == expectedFloat
	}
	return fmt.Sprint(actual) == fmt.Sprint(expected)
}

func validateSchemaTarget(exp Expectation, ctx caseContext) error {
	schema := ""
	if exp.Value != nil {
		schema = strings.ToLower(fmt.Sprint(exp.Value))
	}
	target := strings.TrimSpace(exp.Target)
	if target == "" || target == "suite" {
		if strings.Contains(schema, "run") {
			return fmt.Errorf("suite target cannot validate eval-run schema")
		}
		return ValidateSuite(ctx.suite)
	}
	var data []byte
	switch target {
	case "stdout":
		data = []byte(ctx.stdout)
	case "stderr":
		data = []byte(ctx.stderr)
	default:
		raw, err := os.ReadFile(resolvePath(ctx.suiteDir, target))
		if err != nil {
			return err
		}
		data = raw
	}
	if strings.Contains(schema, "eval-run") || strings.Contains(schema, "run") {
		var run RunRecord
		if err := decodeStrict(data, &run); err != nil {
			return err
		}
		return ValidateRun(&run)
	}
	if strings.Contains(schema, "eval-suite") || strings.Contains(schema, "suite") {
		var suite Suite
		if err := decodeStrict(data, &suite); err != nil {
			return err
		}
		return ValidateSuite(&suite)
	}
	var probe map[string]any
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	if _, ok := probe["run_id"]; ok {
		var run RunRecord
		if err := decodeStrict(data, &run); err != nil {
			return err
		}
		return ValidateRun(&run)
	}
	var suite Suite
	if err := decodeStrict(data, &suite); err != nil {
		return err
	}
	return ValidateSuite(&suite)
}

// autoDetectSpec describes a stdout_contains_auto_detect expectation payload.
// The expectation re-executes command, parses pattern against its combined
// stdout+stderr, captures expected_group, and verifies the case stdout
// contains the full match. tolerance_pct (default 0) allows numeric drift
// between the freshly-detected capture group and the value embedded in the
// case stdout when both are numeric. Strict-by-default: drift fails loud.
type autoDetectSpec struct {
	command       string
	pattern       string
	expectedGroup int
	tolerancePct  float64
}

func parseAutoDetectSpec(exp Expectation) (autoDetectSpec, error) {
	spec := autoDetectSpec{expectedGroup: 1, tolerancePct: 0}
	raw, ok := exp.Value.(map[string]any)
	if !ok {
		return spec, fmt.Errorf("value must be an object with command/pattern fields")
	}
	if cmd, ok := raw["command"].(string); ok {
		spec.command = strings.TrimSpace(cmd)
	}
	if spec.command == "" {
		return spec, fmt.Errorf("command is required")
	}
	if pat, ok := raw["pattern"].(string); ok {
		spec.pattern = pat
	}
	if spec.pattern == "" {
		return spec, fmt.Errorf("pattern is required")
	}
	if grp, ok := raw["expected_group"]; ok {
		if i, ok := intValue(grp); ok {
			spec.expectedGroup = i
		}
	}
	if tol, ok := raw["tolerance_pct"]; ok {
		if f, ok := floatValue(tol); ok {
			spec.tolerancePct = f
		}
	}
	if spec.expectedGroup < 0 {
		return spec, fmt.Errorf("expected_group must be >= 0")
	}
	if spec.tolerancePct < 0 {
		return spec, fmt.Errorf("tolerance_pct must be >= 0")
	}
	return spec, nil
}

func evaluateStdoutContainsAutoDetect(exp Expectation, ctx caseContext) expectationResult {
	spec, err := parseAutoDetectSpec(exp)
	if err != nil {
		return failf("stdout_contains_auto_detect: %v", err)
	}
	re, err := regexp.Compile(spec.pattern)
	if err != nil {
		return failf("stdout_contains_auto_detect: compile pattern %q: %v", spec.pattern, err)
	}
	if err := autoDetectPrecheck(spec.command); err != nil {
		return failf("stdout_contains_auto_detect: %v", err)
	}
	output, runErr := runAutoDetectCommand(ctx, spec.command)
	if runErr != nil {
		return failf("stdout_contains_auto_detect: re-run %q: %v", spec.command, runErr)
	}
	matches := re.FindStringSubmatch(output)
	if matches == nil {
		return failf("stdout_contains_auto_detect: pattern %q did not match auto-detect output (command=%q)", spec.pattern, spec.command)
	}
	if spec.expectedGroup >= len(matches) {
		return failf("stdout_contains_auto_detect: expected_group %d out of range (regex has %d groups)", spec.expectedGroup, len(matches)-1)
	}
	fullMatch := matches[0]
	captured := matches[spec.expectedGroup]
	if !strings.Contains(ctx.stdout, fullMatch) {
		// Tolerance gate: if the haystack contains a different number under
		// the same surrounding pattern, allow within tolerance_pct. Otherwise
		// fail loud with a one-line bump prompt naming the new value.
		if spec.tolerancePct > 0 {
			if accepted, msg := autoDetectTolerated(ctx.stdout, re, spec, captured); accepted {
				return passf("%s", msg)
			}
		}
		return failf(
			"stdout_contains_auto_detect: case stdout missing detected match %q (auto-detect: command=%q pattern=%q captured=%q). BUMP the pinned value to %q.",
			fullMatch, spec.command, spec.pattern, captured, fullMatch,
		)
	}
	return passf("stdout_contains_auto_detect matched %q (auto-detected from %q)", fullMatch, spec.command)
}

// autoDetectPrecheck verifies the command's first token resolves on PATH so
// missing binaries (e.g. bats on CI) fail loud with an explicit message
// instead of being silently treated as a regex miss.
func autoDetectPrecheck(command string) error {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return fmt.Errorf("command is empty")
	}
	binary := fields[0]
	if strings.ContainsAny(binary, "/\\") {
		// Path-qualified — let exec surface its own error if missing.
		return nil
	}
	if _, err := exec.LookPath(binary); err != nil {
		return fmt.Errorf("required binary %q not found on PATH (auto-detect cannot run): %w", binary, err)
	}
	return nil
}

func runAutoDetectCommand(ctx caseContext, command string) (string, error) {
	timeout := defaultTimeoutSeconds
	if ctx.evalCase.TimeoutSeconds > 0 {
		timeout = ctx.evalCase.TimeoutSeconds
	} else if ctx.suite != nil && ctx.suite.Environment.TimeoutSeconds > 0 {
		timeout = ctx.suite.Environment.TimeoutSeconds
	}
	cctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	name, args := shellCommand(command)
	cmd := exec.CommandContext(cctx, name, args...)
	// Match runCase semantics: derive cwd from inputs.cwd if present, else
	// the suite directory. Auto-detect must observe the same working tree
	// as the case it ratchets, otherwise the captured value drifts spuriously.
	if ctx.evalCase.Inputs != nil {
		if cwd, ok := ctx.evalCase.Inputs["cwd"].(string); ok && cwd != "" {
			cmd.Dir = resolvePath(ctx.suiteDir, cwd)
		}
	}
	if cmd.Dir == "" {
		cmd.Dir = ctx.suiteDir
	}
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	runErr := cmd.Run()
	if cctx.Err() == context.DeadlineExceeded {
		return combined.String(), fmt.Errorf("auto-detect command timed out after %ds", timeout)
	}
	// runErr is informational — many of these commands legitimately exit
	// non-zero (e.g. an unrelated failing test) yet still emit the line we
	// need to capture. The regex match is the authoritative signal.
	_ = runErr
	if runtime.GOOS == "windows" && combined.Len() == 0 {
		return "", fmt.Errorf("auto-detect command produced no output (windows)")
	}
	return combined.String(), nil
}

func autoDetectTolerated(haystack string, re *regexp.Regexp, spec autoDetectSpec, fresh string) (bool, string) {
	haystackMatches := re.FindStringSubmatch(haystack)
	if haystackMatches == nil || spec.expectedGroup >= len(haystackMatches) {
		return false, ""
	}
	pinned := haystackMatches[spec.expectedGroup]
	freshNum, freshOK := floatValue(fresh)
	pinnedNum, pinnedOK := floatValue(pinned)
	if !freshOK || !pinnedOK {
		return false, ""
	}
	if pinnedNum == 0 {
		return freshNum == 0, "stdout_contains_auto_detect within tolerance (zero baseline)"
	}
	driftPct := (freshNum - pinnedNum) / pinnedNum * 100
	if driftPct < 0 {
		driftPct = -driftPct
	}
	if driftPct <= spec.tolerancePct {
		return true, fmt.Sprintf(
			"stdout_contains_auto_detect within tolerance: pinned=%s fresh=%s drift=%.2f%% <= %.2f%%",
			pinned, fresh, driftPct, spec.tolerancePct,
		)
	}
	return false, ""
}
