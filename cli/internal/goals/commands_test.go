package goals

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeGoalsMD writes md to a GOALS.md inside a fresh temp dir and returns the path.
func writeGoalsMD(t *testing.T, md string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "GOALS.md")
	if err := os.WriteFile(path, []byte(md), 0o600); err != nil {
		t.Fatalf("write goals file: %v", err)
	}
	return path
}

// runValidateJSON runs RunValidate in JSON mode and decodes the report.
// The returned error is RunValidate's own return value, which is non-nil
// exactly when the report is invalid.
func runValidateJSON(t *testing.T, path string) (ValidateResult, error) {
	t.Helper()
	var buf bytes.Buffer
	runErr := RunValidate(ValidateOptions{GoalsFile: path, JSON: true, Stdout: &buf})
	var got ValidateResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode validate JSON: %v (raw %q)", err, buf.String())
	}
	return got, runErr
}

// errorsMentioning reports whether any error string contains substr.
func errorsMentioning(errs []string, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e, substr) {
			return true
		}
	}
	return false
}

// TestRunValidate_ZeroGatesIsInvalid pins the zero-denominator guard.
//
// The parser legitimately returns zero goals for a GOALS.md with no "## Gates"
// section (see TestExtra_ParseMarkdownGoals_NoGatesSection — that parser
// behavior is correct and unchanged). The bug was at the validity layer: zero
// parsed goals produced no errors, so Valid flipped true and every consumer —
// `ao goals validate --json`, the release smoke test, the nightly fitness
// surface — reported green while measuring 0/0.
func TestRunValidate_ZeroGatesIsInvalid(t *testing.T) {
	path := writeGoalsMD(t, `# Goals

Mission line.

## Fitness properties

1. **Behavior before activity.** Prose only, no measurable gate.
`)

	got, runErr := runValidateJSON(t, path)

	if runErr == nil {
		t.Error("RunValidate returned nil error for a goals file with zero gates")
	}
	if got.Valid {
		t.Error("Valid = true for a goals file with zero gates, want false")
	}
	if got.GoalCount != 0 {
		t.Errorf("GoalCount = %d, want 0", got.GoalCount)
	}
	if !errorsMentioning(got.Errors, `"## Gates"`) {
		t.Errorf("errors %q do not name the missing \"## Gates\" section", got.Errors)
	}
	if !errorsMentioning(got.Errors, "0/0") {
		t.Errorf("errors %q do not name the zero-denominator consequence", got.Errors)
	}
}

// TestRunValidate_NonEmptyGatesTableIsValid is the GREEN half of the guard:
// one well-formed gate row is a nonzero denominator and must validate.
func TestRunValidate_NonEmptyGatesTableIsValid(t *testing.T) {
	path := writeGoalsMD(t, "# Goals\n\nMission line.\n\n"+
		"## Gates\n\n"+
		"| ID | Check | Weight | Description |\n"+
		"|----|-------|--------|-------------|\n"+
		"| example-gate | `bash -c true` | 5 | An executable check. |\n")

	got, runErr := runValidateJSON(t, path)

	if runErr != nil {
		t.Errorf("RunValidate error = %v, want nil", runErr)
	}
	if !got.Valid {
		t.Errorf("Valid = false, want true (errors: %q)", got.Errors)
	}
	if got.GoalCount != 1 {
		t.Errorf("GoalCount = %d, want 1", got.GoalCount)
	}
	if errorsMentioning(got.Errors, `"## Gates"`) {
		t.Errorf("errors %q report a missing Gates section for a populated table", got.Errors)
	}
}

// TestRunValidate_ZeroGoalsYAMLIsInvalid pins the same guard on the legacy
// YAML format, where the empty denominator is an empty `goals:` list rather
// than a missing markdown section.
func TestRunValidate_ZeroGoalsYAMLIsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GOALS.yaml")
	if err := os.WriteFile(path, []byte("version: 3\nmission: Legacy\ngoals: []\n"), 0o600); err != nil {
		t.Fatalf("write goals file: %v", err)
	}

	got, runErr := runValidateJSON(t, path)

	if runErr == nil {
		t.Error("RunValidate returned nil error for a YAML goals file with zero goals")
	}
	if got.Valid {
		t.Error("Valid = true for a YAML goals file with zero goals, want false")
	}
	if !errorsMentioning(got.Errors, "goals:") {
		t.Errorf("errors %q do not name the empty goals: list", got.Errors)
	}
}
