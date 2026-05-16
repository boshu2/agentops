// practices: [sre, resilience-patterns]
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/doctor"
)

func TestDoctor_Integration_HealthyState(t *testing.T) {
	resetCommandState(t)
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Create learnings file so knowledge check passes
	writeFile(t, dir+"/.agents/learnings/test-learning.md", "# Test Learning\nSome content here.\n")

	out, err := executeCommand("doctor")

	// Doctor may return error if required checks fail (e.g., missing hooks in temp dir)
	// but it should always produce output
	if out == "" {
		t.Fatalf("expected doctor output, got empty string (err=%v)", err)
	}

	// Should contain the header
	if !strings.Contains(out, "ao doctor") {
		t.Errorf("expected output to contain 'ao doctor' header, got:\n%s", out)
	}

	// Should contain the ao CLI check (always passes)
	if !strings.Contains(out, "ao CLI") {
		t.Errorf("expected output to contain 'ao CLI' check, got:\n%s", out)
	}

	// Should contain a summary line with check counts
	hasSummary := strings.Contains(out, "checks passed") || strings.Contains(out, "HEALTHY") || strings.Contains(out, "DEGRADED") || strings.Contains(out, "UNHEALTHY")
	if !hasSummary {
		t.Errorf("expected output to contain a summary (checks passed / HEALTHY / DEGRADED / UNHEALTHY), got:\n%s", out)
	}
}

func TestDoctor_Integration_JSONOutput(t *testing.T) {
	resetCommandState(t)
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	writeFile(t, dir+"/.agents/learnings/test-learning.md", "# Learning\nContent.\n")

	out, _ := executeCommand("doctor", "--json")

	if out == "" {
		t.Fatal("expected JSON output, got empty string")
	}

	// `ao doctor --json` is the engine's machine surface: a single Report.
	// Strict unmarshal fails if a second JSON document leaked onto stdout.
	var rep doctor.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("expected a single valid engine Report, got parse error: %v\nraw output:\n%s", err, out)
	}
	if rep.SchemaVersion != "1.0" {
		t.Errorf("expected schema_version 1.0, got %q", rep.SchemaVersion)
	}
	if rep.Tool != "ao" {
		t.Errorf("expected tool 'ao', got %q", rep.Tool)
	}
	if rep.RunID == "" {
		t.Error("expected a non-empty run_id")
	}
	// Diagnose (no --fix) exits 0 (healthy) or 1 (findings present).
	if rep.ExitCode != 0 && rep.ExitCode != 1 {
		t.Errorf("expected diagnose exit_code 0 or 1, got %d", rep.ExitCode)
	}
	if rep.OK != (rep.ExitCode == 0) {
		t.Errorf("ok=%v inconsistent with exit_code=%d", rep.OK, rep.ExitCode)
	}
	if rep.Summary.TotalFindings != len(rep.Findings) {
		t.Errorf("summary.total_findings=%d but findings array has %d entries",
			rep.Summary.TotalFindings, len(rep.Findings))
	}
}

func TestDoctor_Integration_DegradedState(t *testing.T) {
	resetCommandState(t)
	dir := chdirTemp(t)

	// Minimal .agents/ without a learnings dir — should trigger legacy warnings.
	writeFile(t, dir+"/.agents/ao/sessions/.gitkeep", "")

	// The legacy check table runs in human mode (`ao doctor` without --json).
	out, _ := executeCommand("doctor")

	if out == "" {
		t.Fatal("expected doctor output, got empty string")
	}
	// A missing learnings dir must surface as a non-healthy summary.
	if !strings.Contains(out, "DEGRADED") &&
		!strings.Contains(out, "UNHEALTHY") &&
		!strings.Contains(out, "warning") {
		t.Errorf("expected a degraded/unhealthy summary, got:\n%s", out)
	}
}

func TestDoctor_Integration_NoAgentsDir(t *testing.T) {
	resetCommandState(t)
	// Completely empty directory — no .agents/ at all.
	chdirTemp(t)

	out, _ := executeCommand("doctor")

	if out == "" {
		t.Fatal("expected doctor output, got empty string")
	}
	// The ao CLI check always passes regardless of directory state.
	if !strings.Contains(out, "ao CLI") {
		t.Errorf("expected 'ao CLI' check in output, got:\n%s", out)
	}
}
