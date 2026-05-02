package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	evalsub "github.com/boshu2/agentops/cli/internal/evalsubstrate"
	"github.com/spf13/cobra"
)

func TestRunEvalCleanupTransitionsStaleRuns(t *testing.T) {
	root := t.TempDir()
	configureEvalCleanupTest(t, root, "json", false)
	oldStartedAt := time.Now().UTC().Add(-10 * time.Minute)
	freshStartedAt := time.Now().UTC()
	writeEvalCleanupManifest(t, root, "pending-old", evalsub.StatusPending, oldStartedAt)
	writeEvalCleanupManifest(t, root, "running-old", evalsub.StatusRunning, oldStartedAt)
	writeEvalCleanupManifest(t, root, "pending-fresh", evalsub.StatusPending, freshStartedAt)

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runEvalCleanup(cmd, nil); err != nil {
		t.Fatalf("runEvalCleanup: %v", err)
	}

	var report CleanupReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, out.String())
	}
	if report.TransitionsAborted != 1 || report.TransitionsFailed != 1 {
		t.Fatalf("report = %#v, want one aborted and one failed transition", report)
	}
	assertEvalCleanupStatus(t, root, "pending-old", evalsub.StatusAborted, "never_started")
	assertEvalCleanupStatus(t, root, "running-old", evalsub.StatusFailed, "orphaned_process")
	assertEvalCleanupStatus(t, root, "pending-fresh", evalsub.StatusPending, "")
}

func TestRunEvalCleanupDeletePreservesRetractedRuns(t *testing.T) {
	root := t.TempDir()
	configureEvalCleanupTest(t, root, "json", false)
	oldStartedAt := time.Now().UTC().Add(-10 * time.Minute)
	writeEvalCleanupManifest(t, root, "failed-run", evalsub.StatusFailed, oldStartedAt)
	writeEvalCleanupManifest(t, root, "aborted-run", evalsub.StatusAborted, oldStartedAt)
	writeEvalCleanupManifest(t, root, "retracted-run", evalsub.StatusRetracted, oldStartedAt)
	evalCleanupDelete = true

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runEvalCleanup(cmd, nil); err != nil {
		t.Fatalf("runEvalCleanup: %v", err)
	}

	var report CleanupReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, out.String())
	}
	if report.RunsDeleted != 2 {
		t.Fatalf("runs deleted = %d, want 2 (report=%#v)", report.RunsDeleted, report)
	}
	assertEvalCleanupRunMissing(t, root, "failed-run")
	assertEvalCleanupRunMissing(t, root, "aborted-run")
	assertEvalCleanupStatus(t, root, "retracted-run", evalsub.StatusRetracted, "")
}

func configureEvalCleanupTest(t *testing.T, root, outputValue string, verboseValue bool) {
	t.Helper()
	t.Setenv("AGENTOPS_EVALS_ROOT", root)
	oldDelete := evalCleanupDelete
	oldTmpFiles := evalCleanupTmpFiles
	oldAge := evalCleanupAge
	oldDryRun := evalCleanupDryRun
	oldOutput := output
	oldVerbose := verbose
	evalCleanupDelete = false
	evalCleanupTmpFiles = false
	evalCleanupAge = 60
	evalCleanupDryRun = false
	output = outputValue
	verbose = verboseValue
	t.Cleanup(func() {
		evalCleanupDelete = oldDelete
		evalCleanupTmpFiles = oldTmpFiles
		evalCleanupAge = oldAge
		evalCleanupDryRun = oldDryRun
		output = oldOutput
		verbose = oldVerbose
	})
}

func writeEvalCleanupManifest(t *testing.T, root, runID string, status evalsub.RunStatus, startedAt time.Time) {
	t.Helper()
	runDir := evalsub.RunDir(root, runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	m := evalsub.Manifest{
		SchemaVersion:   evalsub.SchemaVersion,
		ID:              runID,
		StartedAt:       startedAt.Format(time.RFC3339),
		StartedAtUnixMs: startedAt.UnixNano() / int64(time.Millisecond),
		Kind:            "task",
		Status:          status,
		Seeds:           []int{},
		CapturedBy:      "eval cleanup test",
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(runDir, "manifest.json"), data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func assertEvalCleanupStatus(t *testing.T, root, runID string, want evalsub.RunStatus, wantReason string) {
	t.Helper()
	m, err := evalsub.LoadManifest(evalsub.ManifestPath(root, runID))
	if err != nil {
		t.Fatalf("load %s: %v", runID, err)
	}
	if m.Status != want {
		t.Fatalf("%s status = %s, want %s", runID, m.Status, want)
	}
	if m.RetractionReason != wantReason {
		t.Fatalf("%s retraction reason = %q, want %q", runID, m.RetractionReason, wantReason)
	}
}

func assertEvalCleanupRunMissing(t *testing.T, root, runID string) {
	t.Helper()
	if _, err := os.Stat(evalsub.RunDir(root, runID)); !os.IsNotExist(err) {
		t.Fatalf("run dir %s exists or stat failed with %v", runID, err)
	}
}
