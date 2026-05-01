package schedule

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/daemon"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", name)
}

func TestParser_LoadValidYAML(t *testing.T) {
	got, err := Load(fixture(t, "valid.yaml"))
	if err != nil {
		t.Fatalf("Load(valid.yaml) returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 schedules, got %d", len(got))
	}

	first := got[0]
	if first.Name != "nightly-wiki" {
		t.Errorf("schedules[0].name = %q, want %q", first.Name, "nightly-wiki")
	}
	if first.Cron != "0 3 * * *" {
		t.Errorf("schedules[0].cron = %q, want %q", first.Cron, "0 3 * * *")
	}
	if first.JobType != daemon.JobTypeLLMWikiLoop {
		t.Errorf("schedules[0].job_type = %q, want %q", first.JobType, daemon.JobTypeLLMWikiLoop)
	}
	if first.Timeout.Minutes() != 30 {
		t.Errorf("schedules[0].timeout = %v, want 30m", first.Timeout)
	}
	if !first.Backpressure.SkipIfRunning {
		t.Errorf("schedules[0].backpressure.skip_if_running = false, want true")
	}
	if first.Backpressure.MaxQueueDepth != 5 {
		t.Errorf("schedules[0].backpressure.max_queue_depth = %d, want 5",
			first.Backpressure.MaxQueueDepth)
	}
	if len(first.Payload) == 0 {
		t.Errorf("schedules[0].payload was empty; expected JSON-encoded body")
	} else if !strings.Contains(string(first.Payload), `"mode":"forge"`) {
		t.Errorf("schedules[0].payload = %s; expected to contain \"mode\":\"forge\"", first.Payload)
	}

	second := got[1]
	if second.Name != "hourly-rebuild" {
		t.Errorf("schedules[1].name = %q, want %q", second.Name, "hourly-rebuild")
	}
	if second.Cron != "@hourly" {
		t.Errorf("schedules[1].cron = %q, want %q", second.Cron, "@hourly")
	}
	if second.JobType != daemon.JobTypeWikiBuild {
		t.Errorf("schedules[1].job_type = %q, want %q", second.JobType, daemon.JobTypeWikiBuild)
	}
	if second.Backpressure.MaxQueueDepth != 10 {
		t.Errorf("schedules[1].backpressure.max_queue_depth = %d, want 10",
			second.Backpressure.MaxQueueDepth)
	}
}

func TestParser_RejectsDuplicateName(t *testing.T) {
	_, err := Load(fixture(t, "duplicate-name.yaml"))
	if err == nil {
		t.Fatalf("expected error for duplicate name, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate name") {
		t.Errorf("expected error to mention 'duplicate name', got: %v", err)
	}
	if !strings.Contains(err.Error(), "nightly-wiki") {
		t.Errorf("expected error to name the offending entry, got: %v", err)
	}
}

func TestParser_RejectsInvalidCron(t *testing.T) {
	_, err := Load(fixture(t, "invalid-cron.yaml"))
	if err == nil {
		t.Fatalf("expected error for invalid cron, got nil")
	}
	// Error must include the original cron string for operator clarity.
	if !strings.Contains(err.Error(), `"not a cron"`) {
		t.Errorf("expected error to include original cron string %q, got: %v",
			"not a cron", err)
	}

	var cpe *daemon.CronParseError
	if !errors.As(err, &cpe) {
		t.Errorf("expected error chain to include *daemon.CronParseError, got: %v", err)
	}
}

func TestParser_RejectsUnknownJobType(t *testing.T) {
	_, err := Load(fixture(t, "unknown-job-type.yaml"))
	if err == nil {
		t.Fatalf("expected error for unknown job_type, got nil")
	}
	if !strings.Contains(err.Error(), "job_type") {
		t.Errorf("expected error to mention job_type, got: %v", err)
	}
	if !strings.Contains(err.Error(), "FooBar") {
		t.Errorf("expected error to include offending value, got: %v", err)
	}
}

func TestParser_RejectsUnknownTopLevelField(t *testing.T) {
	_, err := Load(fixture(t, "unknown-top-field.yaml"))
	if err == nil {
		t.Fatalf("expected error for unknown top-level field, got nil")
	}
	// yaml.v3 KnownFields(true) reports "field <name> not found in type"
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("expected error to mention the unknown field 'bogus', got: %v", err)
	}
}

func TestParser_RejectsUnknownBackpressureField(t *testing.T) {
	_, err := Load(fixture(t, "unknown-backpressure-field.yaml"))
	if err == nil {
		t.Fatalf("expected error for unknown backpressure field, got nil")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("expected error to mention the unknown field 'bogus', got: %v", err)
	}
}

func TestParser_RejectsSubMinuteCron(t *testing.T) {
	// 5-field cron's tightest is `* * * * *` (60s). To test the floor, raise the
	// minimum period env to 120s — the parser must then reject the 60s schedule.
	t.Setenv(EnvMinPeriodSeconds, "120")

	_, err := Load(fixture(t, "sub-minute-cron.yaml"))
	if err == nil {
		t.Fatalf("expected error for sub-minimum cron, got nil")
	}
	if !strings.Contains(err.Error(), "below minimum") {
		t.Errorf("expected error to mention 'below minimum', got: %v", err)
	}
	if !strings.Contains(err.Error(), EnvMinPeriodSeconds) {
		t.Errorf("expected error to mention env var %q, got: %v", EnvMinPeriodSeconds, err)
	}
}

func TestParser_RejectsExcessiveQueueDepth(t *testing.T) {
	_, err := Load(fixture(t, "excessive-queue-depth.yaml"))
	if err == nil {
		t.Fatalf("expected error for excessive max_queue_depth, got nil")
	}
	if !strings.Contains(err.Error(), "max_queue_depth") {
		t.Errorf("expected error to mention max_queue_depth, got: %v", err)
	}
	if !strings.Contains(err.Error(), "9999") {
		t.Errorf("expected error to include the offending value, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ceiling") {
		t.Errorf("expected error to mention ceiling, got: %v", err)
	}
}

func TestParser_LoadStockExample(t *testing.T) {
	// Validates that .agents/schedule.yaml.example, the user-facing starter
	// shipped with agentops, loads without error and has the two expected
	// schedules (nightly-dream, hourly-forge). This protects against silent
	// breakage of the starter file via future schema changes.
	//
	// Use the testdata copy to keep the test hermetic (repo policy forbids
	// symlinks; the copy at testdata/example-validation.yaml is kept in sync
	// with .agents/schedule.yaml.example).
	templates, err := Load(fixture(t, "example-validation.yaml"))
	if err != nil {
		t.Fatalf("expected example to load; got error: %v", err)
	}
	if len(templates) != 2 {
		t.Fatalf("expected 2 schedules; got %d", len(templates))
	}

	names := make([]string, len(templates))
	for i, tmpl := range templates {
		names[i] = tmpl.Name
	}

	expected := []string{"nightly-dream", "hourly-forge"}
	for _, want := range expected {
		found := false
		for _, got := range names {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected schedule %q in starter; got names %v", want, names)
		}
	}

	// Verify both job types are real-bodied (dream.run + wiki.forge —
	// NOT llmwiki.loop, whose stage handlers are stubs in v1.0).
	for _, tmpl := range templates {
		switch tmpl.JobType {
		case daemon.JobTypeDreamRun, daemon.JobTypeWikiForge:
			// OK
		case daemon.JobTypeLLMWikiLoop:
			t.Errorf("starter must NOT include llmwiki.loop (stubs in v1.0); found in schedule %q", tmpl.Name)
		default:
			t.Errorf("starter has unexpected job_type %q in schedule %q", tmpl.JobType, tmpl.Name)
		}
	}
}

func TestParser_HonorsMinPeriodCeilingEnvOverride(t *testing.T) {
	// First, with default ceiling (1000), the excessive fixture (9999) must fail.
	if _, err := Load(fixture(t, "excessive-queue-depth.yaml")); err == nil {
		t.Fatalf("expected default ceiling to reject 9999, got nil error")
	}

	// Raise the ceiling above 9999 — now the same fixture must succeed.
	t.Setenv(EnvMaxQueueDepthCeiling, "10000")
	got, err := Load(fixture(t, "excessive-queue-depth.yaml"))
	if err != nil {
		t.Fatalf("expected raised ceiling to accept 9999, got error: %v", err)
	}
	if len(got) != 1 || got[0].Backpressure.MaxQueueDepth != 9999 {
		t.Fatalf("unexpected parse result: %+v", got)
	}

	// Sanity for the period-override direction: with the default min (60s), the
	// every-minute fixture must succeed.
	got2, err := Load(fixture(t, "sub-minute-cron.yaml"))
	if err != nil {
		t.Fatalf("expected default min-period (60s) to accept */1m cron, got: %v", err)
	}
	if len(got2) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(got2))
	}
}
