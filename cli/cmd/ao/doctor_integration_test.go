// practices: [sre, resilience-patterns]
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/quality"
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

	// JSON is the machine representation of the same bounded health checks as
	// human output, not an implicit run of the failure-mode engine.
	var rep quality.DoctorOutput
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("expected one valid health report, got parse error: %v\nraw output:\n%s", err, out)
	}
	if len(rep.Checks) == 0 || rep.Result == "" || rep.Summary == "" {
		t.Fatalf("incomplete health report: %+v", rep)
	}
}

// A fresh install with an initialized-but-empty knowledge base (no sessions, no
// learnings, no index yet) must NOT manufacture warnings: those are "you
// haven't done anything yet" info lines, not defects. This encodes the FU2
// doctrine — a pristine install is green.
func TestDoctor_Integration_FreshInstallIsNotDegraded(t *testing.T) {
	resetCommandState(t)
	dir := chdirTemp(t)

	// Minimal .agents/ — initialized base, no sessions/learnings/index yet.
	writeFile(t, dir+"/.agents/ao/sessions/.gitkeep", "")

	// The legacy check table runs in human mode (`ao doctor` without --json).
	out, _ := executeCommand("doctor")

	if out == "" {
		t.Fatal("expected doctor output, got empty string")
	}
	if strings.Contains(out, "UNHEALTHY") {
		t.Errorf("fresh install must not be UNHEALTHY, got:\n%s", out)
	}
	// "you haven't started yet" states are info lines, never warnings.
	if strings.Contains(out, "warning") {
		t.Errorf("fresh install must not surface warnings, got:\n%s", out)
	}
	// No check may point the user at a repo-relative script.
	if strings.Contains(out, "scripts/") {
		t.Errorf("fresh install names a repo-relative script, got:\n%s", out)
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
