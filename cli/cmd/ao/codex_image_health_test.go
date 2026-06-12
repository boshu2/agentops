package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexImageHealthDoctorCleanJSONDoesNotMutateLifecycle(t *testing.T) {
	repo := t.TempDir()
	statePath := filepath.Join(repo, ".agents", "ao", "codex", "state.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o750); err != nil {
		t.Fatalf("create lifecycle dir: %v", err)
	}
	if err := os.WriteFile(statePath, []byte(`{"schema_version":1}`+"\n"), 0o600); err != nil {
		t.Fatalf("write lifecycle state: %v", err)
	}

	var seen []codexImageHealthCheckSpec
	withCodexImageHealthTestProject(t, repo)
	withCodexImageHealthFakeRunner(t, func(ctx context.Context, cwd string, spec codexImageHealthCheckSpec) codexImageHealthCheckResult {
		seen = append(seen, spec)
		return codexImageHealthTestResult(spec, "PASS", "")
	})

	out, err := executeCommand("codex", "image-health", "--json")
	if err != nil {
		t.Fatalf("codex image-health returned error: %v\noutput:\n%s", err, out)
	}
	result := decodeCodexImageHealthResult(t, out)
	if result.Status != "PASS" {
		t.Fatalf("status = %q, want PASS", result.Status)
	}
	if result.Summary.Total != 7 || result.Summary.Passed != 7 || result.Summary.Failed != 0 {
		t.Fatalf("summary = %+v, want seven passing checks", result.Summary)
	}
	if result.LifecycleStateMutated {
		t.Fatalf("lifecycle_state_mutated = true, want false")
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read lifecycle state: %v", err)
	}
	if strings.TrimSpace(string(data)) != `{"schema_version":1}` {
		t.Fatalf("lifecycle state changed: %q", string(data))
	}
	if len(seen) != 7 {
		t.Fatalf("seen %d specs, want 7", len(seen))
	}
	for _, spec := range seen {
		command := strings.Join(spec.Command, " ")
		for _, forbidden := range []string{"ao codex start", "ao codex stop", "ao codex ensure-start", "ao codex ensure-stop"} {
			if strings.Contains(command, forbidden) {
				t.Fatalf("image-health check %s uses lifecycle mutation command %q", spec.Name, forbidden)
			}
		}
	}
}

func TestCodexImageHealthDoctorReportsStaleHash(t *testing.T) {
	repo := t.TempDir()
	withCodexImageHealthTestProject(t, repo)
	withCodexImageHealthFakeRunner(t, failingCodexImageHealthRunner("codex-image-verify", "hash drift detected"))

	out, err := executeCommand("codex", "image-health", "--json")
	if err == nil {
		t.Fatalf("codex image-health succeeded, want stale hash failure")
	}
	result := decodeCodexImageHealthResult(t, out)
	check := findCodexImageHealthCheck(t, result, "codex-image-verify")
	if result.Status != "FAIL" || check.Status != "FAIL" {
		t.Fatalf("status result=%q check=%q, want FAIL", result.Status, check.Status)
	}
	if !strings.Contains(check.Error, "hash drift") {
		t.Fatalf("check error = %q, want hash drift", check.Error)
	}
	if !strings.Contains(check.RepairHint, "regen-codex-hashes") {
		t.Fatalf("repair hint = %q, want regen-codex-hashes", check.RepairHint)
	}
}

func TestCodexImageHealthDoctorReportsMissingOverride(t *testing.T) {
	repo := t.TempDir()
	withCodexImageHealthTestProject(t, repo)
	withCodexImageHealthFakeRunner(t, failingCodexImageHealthRunner("codex-override-coverage", "missing override entry"))

	out, err := executeCommand("codex", "image-health", "--json")
	if err == nil {
		t.Fatalf("codex image-health succeeded, want missing override failure")
	}
	result := decodeCodexImageHealthResult(t, out)
	check := findCodexImageHealthCheck(t, result, "codex-override-coverage")
	if check.Status != "FAIL" || !strings.Contains(check.Error, "missing override") {
		t.Fatalf("override check = %+v, want missing override failure", check)
	}
}

func TestCodexImageHealthDoctorReportsStaleLifecycleContract(t *testing.T) {
	repo := t.TempDir()
	withCodexImageHealthTestProject(t, repo)
	withCodexImageHealthFakeRunner(t, failingCodexImageHealthRunner("codex-lifecycle-guards", "stale lifecycle contract"))

	out, err := executeCommand("codex", "image-health", "--json")
	if err == nil {
		t.Fatalf("codex image-health succeeded, want lifecycle contract failure")
	}
	result := decodeCodexImageHealthResult(t, out)
	check := findCodexImageHealthCheck(t, result, "codex-lifecycle-guards")
	if check.Status != "FAIL" || !strings.Contains(check.Error, "stale lifecycle") {
		t.Fatalf("lifecycle check = %+v, want stale lifecycle failure", check)
	}
}

func TestCodexImageHealthDoctorSkipsOptionalRuntimeUnavailable(t *testing.T) {
	repo := t.TempDir()
	withCodexImageHealthTestProject(t, repo)
	withCodexImageHealthFakeRunner(t, func(ctx context.Context, cwd string, spec codexImageHealthCheckSpec) codexImageHealthCheckResult {
		if spec.Name == "codex-headless-runtime-skills" {
			result := codexImageHealthTestResult(spec, "SKIP", "runtime unavailable")
			result.Optional = true
			return result
		}
		return codexImageHealthTestResult(spec, "PASS", "")
	})

	out, err := executeCommand("codex", "image-health", "--json")
	if err != nil {
		t.Fatalf("codex image-health returned error for optional skip: %v\noutput:\n%s", err, out)
	}
	result := decodeCodexImageHealthResult(t, out)
	check := findCodexImageHealthCheck(t, result, "codex-headless-runtime-skills")
	if result.Status != "PASS" || result.Summary.Skipped != 1 {
		t.Fatalf("result status=%q summary=%+v, want PASS with one skip", result.Status, result.Summary)
	}
	if check.Status != "SKIP" || !check.Optional {
		t.Fatalf("headless check = %+v, want optional skip", check)
	}
}

func withCodexImageHealthTestProject(t *testing.T, repo string) {
	t.Helper()
	old := testProjectDir
	testProjectDir = repo
	t.Cleanup(func() { testProjectDir = old })
}

func withCodexImageHealthFakeRunner(t *testing.T, runner func(context.Context, string, codexImageHealthCheckSpec) codexImageHealthCheckResult) {
	t.Helper()
	old := codexImageHealthRunCheck
	codexImageHealthRunCheck = runner
	t.Cleanup(func() { codexImageHealthRunCheck = old })
}

func failingCodexImageHealthRunner(name, message string) func(context.Context, string, codexImageHealthCheckSpec) codexImageHealthCheckResult {
	return func(ctx context.Context, cwd string, spec codexImageHealthCheckSpec) codexImageHealthCheckResult {
		if spec.Name == name {
			return codexImageHealthTestResult(spec, "FAIL", message)
		}
		return codexImageHealthTestResult(spec, "PASS", "")
	}
}

func codexImageHealthTestResult(spec codexImageHealthCheckSpec, status, message string) codexImageHealthCheckResult {
	result := codexImageHealthCheckResult{
		Name:        spec.Name,
		Description: spec.Description,
		Status:      status,
		Command:     append([]string(nil), spec.Command...),
		RepairHint:  spec.RepairHint,
		ExitCode:    0,
	}
	if status == "FAIL" {
		result.ExitCode = 1
		result.Error = message
	}
	if status == "SKIP" {
		result.Error = message
	}
	return result
}

func decodeCodexImageHealthResult(t *testing.T, out string) codexImageHealthResult {
	t.Helper()
	jsonStart := strings.Index(out, "{")
	if jsonStart < 0 {
		t.Fatalf("image health output did not contain JSON object:\n%s", out)
	}
	var result codexImageHealthResult
	if err := json.Unmarshal([]byte(out[jsonStart:]), &result); err != nil {
		t.Fatalf("unmarshal image health JSON: %v\noutput:\n%s", err, out)
	}
	return result
}

func findCodexImageHealthCheck(t *testing.T, result codexImageHealthResult, name string) codexImageHealthCheckResult {
	t.Helper()
	for _, check := range result.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("missing check %q in %+v", name, result.Checks)
	return codexImageHealthCheckResult{}
}
