package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
)

func TestDaemonSoakQueueOnly(t *testing.T) {
	cwd := t.TempDir()
	report, err := runDaemonSoak(context.Background(), cwd, daemonSoakOptions{
		Scenario: "queue-only",
		RunID:    "soak-queue-only",
		Now:      fixedDaemonSoakNow,
	})
	if err != nil {
		t.Fatalf("runDaemonSoak: %v", err)
	}
	if report.Status != "pass" || len(report.Jobs) != 1 {
		t.Fatalf("report = %#v, want pass with one job", report)
	}
	if report.Jobs[0].Status != daemonpkg.JobStatusQueued {
		t.Fatalf("job status = %q, want queued", report.Jobs[0].Status)
	}
	assertDaemonSoakProofFiles(t, report)
}

func TestDaemonSoakRequireTerminalFailsOnQueuedJobs(t *testing.T) {
	cwd := t.TempDir()
	report, err := runDaemonSoak(context.Background(), cwd, daemonSoakOptions{
		Scenario:        "queue-only",
		RequireTerminal: true,
		RunID:           "soak-terminal-required",
		Now:             fixedDaemonSoakNow,
	})
	if err == nil {
		t.Fatal("runDaemonSoak returned nil error for queued require-terminal job")
	}
	if report.Status != "fail" || !strings.Contains(report.Failure, "require-terminal") {
		t.Fatalf("report = %#v, want require-terminal failure", report)
	}
	assertDaemonSoakProofFiles(t, report)
}

func TestDaemonSoakFakeExecutorWritesProof(t *testing.T) {
	cwd := t.TempDir()
	report, err := runDaemonSoak(context.Background(), cwd, daemonSoakOptions{
		Scenario:        "fake-executor",
		RequireTerminal: true,
		RunID:           "soak-fake-executor",
		Now:             fixedDaemonSoakNow,
	})
	if err != nil {
		t.Fatalf("runDaemonSoak: %v", err)
	}
	if report.Status != "pass" || len(report.Jobs) != 1 || len(report.OpenClawJobs) != 1 {
		t.Fatalf("report = %#v, want one daemon and OpenClaw job", report)
	}
	if report.Jobs[0].Status != daemonpkg.JobStatusCompleted {
		t.Fatalf("daemon job status = %q, want completed", report.Jobs[0].Status)
	}
	if report.OpenClawJobs[0].Status != string(daemonpkg.JobStatusCompleted) {
		t.Fatalf("OpenClaw job status = %q, want completed", report.OpenClawJobs[0].Status)
	}
	if report.Jobs[0].Artifacts["soak_report"] != report.Proof.ReportJSON {
		t.Fatalf("job artifacts = %#v, want soak_report proof path", report.Jobs[0].Artifacts)
	}
	assertDaemonSoakProofFiles(t, report)
}

func fixedDaemonSoakNow() time.Time {
	return time.Date(2026, 4, 29, 18, 0, 0, 0, time.UTC)
}

func assertDaemonSoakProofFiles(t *testing.T, report daemonSoakReport) {
	t.Helper()
	for _, path := range []string{
		report.Proof.ScenarioJSON,
		report.Proof.EventsJSONL,
		report.Proof.ReportJSON,
		report.Proof.SummaryMD,
	} {
		if path == "" {
			t.Fatalf("empty proof path in %#v", report.Proof)
		}
		if _, err := os.Stat(filepath.Clean(path)); err != nil {
			t.Fatalf("proof file %s: %v", path, err)
		}
	}
}
