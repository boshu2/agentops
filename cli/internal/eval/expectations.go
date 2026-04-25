package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
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
		want, ok := intValue(exp.Value)
		if !ok {
			want = 0
		}
		if ctx.exitCode == want {
			return passf("exit_code matched %d", want)
		}
		return failf("exit_code = %d, want %d", ctx.exitCode, want)
	case "stdout_contains":
		needle := stringExpectationValue(exp)
		if strings.Contains(ctx.stdout, needle) {
			return passf("stdout contains %q", needle)
		}
		return failf("stdout does not contain %q", needle)
	case "stderr_contains":
		needle := stringExpectationValue(exp)
		if strings.Contains(ctx.stderr, needle) {
			return passf("stderr contains %q", needle)
		}
		return failf("stderr does not contain %q", needle)
	case "file_exists":
		path := resolvePath(ctx.suiteDir, exp.Target)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return passf("file exists: %s", exp.Target)
		}
		return failf("file does not exist: %s", exp.Target)
	case "file_absent":
		path := resolvePath(ctx.suiteDir, exp.Target)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return passf("file absent: %s", exp.Target)
		}
		return failf("file exists: %s", exp.Target)
	case "artifact_contains":
		path := resolvePath(ctx.suiteDir, exp.Target)
		data, err := os.ReadFile(path)
		if err != nil {
			return failf("read artifact %s: %v", exp.Target, err)
		}
		needle := stringExpectationValue(exp)
		if strings.Contains(string(data), needle) {
			return passf("artifact %s contains %q", exp.Target, needle)
		}
		return failf("artifact %s does not contain %q", exp.Target, needle)
	case "json_path":
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
	case "schema_valid":
		if err := validateSchemaTarget(exp, ctx); err != nil {
			return failf("schema_valid %s: %v", exp.Target, err)
		}
		return passf("schema_valid %s matched", exp.Target)
	case "score_at_least":
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
		threshold := 1.0
		if exp.Threshold != nil {
			threshold = *exp.Threshold
		} else if exp.Value != nil {
			if parsed, ok := floatValue(exp.Value); ok {
				threshold = parsed
			}
		}
		if actualScore >= threshold {
			return passf("score %.4f >= %.4f", actualScore, threshold)
		}
		return failf("score %.4f < %.4f", actualScore, threshold)
	case "manual_review":
		return failf("manual_review is not supported by deterministic evals")
	default:
		return failf("unsupported expectation type %q", exp.Type)
	}
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
