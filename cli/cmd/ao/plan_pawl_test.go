package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runDecide invokes the `plan-pawl decide` RunE against the package globals,
// restoring them afterwards, and returns stdout + the exit code (0 = PASS/nil).
func runDecide(t *testing.T, verdicts []string, round, maxRounds int, dir string, oscillation, judgmentFlag, jsonOut bool) (string, int) {
	t.Helper()
	save := func() func() {
		v, r, m, d, o, jf, j := planPawlVerdicts, planPawlRound, planPawlMaxRounds, planPawlDir, planPawlOscillation, planPawlJudgmentFlag, planPawlJSON
		return func() {
			planPawlVerdicts, planPawlRound, planPawlMaxRounds, planPawlDir, planPawlOscillation, planPawlJudgmentFlag, planPawlJSON = v, r, m, d, o, jf, j
		}
	}()
	defer save()

	planPawlVerdicts, planPawlRound, planPawlMaxRounds = verdicts, round, maxRounds
	planPawlDir, planPawlOscillation, planPawlJudgmentFlag, planPawlJSON = dir, oscillation, judgmentFlag, jsonOut

	var buf bytes.Buffer
	planPawlDecideCmd.SetOut(&buf)
	t.Cleanup(func() { planPawlDecideCmd.SetOut(nil); planPawlDecideCmd.SetErr(nil) }) // age-ztf8: shared command; don't leak the writer
	err := planPawlDecideCmd.RunE(planPawlDecideCmd, nil)
	code := 0
	if err != nil {
		var ee *planPawlExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			t.Fatalf("unexpected non-exit error: %v", err)
		}
	}
	return buf.String(), code
}

func TestPlanPawlDecide_PassExitZero(t *testing.T) {
	out, code := runDecide(t, []string{"claude:PASS", "gpt:PASS"}, 1, 3, "", false, false, false)
	if code != planPawlExitPass {
		t.Fatalf("want exit %d, got %d", planPawlExitPass, code)
	}
	if !strings.Contains(out, "decision: PASS") {
		t.Fatalf("want PASS in output, got: %s", out)
	}
}

func TestPlanPawlDecide_FailExitRedo(t *testing.T) {
	out, code := runDecide(t, []string{"claude:PASS", "gpt:FAIL"}, 1, 3, "", false, false, false)
	if code != planPawlExitRedo {
		t.Fatalf("want exit %d (REDO), got %d", planPawlExitRedo, code)
	}
	if !strings.Contains(out, "decision: REDO") {
		t.Fatalf("want REDO in output, got: %s", out)
	}
}

func TestPlanPawlDecide_RoundOverMaxExitBlocked(t *testing.T) {
	out, code := runDecide(t, []string{"claude:PASS", "gpt:FAIL"}, 4, 3, "", false, false, false)
	if code != planPawlExitBlocked {
		t.Fatalf("want exit %d (BLOCKED), got %d", planPawlExitBlocked, code)
	}
	if !strings.Contains(out, "breaker: max-attempts") {
		t.Fatalf("want max-attempts breaker in output, got: %s", out)
	}
}

func TestPlanPawlDecide_JudgmentFlagBlocked(t *testing.T) {
	_, code := runDecide(t, []string{"claude:PASS", "gpt:PASS"}, 1, 3, "", false, true, false)
	if code != planPawlExitBlocked {
		t.Fatalf("want exit %d (BLOCKED via judgment flag), got %d", planPawlExitBlocked, code)
	}
}

func TestPlanPawlDecide_JSONOutput(t *testing.T) {
	out, code := runDecide(t, []string{"claude:PASS", "gpt:PASS"}, 1, 3, "", false, false, true)
	if code != planPawlExitPass {
		t.Fatalf("want PASS exit, got %d", code)
	}
	// Exercise the --json output path (json.MarshalIndent): assert the full
	// indented contract — two-space-indented, complete, and round-trippable —
	// not merely that the Decision field parses. The marshal error return is now
	// checked rather than blank-discarded, but a plain decision struct (strings,
	// ints, []string) cannot fail to marshal, so that error branch is not
	// separately exercisable here; this guards the success-path output shape.
	if !strings.Contains(out, "\n  \"decision\": \"PASS\"") {
		t.Fatalf("want two-space MarshalIndent output, got: %s", out)
	}
	var decoded struct {
		Decision  string   `json:"decision"`
		Round     int      `json:"round"`
		MaxRounds int      `json:"max_rounds"`
		Families  []string `json:"families"`
	}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if decoded.Decision != "PASS" {
		t.Fatalf("want PASS, got %s", decoded.Decision)
	}
	if decoded.Round != 1 || decoded.MaxRounds != 3 {
		t.Fatalf("want round 1/3, got %d/%d", decoded.Round, decoded.MaxRounds)
	}
	if len(decoded.Families) != 2 || decoded.Families[0] != "claude" || decoded.Families[1] != "gpt" {
		t.Fatalf("want families [claude gpt], got %v", decoded.Families)
	}
}

// --dir reader round-trips real on-disk verdict files (fixture fidelity: the
// files are exactly what the skill writes — one JudgeVerdict JSON per pane).
func TestPlanPawlDecide_DirReader(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("claude.json", `{"family":"claude","disposition":"PASS"}`)
	write("gpt.json", `{"family":"gpt","disposition":"FAIL","note":"plan misses a slice"}`)

	out, code := runDecide(t, nil, 1, 3, dir, false, false, false)
	if code != planPawlExitRedo {
		t.Fatalf("want REDO from a dir with a FAIL, got %d\n%s", code, out)
	}
}

// Regression (codex refuter, age-plan-pawl-9yib.2): --judgment-flag must raise the
// hard breaker for --dir verdicts too, not only --verdict tokens. Before the fix,
// PASS/PASS dir files with --judgment-flag exited 0 PASS instead of 4 BLOCKED.
func TestPlanPawlDecide_JudgmentFlagAppliesToDirVerdicts(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []struct{ name, body string }{
		{"claude.json", `{"family":"claude","disposition":"PASS"}`},
		{"gpt.json", `{"family":"gpt","disposition":"PASS"}`},
	} {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, code := runDecide(t, nil, 1, 3, dir, false, true /* judgmentFlag */, false)
	if code != planPawlExitBlocked {
		t.Fatalf("want BLOCKED (%d) — --judgment-flag must apply to --dir verdicts, got %d", planPawlExitBlocked, code)
	}
}

// Regression (codex refuter round 2): a --dir verdict file with a missing/garbage
// disposition must fail-closed, not exit 0 PASS.
func TestPlanPawlDecide_DirFailsClosedOnBadDisposition(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []struct{ name, body string }{
		{"claude.json", `{"family":"claude","disposition":"PASS"}`},
		{"gpt.json", `{"family":"gpt"}`}, // no disposition — must NOT count as clean
	} {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, code := runDecide(t, nil, 1, 3, dir, false, false, false)
	if code == planPawlExitPass {
		t.Fatalf("a --dir file with no disposition must fail-closed, not exit 0 PASS")
	}
}

// age-gascity-port-slate-irye.2: the age-5olx incident, driven end-to-end through
// --dir. The failure_class / failure_reason (and disposition sentinel) JSON fields
// flow through planpawl.JudgeVerdict's json tags with NO cmd plumbing; a warm panel
// where codex+agy timed out must exit DEGRADED (5), not REDO (3) / PASS (0).
func TestPlanPawlDecide_DegradedExitOnTransientDirVerdicts(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []struct{ name, body string }{
		{"claude.json", `{"family":"claude","disposition":"PASS"}`},
		{"codex.json", `{"family":"codex","disposition":"<timeout>"}`},
		{"agy.json", `{"family":"agy","disposition":"PASS","failure_class":"transient","failure_reason":"provider_timeout"}`},
	} {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, code := runDecide(t, nil, 1, 3, dir, false, false, false)
	if code != planPawlExitDegraded {
		t.Fatalf("want exit %d (DEGRADED) for a timed-out panel, got %d\n%s", planPawlExitDegraded, code, out)
	}
	if !strings.Contains(out, "decision: DEGRADED") {
		t.Fatalf("want DEGRADED in output, got: %s", out)
	}
	if !strings.Contains(out, "degraded (transient lane loss)") {
		t.Fatalf("want the degraded-families line, got: %s", out)
	}
}

func TestPlanPawlDecide_NoVerdictsUsageError(t *testing.T) {
	_, code := runDecide(t, nil, 1, 3, "", false, false, false)
	if code != planPawlExitUsage {
		t.Fatalf("want usage exit %d with no verdicts, got %d", planPawlExitUsage, code)
	}
}

func TestPlanPawlDecide_BadTokenUsageError(t *testing.T) {
	_, code := runDecide(t, []string{"claude"}, 1, 3, "", false, false, false)
	if code != planPawlExitUsage {
		t.Fatalf("want usage exit on malformed token, got %d", code)
	}
}
