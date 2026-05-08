package eval

import (
	"strings"
	"testing"
)

// makeAutoDetectExp builds an Expectation of type stdout_contains_auto_detect.
func makeAutoDetectExp(command, pattern string, group int, tolerance float64) Expectation {
	return Expectation{
		Type: "stdout_contains_auto_detect",
		Value: map[string]any{
			"command":        command,
			"pattern":        pattern,
			"expected_group": group,
			"tolerance_pct":  tolerance,
		},
	}
}

func TestAutoDetect_PassWhenStdoutMatchesFreshCapture(t *testing.T) {
	exp := makeAutoDetectExp("printf 'Total: 7\\n'", `Total: ([0-9]+)`, 1, 0)
	ctx := caseContext{stdout: "logs\nTotal: 7\nALL PASSED\n"}
	got := evaluateExpectation(exp, ctx)
	if !got.passed {
		t.Fatalf("expected pass; got fail: %s", got.message)
	}
	if !strings.Contains(got.message, "Total: 7") {
		t.Errorf("pass message should reference detected match; got %q", got.message)
	}
}

func TestAutoDetect_FailLoudOnDrift(t *testing.T) {
	// Fresh detect: 8. Pinned in stdout: 7. Drift; tolerance 0 must fail loud.
	exp := makeAutoDetectExp("printf 'Total: 8\\n'", `Total: ([0-9]+)`, 1, 0)
	ctx := caseContext{stdout: "Total: 7\n"}
	got := evaluateExpectation(exp, ctx)
	if got.passed {
		t.Fatalf("expected fail on drift; got pass: %s", got.message)
	}
	if !strings.Contains(got.message, "BUMP") {
		t.Errorf("fail message should include BUMP prompt; got %q", got.message)
	}
	if !strings.Contains(got.message, "8") {
		t.Errorf("fail message should reveal new value 8; got %q", got.message)
	}
}

func TestAutoDetect_MissingBinaryFailsLoud(t *testing.T) {
	// Use a binary that definitely does not exist on PATH.
	exp := makeAutoDetectExp("definitely-not-a-binary-7c3a9f --plan", `Total: ([0-9]+)`, 1, 0)
	ctx := caseContext{stdout: "Total: 7\n"}
	got := evaluateExpectation(exp, ctx)
	if got.passed {
		t.Fatalf("expected fail when binary missing; got pass: %s", got.message)
	}
	if !strings.Contains(got.message, "not found on PATH") {
		t.Errorf("missing-binary message should be explicit; got %q", got.message)
	}
}

func TestAutoDetect_PatternNoMatchFails(t *testing.T) {
	exp := makeAutoDetectExp("printf 'unrelated\\n'", `Total: ([0-9]+)`, 1, 0)
	ctx := caseContext{stdout: "Total: 7\n"}
	got := evaluateExpectation(exp, ctx)
	if got.passed {
		t.Fatalf("expected fail when pattern misses fresh output; got pass")
	}
	if !strings.Contains(got.message, "did not match") {
		t.Errorf("expected 'did not match' wording; got %q", got.message)
	}
}

func TestAutoDetect_BatsPlanLineFormat(t *testing.T) {
	// Models the pre-push-gate-governance.json migration: bats emits "1..N"
	// as the TAP plan line.
	exp := makeAutoDetectExp("printf '1..51\\nok 1 foo\\n'", `^1\.\.([0-9]+)`, 1, 0)
	ctx := caseContext{stdout: "1..51\nok 1 foo\nok 2 bar\n"}
	got := evaluateExpectation(exp, ctx)
	if !got.passed {
		t.Fatalf("bats plan line should pass; got fail: %s", got.message)
	}
}

func TestAutoDetect_RequiresCommand(t *testing.T) {
	// Missing `command` field should produce an explicit error.
	exp := Expectation{
		Type: "stdout_contains_auto_detect",
		Value: map[string]any{
			"pattern":        `x`,
			"expected_group": 1,
		},
	}
	got := evaluateExpectation(exp, caseContext{stdout: "x"})
	if got.passed {
		t.Fatalf("expected fail when command missing; got pass")
	}
	if !strings.Contains(got.message, "command is required") {
		t.Errorf("expected 'command is required' wording; got %q", got.message)
	}
}

func TestAutoDetect_ToleranceAllowsNumericDrift(t *testing.T) {
	// Fresh: 100, pinned: 99, tolerance 5%. Should pass.
	exp := makeAutoDetectExp("printf 'Total: 100\\n'", `Total: ([0-9]+)`, 1, 5)
	ctx := caseContext{stdout: "Total: 99\n"}
	got := evaluateExpectation(exp, ctx)
	if !got.passed {
		t.Fatalf("expected tolerance to allow 100 vs 99 within 5%%; got fail: %s", got.message)
	}
}

func TestAutoDetect_RegistryListsAutoDetectType(t *testing.T) {
	if !validExpectationType("stdout_contains_auto_detect") {
		t.Fatal("validExpectationType must accept stdout_contains_auto_detect")
	}
}
